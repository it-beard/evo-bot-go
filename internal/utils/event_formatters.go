package utils

import (
	"fmt"
	"strings"

	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
)

// FormatEventListForUsers formats a slice of events for display to users
// It returns a markdown-formatted string with event information
func FormatEventListForUsers(events []repositories.Event, title string) string {
	var response strings.Builder
	response.WriteString(fmt.Sprintf("%s:\n", title))

	for _, event := range events {
		// Handle optional started_at field
		startedAtStr := "не указано"
		if event.StartedAt != nil && !event.StartedAt.IsZero() {
			startedAtStr = event.StartedAt.Format("02.01.2006 в 15:04")
		}

		// Emoji based on event status
		typeEmoji := "🔄"
		if event.Type == "club-call" {
			typeEmoji = "💬"
		} else if event.Type == "meetup" {
			typeEmoji = "🎙"
		}

		response.WriteString(fmt.Sprintf("\n%s _%s_: *%s*\n", typeEmoji, event.Type, event.Name))
		response.WriteString(fmt.Sprintf("└ _ID_ `%d`, _дата проведения_: %s\n",
			event.ID, startedAtStr))
	}

	return response.String()
}

// FormatEventListForUsersWithoutIds formats a slice of events for display to users
// It returns a markdown-formatted string with event information
func FormatEventListForUsersWithoutIds(events []repositories.Event, title string) string {
	var response strings.Builder
	response.WriteString(fmt.Sprintf("%s:\n", title))

	for _, event := range events {
		// Handle optional started_at field
		startedAtStr := "не указано"
		if event.StartedAt != nil && !event.StartedAt.IsZero() {
			startedAtStr = event.StartedAt.Format("02.01.2006 в 15:04")
		}

		// Emoji based on event status
		typeEmoji := "🔄"
		if event.Type == "club-call" {
			typeEmoji = "💬"
		} else if event.Type == "meetup" {
			typeEmoji = "🎙"
		}

		response.WriteString(fmt.Sprintf("\n%s _%s_: *%s*\n", typeEmoji, event.Type, event.Name))
		response.WriteString(fmt.Sprintf("└ _дата проведения_: %s\n", startedAtStr))
	}

	return response.String()
}

// FormatEventListForAdmin formats a slice of events for display to admins
// It returns a markdown-formatted string with event information
func FormatEventListForAdmin(events []repositories.Event, title string, cancelCommand string, actionDescription string) string {
	var response strings.Builder
	response.WriteString(fmt.Sprintf("*%s*\n", title))

	for _, event := range events {
		// Handle optional started_at field
		startedAtStr := "не указано"
		if event.StartedAt != nil && !event.StartedAt.IsZero() {
			startedAtStr = event.StartedAt.Format("02.01.2006 в 15:04")
		}

		// Emoji based on event status
		statusEmoji := "🔄"
		if event.Status == "finished" {
			statusEmoji = "✅"
		} else if event.Status == "actual" {
			statusEmoji = "🔄"
		}

		response.WriteString(fmt.Sprintf("\nID `%d`: *%s*\n", event.ID, event.Name))
		response.WriteString(fmt.Sprintf("└ %s, тип: _%s_, старт: _%s_\n",
			statusEmoji, event.Type, startedAtStr))
	}

	response.WriteString(fmt.Sprintf("\nПожалуйста, отправь ID мероприятия, %s, или /%s для отмены.",
		actionDescription, cancelCommand))

	return response.String()
}

// FormatTopicListForUsers formats a slice of topics for display to users
// It returns a markdown-formatted string with topic information
func FormatTopicListForUsers(topics []repositories.Topic, eventName string, eventType string) string {
	var response strings.Builder
	// Emoji based on event status
	typeEmoji := "🔄"
	if eventType == "club-call" {
		typeEmoji = "💬"
	} else if eventType == "meetup" {
		typeEmoji = "🎙"
	}
	response.WriteString(fmt.Sprintf("\n %s _Мероприятие:_ *%s*\n", typeEmoji, eventName))

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
func FormatTopicListForAdmin(topics []repositories.Topic, eventName string, eventType string) string {
	var response strings.Builder
	typeEmoji := "🔄"
	if eventType == "club-call" {
		typeEmoji = "💬"
	} else if eventType == "meetup" {
		typeEmoji = "🎙"
	}
	response.WriteString(fmt.Sprintf("\n %s _Мероприятие:_ *%s*\n", typeEmoji, eventName))

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
