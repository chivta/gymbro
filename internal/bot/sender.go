package bot

import (
	"context"
	"strconv"

	"golang.org/x/time/rate"
	tele "gopkg.in/telebot.v4"
)

// Telegram allows about 30 calls per second across the bot; stay below it.
const (
	outboundRate  = 25
	outboundBurst = 5
)

// sender is the only way the bot calls Telegram: every outbound call waits on
// one shared limiter.
type sender struct {
	tg  *tele.Bot
	lim *rate.Limiter
}

func newSender(tg *tele.Bot) *sender {
	return &sender{tg: tg, lim: rate.NewLimiter(outboundRate, outboundBurst)}
}

// htmlOpts are the send options for an HTML message with optional buttons.
func htmlOpts(markup *tele.ReplyMarkup) *tele.SendOptions {
	return &tele.SendOptions{ParseMode: tele.ModeHTML, ReplyMarkup: markup}
}

// send posts text to a chat; replyTo may be nil.
func (s *sender) send(ctx context.Context, chatID int64, text string, markup *tele.ReplyMarkup, replyTo *tele.Message) (*tele.Message, error) {
	err := s.lim.Wait(ctx)
	if err != nil {
		return nil, err
	}
	opts := htmlOpts(markup)
	if replyTo != nil {
		opts.ReplyTo = replyTo
		opts.AllowWithoutReply = true
	}
	return s.tg.Send(tele.ChatID(chatID), text, opts)
}

// edit replaces a sent message's text; no markup removes any buttons.
func (s *sender) edit(ctx context.Context, chatID int64, msgID int, text string, markup *tele.ReplyMarkup) error {
	err := s.lim.Wait(ctx)
	if err != nil {
		return err
	}
	_, err = s.tg.Edit(stored(chatID, msgID), text, htmlOpts(markup))
	return err
}

func (s *sender) remove(ctx context.Context, chatID int64, msgID int) error {
	err := s.lim.Wait(ctx)
	if err != nil {
		return err
	}
	return s.tg.Delete(stored(chatID, msgID))
}

// answer responds to a callback query; empty text just dismisses the spinner.
func (s *sender) answer(ctx context.Context, cb *tele.Callback, text string, alert bool) error {
	err := s.lim.Wait(ctx)
	if err != nil {
		return err
	}
	return s.tg.Respond(cb, &tele.CallbackResponse{Text: text, ShowAlert: alert})
}

func stored(chatID int64, msgID int) tele.StoredMessage {
	return tele.StoredMessage{MessageID: strconv.Itoa(msgID), ChatID: chatID}
}
