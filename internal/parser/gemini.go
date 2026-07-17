package parser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/klemanjar0/go-notifier-bot/internal/logger"
	"github.com/klemanjar0/go-notifier-bot/internal/telegram"
	"go.uber.org/zap"
	"google.golang.org/genai"
)

type GeminiParser struct {
	Location *time.Location
	Client   *genai.Client
}

func NewGeminiParser(client *genai.Client) (*GeminiParser, error) {
	loc, err := time.LoadLocation("Europe/Kyiv")

	if err != nil {
		fmt.Println("Error loading location:", err)
		return nil, err
	}

	return &GeminiParser{
		Location: loc,
		Client:   client,
	}, err
}

func (p *GeminiParser) Parse(ctx context.Context, msg string) (*telegram.ParserResult, error) {
	log := logger.Named("gemini")
	var prompt = fmt.Sprintf(`Ты — умный парсер напоминаний. На вход приходит живое сообщение пользователя на русском в свободной форме.
Твоя задача — понять, О ЧЁМ напомнить и КОГДА.

Текущее время: %s (таймзона Europe/Kyiv).
Все относительные выражения считай от этого момента.

Пользователь может писать как угодно, например:
- "напомни мне позвонить маме через час"
- "я хочу сходить в зал завтра в 7 вечера"
- "не забыть оплатить интернет в пятницу"
- "надо забрать посылку послезавтра утром"
- "купить кофе через 20 минут"
- "созвон с командой в понедельник в 10"

Верни JSON:
{
  "ok": bool,        // удалось ли понять время
  "text": string,    // суть напоминания, коротко и в форме действия
  "fire_at": string  // RFC3339 с киевским оффсетом, напр "2026-07-17T15:00:00+03:00"; "" если ok=false
}

Как чистить text:
- Убирай вводные: "напомни мне", "напомни", "я хочу", "мне надо", "надо", "не забыть", "не забудь".
- Убирай всю временную часть ("завтра", "через час", "в 15:00").
- Приводи к короткому действию: "я хочу сходить в зал завтра" -> "сходить в зал".
- Сохраняй важные детали (имена, места, суммы): "встреча с Надей", "забрать посылку на почте".

Как считать fire_at:
- Нет времени в сообщении -> ok=false, text="", fire_at="".
- Относительное ("через N минут/часов/дней/недель") -> прибавляй к текущему времени.
- "завтра" = следующий день, "послезавтра" = +2 дня.
- День недели без "следующий" -> ближайший будущий такой день (если сегодня он уже прошёл — берёшь через неделю).
- Указано только время суток и оно уже прошло сегодня -> переноси на завтра.

Дефолтные часы, если время суток размытое:
- "утром" -> 09:00
- "днём" / "в обед" -> 13:00
- "вечером" -> 19:00
- "ночью" -> 23:00
- просто день без времени ("в пятницу", "завтра") -> 09:00
- "на выходных" -> ближайшая суббота 12:00

Важно:
- Ничего не выдумывай сверх сказанного.
- Если время указано противоречиво или совсем непонятно -> ok=false.

Примеры:
"напомни позвонить маме через 2 часа" -> {"ok":true,"text":"позвонить маме","fire_at":"..."}
"я хочу сходить в зал завтра вечером" -> {"ok":true,"text":"сходить в зал","fire_at":"...T19:00:00+03:00"}
"не забыть оплатить интернет в пятницу" -> {"ok":true,"text":"оплатить интернет","fire_at":"...T09:00:00+03:00"}
"надо забрать посылку послезавтра утром" -> {"ok":true,"text":"забрать посылку","fire_at":"...T09:00:00+03:00"}
"созвон с Надей в понедельник в 10" -> {"ok":true,"text":"созвон с Надей","fire_at":"...T10:00:00+03:00"}
"как дела" -> {"ok":false,"text":"","fire_at":""}
"хочу купить машину" -> {"ok":false,"text":"","fire_at":""}

Сообщение: %q`,
		time.Now().In(p.Location).Format(time.RFC3339),
		msg,
	)

	schema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"ok":      {Type: genai.TypeBoolean},
			"text":    {Type: genai.TypeString},
			"fire_at": {Type: genai.TypeString},
		},
		Required: []string{"ok", "text", "fire_at"},
	}

	config := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema:   schema,
		Temperature:      genai.Ptr[float32](0),
	}

	resp, err := p.Client.Models.GenerateContent(ctx, "gemini-3.1-flash-lite",
		genai.Text(prompt), config)
	if err != nil {
		return nil, err
	}

	var out telegram.ParserResult
	if err := json.Unmarshal([]byte(resp.Text()), &out); err != nil {
		return nil, err
	}

	fireAt, err := time.Parse(time.RFC3339, out.FireAt)
	if err != nil {
		log.Debug("fireAt", zap.String("fireAt", fireAt.GoString()))
		return nil, errors.Join(err, errors.New("не понял( плак плак"))
	}

	return &out, nil
}
