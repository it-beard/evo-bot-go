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
					Text:         "👤 Мой профиль",
					CallbackData: constants.ProfileViewMyProfileCallback,
				},
				{
					Text:         "✏️ Редактировать",
					CallbackData: constants.ProfileEditMyProfileCallback,
				},
			},
			{
				{
					Text:         "📢 Опублик. (+ превью)",
					CallbackData: constants.ProfilePublishCallback,
				},
				{
					Text:         "📢 Опублик. (- превью)",
					CallbackData: constants.ProfilePublishWithoutPreviewCallback,
				},
			},
			{
				{
					Text:         "🔎 Поиск профиля",
					CallbackData: constants.ProfileViewOtherProfileCallback,
				},
				{
					Text:         "🧠 ИИ-поиск по био",
					CallbackData: constants.ProfileBioSearchCallback,
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

func ProfileBackPublishCancelButtons(backCallbackData string) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         "◀️ Назад",
					CallbackData: backCallbackData,
				},
				{
					Text:         "📢 Опубликовать",
					CallbackData: constants.ProfilePublishCallback,
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
				Text:         "📝 О себе",
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

