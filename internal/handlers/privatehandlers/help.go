package privatehandlers

import (
	"your_module_name/internal/handlers"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type HelpHandler struct{}

func NewHelpHandler() handlers.Handler {
	return &HelpHandler{}
}

func (h *HelpHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	helpText := "Доступные команды:\n" +
		"/start - Приветственное сообщение\n" +
		"/help - Инструкция по моему использованию\n" +
		"/save - Команда для сохранения постов из сообщества. В ответ на любой пост или сообщение в сообществе напиши эту команду, и я перешлю его тебе в ЛС. Также, можно просто упомянуть меня в реплае на пост или сообщение, и я сделаю тоже самое."
	_, err := ctx.EffectiveMessage.Reply(b, helpText, nil)
	return err
}

func (h *HelpHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	if ctx.EffectiveMessage == nil {
		return false
	}
	return ctx.EffectiveMessage.Text != "" && ctx.EffectiveMessage.Text == "/help" && ctx.EffectiveMessage.Chat.Type == "private"
}

func (h *HelpHandler) Name() string {
	return "help_handler"
}