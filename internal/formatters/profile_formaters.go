package formatters

import (
	"evo-bot-go/internal/database/repositories"
	"fmt"
	"strconv"
)

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
	if showScore && user.Score > 100 {
		text += fmt.Sprintf("\n<b>%d</b> <i>(что это? хм...)</i>\n", user.Score)
	}

	return text
}
