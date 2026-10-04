package similarity

import (
	"reflect"
	"testing"
)

// Threshold 0.7 sits in the gap between the weakest real match
// (флйа/флай, 0.75: one transposition in four letters) and the strongest
// unrelated pair (підтягування/відтискання, 0.58), with margin on both sides.
func TestScoreThreshold(t *testing.T) {
	tests := []struct {
		a, b  string
		above bool
	}{
		{"розаедення гантелей", "розведення гантелей", true},
		{"підтянування", "підтягування", true},
		{"precher curls", "preacher curls", true},
		{"жим ногамм", "жим ногами", true},
		{"revere fly", "reverse fly", true},
		{"гіперікстензія", "гіперекстензія", true},
		{"пітдягування", "підтягування", true},
		{"флйа", "флай", true},
		{"гантелі жим в нахилі", "жим в нахилі гантелі", true},
		{"тяга верт", "верт тяга", true},
		{"  Жим   Лежачи ", "жим лежачи", true},
		{"жим лежачи", "присідання", false},
		{"флай", "face pulls", false},
		{"станова тяга", "згинання на біцепс", false},
		{"підтягування", "відтискання", false},
		{"", "флай", false},
	}
	for _, tt := range tests {
		t.Run(tt.a+" | "+tt.b, func(t *testing.T) {
			got := Score(tt.a, tt.b)
			if got < 0 || got > 1 {
				t.Fatalf("score %v out of range", got)
			}
			if (got >= Threshold) != tt.above {
				t.Errorf("Score = %.3f, want above threshold %.2f: %v", got, Threshold, tt.above)
			}
		})
	}
}

func TestScoreIdentical(t *testing.T) {
	if got := Score("Жим  лежачи", "жим лежачи "); got != 1 {
		t.Errorf("Score = %v, want 1", got)
	}
}

func TestRank(t *testing.T) {
	cands := []string{"розведення гантелей ззаду", "жим лежачи", "розведення гантелей", "флай"}
	tests := []struct {
		name  string
		limit int
		want  []string
	}{
		{"all above threshold", 0, []string{"розведення гантелей", "розведення гантелей ззаду"}},
		{"limit", 1, []string{"розведення гантелей"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, m := range Rank("розаедення гантелей", cands, tt.limit) {
				got = append(got, m.Name)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
