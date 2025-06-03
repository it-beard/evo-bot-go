package buttons

import "github.com/PaulSonOfLars/gotgbot/v2"

func CancelButton(callbackData string) gotgbot.InlineKeyboardMarkup {
	inlineKeyboard := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         "❌ Отмена",
					CallbackData: callbackData,
				},
			},
		},
	}

	return inlineKeyboard
}

func ConfirmButton(callbackData string) gotgbot.InlineKeyboardMarkup {
	inlineKeyboard := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         "✅ Подтвердить",
					CallbackData: callbackData,
				},
			},
		},
	}

	return inlineKeyboard
}

func ConfirmAndCancelButton(callbackDataYes string, callbackDataNo string) gotgbot.InlineKeyboardMarkup {
	inlineKeyboard := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         "✅ Подтвердить",
					CallbackData: callbackDataYes,
				},
				{
					Text:         "❌ Отмена",
					CallbackData: callbackDataNo,
				},
			},
		},
	}

	return inlineKeyboard
}

func BackAndCancelButton(callbackDataBack string, callbackDataCancel string) gotgbot.InlineKeyboardMarkup {
	inlineKeyboard := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         "◀️ Назад",
					CallbackData: callbackDataBack,
				},
				{
					Text:         "❌ Отмена",
					CallbackData: callbackDataCancel,
				},
			},
		},
	}

	return inlineKeyboard
}
