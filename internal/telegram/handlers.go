package telegram

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/klemanjar0/go-notifier-bot/internal/logger"
	"go.uber.org/zap"
)

type ParserResult struct {
	OK     bool   `json:"ok"`
	Text   string `json:"text"`
	FireAt string `json:"fire_at"`
}

type Parser interface {
	Parse(ctx context.Context, msg string) (result *ParserResult, err error)
}

// Reminders is the reminder service as the handlers need it: persist a parsed
// reminder for a chat. Declared here so the telegram package does not depend on
// the reminder package's concrete type.
type Reminders interface {
	Create(ctx context.Context, chatID int64, text string, fireAt time.Time) (int64, error)
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

func (h *Handlers) AnythingHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log := logger.Named("telegram")

	var rejectTexts = [...]string{
		"к сожалению у меня пока нет полномочий делать такие дела.",
		"ну дела, я пока не знаю как это делать. попрошу начальство разобраться.",
		"я бы с радостью воплотил любые твои мечты, но этой пока надо подождать.",
		"ты классный человек, правда, и у нас точно бы все получилось, но не в этот раз, не в этом мире.",
		"тихо, не спеша, подожди, и я может когда-нибудь научусь это делать, но не сейчас.",
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
