package bot

import (
	"html"
	"strconv"
	"strings"

	tele "gopkg.in/telebot.v4"

	"gymbro/internal/parser"
	"gymbro/internal/workout"
)

const dateLayout = "2006-01-02"

// preview is what the user sees for a draft: message parts (the markup goes on
// the last one) and the suspected-name list the buttons index into.
type preview struct {
	Parts    []string
	Markup   *tele.ReplyMarkup // nil when there are no buttons
	Suspects []suspectRef
}

// textPreview is a preview with a single text and no buttons.
func textPreview(text string) preview {
	return preview{Parts: splitMessage(text, maxMessageRunes)}
}

// renderParseError shows where and why parsing failed. No buttons.
func renderParseError(perr *parser.Error) preview {
	lines := []string{tr(txtParseFailed)}
	if perr.Line > 0 {
		lines = append(lines, tr(txtParseLine, perr.Line, perr.Text))
	}
	if perr.Token != "" {
		lines = append(lines, tr(txtParseToken, perr.Token))
	}
	lines = append(lines, parseReasonText(perr.Reason))
	return textPreview(strings.Join(lines, "\n"))
}

// renderWorkout shows a parsed workout with new-name markers, the suspected
// misspellings and their buttons, and Save once nothing is unresolved.
// msgID is the user's message ID, used in callback data.
func renderWorkout(w parser.Workout, names []nameInfo, kept map[string]bool, savedID int64, msgID int) preview {
	byKey := make(map[string]nameInfo, len(names))
	for _, n := range names {
		byKey[n.Key] = n
	}

	lines := []string{tr(txtHeadDate, w.Date.Format(dateLayout))}
	if w.Type != "" {
		lines = append(lines, tr(txtHeadType, w.Type))
	}
	if w.Kcal != nil && w.Protein != nil {
		lines = append(lines, tr(txtHeadIntake, strconv.Itoa(*w.Kcal), strconv.Itoa(*w.Protein)))
	}
	if w.Note != "" {
		lines = append(lines, tr(txtHeadNote, w.Note))
	}
	lines = append(lines, "")

	for _, e := range w.Entries {
		info := byKey[workout.NameKey(e.Name)]
		lines = append(lines, tr(txtEntry, e.Name, formatSets(e.Sets), marker(info, kept)))
	}

	var suspects []suspectRef
	var rows [][]tele.InlineButton
	var pending []string
	for _, n := range names {
		if !n.suspected() {
			continue
		}
		idx := len(suspects)
		suspects = append(suspects, suspectRef{Key: n.Key, Name: n.Name})
		if kept[n.Key] {
			continue
		}

		matchNames := make([]string, 0, len(n.Matches))
		for _, m := range n.Matches {
			matchNames = append(matchNames, html.EscapeString(m.Name))
			data := callbackData{Kind: cbPick, MsgID: msgID, NameIdx: idx, ExerciseID: m.ID}
			rows = append(rows, []tele.InlineButton{{Text: plain(txtBtnPick, n.Name, m.Name), Data: data.encode()}})
		}
		pending = append(pending, tr(txtSuspectItem, n.Name, rawHTML(strings.Join(matchNames, ", "))))
		keep := callbackData{Kind: cbKeep, MsgID: msgID, NameIdx: idx}
		rows = append(rows, []tele.InlineButton{{Text: plain(txtBtnKeep, n.Name), Data: keep.encode()}})
	}

	if len(pending) > 0 {
		lines = append(lines, tr(txtSuspectsHead))
		lines = append(lines, pending...)
		lines = append(lines, tr(txtNeedResolve))
	} else {
		save := callbackData{Kind: cbSave, MsgID: msgID}
		rows = append(rows, []tele.InlineButton{{Text: plain(txtBtnSave), Data: save.encode()}})
	}
	if savedID != 0 {
		lines = append(lines, tr(txtSavedLine, savedID))
	}

	p := preview{Parts: splitMessage(strings.Join(lines, "\n"), maxMessageRunes), Suspects: suspects}
	if len(rows) > 0 {
		p.Markup = &tele.ReplyMarkup{InlineKeyboard: rows}
	}
	return p
}

func marker(info nameInfo, kept map[string]bool) string {
	switch {
	case info.Known:
		return ""
	case info.suspected() && kept[info.Key]:
		return tr(txtMarkKept)
	case info.suspected():
		return tr(txtMarkSuspect)
	default:
		return tr(txtMarkNew)
	}
}

// formatSets renders sets as "60×10, 60×9".
func formatSets(sets []parser.Set) string {
	parts := make([]string, len(sets))
	for i, s := range sets {
		parts[i] = s.Weight + "×" + strconv.Itoa(s.Reps)
	}
	return strings.Join(parts, ", ")
}
