package topicshandlers

import (
	"errors"
	"testing"
	"time"

	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const topicPromptText = "Отправь мне темы и вопросы к мероприятию *Go Meetup*:"

// selectEventForTopic opens /topicAdd and selects event 7, returning the ids of the list and the prompt
func selectEventForTopic(t *testing.T, d *deps, h ext.Handler) (listID int64, promptID int64) {
	t.Helper()
	listID = openEventList(t, d, h, eventSelectionSpecs()[1])
	d.expectEvent(testEvents[0])
	require.True(t, d.dispatch(t, h, privateText(testUserID, "/7")))
	return listID, d.sentID(t, topicPromptText)
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
	assert.Equal(t, "Markdown", prompt.Params["parse_mode"])
	assert.Contains(t, prompt.Params["reply_parameters"], `"message_id":`)
	assert.Equal(t, []string{topicAddCallbackConfirmCancel}, callbackData(t, prompt))
}

func TestTopicAddHandler_AddsTopicAndNotifiesAdmin(t *testing.T) {
	tests := []struct {
		name     string
		username string
		wantNick string
	}{
		{name: "with username", username: "user501", wantNick: "user501"},
		{name: "without username", username: "", wantNick: "не указано"},
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
			assert.Equal(t, "Markdown", adminNote.Params["parse_mode"])
			assert.Equal(t, "🔔 *Новая тема добавлена*\n\n_Мероприятие:_ Go Meetup\n_Автор:_ @"+tt.wantNick+"\n_Топик:_ Как писать тесты?",
				adminNote.Params["text"])

			confirmation := sends[1]
			assert.Equal(t, testUserID, confirmation.Int64("chat_id"))
			assert.Equal(t, "Добавлено! \nИспользуй команду /topics для просмотра всех тем и вопросов к мероприятию либо /topicAdd для добавления новых тем и вопросов.",
				confirmation.Params["text"])
			assert.Contains(t, confirmation.Params["reply_parameters"], `"message_id":`)

			assertKeyboardRemoved(t, d.client, promptID)
			assert.False(t, d.dispatch(t, h, privateText(testUserID, "ещё тема")), "conversation is ended")
		})
	}
}

func TestTopicAddHandler_EmptyTopicKeepsWaiting(t *testing.T) {
	d := newDeps(t)
	h := d.newTopicAddHandler()
	selectEventForTopic(t, d, h)
	d.client.Reset()

	require.True(t, d.dispatch(t, h, privateText(testUserID, "   ")))

	assert.Equal(t, []string{"Тема не может быть пустой. Пожалуйста, введи текст темы или отмените операцию."}, d.client.SentTexts())
	assert.Empty(t, d.client.CallsTo("editMessageReplyMarkup"))

	// Still waiting for a topic
	d.sql.ExpectQuery(insertTopicSQL).WithArgs("Тема", "user501", 7).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	require.True(t, d.dispatch(t, h, privateText(testUserID, "Тема")))
	assert.Contains(t, lastSend(t, d.client).Params["text"], "Добавлено!")
}

func TestTopicAddHandler_TopicCannotBeSaved(t *testing.T) {
	d := newDeps(t)
	h := d.newTopicAddHandler()
	selectEventForTopic(t, d, h)
	d.client.Reset()
	d.sql.ExpectQuery(insertTopicSQL).WillReturnError(errors.New("db down"))

	require.True(t, d.dispatch(t, h, privateText(testUserID, "Тема")))

	assert.Equal(t, []string{"Ой! Что-то пошло не так..."}, d.client.SentTexts(), "the admin is not notified")
	assert.False(t, d.dispatch(t, h, privateText(testUserID, "Тема")), "conversation is ended")
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

			assert.Equal(t, []string{"Операция добавления темы отменена."}, d.client.SentTexts())
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
				"Мероприятие с ID 9 уже прошло, к нему нельзя добавить темы и вопросы. Пожалуйста, выбери ID из списка выше.",
			}, d.client.SentTexts())
			assert.Empty(t, d.client.CallsTo("editMessageReplyMarkup"), "the event list stays usable")

			// Still waiting for an event: an upcoming one can be picked right away
			d.expectEvent(testEvents[0])
			require.True(t, d.dispatch(t, h, privateText(testUserID, "/7")))
			assert.Equal(t, topicPromptText, lastSend(t, d.client).Params["text"])
		})
	}
}
