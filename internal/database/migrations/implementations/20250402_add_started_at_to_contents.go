package implementations

import (
	"database/sql"
)

type AddStartedAtToContents struct {
	BaseMigration
}

func NewAddStartedAtToContents() *AddStartedAtToContents {
	return &AddStartedAtToContents{
		BaseMigration: BaseMigration{
			name:      "add_started_at_to_contents",
			timestamp: "20250402",
		},
	}
}

func (m *AddStartedAtToContents) Apply(db *sql.DB) error {
	alterSQL := `
		ALTER TABLE contents 
		ADD COLUMN IF NOT EXISTS started_at TIMESTAMP WITH TIME ZONE
	`
	_, err := db.Exec(alterSQL)
	return err
}

func (m *AddStartedAtToContents) Rollback(db *sql.DB) error {
	alterSQL := `
		ALTER TABLE contents 
		DROP COLUMN IF EXISTS started_at
	`
	_, err := db.Exec(alterSQL)
	return err
}
