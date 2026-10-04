package bot

import "strings"

// replaceSeparator splits "old name => new name" in /replace_exercise.
const replaceSeparator = " => "

// parseReplaceArgs splits the /replace_exercise payload on exactly one " => ".
// Names may contain spaces. ok is false when the separator is missing or
// repeated, or either name is empty.
func parseReplaceArgs(payload string) (bad, correct string, ok bool) {
	parts := strings.Split(strings.TrimSpace(payload), replaceSeparator)
	if len(parts) != 2 {
		return "", "", false
	}
	bad = strings.TrimSpace(parts[0])
	correct = strings.TrimSpace(parts[1])
	if bad == "" || correct == "" {
		return "", "", false
	}
	return bad, correct, true
}
