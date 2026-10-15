package formatters

import (
	"html"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/utils"
	"fmt"
	"strconv"
)

// formatProfileHeader renders the linked full name and the @username of a user, escaped for HTML
func formatProfileHeader(user *repositories.User) string {
	fullName := user.Firstname
	if user.Lastname != "" {
		fullName += " " + user.Lastname
	}
	linkedName := "<b><a href=\"tg://user?id=" + strconv.FormatInt(user.TgID, 10) + "\">" + html.EscapeString(fullName) + "</a></b>"

	username := ""
	if user.TgUsername != "" {
		username = "(@" + html.EscapeString(user.TgUsername) + ")"
	}

	return fmt.Sprintf("🖐 %s %s\n", linkedName, username)
}

// Format a readable view of a user profile
func FormatProfileView(user *repositories.User, profile *repositories.Profile, showScore bool) string {
	if profile == nil {
		return "Твой профиль не найден.\n\nСоздай профиль через кнопку \"Редактировать мой профиль\"."
	}

	text := formatProfileHeader(user)

	if profile.Bio != "" {
		text += fmt.Sprintf("\n<blockquote>О себе</blockquote>\n%s\n", html.EscapeString(profile.Bio))
	}

	if showScore && user.Score > 100 {
		text += fmt.Sprintf("\n<b>%d</b> <i>(что это? хм...)</i>\n", user.Score)
	}

	return text
}

// Format a readable view of a user profile for the admin manager
func FormatProfileManagerView(user *repositories.User, profile *repositories.Profile, hasCoffeeBan bool, config *config.Config) string {
	text := formatProfileHeader(user)

	if profile.Bio != "" {
		text += "\n<i>О себе:</i>"
		text += fmt.Sprintf("<blockquote expandable>%s</blockquote>", html.EscapeString(profile.Bio))
	}
	text += fmt.Sprintf("\n\n<i>Карма:</i> <b>%d</b>", user.Score)

	coffeeBanStatus := "✅ Разрешено"
	if hasCoffeeBan {
		coffeeBanStatus = "❌ Запрещено"
	}
	text += fmt.Sprintf("\n<i>Кофейные встречи:</i> %s", coffeeBanStatus)
	text += fmt.Sprintf("\n<i>Telegram ID:</i> <code>%d</code>", user.TgID)
	if profile.PublishedMessageID.Valid {
		linkToPost := utils.GetIntroMessageLink(config, profile.PublishedMessageID.Int64)
		text += fmt.Sprintf("\n<i>Ссылка на профиль:</i> %s", linkToPost)
	}
	return text
}

func FormatPublicProfileForMessage(user *repositories.User, profile *repositories.Profile, showScore bool) string {
	text := formatProfileHeader(user)

	if profile.Bio != "" {
		text += fmt.Sprintf("\n<blockquote>О себе</blockquote>\n%s\n", html.EscapeString(profile.Bio))
	}

	return text
}
