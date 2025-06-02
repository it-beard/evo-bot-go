package implementations

import (
	"database/sql"
)

type AddRandomCoffeePollTables struct {
	BaseMigration
}

func NewAddRandomCoffeePollTables() *AddRandomCoffeePollTables {
	return &AddRandomCoffeePollTables{
		BaseMigration: BaseMigration{
			name:      "add_random_coffee_poll_tables",
			timestamp: "20250602",
		},
	}
}

func (m *AddRandomCoffeePollTables) Apply(db *sql.DB) error {
	// Create random_coffee_polls table
	sql1 := `CREATE TABLE IF NOT EXISTS random_coffee_polls (
		id SERIAL PRIMARY KEY,
		message_id BIGINT NOT NULL,
		chat_id BIGINT NOT NULL,
		week_start_date DATE NOT NULL,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);`

	if _, err := db.Exec(sql1); err != nil {
		return err
	}

	// Create random_coffee_participants table
	sql2 := `CREATE TABLE IF NOT EXISTS random_coffee_participants (
		id SERIAL PRIMARY KEY,
		poll_id INTEGER NOT NULL REFERENCES random_coffee_polls(id) ON DELETE CASCADE,
		user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		is_participating BOOLEAN NOT NULL,
		created_at TIMESTAMPTZ DEFAULT NOW(),
		updated_at TIMESTAMPTZ DEFAULT NOW(),
		UNIQUE (poll_id, user_id)
	);`

	if _, err := db.Exec(sql2); err != nil {
		return err
	}

	// Create trigger function and trigger
	sql3 := `CREATE OR REPLACE FUNCTION update_updated_at_column()
	RETURNS TRIGGER AS $$
	BEGIN
		NEW.updated_at = NOW();
		RETURN NEW;
	END;
	$$ language 'plpgsql';

	CREATE TRIGGER update_random_coffee_participants_updated_at
		BEFORE UPDATE ON random_coffee_participants
		FOR EACH ROW
		EXECUTE FUNCTION update_updated_at_column();`

	if _, err := db.Exec(sql3); err != nil {
		return err
	}

	// Add telegram_poll_id column and constraint
	sql4 := `ALTER TABLE random_coffee_polls
	ADD COLUMN telegram_poll_id TEXT;

	ALTER TABLE random_coffee_polls
	ADD CONSTRAINT random_coffee_polls_telegram_poll_id_unique UNIQUE (telegram_poll_id);

	COMMENT ON COLUMN random_coffee_polls.telegram_poll_id IS 'Unique poll ID string from Telegram (Poll.Id).';`

	if _, err := db.Exec(sql4); err != nil {
		return err
	}

	return nil
}

func (m *AddRandomCoffeePollTables) Rollback(db *sql.DB) error {
	// Remove telegram_poll_id column and constraint
	sql1 := `ALTER TABLE random_coffee_polls DROP CONSTRAINT IF EXISTS random_coffee_polls_telegram_poll_id_unique;
	ALTER TABLE random_coffee_polls DROP COLUMN IF EXISTS telegram_poll_id;`

	if _, err := db.Exec(sql1); err != nil {
		return err
	}

	// Drop trigger and function
	sql2 := `DROP TRIGGER IF EXISTS update_random_coffee_participants_updated_at ON random_coffee_participants;
	DROP FUNCTION IF EXISTS update_updated_at_column();`

	if _, err := db.Exec(sql2); err != nil {
		return err
	}

	// Drop tables (order matters due to foreign key constraints)
	sql3 := `DROP TABLE IF EXISTS random_coffee_participants;
	DROP TABLE IF EXISTS random_coffee_polls;`

	if _, err := db.Exec(sql3); err != nil {
		return err
	}

	return nil
}
