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

	helpText := "<b>📋 Доступные команды</b>\n\n" +
		"<b>🏠 Основные</b>\n" +
		"• /start - Приветственное сообщение\n" +
		"• /help - Показать список моих команд\n\n" +
		"<b>🔍 Поиск</b>\n" +
		"• /tools - Найти инструменты из канала «Инструменты»\n" +
		"• /content - Найти видео из канала «Видео-контент»\n\n" +
		"<b>📅 Мероприятия</b>\n" +
		"• /events - Показать список предстоящих мероприятий\n" +
		"• /topics - Просмотреть темы и вопросы к предстоящим мероприятиям\n" +
		"• /topicAdd - Предложить тему или вопрос к предстоящему мероприятию\n\n" +
		"<i>💡 Подробная инструкция:</i>\n" +
		"<a href=\"https://t.me/c/2069889012/127/9470\">Открыть полное руководство</a>"

	if utils.IsUserAdminOrCreator(b, msg.From.Id, h.config.SuperGroupChatID) {
		adminHelpText := "\n\n<b>🔐 Команды администратора</b>\n" +
			"• /eventEdit - Редактировать мероприятие\n" +
			"• /eventSetup - Создать новое мероприятие\n" +
			"• /eventDelete - Удалить мероприятие\n" +
			"• /eventFinish - Отметить мероприятие как завершенное\n" +
			"• /showTopics - Просмотреть темы и вопросы к предстоящим мероприятиям с возможностью удаления\n" +
			"• /code - Ввести код для авторизации TG-клиента (задом наперед)\n" +
			"• /trySummarize - Тестирование саммаризации общения в клубе\n"

		helpText += adminHelpText
	}

	utils.SendLoggedHtmlReply(b, ctx.EffectiveMessage, helpText, nil)

	return nil
}
