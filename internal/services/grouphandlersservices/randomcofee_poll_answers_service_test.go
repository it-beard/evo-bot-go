package grouphandlersservices

import (
	"testing"

	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testPollID = "tg-poll-1"

func newPollAnswersService(d *testDeps) *RandomCoffeePollAnswersService {
	return NewRandomCoffeePollAnswersService(
		d.sender,
		d.cfg,
		repositories.NewRandomCoffeePollRepository(d.db),
		repositories.NewRandomCoffeeParticipantRepository(d.db),
		d.userRepo,
	)
}

func pollAnswer(userID int64, optionIDs ...int64) *gotgbot.PollAnswer {
	return testutils.PollAnswerUpdate(userID, testPollID, optionIDs).PollAnswer
}

func botPollAnswer(optionIDs ...int64) *gotgbot.PollAnswer {
	answer := pollAnswer(testUserID, optionIDs...)
	answer.User.IsBot = true
	return answer
}

func expectTrackedPoll(mock sqlmock.Sqlmock, pollDBID int64) {
	mock.ExpectQuery(sqlSelectPollByTelegramID).WithArgs(testPollID).
		WillReturnRows(sqlmock.NewRows(pollColumns).AddRow(pollDBID, 321, testCreatedAt, testPollID, testCreatedAt))
}

func TestRandomCoffeePollAnswersService_GetInternalUser(t *testing.T) {
	d := newTestDeps(t)
	s := newPollAnswersService(d)

	d.mock.ExpectQuery(sqlSelectUserByTgID).WithArgs(testUserID).WillReturnRows(userRow(7, testUserID, true, true))

	user := s.GetInternalUser(pollAnswer(testUserID, 0))

	require.NotNil(t, user)
	assert.Equal(t, 7, user.ID)
	assert.Equal(t, testUserID, user.TgID)
	assert.True(t, user.HasCoffeeBan)
}

func TestRandomCoffeePollAnswersService_IsAnswerShouldBeProcessed(t *testing.T) {
	regularUser := &repositories.User{ID: 7, TgID: testUserID}
	bannedUser := &repositories.User{ID: 7, TgID: testUserID, HasCoffeeBan: true}

	tests := []struct {
		name       string
		answer     *gotgbot.PollAnswer
		user       *repositories.User
		expected   bool
		notifyChat int64 // 0 means no message must be sent
		notifyText string
	}{
		{name: "nil answer", answer: nil, user: regularUser, expected: false},
		{name: "bot votes yes", answer: botPollAnswer(0), user: regularUser, expected: false,
			notifyChat: testAdminUserID, notifyText: "для ботов недоступно"},
		{name: "bot votes no", answer: botPollAnswer(1), user: regularUser, expected: false},
		{name: "bot retracts vote", answer: botPollAnswer(), user: regularUser, expected: false},
		{name: "banned user votes yes", answer: pollAnswer(testUserID, 0), user: bannedUser, expected: false,
			notifyChat: testUserID, notifyText: "находишься в бане"},
		{name: "banned user votes no", answer: pollAnswer(testUserID, 1), user: bannedUser, expected: false},
		{name: "banned user retracts vote", answer: pollAnswer(testUserID), user: bannedUser, expected: false},
		{name: "regular user votes yes", answer: pollAnswer(testUserID, 0), user: regularUser, expected: true},
		{name: "regular user votes no", answer: pollAnswer(testUserID, 1), user: regularUser, expected: true},
		{name: "regular user retracts vote", answer: pollAnswer(testUserID), user: regularUser, expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newTestDeps(t)
			s := newPollAnswersService(d)

			assert.Equal(t, tt.expected, s.IsAnswerShouldBeProcessed(tt.answer, tt.user))

			sent := d.client.CallsTo("sendMessage")
			if tt.notifyChat == 0 {
				assert.Empty(t, sent)
				return
			}
			require.Len(t, sent, 1)
			assert.Equal(t, tt.notifyChat, sent[0].Int64("chat_id"))
			assert.Equal(t, "HTML", sent[0].Params["parse_mode"])
			assert.Contains(t, sent[0].Params["text"], tt.notifyText)
		})
	}
}

func TestRandomCoffeePollAnswersService_ProcessAnswer_Votes(t *testing.T) {
	tests := []struct {
		name            string
		optionIDs       []int64
		isParticipating bool
	}{
		{"yes vote", []int64{0}, true},
		{"no vote", []int64{1}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newTestDeps(t)
			s := newPollAnswersService(d)

			expectTrackedPoll(d.mock, 55)
			d.mock.ExpectExec(sqlUpsertCoffeeParticipant).WithArgs(int64(55), int64(7), tt.isParticipating).
				WillReturnResult(sqlmock.NewResult(1, 1))

			err := s.ProcessAnswer(pollAnswer(testUserID, tt.optionIDs...), &repositories.User{ID: 7, TgID: testUserID})

			require.NoError(t, err)
			assert.Empty(t, d.client.Calls())
		})
	}
}

func TestRandomCoffeePollAnswersService_ProcessAnswer_RetractedVoteRemovesParticipant(t *testing.T) {
	d := newTestDeps(t)
	s := newPollAnswersService(d)

	expectTrackedPoll(d.mock, 55)
	d.mock.ExpectExec(sqlDeleteCoffeeParticipant).WithArgs(int64(55), int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, s.ProcessAnswer(pollAnswer(testUserID), &repositories.User{ID: 7, TgID: testUserID}))
}

func TestRandomCoffeePollAnswersService_ProcessAnswer_UntrackedPollIsIgnored(t *testing.T) {
	d := newTestDeps(t)
	s := newPollAnswersService(d)

	d.mock.ExpectQuery(sqlSelectPollByTelegramID).WithArgs(testPollID).WillReturnRows(sqlmock.NewRows(pollColumns))

	require.NoError(t, s.ProcessAnswer(pollAnswer(testUserID, 0), &repositories.User{ID: 7, TgID: testUserID}))
}

func TestRandomCoffeePollAnswersService_ProcessAnswer_DatabaseErrorsAreSwallowed(t *testing.T) {
	user := &repositories.User{ID: 7, TgID: testUserID}

	t.Run("poll lookup fails", func(t *testing.T) {
		d := newTestDeps(t)
		s := newPollAnswersService(d)
		d.mock.ExpectQuery(sqlSelectPollByTelegramID).WithArgs(testPollID).WillReturnError(errDB)

		assert.NoError(t, s.ProcessAnswer(pollAnswer(testUserID, 0), user))
	})

	t.Run("upsert fails", func(t *testing.T) {
		d := newTestDeps(t)
		s := newPollAnswersService(d)
		expectTrackedPoll(d.mock, 55)
		d.mock.ExpectExec(sqlUpsertCoffeeParticipant).WillReturnError(errDB)

		assert.NoError(t, s.ProcessAnswer(pollAnswer(testUserID, 0), user))
	})

	t.Run("remove fails", func(t *testing.T) {
		d := newTestDeps(t)
		s := newPollAnswersService(d)
		expectTrackedPoll(d.mock, 55)
		d.mock.ExpectExec(sqlDeleteCoffeeParticipant).WillReturnError(errDB)

		assert.NoError(t, s.ProcessAnswer(pollAnswer(testUserID), user))
	})
}
