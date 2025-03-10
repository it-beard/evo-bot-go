package privatehandlers

import (
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/handlers"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type StartHandler struct{}

func NewStartHandler() handlers.Handler {
	return &StartHandler{}
}

func (h *StartHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	message := "Привет! Я *Дженкинс Вебстер*, дворецкий бот клуба _\"Эволюция Кода\"_.\n" +
		"Используй /help для просмотра возможностей."
	opts := &gotgbot.SendMessageOpts{ParseMode: "markdown"}
	_, err := b.SendMessage(ctx.EffectiveMessage.From.Id, message, opts)
	return err
}

func (h *StartHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	msg := ctx.EffectiveMessage

	if msg == nil {
		return false
	}
	return msg.Text != "" && msg.Text == "/start" && msg.Chat.Type == "private"
}

func (h *StartHandler) Name() string {
	return constants.StartHandlerName
}
