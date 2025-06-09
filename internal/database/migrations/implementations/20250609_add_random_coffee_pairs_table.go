package implementations

import (
	"database/sql"
)

type AddRandomCoffeePairsTable struct {
	BaseMigration
}

func NewAddRandomCoffeePairsTable() *AddRandomCoffeePairsTable {
	return &AddRandomCoffeePairsTable{
		BaseMigration: BaseMigration{
			name:      "add_random_coffee_pairs_table",
			timestamp: "20250609",
		},
	}
}

func (m *AddRandomCoffeePairsTable) Apply(db *sql.DB) error {
	sql := `
	CREATE TABLE IF NOT EXISTS random_coffee_pairs (
		id SERIAL PRIMARY KEY,
		poll_id INTEGER NOT NULL REFERENCES random_coffee_polls(id) ON DELETE CASCADE,
		user1_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		user2_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		UNIQUE(poll_id, user1_id, user2_id)
	);
	`
	_, err := db.Exec(sql)
	return err
}

func (m *AddRandomCoffeePairsTable) Rollback(db *sql.DB) error {
	sql := `DROP TABLE IF EXISTS random_coffee_pairs;`
	_, err := db.Exec(sql)
	return err
}
