package repositories

import (
	"database/sql"
	"fmt"
	"log"
	"time"
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
		return nil, fmt.Errorf("failed to get profile with ID %d: %w", id, err)
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
		return 0, fmt.Errorf("failed to insert profile: %w", err)
	}
	return id, nil
}

// Update updates profile fields
func (r *ProfileRepository) Update(id int, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return fmt.Errorf("no fields to update for profile with ID %d", id)
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
		return fmt.Errorf("failed to update profile with ID %d: %w", id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Could not get rows affected after update: %v", err)
	} else if rowsAffected == 0 {
		return fmt.Errorf("no profile found with ID %d to update", id)
	}

	return nil
}

// Delete removes a profile record from the database by its ID
func (r *ProfileRepository) Delete(id int) error {
	query := `DELETE FROM profiles WHERE id = $1`
	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete profile with ID %d: %w", id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Could not get rows affected after delete: %v", err)
	} else if rowsAffected == 0 {
		return fmt.Errorf("no profile found with ID %d to delete", id)
	}

	return nil
}

// UpdatePublishedMessageID updates the published_message_id field for a profile
func (r *ProfileRepository) UpdatePublishedMessageID(profileID int, messageID int64) error {
	query := `UPDATE profiles SET published_message_id = $1, updated_at = NOW() WHERE id = $2`
	result, err := r.db.Exec(query, messageID, profileID)
	if err != nil {
		return fmt.Errorf("failed to update published_message_id for profile with ID %d: %w", profileID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Could not get rows affected after update: %v", err)
	} else if rowsAffected == 0 {
		return fmt.Errorf("no profile found with ID %d to update published_message_id", profileID)
	}

	return nil
}

func (r *ProfileRepository) GetOrCreateWithBio(userID int, bio string) (*Profile, error) {
	// Try to get profile
	profile, err := r.getByUserID(userID)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("ProfileRepository: failed to get profile in GetOrCreateByUserId: %w", err)
	}

	// If profile exists, return it
	if err == nil {
		return profile, nil
	}

	// Profile doesn't exist, check if user exists
	userRepo := NewUserRepository(r.db)
	_, err = userRepo.GetByID(userID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("ProfileRepository: user with ID %d not found, cannot create profile", userID)
	}
	if err != nil {
		return nil, fmt.Errorf("ProfileRepository: failed to verify user exists in GetOrCreateByUserId: %w", err)
	}

	// User exists, create profile
	_, err = r.Create(userID, bio)
	if err != nil {
		return nil, fmt.Errorf("ProfileRepository: failed to create profile in GetOrCreateByUserId: %w", err)
	}

	// Get the newly created profile
	newProfile, err := r.getByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("ProfileRepository: failed to get created profile in GetOrCreateByUserId: %w", err)
	}

	return newProfile, nil
}

func (r *ProfileRepository) GetOrCreate(userID int) (*Profile, error) {
	return r.GetOrCreateWithBio(userID, "")
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
		return nil, fmt.Errorf("failed to get profile for user with ID %d: %w", userID, err)
	}

	return &profile, nil
}
