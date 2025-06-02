package repositories

import (
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"evo-bot-go/internal/models"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestUpsertParticipant_InsertSuccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := NewWeeklyMeetingParticipantRepository(db)
	participant := models.WeeklyMeetingParticipant{
		PollID:          1,
		UserID:          10,
		IsParticipating: true,
	}

	mock.ExpectExec(regexp.QuoteMeta(
		`INSERT INTO weekly_meeting_participants (poll_id, user_id, is_participating, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (poll_id, user_id) DO UPDATE SET
			is_participating = EXCLUDED.is_participating,
			updated_at = NOW()`)).
		WithArgs(participant.PollID, participant.UserID, participant.IsParticipating).
		WillReturnResult(sqlmock.NewResult(1, 1)) // Assuming insert, 1 row affected

	err = repo.UpsertParticipant(participant)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpsertParticipant_UpdateSuccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := NewWeeklyMeetingParticipantRepository(db)
	participant := models.WeeklyMeetingParticipant{
		PollID:          1,
		UserID:          10,
		IsParticipating: false, // Changed mind
	}

	// For an update due to ON CONFLICT, RowsAffected might be 1 (if the SET clause results in a change)
	// or 0 (if the values in SET are the same as existing, though updated_at=NOW() usually makes it 1).
	// PostgreSQL typically returns 1 for a successful DO UPDATE that changes a row.
	mock.ExpectExec(regexp.QuoteMeta(
		`INSERT INTO weekly_meeting_participants (poll_id, user_id, is_participating, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (poll_id, user_id) DO UPDATE SET
			is_participating = EXCLUDED.is_participating,
			updated_at = NOW()`)).
		WithArgs(participant.PollID, participant.UserID, participant.IsParticipating).
		WillReturnResult(sqlmock.NewResult(0, 1)) // LastInsertId is 0 for conflict, 1 row affected

	err = repo.UpsertParticipant(participant)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpsertParticipant_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := NewWeeklyMeetingParticipantRepository(db)
	participant := models.WeeklyMeetingParticipant{
		PollID:          1,
		UserID:          10,
		IsParticipating: true,
	}
	dbError := errors.New("upsert failed")

	mock.ExpectExec(regexp.QuoteMeta(
		`INSERT INTO weekly_meeting_participants (poll_id, user_id, is_participating, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (poll_id, user_id) DO UPDATE SET
			is_participating = EXCLUDED.is_participating,
			updated_at = NOW()`)).
		WithArgs(participant.PollID, participant.UserID, participant.IsParticipating).
		WillReturnError(dbError)

	err = repo.UpsertParticipant(participant)
	assert.Error(t, err)
	assert.EqualError(t, err, dbError.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRemoveParticipant_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := NewWeeklyMeetingParticipantRepository(db)
	pollID := int64(1)
	userID := int64(10)

	mock.ExpectExec(regexp.QuoteMeta(
		"DELETE FROM weekly_meeting_participants WHERE poll_id = $1 AND user_id = $2")).
		WithArgs(pollID, userID).
		WillReturnResult(sqlmock.NewResult(0, 1)) // 1 row affected

	err = repo.RemoveParticipant(pollID, userID)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRemoveParticipant_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := NewWeeklyMeetingParticipantRepository(db)
	pollID := int64(1)
	userID := int64(10) // Non-existent participant

	mock.ExpectExec(regexp.QuoteMeta(
		"DELETE FROM weekly_meeting_participants WHERE poll_id = $1 AND user_id = $2")).
		WithArgs(pollID, userID).
		WillReturnResult(sqlmock.NewResult(0, 0)) // 0 rows affected

	err = repo.RemoveParticipant(pollID, userID)
	assert.NoError(t, err) // DELETE with no matching rows is not an error
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRemoveParticipant_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := NewWeeklyMeetingParticipantRepository(db)
	pollID := int64(1)
	userID := int64(10)
	dbError := errors.New("delete failed")

	mock.ExpectExec(regexp.QuoteMeta(
		"DELETE FROM weekly_meeting_participants WHERE poll_id = $1 AND user_id = $2")).
		WithArgs(pollID, userID).
		WillReturnError(dbError)

	err = repo.RemoveParticipant(pollID, userID)
	assert.Error(t, err)
	assert.EqualError(t, err, dbError.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetParticipatingUsers_Success_WithParticipants(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := NewWeeklyMeetingParticipantRepository(db)
	pollID := int64(1)

	expectedUsers := []User{ // repositories.User
		{ID: 1, TgID: 101, Firstname: "Alice", Lastname: "Smith", TgUsername: "alice"},
		{ID: 2, TgID: 102, Firstname: "Bob", Lastname: "Johnson", TgUsername: "bobby"},
	}

	rows := sqlmock.NewRows([]string{"id", "tg_id", "firstname", "lastname", "tg_username"}).
		AddRow(expectedUsers[0].ID, expectedUsers[0].TgID, expectedUsers[0].Firstname, expectedUsers[0].Lastname, expectedUsers[0].TgUsername).
		AddRow(expectedUsers[1].ID, expectedUsers[1].TgID, expectedUsers[1].Firstname, expectedUsers[1].Lastname, expectedUsers[1].TgUsername)

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT u.id, u.tg_id, u.firstname, u.lastname, u.tg_username 
		FROM users u
		JOIN weekly_meeting_participants wmp ON u.id = wmp.user_id
		WHERE wmp.poll_id = $1 AND wmp.is_participating = TRUE`)).
		WithArgs(pollID).
		WillReturnRows(rows)

	users, err := repo.GetParticipatingUsers(pollID)
	assert.NoError(t, err)
	assert.NotNil(t, users)
	assert.Equal(t, expectedUsers, users)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetParticipatingUsers_Success_NoParticipants(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := NewWeeklyMeetingParticipantRepository(db)
	pollID := int64(1)

	rows := sqlmock.NewRows([]string{"id", "tg_id", "firstname", "lastname", "tg_username"}) // No rows added

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT u.id, u.tg_id, u.firstname, u.lastname, u.tg_username 
		FROM users u
		JOIN weekly_meeting_participants wmp ON u.id = wmp.user_id
		WHERE wmp.poll_id = $1 AND wmp.is_participating = TRUE`)).
		WithArgs(pollID).
		WillReturnRows(rows)

	users, err := repo.GetParticipatingUsers(pollID)
	assert.NoError(t, err)
	assert.NotNil(t, users) // Should be an empty slice, not nil
	assert.Len(t, users, 0)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetParticipatingUsers_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := NewWeeklyMeetingParticipantRepository(db)
	pollID := int64(1)
	dbError := errors.New("query failed")

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT u.id, u.tg_id, u.firstname, u.lastname, u.tg_username 
		FROM users u
		JOIN weekly_meeting_participants wmp ON u.id = wmp.user_id
		WHERE wmp.poll_id = $1 AND wmp.is_participating = TRUE`)).
		WithArgs(pollID).
		WillReturnError(dbError)

	users, err := repo.GetParticipatingUsers(pollID)
	assert.Error(t, err)
	assert.EqualError(t, err, dbError.Error())
	assert.Nil(t, users)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetParticipant_Found(t *testing.T) {
    db, mock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
    }
    defer db.Close()

    repo := NewWeeklyMeetingParticipantRepository(db)
    pollID := int64(1)
    userID := int64(10)
    
    expectedParticipant := &models.WeeklyMeetingParticipant{
        ID:              1,
        PollID:          pollID,
        UserID:          userID,
        IsParticipating: true,
        CreatedAt:       time.Now().Add(-time.Hour),
        UpdatedAt:       time.Now(),
    }

    rows := sqlmock.NewRows([]string{"id", "poll_id", "user_id", "is_participating", "created_at", "updated_at"}).
        AddRow(expectedParticipant.ID, expectedParticipant.PollID, expectedParticipant.UserID, 
               expectedParticipant.IsParticipating, expectedParticipant.CreatedAt, expectedParticipant.UpdatedAt)

    mock.ExpectQuery(regexp.QuoteMeta(
        "SELECT id, poll_id, user_id, is_participating, created_at, updated_at FROM weekly_meeting_participants WHERE poll_id = $1 AND user_id = $2")).
        WithArgs(pollID, userID).
        WillReturnRows(rows)

    participant, err := repo.GetParticipant(pollID, userID)

    assert.NoError(t, err)
    assert.NotNil(t, participant)
    // Comparing time.Time objects can be tricky due to monotonic clock. Truncate or use EqualTime.
    assert.Equal(t, expectedParticipant.ID, participant.ID)
    assert.Equal(t, expectedParticipant.PollID, participant.PollID)
    assert.Equal(t, expectedParticipant.UserID, participant.UserID)
    assert.Equal(t, expectedParticipant.IsParticipating, participant.IsParticipating)
    assert.WithinDuration(t, expectedParticipant.CreatedAt, participant.CreatedAt, time.Second)
    assert.WithinDuration(t, expectedParticipant.UpdatedAt, participant.UpdatedAt, time.Second)
    assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetParticipant_NotFound(t *testing.T) {
    db, mock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
    }
    defer db.Close()

    repo := NewWeeklyMeetingParticipantRepository(db)
    pollID := int64(1)
    userID := int64(10) // Non-existent

    mock.ExpectQuery(regexp.QuoteMeta(
        "SELECT id, poll_id, user_id, is_participating, created_at, updated_at FROM weekly_meeting_participants WHERE poll_id = $1 AND user_id = $2")).
        WithArgs(pollID, userID).
        WillReturnError(sql.ErrNoRows)

    participant, err := repo.GetParticipant(pollID, userID)

    assert.NoError(t, err) // Repository transforms sql.ErrNoRows to nil, nil
    assert.Nil(t, participant)
    assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetParticipant_Error(t *testing.T) {
    db, mock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
    }
    defer db.Close()

    repo := NewWeeklyMeetingParticipantRepository(db)
    pollID := int64(1)
    userID := int64(10)
    dbError := errors.New("some db error")

    mock.ExpectQuery(regexp.QuoteMeta(
        "SELECT id, poll_id, user_id, is_participating, created_at, updated_at FROM weekly_meeting_participants WHERE poll_id = $1 AND user_id = $2")).
        WithArgs(pollID, userID).
        WillReturnError(dbError)

    participant, err := repo.GetParticipant(pollID, userID)

    assert.Error(t, err)
    assert.EqualError(t, err, dbError.Error())
    assert.Nil(t, participant)
    assert.NoError(t, mock.ExpectationsWereMet())
}
