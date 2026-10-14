package formatters

import (
	"strings"
	"testing"
	"time"

	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"

	"github.com/stretchr/testify/assert"
)

func timePtr(t time.Time) *time.Time {
	return &t
}

func strPtr(s string) *string {
	return &s
}

var fixedStart = time.Date(2025, time.March, 15, 18, 30, 0, 0, time.UTC)

func TestGetTypeEmoji(t *testing.T) {
	tests := []struct {
		eventType constants.EventType
		expected  string
	}{
		{constants.EventTypeClubCall, "💬"},
		{constants.EventTypeMeetup, "🎙"},
		{constants.EventTypeWorkshop, "⚙️"},
		{constants.EventTypeReadingClub, "📚"},
		{constants.EventTypeConference, "👥"},
		{constants.EventType("unknown"), "🔄"},
		{constants.EventType(""), "🔄"},
	}

	for _, tt := range tests {
		t.Run(string(tt.eventType), func(t *testing.T) {
			assert.Equal(t, tt.expected, GetTypeEmoji(tt.eventType))
		})
	}
}

func TestGetTypeEmoji_AllKnownTypesHaveDedicatedEmoji(t *testing.T) {
	fallback := GetTypeEmoji(constants.EventType("does-not-exist"))
	seen := make(map[string]constants.EventType)
	for _, eventType := range constants.AllEventTypes {
		emoji := GetTypeEmoji(eventType)
		assert.NotEqual(t, fallback, emoji, "event type %q uses the fallback emoji", eventType)
		if other, ok := seen[emoji]; ok {
			t.Errorf("event types %q and %q share emoji %q", other, eventType, emoji)
		}
		seen[emoji] = eventType
	}
}

func TestGetTypeInRussian(t *testing.T) {
	tests := []struct {
		eventType constants.EventType
		expected  string
	}{
		{constants.EventTypeClubCall, "клубный созвон"},
		{constants.EventTypeMeetup, "митап"},
		{constants.EventTypeWorkshop, "воркшоп"},
		{constants.EventTypeReadingClub, "книжный клуб"},
		{constants.EventTypeConference, "конфа"},
		{constants.EventType("hackathon"), "hackathon"},
		{constants.EventType(""), ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.eventType), func(t *testing.T) {
			assert.Equal(t, tt.expected, GetTypeInRussian(tt.eventType))
		})
	}
}

func TestGetStatusEmoji(t *testing.T) {
	tests := []struct {
		status   constants.EventStatus
		expected string
	}{
		{constants.EventStatusFinished, "✅"},
		{constants.EventStatusActual, "🔄"},
		{constants.EventStatus("unknown"), "🔄"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			assert.Equal(t, tt.expected, GetStatusEmoji(tt.status))
		})
	}
}

func TestFormatEventListForTopicsView(t *testing.T) {
	events := []repositories.Event{
		{ID: 1, Name: "Weekly call", Type: string(constants.EventTypeClubCall), StartedAt: timePtr(fixedStart)},
		{ID: 22, Name: "Go meetup", Type: string(constants.EventTypeMeetup), StartedAt: nil},
		{ID: 333, Name: "Zero time", Type: "custom-type", StartedAt: timePtr(time.Time{})},
	}

	result := FormatEventListForTopicsView(events, "Выбери мероприятие")

	expected := "Выбери мероприятие:\n" +
		"\n💬 _клубный созвон_: *Weekly call*\n" +
		"└   _ID_ /1, _когда_: 15.03.2025 в 18:30\n" +
		"\n🎙 _митап_: *Go meetup*\n" +
		"└   _ID_ /22, _когда_: не указано\n" +
		"\n🔄 _custom-type_: *Zero time*\n" +
		"└   _ID_ /333, _когда_: не указано\n"
	assert.Equal(t, expected, result)
}

func TestFormatEventListForTopicsView_Empty(t *testing.T) {
	assert.Equal(t, "Title:\n", FormatEventListForTopicsView(nil, "Title"))
}

func TestFormatEventListForEventsView_StaticCases(t *testing.T) {
	events := []repositories.Event{
		{ID: 1, Name: "Past call", Type: string(constants.EventTypeClubCall), StartedAt: timePtr(fixedStart)},
		{ID: 2, Name: "No date", Type: string(constants.EventTypeWorkshop)},
		{ID: 3, Name: "Zero date", Type: string(constants.EventTypeReadingClub), StartedAt: timePtr(time.Time{})},
	}

	result := FormatEventListForEventsView(events, "Ближайшие мероприятия")

	expected := "Ближайшие мероприятия:\n" +
		"\n💬 _клубный созвон_: *Past call*\n" +
		"└   _когда_: 15.03.2025 в 18:30 UTC\n" +
		"\n⚙️ _воркшоп_: *No date*\n" +
		"└   _когда_: не указано\n" +
		"\n📚 _книжный клуб_: *Zero date*\n" +
		"└   _когда_: не указано\n"
	assert.Equal(t, expected, result)
}

func TestFormatEventListForEventsView_FutureEventCountdown(t *testing.T) {
	// Offsets carry a 30s buffer so the expected values stay stable while the test runs.
	tests := []struct {
		name           string
		offset         time.Duration
		expectedSuffix string
	}{
		{name: "less than an hour", offset: 45*time.Minute + 30*time.Second, expectedSuffix: " _(через 45мин)_"},
		{name: "hours and minutes", offset: 2*time.Hour + 30*time.Minute + 30*time.Second, expectedSuffix: " _(через 2ч 30мин)_"},
		{name: "just under a day", offset: 23*time.Hour + 59*time.Minute + 30*time.Second, expectedSuffix: " _(через 23ч 59мин)_"},
		{name: "days and hours", offset: 3*24*time.Hour + 5*time.Hour + 30*time.Minute, expectedSuffix: " _(через 3д 5ч)_"},
		{name: "just under a week", offset: 6*24*time.Hour + 23*time.Hour + 30*time.Minute, expectedSuffix: " _(через 6д 23ч)_"},
		{name: "more than a week", offset: 10 * 24 * time.Hour, expectedSuffix: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startedAt := time.Now().UTC().Add(tt.offset)
			events := []repositories.Event{
				{ID: 7, Name: "Future", Type: string(constants.EventTypeConference), StartedAt: &startedAt},
			}

			result := FormatEventListForEventsView(events, "T")

			expected := "T:\n" +
				"\n👥 _конфа_: *Future*\n" +
				"└   _когда_: " + startedAt.Format("02.01.2006 в 15:04 UTC") + tt.expectedSuffix + "\n"
			assert.Equal(t, expected, result)
		})
	}
}

func TestFormatEventListForEventsView_Empty(t *testing.T) {
	assert.Equal(t, "Events:\n", FormatEventListForEventsView([]repositories.Event{}, "Events"))
}

func TestFormatEventListForAdmin(t *testing.T) {
	events := []repositories.Event{
		{ID: 5, Name: "Finished meetup", Type: string(constants.EventTypeMeetup), Status: string(constants.EventStatusFinished), StartedAt: timePtr(fixedStart)},
		{ID: 6, Name: "Upcoming call", Type: string(constants.EventTypeClubCall), Status: string(constants.EventStatusActual)},
		{ID: 7, Name: "Odd", Type: "other", Status: "other", StartedAt: timePtr(time.Time{})},
	}

	result := FormatEventListForAdmin(events, "Последние 3 мероприятия:", constants.CancelCommand, "которое ты хочешь удалить")

	expected := "*Последние 3 мероприятия:*\n" +
		"\n🎙 ID /5: *Finished meetup*\n" +
		"└ ✅ _когда_: *15.03.2025 в 18:30 UTC*\n" +
		"\n💬 ID /6: *Upcoming call*\n" +
		"└ 🔄 _когда_: *не указано*\n" +
		"\n🔄 ID /7: *Odd*\n" +
		"└ 🔄 _когда_: *не указано*\n" +
		"\nПожалуйста, отправь ID мероприятия, которое ты хочешь удалить."
	assert.Equal(t, expected, result)
}

func TestFormatEventListForAdmin_Empty(t *testing.T) {
	result := FormatEventListForAdmin(nil, "Title", constants.CancelCommand, "чтобы начать")

	assert.Equal(t, "*Title*\n\nПожалуйста, отправь ID мероприятия, чтобы начать.", result)
}

func TestFormatHtmlTopicListForUsers_NoTopics(t *testing.T) {
	result := FormatHtmlTopicListForUsers(nil, "Go Meetup", string(constants.EventTypeMeetup))

	expected := "\n 🎙 Мероприятие (митап): <b>Go Meetup</b>\n" +
		"\n🔍 Для этого мероприятия пока нет тем и вопросов. \n Используй команду /topicAdd для добавления."
	assert.Equal(t, expected, result)
}

func TestFormatHtmlTopicListForUsers_WithTopics(t *testing.T) {
	topics := []repositories.Topic{
		{ID: 1, Topic: "First question", CreatedAt: time.Date(2025, time.January, 2, 10, 0, 0, 0, time.UTC)},
		{ID: 2, Topic: "Second question", CreatedAt: time.Date(2025, time.February, 28, 23, 59, 0, 0, time.UTC)},
	}

	result := FormatHtmlTopicListForUsers(topics, "Call #5", string(constants.EventTypeClubCall))

	expected := "\n 💬 Мероприятие (клубный созвон): <b>Call #5</b>\n" +
		"📋 Найдено тем и вопросов: <b>2</b>\n\n" +
		"<i>02.01.2025</i> <blockquote expandable>First question</blockquote>\n" +
		"\n" +
		"<i>28.02.2025</i> <blockquote expandable>Second question</blockquote>\n" +
		"\nИспользуй команду /topicAdd для добавления новых тем и вопросов, либо /topics для просмотра тем и вопросов к другому мероприятию."
	assert.Equal(t, expected, result)
}

func TestFormatHtmlTopicListForUsers_SingleTopicHasNoSeparator(t *testing.T) {
	topics := []repositories.Topic{
		{ID: 1, Topic: "Only one", CreatedAt: time.Date(2025, time.May, 1, 0, 0, 0, 0, time.UTC)},
	}

	result := FormatHtmlTopicListForUsers(topics, "Event", string(constants.EventTypeWorkshop))

	assert.Contains(t, result, "Найдено тем и вопросов: <b>1</b>")
	assert.Contains(t, result, "<i>01.05.2025</i> <blockquote expandable>Only one</blockquote>\n\nИспользуй команду /topicAdd")
	assert.Equal(t, 1, strings.Count(result, "<blockquote expandable>"))
}

func TestFormatHtmlTopicListForAdmin_NoTopics(t *testing.T) {
	result := FormatHtmlTopicListForAdmin([]repositories.Topic{}, "Book club", string(constants.EventTypeReadingClub))

	expected := "\n 📚 <i>Мероприятие (книжный клуб):</i> Book club\n\n" +
		"Для этого мероприятия пока нет тем и вопросов."
	assert.Equal(t, expected, result)
}

func TestFormatHtmlTopicListForAdmin_WithTopics(t *testing.T) {
	topics := []repositories.Topic{
		{ID: 10, Topic: "With nickname", UserNickname: strPtr("alice"), CreatedAt: time.Date(2025, time.June, 9, 12, 0, 0, 0, time.UTC)},
		{ID: 11, Topic: "Anonymous", UserNickname: nil, CreatedAt: time.Date(2025, time.June, 10, 12, 0, 0, 0, time.UTC)},
	}

	result := FormatHtmlTopicListForAdmin(topics, "Conf", string(constants.EventTypeConference))

	expected := "\n 👥 <i>Мероприятие (конфа):</i> Conf\n\n" +
		"ID:<code>10</code> / <i>09.06.2025</i> / @alice \n" +
		"<blockquote expandable>With nickname</blockquote> \n" +
		"ID:<code>11</code> / <i>10.06.2025</i> / не указано \n" +
		"<blockquote expandable>Anonymous</blockquote> \n"
	assert.Equal(t, expected, result)
}

func TestFormatHtmlTopicListForAdmin_UnknownEventType(t *testing.T) {
	result := FormatHtmlTopicListForAdmin(nil, "Mystery", "mystery-type")

	assert.True(t, strings.HasPrefix(result, "\n 🔄 <i>Мероприятие (mystery-type):</i> Mystery\n\n"))
}
