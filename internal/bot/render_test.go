package bot

import (
	"strings"
	"testing"
	"time"

	"gymbro/internal/parser"
)

func TestRenderWorkout(t *testing.T) {
	posted := time.Date(2026, time.October, 4, 0, 0, 0, 0, time.UTC)

	w, err := parser.Parse("04.10 push\nжим лежачи 60-10 -9\nрозведення гантелей 20-", posted)
	if err != nil {
		t.Fatal(err)
	}
	p := renderWorkout(w, nil, nil, 0, 1)
	got := strings.Join(p.Parts, "")
	if !strings.Contains(got, tr(txtPlannedUnsavable)) {
		t.Errorf("missing unsavable line: %s", got)
	}
	if strings.Contains(got, "60") {
		t.Errorf("preview must not echo the sets: %s", got)
	}
	if p.Markup != nil {
		t.Errorf("Save button must be absent, got %+v", p.Markup)
	}

	w, _ = parser.Parse("04.10\nжим 60-10\nтестова вправа 10-10", posted)
	names := []nameInfo{{Key: "жим", Name: "жим", Known: true}, {Key: "тестова вправа", Name: "тестова вправа"}}
	p = renderWorkout(w, names, nil, 0, 1)
	got = strings.Join(p.Parts, "")
	if got != tr(txtNewNames, "тестова вправа") {
		t.Errorf("want only the new-names line, got %q", got)
	}
	if p.Markup == nil {
		t.Error("done workout must have Save")
	}

	names[1].Known = true
	if got := strings.Join(renderWorkout(w, names, nil, 0, 1).Parts, ""); got != tr(txtReady) {
		t.Errorf("all known: want ready line, got %q", got)
	}
}

func TestRenderParseError(t *testing.T) {
	_, err := parser.Parse("04.10\nжим 60-10\nфлай 60-x", time.Now())
	perr, ok := err.(*parser.Error)
	if !ok {
		t.Fatalf("want *parser.Error, got %v", err)
	}
	got := strings.Join(renderParseError(perr).Parts, "")
	if strings.Count(got, "\n") != 0 || !strings.Contains(got, "флай 60-x") {
		t.Errorf("want one line naming the failing line, got %q", got)
	}
}
