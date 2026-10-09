package bot

import (
	"context"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gymbro/internal/apiclient"
	"gymbro/internal/bot/state"
	"gymbro/internal/parser"
)

func TestParseAliasArgs(t *testing.T) {
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
			bad, correct, ok := parseAliasArgs(tt.in)
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
	ctx := context.Background()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cache := newDraftCache(openTestStore(t, filepath.Join(t.TempDir(), "bot.db")), 48*time.Hour, func() time.Time { return now })
	k1, k2 := draftKey{1, 1}, draftKey{1, 2}
	cache.put(ctx, k1, &draft{})
	cache.put(ctx, k2, &draft{})

	now = now.Add(47 * time.Hour)
	if _, ok := cache.get(ctx, k1); !ok { // touch k1
		t.Fatal("k1 evicted before TTL")
	}

	now = now.Add(2 * time.Hour) // k2 idle 49h, k1 idle 2h
	cache.sweep(ctx)
	if cache.len() != 1 {
		t.Fatalf("sweep left %d drafts, want 1", cache.len())
	}
	if _, ok := cache.get(ctx, k2); ok {
		t.Error("k2 should be evicted")
	}
	if _, ok := cache.get(ctx, k1); !ok {
		t.Error("touched k1 should survive")
	}

	// Lazy eviction on get without a sweep.
	now = now.Add(49 * time.Hour)
	if _, ok := cache.get(ctx, k1); ok {
		t.Error("expired k1 returned")
	}
	if cache.len() != 0 {
		t.Errorf("lazy eviction left %d drafts", cache.len())
	}
}

func TestDraftCacheLatest(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cache := newDraftCache(openTestStore(t, filepath.Join(t.TempDir(), "bot.db")), 48*time.Hour, func() time.Time { return now })
	d1, d2, other := &draft{}, &draft{}, &draft{}
	k1, k2 := draftKey{1, 1}, draftKey{1, 2}

	if _, _, ok := cache.latest(ctx, 1); ok {
		t.Fatal("empty cache returned a draft")
	}
	cache.put(ctx, k1, d1)
	now = now.Add(time.Hour)
	cache.put(ctx, k2, d2)
	now = now.Add(time.Hour)
	cache.put(ctx, draftKey{2, 3}, other)

	if key, d, ok := cache.latest(ctx, 1); !ok || key != k2 || d != d2 {
		t.Fatalf("latest = (%v, %p, %v), want k2", key, d, ok)
	}
	now = now.Add(time.Hour)
	cache.get(ctx, k1) // touching k1 makes it the latest
	if key, _, _ := cache.latest(ctx, 1); key != k1 {
		t.Fatalf("latest after touch = %v, want k1", key)
	}
	now = now.Add(49 * time.Hour)
	if _, _, ok := cache.latest(ctx, 1); ok {
		t.Fatal("expired drafts returned")
	}
}

func openTestStore(t *testing.T, path string) *state.Store {
	t.Helper()
	st, err := state.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// restartedCache opens a fresh cache over the same DB file, like a bot restart.
func restartedCache(t *testing.T, path string, now func() time.Time) *draftCache {
	return newDraftCache(openTestStore(t, path), 48*time.Hour, now)
}

func TestDraftPersistence(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "bot.db")
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	cache := restartedCache(t, path, clock)

	key := draftKey{1, 10}
	want := &draft{
		Text:           "bench 3x5x100",
		Posted:         time.Date(2025, 12, 31, 18, 30, 0, 0, postZone),
		Kept:           map[string]bool{"benchh": true},
		PreviewIDs:     []int{11, 12},
		Suspects:       []suspectRef{{Key: "benchh", Name: "Benchh"}, {Key: "squatt", Name: "Squatt"}},
		SavedWorkoutID: 42,
	}
	cache.put(ctx, key, want)

	// Restart: a fresh cache loads the draft from SQLite with every field.
	now = now.Add(time.Hour)
	fresh := restartedCache(t, path, clock)
	got, ok := fresh.get(ctx, key)
	if !ok {
		t.Fatal("draft not loaded after restart")
	}
	if got.Text != want.Text || !got.Posted.Equal(want.Posted) || got.SavedWorkoutID != 42 ||
		!reflect.DeepEqual(got.Kept, want.Kept) || !reflect.DeepEqual(got.PreviewIDs, want.PreviewIDs) ||
		!reflect.DeepEqual(got.Suspects, want.Suspects) {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
	if again, _ := fresh.get(ctx, key); again != got {
		t.Error("second get must return the cached draft")
	}

	// A mutation persisted via persist is seen after the next restart.
	got.Text = "edited"
	fresh.persist(ctx, key, got)
	got2, ok := restartedCache(t, path, clock).get(ctx, key)
	if !ok || got2.Text != "edited" {
		t.Fatalf("persisted edit lost: %+v ok=%v", got2, ok)
	}

	if _, ok := fresh.get(ctx, draftKey{1, 99}); ok {
		t.Error("unknown draft found")
	}
}

func TestDraftPersistenceTTL(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "bot.db")
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	cache := restartedCache(t, path, clock)
	k1, k2 := draftKey{1, 1}, draftKey{1, 2}
	cache.put(ctx, k1, &draft{Text: "a"})
	cache.put(ctx, k2, &draft{Text: "b"})

	// A stale row reads as missing and is deleted.
	now = now.Add(49 * time.Hour)
	fresh := restartedCache(t, path, clock)
	if _, ok := fresh.get(ctx, k1); ok {
		t.Fatal("stale row returned")
	}
	st := openTestStore(t, path)
	_, found, err := st.LoadDraft(ctx, k1.ChatID, k1.MsgID)
	if err != nil || found {
		t.Fatalf("stale row not deleted: found=%v err=%v", found, err)
	}

	// The sweep deletes the remaining expired row.
	fresh.sweep(ctx)
	_, found, err = st.LoadDraft(ctx, k2.ChatID, k2.MsgID)
	if err != nil || found {
		t.Fatalf("sweep kept expired row: found=%v err=%v", found, err)
	}
}

func TestDraftPersistenceLatest(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "bot.db")
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	cache := restartedCache(t, path, clock)
	cache.put(ctx, draftKey{1, 1}, &draft{Text: "old"})
	now = now.Add(time.Hour)
	cache.put(ctx, draftKey{1, 2}, &draft{Text: "newer"})
	now = now.Add(time.Hour)
	cache.put(ctx, draftKey{2, 3}, &draft{Text: "other chat"})

	fresh := restartedCache(t, path, clock)
	key, d, ok := fresh.latest(ctx, 1)
	if !ok || key != (draftKey{1, 2}) || d.Text != "newer" {
		t.Fatalf("latest = (%v, %v, %v), want msg 2", key, d, ok)
	}
	if got, _ := fresh.get(ctx, key); got != d {
		t.Error("latest must adopt the draft into the cache")
	}

	now = now.Add(49 * time.Hour)
	if _, _, ok := restartedCache(t, path, clock).latest(ctx, 1); ok {
		t.Fatal("expired rows returned by latest")
	}
}

func TestSourceRef(t *testing.T) {
	// Same chat and message id under two bots must give two different keys.
	first := sourceRef(111, 685751256, 4)
	second := sourceRef(222, 685751256, 4)
	if first != "111:685751256:4" {
		t.Errorf("sourceRef = %q, want 111:685751256:4", first)
	}
	if first == second {
		t.Errorf("two bots share the key %q", first)
	}
}
