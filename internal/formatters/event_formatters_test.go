package formatters

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func timePtr(t time.Time) *time.Time {
	return &t
}

func strPtr(s string) *string {
	return &s
}

var fixedStart = time.Date(2025, time.March, 15, 18, 30, 0, 0, time.UTC)

// moscow is a non-UTC zone used to check that dates are always shown in UTC
var moscow = time.FixedZone("MSK", 3*60*60)

// trickyEventName mixes HTML markup with Markdown markup characters
const trickyEventName = "<b>Go</b> & Rust_* <script>"
const escapedTrickyEventName = "&lt;b&gt;Go&lt;/b&gt; &amp; Rust_* &lt;script&gt;"

var telegramTagPattern = regexp.MustCompile(`^<(/?)(b|i|u|s|code|pre|blockquote|a|tg-spoiler)(\s[^<>]*)?>`)
var telegramEntityPattern = regexp.MustCompile(`^&(lt|gt|amp|quot|#[0-9]+);`)

// assertTelegramHTML checks that s is HTML Telegram can parse: only supported tags, properly nested,
// and no stray "<", ">" or "&" outside tags and entities.
func assertTelegramHTML(t *testing.T, s string) {
	t.Helper()
	var open []string
	for i := 0; i < len(s); {
		switch s[i] {
		case '<':
			m := telegramTagPattern.FindStringSubmatch(s[i:])
			if m == nil {
				t.Errorf("unsupported or unescaped tag at byte %d in %q", i, s)
				return
			}
			if m[1] == "/" {
				if len(open) == 0 || open[len(open)-1] != m[2] {
					t.Errorf("unexpected closing tag </%s> at byte %d in %q", m[2], i, s)
					return
				}
				open = open[:len(open)-1]
			} else {
				open = append(open, m[2])
			}
			i += len(m[0])
		case '>':
			t.Errorf("unescaped '>' at byte %d in %q", i, s)
			return
		case '&':
			m := telegramEntityPattern.FindString(s[i:])
			if m == "" {
				t.Errorf("unescaped '&' at byte %d in %q", i, s)
				return
			}
			i += len(m)
		default:
			i++
		}
	}
	assert.Empty(t, open, "unclosed tags in %q", s)
}

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

func TestFormatEventType(t *testing.T) {
	tests := []struct {
		eventType constants.EventType
		expected  string
	}{
		{constants.EventTypeClubCall, "💬 клубный созвон"},
		{constants.EventTypeMeetup, "🎙 митап"},
		{constants.EventTypeWorkshop, "⚙️ воркшоп"},
		{constants.EventTypeReadingClub, "📚 книжный клуб"},
		{constants.EventTypeConference, "👥 конфа"},
		{constants.EventType("hackathon"), "🔄 hackathon"},
		{constants.EventType("<i>x</i> & y"), "🔄 &lt;i&gt;x&lt;/i&gt; &amp; y"},
	}

	for _, tt := range tests {
		t.Run(string(tt.eventType), func(t *testing.T) {
			assert.Equal(t, tt.expected, FormatEventType(tt.eventType))
		})
	}
}

func TestFormatEventTypeOptions(t *testing.T) {
	expected := "/1. 💬 клубный созвон\n" +
		"/2. 🎙 митап\n" +
		"/3. ⚙️ воркшоп\n" +
		"/4. 📚 книжный клуб\n" +
		"/5. 👥 конфа"
	assert.Equal(t, expected, FormatEventTypeOptions())
}

func TestFormatEventTypeOptions_ListsEveryTypeInOrder(t *testing.T) {
	lines := strings.Split(FormatEventTypeOptions(), "\n")

	require.Len(t, lines, len(constants.AllEventTypes))
	for i, eventType := range constants.AllEventTypes {
		assert.Equal(t, fmt.Sprintf("/%d. %s", i+1, FormatEventType(eventType)), lines[i])
	}
}

func TestFormatEventDate(t *testing.T) {
	tests := []struct {
		name      string
		startedAt *time.Time
		expected  string
	}{
		{name: "nil", startedAt: nil, expected: "не указано"},
		{name: "zero", startedAt: timePtr(time.Time{}), expected: "не указано"},
		{name: "zero in another zone", startedAt: timePtr(time.Time{}.In(moscow)), expected: "не указано"},
		{name: "UTC", startedAt: timePtr(fixedStart), expected: "15.03.2025 в 18:30 UTC"},
		{name: "non-UTC is converted to UTC", startedAt: timePtr(fixedStart.In(moscow)), expected: "15.03.2025 в 18:30 UTC"},
		{
			name:      "conversion to UTC crosses midnight",
			startedAt: timePtr(time.Date(2025, time.January, 1, 1, 15, 0, 0, moscow)),
			expected:  "31.12.2024 в 22:15 UTC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, FormatEventDate(tt.startedAt))
		})
	}
}

func TestFormatEventDate_DoesNotChangeTheInput(t *testing.T) {
	startedAt := fixedStart.In(moscow)

	FormatEventDate(&startedAt)

	assert.Equal(t, moscow, startedAt.Location())
}

func TestEventDateInputLayout_ParsesTheHintExample(t *testing.T) {
	parsed, err := time.Parse(EventDateInputLayout, "25.12.2026 18:30")

	require.NoError(t, err)
	assert.Contains(t, EventDateInputHint, "<code>25.12.2026 18:30</code>")
	assert.Equal(t, "25.12.2026 в 18:30 UTC", FormatEventDate(&parsed), "typed dates are UTC")
	assertTelegramHTML(t, EventDateInputHint)
}

func TestFormatEventName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "plain", input: "Go meetup", expected: "<b>Go meetup</b>"},
		{name: "html tags", input: "<b>bold</b>", expected: "<b>&lt;b&gt;bold&lt;/b&gt;</b>"},
		{name: "ampersand", input: "Q&A", expected: "<b>Q&amp;A</b>"},
		{name: "markdown characters are kept", input: "snake_case *star* `code`", expected: "<b>snake_case *star* `code`</b>"},
		{name: "quotes", input: `"Go" it's`, expected: "<b>&#34;Go&#34; it&#39;s</b>"},
		{name: "empty", input: "", expected: "<b></b>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatEventName(tt.input)

			assert.Equal(t, tt.expected, result)
			assertTelegramHTML(t, result)
		})
	}
}

func TestFormatEventSummary(t *testing.T) {
	tests := []struct {
		name     string
		event    repositories.Event
		expected string
	}{
		{
			name:     "with date",
			event:    repositories.Event{ID: 42, Name: "Go meetup", Type: string(constants.EventTypeMeetup), StartedAt: timePtr(fixedStart)},
			expected: "🎙 <i>митап</i>: <b>Go meetup</b>\n└ <i>ID</i> 42, <i>когда</i>: 15.03.2025 в 18:30 UTC",
		},
		{
			name:     "without date",
			event:    repositories.Event{ID: 7, Name: "Call", Type: string(constants.EventTypeClubCall)},
			expected: "💬 <i>клубный созвон</i>: <b>Call</b>\n└ <i>ID</i> 7, <i>когда</i>: не указано",
		},
		{
			name:     "non-UTC date and markup in name and type",
			event:    repositories.Event{ID: 3, Name: trickyEventName, Type: "custom <type>", StartedAt: timePtr(fixedStart.In(moscow))},
			expected: "🔄 <i>custom &lt;type&gt;</i>: <b>" + escapedTrickyEventName + "</b>\n└ <i>ID</i> 3, <i>когда</i>: 15.03.2025 в 18:30 UTC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatEventSummary(tt.event)

			assert.Equal(t, tt.expected, result)
			assertTelegramHTML(t, result)
		})
	}
}

func TestFormatEventListForTopicsView(t *testing.T) {
	events := []repositories.Event{
		{ID: 1, Name: "Weekly call", Type: string(constants.EventTypeClubCall), StartedAt: timePtr(fixedStart)},
		{ID: 22, Name: "Go meetup", Type: string(constants.EventTypeMeetup), StartedAt: nil},
		{ID: 333, Name: "Zero time", Type: "custom-type", StartedAt: timePtr(time.Time{})},
	}

	result := FormatEventListForTopicsView(events, "📋 Ближайшие мероприятия", "для которого ты хочешь увидеть темы и вопросы")

	expected := "<b>📋 Ближайшие мероприятия</b>\n" +
		"\n💬 <i>клубный созвон</i>: <b>Weekly call</b>\n" +
		"└ <i>ID</i> /1, <i>когда</i>: 15.03.2025 в 18:30 UTC\n" +
		"\n🎙 <i>митап</i>: <b>Go meetup</b>\n" +
		"└ <i>ID</i> /22, <i>когда</i>: не указано\n" +
		"\n🔄 <i>custom-type</i>: <b>Zero time</b>\n" +
		"└ <i>ID</i> /333, <i>когда</i>: не указано\n" +
		"\n👉 Нажми на ID мероприятия, для которого ты хочешь увидеть темы и вопросы."
	assert.Equal(t, expected, result)
	assertTelegramHTML(t, result)
}

func TestFormatEventListForTopicsView_Empty(t *testing.T) {
	assert.Equal(t,
		"<b>Title</b>\n\n👉 Нажми на ID мероприятия, чтобы выбрать его.",
		FormatEventListForTopicsView(nil, "Title", "чтобы выбрать его"))
}

func TestFormatEventListForEventsView_StaticCases(t *testing.T) {
	events := []repositories.Event{
		{ID: 1, Name: "Past call", Type: string(constants.EventTypeClubCall), StartedAt: timePtr(fixedStart)},
		{ID: 2, Name: "No date", Type: string(constants.EventTypeWorkshop)},
		{ID: 3, Name: "Zero date", Type: string(constants.EventTypeReadingClub), StartedAt: timePtr(time.Time{})},
		{ID: 4, Name: "Moscow time", Type: string(constants.EventTypeMeetup), StartedAt: timePtr(fixedStart.In(moscow))},
	}

	result := FormatEventListForEventsView(events, "📋 Ближайшие мероприятия")

	expected := "<b>📋 Ближайшие мероприятия</b>\n" +
		"\n💬 <i>клубный созвон</i>: <b>Past call</b>\n" +
		"└ <i>когда</i>: 15.03.2025 в 18:30 UTC\n" +
		"\n⚙️ <i>воркшоп</i>: <b>No date</b>\n" +
		"└ <i>когда</i>: не указано\n" +
		"\n📚 <i>книжный клуб</i>: <b>Zero date</b>\n" +
		"└ <i>когда</i>: не указано\n" +
		"\n🎙 <i>митап</i>: <b>Moscow time</b>\n" +
		"└ <i>когда</i>: 15.03.2025 в 18:30 UTC\n"
	assert.Equal(t, expected, result)
	assertTelegramHTML(t, result)
}

func TestFormatEventListForEventsView_FutureEventCountdown(t *testing.T) {
	// Offsets carry a 30s buffer so the expected values stay stable while the test runs.
	tests := []struct {
		name           string
		offset         time.Duration
		expectedSuffix string
	}{
		{name: "less than an hour", offset: 45*time.Minute + 30*time.Second, expectedSuffix: " <i>(через 45мин)</i>"},
		{name: "hours and minutes", offset: 2*time.Hour + 30*time.Minute + 30*time.Second, expectedSuffix: " <i>(через 2ч 30мин)</i>"},
		{name: "just under a day", offset: 23*time.Hour + 59*time.Minute + 30*time.Second, expectedSuffix: " <i>(через 23ч 59мин)</i>"},
		{name: "days and hours", offset: 3*24*time.Hour + 5*time.Hour + 30*time.Minute, expectedSuffix: " <i>(через 3д 5ч)</i>"},
		{name: "just under a week", offset: 6*24*time.Hour + 23*time.Hour + 30*time.Minute, expectedSuffix: " <i>(через 6д 23ч)</i>"},
		{name: "more than a week", offset: 10 * 24 * time.Hour, expectedSuffix: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startedAt := time.Now().UTC().Add(tt.offset)
			events := []repositories.Event{
				{ID: 7, Name: "Future", Type: string(constants.EventTypeConference), StartedAt: &startedAt},
			}

			result := FormatEventListForEventsView(events, "T")

			expected := "<b>T</b>\n" +
				"\n👥 <i>конфа</i>: <b>Future</b>\n" +
				"└ <i>когда</i>: " + startedAt.Format("02.01.2006 в 15:04 UTC") + tt.expectedSuffix + "\n"
			assert.Equal(t, expected, result)
			assertTelegramHTML(t, result)
		})
	}
}

func TestFormatEventListForEventsView_FutureNonUTCStartIsShownInUTC(t *testing.T) {
	startedAt := time.Now().In(moscow).Add(2*time.Hour + 30*time.Minute + 30*time.Second)
	events := []repositories.Event{
		{ID: 8, Name: "Soon", Type: string(constants.EventTypeMeetup), StartedAt: &startedAt},
	}

	result := FormatEventListForEventsView(events, "T")

	assert.Contains(t, result, "└ <i>когда</i>: "+startedAt.UTC().Format(EventDateLayout)+" <i>(через 2ч 30мин)</i>\n")
}

func TestFormatEventListForEventsView_Empty(t *testing.T) {
	assert.Equal(t, "<b>Events</b>\n", FormatEventListForEventsView([]repositories.Event{}, "Events"))
}

func TestFormatEventListForAdmin(t *testing.T) {
	events := []repositories.Event{
		{ID: 5, Name: "Finished meetup", Type: string(constants.EventTypeMeetup), Status: string(constants.EventStatusFinished), StartedAt: timePtr(fixedStart)},
		{ID: 6, Name: "Upcoming call", Type: string(constants.EventTypeClubCall), Status: string(constants.EventStatusActual)},
		{ID: 7, Name: "Odd", Type: "other", Status: "other", StartedAt: timePtr(time.Time{})},
	}

	result := FormatEventListForAdmin(events, "📋 Последние мероприятия", "которое ты хочешь удалить")

	expected := "<b>📋 Последние мероприятия</b>\n" +
		"\n✅ 🎙 <i>митап</i>: <b>Finished meetup</b>\n" +
		"└ <i>ID</i> /5, <i>когда</i>: 15.03.2025 в 18:30 UTC\n" +
		"\n🔄 💬 <i>клубный созвон</i>: <b>Upcoming call</b>\n" +
		"└ <i>ID</i> /6, <i>когда</i>: не указано\n" +
		"\n🔄 🔄 <i>other</i>: <b>Odd</b>\n" +
		"└ <i>ID</i> /7, <i>когда</i>: не указано\n" +
		"\n<i>🔄 — актуальное, ✅ — завершено</i>" +
		"\n\n👉 Нажми на ID мероприятия, которое ты хочешь удалить."
	assert.Equal(t, expected, result)
	assertTelegramHTML(t, result)
}

func TestFormatEventListForAdmin_Empty(t *testing.T) {
	result := FormatEventListForAdmin(nil, "Title", "чтобы начать")

	assert.Equal(t,
		"<b>Title</b>\n"+
			"\n<i>🔄 — актуальное, ✅ — завершено</i>"+
			"\n\n👉 Нажми на ID мероприятия, чтобы начать.",
		result)
}

func TestFormatEventListForAdmin_LegendUsesStatusEmojis(t *testing.T) {
	result := FormatEventListForAdmin(nil, "T", "x")

	assert.Contains(t, result, GetStatusEmoji(constants.EventStatusActual)+" — актуальное")
	assert.Contains(t, result, GetStatusEmoji(constants.EventStatusFinished)+" — завершено")
}

func TestEventLists_ShowCountdownInEveryVariant(t *testing.T) {
	startedAt := time.Now().UTC().Add(2*24*time.Hour + 3*time.Hour + 30*time.Minute)
	events := []repositories.Event{
		{ID: 9, Name: "Soon", Type: string(constants.EventTypeWorkshop), Status: string(constants.EventStatusActual), StartedAt: &startedAt},
	}
	expectedWhen := "<i>когда</i>: " + startedAt.Format(EventDateLayout) + " <i>(через 2д 3ч)</i>\n"

	tests := []struct {
		name   string
		result string
	}{
		{name: "events view", result: FormatEventListForEventsView(events, "T")},
		{name: "topics view", result: FormatEventListForTopicsView(events, "T", "x")},
		{name: "admin", result: FormatEventListForAdmin(events, "T", "x")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Contains(t, tt.result, expectedWhen)
			assert.NotContains(t, tt.result, "_(через", "the countdown is HTML, not Markdown")
		})
	}
}

func TestEventLists_EscapeEventNamesAndTypes(t *testing.T) {
	const trickyType = "custom & <type>"
	events := []repositories.Event{
		{ID: 1, Name: trickyEventName, Type: trickyType, Status: string(constants.EventStatusActual), StartedAt: timePtr(fixedStart)},
	}

	tests := []struct {
		name   string
		result string
	}{
		{name: "events view", result: FormatEventListForEventsView(events, "T")},
		{name: "topics view", result: FormatEventListForTopicsView(events, "T", "x")},
		{name: "admin", result: FormatEventListForAdmin(events, "T", "x")},
		{name: "summary", result: FormatEventSummary(events[0])},
		{name: "topic list for users", result: FormatHtmlTopicListForUsers(nil, trickyEventName, trickyType)},
		{name: "topic list for admin", result: FormatHtmlTopicListForAdmin(nil, trickyEventName, trickyType)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Contains(t, tt.result, "<b>"+escapedTrickyEventName+"</b>")
			assert.Contains(t, tt.result, "<i>custom &amp; &lt;type&gt;</i>")
			assert.NotContains(t, tt.result, "<script>")
			assert.NotContains(t, tt.result, "<b>Go</b>")
			assertTelegramHTML(t, tt.result)
		})
	}
}

func TestEventLists_EscapeTitle(t *testing.T) {
	const title = "Events <&>"
	const expectedPrefix = "<b>Events &lt;&amp;&gt;</b>\n"

	assert.True(t, strings.HasPrefix(FormatEventListForEventsView(nil, title), expectedPrefix))
	assert.True(t, strings.HasPrefix(FormatEventListForTopicsView(nil, title, "x"), expectedPrefix))
	assert.True(t, strings.HasPrefix(FormatEventListForAdmin(nil, title, "x"), expectedPrefix))
}

func TestFormatHtmlTopicListForUsers_NoTopics(t *testing.T) {
	result := FormatHtmlTopicListForUsers(nil, "Go Meetup", string(constants.EventTypeMeetup))

	expected := "🎙 <i>митап</i>: <b>Go Meetup</b>\n" +
		"\n🔍 Для этого мероприятия пока нет тем и вопросов.\nИспользуй команду /topicAdd для добавления."
	assert.Equal(t, expected, result)
	assertTelegramHTML(t, result)
}

func TestFormatHtmlTopicListForUsers_WithTopics(t *testing.T) {
	topics := []repositories.Topic{
		{ID: 1, Topic: "First question", CreatedAt: time.Date(2025, time.January, 2, 10, 0, 0, 0, time.UTC)},
		{ID: 2, Topic: "Second question", CreatedAt: time.Date(2025, time.February, 28, 23, 59, 0, 0, time.UTC)},
	}

	result := FormatHtmlTopicListForUsers(topics, "Call #5", string(constants.EventTypeClubCall))

	expected := "💬 <i>клубный созвон</i>: <b>Call #5</b>\n" +
		"📋 Найдено тем и вопросов: <b>2</b>\n\n" +
		"<i>02.01.2025</i> <blockquote expandable>First question</blockquote>\n" +
		"\n" +
		"<i>28.02.2025</i> <blockquote expandable>Second question</blockquote>\n" +
		"\nИспользуй команду /topicAdd для добавления новых тем и вопросов, либо /topics для просмотра тем и вопросов к другому мероприятию."
	assert.Equal(t, expected, result)
	assertTelegramHTML(t, result)
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

	expected := "📚 <i>книжный клуб</i>: <b>Book club</b>\n" +
		"\nДля этого мероприятия пока нет тем и вопросов."
	assert.Equal(t, expected, result)
	assertTelegramHTML(t, result)
}

func TestFormatHtmlTopicListForAdmin_WithTopics(t *testing.T) {
	topics := []repositories.Topic{
		{ID: 10, Topic: "With nickname", UserNickname: strPtr("alice"), CreatedAt: time.Date(2025, time.June, 9, 12, 0, 0, 0, time.UTC)},
		{ID: 11, Topic: "Anonymous", UserNickname: nil, CreatedAt: time.Date(2025, time.June, 10, 12, 0, 0, 0, time.UTC)},
	}

	result := FormatHtmlTopicListForAdmin(topics, "Conf", string(constants.EventTypeConference))

	expected := "👥 <i>конфа</i>: <b>Conf</b>\n" +
		"📋 Найдено тем и вопросов: <b>2</b>\n\n" +
		"ID:<code>10</code> / <i>09.06.2025</i> / @alice \n" +
		"<blockquote expandable>With nickname</blockquote> \n" +
		"ID:<code>11</code> / <i>10.06.2025</i> / не указано \n" +
		"<blockquote expandable>Anonymous</blockquote> \n"
	assert.Equal(t, expected, result)
	assertTelegramHTML(t, result)
}

func TestFormatHtmlTopicListForAdmin_UnknownEventType(t *testing.T) {
	result := FormatHtmlTopicListForAdmin(nil, "Mystery", "mystery-type")

	assert.True(t, strings.HasPrefix(result, "🔄 <i>mystery-type</i>: <b>Mystery</b>\n\nДля этого"))
}

// Topics are stored exactly as users typed them in /topicAdd, so the HTML topic lists must escape them.
// These two tests document a production bug: FormatHtmlTopicListForUsers and FormatHtmlTopicListForAdmin
// insert topic.Topic without html.EscapeString, so a topic like "Generics: List<T> & co" makes Telegram
// reject the whole /topics or /showTopics message ("can't parse entities").
const trickyTopic = "Generics: List<T> & co"
const escapedTrickyTopic = "Generics: List&lt;T&gt; &amp; co"

func TestFormatHtmlTopicListForUsers_EscapesTopicText(t *testing.T) {
	topics := []repositories.Topic{
		{ID: 1, Topic: trickyTopic, CreatedAt: time.Date(2025, time.May, 1, 0, 0, 0, 0, time.UTC)},
	}

	result := FormatHtmlTopicListForUsers(topics, "Event", string(constants.EventTypeMeetup))

	assert.Contains(t, result, "<blockquote expandable>"+escapedTrickyTopic+"</blockquote>")
	assertTelegramHTML(t, result)
}

func TestFormatHtmlTopicListForAdmin_EscapesTopicText(t *testing.T) {
	topics := []repositories.Topic{
		{ID: 1, Topic: trickyTopic, UserNickname: strPtr("alice"), CreatedAt: time.Date(2025, time.May, 1, 0, 0, 0, 0, time.UTC)},
	}

	result := FormatHtmlTopicListForAdmin(topics, "Event", string(constants.EventTypeMeetup))

	assert.Contains(t, result, "<blockquote expandable>"+escapedTrickyTopic+"</blockquote>")
	assertTelegramHTML(t, result)
}
