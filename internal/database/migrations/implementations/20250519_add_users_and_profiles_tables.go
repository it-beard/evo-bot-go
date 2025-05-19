package implementations

import (
	"database/sql"
	"fmt"
)

// AddUsersAndProfilesTables migration creates users and profiles tables
type AddUsersAndProfilesTables struct {
	BaseMigration
}

// NewAddUsersAndProfilesTables creates a new migration instance
func NewAddUsersAndProfilesTables() *AddUsersAndProfilesTables {
	return &AddUsersAndProfilesTables{
		BaseMigration: BaseMigration{
			name:      "add_users_and_profiles_tables",
			timestamp: "20250519",
		},
	}
}

// Apply creates the users and profiles tables
func (m *AddUsersAndProfilesTables) Apply(db *sql.DB) error {
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

	// Create users table
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			tg_id BIGINT NOT NULL UNIQUE,
			firstname TEXT NOT NULL,
			lastname TEXT,
			tg_username TEXT,
			score INTEGER NOT NULL DEFAULT 0,
			has_coffee_ban BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	// Create profiles table
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS profiles (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			bio TEXT,
			linkedin TEXT,
			github TEXT,
			website TEXT,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create profiles table: %w", err)
	}

	// Create index on user_id for faster profile lookups
	_, err = tx.Exec(`
		CREATE INDEX IF NOT EXISTS profiles_user_id_idx ON profiles(user_id)
	`)
	if err != nil {
		return fmt.Errorf("failed to create index on profiles.user_id: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Rollback drops the profiles and users tables
func (m *AddUsersAndProfilesTables) Rollback(db *sql.DB) error {
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

	// Drop the profiles table first (because it references users)
	_, err = tx.Exec("DROP TABLE IF EXISTS profiles")
	if err != nil {
		return fmt.Errorf("failed to drop profiles table: %w", err)
	}

	// Then drop the users table
	_, err = tx.Exec("DROP TABLE IF EXISTS users")
	if err != nil {
		return fmt.Errorf("failed to drop users table: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit rollback transaction: %w", err)
	}

	return nil
}