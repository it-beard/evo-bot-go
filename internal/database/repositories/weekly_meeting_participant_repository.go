package repositories

import (
	"database/sql"
	"evo-bot-go/internal/models"
	// "time" // Not strictly needed for these specific queries if using NOW()
)

type WeeklyMeetingParticipantRepository struct {
	db *sql.DB
}

func NewWeeklyMeetingParticipantRepository(db *sql.DB) *WeeklyMeetingParticipantRepository {
	return &WeeklyMeetingParticipantRepository{db: db}
}

func (r *WeeklyMeetingParticipantRepository) UpsertParticipant(participant models.WeeklyMeetingParticipant) error {
	query := `
		INSERT INTO weekly_meeting_participants (poll_id, user_id, is_participating, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (poll_id, user_id) DO UPDATE SET
			is_participating = EXCLUDED.is_participating,
			updated_at = NOW()
	`
	_, err := r.db.Exec(query, participant.PollID, participant.UserID, participant.IsParticipating)
	return err
}

func (r *WeeklyMeetingParticipantRepository) RemoveParticipant(pollID int64, userID int64) error {
	query := "DELETE FROM weekly_meeting_participants WHERE poll_id = $1 AND user_id = $2"
	_, err := r.db.Exec(query, pollID, userID)
	return err
}

func (r *WeeklyMeetingParticipantRepository) GetParticipant(pollID int64, userID int64) (*models.WeeklyMeetingParticipant, error) {
	query := "SELECT id, poll_id, user_id, is_participating, created_at, updated_at FROM weekly_meeting_participants WHERE poll_id = $1 AND user_id = $2"
	row := r.db.QueryRow(query, pollID, userID)
	p := &models.WeeklyMeetingParticipant{}
	err := row.Scan(&p.ID, &p.PollID, &p.UserID, &p.IsParticipating, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Or a specific "not found" error
		}
		return nil, err
	}
	return p, nil
}

// GetParticipatingUsers retrieves all users who are participating in a given poll
func (r *WeeklyMeetingParticipantRepository) GetParticipatingUsers(pollID int64) ([]User, error) {
	// Note: We are returning []User from this package, which is defined in user_repository.go
	// This creates a slight package dependency issue if we consider repositories strictly separate.
	// Ideally, User model would be in a common models package.
	// For now, we will use the User struct defined in the repositories package.
	query := `
		SELECT u.id, u.tg_id, u.firstname, u.lastname, u.tg_username 
		FROM users u
		JOIN weekly_meeting_participants wmp ON u.id = wmp.user_id
		WHERE wmp.poll_id = $1 AND wmp.is_participating = TRUE
	`
	rows, err := r.db.Query(query, pollID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User // This refers to repositories.User
		// Ensure Scan matches the fields in repositories.User
		if err := rows.Scan(&user.ID, &user.TgID, &user.Firstname, &user.Lastname, &user.TgUsername); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}
