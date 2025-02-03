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
	helpText := "Доступные команды:\n\n" +
		"/start - Приветственное сообщение\n" +
		"/help - Инструкция по моему использованию\n" +
		"/tool или /tools - Поиск ИИ-инструментов для разработки. Используйте команду с описанием того, что вы ищете, например: `/tool лучшая IDE`. Команда работает через GPT-4o, потому ответ может быть не быстрым.\n\n" +
		"Инструкция со всеми моими возможностями: https://t.me/c/2069889012/127/9470"
	_, err := ctx.EffectiveMessage.Reply(b, helpText, &gotgbot.SendMessageOpts{
		ParseMode: "Markdown",
	})
	return err
}

func (h *HelpHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	if ctx.EffectiveMessage == nil {
		return false
	}
	return ctx.EffectiveMessage.Text != "" && ctx.EffectiveMessage.Text == "/help" && ctx.EffectiveMessage.Chat.Type == "private"
}

func (h *HelpHandler) Name() string {
	return helpHandlerName
}
