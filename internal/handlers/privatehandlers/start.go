package privatehandlers

import (
	"your_module_name/internal/handlers"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type StartHandler struct{}

func NewStartHandler() handlers.Handler {
	return &StartHandler{}
}

func (h *StartHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	_, err := b.SendMessage(ctx.EffectiveMessage.From.Id, "Привет! \nЯ *Дженкинс Вебстер*, дворецкий бот клуба _\"Эволюция Кода\"_. \nИспользуй команду /help, чтобы увидеть мои возможности.", &gotgbot.SendMessageOpts{ParseMode: "markdown"})
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
	return "start_handler"
}
