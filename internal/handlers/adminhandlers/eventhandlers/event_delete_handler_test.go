package eventhandlers

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"evo-bot-go/internal/database/repositories"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const deleteCanceledText = "Операция удаления мероприятия отменена."

func deleteTestEvents() []repositories.Event {
	return []repositories.Event{
		{ID: 7, Name: "Go Meetup", Type: "meetup", Status: "actual", StartedAt: timePtr(time.Date(2025, 3, 5, 18, 0, 0, 0, time.UTC))},
		{ID: 3, Name: "Old Call", Type: "club-call", Status: "finished"},
	}
}

// startDelete runs /eventDelete and returns the event list message
func startDelete(t *testing.T, env *testEnv) gotgbot.Message {
	t.Helper()
	env.expectGetLastEvents(deleteTestEvents()...)
	require.True(t, env.sendText(testAdminID, "/eventDelete"))
	return env.lastSent().Msg
}

// selectEventForDelete selects event 7 and returns the confirmation message
func selectEventForDelete(t *testing.T, env *testEnv) gotgbot.Message {
	t.Helper()
	env.expectGetLastEvents(deleteTestEvents()...)
	require.True(t, env.sendText(testAdminID, "/7"))
	return env.lastSent().Msg
}

func TestEventDeleteHandler_FullFlowDeletesEventWithTopics(t *testing.T) {
	env := newTestEnv(t, NewEventDeleteHandler)

	// 1. The command lists the last events
	listMsg := startDelete(t, env)
	listCall := env.lastSent().Call
	assert.Equal(t, testAdminID, listMsg.Chat.Id)
	assert.Equal(t, "Markdown", listCall.Params["parse_mode"])
	assert.Contains(t, listMsg.Text, "*Последние 2 мероприятия:*")
	assert.Contains(t, listMsg.Text, "ID /7: *Go Meetup*")
	assert.Contains(t, listMsg.Text, "05.03.2025 в 18:00 UTC")
	assert.Contains(t, listMsg.Text, "ID /3: *Old Call*")
	assert.Contains(t, listMsg.Text, "которое ты хочешь удалить")
	assert.Equal(t, []string{eventDeleteCallbackConfirmCancel}, callbackData(t, listCall))

	// 2. A non-numeric id keeps the conversation in the selection state
	env.reset()
	require.True(t, env.sendText(testAdminID, "abc"))
	assert.Equal(t, []string{"Некорректный ID. Пожалуйста, введи числовой ID или используй кнопку для отмены."}, env.texts())

	// 3. An id that is not among the last events keeps the selection state
	env.reset()
	env.expectGetLastEvents(deleteTestEvents()...)
	require.True(t, env.sendText(testAdminID, "/99"))
	assert.Equal(t, []string{
		"Мероприятие с ID 99 не найдено. Пожалуйста, введи корректный ID или используй кнопку для отмены.",
	}, env.texts())
	assert.Empty(t, env.client.CallsTo("editMessageReplyMarkup"))

	// 4. A known id asks for confirmation and removes the list keyboard
	env.reset()
	confirmMsg := selectEventForDelete(t, env)
	env.requireKeyboardRemoved(listMsg)
	confirmCall := env.lastSent().Call
	assert.Equal(t,
		"Ты действительно хочешь удалить мероприятие '*Go Meetup*' (ID: 7)?\n\nЭто также удалит все связанные с ним темы и вопросы.",
		confirmMsg.Text)
	assert.Equal(t, "Markdown", confirmCall.Params["parse_mode"])
	assert.Equal(t, []string{eventDeleteCallbackConfirmYes, eventDeleteCallbackConfirmCancel}, callbackData(t, confirmCall))

	// 5. Text during confirmation asks to use the buttons
	env.reset()
	require.True(t, env.sendText(testAdminID, "да"))
	assert.Equal(t, []string{"Пожалуйста, используй кнопки выше для подтверждения или отмены."}, env.texts())

	// 6. Confirmation deletes related topics and then the event
	env.reset()
	env.mock.ExpectQuery(regexp.QuoteMeta(sqlTopicsByEvent)).WithArgs(7).
		WillReturnRows(sqlmock.NewRows(topicColumns).
			AddRow(11, "Topic A", "alice", 7, fixedTime).
			AddRow(12, "Topic B", "bob", 7, fixedTime))
	env.mock.ExpectExec(regexp.QuoteMeta(sqlDeleteTopic)).WithArgs(11).WillReturnResult(sqlmock.NewResult(0, 1))
	env.mock.ExpectExec(regexp.QuoteMeta(sqlDeleteTopic)).WithArgs(12).WillReturnResult(sqlmock.NewResult(0, 1))
	env.mock.ExpectExec(regexp.QuoteMeta(sqlDeleteEvent)).WithArgs(7).WillReturnResult(sqlmock.NewResult(0, 1))

	require.True(t, env.click(testAdminID, eventDeleteCallbackConfirmYes))

	assert.Len(t, env.client.CallsTo("answerCallbackQuery"), 1)
	env.requireKeyboardRemoved(confirmMsg)
	assert.Equal(t, []string{
		"Мероприятие '*Go Meetup*' успешно удалено. \n\nДля просмотра всех команд используй команду /help",
	}, env.texts())
	assert.Equal(t, "Markdown", env.lastSent().Call.Params["parse_mode"])

	// 7. The conversation is over
	assert.False(t, env.sendText(testAdminID, "/7"))
}

func TestEventDeleteHandler_StartFailures(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(env *testEnv)
		wantText string
	}{
		{
			name: "database error",
			setup: func(env *testEnv) {
				env.mock.ExpectQuery(regexp.QuoteMeta(sqlGetLastEvents)).WillReturnError(errors.New("db down"))
			},
			wantText: "Произошла ошибка при получении списка мероприятий.",
		},
		{
			name:     "no events",
			setup:    func(env *testEnv) { env.expectGetLastEvents() },
			wantText: "Нет созданных мероприятий для удаления.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t, NewEventDeleteHandler)
			tc.setup(env)

			require.True(t, env.sendText(testAdminID, "/eventDelete"))

			assert.Equal(t, []string{tc.wantText}, env.texts())
			assert.False(t, env.sendText(testAdminID, "/7"), "conversation must not be started")
		})
	}
}

func TestEventDeleteHandler_SelectEventDatabaseErrorEndsConversation(t *testing.T) {
	env := newTestEnv(t, NewEventDeleteHandler)
	startDelete(t, env)
	env.reset()

	env.mock.ExpectQuery(regexp.QuoteMeta(sqlGetLastEvents)).WillReturnError(errors.New("db down"))
	require.True(t, env.sendText(testAdminID, "/7"))

	assert.Equal(t, []string{"Произошла ошибка при получении списка мероприятий."}, env.texts())
	assert.False(t, env.sendText(testAdminID, "/7"))
}

func TestEventDeleteHandler_DeleteFailures(t *testing.T) {
	tests := []struct {
		name  string
		setup func(mock sqlmock.Sqlmock)
	}{
		{
			name: "topics lookup fails",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(sqlTopicsByEvent)).WithArgs(7).WillReturnError(errors.New("db down"))
			},
		},
		{
			name: "topic delete fails",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(sqlTopicsByEvent)).WithArgs(7).
					WillReturnRows(sqlmock.NewRows(topicColumns).AddRow(11, "Topic A", "alice", 7, fixedTime))
				mock.ExpectExec(regexp.QuoteMeta(sqlDeleteTopic)).WithArgs(11).WillReturnError(errors.New("db down"))
			},
		},
		{
			name: "event already gone",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(sqlTopicsByEvent)).WithArgs(7).WillReturnRows(sqlmock.NewRows(topicColumns))
				mock.ExpectExec(regexp.QuoteMeta(sqlDeleteEvent)).WithArgs(7).WillReturnResult(sqlmock.NewResult(0, 0))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t, NewEventDeleteHandler)
			startDelete(t, env)
			selectEventForDelete(t, env)
			env.reset()

			tc.setup(env.mock)
			require.True(t, env.click(testAdminID, eventDeleteCallbackConfirmYes))

			assert.Equal(t, []string{"Произошла ошибка при удалении мероприятия."}, env.texts())
			assert.False(t, env.click(testAdminID, eventDeleteCallbackConfirmYes), "conversation must be over")
		})
	}
}

func TestEventDeleteHandler_Cancel(t *testing.T) {
	tests := []struct {
		name       string
		reachState func(t *testing.T, env *testEnv) gotgbot.Message
		cancel     func(env *testEnv) bool
		isCallback bool
	}{
		{
			name:       "cancel button on event list",
			reachState: startDelete,
			cancel:     func(env *testEnv) bool { return env.click(testAdminID, eventDeleteCallbackConfirmCancel) },
			isCallback: true,
		},
		{
			name: "cancel button on confirmation",
			reachState: func(t *testing.T, env *testEnv) gotgbot.Message {
				startDelete(t, env)
				return selectEventForDelete(t, env)
			},
			cancel:     func(env *testEnv) bool { return env.click(testAdminID, eventDeleteCallbackConfirmCancel) },
			isCallback: true,
		},
		{
			name:       "cancel command on event list",
			reachState: startDelete,
			cancel:     func(env *testEnv) bool { return env.sendText(testAdminID, "/cancel") },
		},
		{
			name: "cancel command on confirmation",
			reachState: func(t *testing.T, env *testEnv) gotgbot.Message {
				startDelete(t, env)
				return selectEventForDelete(t, env)
			},
			cancel: func(env *testEnv) bool { return env.sendText(testAdminID, "/cancel") },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t, NewEventDeleteHandler)
			keyboardMsg := tc.reachState(t, env)
			env.reset()

			require.True(t, tc.cancel(env))

			assert.Equal(t, []string{deleteCanceledText}, env.texts())
			env.requireKeyboardRemoved(keyboardMsg)
			if tc.isCallback {
				assert.Len(t, env.client.CallsTo("answerCallbackQuery"), 1)
			}
			// Nothing is deleted after cancellation and the conversation is over
			assert.False(t, env.click(testAdminID, eventDeleteCallbackConfirmYes))
			assert.False(t, env.sendText(testAdminID, "/7"))
		})
	}
}
