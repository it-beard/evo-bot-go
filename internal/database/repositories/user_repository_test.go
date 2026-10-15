package repositories

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"

	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	userSelect = `
		SELECT id, tg_id, firstname, lastname, tg_username, score, has_coffee_ban, is_club_member, created_at, updated_at
		FROM users`
	userByIDSQL         = userSelect + ` WHERE id = $1`
	userByTgIDSQL       = userSelect + ` WHERE tg_id = $1`
	userByUsernameSQL   = userSelect + ` WHERE tg_username = $1`
	userSearchByNameSQL = userSelect + ` WHERE LOWER(firstname) = LOWER($1) AND LOWER(lastname) = LOWER($2) LIMIT 1`
	userInsertSQL       = `INSERT INTO users (tg_id, firstname, lastname, tg_username, score, has_coffee_ban, is_club_member)
			VALUES ($1, $2, $3, $4, 0, false, true) RETURNING id`
	userDeleteSQL = `DELETE FROM users WHERE id = $1`
)

var userColumns = []string{
	"id", "tg_id", "firstname", "lastname", "tg_username", "score", "has_coffee_ban", "is_club_member", "created_at", "updated_at",
}

// userRow returns a sqlmock row for the given user
func userRow(u User) []driver.Value {
	return []driver.Value{u.ID, u.TgID, u.Firstname, u.Lastname, u.TgUsername, u.Score, u.HasCoffeeBan, u.IsClubMember, u.CreatedAt, u.UpdatedAt}
}

func sampleUser() User {
	return User{
		ID:           5,
		TgID:         987654321,
		Firstname:    "Ivan",
		Lastname:     "Petrov",
		TgUsername:   "ivanp",
		Score:        12,
		HasCoffeeBan: true,
		IsClubMember: false,
		CreatedAt:    testCreatedAt,
		UpdatedAt:    testUpdatedAt,
	}
}

func TestUserRepository_GetDB(t *testing.T) {
	db, _ := testutils.NewMockDB(t)

	assert.Same(t, db, NewUserRepository(db).GetDB())
}

// userGetCase describes a single-user lookup method
type userGetCase struct {
	name      string
	sql       string
	args      []driver.Value
	call      func(r *UserRepository) (*User, error)
	errSubstr string
}

func userGetCases() []userGetCase {
	return []userGetCase{
		{
			name:      "GetByID",
			sql:       userByIDSQL,
			args:      []driver.Value{5},
			call:      func(r *UserRepository) (*User, error) { return r.GetByID(5) },
			errSubstr: "failed to get user with ID 5",
		},
		{
			name:      "GetByTelegramID",
			sql:       userByTgIDSQL,
			args:      []driver.Value{int64(987654321)},
			call:      func(r *UserRepository) (*User, error) { return r.GetByTelegramID(987654321) },
			errSubstr: "failed to get user with Telegram ID 987654321",
		},
		{
			name:      "GetByTelegramUsername",
			sql:       userByUsernameSQL,
			args:      []driver.Value{"ivanp"},
			call:      func(r *UserRepository) (*User, error) { return r.GetByTelegramUsername("ivanp") },
			errSubstr: "failed to get user with Telegram username ivanp",
		},
		{
			name:      "SearchByName",
			sql:       userSearchByNameSQL,
			args:      []driver.Value{"Ivan", "Petrov"},
			call:      func(r *UserRepository) (*User, error) { return r.SearchByName("Ivan", "Petrov") },
			errSubstr: "failed to search user by name",
		},
	}
}

func TestUserRepository_GetSingle_Success(t *testing.T) {
	for _, tc := range userGetCases() {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewUserRepository(db)

			mock.ExpectQuery(exactSQL(tc.sql)).WithArgs(tc.args...).
				WillReturnRows(sqlmock.NewRows(userColumns).AddRow(userRow(sampleUser())...))

			user, err := tc.call(repo)

			require.NoError(t, err)
			require.NotNil(t, user)
			assert.Equal(t, sampleUser(), *user)
		})
	}
}

func TestUserRepository_GetSingle_NotFoundReturnsErrNoRows(t *testing.T) {
	for _, tc := range userGetCases() {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewUserRepository(db)

			mock.ExpectQuery(exactSQL(tc.sql)).WithArgs(tc.args...).WillReturnRows(sqlmock.NewRows(userColumns))

			user, err := tc.call(repo)

			assert.Nil(t, user)
			// Callers compare with == sql.ErrNoRows, so the sentinel must be returned unwrapped
			assert.Equal(t, sql.ErrNoRows, err)
		})
	}
}

func TestUserRepository_GetSingle_QueryError(t *testing.T) {
	for _, tc := range userGetCases() {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewUserRepository(db)

			mock.ExpectQuery(exactSQL(tc.sql)).WithArgs(tc.args...).WillReturnError(errDB)

			user, err := tc.call(repo)

			require.Error(t, err)
			assert.Nil(t, user)
			assert.ErrorIs(t, err, errDB)
			assert.Contains(t, err.Error(), tc.errSubstr)
		})
	}
}

func TestUserRepository_GetSingle_ScanError(t *testing.T) {
	for _, tc := range userGetCases() {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewUserRepository(db)

			row := userRow(sampleUser())
			row[6] = "not-a-bool" // has_coffee_ban
			mock.ExpectQuery(exactSQL(tc.sql)).WithArgs(tc.args...).
				WillReturnRows(sqlmock.NewRows(userColumns).AddRow(row...))

			user, err := tc.call(repo)

			require.Error(t, err)
			assert.Nil(t, user)
			assert.NotEqual(t, sql.ErrNoRows, err)
			assert.Contains(t, err.Error(), tc.errSubstr)
		})
	}
}

func TestUserRepository_Create_Success(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewUserRepository(db)

	mock.ExpectQuery(exactSQL(userInsertSQL)).
		WithArgs(int64(987654321), "Ivan", "Petrov", "ivanp").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(5))

	id, err := repo.Create(987654321, "Ivan", "Petrov", "ivanp")

	require.NoError(t, err)
	assert.Equal(t, 5, id)
}

func TestUserRepository_Create_QueryError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewUserRepository(db)

	mock.ExpectQuery(exactSQL(userInsertSQL)).
		WithArgs(int64(987654321), "Ivan", "Petrov", "ivanp").
		WillReturnError(errDB)

	id, err := repo.Create(987654321, "Ivan", "Petrov", "ivanp")

	require.Error(t, err)
	assert.Zero(t, id)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to insert user")
}

func TestUserRepository_Update_NoFields(t *testing.T) {
	db, _ := testutils.NewMockDB(t)
	repo := NewUserRepository(db)

	for _, fields := range []map[string]interface{}{nil, {}} {
		err := repo.Update(5, fields)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no fields to update for user with ID 5")
	}
}

func TestUserRepository_Update_SingleField(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewUserRepository(db)

	mock.ExpectExec(exactSQL(`UPDATE users SET updated_at = NOW(), score = $1 WHERE id = $2`)).
		WithArgs(100, 5).
		WillReturnResult(sqlmock.NewResult(0, 1))

	assert.NoError(t, repo.Update(5, map[string]interface{}{"score": 100}))
}

func TestUserRepository_Update_MultipleFields(t *testing.T) {
	db, mock, rec := newRecordingMockDB(t)
	repo := NewUserRepository(db)

	fields := map[string]interface{}{
		"firstname":   "Ivan",
		"lastname":    "Petrov",
		"tg_username": "ivanp",
	}
	captured, matchers := captureArgs(len(fields) + 1)
	mock.ExpectExec(`^UPDATE users SET updated_at = NOW\(\), .* WHERE id = \$4$`).
		WithArgs(matchers...).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, repo.Update(5, fields))

	assertDynamicUpdate(t, "users", rec.last(), captured, fields, 5)
}

func TestUserRepository_Update_Errors(t *testing.T) {
	updateSQL := exactSQL(`UPDATE users SET updated_at = NOW(), score = $1 WHERE id = $2`)

	t.Run("exec error", func(t *testing.T) {
		db, mock := testutils.NewMockDB(t)
		mock.ExpectExec(updateSQL).WithArgs(1, 5).WillReturnError(errDB)

		err := NewUserRepository(db).Update(5, map[string]interface{}{"score": 1})

		require.Error(t, err)
		assert.ErrorIs(t, err, errDB)
		assert.Contains(t, err.Error(), "failed to update user with ID 5")
	})

	t.Run("not found", func(t *testing.T) {
		db, mock := testutils.NewMockDB(t)
		mock.ExpectExec(updateSQL).WithArgs(1, 5).WillReturnResult(sqlmock.NewResult(0, 0))

		err := NewUserRepository(db).Update(5, map[string]interface{}{"score": 1})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no user found with ID 5 to update")
	})

	t.Run("rows affected error is only logged", func(t *testing.T) {
		db, mock := testutils.NewMockDB(t)
		mock.ExpectExec(updateSQL).WithArgs(1, 5).
			WillReturnResult(sqlmock.NewErrorResult(errors.New("rows affected unsupported")))

		assert.NoError(t, NewUserRepository(db).Update(5, map[string]interface{}{"score": 1}))
	})
}

// userExecCase describes a fixed-SQL UPDATE/DELETE method on users
type userExecCase struct {
	name        string
	sql         string
	args        []driver.Value
	call        func(r *UserRepository) error
	execErrText string
	notFound    string
}

func userExecCases() []userExecCase {
	return []userExecCase{
		{
			name:        "Delete",
			sql:         userDeleteSQL,
			args:        []driver.Value{5},
			call:        func(r *UserRepository) error { return r.Delete(5) },
			execErrText: "failed to delete user with ID 5",
			notFound:    "no user found with ID 5 to delete",
		},
		{
			name:        "UpdateScore",
			sql:         `UPDATE users SET score = $1, updated_at = NOW() WHERE id = $2`,
			args:        []driver.Value{42, 5},
			call:        func(r *UserRepository) error { return r.UpdateScore(5, 42) },
			execErrText: "failed to update score for user with ID 5",
			notFound:    "no user found with ID 5 to update score",
		},
		{
			name:        "SetCoffeeBan",
			sql:         `UPDATE users SET has_coffee_ban = $1, updated_at = NOW() WHERE id = $2`,
			args:        []driver.Value{true, 5},
			call:        func(r *UserRepository) error { return r.SetCoffeeBan(5, true) },
			execErrText: "failed to update coffee ban status for user with ID 5",
			notFound:    "no user found with ID 5 to update coffee ban status",
		},
		{
			name:        "SetClubMemberStatus",
			sql:         `UPDATE users SET is_club_member = $1, updated_at = NOW() WHERE id = $2`,
			args:        []driver.Value{false, 5},
			call:        func(r *UserRepository) error { return r.SetClubMemberStatus(5, false) },
			execErrText: "failed to update club member status for user with ID 5",
			notFound:    "no user found with ID 5 to update club member status",
		},
		{
			name:        "UpdateTelegramUsername",
			sql:         `UPDATE users SET tg_username = $1, updated_at = NOW() WHERE id = $2`,
			args:        []driver.Value{"new_nick", 5},
			call:        func(r *UserRepository) error { return r.UpdateTelegramUsername(5, "new_nick") },
			execErrText: "failed to update telegram username for user with ID 5",
			notFound:    "no user found with ID 5 to update telegram username",
		},
		{
			name:        "UpdateFirstname",
			sql:         `UPDATE users SET firstname = $1, updated_at = NOW() WHERE id = $2`,
			args:        []driver.Value{"Pyotr", 5},
			call:        func(r *UserRepository) error { return r.UpdateFirstname(5, "Pyotr") },
			execErrText: "failed to update firstname for user with ID 5",
			notFound:    "no user found with ID 5 to update firstname",
		},
		{
			name:        "UpdateLastname",
			sql:         `UPDATE users SET lastname = $1, updated_at = NOW() WHERE id = $2`,
			args:        []driver.Value{"Sidorov", 5},
			call:        func(r *UserRepository) error { return r.UpdateLastname(5, "Sidorov") },
			execErrText: "failed to update lastname for user with ID 5",
			notFound:    "no user found with ID 5 to update lastname",
		},
	}
}

func TestUserRepository_ExecMethods(t *testing.T) {
	for _, tc := range userExecCases() {
		t.Run(tc.name+"/success", func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			mock.ExpectExec(exactSQL(tc.sql)).WithArgs(tc.args...).WillReturnResult(sqlmock.NewResult(0, 1))

			assert.NoError(t, tc.call(NewUserRepository(db)))
		})

		t.Run(tc.name+"/exec error", func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			mock.ExpectExec(exactSQL(tc.sql)).WithArgs(tc.args...).WillReturnError(errDB)

			err := tc.call(NewUserRepository(db))

			require.Error(t, err)
			assert.ErrorIs(t, err, errDB)
			assert.Contains(t, err.Error(), tc.execErrText)
		})

		t.Run(tc.name+"/not found", func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			mock.ExpectExec(exactSQL(tc.sql)).WithArgs(tc.args...).WillReturnResult(sqlmock.NewResult(0, 0))

			err := tc.call(NewUserRepository(db))

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.notFound)
		})

		t.Run(tc.name+"/rows affected error is only logged", func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			mock.ExpectExec(exactSQL(tc.sql)).WithArgs(tc.args...).
				WillReturnResult(sqlmock.NewErrorResult(errors.New("rows affected unsupported")))

			assert.NoError(t, tc.call(NewUserRepository(db)))
		})
	}
}

func TestUserRepository_GetOrCreate_ExistingUserUnchanged(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewUserRepository(db)

	existing := sampleUser()
	mock.ExpectQuery(exactSQL(userByTgIDSQL)).WithArgs(existing.TgID).
		WillReturnRows(sqlmock.NewRows(userColumns).AddRow(userRow(existing)...))

	// Different first/last name in Telegram must NOT overwrite non-empty DB values; same username => no update
	user, err := repo.GetOrCreate(&gotgbot.User{Id: existing.TgID, FirstName: "Other", LastName: "Name", Username: "ivanp"})

	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, existing, *user)
}

func TestUserRepository_GetOrCreate_ExistingUserEmptyTelegramUsernameIgnored(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewUserRepository(db)

	existing := sampleUser()
	mock.ExpectQuery(exactSQL(userByTgIDSQL)).WithArgs(existing.TgID).
		WillReturnRows(sqlmock.NewRows(userColumns).AddRow(userRow(existing)...))

	user, err := repo.GetOrCreate(&gotgbot.User{Id: existing.TgID, FirstName: "Ivan", Username: ""})

	require.NoError(t, err)
	assert.Equal(t, "ivanp", user.TgUsername)
}

func TestUserRepository_GetOrCreate_ExistingUserUsernameChanged(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewUserRepository(db)

	existing := sampleUser()
	mock.ExpectQuery(exactSQL(userByTgIDSQL)).WithArgs(existing.TgID).
		WillReturnRows(sqlmock.NewRows(userColumns).AddRow(userRow(existing)...))
	mock.ExpectExec(exactSQL(`UPDATE users SET updated_at = NOW(), tg_username = $1 WHERE id = $2`)).
		WithArgs("ivan_new", existing.ID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	user, err := repo.GetOrCreate(&gotgbot.User{Id: existing.TgID, FirstName: "Ivan", LastName: "Petrov", Username: "ivan_new"})

	require.NoError(t, err)
	require.NotNil(t, user)
	expected := existing
	expected.TgUsername = "ivan_new"
	assert.Equal(t, expected, *user)
}

func TestUserRepository_GetOrCreate_ExistingUserFillsAllFields(t *testing.T) {
	db, mock, rec := newRecordingMockDB(t)
	repo := NewUserRepository(db)

	existing := sampleUser()
	existing.Firstname = ""
	existing.Lastname = ""
	existing.TgUsername = "old"
	mock.ExpectQuery(exactSQL(userByTgIDSQL)).WithArgs(existing.TgID).
		WillReturnRows(sqlmock.NewRows(userColumns).AddRow(userRow(existing)...))
	captured, matchers := captureArgs(4)
	mock.ExpectExec(`^UPDATE users SET updated_at = NOW\(\), .* WHERE id = \$4$`).
		WithArgs(matchers...).
		WillReturnResult(sqlmock.NewResult(0, 1))

	user, err := repo.GetOrCreate(&gotgbot.User{Id: existing.TgID, FirstName: "Ivan", LastName: "Petrov", Username: "ivanp"})

	require.NoError(t, err)
	require.NotNil(t, user)
	assertDynamicUpdate(t, "users", rec.last(), captured, map[string]interface{}{
		"firstname":   "Ivan",
		"lastname":    "Petrov",
		"tg_username": "ivanp",
	}, existing.ID)
	assert.Equal(t, "Ivan", user.Firstname)
	assert.Equal(t, "Petrov", user.Lastname)
	assert.Equal(t, "ivanp", user.TgUsername)
	assert.Equal(t, existing.ID, user.ID)
	assert.Equal(t, existing.Score, user.Score)
}

func TestUserRepository_GetOrCreate_UpdateError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewUserRepository(db)

	existing := sampleUser()
	existing.Firstname = ""
	mock.ExpectQuery(exactSQL(userByTgIDSQL)).WithArgs(existing.TgID).
		WillReturnRows(sqlmock.NewRows(userColumns).AddRow(userRow(existing)...))
	mock.ExpectExec(exactSQL(`UPDATE users SET updated_at = NOW(), firstname = $1 WHERE id = $2`)).
		WithArgs("Ivan", existing.ID).
		WillReturnError(errDB)

	user, err := repo.GetOrCreate(&gotgbot.User{Id: existing.TgID, FirstName: "Ivan", Username: "ivanp"})

	require.Error(t, err)
	assert.Nil(t, user)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to update user fields")
}

func TestUserRepository_GetOrCreate_LookupError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewUserRepository(db)

	// No INSERT expectation: a real lookup error must not lead to user creation
	mock.ExpectQuery(exactSQL(userByTgIDSQL)).WithArgs(int64(111)).WillReturnError(errDB)

	user, err := repo.GetOrCreate(&gotgbot.User{Id: 111, FirstName: "New"})

	require.Error(t, err)
	assert.Nil(t, user)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to get user in getOrCreateUser")
}

func TestUserRepository_GetOrCreate_CreatesNewUser(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewUserRepository(db)

	created := User{
		ID: 77, TgID: 111, Firstname: "New", Lastname: "Member", TgUsername: "newbie",
		Score: 0, HasCoffeeBan: false, IsClubMember: true, CreatedAt: testCreatedAt, UpdatedAt: testCreatedAt,
	}
	mock.ExpectQuery(exactSQL(userByTgIDSQL)).WithArgs(int64(111)).WillReturnRows(sqlmock.NewRows(userColumns))
	mock.ExpectQuery(exactSQL(userInsertSQL)).WithArgs(int64(111), "New", "Member", "newbie").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(77))
	mock.ExpectQuery(exactSQL(userByIDSQL)).WithArgs(77).
		WillReturnRows(sqlmock.NewRows(userColumns).AddRow(userRow(created)...))

	user, err := repo.GetOrCreate(&gotgbot.User{Id: 111, FirstName: "New", LastName: "Member", Username: "newbie"})

	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, created, *user)
}

func TestUserRepository_GetOrCreate_CreateError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewUserRepository(db)

	mock.ExpectQuery(exactSQL(userByTgIDSQL)).WithArgs(int64(111)).WillReturnRows(sqlmock.NewRows(userColumns))
	mock.ExpectQuery(exactSQL(userInsertSQL)).WithArgs(int64(111), "New", "", "").WillReturnError(errDB)

	user, err := repo.GetOrCreate(&gotgbot.User{Id: 111, FirstName: "New"})

	require.Error(t, err)
	assert.Nil(t, user)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to create user in getOrCreateUser")
}

func TestUserRepository_GetOrCreate_GetCreatedUserError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewUserRepository(db)

	mock.ExpectQuery(exactSQL(userByTgIDSQL)).WithArgs(int64(111)).WillReturnRows(sqlmock.NewRows(userColumns))
	mock.ExpectQuery(exactSQL(userInsertSQL)).WithArgs(int64(111), "New", "", "").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(77))
	mock.ExpectQuery(exactSQL(userByIDSQL)).WithArgs(77).WillReturnRows(sqlmock.NewRows(userColumns))

	user, err := repo.GetOrCreate(&gotgbot.User{Id: 111, FirstName: "New"})

	require.Error(t, err)
	assert.Nil(t, user)
	assert.ErrorIs(t, err, sql.ErrNoRows)
	assert.Contains(t, err.Error(), "failed to get created user in getOrCreateUser")
}

func TestUserRepository_GetOrFullCreate_Success(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewUserRepository(db)

	existing := sampleUser()
	mock.ExpectQuery(exactSQL(userByTgIDSQL)).WithArgs(existing.TgID).
		WillReturnRows(sqlmock.NewRows(userColumns).AddRow(userRow(existing)...))
	mock.ExpectQuery(exactSQL(profileByUserIDSQL)).WithArgs(existing.ID).
		WillReturnRows(sqlmock.NewRows(profileColumns).AddRow(3, existing.ID, "bio", int64(555), testCreatedAt, testUpdatedAt))

	user, profile, err := repo.GetOrFullCreate(&gotgbot.User{Id: existing.TgID, FirstName: "Ivan", LastName: "Petrov", Username: "ivanp"})

	require.NoError(t, err)
	assert.Equal(t, existing, *user)
	require.NotNil(t, profile)
	assert.Equal(t, 3, profile.ID)
	assert.Equal(t, existing.ID, profile.UserID)
	assert.Equal(t, sql.NullInt64{Int64: 555, Valid: true}, profile.PublishedMessageID)
}

func TestUserRepository_GetOrFullCreate_UserError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewUserRepository(db)

	mock.ExpectQuery(exactSQL(userByTgIDSQL)).WithArgs(int64(111)).WillReturnError(errDB)

	user, profile, err := repo.GetOrFullCreate(&gotgbot.User{Id: 111})

	require.Error(t, err)
	assert.Nil(t, user)
	assert.Nil(t, profile)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to get user in getOrCreateWithProfile")
}

func TestUserRepository_GetOrFullCreate_ProfileError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewUserRepository(db)

	existing := sampleUser()
	mock.ExpectQuery(exactSQL(userByTgIDSQL)).WithArgs(existing.TgID).
		WillReturnRows(sqlmock.NewRows(userColumns).AddRow(userRow(existing)...))
	mock.ExpectQuery(exactSQL(profileByUserIDSQL)).WithArgs(existing.ID).WillReturnError(errDB)

	user, profile, err := repo.GetOrFullCreate(&gotgbot.User{Id: existing.TgID, Username: "ivanp"})

	require.Error(t, err)
	assert.Nil(t, user)
	assert.Nil(t, profile)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to get profile in getOrCreateWithProfile")
}
