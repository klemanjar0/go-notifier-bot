package telegram

import (
	"context"
	"math/rand"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/klemanjar0/go-notifier-bot/internal/logger"
	"go.uber.org/zap"
)

func StartHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
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

var rejectTexts = [...]string{
	"к сожалению у меня пока нет полномочий делать такие дела.",
	"ну дела, я пока не знаю как это делать. попрошу начальство разобраться.",
	"я бы с радостью воплотил любые твои мечты, но этой пока надо подождать.",
	"ты классный человек, правда, и у нас точно бы все получилось, но не в этот раз, не в этом мире.",
	"тихо, не спеша, подожди, и я может когда-нибудь научусь это делать, но не сейчас.",
}

const postfix = "я начальству скажу, пусть что-то с этим сделают"

func AnythingHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log := logger.Named("telegram")
	text := rejectTexts[rand.Intn(4)] + " " + postfix

	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   text,
	}); err != nil {
		log.Error("send message failed", zap.Error(err))
		return
	}
}
