package repositories

import (
	"database/sql"
	"evo-bot-go/internal/models"
	"time"
)

type WeeklyMeetingPollRepository struct {
	db *sql.DB
}

func NewWeeklyMeetingPollRepository(db *sql.DB) *WeeklyMeetingPollRepository {
	return &WeeklyMeetingPollRepository{db: db}
}

func (r *WeeklyMeetingPollRepository) CreatePoll(poll models.WeeklyMeetingPoll) (int64, error) {
	query := `
		INSERT INTO weekly_meeting_polls (message_id, chat_id, week_start_date, telegram_poll_id, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	var id int64
	// If CreatedAt is the zero value, set it to time.Now()
	if poll.CreatedAt.IsZero() {
		poll.CreatedAt = time.Now()
	}
	err := r.db.QueryRow(
		query,
		poll.MessageID,
		poll.ChatID,
		poll.WeekStartDate,
		poll.TelegramPollID, // New field
		poll.CreatedAt,
	).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (r *WeeklyMeetingPollRepository) GetPollByTelegramPollID(telegramPollID string) (*models.WeeklyMeetingPoll, error) {
	query := `
		SELECT id, message_id, chat_id, week_start_date, telegram_poll_id, created_at
		FROM weekly_meeting_polls
		WHERE telegram_poll_id = $1
	`
	poll := &models.WeeklyMeetingPoll{}
	err := r.db.QueryRow(query, telegramPollID).Scan(
		&poll.ID,
		&poll.MessageID,
		&poll.ChatID,
		&poll.WeekStartDate,
		&poll.TelegramPollID,
		&poll.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Or a custom not found error
		}
		return nil, err
	}
	return poll, nil
}

// GetLatestPollForChat retrieves the latest poll for a given chat ID
func (r *WeeklyMeetingPollRepository) GetLatestPollForChat(chatID int64) (*models.WeeklyMeetingPoll, error) {
	query := `
		SELECT id, message_id, chat_id, week_start_date, telegram_poll_id, created_at
		FROM weekly_meeting_polls
		WHERE chat_id = $1
		ORDER BY week_start_date DESC, id DESC 
		LIMIT 1
	`
	poll := &models.WeeklyMeetingPoll{}
	err := r.db.QueryRow(query, chatID).Scan(
		&poll.ID,
		&poll.MessageID,
		&poll.ChatID,
		&poll.WeekStartDate,
		&poll.TelegramPollID,
		&poll.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No poll found for this chat
		}
		return nil, err
	}
	return poll, nil
}
