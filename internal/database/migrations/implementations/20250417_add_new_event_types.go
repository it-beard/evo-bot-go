package implementations

import (
	"database/sql"
	"fmt"
)

// AddNewEventTypes migration adds workshop, reading-club, and conference event types
type AddNewEventTypes struct {
	BaseMigration
}

// NewAddNewEventTypes creates a new migration instance
func NewAddNewEventTypes() *AddNewEventTypes {
	return &AddNewEventTypes{
		BaseMigration: BaseMigration{
			name:      "add_new_event_types",
			timestamp: "20250417",
		},
	}
}

// Apply updates the check constraint on the events table to include the new event types
func (m *AddNewEventTypes) Apply(db *sql.DB) error {
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

	// Drop the existing check constraint
	_, err = tx.Exec("ALTER TABLE events DROP CONSTRAINT IF EXISTS events_type_check")
	if err != nil {
		return fmt.Errorf("failed to drop type check constraint: %w", err)
	}

	// Add the new check constraint with additional event types
	_, err = tx.Exec("ALTER TABLE events ADD CONSTRAINT events_type_check CHECK (type IN ('club-call', 'meetup', 'workshop', 'reading-club', 'conference'))")
	if err != nil {
		return fmt.Errorf("failed to add new type check constraint: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Rollback reverts the changes made by the Apply method
func (m *AddNewEventTypes) Rollback(db *sql.DB) error {
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

	// Drop the updated check constraint
	_, err = tx.Exec("ALTER TABLE events DROP CONSTRAINT IF EXISTS events_type_check")
	if err != nil {
		return fmt.Errorf("failed to drop type check constraint: %w", err)
	}

	// Restore the original check constraint with only club-call and meetup
	_, err = tx.Exec("ALTER TABLE events ADD CONSTRAINT events_type_check CHECK (type IN ('club-call', 'meetup'))")
	if err != nil {
		return fmt.Errorf("failed to restore original type check constraint: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit rollback transaction: %w", err)
	}

	return nil
}
