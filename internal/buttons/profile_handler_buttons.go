package buttons

import (
	"evo-bot-go/internal/constants"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func ProfileMainButtons() gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         "✏️ Редактировать",
					CallbackData: constants.ProfileEditMyProfileCallback,
				},
			},
			{
				{
					Text:         "🔎 Поиск профиля по имени/нику",
					CallbackData: constants.ProfileSearchProfileCallback,
				},
			},
			{
				{
					Text:         "❌ Отмена",
					CallbackData: constants.ProfileFullCancel,
				},
			},
		},
	}
}

func ProfileEditBackCancelButtons(backCallbackData string) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         "◀️ Назад",
					CallbackData: backCallbackData,
				},
				{
					Text:         "✏️ Редактировать",
					CallbackData: constants.ProfileEditMyProfileCallback,
				},
				{
					Text:         "❌ Отмена",
					CallbackData: constants.ProfileFullCancel,
				},
			},
		},
	}
}

func ProfileBackCancelButtons(backCallbackData string) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         "◀️ Назад",
					CallbackData: backCallbackData,
				},
				{
					Text:         "❌ Отмена",
					CallbackData: constants.ProfileFullCancel,
				},
			},
		},
	}
}

func ProfileEditButtons(backCallbackData string) gotgbot.InlineKeyboardMarkup {
	buttons := [][]gotgbot.InlineKeyboardButton{
		{
			{
				Text:         "👤 Имя",
				CallbackData: constants.ProfileEditFirstnameCallback,
			},
			{
				Text:         "👤 Фамилия",
				CallbackData: constants.ProfileEditLastnameCallback,
			},
			{
				Text:         "📝 Био",
				CallbackData: constants.ProfileEditBioCallback,
			},
		},
		{
			{
				Text:         "◀️ Назад",
				CallbackData: backCallbackData,
			},
			{
				Text:         "❌ Отмена",
				CallbackData: constants.ProfileFullCancel,
			},
		},
	}

	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}
