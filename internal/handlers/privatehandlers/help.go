package privatehandlers

import (
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

type helpHandler struct {
	config *config.Config
}

func NewHelpHandler(config *config.Config) ext.Handler {
	h := &helpHandler{
		config: config,
	}

	return handlers.NewCommand(constants.HelpCommand, h.handleCommand)
}

func (h *helpHandler) handleCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Only proceed if this is a private chat
	if !utils.CheckPrivateChatType(b, ctx) {
		return nil
	}

	// Check if user is a club member
	if !utils.CheckClubMemberPermissions(b, msg, h.config, constants.HelpCommand) {
		return nil
	}

	helpText := "<blockquote> Доступные команды</blockquote>\n" +
		"/start - Приветственное сообщение\n" +
		"/help - Инструкция по моему использованию\n" +
		"/tool или /tools - Поиск ИИ-инструментов для разработки. Используйте команду с описанием того, что вы ищете, например: `/tool лучшая IDE`.\n" +
		"/content - Поиск видео-контента клуба. Используй команду с описанием того, что ты ищешь, например: <code>/content обзор про MCP</code>. \n\n" +
		"Инструкция со всеми моими возможностями: https://t.me/c/2069889012/127/9470\n"

	if utils.IsUserAdminOrCreator(b, msg.From.Id, h.config.SuperGroupChatID) {
		adminHelpText := "<blockquote> Доступные команды для администраторов</blockquote>\n" +
			"/contentEdit - Редактирование контента клуба.\n" +
			"/contentSetup - Создание нового контента клуба.\n" +
			"/contentDelete - Удаление контента клуба.\n" +
			"/contentFinish - Изменение статуса контента на 'finished'.\n"

		helpText += "\n" + adminHelpText
	}

	utils.SendLoggedHtmlReply(b, ctx.EffectiveMessage, helpText, nil)

	return nil
}
