package parser

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func intp(n int) *int { return &n }

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

var posted = time.Date(2026, time.October, 3, 20, 0, 0, 0, time.UTC)

func sets(pairs ...any) []Set {
	var out []Set
	for i := 0; i < len(pairs); i += 2 {
		out = append(out, Set{Weight: pairs[i].(string), Reps: pairs[i+1].(int)})
	}
	return out
}

func TestParse(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		posted time.Time
		want   Workout
	}{
		{
			name: "data_model example",
			text: "03.10 upper (680/37)\nжим в нахилі сміт 60-10 -9 50-11 -10\nпідтягування 82-9 -9\nскручування -10 -20 -30\n",
			want: Workout{
				Date: day(2026, 10, 3), Type: "upper", Kcal: intp(680), Protein: intp(37),
				Entries: []Entry{
					{"жим в нахилі сміт", sets("60", 10, "60", 9, "50", 11, "50", 10)},
					{"підтягування", sets("82", 9, "82", 9)},
					{"скручування", sets("0", 10, "0", 20, "0", 30)},
				},
			},
		},
		{
			name: "weight carries forward and resets per line",
			text: "03.10\nжим 60-10 -9\nтяга -5 40-6 -7\nприсід -8",
			want: Workout{
				Date: day(2026, 10, 3),
				Entries: []Entry{
					{"жим", sets("60", 10, "60", 9)},
					{"тяга", sets("0", 5, "40", 6, "40", 7)},
					{"присід", sets("0", 8)},
				},
			},
		},
		{
			name: "decimal weights",
			text: "03.10\nрозведення 13.5-8 -8\nтяга 18.75-18",
			want: Workout{
				Date: day(2026, 10, 3),
				Entries: []Entry{
					{"розведення", sets("13.5", 8, "13.5", 8)},
					{"тяга", sets("18.75", 18)},
				},
			},
		},
		{
			name: "multi-word cyrillic name and blank lines",
			text: "\n\n03.10 push\n\nжим гантелей в нахилі 30-10\n\n",
			want: Workout{
				Date: day(2026, 10, 3), Type: "push",
				Entries: []Entry{{"жим гантелей в нахилі", sets("30", 10)}},
			},
		},
		{name: "missing type and intake", text: "03.10\nжим 60-10", want: Workout{
			Date: day(2026, 10, 3), Entries: []Entry{{"жим", sets("60", 10)}}}},
		{name: "intake without type", text: "03.10 (500/30)\nжим 60-10", want: Workout{
			Date: day(2026, 10, 3), Kcal: intp(500), Protein: intp(30), Entries: []Entry{{"жим", sets("60", 10)}}}},
		{name: "type with note", text: "03.10 upper в бергені\nжим 60-10", want: Workout{
			Date: day(2026, 10, 3), Type: "upper", Note: "в бергені", Entries: []Entry{{"жим", sets("60", 10)}}}},
		{name: "unrecognized type keeps whole tail as note", text: "03.10 талін дн!! pull\nжим 60-10", want: Workout{
			Date: day(2026, 10, 3), Note: "талін дн!! pull", Entries: []Entry{{"жим", sets("60", 10)}}}},
		{name: "case-insensitive type", text: "03.10 Push\nжим 60-10", want: Workout{
			Date: day(2026, 10, 3), Type: "push", Entries: []Entry{{"жим", sets("60", 10)}}}},
		{name: "canonical spelling for mixed-case multiword type", text: "03.10 WORKOUT a\nжим 60-10", want: Workout{
			Date: day(2026, 10, 3), Type: "workout A", Entries: []Entry{{"жим", sets("60", 10)}}}},
		{name: "cyrillic type, uppercase", text: "03.10 День 1\nжим 60-10", want: Workout{
			Date: day(2026, 10, 3), Type: "день 1", Entries: []Entry{{"жим", sets("60", 10)}}}},
		{name: "longest type wins", text: "03.10 push+pull\nжим 60-10", want: Workout{
			Date: day(2026, 10, 3), Type: "push+pull", Entries: []Entry{{"жим", sets("60", 10)}}}},
		{name: "type is not a prefix of a longer word", text: "03.10 pushes\nжим 60-10", want: Workout{
			Date: day(2026, 10, 3), Note: "pushes", Entries: []Entry{{"жим", sets("60", 10)}}}},
		{name: "intake before type", text: "03.10 (1/2) pull\nжим 60-10", want: Workout{
			Date: day(2026, 10, 3), Type: "pull", Kcal: intp(1), Protein: intp(2), Entries: []Entry{{"жим", sets("60", 10)}}}},
		{name: "intake after note", text: "03.10 lower болить спина (400/20)\nжим 60-10", want: Workout{
			Date: day(2026, 10, 3), Type: "lower", Note: "болить спина", Kcal: intp(400), Protein: intp(20),
			Entries: []Entry{{"жим", sets("60", 10)}}}},
		{name: "intake with trailing words", text: "03.10 pull (340/11 ккал/б перед тренуванням)\nжим 60-10", want: Workout{
			Date: day(2026, 10, 3), Type: "pull", Kcal: intp(340), Protein: intp(11), Entries: []Entry{{"жим", sets("60", 10)}}}},
		{name: "explicit year", text: "05.01.2025 pull\nжим 60-10", want: Workout{
			Date: day(2025, 1, 5), Type: "pull", Entries: []Entry{{"жим", sets("60", 10)}}}},
		{name: "no year uses post year without rollover", text: "31.12\nжим 60-10", posted: time.Date(2027, 1, 2, 9, 0, 0, 0, time.UTC),
			want: Workout{Date: day(2027, 12, 31), Entries: []Entry{{"жим", sets("60", 10)}}}},
		{name: "leap day with explicit year", text: "29.02.2024\nжим 60-10", want: Workout{
			Date: day(2024, 2, 29), Entries: []Entry{{"жим", sets("60", 10)}}}},
		{name: "header only", text: "03.10 рест", want: Workout{Date: day(2026, 10, 3), Type: "рест"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.posted
			if p.IsZero() {
				p = posted
			}
			got, err := Parse(tt.text, p)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got  %+v\nwant %+v", got, tt.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		line  int
		token string
		want  Reason
	}{
		{"empty text", "", 0, "", ReasonEmpty},
		{"whitespace only", " \n\t\n", 0, "", ReasonEmpty},
		{"header without date", "upper\nжим 60-10", 1, "upper", ReasonBadDate},
		{"date glued to text", "03.10upper\nжим 60-10", 1, "03.10upper", ReasonBadDate},
		{"invalid day", "32.10\nжим 60-10", 1, "32.10", ReasonBadDate},
		{"invalid month", "03.13\nжим 60-10", 1, "03.13", ReasonBadDate},
		{"feb 29 in non-leap post year", "29.02\nжим 60-10", 1, "29.02", ReasonBadDate},
		{"blank lines count toward line numbers", "\n\nupper", 3, "upper", ReasonBadDate},
		{"intake overflow", "03.10 (99999999999999999999/1)\nжим 60-10", 1, "(99999999999999999999/1)", ReasonBadIntake},
		{"line without sets", "03.10\nжим 60-10\nприсідання", 3, "", ReasonNoSets},
		{"line without name", "03.10\n60-10 -9", 2, "60-10", ReasonNoName},
		{"bare reps without name", "03.10\n-10", 2, "-10", ReasonNoName},
		{"bad token after sets", "03.10\nжим 60-10 abc -9", 2, "abc", ReasonBadToken},
		{"three decimals is not a set", "03.10\nжим 60-10 13.555-8", 2, "13.555-8", ReasonBadToken},
		{"comma decimal", "03.10\nжим 60-10 13,5-8", 2, "13,5-8", ReasonBadToken},
		{"weight without reps", "03.10\nжим 60-10 50-", 2, "50-", ReasonBadToken},
		{"zero reps", "03.10\nжим 60-10\nтяга 40-0", 3, "40-0", ReasonZeroReps},
		{"zero reps bare", "03.10\nжим 60-10 -0", 2, "-0", ReasonZeroReps},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.text, posted)
			var pe *Error
			if !errors.As(err, &pe) {
				t.Fatalf("err = %v, want *Error", err)
			}
			if pe.Reason != tt.want || pe.Line != tt.line || pe.Token != tt.token {
				t.Errorf("got line=%d token=%q reason=%s, want line=%d token=%q reason=%s",
					pe.Line, pe.Token, pe.Reason, tt.line, tt.token, tt.want)
			}
		})
	}
}
