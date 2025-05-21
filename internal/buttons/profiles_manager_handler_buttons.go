package buttons

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
				{
					Text:         "📝 О себе",
					CallbackData: constants.AdminProfilesEditBioCallback,
				},
			},
			{
				{
					Text:         "📢 Го! (превью)",
					CallbackData: constants.AdminProfilesPublishCallback,
				},
				{
					Text:         "📢 Го! (без превью)",
					CallbackData: constants.AdminProfilesPublishNoPreviewCallback,
				},
				{
					Text:         "☕️ Кофе?",
					CallbackData: constants.AdminProfilesEditCoffeeBanCallback,
				},
			},
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

func ProfilesMainMenuButtons() gotgbot.InlineKeyboardMarkup {
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
