package bot

import "strings"

// maxMessageRunes is Telegram's 4096 limit with a margin: Telegram counts
// UTF-16 units, this package counts runes.
const maxMessageRunes = 4000

// splitMessage cuts text into chunks of at most limit runes, breaking at line
// boundaries. A single line longer than limit is cut at a rune boundary, which
// can split an HTML entity, so callers keep individual lines short.
func splitMessage(text string, limit int) []string {
	var parts []string
	var cur strings.Builder
	curRunes := 0

	flush := func() {
		if curRunes > 0 {
			parts = append(parts, cur.String())
			cur.Reset()
			curRunes = 0
		}
	}

	for _, line := range strings.Split(text, "\n") {
		runes := []rune(line)
		// A long line is cut into limit-sized pieces, each its own chunk.
		for len(runes) > limit {
			flush()
			parts = append(parts, string(runes[:limit]))
			runes = runes[limit:]
		}

		need := len(runes)
		if curRunes > 0 {
			need++ // the newline joining it to the previous line
		}
		if curRunes+need > limit {
			flush()
			need = len(runes)
		}
		if curRunes > 0 {
			cur.WriteByte('\n')
		}
		cur.WriteString(string(runes))
		curRunes += need
	}
	flush()

	if len(parts) == 0 {
		return []string{""}
	}
	return parts
}
