package formatters

import (
	"evo-bot-go/internal/constants"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func ProfilesBackCancelButtons(backCallbackData string) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         "◀️ Назад",
					CallbackData: backCallbackData,
				},
				{
					Text:         "❌ Отмена",
					CallbackData: constants.AdminProfilesCancelCallback,
				},
			},
		},
	}
}

func ProfilesEditMenuButtons(backCallbackData string) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         "👤 Имя",
					CallbackData: constants.AdminProfilesEditFirstnameCallback,
				},
				{
					Text:         "👤 Фамилия",
					CallbackData: constants.AdminProfilesEditLastnameCallback,
				},
			},
			{
				{
					Text:         "📝 О себе",
					CallbackData: constants.AdminProfilesEditBioCallback,
				},
				{
					Text:         "☕️ Кофе",
					CallbackData: constants.AdminProfilesEditCoffeeBanCallback,
				},
			},
			{
				{
					Text:         "💼 LinkedIn",
					CallbackData: constants.AdminProfilesEditLinkedinCallback,
				},
				{
					Text:         "💾 GitHub",
					CallbackData: constants.AdminProfilesEditGithubCallback,
				},
				{
					Text:         "🌐 Ссылка",
					CallbackData: constants.AdminProfilesEditFreeLinkCallback,
				},
			},
			{
				{
					Text:         "📢 Опубликовать",
					CallbackData: constants.AdminProfilesPublishCallback,
				},
				{
					Text:         "📢 Опубликовать (без превью)",
					CallbackData: constants.AdminProfilesPublishNoPreviewCallback,
				},
			},
			{
				{
					Text:         "◀️ Назад",
					CallbackData: constants.AdminProfilesBackCallback,
				},
				{
					Text:         "❌ Отмена",
					CallbackData: constants.AdminProfilesCancelCallback,
				},
			},
		},
	}
}

func ProfilesMainMenuButtons(backCallbackData string) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         "📝 Редактировать профиль",
					CallbackData: constants.AdminProfilesEditCallback,
				},
			},
			{
				{
					Text:         "➕ Создать профиль",
					CallbackData: constants.AdminProfilesCreateCallback,
				},
			},
			{
				{
					Text:         "❌ Отмена",
					CallbackData: constants.AdminProfilesCancelCallback,
				},
			},
		},
	}
}
