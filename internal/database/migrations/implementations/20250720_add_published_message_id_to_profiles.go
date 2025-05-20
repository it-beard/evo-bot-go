package implementations

import (
	"database/sql"
	"fmt"
)

// AddPublishedMessageIDToProfiles migration adds published_message_id column to profiles table
type AddPublishedMessageIDToProfiles struct {
	BaseMigration
}

// NewAddPublishedMessageIDToProfiles creates a new migration instance
func NewAddPublishedMessageIDToProfiles() *AddPublishedMessageIDToProfiles {
	return &AddPublishedMessageIDToProfiles{
		BaseMigration: BaseMigration{
			name:      "add_published_message_id_to_profiles",
			timestamp: "20250720",
		},
	}
}

// Apply creates the published_message_id column in the profiles table
func (m *AddPublishedMessageIDToProfiles) Apply(db *sql.DB) error {
	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Add the published_message_id column to the profiles table
	_, err = tx.Exec(`
		ALTER TABLE profiles
		ADD COLUMN published_message_id BIGINT DEFAULT NULL
	`)
	if err != nil {
		return fmt.Errorf("failed to add published_message_id column to profiles table: %w", err)
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Rollback removes the published_message_id column from the profiles table
func (m *AddPublishedMessageIDToProfiles) Rollback(db *sql.DB) error {
	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Remove the published_message_id column from the profiles table
	_, err = tx.Exec(`
		ALTER TABLE profiles
		DROP COLUMN IF EXISTS published_message_id
	`)
	if err != nil {
		return fmt.Errorf("failed to drop published_message_id column from profiles table: %w", err)
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
