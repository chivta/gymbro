package bot

import (
	"gymbro/internal/apiclient"
	"gymbro/internal/parser"
	"gymbro/internal/similarity"
	"gymbro/internal/workout"
)

// maxSuggestions is how many similar existing exercises are offered per suspected name.
const maxSuggestions = 3

// nameInfo describes one distinct exercise name (by key) of a parsed workout.
type nameInfo struct {
	Name string // first spelling as written
	Key  string
	// Known: the key matches an exercise key or an alias key of the user.
	Known bool
	// Matches are similar existing exercises. Non-empty only for an unknown
	// name, which makes it a suspected misspelling; empty means a plain new name.
	Matches []apiclient.Exercise
}

func (n nameInfo) suspected() bool { return !n.Known && len(n.Matches) > 0 }

// analyzeNames classifies every distinct name in entries, in order of first
// appearance, against the user's exercises and aliases.
func analyzeNames(entries []parser.Entry, exercises []apiclient.Exercise) []nameInfo {
	known := make(map[string]bool, len(exercises))
	byName := make(map[string]apiclient.Exercise, len(exercises))
	displayNames := make([]string, 0, len(exercises))
	for _, ex := range exercises {
		known[workout.NameKey(ex.Name)] = true
		for _, alias := range ex.Aliases {
			known[workout.NameKey(alias)] = true
		}
		byName[ex.Name] = ex
		displayNames = append(displayNames, ex.Name)
	}

	seen := make(map[string]bool, len(entries))
	var out []nameInfo
	for _, e := range entries {
		key := workout.NameKey(e.Name)
		if seen[key] {
			continue
		}
		seen[key] = true

		info := nameInfo{Name: e.Name, Key: key, Known: known[key]}
		if !info.Known {
			for _, m := range similarity.Rank(e.Name, displayNames, maxSuggestions) {
				info.Matches = append(info.Matches, byName[m.Name])
			}
		}
		out = append(out, info)
	}
	return out
}

// unresolved reports whether any suspected misspelling is neither kept as new
// (kept holds name keys) nor known.
func unresolved(names []nameInfo, kept map[string]bool) bool {
	for _, n := range names {
		if n.suspected() && !kept[n.Key] {
			return true
		}
	}
	return false
}
