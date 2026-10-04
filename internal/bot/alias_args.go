package bot

import "strings"

// aliasSeparator splits "alias => existing exercise" in /alias_exercise.
const aliasSeparator = " => "

// parseAliasArgs splits the /alias_exercise payload on exactly one " => ".
// Names may contain spaces. ok is false when the separator is missing or
// repeated, or either name is empty.
func parseAliasArgs(payload string) (alias, exercise string, ok bool) {
	parts := strings.Split(strings.TrimSpace(payload), aliasSeparator)
	if len(parts) != 2 {
		return "", "", false
	}
	alias = strings.TrimSpace(parts[0])
	exercise = strings.TrimSpace(parts[1])
	if alias == "" || exercise == "" {
		return "", "", false
	}
	return alias, exercise, true
}
