package privatehandlers

import (
	"github.com/it-beard/evo-bot-go/internal/constants"
	"github.com/it-beard/evo-bot-go/internal/handlers"

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
		"/tool или /tools - Поиск ИИ-инструментов для разработки. Используйте команду с описанием того, что вы ищете, например: `/tool лучшая IDE`.\n" +
		"/content - Поиск видео-контента клуба. Используй команду с описанием того, что ты ищешь, например: `/content обзор про MCP`. \n" +
		"/summarize - Запуск ручной генерации сводки (только для администраторов)\n\n" +
		"Ежедневная сводка: Я автоматически собираю сообщения из настроенных чатов и создаю ежедневную сводку активности, которая публикуется в указанное время в специальном чате.\n\n" +
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
	return constants.HelpHandlerName
}
