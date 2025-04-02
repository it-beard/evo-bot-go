package implementations

import "database/sql"

// BaseMigration provides common functionality for all migrations
type BaseMigration struct {
	name      string
	timestamp string
}

func (m *BaseMigration) Name() string {
	return m.name
}

func (m *BaseMigration) Timestamp() string {
	return m.timestamp
}

// Migration interface defines methods that all migrations must implement
type Migration interface {
	Name() string
	Timestamp() string
	Apply(db *sql.DB) error
	Rollback(db *sql.DB) error
}
