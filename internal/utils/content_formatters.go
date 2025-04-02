package utils

import (
	"fmt"
	"strings"

	"evo-bot-go/internal/database/repositories"
)

// FormatContentListForUsers formats a slice of contents for display to users
// It returns a markdown-formatted string with content information
func FormatContentListForUsers(contents []repositories.Content, title string, cancelCommand string, actionDescription string) string {
	var response strings.Builder
	response.WriteString(fmt.Sprintf("*%s*\n", title))

	for _, content := range contents {
		// Handle optional started_at field
		startedAtStr := "не указано"
		if content.StartedAt != nil && !content.StartedAt.IsZero() {
			startedAtStr = content.StartedAt.Format("02.01.2006 в 15:04")
		}

		// Emoji based on content status
		typeEmoji := "🔄"
		if content.Type == "club-call" {
			typeEmoji = "💬"
		} else if content.Type == "meetup" {
			typeEmoji = "🎙"
		}

		response.WriteString(fmt.Sprintf("\nID `%d`: *%s*\n", content.ID, content.Name))
		response.WriteString(fmt.Sprintf("└ %s _%s_, старт: _%s_\n",
			typeEmoji, content.Type, startedAtStr))
	}

	response.WriteString(fmt.Sprintf("\nПожалуйста, отправь ID контента, %s, или /%s для отмены.",
		actionDescription, cancelCommand))

	return response.String()
}

// FormatContentListForAdmin formats a slice of contents for display to admins
// It returns a markdown-formatted string with content information
func FormatContentListForAdmin(contents []repositories.Content, title string, cancelCommand string, actionDescription string) string {
	var response strings.Builder
	response.WriteString(fmt.Sprintf("*%s*\n", title))

	for _, content := range contents {
		// Handle optional started_at field
		startedAtStr := "не указано"
		if content.StartedAt != nil && !content.StartedAt.IsZero() {
			startedAtStr = content.StartedAt.Format("02.01.2006 в 15:04")
		}

		// Emoji based on content status
		statusEmoji := "🔄"
		if content.Status == "finished" {
			statusEmoji = "✅"
		} else if content.Status == "actual" {
			statusEmoji = "🔄"
		}

		response.WriteString(fmt.Sprintf("\nID `%d`: *%s*\n", content.ID, content.Name))
		response.WriteString(fmt.Sprintf("└ %s, тип: _%s_, старт: _%s_\n",
			statusEmoji, content.Type, startedAtStr))
	}

	response.WriteString(fmt.Sprintf("\nПожалуйста, отправь ID контента, %s, или /%s для отмены.",
		actionDescription, cancelCommand))

	return response.String()
}

// FormatTopicListForUsers formats a slice of topics for display to users
// It returns a markdown-formatted string with topic information
func FormatTopicListForUsers(topics []repositories.Topic, contentName string, cancelCommand string) string {
	var response strings.Builder
	response.WriteString(fmt.Sprintf("Темы и вопросы для мероприятия: *%s*\n", contentName))

	if len(topics) == 0 {
		response.WriteString("\nДля этого мероприятия пока нет тем и вопросов.")
	} else {
		for _, topic := range topics {
			dateFormatted := topic.CreatedAt.Format("02.01.2006 в 15:04")
			response.WriteString(fmt.Sprintf("\nID `%d`: *%s*\n", topic.ID, topic.Topic))
			response.WriteString(fmt.Sprintf("└ Создано: _%s_, Пользователь ID: `%d`\n",
				dateFormatted, topic.UserID))
		}
	}

	response.WriteString(fmt.Sprintf("\nИспользуйте /%s для возврата.",
		cancelCommand))

	return response.String()
}
