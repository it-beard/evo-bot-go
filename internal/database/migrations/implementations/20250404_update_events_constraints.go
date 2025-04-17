package implementations

import (
	"database/sql"
	"fmt"
)

// UpdateEventsConstraints migration drops the contents_type_check constraint
// and renames contents_status_check to events_status_check for the events table
type UpdateEventsConstraints struct {
	BaseMigration
}

// NewUpdateEventsConstraints creates a new migration instance
func NewUpdateEventsConstraints() *UpdateEventsConstraints {
	return &UpdateEventsConstraints{
		BaseMigration: BaseMigration{
			name:      "update_events_constraints",
			timestamp: "20250404",
		},
	}
}

// Apply drops and renames the constraints
func (m *UpdateEventsConstraints) Apply(db *sql.DB) error {
	// Start a transaction to ensure all changes are applied atomically
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Drop the contents_type_check constraint
	_, err = tx.Exec("ALTER TABLE events DROP CONSTRAINT IF EXISTS contents_type_check")
	if err != nil {
		return fmt.Errorf("failed to drop contents_type_check constraint: %w", err)
	}

	// Rename contents_status_check to events_status_check
	// First, we need to drop the old constraint
	_, err = tx.Exec("ALTER TABLE events DROP CONSTRAINT IF EXISTS contents_status_check")
	if err != nil {
		return fmt.Errorf("failed to drop contents_status_check constraint: %w", err)
	}

	// Then recreate it with the new name using the correct status values
	_, err = tx.Exec("ALTER TABLE events ADD CONSTRAINT events_status_check CHECK (status IN ('finished', 'actual'))")
	if err != nil {
		return fmt.Errorf("failed to add events_status_check constraint: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Rollback reverts the changes made by the Apply method
func (m *UpdateEventsConstraints) Rollback(db *sql.DB) error {
	// Start a transaction to ensure all changes are reverted atomically
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Drop the events_status_check constraint
	_, err = tx.Exec("ALTER TABLE events DROP CONSTRAINT IF EXISTS events_status_check")
	if err != nil {
		return fmt.Errorf("failed to drop events_status_check constraint: %w", err)
	}

	// Recreate the contents_status_check constraint with the original status values
	// Note: Keeping original constraint values for backward compatibility
	_, err = tx.Exec("ALTER TABLE events ADD CONSTRAINT contents_status_check CHECK (status IN ('pending', 'processing', 'completed', 'failed'))")
	if err != nil {
		return fmt.Errorf("failed to add contents_status_check constraint: %w", err)
	}

	// Recreate the contents_type_check constraint
	// Note: We're assuming the type check validates the same values
	// If the check condition needs to be changed, it should be updated here
	_, err = tx.Exec("ALTER TABLE events ADD CONSTRAINT contents_type_check CHECK (type IN ('lesson', 'exercise', 'quiz'))")
	if err != nil {
		return fmt.Errorf("failed to add contents_type_check constraint: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit rollback transaction: %w", err)
	}

	return nil
}
