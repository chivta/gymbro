package bot

import (
	"math"
	"strings"
	"testing"
	"time"

	"gymbro/internal/apiclient"
	"gymbro/internal/parser"
)

func TestParseReplaceArgs(t *testing.T) {
	tests := []struct {
		name         string
		in           string
		bad, correct string
		ok           bool
	}{
		{"simple", "a => b", "a", "b", true},
		{"spaces in names", "жим лежачи => жим штанги лежачи", "жим лежачи", "жим штанги лежачи", true},
		{"surrounding whitespace", "  a => b  ", "a", "b", true},
		{"extra inner spaces", "a =>  b", "a", "b", true},
		{"empty", "", "", "", false},
		{"no separator", "a b", "", "", false},
		{"no spaces around arrow", "a=>b", "", "", false},
		{"two separators", "a => b => c", "", "", false},
		{"missing old", "=> b", "", "", false},
		{"missing new", "a =>", "", "", false},
		{"blank new", "a =>  ", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bad, correct, ok := parseReplaceArgs(tt.in)
			if ok != tt.ok || bad != tt.bad || correct != tt.correct {
				t.Fatalf("got (%q, %q, %v), want (%q, %q, %v)", bad, correct, ok, tt.bad, tt.correct, tt.ok)
			}
		})
	}
}

func TestSplitMessage(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		limit int
		want  []string
	}{
		{"fits", "a\nb", 10, []string{"a\nb"}},
		{"exact fit", "ab\ncd", 5, []string{"ab\ncd"}},
		{"splits at line boundary", "ab\ncd\nef", 5, []string{"ab\ncd", "ef"}},
		{"one line per chunk", "abc\ndef\nghi", 4, []string{"abc", "def", "ghi"}},
		{"counts runes not bytes", "жжж\nжжж", 7, []string{"жжж\nжжж"}},
		{"long line is cut", "abcdefg", 3, []string{"abc", "def", "g"}},
		{"long line between lines", "x\nabcdef\ny", 4, []string{"x", "abcd", "ef\ny"}},
		{"empty", "", 5, []string{""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitMessage(tt.text, tt.limit)
			if strings.Join(got, "|") != strings.Join(tt.want, "|") || len(got) != len(tt.want) {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
			for _, p := range got {
				if len([]rune(p)) > tt.limit {
					t.Fatalf("part %q exceeds limit %d", p, tt.limit)
				}
			}
		})
	}
}

func TestCallbackRoundTrip(t *testing.T) {
	tests := []callbackData{
		{Kind: cbPick, MsgID: 12, NameIdx: 0, ExerciseID: 345},
		{Kind: cbPick, MsgID: math.MaxInt64, NameIdx: math.MaxInt32, ExerciseID: math.MaxInt64},
		{Kind: cbKeep, MsgID: 7, NameIdx: 3},
		{Kind: cbKeep, MsgID: math.MaxInt64, NameIdx: math.MaxInt64},
		{Kind: cbSave, MsgID: 99},
		{Kind: cbSave, MsgID: math.MaxInt64},
	}
	for _, tt := range tests {
		enc := tt.encode()
		if len(enc) > 64 {
			t.Errorf("%q is %d bytes, over the 64 byte limit", enc, len(enc))
		}
		got, ok := decodeCallback(enc)
		if !ok || got != tt {
			t.Errorf("decode(%q) = %+v, %v; want %+v", enc, got, ok, tt)
		}
	}
}

func TestDecodeCallbackRejects(t *testing.T) {
	for _, in := range []string{"", "x:1", "p:1:2", "p:1:2:3:4", "k:1", "s", "s:a", "s:-1", "k:1:-2", "p:1:2:x", "s:1:2"} {
		_, ok := decodeCallback(in)
		if ok {
			t.Errorf("decode(%q) accepted", in)
		}
	}
}

// Real exercise list: "розведення гантелі" is an alias of "розведення гантелей".
var testExercises = []apiclient.Exercise{
	{ID: 1, Name: "розведення гантелей", Aliases: []string{"розведення гантелі"}},
	{ID: 2, Name: "жим лежачи"},
	{ID: 3, Name: "підтягування"},
}

func TestAnalyzeNames(t *testing.T) {
	tests := []struct {
		name      string
		written   string
		known     bool
		suspected bool
		matchIDs  []int64
	}{
		{"misspelling is suspected", "розаедення гантелей", false, true, []int64{1}},
		{"alias is known", "розведення гантелі", true, false, nil},
		{"alias with different case and spacing", "  Розведення   Гантелі ", true, false, nil},
		{"exercise name is known", "жим лежачи", true, false, nil},
		{"no similar exercise is a plain new name", "тестова вправа", false, false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analyzeNames([]parser.Entry{{Name: tt.written}}, testExercises)
			if len(got) != 1 {
				t.Fatalf("got %d names, want 1", len(got))
			}
			n := got[0]
			if n.Known != tt.known || n.suspected() != tt.suspected {
				t.Fatalf("known=%v suspected=%v, want %v %v", n.Known, n.suspected(), tt.known, tt.suspected)
			}
			var ids []int64
			for _, m := range n.Matches {
				ids = append(ids, m.ID)
			}
			if len(ids) != len(tt.matchIDs) || (len(ids) > 0 && ids[0] != tt.matchIDs[0]) {
				t.Fatalf("match ids %v, want %v", ids, tt.matchIDs)
			}
		})
	}
}

func TestAnalyzeNamesDedupesByKeyAndResolves(t *testing.T) {
	entries := []parser.Entry{{Name: "розаедення гантелей"}, {Name: "Розаедення  Гантелей"}, {Name: "жим лежачи"}}
	names := analyzeNames(entries, testExercises)
	if len(names) != 2 {
		t.Fatalf("got %d names, want 2 distinct", len(names))
	}
	if !unresolved(names, nil) {
		t.Error("suspected name must be unresolved")
	}
	if unresolved(names, map[string]bool{names[0].Key: true}) {
		t.Error("kept name must count as resolved")
	}
}

func TestDraftCacheTTL(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cache := newDraftCache(48*time.Hour, func() time.Time { return now })
	k1, k2 := draftKey{1, 1}, draftKey{1, 2}
	cache.put(k1, &draft{})
	cache.put(k2, &draft{})

	now = now.Add(47 * time.Hour)
	if _, ok := cache.get(k1); !ok { // touch k1
		t.Fatal("k1 evicted before TTL")
	}

	now = now.Add(2 * time.Hour) // k2 idle 49h, k1 idle 2h
	cache.sweep()
	if cache.len() != 1 {
		t.Fatalf("sweep left %d drafts, want 1", cache.len())
	}
	if _, ok := cache.get(k2); ok {
		t.Error("k2 should be evicted")
	}
	if _, ok := cache.get(k1); !ok {
		t.Error("touched k1 should survive")
	}

	// Lazy eviction on get without a sweep.
	now = now.Add(49 * time.Hour)
	if _, ok := cache.get(k1); ok {
		t.Error("expired k1 returned")
	}
	if cache.len() != 0 {
		t.Errorf("lazy eviction left %d drafts", cache.len())
	}
}
