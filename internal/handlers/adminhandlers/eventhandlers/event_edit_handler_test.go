package eventhandlers

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const editCanceledText = "Операция редактирования мероприятия отменена."

func editTestEvent() repositories.Event {
	return repositories.Event{
		ID: 7, Name: "Go Meetup", Type: "meetup", Status: "actual",
		StartedAt: timePtr(time.Date(2025, 3, 5, 18, 0, 0, 0, time.UTC)),
	}
}

// startEdit runs /eventEdit and returns the event list message
func startEdit(t *testing.T, env *testEnv) gotgbot.Message {
	t.Helper()
	env.expectGetLastEvents(editTestEvent())
	require.True(t, env.sendText(testAdminID, "/eventEdit"))
	return env.lastSent().Msg
}

// selectEventForEdit selects event 7 and returns the "what to edit" prompt
func selectEventForEdit(t *testing.T, env *testEnv) gotgbot.Message {
	t.Helper()
	startEdit(t, env)
	ev := editTestEvent()
	env.expectGetEventByID(7, &ev)
	require.True(t, env.sendText(testAdminID, "/7"))
	return env.lastSent().Msg
}

// chooseEditOption selects event 7 and the given edit option, returning the prompt for the new value
func chooseEditOption(t *testing.T, env *testEnv, option string, event repositories.Event) gotgbot.Message {
	t.Helper()
	selectEventForEdit(t, env)
	env.expectGetEventByID(7, &event)
	require.True(t, env.sendText(testAdminID, option))
	return env.lastSent().Msg
}

func TestEventEditHandler_StartFailures(t *testing.T) {
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
			wantText: "Нет созданных мероприятий для редактирования.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t, NewEventEditHandler)
			tc.setup(env)

			require.True(t, env.sendText(testAdminID, "/eventEdit"))

			assert.Equal(t, []string{tc.wantText}, env.texts())
			assert.False(t, env.sendText(testAdminID, "/7"), "conversation must not be started")
		})
	}
}

func TestEventEditHandler_SelectEvent(t *testing.T) {
	env := newTestEnv(t, NewEventEditHandler)

	listMsg := startEdit(t, env)
	listCall := env.lastSent().Call
	assert.Equal(t, "Markdown", listCall.Params["parse_mode"])
	assert.Contains(t, listMsg.Text, "*Последние 1 мероприятия:*")
	assert.Contains(t, listMsg.Text, "ID /7: *Go Meetup*")
	assert.Contains(t, listMsg.Text, "которое ты хочешь отредактировать")
	assert.Equal(t, []string{eventEditCallbackConfirmCancel}, callbackData(t, listCall))

	// Non-numeric id keeps the state
	env.reset()
	require.True(t, env.sendText(testAdminID, "abc"))
	assert.Equal(t, []string{"Неверный ID. Пожалуйста, введи числовой ID или используй кнопку для отмены."}, env.texts())

	// Unknown id keeps the state
	env.reset()
	env.expectGetEventByID(42, nil)
	require.True(t, env.sendText(testAdminID, "/42"))
	assert.Equal(t, []string{
		"Мероприятие с ID 42 не найдено. Пожалуйста, введи существующий ID или используй кнопку для отмены.",
	}, env.texts())
	assert.Empty(t, env.client.CallsTo("editMessageReplyMarkup"))

	// Existing id asks what to edit
	env.reset()
	ev := editTestEvent()
	env.expectGetEventByID(7, &ev)
	require.True(t, env.sendText(testAdminID, "/7"))

	env.requireKeyboardRemoved(listMsg)
	prompt := env.lastSent()
	assert.Equal(t, "Что ты хочешь отредактировать?\n/1. Название\n/2. Дату начала\n/3. Тип\n\nВведи номер:", prompt.Msg.Text)
	assert.Empty(t, prompt.Call.Params["parse_mode"])
	assert.Equal(t, []string{eventEditCallbackConfirmCancel}, callbackData(t, prompt.Call))
}

func TestEventEditHandler_SelectEditTypeValidation(t *testing.T) {
	env := newTestEnv(t, NewEventEditHandler)
	selectEventForEdit(t, env)

	for _, input := range []string{"abc", "/0", "/4"} {
		env.reset()
		ev := editTestEvent()
		env.expectGetEventByID(7, &ev)
		require.True(t, env.sendText(testAdminID, input))
		assert.Equal(t, []string{
			"Неверный выбор. Пожалуйста, введи число от 1 до 3, или используй кнопку для отмены",
		}, env.texts(), "input %q", input)
		assert.Empty(t, env.client.CallsTo("editMessageReplyMarkup"))
	}

	// The event disappearing ends the conversation
	env.reset()
	env.mock.ExpectQuery(regexp.QuoteMeta(sqlGetEventByID)).WithArgs(7).WillReturnError(errors.New("db down"))
	require.True(t, env.sendText(testAdminID, "/1"))
	assert.Equal(t, []string{"Ошибка при получении мероприятия с ID 7"}, env.texts())
	assert.False(t, env.sendText(testAdminID, "/1"))
}

func TestEventEditHandler_EditName(t *testing.T) {
	env := newTestEnv(t, NewEventEditHandler)

	selectPrompt := selectEventForEdit(t, env)
	env.reset()
	ev := editTestEvent()
	env.expectGetEventByID(7, &ev)
	require.True(t, env.sendText(testAdminID, "/1"))

	env.requireKeyboardRemoved(selectPrompt)
	namePrompt := env.lastSent()
	assert.Equal(t, "Текущее название: *Go Meetup*\n\nВведи новое название:", namePrompt.Msg.Text)
	assert.Equal(t, "Markdown", namePrompt.Call.Params["parse_mode"])
	assert.Equal(t, []string{eventEditCallbackConfirmCancel}, callbackData(t, namePrompt.Call))

	// Blank name keeps the state
	env.reset()
	require.True(t, env.sendText(testAdminID, "   "))
	assert.Equal(t, []string{
		"Название не может быть пустым. Пожалуйста, введи новое название или используй кнопку для отмены:",
	}, env.texts())

	// New name is trimmed and saved
	env.reset()
	env.mock.ExpectExec(regexp.QuoteMeta(sqlUpdateName)).WithArgs("Rust Meetup", 7).WillReturnResult(sqlmock.NewResult(0, 1))
	require.True(t, env.sendText(testAdminID, "  Rust Meetup  "))

	env.requireKeyboardRemoved(namePrompt.Msg)
	assert.Equal(t, []string{
		"Название мероприятия с ID `7` успешно обновлено на *\"Rust Meetup\"* \n\n" +
			"Для продолжения редактирования мероприятия используй команду /eventEdit.\n" +
			"Для просмотра всех команд используй команду /help",
	}, env.texts())
	assert.Equal(t, "Markdown", env.lastSent().Call.Params["parse_mode"])
	assert.False(t, env.sendText(testAdminID, "Another"), "conversation must be over")
}

func TestEventEditHandler_EditStartDate(t *testing.T) {
	tests := []struct {
		name        string
		startedAt   *time.Time
		wantCurrent string
	}{
		{"event with start date", timePtr(time.Date(2025, 3, 5, 18, 0, 0, 0, time.UTC)), "`05.03.2025 18:00`"},
		{"event without start date", nil, "`не задана`"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t, NewEventEditHandler)
			ev := editTestEvent()
			ev.StartedAt = tc.startedAt

			datePrompt := chooseEditOption(t, env, "/2", ev)
			assert.Equal(t,
				"Текущая дата старта: "+tc.wantCurrent+" (UTC)\nВведи новую дату и время в формате DD.MM.YYYY HH:MM (UTC):",
				datePrompt.Text)

			// Wrong format keeps the state
			env.reset()
			require.True(t, env.sendText(testAdminID, "2025-04-10 19:30"))
			assert.Equal(t, []string{
				"Неверный формат даты. Пожалуйста, введи дату и время в формате *DD.MM.YYYY HH:MM* (UTC) или используй кнопку для отмены.",
			}, env.texts())
			assert.Equal(t, "Markdown", env.lastSent().Call.Params["parse_mode"])

			// Valid date is saved as UTC
			env.reset()
			newStart := time.Date(2025, 4, 10, 19, 30, 0, 0, time.UTC)
			env.mock.ExpectExec(regexp.QuoteMeta(sqlUpdateStart)).WithArgs(newStart, 7).WillReturnResult(sqlmock.NewResult(0, 1))
			require.True(t, env.sendText(testAdminID, " 10.04.2025 19:30 "))

			env.requireKeyboardRemoved(datePrompt)
			assert.Equal(t, []string{
				"Дата начала мероприятия с ID 7 успешно обновлена на *10.04.2025 19:30 UTC* \n\n" +
					"Для продолжения редактирования мероприятия используй команду /eventEdit.\n" +
					"Для просмотра всех команд используй команду /help",
			}, env.texts())
			assert.False(t, env.sendText(testAdminID, "10.04.2025 19:30"), "conversation must be over")
		})
	}
}

func TestEventEditHandler_EditTypePromptAndValidation(t *testing.T) {
	env := newTestEnv(t, NewEventEditHandler)

	typePrompt := chooseEditOption(t, env, "/3", editTestEvent())
	assert.Equal(t,
		"Текущий тип: *meetup*\n\nДоступные типы:\n"+
			"/1. 💬 club-call\n/2. 🎙 meetup\n/3. ⚙️ workshop\n/4. 📚 reading-club\n/5. 👥 conference\n"+
			"\nВведи новый тип или его номер:",
		typePrompt.Text)

	invalidTypeText := "Неверный тип мероприятия. Допустимые типы: club-call, meetup, workshop, reading-club, conference"
	tests := []struct {
		input    string
		wantText string
	}{
		{"/", "Тип не может быть пустым. Пожалуйста, введи новый тип или его номер, или используй кнопку для отмены:"},
		{"party", invalidTypeText},
		{"/0", invalidTypeText},
		{"/6", invalidTypeText},
		{"Meetup", invalidTypeText},
	}
	for _, tc := range tests {
		env.reset()
		require.True(t, env.sendText(testAdminID, tc.input))
		assert.Equal(t, []string{tc.wantText}, env.texts(), "input %q", tc.input)
		assert.Empty(t, env.client.CallsTo("editMessageReplyMarkup"), "input %q", tc.input)
	}
}

func TestEventEditHandler_EditTypeSaves(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType constants.EventType
		wantText string
	}{
		{"by number", "/3", constants.EventTypeWorkshop, "⚙️ *'workshop'*"},
		{"by name", "reading-club", constants.EventTypeReadingClub, "📚 *'reading-club'*"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t, NewEventEditHandler)
			typePrompt := chooseEditOption(t, env, "/3", editTestEvent())
			env.reset()

			env.mock.ExpectExec(regexp.QuoteMeta(sqlUpdateType)).WithArgs(string(tc.wantType), 7).
				WillReturnResult(sqlmock.NewResult(0, 1))
			require.True(t, env.sendText(testAdminID, tc.input))

			env.requireKeyboardRemoved(typePrompt)
			assert.Equal(t, []string{
				"Тип мероприятия с ID 7 успешно обновлен на " + tc.wantText + " \n\n" +
					"Для продолжения редактирования мероприятия используй команду /eventEdit.\n" +
					"Для просмотра всех команд используй команду /help",
			}, env.texts())
			assert.Equal(t, "Markdown", env.lastSent().Call.Params["parse_mode"])
			assert.False(t, env.sendText(testAdminID, "/2"), "conversation must be over")
		})
	}
}

func TestEventEditHandler_UpdateFailures(t *testing.T) {
	tests := []struct {
		name     string
		option   string
		input    string
		setup    func(mock sqlmock.Sqlmock)
		wantText string
	}{
		{
			name:   "name update fails",
			option: "/1",
			input:  "New name",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(sqlUpdateName)).WillReturnError(errors.New("db down"))
			},
			wantText: "Произошла ошибка при обновлении названия мероприятия.",
		},
		{
			name:   "name update hits no rows",
			option: "/1",
			input:  "New name",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(sqlUpdateName)).WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantText: "Произошла ошибка при обновлении названия мероприятия.",
		},
		{
			name:   "start date update fails",
			option: "/2",
			input:  "10.04.2025 19:30",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(sqlUpdateStart)).WillReturnError(errors.New("db down"))
			},
			wantText: "Произошла ошибка при обновлении даты начала мероприятия.",
		},
		{
			name:   "type update fails",
			option: "/3",
			input:  "/1",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(sqlUpdateType)).WillReturnError(errors.New("db down"))
			},
			wantText: "Произошла ошибка при обновлении типа мероприятия.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t, NewEventEditHandler)
			chooseEditOption(t, env, tc.option, editTestEvent())
			env.reset()

			tc.setup(env.mock)
			require.True(t, env.sendText(testAdminID, tc.input))

			assert.Equal(t, []string{tc.wantText}, env.texts())
			assert.False(t, env.sendText(testAdminID, tc.input), "conversation must be over")
		})
	}
}

func TestEventEditHandler_Cancel(t *testing.T) {
	states := []struct {
		name       string
		reachState func(t *testing.T, env *testEnv) gotgbot.Message
	}{
		{"select event", startEdit},
		{"ask edit type", selectEventForEdit},
		{"edit name", func(t *testing.T, env *testEnv) gotgbot.Message {
			return chooseEditOption(t, env, "/1", editTestEvent())
		}},
		{"edit start date", func(t *testing.T, env *testEnv) gotgbot.Message {
			return chooseEditOption(t, env, "/2", editTestEvent())
		}},
		{"edit type", func(t *testing.T, env *testEnv) gotgbot.Message {
			return chooseEditOption(t, env, "/3", editTestEvent())
		}},
	}

	for _, state := range states {
		for _, viaButton := range []bool{true, false} {
			name := state.name + "/command"
			if viaButton {
				name = state.name + "/button"
			}
			t.Run(name, func(t *testing.T) {
				env := newTestEnv(t, NewEventEditHandler)
				keyboardMsg := state.reachState(t, env)
				env.reset()

				if viaButton {
					require.True(t, env.click(testAdminID, eventEditCallbackConfirmCancel))
					assert.Len(t, env.client.CallsTo("answerCallbackQuery"), 1)
				} else {
					require.True(t, env.sendText(testAdminID, "/cancel"))
				}

				assert.Equal(t, []string{editCanceledText}, env.texts())
				env.requireKeyboardRemoved(keyboardMsg)
				assert.False(t, env.sendText(testAdminID, "/1"), "conversation must be over")
			})
		}
	}
}
