package testhandlers

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sqlLatestPoll = "SELECT id, message_id, week_start_date, telegram_poll_id, created_at FROM random_coffee_polls " +
		"ORDER BY week_start_date DESC, id DESC LIMIT 1"
	sqlParticipants = "SELECT u.id, u.tg_id, u.firstname, u.lastname, u.tg_username FROM users u " +
		"JOIN random_coffee_participants rpc ON u.id = rpc.user_id WHERE rpc.poll_id = $1 AND rpc.is_participating = TRUE"
	sqlUserByTgID    = "FROM users WHERE tg_id = $1"
	sqlPairsHistory  = "SELECT p.user1_id, p.user2_id, poll.id as poll_id FROM random_coffee_pairs p"
	sqlCreatePair    = "INSERT INTO random_coffee_pairs (poll_id, user1_id, user2_id) VALUES ($1, $2, $3)"
	sqlProfileByUser = "SELECT id, user_id, bio, published_message_id, created_at, updated_at FROM profiles WHERE user_id = $1"

	pairsHeader       = "<b>Генерация пар для Random Coffee</b>"
	pairsCanceledText = "Генерация пар для Random Coffee отменена."
	pairsSuccessText  = pairsHeader + "\n\n✅ Пары успешно сгенерированы и отправлены в супергруппу!"

	testPollID             int64 = 9
	testPairsPollMessageID int64 = 555
)

var testWeekStart = time.Date(2025, 4, 7, 0, 0, 0, 0, time.UTC)

type participant struct {
	id        int
	tgID      int64
	firstname string
	username  string
}

var (
	alice = participant{id: 1, tgID: 111, firstname: "Alice", username: "alice"}
	bob   = participant{id: 2, tgID: 222, firstname: "Bob", username: "bob"}
)

func newPairsEnv(t *testing.T) (*testEnv, sqlmock.Sqlmock) {
	env := newTestEnv(t)
	db, mock := newRepositoriesDB(t)
	env.handler = NewTryGenerateCoffeePairsHandler(
		env.cfg, env.permissions, env.sender, db.polls, db.participants, db.profiles, newCoffeeService(env, db),
	)
	env.client.OnJSON("stopPoll", gotgbot.Poll{Id: "poll-abc", Question: "q", Type: "regular", IsClosed: true})
	return env, mock
}

func expectLatestPoll(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(regexp.QuoteMeta(sqlLatestPoll)).WillReturnRows(
		sqlmock.NewRows([]string{"id", "message_id", "week_start_date", "telegram_poll_id", "created_at"}).
			AddRow(testPollID, testPairsPollMessageID, testWeekStart, "poll-abc", testWeekStart))
}

func expectParticipants(mock sqlmock.Sqlmock, people ...participant) {
	rows := sqlmock.NewRows([]string{"id", "tg_id", "firstname", "lastname", "tg_username"})
	for _, p := range people {
		rows.AddRow(p.id, p.tgID, p.firstname, "", p.username)
	}
	mock.ExpectQuery(regexp.QuoteMeta(sqlParticipants)).WithArgs(testPollID).WillReturnRows(rows)
}

func expectProfile(mock sqlmock.Sqlmock, userID int, publishedMessageID any) {
	mock.ExpectQuery(regexp.QuoteMeta(sqlProfileByUser)).WithArgs(userID).WillReturnRows(
		sqlmock.NewRows([]string{"id", "user_id", "bio", "published_message_id", "created_at", "updated_at"}).
			AddRow(100+userID, userID, "bio", publishedMessageID, testWeekStart, testWeekStart))
}

// expectSuccessfulPairing sets up the DB for pairing alice and bob; profile lookups happen in random order
func expectSuccessfulPairing(mock sqlmock.Sqlmock) {
	mock.MatchExpectationsInOrder(false)
	expectLatestPoll(mock)
	expectParticipants(mock, alice, bob)
	// Users are unknown in the users table, so their Telegram data is not synchronised
	userColumns := []string{"id", "tg_id", "firstname", "lastname", "tg_username", "score", "has_coffee_ban", "is_club_member", "created_at", "updated_at"}
	mock.ExpectQuery(regexp.QuoteMeta(sqlUserByTgID)).WithArgs(alice.tgID).WillReturnRows(sqlmock.NewRows(userColumns))
	mock.ExpectQuery(regexp.QuoteMeta(sqlUserByTgID)).WithArgs(bob.tgID).WillReturnRows(sqlmock.NewRows(userColumns))
	// No pair history: last 4 polls for users 1 and 2
	mock.ExpectQuery(regexp.QuoteMeta(sqlPairsHistory)).WithArgs(4, alice.id, bob.id).
		WillReturnRows(sqlmock.NewRows([]string{"user1_id", "user2_id", "poll_id"}))
	mock.ExpectExec(regexp.QuoteMeta(sqlCreatePair)).WithArgs(int(testPollID), alice.id, bob.id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	expectProfile(mock, alice.id, int64(777))
	expectProfile(mock, bob.id, nil)
}

// openPairsMenu runs the command with the given participants and returns the confirmation menu
func openPairsMenu(t *testing.T, env *testEnv, mock sqlmock.Sqlmock, people ...participant) gotgbot.Message {
	t.Helper()
	expectLatestPoll(mock)
	expectParticipants(mock, people...)
	require.True(t, env.sendText(testAdminID, "/tryGenerateCoffeePairs"))
	return env.lastSent().Msg
}

func pairsMenuText(participants int) string {
	return pairsHeader + "\n\n⚠️ ЭТА КОМАНДА НУЖНА ДЛЯ ТЕСТИРОВАНИЯ ФУНКЦИОНАЛА!" +
		"\n\nТочно сгенерировать пары для текущего опроса?" +
		"\n\n📊 Опрос: неделя 2025-04-07" +
		"\n👥 Участников: " + strconv.Itoa(participants) +
		"\n\n⚠️ Пары будут отправлены в сообщество."
}

func TestTryGenerateCoffeePairsHandler_ConfirmSendsAndPinsPairs(t *testing.T) {
	env, mock := newPairsEnv(t)

	// 1. Confirmation menu with poll info
	menu := openPairsMenu(t, env, mock, alice, bob)
	menuCall := env.lastSent().Call
	assert.Equal(t, testAdminID, menu.Chat.Id)
	assert.Equal(t, pairsMenuText(2), menu.Text)
	assert.Equal(t, "HTML", menuCall.Params["parse_mode"])
	assert.Equal(t,
		[]string{constants.TryGenerateCoffeePairsConfirmCallback, constants.TryGenerateCoffeePairsCancelCallback},
		callbackData(t, menuCall))

	// 2. Stray text is deleted while waiting for a button
	env.reset()
	stray := testutils.NewPrivateMessage(testAdminID, "hello")
	require.True(t, env.dispatch(testutils.MessageUpdate(stray)))
	assert.Equal(t, [][2]int64{msgRef(*stray)}, env.deletedMessages())
	assert.Empty(t, env.texts())

	// 3. Confirmation stops the poll, pairs participants and posts the pairs to the group
	env.reset()
	expectSuccessfulPairing(mock)
	require.True(t, env.click(testAdminID, constants.TryGenerateCoffeePairsConfirmCallback))

	stop, ok := env.client.LastCallTo("stopPoll")
	require.True(t, ok)
	assert.Equal(t, fullChatID(), stop.Int64("chat_id"))
	assert.Equal(t, testPairsPollMessageID, stop.Int64("message_id"))

	sent := env.sentMessages()
	require.Len(t, sent, 3)
	processing, pairs, success := sent[0], sent[1], sent[2]

	assert.Equal(t, testAdminID, processing.Msg.Chat.Id)
	assert.Equal(t, pairsHeader+"\n\n⏳ Генерация пар...", processing.Msg.Text)

	assert.Equal(t, fullChatID(), pairs.Msg.Chat.Id)
	assert.Equal(t, int64(testRandomCoffeeTopicID), pairs.Call.Int64("message_thread_id"))
	assert.Equal(t, "HTML", pairs.Call.Params["parse_mode"])
	aliceDisplay := `@alice <i>(<a href="https://telegram.me/c/2069889012/44/777">профиль</a>)</i>`
	pairsText := func(first, second string) string {
		return "☕️ Пары для рандом кофе ➪ <b><i>неделя Mon, Apr 7</i></b>:\n\n" +
			"➪ " + first + " x " + second + "\n" +
			"\n🗓 День, время и формат встречи вы выбираете сами. Просто напиши своей паре в личку, когда и в каком формате тебе удобно встретиться."
	}
	assert.Contains(t, []string{pairsText(aliceDisplay, "@bob"), pairsText("@bob", aliceDisplay)}, pairs.Msg.Text)

	pin, ok := env.client.LastCallTo("pinChatMessage")
	require.True(t, ok)
	assert.Equal(t, fullChatID(), pin.Int64("chat_id"))
	assert.Equal(t, pairs.Msg.MessageId, pin.Int64("message_id"))
	assert.Equal(t, "true", pin.Params["disable_notification"], "pairs are pinned silently")

	assert.Equal(t, testAdminID, success.Msg.Chat.Id)
	assert.Equal(t, pairsSuccessText, success.Msg.Text)
	assert.Equal(t, "HTML", success.Call.Params["parse_mode"])

	assert.Equal(t, [][2]int64{msgRef(menu), msgRef(processing.Msg)}, env.deletedMessages())

	// 4. The conversation is over
	assert.False(t, env.click(testAdminID, constants.TryGenerateCoffeePairsConfirmCallback))
}

func TestTryGenerateCoffeePairsHandler_FailureOffersBackAndCancel(t *testing.T) {
	env, mock := newPairsEnv(t)
	menu := openPairsMenu(t, env, mock, alice)
	assert.Equal(t, pairsMenuText(1), menu.Text)

	// 1. Pairing fails with a single participant: an error with Back/Cancel buttons is shown
	env.reset()
	expectLatestPoll(mock)
	expectParticipants(mock, alice)
	require.True(t, env.click(testAdminID, constants.TryGenerateCoffeePairsConfirmCallback))

	sent := env.sentMessages()
	require.Len(t, sent, 2)
	processing, failure := sent[0], sent[1]
	assert.Equal(t, pairsHeader+"\n\n❌ Ошибка при генерации пар:"+
		"\n<code>недостаточно участников для создания пар (нужно минимум 2, зарегистрировалось 1)</code>"+
		"\n\nВернуться к подтверждению?", failure.Msg.Text)
	assert.Equal(t, "HTML", failure.Call.Params["parse_mode"])
	assert.Equal(t,
		[]string{constants.TryGenerateCoffeePairsBackCallback, constants.TryGenerateCoffeePairsCancelCallback},
		callbackData(t, failure.Call))
	assert.Equal(t, [][2]int64{msgRef(menu), msgRef(processing.Msg)}, env.deletedMessages())
	assert.Empty(t, env.client.CallsTo("pinChatMessage"))

	// 2. Back shows a fresh confirmation menu
	env.reset()
	expectLatestPoll(mock)
	expectParticipants(mock, alice, bob)
	require.True(t, env.click(testAdminID, constants.TryGenerateCoffeePairsBackCallback))

	newMenu := env.lastSent()
	assert.Equal(t, pairsMenuText(2), newMenu.Msg.Text)
	assert.Equal(t,
		[]string{constants.TryGenerateCoffeePairsConfirmCallback, constants.TryGenerateCoffeePairsCancelCallback},
		callbackData(t, newMenu.Call))
	assert.Equal(t, [][2]int64{msgRef(failure.Msg)}, env.deletedMessages())

	// 3. Cancel ends the conversation
	env.reset()
	require.True(t, env.click(testAdminID, constants.TryGenerateCoffeePairsCancelCallback))
	assert.Equal(t, []string{pairsCanceledText}, env.texts())
	assert.Equal(t, [][2]int64{msgRef(newMenu.Msg)}, env.deletedMessages())
	assert.False(t, env.click(testAdminID, constants.TryGenerateCoffeePairsConfirmCallback))
}

func TestTryGenerateCoffeePairsHandler_StartFailures(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(mock sqlmock.Sqlmock)
		wantText string
	}{
		{
			name: "poll lookup fails",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(sqlLatestPoll)).WillReturnError(errors.New("db down"))
			},
			wantText: "Ошибка при получении информации об опросе.",
		},
		{
			name: "no poll yet",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(sqlLatestPoll)).
					WillReturnRows(sqlmock.NewRows([]string{"id", "message_id", "week_start_date", "telegram_poll_id", "created_at"}))
			},
			wantText: "Опрос для рандом кофе не найден.",
		},
		{
			name: "participants lookup fails",
			setup: func(mock sqlmock.Sqlmock) {
				expectLatestPoll(mock)
				mock.ExpectQuery(regexp.QuoteMeta(sqlParticipants)).WillReturnError(errors.New("db down"))
			},
			wantText: "Ошибка при получении списка участников.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env, mock := newPairsEnv(t)
			tc.setup(mock)

			require.True(t, env.sendText(testAdminID, "/tryGenerateCoffeePairs"))

			assert.Equal(t, []string{tc.wantText}, env.texts())
			assert.False(t, env.click(testAdminID, constants.TryGenerateCoffeePairsConfirmCallback), "conversation must not be started")
		})
	}
}

func TestTryGenerateCoffeePairsHandler_TelegramFailuresAreReturned(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(env *testEnv, mock sqlmock.Sqlmock)
		wantInError string
	}{
		{
			name: "processing message cannot be sent",
			setup: func(env *testEnv, _ sqlmock.Sqlmock) {
				env.failSendWhen(func(call testutils.APICall) bool { return strings.Contains(call.Params["text"], "⏳") })
			},
			wantInError: "failed to send processing message",
		},
		{
			name: "error message cannot be sent",
			setup: func(env *testEnv, mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(sqlLatestPoll)).WillReturnError(errors.New("db down"))
				env.failSendWhen(func(call testutils.APICall) bool { return strings.Contains(call.Params["text"], "❌") })
			},
			wantInError: "failed to send error message",
		},
		{
			name: "success message cannot be sent",
			setup: func(env *testEnv, mock sqlmock.Sqlmock) {
				expectSuccessfulPairing(mock)
				env.failSendWhen(func(call testutils.APICall) bool { return call.Params["text"] == pairsSuccessText })
			},
			wantInError: "failed to send success message",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env, mock := newPairsEnv(t)
			openPairsMenu(t, env, mock, alice, bob)
			env.reset()
			tc.setup(env, mock)

			handled, err := env.tryDispatch(testutils.CallbackUpdate(testAdminID, constants.TryGenerateCoffeePairsConfirmCallback))

			assert.True(t, handled)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantInError)
		})
	}
}

func TestTryGenerateCoffeePairsHandler_MenuSendFailureIsReturned(t *testing.T) {
	env, mock := newPairsEnv(t)
	expectLatestPoll(mock)
	expectParticipants(mock, alice, bob)
	env.failSendWhen(func(call testutils.APICall) bool { return call.Int64("chat_id") == testAdminID })

	handled, err := env.tryDispatch(testutils.MessageUpdate(testutils.NewPrivateMessage(testAdminID, "/tryGenerateCoffeePairs")))

	assert.True(t, handled)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send message in showConfirmationMenu")
}

func TestTryGenerateCoffeePairsHandler_CancelCommand(t *testing.T) {
	env, mock := newPairsEnv(t)
	menu := openPairsMenu(t, env, mock, alice, bob)
	env.reset()

	require.True(t, env.sendText(testAdminID, "/cancel"))

	assert.Equal(t, []string{pairsCanceledText}, env.texts())
	assert.Equal(t, [][2]int64{msgRef(menu)}, env.deletedMessages())
	assert.Empty(t, env.answeredCallbacks(), "a command has no button press to answer")
	assert.Empty(t, env.client.CallsTo("stopPoll"))
	assert.False(t, env.click(testAdminID, constants.TryGenerateCoffeePairsConfirmCallback), "conversation must be over")
}

func TestTryGenerateCoffeePairsHandler_CancelSendFailureIsReturned(t *testing.T) {
	env, mock := newPairsEnv(t)
	openPairsMenu(t, env, mock, alice, bob)
	env.failSendWhen(func(call testutils.APICall) bool { return call.Params["text"] == pairsCanceledText })

	handled, err := env.tryDispatch(testutils.CallbackUpdate(testAdminID, constants.TryGenerateCoffeePairsCancelCallback))

	assert.True(t, handled)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send cancel message")
}

func TestTryGenerateCoffeePairsHandler_AccessIsRestricted(t *testing.T) {
	t.Run("non-admin", func(t *testing.T) {
		env, _ := newPairsEnv(t)

		require.True(t, env.sendText(testUserID, "/tryGenerateCoffeePairs"))

		assert.Equal(t, []string{adminOnlyText}, env.texts())
		assert.Len(t, env.client.CallsTo("getChatMember"), 1)
		assert.False(t, env.click(testUserID, constants.TryGenerateCoffeePairsConfirmCallback), "conversation must not be started")
	})

	for _, sender := range groupChatSenders {
		t.Run("group chat by "+sender.name, func(t *testing.T) {
			env, _ := newPairsEnv(t)

			require.True(t, env.sendGroupText(sender.id, "/tryGenerateCoffeePairs"))

			env.requirePrivateOnlyNotice(sender.id)
			assert.False(t, env.sendGroupText(sender.id, "hello"), "conversation must not be started")
		})
	}
}

func TestTryGenerateCoffeePairsHandler_ButtonPressesAreAnswered(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		setup   func(env *testEnv, mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name:  "confirm",
			data:  constants.TryGenerateCoffeePairsConfirmCallback,
			setup: func(_ *testEnv, mock sqlmock.Sqlmock) { expectSuccessfulPairing(mock) },
		},
		{
			name: "confirm when pairing fails",
			data: constants.TryGenerateCoffeePairsConfirmCallback,
			setup: func(_ *testEnv, mock sqlmock.Sqlmock) {
				expectLatestPoll(mock)
				expectParticipants(mock, alice)
			},
		},
		{
			name: "confirm when the processing message cannot be sent",
			data: constants.TryGenerateCoffeePairsConfirmCallback,
			setup: func(env *testEnv, _ sqlmock.Sqlmock) {
				env.failSendWhen(func(call testutils.APICall) bool { return strings.Contains(call.Params["text"], "⏳") })
			},
			wantErr: true,
		},
		{
			name: "back",
			data: constants.TryGenerateCoffeePairsBackCallback,
			setup: func(_ *testEnv, mock sqlmock.Sqlmock) {
				expectLatestPoll(mock)
				expectParticipants(mock, alice, bob)
			},
		},
		{
			name: "cancel",
			data: constants.TryGenerateCoffeePairsCancelCallback,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env, mock := newPairsEnv(t)
			openPairsMenu(t, env, mock, alice, bob)
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
