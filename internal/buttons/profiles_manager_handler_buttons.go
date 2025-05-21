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

func ProfilesBackStartCancelButtons(backCallbackData string) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         "◀️ Назад",
					CallbackData: backCallbackData,
				},
				{
					Text:         "⏪ Старт",
					CallbackData: constants.AdminProfilesStartCallback,
				},
				{
					Text:         "❌ Отмена",
					CallbackData: constants.AdminProfilesCancelCallback,
				},
			},
		},
	}
}

// ProfilesCoffeeBanButtons returns buttons for managing coffee ban status
func ProfilesCoffeeBanButtons(backCallbackData string, hasCoffeeBan bool) gotgbot.InlineKeyboardMarkup {
	var toggleButtonText string
	if hasCoffeeBan {
		toggleButtonText = "✅ Разрешить"
	} else {
		toggleButtonText = "❌ Запретить"
	}

	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         toggleButtonText,
					CallbackData: constants.AdminProfilesToggleCoffeeBanCallback,
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
					Text:         "👤 Username",
					CallbackData: constants.AdminProfilesEditUsernameCallback,
				},
			},
			{
				{
					Text:         "📝 О себе",
					CallbackData: constants.AdminProfilesEditBioCallback,
				},
				{
					Text:         "☕️ Кофе?",
					CallbackData: constants.AdminProfilesEditCoffeeBanCallback,
				},
			},
			{
				{
					Text:         "📢 Го! (+ превью)",
					CallbackData: constants.AdminProfilesPublishCallback,
				},
				{
					Text:         "📢 Го! (- превью)",
					CallbackData: constants.AdminProfilesPublishNoPreviewCallback,
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
					Text:         "🔍 Поиск по Telegram Username",
					CallbackData: constants.AdminProfilesSearchByUsernameCallback,
				},
			},
			{
				{
					Text:         "🔍 Поиск по Telegram ID",
					CallbackData: constants.AdminProfilesSearchByTelegramIDCallback,
				},
			},
			{
				{
					Text:         "🔍 Поиск по имени и фамилии",
					CallbackData: constants.AdminProfilesSearchByFullNameCallback,
				},
			},
			{
				{
					Text:         "➕ Создать профиль (через реплай)",
					CallbackData: constants.AdminProfilesCreateByForwardedMessageCallback,
				},
			},
			{
				{
					Text:         "🆔 Создать профиль по TelegramID",
					CallbackData: constants.AdminProfilesCreateByTelegramIDCallback,
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
