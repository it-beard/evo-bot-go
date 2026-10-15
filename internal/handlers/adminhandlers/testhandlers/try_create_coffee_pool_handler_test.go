package testhandlers

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sqlCreatePoll = "INSERT INTO random_coffee_polls (message_id, week_start_date, telegram_poll_id, created_at) " +
		"VALUES ($1, $2, $3, $4) RETURNING id"

	coffeePoolMenuText = "<b>Запуск опроса по кофейным встречам</b>\n\n⚠️ ЭТА КОМАНДА НУЖНА ДЛЯ ТЕСТИРОВАНИЯ ФУНКЦИОНАЛА!" +
		"\n\nТочно запустить новый опрос по кофейным встречам?" +
		"\n\nОпрос будет отправлен в топик \"Random Coffee\" (ID: 33)."
	coffeePoolProcessingText = "<b>Запуск опроса по кофейным встречам</b>\n\n⏳ Создание опроса..."
	coffeePoolSuccessText    = "<b>Запуск опроса по кофейным встречам</b>\n\n✅ Опрос по кофейным встречам успешно создан и отправлен в группу!"
	coffeePoolCanceledText   = "Создание опроса по кофейным встречам отменено."

	testPollMessageID int64 = 4242
	testTelegramPoll        = "poll-xyz"
)

func newCoffeePoolEnv(t *testing.T) (*testEnv, sqlmock.Sqlmock) {
	env := newTestEnv(t)
	db, mock := newRepositoriesDB(t)
	env.handler = NewTryCreateCoffeePoolHandler(env.cfg, env.sender, env.permissions, newCoffeeService(env, db))
	env.client.OnJSON("sendPoll", gotgbot.Message{
		MessageId: testPollMessageID,
		Chat:      gotgbot.Chat{Id: fullChatID(), Type: testutils.SuperGroupChatType},
		Date:      1700000000,
		Poll:      &gotgbot.Poll{Id: testTelegramPoll, Question: "q", Type: "regular"},
	})
	return env, mock
}

func expectPollSaved(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(regexp.QuoteMeta(sqlCreatePoll)).
		WithArgs(testPollMessageID, futureMonday{}, testTelegramPoll, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
}

// openCoffeePoolMenu runs the command and returns the confirmation menu
func openCoffeePoolMenu(t *testing.T, env *testEnv) gotgbot.Message {
	t.Helper()
	require.True(t, env.sendText(testAdminID, "/tryCreateCoffeePool"))
	return env.lastSent().Msg
}

func TestTryCreateCoffeePoolHandler_ConfirmCreatesPinsAndSavesPoll(t *testing.T) {
	env, mock := newCoffeePoolEnv(t)

	// 1. Confirmation menu
	menu := openCoffeePoolMenu(t, env)
	menuCall := env.lastSent().Call
	assert.Equal(t, testAdminID, menu.Chat.Id)
	assert.Equal(t, coffeePoolMenuText, menu.Text)
	assert.Equal(t, "HTML", menuCall.Params["parse_mode"])
	assert.Equal(t,
		[]string{constants.TryCreateCoffeePoolConfirmCallback, constants.TryCreateCoffeePoolCancelCallback},
		callbackData(t, menuCall))

	// 2. Stray text while waiting for a button is deleted
	env.reset()
	stray := testutils.NewPrivateMessage(testAdminID, "hello")
	require.True(t, env.dispatch(testutils.MessageUpdate(stray)))
	assert.Equal(t, [][2]int64{msgRef(*stray)}, env.deletedMessages())
	assert.Empty(t, env.texts())

	// 3. Confirmation sends the announcement and the poll, pins the poll and stores it
	env.reset()
	expectPollSaved(mock)
	require.True(t, env.click(testAdminID, constants.TryCreateCoffeePoolConfirmCallback))

	assert.Equal(t, [][2]int64{msgRef(menu)}, env.deletedMessages(), "menu is removed")

	sent := env.sentMessages()
	require.Len(t, sent, 2)
	processing := sent[0]
	assert.Equal(t, testAdminID, processing.Msg.Chat.Id)
	assert.Equal(t, coffeePoolProcessingText, processing.Msg.Text)
	assert.Equal(t, "HTML", processing.Call.Params["parse_mode"])

	intro := sent[1]
	assert.Equal(t, fullChatID(), intro.Msg.Chat.Id)
	assert.Equal(t, int64(testRandomCoffeeTopicID), intro.Call.Int64("message_thread_id"))
	assert.Equal(t, "HTML", intro.Call.Params["parse_mode"])
	assert.Contains(t, intro.Msg.Text, `<a href="https://telegram.me/c/2069889012/33/34">правила участия</a>`)

	pollCall, ok := env.client.LastCallTo("sendPoll")
	require.True(t, ok)
	assert.Equal(t, fullChatID(), pollCall.Int64("chat_id"))
	assert.Equal(t, int64(testRandomCoffeeTopicID), pollCall.Int64("message_thread_id"))
	assert.Equal(t, "Будешь участвовать в Random Coffee на следующей неделе? ☕️", pollCall.Params["question"])
	assert.Equal(t, "false", pollCall.Params["is_anonymous"])
	var options []gotgbot.InputPollOption
	require.NoError(t, pollCall.DecodeJSON("options", &options))
	require.Len(t, options, 2)
	assert.Equal(t, "Да! 🤗", options[0].Text)
	assert.Equal(t, "Не в этот раз 💁🏽", options[1].Text)

	pin, ok := env.client.LastCallTo("pinChatMessage")
	require.True(t, ok)
	assert.Equal(t, fullChatID(), pin.Int64("chat_id"))
	assert.Equal(t, testPollMessageID, pin.Int64("message_id"))
	assert.Equal(t, "false", pin.Params["disable_notification"])

	edit, ok := env.client.LastCallTo("editMessageText")
	require.True(t, ok)
	assert.Equal(t, testAdminID, edit.Int64("chat_id"))
	assert.Equal(t, processing.Msg.MessageId, edit.Int64("message_id"))
	assert.Equal(t, "HTML", edit.Params["parse_mode"])
	assert.Equal(t, coffeePoolSuccessText, edit.Params["text"])

	// 4. The conversation is over
	assert.False(t, env.click(testAdminID, constants.TryCreateCoffeePoolConfirmCallback))
	assert.False(t, env.sendText(testAdminID, "hello"))
}

func TestTryCreateCoffeePoolHandler_PollCreationFailures(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(env *testEnv, mock sqlmock.Sqlmock)
		wantInError string
	}{
		{
			name:        "random coffee topic is not configured",
			setup:       func(env *testEnv, _ sqlmock.Sqlmock) { env.cfg.RandomCoffeeTopicID = 0 },
			wantInError: "RandomCoffeeTopicID is not configured",
		},
		{
			name: "poll cannot be sent",
			setup: func(env *testEnv, _ sqlmock.Sqlmock) {
				env.client.OnError("sendPoll", "Bad Request: not enough rights to send polls")
			},
			wantInError: "not enough rights to send polls",
		},
		{
			name: "poll cannot be pinned",
			setup: func(env *testEnv, _ sqlmock.Sqlmock) {
				env.client.OnError("pinChatMessage", "Bad Request: not enough rights to pin")
			},
			wantInError: "Failed to pin poll",
		},
		{
			name: "poll cannot be saved",
			setup: func(_ *testEnv, mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(sqlCreatePoll)).WillReturnError(errors.New("db down"))
			},
			wantInError: "db down",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env, mock := newCoffeePoolEnv(t)
			openCoffeePoolMenu(t, env)
			env.reset()
			tc.setup(env, mock)

			handled, err := env.tryDispatch(testutils.CallbackUpdate(testAdminID, constants.TryCreateCoffeePoolConfirmCallback))

			assert.True(t, handled)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to create coffee poll")
			assert.Contains(t, err.Error(), tc.wantInError)

			processing := env.sentMessages()[0]
			assert.Equal(t, coffeePoolProcessingText, processing.Msg.Text)
			edit, ok := env.client.LastCallTo("editMessageText")
			require.True(t, ok)
			assert.Equal(t, processing.Msg.MessageId, edit.Int64("message_id"))
			assert.Equal(t, "HTML", edit.Params["parse_mode"])
			assert.True(t, strings.HasPrefix(edit.Params["text"],
				"<b>Запуск опроса по кофейным встречам</b>\n\n❌ Ошибка при создании опроса:\n<code>"), edit.Params["text"])
			assert.Contains(t, edit.Params["text"], tc.wantInError)
		})
	}
}

func TestTryCreateCoffeePoolHandler_TelegramFailuresAreReturned(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(env *testEnv, mock sqlmock.Sqlmock)
		wantInError string
	}{
		{
			name: "processing message cannot be sent",
			setup: func(env *testEnv, _ sqlmock.Sqlmock) {
				env.failSendWhen(func(call testutils.APICall) bool {
					return call.Params["text"] == coffeePoolProcessingText
				})
			},
			wantInError: "failed to send processing message",
		},
		{
			name: "error message cannot be shown",
			setup: func(env *testEnv, _ sqlmock.Sqlmock) {
				env.cfg.RandomCoffeeTopicID = 0
				env.client.OnError("editMessageText", "Bad Request: message to edit not found")
			},
			wantInError: "failed to edit error message",
		},
		{
			name: "success message cannot be shown",
			setup: func(env *testEnv, mock sqlmock.Sqlmock) {
				expectPollSaved(mock)
				env.client.OnError("editMessageText", "Bad Request: message to edit not found")
			},
			wantInError: "failed to update success message",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env, mock := newCoffeePoolEnv(t)
			openCoffeePoolMenu(t, env)
			env.reset()
			tc.setup(env, mock)

			handled, err := env.tryDispatch(testutils.CallbackUpdate(testAdminID, constants.TryCreateCoffeePoolConfirmCallback))

			assert.True(t, handled)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantInError)
		})
	}
}

func TestTryCreateCoffeePoolHandler_MenuSendFailureIsReturned(t *testing.T) {
	env, _ := newCoffeePoolEnv(t)
	env.failSendWhen(func(call testutils.APICall) bool { return call.Params["text"] == coffeePoolMenuText })

	handled, err := env.tryDispatch(testutils.MessageUpdate(testutils.NewPrivateMessage(testAdminID, "/tryCreateCoffeePool")))

	assert.True(t, handled)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send message in showConfirmationMenu")
	assert.False(t, env.click(testAdminID, constants.TryCreateCoffeePoolConfirmCallback), "conversation must not be started")
}

func TestTryCreateCoffeePoolHandler_Cancel(t *testing.T) {
	tests := []struct {
		name       string
		cancel     func(env *testEnv) bool
		isCallback bool
	}{
		{"cancel button", func(env *testEnv) bool { return env.click(testAdminID, constants.TryCreateCoffeePoolCancelCallback) }, true},
		{"cancel command", func(env *testEnv) bool { return env.sendText(testAdminID, "/cancel") }, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env, _ := newCoffeePoolEnv(t)
			menu := openCoffeePoolMenu(t, env)
			env.reset()

			require.True(t, tc.cancel(env))

			if tc.isCallback {
				assert.Len(t, env.answeredCallbacks(), 1)
			} else {
				assert.Empty(t, env.answeredCallbacks())
			}
			assert.Equal(t, [][2]int64{msgRef(menu)}, env.deletedMessages())
			assert.Equal(t, []string{coffeePoolCanceledText}, env.texts())
			assert.Equal(t, testAdminID, env.lastSent().Msg.Chat.Id)
			assert.Empty(t, env.client.CallsTo("sendPoll"))
			assert.False(t, env.click(testAdminID, constants.TryCreateCoffeePoolConfirmCallback), "conversation must be over")
		})
	}
}

func TestTryCreateCoffeePoolHandler_CancelSendFailureIsReturned(t *testing.T) {
	env, _ := newCoffeePoolEnv(t)
	openCoffeePoolMenu(t, env)
	env.failSendWhen(func(call testutils.APICall) bool { return call.Params["text"] == coffeePoolCanceledText })

	handled, err := env.tryDispatch(testutils.CallbackUpdate(testAdminID, constants.TryCreateCoffeePoolCancelCallback))

	assert.True(t, handled)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send cancel message")
}

func TestTryCreateCoffeePoolHandler_AccessIsRestricted(t *testing.T) {
	t.Run("non-admin", func(t *testing.T) {
		env, _ := newCoffeePoolEnv(t)

		require.True(t, env.sendText(testUserID, "/tryCreateCoffeePool"))

		assert.Equal(t, []string{adminOnlyText}, env.texts())
		assert.Len(t, env.client.CallsTo("getChatMember"), 1)
		assert.False(t, env.click(testUserID, constants.TryCreateCoffeePoolConfirmCallback), "conversation must not be started")
		assert.Empty(t, env.client.CallsTo("sendPoll"))
	})

	for _, sender := range groupChatSenders {
		t.Run("group chat by "+sender.name, func(t *testing.T) {
			env, _ := newCoffeePoolEnv(t)

			require.True(t, env.sendGroupText(sender.id, "/tryCreateCoffeePool"))

			env.requirePrivateOnlyNotice(sender.id)
			assert.False(t, env.sendGroupText(sender.id, "hello"), "conversation must not be started")
		})
	}
}

func TestTryCreateCoffeePoolHandler_ButtonPressesAreAnswered(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		setup   func(env *testEnv, mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name:  "confirm",
			data:  constants.TryCreateCoffeePoolConfirmCallback,
			setup: func(_ *testEnv, mock sqlmock.Sqlmock) { expectPollSaved(mock) },
		},
		{
			name:    "confirm when the poll cannot be created",
			data:    constants.TryCreateCoffeePoolConfirmCallback,
			setup:   func(env *testEnv, _ sqlmock.Sqlmock) { env.cfg.RandomCoffeeTopicID = 0 },
			wantErr: true,
		},
		{
			name: "confirm when the processing message cannot be sent",
			data: constants.TryCreateCoffeePoolConfirmCallback,
			setup: func(env *testEnv, _ sqlmock.Sqlmock) {
				env.failSendWhen(func(call testutils.APICall) bool { return call.Params["text"] == coffeePoolProcessingText })
			},
			wantErr: true,
		},
		{
			name: "cancel",
			data: constants.TryCreateCoffeePoolCancelCallback,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env, mock := newCoffeePoolEnv(t)
			openCoffeePoolMenu(t, env)
			env.reset()
			if tc.setup != nil {
				tc.setup(env, mock)
			}

			update := testutils.CallbackUpdate(testAdminID, tc.data)
			handled, err := env.tryDispatch(update)

			assert.True(t, handled)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, []string{update.CallbackQuery.Id}, env.answeredCallbacks(), "the button press is answered exactly once")
		})
	}
}
