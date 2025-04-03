package implementations

import (
	"database/sql"
	"fmt"
)

// RenameContentsToEvents migration renames the contents table to events
// and updates the content_id column in topics table to event_id
type RenameContentsToEvents struct {
	BaseMigration
}

// NewRenameContentsToEvents creates a new migration instance
func NewRenameContentsToEvents() *RenameContentsToEvents {
	return &RenameContentsToEvents{
		BaseMigration: BaseMigration{
			name:      "rename_contents_to_events",
			timestamp: "20250403",
		},
	}
}

// Apply renames the contents table to events and updates the related foreign key
func (m *RenameContentsToEvents) Apply(db *sql.DB) error {
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

	// Drop the foreign key constraint first
	_, err = tx.Exec("ALTER TABLE topics DROP CONSTRAINT IF EXISTS topics_content_id_fkey")
	if err != nil {
		return fmt.Errorf("failed to drop foreign key constraint: %w", err)
	}

	// Rename the contents table to events
	_, err = tx.Exec("ALTER TABLE contents RENAME TO events")
	if err != nil {
		return fmt.Errorf("failed to rename contents table to events: %w", err)
	}

	// Rename content_id column in topics table to event_id
	_, err = tx.Exec("ALTER TABLE topics RENAME COLUMN content_id TO event_id")
	if err != nil {
		return fmt.Errorf("failed to rename content_id column: %w", err)
	}

	// Re-add the foreign key constraint
	_, err = tx.Exec("ALTER TABLE topics ADD CONSTRAINT topics_event_id_fkey FOREIGN KEY (event_id) REFERENCES events(id)")
	if err != nil {
		return fmt.Errorf("failed to add new foreign key constraint: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Rollback reverts the changes made by the Apply method
func (m *RenameContentsToEvents) Rollback(db *sql.DB) error {
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

	// Drop the foreign key constraint first
	_, err = tx.Exec("ALTER TABLE topics DROP CONSTRAINT IF EXISTS topics_event_id_fkey")
	if err != nil {
		return fmt.Errorf("failed to drop foreign key constraint: %w", err)
	}

	// Rename event_id column in topics table back to content_id
	_, err = tx.Exec("ALTER TABLE topics RENAME COLUMN event_id TO content_id")
	if err != nil {
		return fmt.Errorf("failed to rename event_id column: %w", err)
	}

	// Rename the events table back to contents
	_, err = tx.Exec("ALTER TABLE events RENAME TO contents")
	if err != nil {
		return fmt.Errorf("failed to rename events table back to contents: %w", err)
	}

	// Re-add the foreign key constraint
	_, err = tx.Exec("ALTER TABLE topics ADD CONSTRAINT topics_content_id_fkey FOREIGN KEY (content_id) REFERENCES contents(id)")
	if err != nil {
		return fmt.Errorf("failed to add original foreign key constraint: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit rollback transaction: %w", err)
	}

	return nil
}
