package repositories

import (
	"database/sql"
	"time"
)

type RandomCoffeePoll struct {
	ID             int64     `db:"id"`
	MessageID      int64     `db:"message_id"`
	WeekStartDate  time.Time `db:"week_start_date"`
	TelegramPollID string    `db:"telegram_poll_id"`
	CreatedAt      time.Time `db:"created_at"`
}

type RandomCoffeePollRepository struct {
	db *sql.DB
}

func NewRandomCoffeePollRepository(db *sql.DB) *RandomCoffeePollRepository {
	return &RandomCoffeePollRepository{db: db}
}

func (r *RandomCoffeePollRepository) CreatePoll(poll RandomCoffeePoll) (int64, error) {
	query := `
		INSERT INTO random_coffee_polls (message_id, week_start_date, telegram_poll_id, created_at)
		VALUES ($1, $2, $3, $4)
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
		poll.WeekStartDate,
		poll.TelegramPollID,
		poll.CreatedAt,
	).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (r *RandomCoffeePollRepository) GetPollByTelegramPollID(telegramPollID string) (*RandomCoffeePoll, error) {
	query := `
		SELECT id, message_id, week_start_date, telegram_poll_id, created_at
		FROM random_coffee_polls
		WHERE telegram_poll_id = $1
	`
	poll := &RandomCoffeePoll{}
	err := r.db.QueryRow(query, telegramPollID).Scan(
		&poll.ID,
		&poll.MessageID,
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

// GetLatestPoll retrieves the latest poll globally
func (r *RandomCoffeePollRepository) GetLatestPoll() (*RandomCoffeePoll, error) {
	query := `
		SELECT id, message_id, week_start_date, telegram_poll_id, created_at
		FROM random_coffee_polls
		ORDER BY week_start_date DESC, id DESC 
		LIMIT 1
	`
	poll := &RandomCoffeePoll{}
	err := r.db.QueryRow(query).Scan(
		&poll.ID,
		&poll.MessageID,
		&poll.WeekStartDate,
		&poll.TelegramPollID,
		&poll.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No poll found
		}
		return nil, err
	}
	return poll, nil
}
