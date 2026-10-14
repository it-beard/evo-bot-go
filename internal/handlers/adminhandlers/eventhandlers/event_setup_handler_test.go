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

const setupCanceledText = "Операция создания мероприятия отменена."

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

// chooseSetupType selects the "meetup" type (event gets id 15) and returns the start date prompt
func chooseSetupType(t *testing.T, env *testEnv) gotgbot.Message {
	t.Helper()
	enterSetupName(t, env)
	env.mock.ExpectQuery(regexp.QuoteMeta(sqlCreateEvent)).
		WithArgs("Go Meetup", constants.EventTypeMeetup, constants.EventStatusActual).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(15))
	require.True(t, env.sendText(testAdminID, "/2"))
	return env.lastSent().Msg
}

func TestEventSetupHandler_FullFlowCreatesEvent(t *testing.T) {
	env := newTestEnv(t, NewEventSetupHandler)

	// 1. Ask for the name
	namePrompt := startSetup(t, env)
	assert.Equal(t, "Пожалуйста, введи название для нового мероприятия:", namePrompt.Text)
	assert.Equal(t, []string{eventSetupCallbackConfirmCancel}, callbackData(t, env.lastSent().Call))

	// 2. Blank name keeps the state
	env.reset()
	require.True(t, env.sendText(testAdminID, "  "))
	assert.Equal(t, []string{
		"Название не может быть пустым. Пожалуйста, введи название для мероприятия или используй кнопку для отмены.",
	}, env.texts())
	assert.Empty(t, env.client.CallsTo("editMessageReplyMarkup"))

	// 3. A name moves on to the type choice
	env.reset()
	require.True(t, env.sendText(testAdminID, "  Go Meetup "))
	env.requireKeyboardRemoved(namePrompt)
	typePrompt := env.lastSent()
	assert.Equal(t,
		"Выбери тип мероприятия (введи число):\n/1. club-call\n/2. meetup\n/3. workshop\n/4. reading-club\n/5. conference",
		typePrompt.Msg.Text)
	assert.Equal(t, []string{eventSetupCallbackConfirmCancel}, callbackData(t, typePrompt.Call))

	// 4. Invalid type choices keep the state
	for _, input := range []string{"meetup", "/0", "/6"} {
		env.reset()
		require.True(t, env.sendText(testAdminID, input))
		assert.Equal(t, []string{
			"Неверный выбор. Пожалуйста, введи число от 1 до 5, или используй кнопку для отмены.",
		}, env.texts(), "input %q", input)
		assert.Empty(t, env.client.CallsTo("editMessageReplyMarkup"), "input %q", input)
	}

	// 5. A valid type creates the event and asks for the start date
	env.reset()
	env.mock.ExpectQuery(regexp.QuoteMeta(sqlCreateEvent)).
		WithArgs("Go Meetup", constants.EventTypeMeetup, constants.EventStatusActual).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(15))
	require.True(t, env.sendText(testAdminID, "/2"))
	env.requireKeyboardRemoved(typePrompt.Msg)
	datePrompt := env.lastSent()
	assert.Equal(t, "Когда стартует мероприятие? Введи дату и время в формате DD.MM.YYYY HH:MM:", datePrompt.Msg.Text)
	assert.Equal(t, []string{eventSetupCallbackConfirmCancel}, callbackData(t, datePrompt.Call))

	// 6. Invalid date keeps the state
	env.reset()
	require.True(t, env.sendText(testAdminID, "завтра в 19:30"))
	assert.Equal(t, []string{
		"Неверный формат даты. Пожалуйста, введи дату и время в формате DD.MM.YYYY HH:MM или используй кнопку для отмены.",
	}, env.texts())

	// 7. A valid date is stored on the created event
	env.reset()
	env.mock.ExpectExec(regexp.QuoteMeta(sqlUpdateStart)).
		WithArgs(time.Date(2025, 4, 10, 19, 30, 0, 0, time.UTC), 15).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.True(t, env.sendText(testAdminID, "10.04.2025 19:30"))

	env.requireKeyboardRemoved(datePrompt.Msg)
	assert.Equal(t, []string{
		"Запись о мероприятии '*Go Meetup*' успешно создана с ID: 15 и датой старта: *10.04.2025 19:30*\n\n" +
			"Для редактирования мероприятия используй команду /eventEdit.\n" +
			"Для просмотра всех команд используй команду /help",
	}, env.texts())
	assert.Equal(t, "Markdown", env.lastSent().Call.Params["parse_mode"])
	assert.False(t, env.sendText(testAdminID, "10.04.2025 19:30"), "conversation must be over")
}

func TestEventSetupHandler_CreateEventFailureEndsConversation(t *testing.T) {
	env := newTestEnv(t, NewEventSetupHandler)
	enterSetupName(t, env)
	env.reset()

	env.mock.ExpectQuery(regexp.QuoteMeta(sqlCreateEvent)).
		WithArgs("Go Meetup", constants.EventTypeClubCall, constants.EventStatusActual).
		WillReturnError(errors.New("db down"))
	require.True(t, env.sendText(testAdminID, "/1"))

	assert.Equal(t, []string{"Произошла ошибка при создании записи о мероприятии."}, env.texts())
	assert.False(t, env.sendText(testAdminID, "10.04.2025 19:30"), "conversation must be over")
}

func TestEventSetupHandler_StartDateUpdateFailureEndsConversation(t *testing.T) {
	tests := []struct {
		name  string
		setup func(mock sqlmock.Sqlmock)
	}{
		{"database error", func(mock sqlmock.Sqlmock) {
			mock.ExpectExec(regexp.QuoteMeta(sqlUpdateStart)).WillReturnError(errors.New("db down"))
		}},
		{"event vanished", func(mock sqlmock.Sqlmock) {
			mock.ExpectExec(regexp.QuoteMeta(sqlUpdateStart)).WillReturnResult(sqlmock.NewResult(0, 0))
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t, NewEventSetupHandler)
			chooseSetupType(t, env)
			env.reset()

			tc.setup(env.mock)
			require.True(t, env.sendText(testAdminID, "10.04.2025 19:30"))

			assert.Equal(t, []string{"Произошла ошибка при обновлении даты начала мероприятия."}, env.texts())
			assert.False(t, env.sendText(testAdminID, "10.04.2025 19:30"), "conversation must be over")
		})
	}
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
			})
		}
	}
}
