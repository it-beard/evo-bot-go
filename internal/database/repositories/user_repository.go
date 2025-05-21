package repositories

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// User represents a row in the users table
type User struct {
	ID           int
	TgID         int64
	Firstname    string
	Lastname     string
	TgUsername   string
	Score        int
	HasCoffeeBan bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// UserRepository handles database operations for users
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository creates a new UserRepository
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// GetByID retrieves a user by ID
func (r *UserRepository) GetByID(id int) (*User, error) {
	query := `
		SELECT id, tg_id, firstname, lastname, tg_username, score, has_coffee_ban, created_at, updated_at
		FROM users
		WHERE id = $1`

	var user User
	err := r.db.QueryRow(query, id).Scan(
		&user.ID,
		&user.TgID,
		&user.Firstname,
		&user.Lastname,
		&user.TgUsername,
		&user.Score,
		&user.HasCoffeeBan,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get user with ID %d: %w", id, err)
	}

	return &user, nil
}

// GetByTelegramID retrieves a user by Telegram ID
func (r *UserRepository) GetByTelegramID(tgID int64) (*User, error) {
	query := `
		SELECT id, tg_id, firstname, lastname, tg_username, score, has_coffee_ban, created_at, updated_at
		FROM users
		WHERE tg_id = $1`

	var user User
	err := r.db.QueryRow(query, tgID).Scan(
		&user.ID,
		&user.TgID,
		&user.Firstname,
		&user.Lastname,
		&user.TgUsername,
		&user.Score,
		&user.HasCoffeeBan,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get user with Telegram ID %d: %w", tgID, err)
	}

	return &user, nil
}

// GetByTelegramUsername retrieves a user by Telegram username
func (r *UserRepository) GetByTelegramUsername(tgUsername string) (*User, error) {
	query := `
		SELECT id, tg_id, firstname, lastname, tg_username, score, has_coffee_ban, created_at, updated_at
		FROM users
		WHERE tg_username = $1`

	var user User
	err := r.db.QueryRow(query, tgUsername).Scan(
		&user.ID,
		&user.TgID,
		&user.Firstname,
		&user.Lastname,
		&user.TgUsername,
		&user.Score,
		&user.HasCoffeeBan,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get user with Telegram username %s: %w", tgUsername, err)
	}

	return &user, nil
}

// Create inserts a new user record into the database
func (r *UserRepository) Create(tgID int64, firstname string, lastname string, username string) (int, error) {
	var id int
	query := `INSERT INTO users (tg_id, firstname, lastname, tg_username, score, has_coffee_ban) 
			VALUES ($1, $2, $3, $4, 0, false) RETURNING id`
	err := r.db.QueryRow(query, tgID, firstname, lastname, username).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to insert user: %w", err)
	}
	return id, nil
}

// Update updates user fields
func (r *UserRepository) Update(id int, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return fmt.Errorf("no fields to update for user with ID %d", id)
	}

	// Build query dynamically based on provided fields
	query := "UPDATE users SET updated_at = NOW()"
	args := []interface{}{}
	i := 1

	for key, value := range fields {
		query += fmt.Sprintf(", %s = $%d", key, i)
		args = append(args, value)
		i++
	}

	query += fmt.Sprintf(" WHERE id = $%d", i)
	args = append(args, id)

	result, err := r.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update user with ID %d: %w", id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Could not get rows affected after update: %v", err)
	} else if rowsAffected == 0 {
		return fmt.Errorf("no user found with ID %d to update", id)
	}

	return nil
}

// Delete removes a user record from the database by its ID
func (r *UserRepository) Delete(id int) error {
	query := `DELETE FROM users WHERE id = $1`
	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user with ID %d: %w", id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Could not get rows affected after delete: %v", err)
	} else if rowsAffected == 0 {
		return fmt.Errorf("no user found with ID %d to delete", id)
	}

	return nil
}

// UpdateScore updates a user's score
func (r *UserRepository) UpdateScore(id int, score int) error {
	query := `UPDATE users SET score = $1, updated_at = NOW() WHERE id = $2`
	result, err := r.db.Exec(query, score, id)
	if err != nil {
		return fmt.Errorf("failed to update score for user with ID %d: %w", id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Could not get rows affected after update: %v", err)
	} else if rowsAffected == 0 {
		return fmt.Errorf("no user found with ID %d to update score", id)
	}

	return nil
}

// SetCoffeeBan sets a user's coffee ban status
func (r *UserRepository) SetCoffeeBan(id int, banned bool) error {
	query := `UPDATE users SET has_coffee_ban = $1, updated_at = NOW() WHERE id = $2`
	result, err := r.db.Exec(query, banned, id)
	if err != nil {
		return fmt.Errorf("failed to update coffee ban status for user with ID %d: %w", id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Could not get rows affected after update: %v", err)
	} else if rowsAffected == 0 {
		return fmt.Errorf("no user found with ID %d to update coffee ban status", id)
	}

	return nil
}

func (h *UserRepository) GetOrCreateUser(tgUser *gotgbot.User) (*User, error) {
	// Try to get user by Telegram ID
	dbUser, err := h.GetByTelegramID(int64(tgUser.Id))
	if err == nil {
		// User exists, return it
		return dbUser, nil
	}

	// If error is not "no rows", it's a real error
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("ProfileHandler: failed to get user in getOrCreateUser: %w", err)
	}

	// User doesn't exist, create new user
	userID, err := h.Create(
		int64(tgUser.Id),
		tgUser.FirstName,
		tgUser.LastName,
		tgUser.Username,
	)
	if err != nil {
		return nil, fmt.Errorf("ProfileHandler: failed to create user in getOrCreateUser: %w", err)
	}

	// Get the newly created user
	dbUser, err = h.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("ProfileHandler: failed to get created user in getOrCreateUser: %w", err)
	}

	return dbUser, nil
}
