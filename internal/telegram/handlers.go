package telegram

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/klemanjar0/go-notifier-bot/internal/db"
	"github.com/klemanjar0/go-notifier-bot/internal/logger"
	"go.uber.org/zap"
)

// cancelPrefix marks inline-button callback data for the /list cancel buttons.
// The reminder id is appended, e.g. "cancel:42".
const cancelPrefix = "cancel:"

type ParserResult struct {
	OK     bool   `json:"ok"`
	Text   string `json:"text"`
	FireAt string `json:"fire_at"`
}

type Parser interface {
	Parse(ctx context.Context, msg string) (result *ParserResult, err error)
}

type Reminders interface {
	Create(ctx context.Context, chatID int64, text string, fireAt time.Time) (int64, error)
	ListPending(ctx context.Context, chatID int64) ([]db.Reminder, error)
	Cancel(ctx context.Context, chatID, id int64) error
	MarkAllSentForChat(ctx context.Context, chatID int64) error
}

type Handlers struct {
	parser    Parser
	reminders Reminders
}

func NewHandlers(parser Parser, reminders Reminders) *Handlers {
	return &Handlers{
		parser:    parser,
		reminders: reminders,
	}
}

func (h *Handlers) StartHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log := logger.Named("telegram")
	text := `
дарова голова.
я бот который напомнит тебе сделать всё то, что твоя голова не удержала.
пиши мне что-то типа напомни развесить белье через час, а я напишу как часики оттикают.
	`

	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   text,
	}); err != nil {
		log.Error("send message failed", zap.Error(err))
		return
	}
}

func (h *Handlers) ClearAllHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log := logger.Named("telegram")
	var text = "сказано - сделано, все напоминания ушли в небытие."

	err := h.reminders.MarkAllSentForChat(ctx, update.Message.Chat.ID)

	if err != nil {
		text = "на этом наши полномочия всё, не получилось. попробуй позже еще раз."
	}

	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   text,
	}); err != nil {
		log.Error("send message failed", zap.Error(err))
		return
	}
}

func (h *Handlers) ListHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log := logger.Named("telegram")
	chatID := update.Message.Chat.ID

	items, err := h.reminders.ListPending(ctx, chatID)
	if err != nil {
		log.Error("list pending failed", zap.Int64("chat_id", chatID), zap.Error(err))
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "не смог поднять список, что-то заскрипело. попробуй позже.",
		})
		return
	}

	if len(items) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "пусто. ни одной напоминалки на тебе не висит.",
		})
		return
	}

	text, markup := renderList(items)
	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: markup,
	}); err != nil {
		log.Error("send message failed", zap.Error(err))
	}
}

// CancelCallbackHandler handles the "❌" buttons rendered by /list. It marks the
// selected reminder as sent and refreshes the list message in place.
func (h *Handlers) CancelCallbackHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log := logger.Named("telegram")
	cb := update.CallbackQuery

	idStr := strings.TrimPrefix(cb.Data, cancelPrefix)
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		log.Error("parse cancel id failed", zap.String("data", cb.Data), zap.Error(err))
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: cb.ID})
		return
	}

	// The originating message carries the chat we're allowed to touch; scoping the
	// cancel by chat_id keeps one chat from cancelling another's reminders.
	msg := cb.Message.Message
	if msg == nil {
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: cb.ID})
		return
	}
	chatID := msg.Chat.ID

	if err := h.reminders.Cancel(ctx, chatID, id); err != nil {
		log.Error("cancel reminder failed", zap.Int64("id", id), zap.Int64("chat_id", chatID), zap.Error(err))
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: cb.ID,
			Text:            "не вышло отменить, попробуй ещё раз",
		})
		return
	}

	b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: cb.ID,
		Text:            "отменил ✅",
	})

	// Re-render the list so the cancelled item drops off the message.
	items, err := h.reminders.ListPending(ctx, chatID)
	if err != nil {
		log.Error("list pending failed", zap.Int64("chat_id", chatID), zap.Error(err))
		return
	}

	if len(items) == 0 {
		b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: msg.ID,
			Text:      "всё, список пуст. чистота.",
		})
		return
	}

	text, markup := renderList(items)
	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   msg.ID,
		Text:        text,
		ReplyMarkup: markup,
	})
}

// renderList builds the /list message text and the per-item cancel keyboard.
func renderList(items []db.Reminder) (string, models.InlineKeyboardMarkup) {
	var sb strings.Builder
	sb.WriteString("вот что я тебе обещал напомнить:\n\n")

	rows := make([][]models.InlineKeyboardButton, 0, len(items))
	for i, r := range items {
		fmt.Fprintf(&sb, "%d. %s — %s\n", i+1, r.Text, r.FireAt.Time.Format("02.01 15:04"))
		rows = append(rows, []models.InlineKeyboardButton{{
			Text:         fmt.Sprintf("❌ %d. %s", i+1, truncate(r.Text, 40)),
			CallbackData: cancelPrefix + strconv.FormatInt(r.ID, 10),
		}})
	}

	return sb.String(), models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// truncate keeps inline-button labels within Telegram's limits, cutting on runes
// so multi-byte text isn't split mid-character.
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

func (h *Handlers) AnythingHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log := logger.Named("telegram")

	var rejectTexts = [...]string{
		"к сожалению у меня пока нет полномочий делать такие дела.",
		"ну дела, я пока не знаю как это делать.",
		"я бы с радостью воплотил любые твои мечты, но этой пока надо подождать.",
		"ты классный человек, правда, и у нас точно бы все получилось, но не в этот раз, не в этом мире.",
		"тихо, не спеша, подожди, и я может когда-нибудь научусь это делать, но не сейчас.",
		"а в рот шампанским не пописять? я пока такое не умею.",
	}

	const postfix = "я начальству скажу, пусть что-то с этим сделают"

	text := rejectTexts[rand.Intn(len(rejectTexts))] + " " + postfix

	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   text,
	}); err != nil {
		log.Error("send message failed", zap.Error(err))
		return
	}
}

func (h *Handlers) OnMessage(ctx context.Context, b *bot.Bot, update *models.Update) {
	log := logger.Named("OnMessage")

	if update.Message == nil {
		return
	}

	msg := update.Message.Text
	log.Debug("message input", zap.String("msg", msg))

	res, err := h.parser.Parse(ctx, msg) // gemini
	if err != nil || !res.OK {
		log.Debug("message gemini parse failed", zap.Error(err))

		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "не товарищ, так не пойдет, ты или формируешь РЕАЛЬНОЕ задание, или гуляй лесом",
		})
		return
	}

	log.Debug("message input parsed", zap.String("res", res.Text))

	fireAt, err := time.Parse(time.RFC3339, res.FireAt)
	if err != nil {
		log.Error("parse fire_at failed", zap.String("fire_at", res.FireAt), zap.Error(err))
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "что-то я запутался во времени, попробуй сказать ещё раз",
		})
		return
	}

	chatID := update.Message.Chat.ID
	id, err := h.reminders.Create(ctx, chatID, res.Text, fireAt)
	if err != nil {
		log.Error("create reminder failed", zap.Int64("chat_id", chatID), zap.Error(err))
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "ой, не смог записать напоминалку, давай попробуем ещё раз попозже",
		})
		return
	}

	log.Debug("reminder created", zap.Int64("reminder_id", id), zap.Time("fire_at", fireAt))

	var successMessages = [...]string{
		"так точно, есть напомнить",
		"слушаюсь, босс,",
		"ну у тебя конечно и большие планы -",
		"дела ладить не х#й гладить, а тебе надо",
		"постараюсь не забыть, и тебе напомнить",
	}

	text := successMessages[rand.Intn(len(successMessages))]

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   fmt.Sprintf("%s %s в %s", text, res.Text, fireAt.Format("02.01 15:04")),
	})
}
