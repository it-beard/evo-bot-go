package implementations

import (
	"database/sql"
)

type AddGroupTopicsTable struct {
	BaseMigration
}

func NewAddGroupTopicsTable() *AddGroupTopicsTable {
	return &AddGroupTopicsTable{
		BaseMigration: BaseMigration{
			name:      "add_group_topics_table",
			timestamp: "20250917",
		},
	}
}

func (m *AddGroupTopicsTable) Apply(db *sql.DB) error {
	sql := `
	CREATE TABLE IF NOT EXISTS group_topics (
		id SERIAL PRIMARY KEY,
		topic_id BIGINT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	`
	_, err := db.Exec(sql)
	return err
}

func (m *AddGroupTopicsTable) Rollback(db *sql.DB) error {
	sql := `DROP TABLE IF EXISTS group_topics;`
	_, err := db.Exec(sql)
	return err
}
