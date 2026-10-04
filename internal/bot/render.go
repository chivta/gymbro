package bot

import (
	"html"
	"strings"

	tele "gopkg.in/telebot.v4"

	"gymbro/internal/parser"
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

// renderParseError shows the first line that failed and why, in one line. No buttons.
func renderParseError(perr *parser.Error) preview {
	reason := rawHTML(parseReasonText(perr.Reason))
	if perr.Line == 0 {
		return textPreview(tr(txtParseFailed, reason))
	}
	return textPreview(tr(txtParseFailedLine, perr.Line, perr.Text, reason))
}

// renderWorkout is a short status of a parsed workout: which names are new, the
// suspected misspellings with their buttons, and Save once nothing is unresolved
// and no set is planned. The workout itself is not echoed back.
// msgID is the user's message ID, used in callback data.
func renderWorkout(w parser.Workout, names []nameInfo, kept map[string]bool, savedID int64, msgID int) preview {
	var lines []string
	var plainNew []string
	for _, n := range names {
		if !n.Known && (!n.suspected() || kept[n.Key]) {
			plainNew = append(plainNew, html.EscapeString(n.Name))
		}
	}
	if len(plainNew) > 0 {
		lines = append(lines, tr(txtNewNames, rawHTML(strings.Join(plainNew, ", "))))
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

	if w.Planned() {
		lines = append(lines, tr(txtPlannedUnsavable))
	}
	if len(pending) > 0 {
		lines = append(lines, tr(txtSuspectsHead))
		lines = append(lines, pending...)
		lines = append(lines, tr(txtNeedResolve))
	} else if !w.Planned() {
		save := callbackData{Kind: cbSave, MsgID: msgID}
		rows = append(rows, []tele.InlineButton{{Text: plain(txtBtnSave), Data: save.encode()}})
	}
	if savedID != 0 {
		lines = append(lines, tr(txtSavedLine, savedID))
	}
	if len(lines) == 0 {
		lines = append(lines, tr(txtReady))
	}

	p := preview{Parts: splitMessage(strings.Join(lines, "\n"), maxMessageRunes), Suspects: suspects}
	if len(rows) > 0 {
		p.Markup = &tele.ReplyMarkup{InlineKeyboard: rows}
	}
	return p
}
