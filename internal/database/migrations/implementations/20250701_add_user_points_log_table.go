package implementations

import (
	"database/sql"
	"fmt"
)

// AddUserPointsLogTable migration creates user_points_log table
type AddUserPointsLogTable struct {
	BaseMigration
}

// NewAddUserPointsLogTable creates a new migration instance
func NewAddUserPointsLogTable() *AddUserPointsLogTable {
	return &AddUserPointsLogTable{
		BaseMigration: BaseMigration{
			name:      "add_user_points_log_table",
			timestamp: "20250701",
		},
	}
}

// Apply creates the user_points_log table
func (m *AddUserPointsLogTable) Apply(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Create user_points_log table
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS user_points_log (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			points INTEGER NOT NULL,
			reason TEXT NOT NULL,
			poll_id INTEGER REFERENCES random_coffee_polls(id) ON DELETE SET NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create user_points_log table: %w", err)
	}

	// Create index on user_id for faster lookups
	_, err = tx.Exec(`
		CREATE INDEX IF NOT EXISTS user_points_log_user_id_idx ON user_points_log(user_id)
	`)
	if err != nil {
		return fmt.Errorf("failed to create index on user_points_log.user_id: %w", err)
	}

	// Create index on created_at for time-based queries
	_, err = tx.Exec(`
		CREATE INDEX IF NOT EXISTS user_points_log_created_at_idx ON user_points_log(created_at)
	`)
	if err != nil {
		return fmt.Errorf("failed to create index on user_points_log.created_at: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Rollback drops the user_points_log table
func (m *AddUserPointsLogTable) Rollback(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	_, err = tx.Exec("DROP TABLE IF EXISTS user_points_log")
	if err != nil {
		return fmt.Errorf("failed to drop user_points_log table: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit rollback transaction: %w", err)
	}

	return nil
}