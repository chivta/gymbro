package server

import (
	"testing"

	"gymbro/internal/apiclient"
)

func TestSaveWorkoutValidation(t *testing.T) {
	v, err := newValidator()
	if err != nil {
		t.Fatal(err)
	}
	neg := -1

	valid := func() apiclient.SaveWorkoutRequest {
		return apiclient.SaveWorkoutRequest{
			PerformedOn: "2026-10-03", Type: "Upper", RawText: "x", Source: "s", SourceRef: "1",
			Entries: []apiclient.Entry{{Name: "жим", Sets: []apiclient.Set{{Weight: "60.5", Reps: 10}}}},
		}
	}
	tests := []struct {
		name   string
		mutate func(*apiclient.SaveWorkoutRequest)
		ok     bool
	}{
		{"valid", func(*apiclient.SaveWorkoutRequest) {}, true},
		{"no entries sets", func(r *apiclient.SaveWorkoutRequest) { r.Entries[0].Sets = nil }, true},
		{"bad type", func(r *apiclient.SaveWorkoutRequest) { r.Type = "cardio" }, false},
		{"bad date", func(r *apiclient.SaveWorkoutRequest) { r.PerformedOn = "03.10.2026" }, false},
		{"negative kcal", func(r *apiclient.SaveWorkoutRequest) { r.Kcal = &neg }, false},
		{"three decimals", func(r *apiclient.SaveWorkoutRequest) { r.Entries[0].Sets[0].Weight = "60.125" }, false},
		{"negative weight", func(r *apiclient.SaveWorkoutRequest) { r.Entries[0].Sets[0].Weight = "-5" }, false},
		{"zero reps", func(r *apiclient.SaveWorkoutRequest) { r.Entries[0].Sets[0].Reps = 0 }, false},
		{"empty name", func(r *apiclient.SaveWorkoutRequest) { r.Entries[0].Name = "" }, false},
		{"no source ref", func(r *apiclient.SaveWorkoutRequest) { r.SourceRef = "" }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := valid()
			tt.mutate(&req)
			err := v.Struct(req)
			if (err == nil) != tt.ok {
				t.Fatalf("ok=%v, err=%v", tt.ok, err)
			}
		})
	}
}
