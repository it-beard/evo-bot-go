package repositories

import (
	"testing"

	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	participantUpsertSQL = `
		INSERT INTO random_coffee_participants (poll_id, user_id, is_participating, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (poll_id, user_id) DO UPDATE SET
			is_participating = EXCLUDED.is_participating,
			updated_at = NOW()`
	participantDeleteSQL  = `DELETE FROM random_coffee_participants WHERE poll_id = $1 AND user_id = $2`
	participantGetSQL     = `SELECT id, poll_id, user_id, is_participating, created_at, updated_at FROM random_coffee_participants WHERE poll_id = $1 AND user_id = $2`
	participatingUsersSQL = `
		SELECT u.id, u.tg_id, u.firstname, u.lastname, u.tg_username
		FROM users u
		JOIN random_coffee_participants rpc ON u.id = rpc.user_id
		WHERE rpc.poll_id = $1 AND rpc.is_participating = TRUE
			AND u.has_coffee_ban = FALSE AND u.is_club_member IS NOT FALSE`
)

var (
	participantColumns       = []string{"id", "poll_id", "user_id", "is_participating", "created_at", "updated_at"}
	participatingUserColumns = []string{"id", "tg_id", "firstname", "lastname", "tg_username"}
)

func TestRandomCoffeeParticipantRepository_UpsertParticipant(t *testing.T) {
	tests := []struct {
		name            string
		isParticipating bool
	}{
		{name: "participating", isParticipating: true},
		{name: "not participating", isParticipating: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewRandomCoffeeParticipantRepository(db)

			mock.ExpectExec(exactSQL(participantUpsertSQL)).
				WithArgs(int64(12), int64(5), tc.isParticipating).
				WillReturnResult(sqlmock.NewResult(0, 1))

			err := repo.UpsertParticipant(RandomCoffeeParticipant{
				ID: 999, PollID: 12, UserID: 5, IsParticipating: tc.isParticipating,
				CreatedAt: testCreatedAt, UpdatedAt: testUpdatedAt,
			})

			assert.NoError(t, err)
		})
	}
}

func TestRandomCoffeeParticipantRepository_UpsertParticipant_ExecError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeeParticipantRepository(db)

	mock.ExpectExec(exactSQL(participantUpsertSQL)).WithArgs(int64(12), int64(5), true).WillReturnError(errDB)

	err := repo.UpsertParticipant(RandomCoffeeParticipant{PollID: 12, UserID: 5, IsParticipating: true})

	assert.ErrorIs(t, err, errDB)
}

func TestRandomCoffeeParticipantRepository_RemoveParticipant(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeeParticipantRepository(db)

	mock.ExpectExec(exactSQL(participantDeleteSQL)).WithArgs(int64(12), int64(5)).WillReturnResult(sqlmock.NewResult(0, 1))

	assert.NoError(t, repo.RemoveParticipant(12, 5))
}

func TestRandomCoffeeParticipantRepository_RemoveParticipant_ExecError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeeParticipantRepository(db)

	mock.ExpectExec(exactSQL(participantDeleteSQL)).WithArgs(int64(12), int64(5)).WillReturnError(errDB)

	assert.ErrorIs(t, repo.RemoveParticipant(12, 5), errDB)
}

func TestRandomCoffeeParticipantRepository_GetParticipant_Success(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeeParticipantRepository(db)

	mock.ExpectQuery(exactSQL(participantGetSQL)).WithArgs(int64(12), int64(5)).
		WillReturnRows(sqlmock.NewRows(participantColumns).AddRow(int64(77), int64(12), int64(5), true, testCreatedAt, testUpdatedAt))

	p, err := repo.GetParticipant(12, 5)

	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, RandomCoffeeParticipant{
		ID: 77, PollID: 12, UserID: 5, IsParticipating: true, CreatedAt: testCreatedAt, UpdatedAt: testUpdatedAt,
	}, *p)
}

func TestRandomCoffeeParticipantRepository_GetParticipant_NotFound(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeeParticipantRepository(db)

	mock.ExpectQuery(exactSQL(participantGetSQL)).WithArgs(int64(12), int64(5)).
		WillReturnRows(sqlmock.NewRows(participantColumns))

	p, err := repo.GetParticipant(12, 5)

	assert.NoError(t, err)
	assert.Nil(t, p)
}

func TestRandomCoffeeParticipantRepository_GetParticipant_QueryError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeeParticipantRepository(db)

	mock.ExpectQuery(exactSQL(participantGetSQL)).WithArgs(int64(12), int64(5)).WillReturnError(errDB)

	p, err := repo.GetParticipant(12, 5)

	require.Error(t, err)
	assert.Nil(t, p)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to get participant")
}

func TestRandomCoffeeParticipantRepository_GetParticipatingUsers_Success(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeeParticipantRepository(db)

	mock.ExpectQuery(exactSQL(participatingUsersSQL)).WithArgs(int64(12)).
		WillReturnRows(sqlmock.NewRows(participatingUserColumns).
			AddRow(1, int64(1001), "Ivan", "Petrov", "ivanp").
			AddRow(2, int64(1002), "Anna", "", ""))

	users, err := repo.GetParticipatingUsers(12)

	require.NoError(t, err)
	assert.Equal(t, []User{
		{ID: 1, TgID: 1001, Firstname: "Ivan", Lastname: "Petrov", TgUsername: "ivanp"},
		{ID: 2, TgID: 1002, Firstname: "Anna", Lastname: "", TgUsername: ""},
	}, users)
}

func TestRandomCoffeeParticipantRepository_GetParticipatingUsers_Empty(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeeParticipantRepository(db)

	mock.ExpectQuery(exactSQL(participatingUsersSQL)).WithArgs(int64(12)).
		WillReturnRows(sqlmock.NewRows(participatingUserColumns))

	users, err := repo.GetParticipatingUsers(12)

	require.NoError(t, err)
	assert.Empty(t, users)
}

func TestRandomCoffeeParticipantRepository_GetParticipatingUsers_QueryError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeeParticipantRepository(db)

	mock.ExpectQuery(exactSQL(participatingUsersSQL)).WithArgs(int64(12)).WillReturnError(errDB)

	users, err := repo.GetParticipatingUsers(12)

	require.Error(t, err)
	assert.Nil(t, users)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to get participating users")
}

func TestRandomCoffeeParticipantRepository_GetParticipatingUsers_ScanError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeeParticipantRepository(db)

	mock.ExpectQuery(exactSQL(participatingUsersSQL)).WithArgs(int64(12)).
		WillReturnRows(sqlmock.NewRows(participatingUserColumns).AddRow("x", int64(1001), "Ivan", "Petrov", "ivanp"))

	users, err := repo.GetParticipatingUsers(12)

	require.Error(t, err)
	assert.Nil(t, users)
}

func TestRandomCoffeeParticipantRepository_GetParticipatingUsers_RowsError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewRandomCoffeeParticipantRepository(db)

	mock.ExpectQuery(exactSQL(participatingUsersSQL)).WithArgs(int64(12)).
		WillReturnRows(sqlmock.NewRows(participatingUserColumns).
			AddRow(1, int64(1001), "Ivan", "Petrov", "ivanp").
			AddRow(2, int64(1002), "Anna", "", "").
			RowError(1, errDB))

	users, err := repo.GetParticipatingUsers(12)

	require.Error(t, err)
	assert.Nil(t, users)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "error during rows iteration for participating users")
}
