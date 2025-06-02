package models

import "time"

type WeeklyMeetingPoll struct {
	ID            int64     `db:"id"`
	MessageID     int64     `db:"message_id"`
	ChatID        int64     `db:"chat_id"`
	WeekStartDate time.Time `db:"week_start_date"`
	TelegramPollID string   `db:"telegram_poll_id"`
	CreatedAt     time.Time `db:"created_at"`
}
