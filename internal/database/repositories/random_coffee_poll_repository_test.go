package repositories

import (
	"testing"
	"time"

	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	pollInsertSQL = `
		INSERT INTO random_coffee_polls (message_id, week_start_date, telegram_poll_id, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id`
	pollByTelegramIDSQL = `
		SELECT id, message_id, week_start_date, telegram_poll_id, created_at
		FROM random_coffee_polls
		WHERE telegram_poll_id = $1`
	pollLatestSQL = `
		SELECT id, message_id, week_start_date, telegram_poll_id, created_at
		FROM random_coffee_polls
		ORDER BY week_start_date DESC, id DESC
		LIMIT 1`
)

var (
	pollColumns   = []string{"id", "message_id", "week_start_date", "telegram_poll_id", "created_at"}
	testWeekStart = time.Date(2025, 9, 8, 0, 0, 0, 0, time.UTC)
)

func TestRandomCoffeePollRepository_CreatePoll_WithCreatedAt(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeePollRepository(db)

	mock.ExpectQuery(exactSQL(pollInsertSQL)).
		WithArgs(int64(555), testWeekStart, "tg-poll-1", testCreatedAt).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(12)))

	id, err := repo.CreatePoll(RandomCoffeePoll{
		MessageID:      555,
		WeekStartDate:  testWeekStart,
		TelegramPollID: "tg-poll-1",
		CreatedAt:      testCreatedAt,
	})

	require.NoError(t, err)
	assert.Equal(t, int64(12), id)
}

func TestRandomCoffeePollRepository_CreatePoll_ZeroCreatedAtDefaultsToNow(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeePollRepository(db)

	before := time.Now()
	mock.ExpectQuery(exactSQL(pollInsertSQL)).
		WithArgs(int64(555), testWeekStart, "tg-poll-1", timeBetween{from: before, to: before.Add(time.Minute)}).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(13)))

	id, err := repo.CreatePoll(RandomCoffeePoll{MessageID: 555, WeekStartDate: testWeekStart, TelegramPollID: "tg-poll-1"})

	require.NoError(t, err)
	assert.Equal(t, int64(13), id)
}

func TestRandomCoffeePollRepository_CreatePoll_QueryError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeePollRepository(db)

	mock.ExpectQuery(exactSQL(pollInsertSQL)).WillReturnError(errDB)

	id, err := repo.CreatePoll(RandomCoffeePoll{MessageID: 555, WeekStartDate: testWeekStart, TelegramPollID: "tg-poll-1"})

	require.Error(t, err)
	assert.Zero(t, id)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to create poll")
}

func TestRandomCoffeePollRepository_GetPollByTelegramPollID_Success(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeePollRepository(db)

	mock.ExpectQuery(exactSQL(pollByTelegramIDSQL)).WithArgs("tg-poll-1").
		WillReturnRows(sqlmock.NewRows(pollColumns).AddRow(int64(12), int64(555), testWeekStart, "tg-poll-1", testCreatedAt))

	poll, err := repo.GetPollByTelegramPollID("tg-poll-1")

	require.NoError(t, err)
	require.NotNil(t, poll)
	assert.Equal(t, RandomCoffeePoll{
		ID: 12, MessageID: 555, WeekStartDate: testWeekStart, TelegramPollID: "tg-poll-1", CreatedAt: testCreatedAt,
	}, *poll)
}

func TestRandomCoffeePollRepository_GetPollByTelegramPollID_NotFound(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeePollRepository(db)

	mock.ExpectQuery(exactSQL(pollByTelegramIDSQL)).WithArgs("missing").WillReturnRows(sqlmock.NewRows(pollColumns))

	poll, err := repo.GetPollByTelegramPollID("missing")

	assert.NoError(t, err)
	assert.Nil(t, poll)
}

func TestRandomCoffeePollRepository_GetPollByTelegramPollID_QueryError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeePollRepository(db)

	mock.ExpectQuery(exactSQL(pollByTelegramIDSQL)).WithArgs("tg-poll-1").WillReturnError(errDB)

	poll, err := repo.GetPollByTelegramPollID("tg-poll-1")

	require.Error(t, err)
	assert.Nil(t, poll)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to get poll by telegram poll ID")
}

func TestRandomCoffeePollRepository_GetLatestPoll_Success(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeePollRepository(db)

	mock.ExpectQuery(exactSQL(pollLatestSQL)).
		WillReturnRows(sqlmock.NewRows(pollColumns).AddRow(int64(20), int64(600), testWeekStart, "tg-poll-20", testCreatedAt))

	poll, err := repo.GetLatestPoll()

	require.NoError(t, err)
	require.NotNil(t, poll)
	assert.Equal(t, RandomCoffeePoll{
		ID: 20, MessageID: 600, WeekStartDate: testWeekStart, TelegramPollID: "tg-poll-20", CreatedAt: testCreatedAt,
	}, *poll)
}

func TestRandomCoffeePollRepository_GetLatestPoll_NoPolls(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeePollRepository(db)

	mock.ExpectQuery(exactSQL(pollLatestSQL)).WillReturnRows(sqlmock.NewRows(pollColumns))

	poll, err := repo.GetLatestPoll()

	assert.NoError(t, err)
	assert.Nil(t, poll)
}

func TestRandomCoffeePollRepository_GetLatestPoll_QueryError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeePollRepository(db)

	mock.ExpectQuery(exactSQL(pollLatestSQL)).WillReturnError(errDB)

	poll, err := repo.GetLatestPoll()

	require.Error(t, err)
	assert.Nil(t, poll)
	assert.ErrorIs(t, err, errDB)
}

func TestRandomCoffeePollRepository_GetLatestPoll_ScanError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeePollRepository(db)

	mock.ExpectQuery(exactSQL(pollLatestSQL)).
		WillReturnRows(sqlmock.NewRows(pollColumns).AddRow(int64(20), int64(600), "not-a-date", "tg-poll-20", testCreatedAt))

	poll, err := repo.GetLatestPoll()

	require.Error(t, err)
	assert.Nil(t, poll)
}
