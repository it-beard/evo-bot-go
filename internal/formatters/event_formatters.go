package formatters

import (
	"fmt"
	"strings"
	"time"

	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
)

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

func GetTypeInRussian(eventType constants.EventType) string {
	switch eventType {
	case constants.EventTypeClubCall:
		return "клубный созвон"
	case constants.EventTypeMeetup:
		return "митап"
	case constants.EventTypeWorkshop:
		return "воркшоп"
	case constants.EventTypeReadingClub:
		return "книжный клуб"
	case constants.EventTypeConference:
		return "конфа"
	default:
		return string(eventType)
	}
}

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

func FormatEventListForTopicsView(events []repositories.Event, title string) string {
	var response strings.Builder
	response.WriteString(fmt.Sprintf("%s:\n", title))

	for _, event := range events {
		// Handle optional started_at field
		startedAtStr := "не указано"
		if event.StartedAt != nil && !event.StartedAt.IsZero() {
			startedAtStr = event.StartedAt.Format("02.01.2006 в 15:04")
		}

		typeEmoji := GetTypeEmoji(constants.EventType(event.Type))
		typeInRussian := GetTypeInRussian(constants.EventType(event.Type))

		response.WriteString(fmt.Sprintf("\n%s _%s_: *%s*\n", typeEmoji, typeInRussian, event.Name))
		response.WriteString(fmt.Sprintf("└   _ID_ /%d, _когда_: %s\n",
			event.ID, startedAtStr))
	}

	return response.String()
}

func FormatEventListForEventsView(events []repositories.Event, title string) string {
	var response strings.Builder
	response.WriteString(fmt.Sprintf("%s:\n", title))

	for _, event := range events {
		// Handle optional started_at field
		startedAtStr := "не указано"
		if event.StartedAt != nil && !event.StartedAt.IsZero() {
			startedAtStr = event.StartedAt.Format("02.01.2006 в 15:04 UTC")

			// Add time remaining if event is in the future
			utcNow := time.Now().UTC()
			if event.StartedAt.After(utcNow) {
				timeUntil := event.StartedAt.Sub(utcNow)

				switch {
				case timeUntil <= 24*time.Hour:
					// Less than 24 hours
					hours := int(timeUntil.Hours())
					mins := int(timeUntil.Minutes()) % 60
					if hours > 0 {
						startedAtStr += fmt.Sprintf(" _(через %dч %dмин)_", hours, mins)
					} else {
						startedAtStr += fmt.Sprintf(" _(через %dмин)_", mins)
					}
				case timeUntil <= 7*24*time.Hour:
					// Less than 7 days
					days := int(timeUntil.Hours() / 24)
					hours := int(timeUntil.Hours()) % 24
					startedAtStr += fmt.Sprintf(" _(через %dд %dч)_", days, hours)
				}
			}
		}

		typeEmoji := GetTypeEmoji(constants.EventType(event.Type))
		typeInRussian := GetTypeInRussian(constants.EventType(event.Type))

		response.WriteString(fmt.Sprintf("\n%s _%s_: *%s*\n", typeEmoji, typeInRussian, event.Name))
		response.WriteString(fmt.Sprintf("└   _когда_: %s\n", startedAtStr))
	}

	return response.String()
}

func FormatEventListForAdmin(events []repositories.Event, title string, cancelCommand string, actionDescription string) string {
	var response strings.Builder
	response.WriteString(fmt.Sprintf("*%s*\n", title))

	for _, event := range events {
		// Handle optional started_at field
		startedAtStr := "не указано"
		if event.StartedAt != nil && !event.StartedAt.IsZero() {
			startedAtStr = event.StartedAt.Format("02.01.2006 в 15:04 UTC")
		}

		statusEmoji := GetStatusEmoji(constants.EventStatus(event.Status))
		typeEmoji := GetTypeEmoji(constants.EventType(event.Type))

		response.WriteString(fmt.Sprintf("\n%s ID /%d: *%s*\n", typeEmoji, event.ID, event.Name))
		response.WriteString(fmt.Sprintf("└ %s _когда_: *%s*\n",
			statusEmoji, startedAtStr))
	}

	response.WriteString(fmt.Sprintf("\nПожалуйста, отправь ID мероприятия, %s.", actionDescription))

	return response.String()
}

func FormatHtmlTopicListForUsers(topics []repositories.Topic, eventName string, eventType string) string {
	var response strings.Builder

	typeEmoji := GetTypeEmoji(constants.EventType(eventType))
	typeInRussian := GetTypeInRussian(constants.EventType(eventType))

	response.WriteString(fmt.Sprintf("\n %s Мероприятие (%s): <b>%s</b>\n", typeEmoji, typeInRussian, eventName))

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

func FormatHtmlTopicListForAdmin(topics []repositories.Topic, eventName string, eventType string) string {
	var response strings.Builder

	typeEmoji := GetTypeEmoji(constants.EventType(eventType))
	typeInRussian := GetTypeInRussian(constants.EventType(eventType))

	response.WriteString(fmt.Sprintf("\n %s <i>Мероприятие (%s):</i> %s\n\n", typeEmoji, typeInRussian, eventName))

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
