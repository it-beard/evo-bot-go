package formatters

import (
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"fmt"

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
					Text:         "🔎 Поиск профиля",
					CallbackData: constants.ProfileViewOtherProfileCallback,
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
					Text:         "✏️ Редактировать мой профиль",
					CallbackData: constants.ProfileEditMyProfileCallback,
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

func ProfileSearchBackCancelButtons(backCallbackData string) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         "🔎 Ещё раз",
					CallbackData: constants.ProfileViewOtherProfileCallback,
				},
			}, {
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
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         "👤 Имя",
					CallbackData: constants.ProfileEditFirstnameCallback,
				},
				{
					Text:         "👤 Фамилия",
					CallbackData: constants.ProfileEditLastnameCallback,
				},
			},
			{
				{
					Text:         "📝 Биография",
					CallbackData: constants.ProfileEditBioCallback,
				},
			},
			{
				{
					Text:         "💼 LinkedIn",
					CallbackData: constants.ProfileEditLinkedinCallback,
				},
				{
					Text:         "💾 GitHub",
					CallbackData: constants.ProfileEditGithubCallback,
				},
				{
					Text:         "🌐 Ссылка",
					CallbackData: constants.ProfileEditWebsiteCallback,
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
		},
	}
}

// Format a readable view of a user profile
func FormatProfileView(user *repositories.User, profile *repositories.Profile, showScore bool) string {
	if profile == nil {
		return "Ваш профиль не найден.\n\nСоздайте профиль через кнопку \"Редактировать мой профиль\"."
	}

	// Format username
	username := user.Firstname
	if user.Lastname != "" {
		username += " " + user.Lastname
	}
	if user.TgUsername != "" {
		username += " (@" + user.TgUsername + ")"
	}

	// Build profile text
	text := fmt.Sprintf("<b>Профиль пользователя:</b> %s\n", username)

	if profile.Bio != "" {
		text += fmt.Sprintf("\n<b>О себе:</b>\n%s\n", profile.Bio)
	}

	// Add social links section if any exists
	hasLinks := profile.LinkedIn != "" || profile.GitHub != "" || profile.Website != ""
	if hasLinks {
		text += "\n<b>Ссылки:</b>\n"

		if profile.LinkedIn != "" {
			text += fmt.Sprintf("• LinkedIn: %s\n", profile.LinkedIn)
		}

		if profile.GitHub != "" {
			text += fmt.Sprintf("• GitHub: %s\n", profile.GitHub)
		}

		if profile.Website != "" {
			text += fmt.Sprintf("• Вебсайт: %s\n", profile.Website)
		}
	}

	if showScore && user.Score > 100 {
		text += fmt.Sprintf("\n<b>%d</b> <i>(что это? хм...)</i>\n", user.Score)
	}

	return text
}
