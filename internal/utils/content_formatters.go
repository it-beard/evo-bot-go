package utils

import (
	"fmt"
	"strings"

	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
)

// FormatContentListForUsers formats a slice of contents for display to users
// It returns a markdown-formatted string with content information
func FormatContentListForUsers(contents []repositories.Content, title string) string {
	var response strings.Builder
	response.WriteString(fmt.Sprintf("%s:\n", title))

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

		response.WriteString(fmt.Sprintf("\n%s _%s_: *%s*\n", typeEmoji, content.Type, content.Name))
		response.WriteString(fmt.Sprintf("└ _ID_ `%d`, _дата проведения_: %s\n",
			content.ID, startedAtStr))
	}

	return response.String()
}

// FormatContentListForUsersWithoutIds formats a slice of contents for display to users
// It returns a markdown-formatted string with content information
func FormatContentListForUsersWithoutIds(contents []repositories.Content, title string) string {
	var response strings.Builder
	response.WriteString(fmt.Sprintf("%s:\n", title))

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

		response.WriteString(fmt.Sprintf("\n%s _%s_: *%s*\n", typeEmoji, content.Type, content.Name))
		response.WriteString(fmt.Sprintf("└ _дата проведения_: %s\n", startedAtStr))
	}

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
func FormatTopicListForUsers(topics []repositories.Topic, contentName string, contentType string) string {
	var response strings.Builder
	// Emoji based on content status
	typeEmoji := "🔄"
	if contentType == "club-call" {
		typeEmoji = "💬"
	} else if contentType == "meetup" {
		typeEmoji = "🎙"
	}
	response.WriteString(fmt.Sprintf("\n %s _Мероприятие:_ *%s*\n", typeEmoji, contentName))

	if len(topics) == 0 {
		response.WriteString(
			fmt.Sprintf("\n🔍 Для этого мероприятия пока нет тем и вопросов. \n Используй команду /%s для добавления.", constants.TopicAddCommand))
	} else {
		topicCount := len(topics)
		response.WriteString(fmt.Sprintf("📋 _Найдено тем:_ *%d*\n\n", topicCount))

		for i, topic := range topics {
			// Format date as DD.MM.YYYY for better readability
			dateFormatted := topic.CreatedAt.Format("02.01.2006")
			response.WriteString(fmt.Sprintf("🔸 _%s_ / *%s*", dateFormatted, topic.Topic))

			// Don't add separator after the last item
			if i < topicCount-1 {
				response.WriteString("\n")
			}
		}
	}

	return response.String()
}

// FormatTopicListForAdmin formats a slice of topics for display to admins
// It returns a markdown-formatted string with topic information
func FormatTopicListForAdmin(topics []repositories.Topic, contentName string, contentType string) string {
	var response strings.Builder
	typeEmoji := "🔄"
	if contentType == "club-call" {
		typeEmoji = "💬"
	} else if contentType == "meetup" {
		typeEmoji = "🎙"
	}
	response.WriteString(fmt.Sprintf("\n %s _Мероприятие:_ *%s*\n", typeEmoji, contentName))

	if len(topics) == 0 {
		response.WriteString("\nДля этого мероприятия пока нет тем и вопросов.")
	} else {
		for _, topic := range topics {
			userNickname := "не указано"
			if topic.UserNickname != nil {
				userNickname = "@" + *topic.UserNickname
			}
			dateFormatted := topic.CreatedAt.Format("02.01.2006")
			response.WriteString(fmt.Sprintf("\n_%s_ / *%s*\n", dateFormatted, topic.Topic))
			response.WriteString(fmt.Sprintf("└ _ID_ `%d`, _пользователь_: %s\n",
				topic.ID, userNickname))
		}
	}

	return response.String()
}
