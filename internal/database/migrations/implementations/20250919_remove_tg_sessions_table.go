package implementations

import (
	"database/sql"
)

type RemoveTgSessionsTable struct {
	BaseMigration
}

func NewRemoveTgSessionsTable() *RemoveTgSessionsTable {
	return &RemoveTgSessionsTable{
		BaseMigration: BaseMigration{
			name:      "remove_tg_sessions_table",
			timestamp: "20250919",
		},
	}
}

func (m *RemoveTgSessionsTable) Apply(db *sql.DB) error {
	sql := `DROP TABLE IF EXISTS tg_sessions`
	_, err := db.Exec(sql)
	return err
}

func (m *RemoveTgSessionsTable) Rollback(db *sql.DB) error {
	// Note: This rollback creates a basic tg_sessions table structure
	// You may need to adjust the schema based on your original table structure
	sql := `CREATE TABLE IF NOT EXISTS tg_sessions (
		id BIGSERIAL PRIMARY KEY,
		session_data BYTEA,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`
	_, err := db.Exec(sql)
	return err
}
