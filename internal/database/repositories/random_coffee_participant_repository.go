package repositories

import (
	"database/sql"
	"time"
)

type RandomCoffeeParticipant struct {
	ID              int64     `db:"id"`
	PollID          int64     `db:"poll_id"`
	UserID          int64     `db:"user_id"`
	IsParticipating bool      `db:"is_participating"`
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
}

type RandomCoffeeParticipantRepository struct {
	db *sql.DB
}

func NewRandomCoffeeParticipantRepository(db *sql.DB) *RandomCoffeeParticipantRepository {
	return &RandomCoffeeParticipantRepository{db: db}
}

func (r *RandomCoffeeParticipantRepository) UpsertParticipant(participant RandomCoffeeParticipant) error {
	query := `
		INSERT INTO random_coffee_participants (poll_id, user_id, is_participating, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (poll_id, user_id) DO UPDATE SET
			is_participating = EXCLUDED.is_participating,
			updated_at = NOW()
	`
	_, err := r.db.Exec(query, participant.PollID, participant.UserID, participant.IsParticipating)
	return err
}

func (r *RandomCoffeeParticipantRepository) RemoveParticipant(pollID int64, userID int64) error {
	query := "DELETE FROM random_coffee_participants WHERE poll_id = $1 AND user_id = $2"
	_, err := r.db.Exec(query, pollID, userID)
	return err
}

func (r *RandomCoffeeParticipantRepository) GetParticipant(pollID int64, userID int64) (*RandomCoffeeParticipant, error) {
	query := "SELECT id, poll_id, user_id, is_participating, created_at, updated_at FROM random_coffee_participants WHERE poll_id = $1 AND user_id = $2"
	row := r.db.QueryRow(query, pollID, userID)
	p := &RandomCoffeeParticipant{}
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
func (r *RandomCoffeeParticipantRepository) GetParticipatingUsers(pollID int64) ([]User, error) {
	query := `
		SELECT u.id, u.tg_id, u.firstname, u.lastname, u.tg_username 
		FROM users u
		JOIN random_coffee_participants rpc ON u.id = rpc.user_id
		WHERE rpc.poll_id = $1 AND rpc.is_participating = TRUE
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
