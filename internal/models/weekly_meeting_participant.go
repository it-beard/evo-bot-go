package models

import "time"

type WeeklyMeetingParticipant struct {
	ID              int64     `db:"id"`
	PollID          int64     `db:"poll_id"`
	UserID          int64     `db:"user_id"`
	IsParticipating bool      `db:"is_participating"`
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
}
