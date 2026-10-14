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
	profileSelect = `
		SELECT id, user_id, bio, published_message_id, created_at, updated_at
		FROM profiles`
	profileByIDSQL     = profileSelect + ` WHERE id = $1`
	profileByUserIDSQL = profileSelect + ` WHERE user_id = $1`
	profileInsertSQL   = `INSERT INTO profiles (user_id, bio)
			VALUES ($1, $2) RETURNING id`
	profileDeleteSQL          = `DELETE FROM profiles WHERE id = $1`
	profileUpdatePublishedSQL = `UPDATE profiles SET published_message_id = $1, updated_at = NOW() WHERE id = $2`
	profileActiveWithUsersSQL = `
		SELECT
			p.id, p.user_id, p.bio, p.published_message_id, p.created_at, p.updated_at,
			u.id, u.tg_id, u.firstname, u.lastname, u.tg_username, u.score, u.has_coffee_ban, u.is_club_member, u.created_at, u.updated_at
		FROM profiles p
		INNER JOIN users u ON p.user_id = u.id
		WHERE p.bio != '' AND p.bio IS NOT NULL AND p.published_message_id IS NOT NULL AND u.is_club_member = true
		ORDER BY p.updated_at DESC`
)

var (
	profileColumns            = []string{"id", "user_id", "bio", "published_message_id", "created_at", "updated_at"}
	profileWithUserColumns    = append(append([]string{}, profileColumns...), userColumns...)
	errRowsAffectedNotSupport = errors.New("rows affected unsupported")
)

func TestProfileRepository_GetByID_Success(t *testing.T) {
	tests := []struct {
		name      string
		published driver.Value
		want      sql.NullInt64
	}{
		{name: "published", published: int64(9001), want: sql.NullInt64{Int64: 9001, Valid: true}},
		{name: "not published", published: nil, want: sql.NullInt64{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewProfileRepository(db)

			mock.ExpectQuery(exactSQL(profileByIDSQL)).WithArgs(3).
				WillReturnRows(sqlmock.NewRows(profileColumns).AddRow(3, 5, "Go dev", tc.published, testCreatedAt, testUpdatedAt))

			profile, err := repo.GetByID(3)

			require.NoError(t, err)
			require.NotNil(t, profile)
			assert.Equal(t, Profile{
				ID: 3, UserID: 5, Bio: "Go dev", PublishedMessageID: tc.want,
				CreatedAt: testCreatedAt, UpdatedAt: testUpdatedAt,
			}, *profile)
		})
	}
}

func TestProfileRepository_GetByID_NotFoundReturnsErrNoRows(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewProfileRepository(db)

	mock.ExpectQuery(exactSQL(profileByIDSQL)).WithArgs(3).WillReturnRows(sqlmock.NewRows(profileColumns))

	profile, err := repo.GetByID(3)

	assert.Nil(t, profile)
	assert.Equal(t, sql.ErrNoRows, err)
}

func TestProfileRepository_GetByID_QueryError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewProfileRepository(db)

	mock.ExpectQuery(exactSQL(profileByIDSQL)).WithArgs(3).WillReturnError(errDB)

	profile, err := repo.GetByID(3)

	require.Error(t, err)
	assert.Nil(t, profile)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to get profile with ID 3")
}

func TestProfileRepository_Create_Success(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewProfileRepository(db)

	mock.ExpectQuery(exactSQL(profileInsertSQL)).WithArgs(5, "Go dev").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))

	id, err := repo.Create(5, "Go dev")

	require.NoError(t, err)
	assert.Equal(t, 3, id)
}

func TestProfileRepository_Create_QueryError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewProfileRepository(db)

	mock.ExpectQuery(exactSQL(profileInsertSQL)).WithArgs(5, "Go dev").WillReturnError(errDB)

	id, err := repo.Create(5, "Go dev")

	require.Error(t, err)
	assert.Zero(t, id)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to insert profile")
}

func TestProfileRepository_Update_NoFields(t *testing.T) {
	db, _ := testutils.NewMockDB(t)
	repo := NewProfileRepository(db)

	err := repo.Update(3, map[string]interface{}{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no fields to update for profile with ID 3")
}

func TestProfileRepository_Update_SingleField(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewProfileRepository(db)

	mock.ExpectExec(exactSQL(`UPDATE profiles SET updated_at = NOW(), bio = $1 WHERE id = $2`)).
		WithArgs("New bio", 3).
		WillReturnResult(sqlmock.NewResult(0, 1))

	assert.NoError(t, repo.Update(3, map[string]interface{}{"bio": "New bio"}))
}

func TestProfileRepository_Update_MultipleFields(t *testing.T) {
	db, mock, rec := newRecordingMockDB(t)
	repo := NewProfileRepository(db)

	fields := map[string]interface{}{
		"bio":                  "New bio",
		"published_message_id": int64(777),
	}
	captured, matchers := captureArgs(len(fields) + 1)
	mock.ExpectExec(`^UPDATE profiles SET updated_at = NOW\(\), .* WHERE id = \$3$`).
		WithArgs(matchers...).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, repo.Update(3, fields))

	assertDynamicUpdate(t, "profiles", rec.last(), captured, fields, 3)
}

func TestProfileRepository_Update_Errors(t *testing.T) {
	updateSQL := exactSQL(`UPDATE profiles SET updated_at = NOW(), bio = $1 WHERE id = $2`)

	t.Run("exec error", func(t *testing.T) {
		db, mock := testutils.NewMockDB(t)
		mock.ExpectExec(updateSQL).WithArgs("x", 3).WillReturnError(errDB)

		err := NewProfileRepository(db).Update(3, map[string]interface{}{"bio": "x"})

		require.Error(t, err)
		assert.ErrorIs(t, err, errDB)
		assert.Contains(t, err.Error(), "failed to update profile with ID 3")
	})

	t.Run("not found", func(t *testing.T) {
		db, mock := testutils.NewMockDB(t)
		mock.ExpectExec(updateSQL).WithArgs("x", 3).WillReturnResult(sqlmock.NewResult(0, 0))

		err := NewProfileRepository(db).Update(3, map[string]interface{}{"bio": "x"})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no profile found with ID 3 to update")
	})

	t.Run("rows affected error is only logged", func(t *testing.T) {
		db, mock := testutils.NewMockDB(t)
		mock.ExpectExec(updateSQL).WithArgs("x", 3).WillReturnResult(sqlmock.NewErrorResult(errRowsAffectedNotSupport))

		assert.NoError(t, NewProfileRepository(db).Update(3, map[string]interface{}{"bio": "x"}))
	})
}

func TestProfileRepository_ExecMethods(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		args        []driver.Value
		call        func(r *ProfileRepository) error
		execErrText string
		notFound    string
	}{
		{
			name:        "Delete",
			sql:         profileDeleteSQL,
			args:        []driver.Value{3},
			call:        func(r *ProfileRepository) error { return r.Delete(3) },
			execErrText: "failed to delete profile with ID 3",
			notFound:    "no profile found with ID 3 to delete",
		},
		{
			name:        "UpdatePublishedMessageID",
			sql:         profileUpdatePublishedSQL,
			args:        []driver.Value{int64(9001), 3},
			call:        func(r *ProfileRepository) error { return r.UpdatePublishedMessageID(3, 9001) },
			execErrText: "failed to update published_message_id for profile with ID 3",
			notFound:    "no profile found with ID 3 to update published_message_id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name+"/success", func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			mock.ExpectExec(exactSQL(tc.sql)).WithArgs(tc.args...).WillReturnResult(sqlmock.NewResult(0, 1))

			assert.NoError(t, tc.call(NewProfileRepository(db)))
		})

		t.Run(tc.name+"/exec error", func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			mock.ExpectExec(exactSQL(tc.sql)).WithArgs(tc.args...).WillReturnError(errDB)

			err := tc.call(NewProfileRepository(db))

			require.Error(t, err)
			assert.ErrorIs(t, err, errDB)
			assert.Contains(t, err.Error(), tc.execErrText)
		})

		t.Run(tc.name+"/not found", func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			mock.ExpectExec(exactSQL(tc.sql)).WithArgs(tc.args...).WillReturnResult(sqlmock.NewResult(0, 0))

			err := tc.call(NewProfileRepository(db))

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.notFound)
		})

		t.Run(tc.name+"/rows affected error is only logged", func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			mock.ExpectExec(exactSQL(tc.sql)).WithArgs(tc.args...).
				WillReturnResult(sqlmock.NewErrorResult(errRowsAffectedNotSupport))

			assert.NoError(t, tc.call(NewProfileRepository(db)))
		})
	}
}

func TestProfileRepository_GetOrCreateWithBio_ExistingProfile(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewProfileRepository(db)

	mock.ExpectQuery(exactSQL(profileByUserIDSQL)).WithArgs(5).
		WillReturnRows(sqlmock.NewRows(profileColumns).AddRow(3, 5, "Existing bio", nil, testCreatedAt, testUpdatedAt))

	profile, err := repo.GetOrCreateWithBio(5, "ignored bio")

	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.Equal(t, Profile{ID: 3, UserID: 5, Bio: "Existing bio", CreatedAt: testCreatedAt, UpdatedAt: testUpdatedAt}, *profile)
}

func TestProfileRepository_GetOrCreateWithBio_CreatesProfile(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewProfileRepository(db)

	mock.ExpectQuery(exactSQL(profileByUserIDSQL)).WithArgs(5).WillReturnRows(sqlmock.NewRows(profileColumns))
	mock.ExpectQuery(exactSQL(userByIDSQL)).WithArgs(5).
		WillReturnRows(sqlmock.NewRows(userColumns).AddRow(userRow(sampleUser())...))
	mock.ExpectQuery(exactSQL(profileInsertSQL)).WithArgs(5, "Hello").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	mock.ExpectQuery(exactSQL(profileByUserIDSQL)).WithArgs(5).
		WillReturnRows(sqlmock.NewRows(profileColumns).AddRow(3, 5, "Hello", nil, testCreatedAt, testCreatedAt))

	profile, err := repo.GetOrCreateWithBio(5, "Hello")

	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.Equal(t, Profile{ID: 3, UserID: 5, Bio: "Hello", CreatedAt: testCreatedAt, UpdatedAt: testCreatedAt}, *profile)
}

func TestProfileRepository_GetOrCreateWithBio_LookupError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewProfileRepository(db)

	mock.ExpectQuery(exactSQL(profileByUserIDSQL)).WithArgs(5).WillReturnError(errDB)

	profile, err := repo.GetOrCreateWithBio(5, "Hello")

	require.Error(t, err)
	assert.Nil(t, profile)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to get profile in GetOrCreateWithBio")
}

func TestProfileRepository_GetOrCreateWithBio_UserNotFound(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewProfileRepository(db)

	mock.ExpectQuery(exactSQL(profileByUserIDSQL)).WithArgs(5).WillReturnRows(sqlmock.NewRows(profileColumns))
	mock.ExpectQuery(exactSQL(userByIDSQL)).WithArgs(5).WillReturnRows(sqlmock.NewRows(userColumns))

	profile, err := repo.GetOrCreateWithBio(5, "Hello")

	require.Error(t, err)
	assert.Nil(t, profile)
	assert.Contains(t, err.Error(), "user with ID 5 not found, cannot create profile")
}

func TestProfileRepository_GetOrCreateWithBio_UserLookupError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewProfileRepository(db)

	mock.ExpectQuery(exactSQL(profileByUserIDSQL)).WithArgs(5).WillReturnRows(sqlmock.NewRows(profileColumns))
	mock.ExpectQuery(exactSQL(userByIDSQL)).WithArgs(5).WillReturnError(errDB)

	profile, err := repo.GetOrCreateWithBio(5, "Hello")

	require.Error(t, err)
	assert.Nil(t, profile)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to verify user exists in GetOrCreateWithBio")
}

func TestProfileRepository_GetOrCreateWithBio_CreateError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewProfileRepository(db)

	mock.ExpectQuery(exactSQL(profileByUserIDSQL)).WithArgs(5).WillReturnRows(sqlmock.NewRows(profileColumns))
	mock.ExpectQuery(exactSQL(userByIDSQL)).WithArgs(5).
		WillReturnRows(sqlmock.NewRows(userColumns).AddRow(userRow(sampleUser())...))
	mock.ExpectQuery(exactSQL(profileInsertSQL)).WithArgs(5, "Hello").WillReturnError(errDB)

	profile, err := repo.GetOrCreateWithBio(5, "Hello")

	require.Error(t, err)
	assert.Nil(t, profile)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to create profile in GetOrCreateWithBio")
}

func TestProfileRepository_GetOrCreateWithBio_GetCreatedProfileError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewProfileRepository(db)

	mock.ExpectQuery(exactSQL(profileByUserIDSQL)).WithArgs(5).WillReturnRows(sqlmock.NewRows(profileColumns))
	mock.ExpectQuery(exactSQL(userByIDSQL)).WithArgs(5).
		WillReturnRows(sqlmock.NewRows(userColumns).AddRow(userRow(sampleUser())...))
	mock.ExpectQuery(exactSQL(profileInsertSQL)).WithArgs(5, "Hello").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	mock.ExpectQuery(exactSQL(profileByUserIDSQL)).WithArgs(5).WillReturnError(errDB)

	profile, err := repo.GetOrCreateWithBio(5, "Hello")

	require.Error(t, err)
	assert.Nil(t, profile)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to get created profile in GetOrCreateWithBio")
	assert.Contains(t, err.Error(), "failed to get profile for user with ID 5")
}

func TestProfileRepository_GetOrCreate_UsesEmptyBio(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewProfileRepository(db)

	mock.ExpectQuery(exactSQL(profileByUserIDSQL)).WithArgs(5).WillReturnRows(sqlmock.NewRows(profileColumns))
	mock.ExpectQuery(exactSQL(userByIDSQL)).WithArgs(5).
		WillReturnRows(sqlmock.NewRows(userColumns).AddRow(userRow(sampleUser())...))
	mock.ExpectQuery(exactSQL(profileInsertSQL)).WithArgs(5, "").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	mock.ExpectQuery(exactSQL(profileByUserIDSQL)).WithArgs(5).
		WillReturnRows(sqlmock.NewRows(profileColumns).AddRow(3, 5, "", nil, testCreatedAt, testCreatedAt))

	profile, err := repo.GetOrCreate(5)

	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.Equal(t, 3, profile.ID)
	assert.Empty(t, profile.Bio)
}

func TestProfileRepository_GetOrFullCreate_Success(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewProfileRepository(db)

	existing := sampleUser()
	mock.ExpectQuery(exactSQL(userByTgIDSQL)).WithArgs(existing.TgID).
		WillReturnRows(sqlmock.NewRows(userColumns).AddRow(userRow(existing)...))
	mock.ExpectQuery(exactSQL(profileByUserIDSQL)).WithArgs(existing.ID).
		WillReturnRows(sqlmock.NewRows(profileColumns).AddRow(3, existing.ID, "bio", nil, testCreatedAt, testUpdatedAt))

	profile, err := repo.GetOrFullCreate(&gotgbot.User{Id: existing.TgID, FirstName: "Ivan", LastName: "Petrov", Username: "ivanp"})

	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.Equal(t, Profile{ID: 3, UserID: existing.ID, Bio: "bio", CreatedAt: testCreatedAt, UpdatedAt: testUpdatedAt}, *profile)
}

func TestProfileRepository_GetOrFullCreate_Error(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewProfileRepository(db)

	mock.ExpectQuery(exactSQL(userByTgIDSQL)).WithArgs(int64(111)).WillReturnError(errDB)

	profile, err := repo.GetOrFullCreate(&gotgbot.User{Id: 111})

	require.Error(t, err)
	assert.Nil(t, profile)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to get user in GetOrFullCreate")
}

func TestProfileRepository_GetAllActiveWithUserInfo_Success(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewProfileRepository(db)

	user1 := sampleUser()
	user2 := User{
		ID: 6, TgID: 222, Firstname: "Anna", Lastname: "", TgUsername: "", Score: 0,
		HasCoffeeBan: false, IsClubMember: true, CreatedAt: testCreatedAt, UpdatedAt: testCreatedAt,
	}
	row1 := append([]driver.Value{10, user1.ID, "Bio one", int64(1001), testCreatedAt, testUpdatedAt}, userRow(user1)...)
	row2 := append([]driver.Value{11, user2.ID, "Bio two", int64(1002), testUpdatedAt, testUpdatedAt}, userRow(user2)...)
	mock.ExpectQuery(exactSQL(profileActiveWithUsersSQL)).
		WillReturnRows(sqlmock.NewRows(profileWithUserColumns).AddRow(row1...).AddRow(row2...))

	result, err := repo.GetAllActiveWithUserInfo()

	require.NoError(t, err)
	require.Len(t, result, 2)

	assert.Equal(t, Profile{
		ID: 10, UserID: user1.ID, Bio: "Bio one", PublishedMessageID: sql.NullInt64{Int64: 1001, Valid: true},
		CreatedAt: testCreatedAt, UpdatedAt: testUpdatedAt,
	}, *result[0].Profile)
	assert.Equal(t, user1, *result[0].User)

	assert.Equal(t, Profile{
		ID: 11, UserID: user2.ID, Bio: "Bio two", PublishedMessageID: sql.NullInt64{Int64: 1002, Valid: true},
		CreatedAt: testUpdatedAt, UpdatedAt: testUpdatedAt,
	}, *result[1].Profile)
	assert.Equal(t, user2, *result[1].User)

	// Each element must point to its own structs (no loop-variable aliasing)
	assert.NotSame(t, result[0].Profile, result[1].Profile)
	assert.NotSame(t, result[0].User, result[1].User)
}

func TestProfileRepository_GetAllActiveWithUserInfo_Empty(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewProfileRepository(db)

	mock.ExpectQuery(exactSQL(profileActiveWithUsersSQL)).WillReturnRows(sqlmock.NewRows(profileWithUserColumns))

	result, err := repo.GetAllActiveWithUserInfo()

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestProfileRepository_GetAllActiveWithUserInfo_QueryError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewProfileRepository(db)

	mock.ExpectQuery(exactSQL(profileActiveWithUsersSQL)).WillReturnError(errDB)

	result, err := repo.GetAllActiveWithUserInfo()

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to query profiles with users")
}

func TestProfileRepository_GetAllActiveWithUserInfo_ScanError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewProfileRepository(db)

	// Only profile columns: the 16 scan destinations cannot be satisfied
	mock.ExpectQuery(exactSQL(profileActiveWithUsersSQL)).
		WillReturnRows(sqlmock.NewRows(profileColumns).AddRow(10, 5, "Bio", int64(1), testCreatedAt, testUpdatedAt))

	result, err := repo.GetAllActiveWithUserInfo()

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to scan profile with user")
}

func TestProfileRepository_GetAllActiveWithUserInfo_RowsError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewProfileRepository(db)

	row := append([]driver.Value{10, 5, "Bio", int64(1), testCreatedAt, testUpdatedAt}, userRow(sampleUser())...)
	mock.ExpectQuery(exactSQL(profileActiveWithUsersSQL)).
		WillReturnRows(sqlmock.NewRows(profileWithUserColumns).AddRow(row...).RowError(0, errDB))

	result, err := repo.GetAllActiveWithUserInfo()

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "error iterating over profiles with users")
}
