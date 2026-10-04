package bot

import (
	"html"
	"sort"
	"strings"

	tele "gopkg.in/telebot.v4"

	"gymbro/internal/apiclient"
	"gymbro/internal/parser"
	"gymbro/internal/workout"
)

func (b *Bot) onStart(c tele.Context) error {
	_, err := b.out.send(b.ctx, c.Chat().ID, tr(txtStart), nil, nil)
	return err
}

func (b *Bot) onHelp(c tele.Context) error {
	_, err := b.out.send(b.ctx, c.Chat().ID, tr(txtHelp), nil, nil)
	return err
}

// onText previews a new workout message. Unknown commands are ignored.
func (b *Bot) onText(c tele.Context) error {
	msg := c.Message()
	if strings.HasPrefix(msg.Text, "/") {
		return nil
	}
	return b.startDraft(msg)
}

// onEdited updates the preview of an edited message in place. The message's
// Date is the original post date, not the edit time. A message that is not in
// the cache (e.g. after a restart) gets a fresh preview and a new draft.
func (b *Bot) onEdited(c tele.Context) error {
	msg := c.Message()
	if msg.Text == "" || strings.HasPrefix(msg.Text, "/") {
		return nil
	}
	d, ok := b.drafts.get(draftKey{ChatID: msg.Chat.ID, MsgID: msg.ID})
	if !ok {
		return b.startDraft(msg)
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.Text = msg.Text
	d.Posted = msg.Time().In(postZone)
	d.SavedWorkoutID = 0 // the saved workout no longer matches the text
	return b.refreshPreview(msg, d)
}

// onExercises lists exercise names with aliases, sorted.
func (b *Bot) onExercises(c tele.Context) error {
	chatID := c.Chat().ID
	exercises, err := b.listExercises()
	if err != nil {
		logAPIError("list_exercises", err)
		_, err = b.out.send(b.ctx, chatID, apiErrorText(err), nil, nil)
		return err
	}
	if len(exercises) == 0 {
		_, err = b.out.send(b.ctx, chatID, tr(txtExercisesEmpty), nil, nil)
		return err
	}

	sort.Slice(exercises, func(i, j int) bool {
		return workout.NameKey(exercises[i].Name) < workout.NameKey(exercises[j].Name)
	})
	lines := make([]string, len(exercises))
	for i, ex := range exercises {
		if len(ex.Aliases) == 0 {
			lines[i] = html.EscapeString(ex.Name)
			continue
		}
		aliases := append([]string(nil), ex.Aliases...)
		sort.Strings(aliases)
		lines[i] = tr(txtExerciseAlias, ex.Name, strings.Join(aliases, ", "))
	}
	return b.sendLong(chatID, strings.Join(lines, "\n"))
}

// onAliasExercise handles "/alias_exercise alias => existing exercise". On
// success it re-renders the chat's most recent draft, since names may now
// resolve through the new alias.
func (b *Bot) onAliasExercise(c tele.Context) error {
	chat := c.Chat()
	alias, exercise, ok := parseAliasArgs(c.Message().Payload)
	if !ok {
		_, err := b.out.send(b.ctx, chat.ID, tr(txtAliasUsage), nil, nil)
		return err
	}

	report, done := b.alias(alias, exercise)
	_, err := b.out.send(b.ctx, chat.ID, report, nil, nil)
	if err != nil || !done {
		return err
	}
	return b.refreshLatestDraft(chat)
}

// refreshLatestDraft re-renders the preview of the chat's most recently touched
// draft; no draft means nothing to do. A saved draft needs no more than the
// re-render: the API merge already repointed its entries.
func (b *Bot) refreshLatestDraft(chat *tele.Chat) error {
	key, d, ok := b.drafts.latest(chat.ID)
	if !ok {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return b.refreshPreview(&tele.Message{ID: key.MsgID, Chat: chat}, d)
}

// alias calls ReplaceExercise and returns the user-facing report; done is true
// when the alias was recorded.
func (b *Bot) alias(alias, exercise string) (report string, done bool) {
	userID, err := b.resolveUser()
	if err != nil {
		logAPIError("resolve_identity", err)
		return apiErrorText(err), false
	}
	ctx, cancel := b.apiCtx()
	defer cancel()
	resp, err := b.api.ReplaceExercise(ctx, userID, apiclient.ReplaceExerciseRequest{BadName: alias, CorrectName: exercise})
	if err != nil {
		apiErr, isAPI := asAPIError(err)
		if isAPI && apiErr.Code == apiclient.CodeExerciseNotFound {
			return tr(txtAliasNotFound, exercise), false
		}
		logAPIError("alias_exercise", err)
		return apiErrorText(err), false
	}

	if resp.Outcome == apiclient.OutcomeMerged {
		return tr(txtAliasMerged, alias, resp.Exercise.Name, alias), true
	}
	return tr(txtAliasAliasOnly, alias, resp.Exercise.Name), true
}

// onCallback handles the Save / pick / keep buttons (see callback.go).
func (b *Bot) onCallback(c tele.Context) error {
	cb := c.Callback()
	data, ok := decodeCallback(cb.Data)
	if !ok {
		return b.out.answer(b.ctx, cb, "", false)
	}

	chatID := c.Chat().ID
	d, ok := b.drafts.get(draftKey{ChatID: chatID, MsgID: data.MsgID})
	if !ok {
		return b.out.answer(b.ctx, cb, plain(txtCbDraftMissing), true)
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	userMsg := &tele.Message{ID: data.MsgID, Chat: c.Chat()}
	switch data.Kind {
	case cbKeep:
		return b.onKeep(cb, userMsg, d, data)
	case cbPick:
		return b.onPick(cb, userMsg, d, data)
	default:
		return b.onSave(cb, userMsg, d)
	}
}

// staleReply tells the user the buttons no longer matched and redraws them.
func (b *Bot) staleReply(cb *tele.Callback, userMsg *tele.Message, d *draft, key string) error {
	err := b.out.answer(b.ctx, cb, plain(key), false)
	if err != nil {
		return err
	}
	return b.refreshPreview(userMsg, d)
}

func (b *Bot) onKeep(cb *tele.Callback, userMsg *tele.Message, d *draft, data callbackData) error {
	if data.NameIdx >= len(d.Suspects) {
		return b.staleReply(cb, userMsg, d, txtCbStale)
	}
	d.Kept[d.Suspects[data.NameIdx].Key] = true
	err := b.out.answer(b.ctx, cb, "", false)
	if err != nil {
		return err
	}
	return b.refreshPreview(userMsg, d)
}

func (b *Bot) onPick(cb *tele.Callback, userMsg *tele.Message, d *draft, data callbackData) error {
	if data.NameIdx >= len(d.Suspects) {
		return b.staleReply(cb, userMsg, d, txtCbStale)
	}
	suspect := d.Suspects[data.NameIdx]

	exercises, err := b.listExercises()
	if err != nil {
		logAPIError("list_exercises", err)
		return b.out.answer(b.ctx, cb, apiErrorPlain(err), true)
	}
	var target *apiclient.Exercise
	for i := range exercises {
		if exercises[i].ID == data.ExerciseID {
			target = &exercises[i]
		}
	}
	if target == nil {
		return b.staleReply(cb, userMsg, d, txtCbStale)
	}

	userID, err := b.resolveUser()
	if err != nil {
		logAPIError("resolve_identity", err)
		return b.out.answer(b.ctx, cb, apiErrorPlain(err), true)
	}
	ctx, cancel := b.apiCtx()
	defer cancel()
	_, err = b.api.ReplaceExercise(ctx, userID, apiclient.ReplaceExerciseRequest{BadName: suspect.Name, CorrectName: target.Name})
	if err != nil {
		apiErr, isAPI := asAPIError(err)
		// Already resolved by someone else: just redraw.
		if isAPI && (apiErr.Code == apiclient.CodeBadNameIsAlias || apiErr.Code == apiclient.CodeSameExercise ||
			apiErr.Code == apiclient.CodeExerciseNotFound) {
			return b.staleReply(cb, userMsg, d, txtCbDone)
		}
		logAPIError("alias_exercise", err)
		return b.out.answer(b.ctx, cb, apiErrorPlain(err), true)
	}

	err = b.out.answer(b.ctx, cb, "", false)
	if err != nil {
		return err
	}
	return b.refreshPreview(userMsg, d)
}

// onSave re-parses the cached text (refusing planned sets) and re-checks the names against the API:
// nothing from render time is trusted. On success the preview keeps its
// content and Save button and gains a "saved" line (an edit clears the line,
// and Save can be pressed again: same source_ref, so the API updates the workout).
func (b *Bot) onSave(cb *tele.Callback, userMsg *tele.Message, d *draft) error {
	w, err := parser.Parse(d.Text, d.Posted)
	if err != nil {
		return b.staleReply(cb, userMsg, d, txtCbCannotSave)
	}
	if w.Planned() {
		return b.staleReply(cb, userMsg, d, txtCbPlanned)
	}
	names, err := b.loadNames(w)
	if err != nil {
		logAPIError("list_exercises", err)
		return b.out.answer(b.ctx, cb, apiErrorPlain(err), true)
	}
	if unresolved(names, d.Kept) {
		return b.staleReply(cb, userMsg, d, txtCbCannotSave)
	}

	userID, err := b.resolveUser()
	if err != nil {
		logAPIError("resolve_identity", err)
		return b.out.answer(b.ctx, cb, apiErrorPlain(err), true)
	}
	ctx, cancel := b.apiCtx()
	defer cancel()
	resp, err := b.api.SaveWorkout(ctx, userID, saveRequest(w, d.Text, userMsg))
	if err != nil {
		logAPIError("save_workout", err)
		return b.out.answer(b.ctx, cb, apiErrorPlain(err), true)
	}

	d.SavedWorkoutID = resp.WorkoutID
	err = b.out.answer(b.ctx, cb, plain(txtCbSaved), false)
	if err != nil {
		return err
	}
	err = b.refreshPreview(userMsg, d)
	if err != nil {
		return err
	}
	if len(resp.NewExercises) == 0 {
		return nil
	}
	lines := []string{tr(txtNewExercisesHead)}
	for _, name := range resp.NewExercises {
		lines = append(lines, tr(txtNewExerciseItem, name, name))
	}
	return b.sendLong(userMsg.Chat.ID, strings.Join(lines, "\n"))
}

// saveRequest builds the API request from a parse. source_ref is "<chat id>:<message id>".
func saveRequest(w parser.Workout, rawText string, userMsg *tele.Message) apiclient.SaveWorkoutRequest {
	req := apiclient.SaveWorkoutRequest{
		PerformedOn: w.Date.Format(dateLayout),
		Type:        w.Type,
		Kcal:        w.Kcal,
		ProteinG:    w.Protein,
		Note:        w.Note,
		RawText:     rawText,
		Source:      sourceTelegramBot,
		SourceRef:   sourceRef(userMsg.Chat.ID, userMsg.ID),
		Entries:     make([]apiclient.Entry, len(w.Entries)),
	}
	for i, e := range w.Entries {
		sets := make([]apiclient.Set, len(e.Sets))
		for j, s := range e.Sets {
			sets[j] = apiclient.Set{Weight: s.Weight, Reps: s.Reps}
		}
		req.Entries[i] = apiclient.Entry{Name: e.Name, Sets: sets}
	}
	return req
}
