package repositories

import (
	"regexp"
	"testing"

	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	pairInsertSQL = `
		INSERT INTO random_coffee_pairs (poll_id, user1_id, user2_id)
		VALUES ($1, $2, $3)`
	pairMostRecentSQL = `
		SELECT poll.id
		FROM random_coffee_pairs p
		JOIN random_coffee_polls poll ON p.poll_id = poll.id
		WHERE ((p.user1_id = $1 AND p.user2_id = $2) OR (p.user1_id = $2 AND p.user2_id = $1))
		AND poll.id IN (
			SELECT id FROM random_coffee_polls
			ORDER BY week_start_date DESC
			LIMIT $3
		)
		ORDER BY poll.week_start_date DESC
		LIMIT 1`
)

var pairHistoryColumns = []string{"user1_id", "user2_id", "poll_id"}

// pairHistorySQL matches the history query for the given IN-clause placeholders.
// Only the selected columns, the join and the user filter are asserted; the LIMIT
// sub-select is intentionally left unconstrained.
func pairHistorySQL(placeholders string) string {
	return `^` + regexp.QuoteMeta(
		`SELECT p.user1_id, p.user2_id, poll.id as poll_id FROM random_coffee_pairs p `+
			`JOIN random_coffee_polls poll ON p.poll_id = poll.id `+
			`WHERE (p.user1_id IN (`+placeholders+`) OR p.user2_id IN (`+placeholders+`)) `+
			`ORDER BY poll.week_start_date DESC LIMIT `) + `.*\$1.*$`
}

func TestRandomCoffeePairRepository_CreatePair_Success(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeePairRepository(db)

	mock.ExpectExec(exactSQL(pairInsertSQL)).WithArgs(12, 3, 4).WillReturnResult(sqlmock.NewResult(1, 1))

	assert.NoError(t, repo.CreatePair(12, 3, 4))
}

func TestRandomCoffeePairRepository_CreatePair_ExecError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeePairRepository(db)

	mock.ExpectExec(exactSQL(pairInsertSQL)).WithArgs(12, 3, 4).WillReturnError(errDB)

	err := repo.CreatePair(12, 3, 4)

	require.Error(t, err)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "error creating random coffee pair")
}

func TestRandomCoffeePairRepository_GetPairsHistoryForUsers_NoUsers(t *testing.T) {
	db, _ := testutils.NewMockDB(t)
	repo := NewRandomCoffeePairRepository(db)

	for _, ids := range [][]int{nil, {}} {
		history, err := repo.GetPairsHistoryForUsers(ids, 4)

		require.NoError(t, err)
		require.NotNil(t, history)
		assert.Empty(t, history)
	}
}

func TestRandomCoffeePairRepository_GetPairsHistoryForUsers_Success(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeePairRepository(db)

	mock.ExpectQuery(pairHistorySQL("$2,$3,$4")).
		WithArgs(4, 1, 2, 3).
		WillReturnRows(sqlmock.NewRows(pairHistoryColumns).
			AddRow(1, 2, 30).
			AddRow(2, 3, 30).
			AddRow(2, 1, 20))

	history, err := repo.GetPairsHistoryForUsers([]int{1, 2, 3}, 4)

	require.NoError(t, err)
	assert.Equal(t, map[string][]int{
		"1-2": {30, 20},
		"2-1": {30, 20},
		"2-3": {30},
		"3-2": {30},
	}, history)
}

func TestRandomCoffeePairRepository_GetPairsHistoryForUsers_SingleUserPlaceholders(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeePairRepository(db)

	mock.ExpectQuery(pairHistorySQL("$2")).
		WithArgs(2, 7).
		WillReturnRows(sqlmock.NewRows(pairHistoryColumns))

	history, err := repo.GetPairsHistoryForUsers([]int{7}, 2)

	require.NoError(t, err)
	require.NotNil(t, history)
	assert.Empty(t, history)
}

func TestRandomCoffeePairRepository_GetPairsHistoryForUsers_QueryError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeePairRepository(db)

	mock.ExpectQuery(pairHistorySQL("$2,$3")).WithArgs(4, 1, 2).WillReturnError(errDB)

	history, err := repo.GetPairsHistoryForUsers([]int{1, 2}, 4)

	require.Error(t, err)
	assert.Nil(t, history)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "error getting pairs history")
}

func TestRandomCoffeePairRepository_GetPairsHistoryForUsers_ScanError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeePairRepository(db)

	mock.ExpectQuery(pairHistorySQL("$2,$3")).WithArgs(4, 1, 2).
		WillReturnRows(sqlmock.NewRows(pairHistoryColumns).AddRow(1, "x", 30))

	history, err := repo.GetPairsHistoryForUsers([]int{1, 2}, 4)

	require.Error(t, err)
	assert.Nil(t, history)
	assert.Contains(t, err.Error(), "error scanning pair history row")
}

func TestRandomCoffeePairRepository_GetPairsHistoryForUsers_RowsError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeePairRepository(db)

	mock.ExpectQuery(pairHistorySQL("$2,$3")).WithArgs(4, 1, 2).
		WillReturnRows(sqlmock.NewRows(pairHistoryColumns).
			AddRow(1, 2, 30).
			AddRow(2, 1, 20).
			RowError(1, errDB))

	history, err := repo.GetPairsHistoryForUsers([]int{1, 2}, 4)

	require.Error(t, err)
	assert.Nil(t, history)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "error during rows iteration for pair history")
}

func TestRandomCoffeePairRepository_GetMostRecentPairPoll_Found(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeePairRepository(db)

	mock.ExpectQuery(exactSQL(pairMostRecentSQL)).WithArgs(3, 4, 6).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(41))

	pollID, err := repo.GetMostRecentPairPoll(3, 4, 6)

	require.NoError(t, err)
	assert.Equal(t, 41, pollID)
}

func TestRandomCoffeePairRepository_GetMostRecentPairPoll_NeverPaired(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeePairRepository(db)

	mock.ExpectQuery(exactSQL(pairMostRecentSQL)).WithArgs(3, 4, 6).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	pollID, err := repo.GetMostRecentPairPoll(3, 4, 6)

	require.NoError(t, err)
	assert.Zero(t, pollID)
}

func TestRandomCoffeePairRepository_GetMostRecentPairPoll_QueryError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeePairRepository(db)

	mock.ExpectQuery(exactSQL(pairMostRecentSQL)).WithArgs(3, 4, 6).WillReturnError(errDB)

	pollID, err := repo.GetMostRecentPairPoll(3, 4, 6)

	require.Error(t, err)
	assert.Zero(t, pollID)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "error getting most recent pair poll")
}
