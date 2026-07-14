package formatters

import (
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"fmt"
)

// FormatHelpMessage generates the help message text with appropriate commands based on user permissions
func FormatHelpMessage(isAdmin bool, config *config.Config) string {
	helpText := "<b>📋 Функционал бота</b>\n\n" +
		"<b>🏠 Базовые команды</b>\n" +
		"└ /start - Приветственное сообщение\n" +
		"└ /help - Показать список моих команд\n" +
		"└ /cancel - Принудительно отменяет любой диалог\n\n" +
		"<b>👤 Профиль</b>\n" +
		"└ /profile - Управление своим профилем, поиск профилей клубчан, публикация и обновление информации о себе в канале «Интро»\n\n" +
		"<b>🔍 Поиск</b>\n" +
		"└ /tools - Найти инструменты из канала «Инструменты»\n" +
		"└ /content - Найти видео из канала «Видео-контент»\n" +
		"└ /intro - Найти информацию об участниках клуба из канала «Интро» (умный поиск по профилям клубчан)\n\n" +
		"<b>📅 Мероприятия</b>\n" +
		"└ /events - Показать список предстоящих мероприятий\n" +
		"└ /topics - Просмотреть темы и вопросы к предстоящим мероприятиям\n" +
		"└ /topicAdd - Предложить тему или вопрос к предстоящему мероприятию"

	featuresDescription := "\n\n<b>☕️ Random Coffee</b>\n" +
		"Я создаю еженедельные опросы для участия в клубных встречах. " +
		"Используй опрос, чтобы поучаствовать в созвонах и познакомиться с другими клубчанами. " +
		fmt.Sprintf("Пары для созвонов объявляются в начале недели в канале <a href=\"https://telegram.me/c/%d/%d\">«Random Coffee»</a>.",
			config.SuperGroupChatID, config.RandomCoffeeTopicID)

	helpText += featuresDescription

	helpText += "\n\n" + // Add some spacing before the link
		"<i>💡 <a href=\"https://telegram.me/c/2069889012/127/9470\">Открыть полное руководство</a></i>"

	if isAdmin {
		adminHelpText := "\n\n<b>🔐 Команды администратора</b>\n" +
			fmt.Sprintf("└ /%s - Начать мероприятие\n", constants.EventStartCommand) +
			fmt.Sprintf("└ /%s - Создать новое мероприятие\n", constants.EventSetupCommand) +
			fmt.Sprintf("└ /%s - Редактировать мероприятие\n", constants.EventEditCommand) +
			fmt.Sprintf("└ /%s - Удалить мероприятие\n", constants.EventDeleteCommand) +
			fmt.Sprintf("└ /%s - Просмотреть темы и вопросы к предстоящим мероприятиям <b>с возможностью удаления</b>\n", constants.ShowTopicsCommand) +
			fmt.Sprintf("└ /%s - Ввести код для авторизации TG-клиента (задом наперед)\n", constants.CodeCommand) +
			fmt.Sprintf("└ /%s - Управление профилями клубчан", constants.AdminProfilesCommand)

		testCommandsHelpText := "\n\n<b>⚙️ Команды для тестирования</b>\n" +
			fmt.Sprintf("└ /%s - Ручная генерация саммаризации общения в клубе\n", constants.TrySummarizeCommand) +
			fmt.Sprintf("└ /%s - Ручное создание нового опроса по Random Coffee\n", constants.TryCreateCoffeePoolCommand) +
			fmt.Sprintf("└ /%s - Ручная генерация пар для Random Coffee\n", constants.TryGenerateCoffeePairsCommand) +
			fmt.Sprintf("└ /%s - Отправить ссылку на базу знаний в ЛС\n", constants.TryLinkToLearnCommand)

		helpText += adminHelpText
		helpText += testCommandsHelpText
	}

	return helpText
}
