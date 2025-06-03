package implementations

import (
	"database/sql"
)

type RemoveChatIdFromRandomCoffeePolls struct {
	BaseMigration
}

func NewRemoveChatIdFromRandomCoffeePolls() *RemoveChatIdFromRandomCoffeePolls {
	return &RemoveChatIdFromRandomCoffeePolls{
		BaseMigration: BaseMigration{
			name:      "remove_chat_id_from_random_coffee_polls",
			timestamp: "20250603",
		},
	}
}

func (m *RemoveChatIdFromRandomCoffeePolls) Apply(db *sql.DB) error {
	sql := `ALTER TABLE random_coffee_polls DROP COLUMN IF EXISTS chat_id`
	_, err := db.Exec(sql)
	return err
}

func (m *RemoveChatIdFromRandomCoffeePolls) Rollback(db *sql.DB) error {
	sql := `ALTER TABLE random_coffee_polls ADD COLUMN IF NOT EXISTS chat_id BIGINT NOT NULL DEFAULT 0`
	_, err := db.Exec(sql)
	return err
}
