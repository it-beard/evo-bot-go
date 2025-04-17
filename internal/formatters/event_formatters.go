package formatters

import (
	"fmt"
	"strings"

	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
)

// GetTypeEmoji returns an emoji corresponding to the event type
func GetTypeEmoji(eventType constants.EventType) string {
	switch eventType {
	case constants.EventTypeClubCall:
		return "💬"
	case constants.EventTypeMeetup:
		return "🎙"
	case constants.EventTypeWorkshop:
		return "⚙️"
	case constants.EventTypeReadingClub:
		return "📚"
	case constants.EventTypeConference:
		return "👥"
	default:
		return "🔄"
	}
}

// GetStatusEmoji returns an emoji corresponding to the event status
func GetStatusEmoji(status constants.EventStatus) string {
	switch status {
	case constants.EventStatusFinished:
		return "✅"
	case constants.EventStatusActual:
		return "🔄"
	default:
		return "🔄"
	}
}

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

		typeEmoji := GetTypeEmoji(constants.EventType(event.Type))

		response.WriteString(fmt.Sprintf("\n%s _%s_: *%s*\n", typeEmoji, event.Type, event.Name))
		response.WriteString(fmt.Sprintf("└ _ID_ /%d, _дата проведения_: %s\n",
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

		typeEmoji := GetTypeEmoji(constants.EventType(event.Type))

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

		statusEmoji := GetStatusEmoji(constants.EventStatus(event.Status))
		typeEmoji := GetTypeEmoji(constants.EventType(event.Type))

		response.WriteString(fmt.Sprintf("\n%s ID /%d: *%s*\n", typeEmoji, event.ID, event.Name))
		response.WriteString(fmt.Sprintf("└ %s _старт_: *%s*\n",
			statusEmoji, startedAtStr))
	}

	response.WriteString(fmt.Sprintf("\nПожалуйста, отправь ID мероприятия, %s.", actionDescription))

	return response.String()
}

// FormatHtmlTopicListForUsers formats a slice of topics for display to users
// It returns a html-formatted string with topic information
func FormatHtmlTopicListForUsers(topics []repositories.Topic, eventName string, eventType string) string {
	var response strings.Builder

	typeEmoji := GetTypeEmoji(constants.EventType(eventType))

	response.WriteString(fmt.Sprintf("\n %s Мероприятие: <b>%s</b>\n", typeEmoji, eventName))

	if len(topics) == 0 {
		response.WriteString(
			fmt.Sprintf("\n🔍 Для этого мероприятия пока нет тем и вопросов. \n Используй команду /%s для добавления.", constants.TopicAddCommand))
	} else {
		topicCount := len(topics)
		response.WriteString(fmt.Sprintf("📋 Найдено тем и вопросов: <b>%d</b>\n\n", topicCount))

		for i, topic := range topics {
			// Format date as DD.MM.YYYY for better readability
			dateFormatted := topic.CreatedAt.Format("02.01.2006")
			response.WriteString(fmt.Sprintf(
				"<i>%s</i> <blockquote expandable>%s</blockquote>\n",
				dateFormatted,
				topic.Topic,
			))

			// Don't add separator after the last item
			if i < topicCount-1 {
				response.WriteString("\n")
			}
		}

		response.WriteString(
			fmt.Sprintf(
				"\nИспользуй команду /%s для добавления новых тем и вопросов, либо /%s для просмотра тем и вопросов к другому мероприятию.",
				constants.TopicAddCommand,
				constants.TopicsCommand,
			),
		)
	}

	return response.String()
}

// FormatHtmlTopicListForAdmin formats a slice of topics for display to admins
// It returns a html-formatted string with topic information
func FormatHtmlTopicListForAdmin(topics []repositories.Topic, eventName string, eventType string) string {
	var response strings.Builder

	typeEmoji := GetTypeEmoji(constants.EventType(eventType))

	response.WriteString(fmt.Sprintf("\n %s <i>Мероприятие:</i> %s\n\n", typeEmoji, eventName))

	if len(topics) == 0 {
		response.WriteString("Для этого мероприятия пока нет тем и вопросов.")
	} else {
		for _, topic := range topics {
			userNickname := "не указано"
			if topic.UserNickname != nil {
				userNickname = "@" + *topic.UserNickname
			}
			dateFormatted := topic.CreatedAt.Format("02.01.2006")
			response.WriteString(fmt.Sprintf(
				"ID:<code>%d</code> / <i>%s</i> / %s \n",
				topic.ID,
				dateFormatted,
				userNickname,
			))
			response.WriteString(fmt.Sprintf("<blockquote expandable>%s</blockquote> \n", topic.Topic))
		}
	}

	return response.String()
}
