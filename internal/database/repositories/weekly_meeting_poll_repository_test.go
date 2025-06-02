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

func TestCreatePoll_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := NewWeeklyMeetingPollRepository(db)

	now := time.Now()
	poll := models.WeeklyMeetingPoll{
		MessageID:      12345,
		ChatID:         -100123,
		WeekStartDate:  now.Truncate(24 * time.Hour),
		TelegramPollID: "poll123xyz",
		// CreatedAt will be set by the method if zero, or use this if provided
	}
	expectedID := int64(1)

	// Expect QueryRow to be called with the correct SQL statement (escaped for regex)
	// and arguments. It should return a row with the new ID.
	mock.ExpectQuery(regexp.QuoteMeta(
		`INSERT INTO weekly_meeting_polls (message_id, chat_id, week_start_date, telegram_poll_id, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`)).
		WithArgs(poll.MessageID, poll.ChatID, poll.WeekStartDate, poll.TelegramPollID, sqlmock.AnyArg()). // sqlmock.AnyArg() for time.Now()
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(expectedID))

	id, err := repo.CreatePoll(poll)

	assert.NoError(t, err)
	assert.Equal(t, expectedID, id)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreatePoll_WithProvidedCreatedAt(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := NewWeeklyMeetingPollRepository(db)

	providedTime := time.Now().Add(-1 * time.Hour).Truncate(time.Second) // Ensure it's specific
	poll := models.WeeklyMeetingPoll{
		MessageID:      12345,
		ChatID:         -100123,
		WeekStartDate:  time.Now().Truncate(24 * time.Hour),
		TelegramPollID: "poll123xyz",
		CreatedAt:      providedTime,
	}
	expectedID := int64(1)

	mock.ExpectQuery(regexp.QuoteMeta(
		`INSERT INTO weekly_meeting_polls (message_id, chat_id, week_start_date, telegram_poll_id, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`)).
		WithArgs(poll.MessageID, poll.ChatID, poll.WeekStartDate, poll.TelegramPollID, providedTime).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(expectedID))

	id, err := repo.CreatePoll(poll)

	assert.NoError(t, err)
	assert.Equal(t, expectedID, id)
	assert.NoError(t, mock.ExpectationsWereMet())
}


func TestCreatePoll_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := NewWeeklyMeetingPollRepository(db)

	poll := models.WeeklyMeetingPoll{
		MessageID:      12345,
		ChatID:         -100123,
		WeekStartDate:  time.Now().Truncate(24 * time.Hour),
		TelegramPollID: "poll123xyz",
	}
	dbError := errors.New("database error")

	mock.ExpectQuery(regexp.QuoteMeta(
		`INSERT INTO weekly_meeting_polls (message_id, chat_id, week_start_date, telegram_poll_id, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`)).
		WithArgs(poll.MessageID, poll.ChatID, poll.WeekStartDate, poll.TelegramPollID, sqlmock.AnyArg()).
		WillReturnError(dbError)

	id, err := repo.CreatePoll(poll)

	assert.Error(t, err)
	assert.EqualError(t, err, dbError.Error())
	assert.Equal(t, int64(0), id)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPollByTelegramPollID_Found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := NewWeeklyMeetingPollRepository(db)
	telegramPollID := "test-poll-id"
	expectedPoll := &models.WeeklyMeetingPoll{
		ID:             1,
		MessageID:      123,
		ChatID:         -100,
		WeekStartDate:  time.Now().Truncate(24 * time.Hour),
		TelegramPollID: telegramPollID,
		CreatedAt:      time.Now(),
	}

	rows := sqlmock.NewRows([]string{"id", "message_id", "chat_id", "week_start_date", "telegram_poll_id", "created_at"}).
		AddRow(expectedPoll.ID, expectedPoll.MessageID, expectedPoll.ChatID, expectedPoll.WeekStartDate, expectedPoll.TelegramPollID, expectedPoll.CreatedAt)

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT id, message_id, chat_id, week_start_date, telegram_poll_id, created_at
		FROM weekly_meeting_polls
		WHERE telegram_poll_id = $1`)).
		WithArgs(telegramPollID).
		WillReturnRows(rows)

	poll, err := repo.GetPollByTelegramPollID(telegramPollID)

	assert.NoError(t, err)
	assert.NotNil(t, poll)
	assert.Equal(t, expectedPoll, poll)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPollByTelegramPollID_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := NewWeeklyMeetingPollRepository(db)
	telegramPollID := "non-existent-poll-id"

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT id, message_id, chat_id, week_start_date, telegram_poll_id, created_at
		FROM weekly_meeting_polls
		WHERE telegram_poll_id = $1`)).
		WithArgs(telegramPollID).
		WillReturnError(sql.ErrNoRows)

	poll, err := repo.GetPollByTelegramPollID(telegramPollID)

	assert.NoError(t, err) // Repository transforms sql.ErrNoRows to nil, nil
	assert.Nil(t, poll)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPollByTelegramPollID_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := NewWeeklyMeetingPollRepository(db)
	telegramPollID := "test-poll-id"
	dbError := errors.New("database error")

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT id, message_id, chat_id, week_start_date, telegram_poll_id, created_at
		FROM weekly_meeting_polls
		WHERE telegram_poll_id = $1`)).
		WithArgs(telegramPollID).
		WillReturnError(dbError)

	poll, err := repo.GetPollByTelegramPollID(telegramPollID)

	assert.Error(t, err)
	assert.EqualError(t, err, dbError.Error())
	assert.Nil(t, poll)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetLatestPollForChat_Found(t *testing.T) {
    db, mock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
    }
    defer db.Close()

    repo := NewWeeklyMeetingPollRepository(db)
    chatID := int64(-10012345)
    expectedPoll := &models.WeeklyMeetingPoll{
        ID:             1,
        MessageID:      123,
        ChatID:         chatID,
        WeekStartDate:  time.Now().Truncate(24 * time.Hour),
        TelegramPollID: "poll-id-123",
        CreatedAt:      time.Now(),
    }

    rows := sqlmock.NewRows([]string{"id", "message_id", "chat_id", "week_start_date", "telegram_poll_id", "created_at"}).
        AddRow(expectedPoll.ID, expectedPoll.MessageID, expectedPoll.ChatID, expectedPoll.WeekStartDate, expectedPoll.TelegramPollID, expectedPoll.CreatedAt)

    mock.ExpectQuery(regexp.QuoteMeta(
        `SELECT id, message_id, chat_id, week_start_date, telegram_poll_id, created_at
        FROM weekly_meeting_polls
        WHERE chat_id = $1
        ORDER BY week_start_date DESC, id DESC 
        LIMIT 1`)).
        WithArgs(chatID).
        WillReturnRows(rows)

    poll, err := repo.GetLatestPollForChat(chatID)

    assert.NoError(t, err)
    assert.NotNil(t, poll)
    assert.Equal(t, expectedPoll, poll)
    assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetLatestPollForChat_NotFound(t *testing.T) {
    db, mock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
    }
    defer db.Close()

    repo := NewWeeklyMeetingPollRepository(db)
    chatID := int64(-10012345)

    mock.ExpectQuery(regexp.QuoteMeta(
        `SELECT id, message_id, chat_id, week_start_date, telegram_poll_id, created_at
        FROM weekly_meeting_polls
        WHERE chat_id = $1
        ORDER BY week_start_date DESC, id DESC 
        LIMIT 1`)).
        WithArgs(chatID).
        WillReturnError(sql.ErrNoRows)

    poll, err := repo.GetLatestPollForChat(chatID)

    assert.NoError(t, err) // Repository transforms sql.ErrNoRows to nil, nil
    assert.Nil(t, poll)
    assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetLatestPollForChat_Error(t *testing.T) {
    db, mock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
    }
    defer db.Close()

    repo := NewWeeklyMeetingPollRepository(db)
    chatID := int64(-10012345)
    dbError := errors.New("some db error")

    mock.ExpectQuery(regexp.QuoteMeta(
        `SELECT id, message_id, chat_id, week_start_date, telegram_poll_id, created_at
        FROM weekly_meeting_polls
        WHERE chat_id = $1
        ORDER BY week_start_date DESC, id DESC 
        LIMIT 1`)).
        WithArgs(chatID).
        WillReturnError(dbError)

    poll, err := repo.GetLatestPollForChat(chatID)

    assert.Error(t, err)
    assert.EqualError(t, err, dbError.Error())
    assert.Nil(t, poll)
    assert.NoError(t, mock.ExpectationsWereMet())
}
