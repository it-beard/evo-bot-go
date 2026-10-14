package topicshandlers

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/formatters"
	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	topicPromptText       = "Отправь мне темы и вопросы к мероприятию <b>Go Meetup</b>:"
	topicAddedText        = "✅ Добавлено!\n\nПосмотреть все темы и вопросы — /topics, добавить ещё — /topicAdd."
	topicAddCancelledText = "Операция добавления темы отменена."
)

// selectEventForTopic opens /topicAdd and selects event 7, returning the ids of the list and the prompt
func selectEventForTopic(t *testing.T, d *deps, h ext.Handler) (listID int64, promptID int64) {
	t.Helper()
	return selectGivenEventForTopic(t, d, h, testEvents[0], topicPromptText)
}

// selectGivenEventForTopic opens /topicAdd with only the given event listed and selects it
func selectGivenEventForTopic(t *testing.T, d *deps, h ext.Handler, event repositories.Event, wantPrompt string) (listID int64, promptID int64) {
	t.Helper()
	listID = openEventList(t, d, h, eventSelectionSpecs()[1], event)
	d.expectEvent(event)
	require.True(t, d.dispatch(t, h, privateText(testUserID, "/"+strconv.Itoa(event.ID))))
	return listID, d.sentID(t, wantPrompt)
}

func TestTopicAddHandler_SelectEventAsksForTopic(t *testing.T) {
	d := newDeps(t)
	h := d.newTopicAddHandler()
	listID := openEventList(t, d, h, eventSelectionSpecs()[1])
	d.client.Reset()
	d.expectEvent(testEvents[0])

	require.True(t, d.dispatch(t, h, privateText(testUserID, "/7")))

	assertKeyboardRemoved(t, d.client, listID)
	prompt := lastSend(t, d.client)
	assert.Equal(t, topicPromptText, prompt.Params["text"])
	assert.Equal(t, "HTML", prompt.Params["parse_mode"])
	assert.Contains(t, prompt.Params["reply_parameters"], `"message_id":`)
	assert.Equal(t, []string{topicAddCallbackConfirmCancel}, callbackData(t, prompt))
}

func TestTopicAddHandler_PromptEscapesEventName(t *testing.T) {
	d := newDeps(t)
	h := d.newTopicAddHandler()
	start := meetupStart
	event := repositories.Event{ID: 13, Name: "Go <Generics> & Co", Type: "meetup", Status: "actual",
		StartedAt: &start, CreatedAt: testTime, UpdatedAt: testTime}

	_, promptID := selectGivenEventForTopic(t, d, h, event, "Отправь мне темы и вопросы к мероприятию <b>Go &lt;Generics&gt; &amp; Co</b>:")

	assert.NotZero(t, promptID)
	prompt := lastSend(t, d.client)
	assert.Equal(t, "HTML", prompt.Params["parse_mode"])
	assert.NotContains(t, prompt.Params["text"], "<Generics>")
}

func TestTopicAddHandler_AddsTopicAndNotifiesAdmin(t *testing.T) {
	tests := []struct {
		name       string
		username   string
		wantNick   string
		wantAuthor string
	}{
		{name: "with username", username: "user501", wantNick: "user501", wantAuthor: "@user501"},
		{name: "without username", username: "", wantNick: "не указано", wantAuthor: "не указано"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t)
			h := d.newTopicAddHandler()
			_, promptID := selectEventForTopic(t, d, h)
			d.client.Reset()
			d.sql.ExpectQuery(insertTopicSQL).WithArgs("Как писать тесты?", tt.wantNick, 7).
				WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
			topic := testutils.NewPrivateMessage(testUserID, "  Как писать тесты?  ")
			topic.From.Username = tt.username

			require.True(t, d.dispatch(t, h, testutils.MessageUpdate(topic)))

			sends := d.client.CallsTo("sendMessage")
			require.Len(t, sends, 2)

			adminNote := sends[0]
			assert.Equal(t, testAdminUserID, adminNote.Int64("chat_id"))
			assert.Equal(t, "HTML", adminNote.Params["parse_mode"])
			assert.Equal(t, "🔔 <b>Новая тема добавлена</b>\n\n<i>Мероприятие:</i> Go Meetup\n<i>Автор:</i> "+tt.wantAuthor+"\n<i>Тема:</i> Как писать тесты?",
				adminNote.Params["text"])

			confirmation := sends[1]
			assert.Equal(t, testUserID, confirmation.Int64("chat_id"))
			assert.Equal(t, topicAddedText, confirmation.Params["text"])
			assert.Empty(t, confirmation.Params["parse_mode"], "the confirmation is plain text")
			assert.Contains(t, confirmation.Params["reply_parameters"], `"message_id":`)

			assertKeyboardRemoved(t, d.client, promptID)
			assert.False(t, d.dispatch(t, h, privateText(testUserID, "ещё тема")), "conversation is ended")
		})
	}
}

func TestTopicAddHandler_AuthorWithoutUsernameHasNoAt(t *testing.T) {
	d := newDeps(t)
	h := d.newTopicAddHandler()
	selectEventForTopic(t, d, h)
	d.client.Reset()
	d.sql.ExpectQuery(insertTopicSQL).WithArgs("Тема", "не указано", 7).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	topic := testutils.NewPrivateMessage(testUserID, "Тема")
	topic.From.Username = ""

	require.True(t, d.dispatch(t, h, testutils.MessageUpdate(topic)))

	adminNote := d.client.CallsTo("sendMessage")[0]
	assert.Equal(t, testAdminUserID, adminNote.Int64("chat_id"))
	assert.Contains(t, adminNote.Params["text"], "<i>Автор:</i> не указано\n")
	assert.NotContains(t, adminNote.Params["text"], "@")
}

func TestTopicAddHandler_AdminNotificationEscapesHTML(t *testing.T) {
	d := newDeps(t)
	h := d.newTopicAddHandler()
	start := meetupStart
	event := repositories.Event{ID: 13, Name: "Go <Generics> & Co", Type: "meetup", Status: "actual",
		StartedAt: &start, CreatedAt: testTime, UpdatedAt: testTime}
	selectGivenEventForTopic(t, d, h, event, "Отправь мне темы и вопросы к мероприятию <b>Go &lt;Generics&gt; &amp; Co</b>:")
	d.client.Reset()

	rawTopic := "<script>alert(1)</script> и <b>жирный</b> & _курсив_"
	// The topic and the nickname are stored as typed, only the notification is escaped
	d.sql.ExpectQuery(insertTopicSQL).WithArgs(rawTopic, "user_name", 13).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(5))
	topic := testutils.NewPrivateMessage(testUserID, rawTopic)
	topic.From.Username = "user_name"

	require.True(t, d.dispatch(t, h, testutils.MessageUpdate(topic)))

	sends := d.client.CallsTo("sendMessage")
	require.Len(t, sends, 2)
	adminNote := sends[0]
	assert.Equal(t, testAdminUserID, adminNote.Int64("chat_id"))
	assert.Equal(t, "HTML", adminNote.Params["parse_mode"])
	assert.Equal(t,
		"🔔 <b>Новая тема добавлена</b>\n\n"+
			"<i>Мероприятие:</i> Go &lt;Generics&gt; &amp; Co\n"+
			"<i>Автор:</i> @user_name\n"+
			"<i>Тема:</i> &lt;script&gt;alert(1)&lt;/script&gt; и &lt;b&gt;жирный&lt;/b&gt; &amp; _курсив_",
		adminNote.Params["text"])
	assert.Equal(t, topicAddedText, sends[1].Params["text"])
}

func TestTopicAddHandler_EmptyTopicKeepsWaiting(t *testing.T) {
	d := newDeps(t)
	h := d.newTopicAddHandler()
	selectEventForTopic(t, d, h)
	d.client.Reset()

	require.True(t, d.dispatch(t, h, privateText(testUserID, "   ")))

	assert.Equal(t, []string{"Тема не может быть пустой. Пожалуйста, отправь текст темы или вопроса или /cancel для отмены."},
		d.client.SentTexts())
	assert.Contains(t, d.client.SentTexts()[0], formatters.CancelHint)
	assert.Empty(t, d.client.CallsTo("editMessageReplyMarkup"))

	// Still waiting for a topic
	d.sql.ExpectQuery(insertTopicSQL).WithArgs("Тема", "user501", 7).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	require.True(t, d.dispatch(t, h, privateText(testUserID, "Тема")))
	assert.Equal(t, topicAddedText, lastSend(t, d.client).Params["text"])
}

func TestTopicAddHandler_TopicCannotBeSaved(t *testing.T) {
	d := newDeps(t)
	h := d.newTopicAddHandler()
	selectEventForTopic(t, d, h)
	d.client.Reset()
	d.sql.ExpectQuery(insertTopicSQL).WillReturnError(errors.New("db down"))

	require.True(t, d.dispatch(t, h, privateText(testUserID, "Тема")))

	assert.Equal(t, []string{"Произошла ошибка при сохранении темы. Пожалуйста, попробуй ещё раз позже."}, d.client.SentTexts(),
		"the admin is not notified")
	assert.False(t, d.dispatch(t, h, privateText(testUserID, "Тема")), "conversation is ended")
}

func TestTopicAddHandler_LostSelectedEventEndsConversation(t *testing.T) {
	tests := []struct {
		name  string
		setup func(h *topicAddHandler)
	}{
		{
			name:  "event id missing",
			setup: func(h *topicAddHandler) {},
		},
		{
			name: "event id of unexpected type",
			setup: func(h *topicAddHandler) {
				h.userStore.Set(testUserID, topicAddCtxDataKeySelectedEventID, "7")
				h.userStore.Set(testUserID, topicAddCtxDataKeySelectedEventName, "Go Meetup")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t)
			h := d.newTopicAddState()
			tt.setup(h)
			update := privateText(testUserID, "Тема")

			err := h.handleTopicEntry(d.bot, ext.NewContext(d.bot, update, nil))

			assert.Equal(t, handlers.EndConversation(), err)
			assert.Equal(t, []string{formatters.LostEventText("topicAdd")}, d.client.SentTexts(), "no topic is saved, the admin is not notified")
			assert.Equal(t, "Произошла ошибка: не удалось найти выбранное мероприятие. Пожалуйста, начни заново с /topicAdd.",
				formatters.LostEventText("topicAdd"))
			_, stillStored := h.userStore.Get(testUserID, topicAddCtxDataKeySelectedEventID)
			assert.False(t, stillStored, "the conversation data is cleared")
		})
	}
}

func TestTopicAddHandler_CancelWhileEnteringTopic(t *testing.T) {
	for _, byButton := range []bool{false, true} {
		name := "command"
		if byButton {
			name = "button"
		}
		t.Run(name, func(t *testing.T) {
			d := newDeps(t)
			h := d.newTopicAddHandler()
			_, promptID := selectEventForTopic(t, d, h)
			d.client.Reset()

			update := privateText(testUserID, "/cancel")
			if byButton {
				update = testutils.CallbackUpdate(testUserID, topicAddCallbackConfirmCancel)
			}
			require.True(t, d.dispatch(t, h, update))

			if byButton {
				assert.Len(t, d.client.CallsTo("answerCallbackQuery"), 1)
			}
			assert.Equal(t, []string{topicAddCancelledText}, d.client.SentTexts())
			assertKeyboardRemoved(t, d.client, promptID)
			assert.False(t, d.dispatch(t, h, privateText(testUserID, "Тема")), "conversation is ended")
		})
	}
}

func TestTopicAddHandler_PastEventIsRejected(t *testing.T) {
	pastStart := time.Date(2026, 9, 3, 16, 0, 0, 0, time.UTC)
	futureStart := time.Now().Add(72 * time.Hour)
	tests := []struct {
		name  string
		event repositories.Event
	}{
		{
			name:  "actual event whose start date has passed",
			event: repositories.Event{ID: 9, Name: "late coffee #0", Type: "meetup", Status: "actual", StartedAt: &pastStart, CreatedAt: testTime, UpdatedAt: testTime},
		},
		{
			name:  "finished event",
			event: repositories.Event{ID: 9, Name: "late coffee #0", Type: "meetup", Status: "finished", StartedAt: &futureStart, CreatedAt: testTime, UpdatedAt: testTime},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t)
			h := d.newTopicAddHandler()
			openEventList(t, d, h, eventSelectionSpecs()[1])
			d.client.Reset()

			d.expectEvent(tt.event)
			require.True(t, d.dispatch(t, h, privateText(testUserID, "/9")))

			assert.Equal(t, []string{
				"Мероприятие с ID 9 уже прошло, к нему нельзя добавить темы и вопросы. Пожалуйста, нажми на ID мероприятия из списка выше или /cancel для отмены.",
			}, d.client.SentTexts())
			assert.Contains(t, d.client.SentTexts()[0], formatters.CancelHint)
			assert.Empty(t, d.client.CallsTo("editMessageReplyMarkup"), "the event list stays usable")

			// Still waiting for an event: an upcoming one can be picked right away
			d.expectEvent(testEvents[0])
			require.True(t, d.dispatch(t, h, privateText(testUserID, "/7")))
			assert.Equal(t, topicPromptText, lastSend(t, d.client).Params["text"])
		})
	}
}

func TestTopicAddHandler_PastEventCanBeCancelled(t *testing.T) {
	pastStart := time.Date(2026, 9, 3, 16, 0, 0, 0, time.UTC)
	event := repositories.Event{ID: 9, Name: "late coffee #0", Type: "meetup", Status: "actual", StartedAt: &pastStart, CreatedAt: testTime, UpdatedAt: testTime}
	d := newDeps(t)
	h := d.newTopicAddHandler()
	listID := openEventList(t, d, h, eventSelectionSpecs()[1])
	d.expectEvent(event)
	require.True(t, d.dispatch(t, h, privateText(testUserID, "/9")))
	d.client.Reset()

	require.True(t, d.dispatch(t, h, privateText(testUserID, "/cancel")))

	assert.Equal(t, []string{topicAddCancelledText}, d.client.SentTexts())
	assertKeyboardRemoved(t, d.client, listID)
}

func TestTopicAddHandler_TooLongTopicIsRejected(t *testing.T) {
	d := newDeps(t)
	h := d.newTopicAddHandler()
	selectEventForTopic(t, d, h)
	d.client.Reset()

	require.True(t, d.dispatch(t, h, privateText(testUserID, strings.Repeat("я", 1001))))

	assert.Equal(t, []string{
		"Тема слишком длинная (1001 символов). Пожалуйста, сократи её до 1000 символов и отправь снова или /cancel для отмены.",
	}, d.client.SentTexts())
	assert.Empty(t, d.client.CallsTo("editMessageReplyMarkup"), "still waiting for the topic")

	// A topic of exactly the limit is accepted
	topic := strings.Repeat("я", 1000)
	d.sql.ExpectQuery(insertTopicSQL).WithArgs(topic, "user501", 7).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	require.True(t, d.dispatch(t, h, privateText(testUserID, topic)))
	assert.Contains(t, lastSend(t, d.client).Params["text"], "Добавлено!")
}
