// Package parser turns the compact workout text log (data_model.md, "Text log
// format") into a structured Workout. It does no I/O.
package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"gymbro/internal/workout"
)

// Reason is a machine-readable parse failure code; the bot maps it to a message.
type Reason string

const (
	ReasonEmpty     Reason = "empty"      // no non-empty line in the text
	ReasonBadDate   Reason = "bad_date"   // header has no valid DD.MM[.YYYY] date
	ReasonBadIntake Reason = "bad_intake" // intake block numbers do not fit an int
	ReasonNoSets    Reason = "no_sets"    // exercise line has no set tokens
	ReasonNoName    Reason = "no_name"    // exercise line starts with a set token
	ReasonBadToken  Reason = "bad_token"  // non-set token after the first set token
	ReasonZeroReps  Reason = "zero_reps"  // set token with 0 reps
)

// Error is the failure of a whole message. Line is 1-based within the message
// (0 when the text is empty), Text is that line, Token the offending token
// (may be empty). Error() is for logs; the bot renders Reason itself.
type Error struct {
	Line   int
	Text   string
	Token  string
	Reason Reason
}

func (e *Error) Error() string {
	return fmt.Sprintf("parse error: line %d: reason %s token %q", e.Line, e.Reason, e.Token)
}

// Workout is the parsed message. Date is midnight UTC of the calendar date.
// Type is the canonical spelling from workout.Types, "" when absent.
// Kcal and Protein are nil when there is no intake block.
type Workout struct {
	Date    time.Time
	Type    string
	Kcal    *int
	Protein *int
	Note    string
	Entries []Entry
}

// Entry is one exercise line: the name as written and its sets in order.
type Entry struct {
	Name string
	Sets []Set
}

// Set is one set. Weight is the decimal string as written ("60", "13.5"),
// never a float, so it round-trips exactly.
type Set struct {
	Weight string
	Reps   int
}

var (
	dateRe   = regexp.MustCompile(`^(\d{1,2})\.(\d{1,2})(?:\.(\d{4}))?$`)
	intakeRe = regexp.MustCompile(`\(\s*(\d+)\s*/\s*(\d+)(?:\s[^)]*)?\)`)
	setRe    = regexp.MustCompile(`^(\d+(?:\.\d{1,2})?)?-(\d+)$`)
)

const defaultWeight = "0"

// Parse parses a whole message. posted supplies the year for a header date
// without one. Parsing is all-or-nothing: any problem returns a *Error.
func Parse(text string, posted time.Time) (Workout, error) {
	lines := strings.Split(text, "\n")
	var w Workout
	headerSeen := false
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if !headerSeen {
			headerSeen = true
			if err := parseHeader(&w, line, posted); err != nil {
				err.Line = i + 1
				err.Text = line
				return Workout{}, err
			}
			continue
		}
		entry, err := parseExercise(line)
		if err != nil {
			err.Line = i + 1
			err.Text = line
			return Workout{}, err
		}
		w.Entries = append(w.Entries, entry)
	}
	if !headerSeen {
		return Workout{}, &Error{Reason: ReasonEmpty}
	}
	return w, nil
}

// parseHeader fills date, type, intake and note. The intake block is cut out
// first, so the type is matched as a prefix of what remains after the date.
func parseHeader(w *Workout, line string, posted time.Time) *Error {
	fields := strings.Fields(line)
	date, ok := parseDate(fields[0], posted.Year())
	if !ok {
		return &Error{Token: fields[0], Reason: ReasonBadDate}
	}
	w.Date = date

	rest := strings.Join(fields[1:], " ")
	if m := intakeRe.FindStringSubmatchIndex(rest); m != nil {
		kcal, err1 := strconv.Atoi(rest[m[2]:m[3]])
		protein, err2 := strconv.Atoi(rest[m[4]:m[5]])
		if err1 != nil || err2 != nil {
			return &Error{Token: rest[m[0]:m[1]], Reason: ReasonBadIntake}
		}
		w.Kcal, w.Protein = &kcal, &protein
		rest = rest[:m[0]] + " " + rest[m[1]:]
		rest = strings.Join(strings.Fields(rest), " ")
	}

	w.Type, rest = cutType(rest)
	w.Note = strings.Join(strings.Fields(rest), " ")
	return nil
}

func parseDate(tok string, postedYear int) (time.Time, bool) {
	m := dateRe.FindStringSubmatch(tok)
	if m == nil {
		return time.Time{}, false
	}
	day, _ := strconv.Atoi(m[1])
	month, _ := strconv.Atoi(m[2])
	year := postedYear
	if m[3] != "" {
		year, _ = strconv.Atoi(m[3])
	}
	d := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if d.Year() != year || int(d.Month()) != month || d.Day() != day {
		return time.Time{}, false
	}
	return d, true
}

// cutType removes a workout type that prefixes s (case-insensitive, ending at
// a word boundary) and returns its canonical spelling and the remainder.
func cutType(s string) (string, string) {
	runes := []rune(s)
	for _, t := range workout.Types {
		n := len([]rune(t))
		if len(runes) < n || !strings.EqualFold(string(runes[:n]), t) {
			continue
		}
		if len(runes) > n && (unicode.IsLetter(runes[n]) || unicode.IsDigit(runes[n])) {
			continue
		}
		return t, string(runes[n:])
	}
	return "", s
}

// parseExercise splits a line into the name and set tokens.
func parseExercise(line string) (Entry, *Error) {
	fields := strings.Fields(line)
	first := -1
	for i, f := range fields {
		if setRe.MatchString(f) {
			first = i
			break
		}
	}
	if first < 0 {
		return Entry{}, &Error{Reason: ReasonNoSets}
	}
	if first == 0 {
		return Entry{}, &Error{Token: fields[0], Reason: ReasonNoName}
	}

	entry := Entry{Name: strings.Join(fields[:first], " ")}
	weight := defaultWeight
	for _, tok := range fields[first:] {
		m := setRe.FindStringSubmatch(tok)
		if m == nil {
			return Entry{}, &Error{Token: tok, Reason: ReasonBadToken}
		}
		reps, err := strconv.Atoi(m[2])
		if err != nil {
			return Entry{}, &Error{Token: tok, Reason: ReasonBadToken}
		}
		if reps == 0 {
			return Entry{}, &Error{Token: tok, Reason: ReasonZeroReps}
		}
		if m[1] != "" {
			weight = m[1]
		}
		entry.Sets = append(entry.Sets, Set{Weight: weight, Reps: reps})
	}
	return entry, nil
}
