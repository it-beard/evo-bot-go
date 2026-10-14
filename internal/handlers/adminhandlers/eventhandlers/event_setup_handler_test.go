package eventhandlers

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"evo-bot-go/internal/constants"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	setupCanceledText = "Операция создания мероприятия отменена."
	// setupNextStepsHint ends the success message
	setupNextStepsHint = "\n\nРедактировать или завершить мероприятие — /eventEdit, список всех команд — /help."
)

// startSetup runs /eventSetup and returns the name prompt
func startSetup(t *testing.T, env *testEnv) gotgbot.Message {
	t.Helper()
	require.True(t, env.sendText(testAdminID, "/eventSetup"))
	return env.lastSent().Msg
}

// enterSetupName enters the event name and returns the type prompt
func enterSetupName(t *testing.T, env *testEnv) gotgbot.Message {
	t.Helper()
	startSetup(t, env)
	require.True(t, env.sendText(testAdminID, "Go Meetup"))
	return env.lastSent().Msg
}

// chooseSetupType selects the "meetup" type and returns the start date prompt
func chooseSetupType(t *testing.T, env *testEnv) gotgbot.Message {
	t.Helper()
	enterSetupName(t, env)
	require.True(t, env.sendText(testAdminID, "/2"))
	return env.lastSent().Msg
}

// expectCreateEvent expects the "Go Meetup" meetup to be created with the given start
func expectCreateEvent(env *testEnv, startedAt time.Time) *sqlmock.ExpectedQuery {
	return env.mock.ExpectQuery(regexp.QuoteMeta(sqlCreateEvent)).
		WithArgs("Go Meetup", constants.EventTypeMeetup, constants.EventStatusActual, startedAt)
}

func TestEventSetupHandler_FullFlowCreatesEvent(t *testing.T) {
	env := newTestEnv(t, NewEventSetupHandler)

	// 1. Ask for the name
	namePrompt := startSetup(t, env)
	assert.Equal(t, "🆕 <b>Новое мероприятие</b>\n\nВведи название мероприятия:", namePrompt.Text)
	assert.Equal(t, "HTML", env.lastSent().Call.Params["parse_mode"])
	assert.Equal(t, []string{eventSetupCallbackConfirmCancel}, callbackData(t, env.lastSent().Call))

	// 2. Blank name keeps the state
	env.reset()
	require.True(t, env.sendText(testAdminID, "  "))
	assert.Equal(t, []string{
		"Название не может быть пустым. Пожалуйста, введи название мероприятия или /cancel для отмены.",
	}, env.texts())
	assert.Empty(t, env.client.CallsTo("editMessageReplyMarkup"))

	// 3. A name moves on to the type choice
	env.reset()
	require.True(t, env.sendText(testAdminID, "  Go Meetup "))
	env.requireKeyboardRemoved(namePrompt)
	typePrompt := env.lastSent()
	assert.Equal(t,
		"Выбери тип мероприятия <b>Go Meetup</b>:\n"+typeOptionsText+"\n\n👉 Нажми на номер типа.",
		typePrompt.Msg.Text)
	assert.Equal(t, "HTML", typePrompt.Call.Params["parse_mode"])
	assert.Equal(t, []string{eventSetupCallbackConfirmCancel}, callbackData(t, typePrompt.Call))

	// 4. Invalid type choices keep the state
	for _, input := range []string{"party", "/0", "/6", "Meetup", "/"} {
		env.reset()
		require.True(t, env.sendText(testAdminID, input))
		assert.Equal(t, []string{invalidTypeText}, env.texts(), "input %q", input)
		assert.Empty(t, env.client.CallsTo("editMessageReplyMarkup"), "input %q", input)
	}

	// 5. A valid type asks for the start date without touching the database yet
	env.reset()
	require.True(t, env.sendText(testAdminID, "/2"))
	require.NoError(t, env.mock.ExpectationsWereMet(), "the event must not be created before the date is known")
	env.requireKeyboardRemoved(typePrompt.Msg)
	datePrompt := env.lastSent()
	assert.Equal(t, "Когда стартует мероприятие? Введи дату и время в формате "+dateInputHint+":", datePrompt.Msg.Text)
	assert.Equal(t, "HTML", datePrompt.Call.Params["parse_mode"])
	assert.Equal(t, []string{eventSetupCallbackConfirmCancel}, callbackData(t, datePrompt.Call))

	// 6. Invalid date keeps the state
	env.reset()
	require.True(t, env.sendText(testAdminID, "завтра в 19:30"))
	assert.Equal(t, []string{invalidDateText}, env.texts())
	assert.Equal(t, "HTML", env.lastSent().Call.Params["parse_mode"])

	// 7. A valid date creates the event with its start date in one go
	env.reset()
	expectCreateEvent(env, time.Date(2030, 4, 10, 19, 30, 0, 0, time.UTC)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(15))
	require.True(t, env.sendText(testAdminID, "10.04.2030 19:30"))

	env.requireKeyboardRemoved(datePrompt.Msg)
	assert.Equal(t, []string{
		"✅ Мероприятие создано!\n\n" +
			"🎙 <i>митап</i>: <b>Go Meetup</b>\n└ <i>ID</i> 15, <i>когда</i>: 10.04.2030 в 19:30 UTC" +
			setupNextStepsHint,
	}, env.texts())
	assert.Equal(t, "HTML", env.lastSent().Call.Params["parse_mode"])
	assert.False(t, env.sendText(testAdminID, "10.04.2030 19:30"), "conversation must be over")
}

func TestEventSetupHandler_TypeCanBeChosenByNumberOrID(t *testing.T) {
	tests := []struct {
		input       string
		wantType    constants.EventType
		wantSummary string
	}{
		{"/2", constants.EventTypeMeetup, "🎙 <i>митап</i>"},
		{"2", constants.EventTypeMeetup, "🎙 <i>митап</i>"},
		{"meetup", constants.EventTypeMeetup, "🎙 <i>митап</i>"},
		{" club-call ", constants.EventTypeClubCall, "💬 <i>клубный созвон</i>"},
		{"/reading-club", constants.EventTypeReadingClub, "📚 <i>книжный клуб</i>"},
		{"/5", constants.EventTypeConference, "👥 <i>конфа</i>"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			env := newTestEnv(t, NewEventSetupHandler)
			typePrompt := enterSetupName(t, env)
			env.reset()

			require.True(t, env.sendText(testAdminID, tc.input))
			env.requireKeyboardRemoved(typePrompt)
			assert.Contains(t, env.lastSent().Msg.Text, "Когда стартует мероприятие?")

			env.reset()
			startedAt := time.Date(2030, 4, 10, 19, 30, 0, 0, time.UTC)
			env.mock.ExpectQuery(regexp.QuoteMeta(sqlCreateEvent)).
				WithArgs("Go Meetup", tc.wantType, constants.EventStatusActual, startedAt).
				WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(15))
			require.True(t, env.sendText(testAdminID, "10.04.2030 19:30"))

			assert.Equal(t, []string{
				"✅ Мероприятие создано!\n\n" +
					tc.wantSummary + ": <b>Go Meetup</b>\n└ <i>ID</i> 15, <i>когда</i>: 10.04.2030 в 19:30 UTC" +
					setupNextStepsHint,
			}, env.texts())
		})
	}
}

func TestEventSetupHandler_EscapesEventName(t *testing.T) {
	env := newTestEnv(t, NewEventSetupHandler)
	startSetup(t, env)

	env.reset()
	require.True(t, env.sendText(testAdminID, htmlEventName))
	assert.Equal(t,
		"Выбери тип мероприятия "+escapedEventName+":\n"+typeOptionsText+"\n\n👉 Нажми на номер типа.",
		env.lastSent().Msg.Text)

	require.True(t, env.sendText(testAdminID, "/2"))

	// The name is stored as typed and escaped in the success message
	env.reset()
	startedAt := time.Date(2030, 4, 10, 19, 30, 0, 0, time.UTC)
	env.mock.ExpectQuery(regexp.QuoteMeta(sqlCreateEvent)).
		WithArgs(htmlEventName, constants.EventTypeMeetup, constants.EventStatusActual, startedAt).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(15))
	require.True(t, env.sendText(testAdminID, "10.04.2030 19:30"))

	assert.Equal(t, []string{
		"✅ Мероприятие создано!\n\n" +
			"🎙 <i>митап</i>: " + escapedEventName + "\n└ <i>ID</i> 15, <i>когда</i>: 10.04.2030 в 19:30 UTC" +
			setupNextStepsHint,
	}, env.texts())
}

func TestEventSetupHandler_PastStartDateIsSavedWithWarning(t *testing.T) {
	env := newTestEnv(t, NewEventSetupHandler)
	chooseSetupType(t, env)
	env.reset()

	expectCreateEvent(env, time.Date(2025, 4, 10, 19, 30, 0, 0, time.UTC)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(15))
	require.True(t, env.sendText(testAdminID, "10.04.2025 19:30"))

	assert.Equal(t, []string{
		"✅ Мероприятие создано!\n\n" +
			"🎙 <i>митап</i>: <b>Go Meetup</b>\n└ <i>ID</i> 15, <i>когда</i>: 10.04.2025 в 19:30 UTC\n" +
			pastStartDateWarning + setupNextStepsHint,
	}, env.texts())
	assert.Contains(t, pastStartDateWarning, "не показывается в /events")
}

func TestEventSetupHandler_CreateEventFailureEndsConversation(t *testing.T) {
	env := newTestEnv(t, NewEventSetupHandler)
	chooseSetupType(t, env)
	env.reset()

	expectCreateEvent(env, time.Date(2030, 4, 10, 19, 30, 0, 0, time.UTC)).
		WillReturnError(errors.New("db down"))
	require.True(t, env.sendText(testAdminID, "10.04.2030 19:30"))

	assert.Equal(t, []string{"Произошла ошибка при создании мероприятия."}, env.texts())
	assert.False(t, env.sendText(testAdminID, "10.04.2030 19:30"), "conversation must be over")
}

func TestEventSetupHandler_Cancel(t *testing.T) {
	states := []struct {
		name       string
		reachState func(t *testing.T, env *testEnv) gotgbot.Message
	}{
		{"ask name", startSetup},
		{"ask type", enterSetupName},
		{"ask start date", chooseSetupType},
	}

	for _, state := range states {
		for _, viaButton := range []bool{true, false} {
			name := state.name + "/command"
			if viaButton {
				name = state.name + "/button"
			}
			t.Run(name, func(t *testing.T) {
				env := newTestEnv(t, NewEventSetupHandler)
				keyboardMsg := state.reachState(t, env)
				env.reset()

				if viaButton {
					require.True(t, env.click(testAdminID, eventSetupCallbackConfirmCancel))
					assert.Len(t, env.client.CallsTo("answerCallbackQuery"), 1)
				} else {
					require.True(t, env.sendText(testAdminID, "/cancel"))
				}

				assert.Equal(t, []string{setupCanceledText}, env.texts())
				env.requireKeyboardRemoved(keyboardMsg)
				assert.False(t, env.sendText(testAdminID, "/1"), "conversation must be over")
				// Nothing was written to the database, so no half-created event is left behind
				require.NoError(t, env.mock.ExpectationsWereMet())
			})
		}
	}
}
