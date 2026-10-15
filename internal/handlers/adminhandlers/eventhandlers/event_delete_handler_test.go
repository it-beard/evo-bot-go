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

// selectEventForDelete selects event 7 and returns the confirmation message.
// Only the selected event is looked up: the list is not loaded again (sqlmock fails on unexpected queries).
func selectEventForDelete(t *testing.T, env *testEnv) gotgbot.Message {
	t.Helper()
	ev := deleteTestEvents()[0]
	env.expectGetEventByID(7, &ev)
	require.True(t, env.sendText(testAdminID, "/7"))
	return env.lastSent().Msg
}

func TestEventDeleteHandler_FullFlowDeletesEventWithTopics(t *testing.T) {
	env := newTestEnv(t, NewEventDeleteHandler)

	// 1. The command lists the last events with their statuses
	listMsg := startDelete(t, env)
	listCall := env.lastSent().Call
	assert.Equal(t, testAdminID, listMsg.Chat.Id)
	assert.Equal(t, "HTML", listCall.Params["parse_mode"])
	assert.Equal(t,
		adminListTitle+
			"\n🔄 🎙 <i>митап</i>: <b>Go Meetup</b>\n└ <i>ID</i> /7, <i>когда</i>: 05.03.2025 в 18:00 UTC\n"+
			"\n✅ 💬 <i>клубный созвон</i>: <b>Old Call</b>\n└ <i>ID</i> /3, <i>когда</i>: не указано\n"+
			adminListLegend+
			"\n\n👉 Нажми на ID мероприятия, которое ты хочешь удалить.",
		listMsg.Text)
	assert.Equal(t, []string{eventDeleteCallbackConfirmCancel}, callbackData(t, listCall))

	// 2. A non-numeric id keeps the conversation in the selection state
	env.reset()
	require.True(t, env.sendText(testAdminID, "abc"))
	assert.Equal(t, []string{invalidEventIDText}, env.texts())

	// 3. An unknown id keeps the selection state as well
	env.reset()
	env.expectGetEventByID(99, nil)
	require.True(t, env.sendText(testAdminID, "/99"))
	assert.Equal(t, []string{eventNotFoundText(99)}, env.texts())
	assert.Empty(t, env.client.CallsTo("editMessageReplyMarkup"))

	// 4. A known id asks for confirmation and removes the list keyboard
	env.reset()
	confirmMsg := selectEventForDelete(t, env)
	env.requireKeyboardRemoved(listMsg)
	confirmCall := env.lastSent().Call
	assert.Equal(t,
		"🗑 <b>Удалить мероприятие?</b>\n\n"+
			"🎙 <i>митап</i>: <b>Go Meetup</b>\n└ <i>ID</i> 7, <i>когда</i>: 05.03.2025 в 18:00 UTC\n\n"+
			"⚠️ Вместе с ним будут удалены все его темы и вопросы. Это действие нельзя отменить.",
		confirmMsg.Text)
	assert.Equal(t, "HTML", confirmCall.Params["parse_mode"])
	assert.Equal(t, []string{eventDeleteCallbackConfirmYes, eventDeleteCallbackConfirmCancel}, callbackData(t, confirmCall))

	// 5. Text during confirmation asks to use the buttons
	env.reset()
	require.True(t, env.sendText(testAdminID, "да"))
	assert.Equal(t, []string{useButtonsText}, env.texts())

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
		"✅ Мероприятие <b>Go Meetup</b> (ID 7) удалено.\n\nСписок всех команд — /help.",
	}, env.texts())
	assert.Equal(t, "HTML", env.lastSent().Call.Params["parse_mode"])

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
			wantText: eventListErrorText,
		},
		{
			name:     "no events",
			setup:    func(env *testEnv) { env.expectGetLastEvents() },
			wantText: noEventsText,
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

func TestEventDeleteHandler_SelectEventLookupErrorKeepsWaiting(t *testing.T) {
	env := newTestEnv(t, NewEventDeleteHandler)
	listMsg := startDelete(t, env)
	env.reset()

	// A failed lookup is reported like an unknown id and the admin can pick again
	env.mock.ExpectQuery(regexp.QuoteMeta(sqlGetEventByID)).WithArgs(7).WillReturnError(errors.New("db down"))
	require.True(t, env.sendText(testAdminID, "/7"))

	assert.Equal(t, []string{eventNotFoundText(7)}, env.texts())
	assert.Empty(t, env.client.CallsTo("editMessageReplyMarkup"))

	// The conversation still waits for an event id
	env.reset()
	confirmMsg := selectEventForDelete(t, env)
	env.requireKeyboardRemoved(listMsg)
	assert.Contains(t, confirmMsg.Text, "🗑 <b>Удалить мероприятие?</b>")
}

func TestEventDeleteHandler_SelectAnyExistingEvent(t *testing.T) {
	env := newTestEnv(t, NewEventDeleteHandler)
	startDelete(t, env)
	env.reset()

	// The event is looked up by id, so an event that is not among the listed ones can be deleted too
	old := repositories.Event{ID: 1, Name: "Ancient Call", Type: "club-call", Status: "finished"}
	env.expectGetEventByID(1, &old)
	require.True(t, env.sendText(testAdminID, "/1"))

	assert.Contains(t, env.lastSent().Msg.Text, "💬 <i>клубный созвон</i>: <b>Ancient Call</b>\n└ <i>ID</i> 1, <i>когда</i>: не указано")

	env.reset()
	env.mock.ExpectQuery(regexp.QuoteMeta(sqlTopicsByEvent)).WithArgs(1).WillReturnRows(sqlmock.NewRows(topicColumns))
	env.mock.ExpectExec(regexp.QuoteMeta(sqlDeleteEvent)).WithArgs(1).WillReturnResult(sqlmock.NewResult(0, 1))
	require.True(t, env.click(testAdminID, eventDeleteCallbackConfirmYes))

	assert.Equal(t, []string{
		"✅ Мероприятие <b>Ancient Call</b> (ID 1) удалено.\n\nСписок всех команд — /help.",
	}, env.texts())
}

func TestEventDeleteHandler_EscapesEventName(t *testing.T) {
	env := newTestEnv(t, NewEventDeleteHandler)
	ev := repositories.Event{ID: 7, Name: htmlEventName, Type: "meetup", Status: "actual"}

	env.expectGetLastEvents(ev)
	require.True(t, env.sendText(testAdminID, "/eventDelete"))
	assert.Contains(t, env.lastSent().Msg.Text, "🔄 🎙 <i>митап</i>: "+escapedEventName+"\n")
	assert.NotContains(t, env.lastSent().Msg.Text, htmlEventName)

	env.reset()
	env.expectGetEventByID(7, &ev)
	require.True(t, env.sendText(testAdminID, "/7"))
	assert.Contains(t, env.lastSent().Msg.Text, "🎙 <i>митап</i>: "+escapedEventName+"\n└ <i>ID</i> 7")
	assert.NotContains(t, env.lastSent().Msg.Text, htmlEventName)

	env.reset()
	env.mock.ExpectQuery(regexp.QuoteMeta(sqlTopicsByEvent)).WithArgs(7).WillReturnRows(sqlmock.NewRows(topicColumns))
	env.mock.ExpectExec(regexp.QuoteMeta(sqlDeleteEvent)).WithArgs(7).WillReturnResult(sqlmock.NewResult(0, 1))
	require.True(t, env.click(testAdminID, eventDeleteCallbackConfirmYes))
	assert.Equal(t, []string{
		"✅ Мероприятие " + escapedEventName + " (ID 7) удалено.\n\nСписок всех команд — /help.",
	}, env.texts())
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
