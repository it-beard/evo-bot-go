package repositories

import (
	"database/sql"
	"evo-bot-go/internal/utils"
	"fmt"
	"log"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// Profile represents a row in the profiles table
type Profile struct {
	ID                 int
	UserID             int
	Bio                string
	PublishedMessageID sql.NullInt64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// ProfileRepository handles database operations for profiles
type ProfileRepository struct {
	db *sql.DB
}

// NewProfileRepository creates a new ProfileRepository
func NewProfileRepository(db *sql.DB) *ProfileRepository {
	return &ProfileRepository{db: db}
}

// GetByID retrieves a profile by ID
func (r *ProfileRepository) GetByID(id int) (*Profile, error) {
	query := `
		SELECT id, user_id, bio, published_message_id, created_at, updated_at
		FROM profiles
		WHERE id = $1`

	var profile Profile
	err := r.db.QueryRow(query, id).Scan(
		&profile.ID,
		&profile.UserID,
		&profile.Bio,
		&profile.PublishedMessageID,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}

	if err != nil {
		return nil, fmt.Errorf("%s: failed to get profile with ID %d: %w", utils.GetCurrentTypeName(), id, err)
	}

	return &profile, nil
}

// Create inserts a new profile record into the database
func (r *ProfileRepository) Create(userID int, bio string) (int, error) {
	var id int
	query := `INSERT INTO profiles (user_id, bio) 
			VALUES ($1, $2) RETURNING id`
	err := r.db.QueryRow(query, userID, bio).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("%s: failed to insert profile: %w", utils.GetCurrentTypeName(), err)
	}
	return id, nil
}

// Update updates profile fields
func (r *ProfileRepository) Update(id int, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return fmt.Errorf("%s: no fields to update for profile with ID %d", utils.GetCurrentTypeName(), id)
	}

	// Build query dynamically based on provided fields
	query := "UPDATE profiles SET updated_at = NOW()"
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
		return fmt.Errorf("%s: failed to update profile with ID %d: %w", utils.GetCurrentTypeName(), id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("%s: Could not get rows affected after update: %v", utils.GetCurrentTypeName(), err)
	} else if rowsAffected == 0 {
		return fmt.Errorf("%s: no profile found with ID %d to update", utils.GetCurrentTypeName(), id)
	}

	return nil
}

// Delete removes a profile record from the database by its ID
func (r *ProfileRepository) Delete(id int) error {
	query := `DELETE FROM profiles WHERE id = $1`
	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("%s: failed to delete profile with ID %d: %w", utils.GetCurrentTypeName(), id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("%s: Could not get rows affected after delete: %v", utils.GetCurrentTypeName(), err)
	} else if rowsAffected == 0 {
		return fmt.Errorf("%s: no profile found with ID %d to delete", utils.GetCurrentTypeName(), id)
	}

	return nil
}

// UpdatePublishedMessageID updates the published_message_id field for a profile
func (r *ProfileRepository) UpdatePublishedMessageID(profileID int, messageID int64) error {
	query := `UPDATE profiles SET published_message_id = $1, updated_at = NOW() WHERE id = $2`
	result, err := r.db.Exec(query, messageID, profileID)
	if err != nil {
		return fmt.Errorf("%s: failed to update published_message_id for profile with ID %d: %w", utils.GetCurrentTypeName(), profileID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("%s: Could not get rows affected after update: %v", utils.GetCurrentTypeName(), err)
	} else if rowsAffected == 0 {
		return fmt.Errorf("%s: no profile found with ID %d to update published_message_id", utils.GetCurrentTypeName(), profileID)
	}

	return nil
}

func (r *ProfileRepository) GetOrCreateWithBio(userID int, bio string) (*Profile, error) {
	// Try to get profile
	profile, err := r.getByUserID(userID)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("%s: failed to get profile in GetOrCreateWithBio: %w", utils.GetCurrentTypeName(), err)
	}

	// If profile exists, return it
	if err == nil {
		return profile, nil
	}

	// Profile doesn't exist, check if user exists
	userRepo := NewUserRepository(r.db)
	_, err = userRepo.GetByID(userID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%s: user with ID %d not found, cannot create profile", utils.GetCurrentTypeName(), userID)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: failed to verify user exists in GetOrCreateWithBio: %w", utils.GetCurrentTypeName(), err)
	}

	// User exists, create profile
	_, err = r.Create(userID, bio)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create profile in GetOrCreateWithBio: %w", utils.GetCurrentTypeName(), err)
	}

	// Get the newly created profile
	newProfile, err := r.getByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to get created profile in GetOrCreateWithBio: %w", utils.GetCurrentTypeName(), err)
	}

	return newProfile, nil
}

func (r *ProfileRepository) GetOrCreate(userID int) (*Profile, error) {
	return r.GetOrCreateWithBio(userID, "")
}

// GetOrFullCreate gets or creates a user with default profile
func (r *ProfileRepository) GetOrFullCreate(user *gotgbot.User) (*Profile, error) {
	// Get or create user
	userRepo := NewUserRepository(r.db)
	_, profile, err := userRepo.GetOrFullCreate(user)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to get user in GetOrFullCreate: %w", utils.GetCurrentTypeName(), err)
	}
	return profile, nil
}

// GetByUserID retrieves a profile by user ID
func (r *ProfileRepository) getByUserID(userID int) (*Profile, error) {
	query := `
		SELECT id, user_id, bio, published_message_id, created_at, updated_at
		FROM profiles
		WHERE user_id = $1`

	var profile Profile
	err := r.db.QueryRow(query, userID).Scan(
		&profile.ID,
		&profile.UserID,
		&profile.Bio,
		&profile.PublishedMessageID,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}

	if err != nil {
		return nil, fmt.Errorf("%s: failed to get profile for user with ID %d: %w", utils.GetCurrentTypeName(), userID, err)
	}

	return &profile, nil
}

// ProfileWithUser represents a profile with associated user information
type ProfileWithUser struct {
	Profile *Profile
	User    *User
}

// GetAllWithUsers retrieves all profiles with their associated user information
func (r *ProfileRepository) GetAllWithUsers() ([]ProfileWithUser, error) {
	query := `
		SELECT 
			p.id, p.user_id, p.bio, p.published_message_id, p.created_at, p.updated_at,
			u.id, u.tg_id, u.firstname, u.lastname, u.tg_username, u.score, u.has_coffee_ban, u.is_club_member, u.created_at, u.updated_at
		FROM profiles p
		INNER JOIN users u ON p.user_id = u.id
		WHERE p.bio != '' AND p.bio IS NOT NULL AND p.published_message_id IS NOT NULL
		ORDER BY p.updated_at DESC`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to query profiles with users: %w", utils.GetCurrentTypeName(), err)
	}
	defer rows.Close()

	var profiles []ProfileWithUser
	for rows.Next() {
		var profile Profile
		var user User

		err := rows.Scan(
			&profile.ID,
			&profile.UserID,
			&profile.Bio,
			&profile.PublishedMessageID,
			&profile.CreatedAt,
			&profile.UpdatedAt,
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
		if err != nil {
			return nil, fmt.Errorf("%s: failed to scan profile with user: %w", utils.GetCurrentTypeName(), err)
		}

		profiles = append(profiles, ProfileWithUser{
			Profile: &profile,
			User:    &user,
		})
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: error iterating over profiles with users: %w", utils.GetCurrentTypeName(), err)
	}

	return profiles, nil
}
