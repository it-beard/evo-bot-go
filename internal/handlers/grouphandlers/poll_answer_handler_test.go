package grouphandlers_test

import (
	"testing"

	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testPollID = "tg-poll-1"

func expectTrackedPoll(mock sqlmock.Sqlmock, pollDBID int64) {
	mock.ExpectQuery(sqlSelectPollByTelegramID).WithArgs(testPollID).
		WillReturnRows(sqlmock.NewRows(pollColumns).AddRow(pollDBID, 321, testCreatedAt, testPollID, testCreatedAt))
}

func TestPollAnswerHandler_VotesAreStored(t *testing.T) {
	tests := []struct {
		name            string
		optionIDs       []int64
		isParticipating bool
	}{
		{"yes", []int64{0}, true},
		{"no", []int64{1}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newHandlerEnv(t)

			expectExistingUser(e.mock, 7, testUserID)
			expectTrackedPoll(e.mock, 55)
			e.mock.ExpectExec(sqlUpsertCoffeeParticipant).WithArgs(int64(55), int64(7), tt.isParticipating).
				WillReturnResult(sqlmock.NewResult(1, 1))

			handled, err := testutils.Dispatch(e.bot, e.pollAnswerHandler(), testutils.PollAnswerUpdate(testUserID, testPollID, tt.optionIDs))

			require.NoError(t, err)
			assert.True(t, handled)
			assert.Empty(t, e.client.Calls())
		})
	}
}

func TestPollAnswerHandler_RetractedVoteRemovesParticipant(t *testing.T) {
	e := newHandlerEnv(t)

	expectExistingUser(e.mock, 7, testUserID)
	expectTrackedPoll(e.mock, 55)
	e.mock.ExpectExec(sqlDeleteCoffeeParticipant).WithArgs(int64(55), int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))

	handled, err := testutils.Dispatch(e.bot, e.pollAnswerHandler(), testutils.PollAnswerUpdate(testUserID, testPollID, nil))

	require.NoError(t, err)
	assert.True(t, handled)
}

func TestPollAnswerHandler_UntrackedPollIsIgnored(t *testing.T) {
	e := newHandlerEnv(t)

	expectExistingUser(e.mock, 7, testUserID)
	e.mock.ExpectQuery(sqlSelectPollByTelegramID).WithArgs(testPollID).WillReturnRows(sqlmock.NewRows(pollColumns))

	handled, err := testutils.Dispatch(e.bot, e.pollAnswerHandler(), testutils.PollAnswerUpdate(testUserID, testPollID, []int64{0}))

	require.NoError(t, err)
	assert.True(t, handled)
	assert.Empty(t, e.client.Calls())
}

func TestPollAnswerHandler_BannedUserIsWarnedAndNotStored(t *testing.T) {
	e := newHandlerEnv(t)

	e.mock.ExpectQuery(sqlSelectUserByTgID).WithArgs(testUserID).WillReturnRows(userRow(7, testUserID, true))
	// no poll / participant queries

	handled, err := testutils.Dispatch(e.bot, e.pollAnswerHandler(), testutils.PollAnswerUpdate(testUserID, testPollID, []int64{0}))

	require.NoError(t, err)
	assert.True(t, handled)
	sent := e.client.CallsTo("sendMessage")
	require.Len(t, sent, 1)
	assert.Equal(t, testUserID, sent[0].Int64("chat_id"))
	assert.Contains(t, sent[0].Params["text"], "находишься в бане")
}

func TestPollAnswerHandler_BotVoteIsReportedToAdmin(t *testing.T) {
	e := newHandlerEnv(t)
	update := testutils.PollAnswerUpdate(testUserID, testPollID, []int64{0})
	update.PollAnswer.User.IsBot = true

	expectExistingUser(e.mock, 7, testUserID)

	handled, err := testutils.Dispatch(e.bot, e.pollAnswerHandler(), update)

	require.NoError(t, err)
	assert.True(t, handled)
	sent := e.client.CallsTo("sendMessage")
	require.Len(t, sent, 1)
	assert.Equal(t, testAdminUserID, sent[0].Int64("chat_id"))
	assert.Contains(t, sent[0].Params["text"], "для ботов недоступно")
}

func TestPollAnswerHandler_DoesNotHandleOtherUpdates(t *testing.T) {
	e := newHandlerEnv(t)

	handled, err := testutils.Dispatch(e.bot, e.pollAnswerHandler(),
		testutils.MessageUpdate(newGroupMsg(testUserID, testOpenTopicID, "hi")))

	require.NoError(t, err)
	assert.False(t, handled)
}
