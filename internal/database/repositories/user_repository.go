package repositories

import (
	"database/sql"
	"evo-bot-go/internal/utils"
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
	IsClubMember bool
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

// GetDB returns the database connection
func (r *UserRepository) GetDB() *sql.DB {
	return r.db
}

// GetByID retrieves a user by ID
func (r *UserRepository) GetByID(id int) (*User, error) {
	query := `
		SELECT id, tg_id, firstname, lastname, tg_username, score, has_coffee_ban, is_club_member, created_at, updated_at
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
		&user.IsClubMember,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}

	if err != nil {
		return nil, fmt.Errorf("%s: failed to get user with ID %d: %w", utils.GetCurrentTypeName(), id, err)
	}

	return &user, nil
}

// GetByTelegramID retrieves a user by Telegram ID
func (r *UserRepository) GetByTelegramID(tgID int64) (*User, error) {
	query := `
		SELECT id, tg_id, firstname, lastname, tg_username, score, has_coffee_ban, is_club_member, created_at, updated_at
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
		&user.IsClubMember,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}

	if err != nil {
		return nil, fmt.Errorf("%s: failed to get user with Telegram ID %d: %w", utils.GetCurrentTypeName(), tgID, err)
	}

	return &user, nil
}

// GetByTelegramUsername retrieves a user by Telegram username
func (r *UserRepository) GetByTelegramUsername(tgUsername string) (*User, error) {
	query := `
		SELECT id, tg_id, firstname, lastname, tg_username, score, has_coffee_ban, is_club_member, created_at, updated_at
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
		&user.IsClubMember,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}

	if err != nil {
		return nil, fmt.Errorf("%s: failed to get user with Telegram username %s: %w", utils.GetCurrentTypeName(), tgUsername, err)
	}

	return &user, nil
}

// Create inserts a new user record into the database
func (r *UserRepository) Create(tgID int64, firstname string, lastname string, username string) (int, error) {
	var id int
	query := `INSERT INTO users (tg_id, firstname, lastname, tg_username, score, has_coffee_ban, is_club_member) 
			VALUES ($1, $2, $3, $4, 0, false, true) RETURNING id`
	err := r.db.QueryRow(query, tgID, firstname, lastname, username).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("%s: failed to insert user: %w", utils.GetCurrentTypeName(), err)
	}
	return id, nil
}

// Update updates user fields
func (r *UserRepository) Update(id int, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return fmt.Errorf("%s: no fields to update for user with ID %d", utils.GetCurrentTypeName(), id)
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
		return fmt.Errorf("%s: failed to update user with ID %d: %w", utils.GetCurrentTypeName(), id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("%s: Could not get rows affected after update: %v", utils.GetCurrentTypeName(), err)
	} else if rowsAffected == 0 {
		return fmt.Errorf("%s: no user found with ID %d to update", utils.GetCurrentTypeName(), id)
	}

	return nil
}

// Delete removes a user record from the database by its ID
func (r *UserRepository) Delete(id int) error {
	query := `DELETE FROM users WHERE id = $1`
	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("%s: failed to delete user with ID %d: %w", utils.GetCurrentTypeName(), id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("%s: Could not get rows affected after delete: %v", utils.GetCurrentTypeName(), err)
	} else if rowsAffected == 0 {
		return fmt.Errorf("%s: no user found with ID %d to delete", utils.GetCurrentTypeName(), id)
	}

	return nil
}

// UpdateScore updates a user's score
func (r *UserRepository) UpdateScore(id int, score int) error {
	query := `UPDATE users SET score = $1, updated_at = NOW() WHERE id = $2`
	result, err := r.db.Exec(query, score, id)
	if err != nil {
		return fmt.Errorf("%s: failed to update score for user with ID %d: %w", utils.GetCurrentTypeName(), id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("%s: Could not get rows affected after update: %v", utils.GetCurrentTypeName(), err)
	} else if rowsAffected == 0 {
		return fmt.Errorf("%s: no user found with ID %d to update score", utils.GetCurrentTypeName(), id)
	}

	return nil
}

// SetCoffeeBan sets a user's coffee ban status
func (r *UserRepository) SetCoffeeBan(id int, banned bool) error {
	query := `UPDATE users SET has_coffee_ban = $1, updated_at = NOW() WHERE id = $2`
	result, err := r.db.Exec(query, banned, id)
	if err != nil {
		return fmt.Errorf("%s: failed to update coffee ban status for user with ID %d: %w", utils.GetCurrentTypeName(), id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("%s: Could not get rows affected after update: %v", utils.GetCurrentTypeName(), err)
	} else if rowsAffected == 0 {
		return fmt.Errorf("%s: no user found with ID %d to update coffee ban status", utils.GetCurrentTypeName(), id)
	}

	return nil
}

// SetClubMemberStatus sets a user's club member status
func (r *UserRepository) SetClubMemberStatus(id int, isMember bool) error {
	query := `UPDATE users SET is_club_member = $1, updated_at = NOW() WHERE id = $2`
	result, err := r.db.Exec(query, isMember, id)
	if err != nil {
		return fmt.Errorf("%s: failed to update club member status for user with ID %d: %w", utils.GetCurrentTypeName(), id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("%s: Could not get rows affected after update: %v", utils.GetCurrentTypeName(), err)
	} else if rowsAffected == 0 {
		return fmt.Errorf("%s: no user found with ID %d to update club member status", utils.GetCurrentTypeName(), id)
	}

	return nil
}

func (h *UserRepository) GetOrCreate(tgUser *gotgbot.User) (*User, error) {
	// Try to get user by Telegram ID
	dbUser, err := h.GetByTelegramID(int64(tgUser.Id))
	if err == nil {
		// User exists, return it
		return dbUser, nil
	}

	// If error is not "no rows", it's a real error
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("%s: failed to get user in getOrCreateUser: %w", utils.GetCurrentTypeName(), err)
	}

	// User doesn't exist, create new user
	userID, err := h.Create(
		int64(tgUser.Id),
		tgUser.FirstName,
		tgUser.LastName,
		tgUser.Username,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create user in getOrCreateUser: %w", utils.GetCurrentTypeName(), err)
	}

	// Get the newly created user
	dbUser, err = h.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to get created user in getOrCreateUser: %w", utils.GetCurrentTypeName(), err)
	}

	return dbUser, nil
}

// GetOrFullCreate gets or creates a user with default profile
func (h *UserRepository) GetOrFullCreate(user *gotgbot.User) (*User, *Profile, error) {
	dbUser, err := h.GetOrCreate(user)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: failed to get user in getOrCreateWithProfile: %w", utils.GetCurrentTypeName(), err)
	}

	// Get or create profile
	profileRepo := NewProfileRepository(h.db)
	profile, err := profileRepo.GetOrCreate(dbUser.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: failed to get profile in getOrCreateWithProfile: %w", utils.GetCurrentTypeName(), err)
	}

	return dbUser, profile, nil
}

// SearchByName searches for users with matching first and last name
func (r *UserRepository) SearchByName(firstname, lastname string) (*User, error) {
	query := `
		SELECT id, tg_id, firstname, lastname, tg_username, score, has_coffee_ban, is_club_member, created_at, updated_at
		FROM users
		WHERE LOWER(firstname) = LOWER($1) AND LOWER(lastname) = LOWER($2)
		LIMIT 1`

	var user User
	err := r.db.QueryRow(query, firstname, lastname).Scan(
		&user.ID,
		&user.TgID,
		&user.Firstname,
		&user.Lastname,
		&user.TgUsername,
		&user.Score,
		&user.HasCoffeeBan,
		&user.IsClubMember,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}

	if err != nil {
		return nil, fmt.Errorf("%s: failed to search user by name: %w", utils.GetCurrentTypeName(), err)
	}

	return &user, nil
}
