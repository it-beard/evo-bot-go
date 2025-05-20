package formatters

import (
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"fmt"
	"strconv"

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
					Text:         "📢 Опубликовать",
					CallbackData: constants.ProfilePublishCallback,
				},
				{
					Text:         "📢 Опубликовать (без превью)",
					CallbackData: constants.ProfilePublishWithoutPreviewCallback,
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
				Text:         "💼 LinkedIn",
				CallbackData: constants.ProfileEditLinkedinCallback,
			},
			{
				Text:         "💾 GitHub",
				CallbackData: constants.ProfileEditGithubCallback,
			},
			{
				Text:         "🌐 Ссылка",
				CallbackData: constants.ProfileEditFreeLinkCallback,
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

// Format a readable view of a user profile
func FormatProfileView(user *repositories.User, profile *repositories.Profile, showScore bool) string {
	if profile == nil {
		return "Твой профиль не найден.\n\nСоздай профиль через кнопку \"Редактировать мой профиль\"."
	}

	// Format username
	username := ""
	fullName := user.Firstname
	if user.Lastname != "" {
		fullName += " " + user.Lastname
	}
	fullName = "<b><a href = 'tg://user?id=" + strconv.FormatInt(user.TgID, 10) + "'>" + fullName + "</a></b>"

	if user.TgUsername != "" {
		username = " (@" + user.TgUsername + ")"
	}

	// Build profile text
	text := fmt.Sprintf("🖐 %s %s\n", fullName, username)

	if profile.Bio != "" {
		text += fmt.Sprintf("\n<blockquote>О себе</blockquote>\n%s\n", profile.Bio)
	}

	// Add social links section if any exists
	hasLinks := profile.LinkedIn != "" || profile.GitHub != "" || profile.FreeLink != ""
	if hasLinks {
		text += "\n<blockquote>Ссылки</blockquote>\n"

		if profile.LinkedIn != "" {
			text += fmt.Sprintf("🔸 LinkedIn: %s\n", profile.LinkedIn)
		}

		if profile.GitHub != "" {
			text += fmt.Sprintf("🔸 GitHub: %s\n", profile.GitHub)
		}

		if profile.FreeLink != "" {
			text += fmt.Sprintf("🔸 Ссылка: %s\n", profile.FreeLink)
		}
	}

	if showScore && user.Score > 100 {
		text += fmt.Sprintf("\n<b>%d</b> <i>(что это? хм...)</i>\n", user.Score)
	}

	return text
}

func FormatPublicProfileForMessage(user *repositories.User, profile *repositories.Profile, showScore bool) string {

	// Format username
	username := ""
	fullName := user.Firstname
	if user.Lastname != "" {
		fullName += " " + user.Lastname
	}
	fullName = "<b><a href=\"tg://user?id=" + strconv.FormatInt(user.TgID, 10) + "\">" + fullName + "</a></b>"

	if user.TgUsername != "" {
		username = " (@" + user.TgUsername + ")"
	}

	// Build profile text
	text := fmt.Sprintf("🖐 %s %s\n", fullName, username)

	if profile.Bio != "" {
		text += fmt.Sprintf("\n<blockquote>О себе</blockquote>\n%s\n", profile.Bio)
	}

	// Add social links section if any exists
	hasLinks := profile.LinkedIn != "" || profile.GitHub != "" || profile.FreeLink != ""
	if hasLinks {
		text += "\n<blockquote>Ссылки</blockquote>\n"

		if profile.LinkedIn != "" {
			text += fmt.Sprintf("🔸 LinkedIn: %s\n", profile.LinkedIn)
		}

		if profile.GitHub != "" {
			text += fmt.Sprintf("🔸 GitHub: %s\n", profile.GitHub)
		}

		if profile.FreeLink != "" {
			text += fmt.Sprintf("🔸 Ссылка: %s\n", profile.FreeLink)
		}
	}

	if showScore && user.Score > 100 {
		text += fmt.Sprintf("\n<b>%d</b> <i>(что это? хм...)</i>\n", user.Score)
	}

	return text
}
