// Package workout holds domain rules shared by the API and the bot.
package workout

import "strings"

// Types is the closed set of workout types (data_model.md). Order matters for
// prefix matching: a longer type must come before any type that is its prefix
// ("push+pull" before "push").
var Types = []string{
	"push+pull", "push+lower",
	"lower", "upper", "push", "pull", "full body",
	"workout A", "workout B", "день 1", "день 2", "руки", "рест",
}

// IsType reports whether t is one of Types, compared case-insensitively.
func IsType(t string) bool {
	for _, known := range Types {
		if strings.EqualFold(t, known) {
			return true
		}
	}
	return false
}

// NameKey normalizes an exercise name or alias exactly like the name_key and
// alias_key generated columns: trim, collapse whitespace runs to one space, lowercase.
func NameKey(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(name), " "))
}
