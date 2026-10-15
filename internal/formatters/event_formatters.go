package formatters

import (
	"fmt"
	"html"
	"strings"
	"time"

	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/utils"
)

// topicAuthorUnknown is stored as the author of topics added by members without a username
const topicAuthorUnknown = "не указано"

// EventDateLayout is the only format event dates are shown in; dates are always displayed in UTC
const EventDateLayout = "02.01.2006 в 15:04 UTC"

// EventDateInputLayout is the format admins type event dates in (interpreted as UTC)
const EventDateInputLayout = "02.01.2006 15:04"

// EventDateInputHint explains EventDateInputLayout to admins
const EventDateInputHint = "<b>ДД.ММ.ГГГГ ЧЧ:ММ</b> по UTC, например <code>25.12.2026 18:30</code>"

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

// FormatEventType renders an event type as emoji plus its Russian name, e.g. "🎙 митап"
func FormatEventType(eventType constants.EventType) string {
	return fmt.Sprintf("%s %s", GetTypeEmoji(eventType), html.EscapeString(GetTypeInRussian(eventType)))
}

// FormatEventTypeOptions lists all event types as tappable numbered commands
func FormatEventTypeOptions() string {
	var options strings.Builder
	for i, eventType := range constants.AllEventTypes {
		if i > 0 {
			options.WriteString("\n")
		}
		options.WriteString(fmt.Sprintf("/%d. %s", i+1, FormatEventType(eventType)))
	}
	return options.String()
}

// FormatEventDate renders an optional event start in EventDateLayout
func FormatEventDate(startedAt *time.Time) string {
	if startedAt == nil || startedAt.IsZero() {
		return "не указано"
	}
	return startedAt.UTC().Format(EventDateLayout)
}

// FormatEventName renders an event name in bold, escaped for HTML
func FormatEventName(name string) string {
	return "<b>" + html.EscapeString(name) + "</b>"
}

// FormatEventSummary renders a single event in the list entry layout, for prompts and confirmations
func FormatEventSummary(event repositories.Event) string {
	eventType := constants.EventType(event.Type)
	return fmt.Sprintf("%s <i>%s</i>: %s\n└ <i>ID</i> %d, <i>когда</i>: %s",
		GetTypeEmoji(eventType), html.EscapeString(GetTypeInRussian(eventType)), FormatEventName(event.Name),
		event.ID, FormatEventDate(event.StartedAt))
}

// formatTimeUntil returns a hint like " <i>(через 2д 5ч)</i>" for events starting within a week
func formatTimeUntil(startedAt *time.Time, now time.Time) string {
	if startedAt == nil || !startedAt.After(now) {
		return ""
	}

	timeUntil := startedAt.Sub(now)
	switch {
	case timeUntil <= 24*time.Hour:
		hours := int(timeUntil.Hours())
		mins := int(timeUntil.Minutes()) % 60
		if hours > 0 {
			return fmt.Sprintf(" <i>(через %dч %dмин)</i>", hours, mins)
		}
		return fmt.Sprintf(" <i>(через %dмин)</i>", mins)
	case timeUntil <= 7*24*time.Hour:
		days := int(timeUntil.Hours() / 24)
		hours := int(timeUntil.Hours()) % 24
		return fmt.Sprintf(" <i>(через %dд %dч)</i>", days, hours)
	default:
		return ""
	}
}

// eventEntryOptions controls which details an event list entry shows
type eventEntryOptions struct {
	withID     bool
	withStatus bool
}

// formatEventEntry renders one event of a list; every event list of the bot uses this layout
func formatEventEntry(event repositories.Event, opts eventEntryOptions, now time.Time) string {
	var entry strings.Builder

	entry.WriteString("\n")
	if opts.withStatus {
		entry.WriteString(GetStatusEmoji(constants.EventStatus(event.Status)) + " ")
	}
	eventType := constants.EventType(event.Type)
	entry.WriteString(fmt.Sprintf("%s <i>%s</i>: %s\n",
		GetTypeEmoji(eventType), html.EscapeString(GetTypeInRussian(eventType)), FormatEventName(event.Name)))

	entry.WriteString("└ ")
	if opts.withID {
		entry.WriteString(fmt.Sprintf("<i>ID</i> /%d, ", event.ID))
	}
	entry.WriteString(fmt.Sprintf("<i>когда</i>: %s%s\n", FormatEventDate(event.StartedAt), formatTimeUntil(event.StartedAt, now)))

	return entry.String()
}

func formatEventList(events []repositories.Event, title string, opts eventEntryOptions) string {
	var response strings.Builder
	response.WriteString(fmt.Sprintf("<b>%s</b>\n", html.EscapeString(title)))

	now := time.Now().UTC()
	for _, event := range events {
		response.WriteString(formatEventEntry(event, opts, now))
	}

	return response.String()
}

// FormatEventListForEventsView renders the /events list for club members
func FormatEventListForEventsView(events []repositories.Event, title string) string {
	return formatEventList(events, title, eventEntryOptions{})
}

// FormatEventListForTopicsView renders a list of events to pick one by ID, e.g. in /topics
func FormatEventListForTopicsView(events []repositories.Event, title string, actionDescription string) string {
	return formatEventList(events, title, eventEntryOptions{withID: true}) +
		fmt.Sprintf("\n👉 Нажми на ID мероприятия, %s.", actionDescription)
}

// FormatEventListForAdmin renders a list of events with their statuses to pick one by ID in admin commands
func FormatEventListForAdmin(events []repositories.Event, title string, actionDescription string) string {
	return formatEventList(events, title, eventEntryOptions{withID: true, withStatus: true}) +
		fmt.Sprintf("\n<i>%s — актуальное, %s — завершено</i>",
			GetStatusEmoji(constants.EventStatusActual), GetStatusEmoji(constants.EventStatusFinished)) +
		fmt.Sprintf("\n\n👉 Нажми на ID мероприятия, %s.", actionDescription)
}

// formatTopicListHeader renders the event the topic list belongs to
func formatTopicListHeader(eventName string, eventType string) string {
	typed := constants.EventType(eventType)
	return fmt.Sprintf("%s <i>%s</i>: %s\n",
		GetTypeEmoji(typed), html.EscapeString(GetTypeInRussian(typed)), FormatEventName(eventName))
}

// TopicListChunkLimit keeps every message of a split topic list below Telegram's 4096 characters
const TopicListChunkLimit = 4000

// FormatHtmlTopicListForUsers renders the /topics list of an event as a single message
func FormatHtmlTopicListForUsers(topics []repositories.Topic, eventName string, eventType string) string {
	return strings.Join(topicListBlocksForUsers(topics, eventName, eventType), "")
}

// FormatHtmlTopicListForUsersMessages renders the /topics list split into messages that fit Telegram's limit.
// Topics are never cut in half, so their markup stays balanced.
func FormatHtmlTopicListForUsersMessages(topics []repositories.Topic, eventName string, eventType string) []string {
	return utils.JoinBlocksIntoChunks(topicListBlocksForUsers(topics, eventName, eventType), TopicListChunkLimit)
}

func topicListBlocksForUsers(topics []repositories.Topic, eventName string, eventType string) []string {
	header := formatTopicListHeader(eventName, eventType)

	if len(topics) == 0 {
		return []string{header + fmt.Sprintf(
			"\n🔍 Для этого мероприятия пока нет тем и вопросов.\nИспользуй команду /%s для добавления.", constants.TopicAddCommand)}
	}

	topicCount := len(topics)
	blocks := []string{header + fmt.Sprintf("📋 Найдено тем и вопросов: <b>%d</b>\n\n", topicCount)}

	for i, topic := range topics {
		// Format date as DD.MM.YYYY for better readability
		block := fmt.Sprintf(
			"<i>%s</i> <blockquote expandable>%s</blockquote>\n",
			topic.CreatedAt.Format("02.01.2006"),
			html.EscapeString(topic.Topic),
		)

		// Don't add separator after the last item
		if i < topicCount-1 {
			block += "\n"
		}
		blocks = append(blocks, block)
	}

	blocks = append(blocks, fmt.Sprintf(
		"\nИспользуй команду /%s для добавления новых тем и вопросов, либо /%s для просмотра тем и вопросов к другому мероприятию.",
		constants.TopicAddCommand,
		constants.TopicsCommand,
	))

	return blocks
}

// FormatHtmlTopicListForAdmin renders the /showTopics list of an event as a single message
func FormatHtmlTopicListForAdmin(topics []repositories.Topic, eventName string, eventType string) string {
	return strings.Join(topicListBlocksForAdmin(topics, eventName, eventType), "")
}

// FormatHtmlTopicListForAdminMessages renders the /showTopics list split into messages that fit Telegram's limit
func FormatHtmlTopicListForAdminMessages(topics []repositories.Topic, eventName string, eventType string) []string {
	return utils.JoinBlocksIntoChunks(topicListBlocksForAdmin(topics, eventName, eventType), TopicListChunkLimit)
}

func topicListBlocksForAdmin(topics []repositories.Topic, eventName string, eventType string) []string {
	header := formatTopicListHeader(eventName, eventType)

	if len(topics) == 0 {
		return []string{header + "\nДля этого мероприятия пока нет тем и вопросов."}
	}

	blocks := []string{header + fmt.Sprintf("📋 Найдено тем и вопросов: <b>%d</b>\n\n", len(topics))}
	for _, topic := range topics {
		// Topics of members without a username are stored with this placeholder
		userNickname := topicAuthorUnknown
		if topic.UserNickname != nil && *topic.UserNickname != "" && *topic.UserNickname != topicAuthorUnknown {
			userNickname = "@" + html.EscapeString(*topic.UserNickname)
		}
		blocks = append(blocks, fmt.Sprintf(
			"ID:<code>%d</code> / <i>%s</i> / %s \n<blockquote expandable>%s</blockquote> \n",
			topic.ID,
			topic.CreatedAt.Format("02.01.2006"),
			userNickname,
			html.EscapeString(topic.Topic),
		))
	}

	return blocks
}
