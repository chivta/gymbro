package bot

import (
	"fmt"
	"strconv"
	"strings"
)

// Telegram caps callback data at 64 bytes and names are Cyrillic, so buttons
// carry short numeric references only:
//
//	p:<msgID>:<nameIdx>:<exerciseID>   pick an existing exercise for a suspected name
//	k:<msgID>:<nameIdx>                keep a suspected name as new
//	s:<msgID>                          save the draft
//
// msgID is the user's message ID (the chat comes from the callback's message),
// nameIdx indexes the draft's suspected names as last rendered, exerciseID is
// the API exercise ID.
const (
	cbPick = "p"
	cbKeep = "k"
	cbSave = "s"

	cbSep = ":"
)

// callbackData is a decoded button reference. Fields not used by Kind are zero.
type callbackData struct {
	Kind       string
	MsgID      int
	NameIdx    int
	ExerciseID int64
}

func (d callbackData) encode() string {
	switch d.Kind {
	case cbPick:
		return fmt.Sprintf("%s:%d:%d:%d", cbPick, d.MsgID, d.NameIdx, d.ExerciseID)
	case cbKeep:
		return fmt.Sprintf("%s:%d:%d", cbKeep, d.MsgID, d.NameIdx)
	default:
		return fmt.Sprintf("%s:%d", cbSave, d.MsgID)
	}
}

// decodeCallback parses data produced by encode; ok is false for anything else.
func decodeCallback(data string) (callbackData, bool) {
	parts := strings.Split(data, cbSep)
	var want int
	switch parts[0] {
	case cbPick:
		want = 4
	case cbKeep:
		want = 3
	case cbSave:
		want = 2
	default:
		return callbackData{}, false
	}
	if len(parts) != want {
		return callbackData{}, false
	}

	d := callbackData{Kind: parts[0]}
	msgID, err := strconv.Atoi(parts[1])
	if err != nil || msgID < 0 {
		return callbackData{}, false
	}
	d.MsgID = msgID

	if want >= 3 {
		idx, err := strconv.Atoi(parts[2])
		if err != nil || idx < 0 {
			return callbackData{}, false
		}
		d.NameIdx = idx
	}
	if want == 4 {
		id, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil || id < 0 {
			return callbackData{}, false
		}
		d.ExerciseID = id
	}
	return d, true
}
