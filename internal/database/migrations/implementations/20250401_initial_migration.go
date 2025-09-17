package implementations

import (
	"database/sql"
	"evo-bot-go/internal/database/prompts"
	"fmt"
	"log"
)

const (
	createSessionsTableSQL = `
		CREATE TABLE IF NOT EXISTS tg_sessions (
			id TEXT PRIMARY KEY,
			data BYTEA NOT NULL,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`

	createPromptingTemplatesTableSQL = `
		CREATE TABLE IF NOT EXISTS prompting_templates (
			template_key TEXT PRIMARY KEY,
			template_text TEXT NOT NULL
		)
	`

	createContentsTableSQL = `
		CREATE TABLE IF NOT EXISTS contents (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT NOT NULL CHECK (type IN ('club-call', 'meetup')),
			status TEXT NOT NULL DEFAULT 'actual' CHECK (status IN ('finished', 'actual')),
			started_at TIMESTAMP WITH TIME ZONE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`

	createTopicsTableSQL = `
		CREATE TABLE IF NOT EXISTS topics (
			id SERIAL PRIMARY KEY,
			topic TEXT NOT NULL,
			user_id INTEGER NOT NULL,
			content_id INTEGER REFERENCES contents(id),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`

	createUsersTableSQL = `
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			tg_id BIGINT NOT NULL UNIQUE,
			firstname TEXT NOT NULL,
			lastname TEXT,
			tg_username TEXT,
			score INTEGER NOT NULL DEFAULT 0,
			has_coffee_ban BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		)
	`

	createProfilesTableSQL = `
		CREATE TABLE IF NOT EXISTS profiles (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			bio TEXT,
			linkedin TEXT,
			github TEXT,
			website TEXT,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		)
	`

	createProfilesIndexSQL = `
		CREATE INDEX IF NOT EXISTS profiles_user_id_idx ON profiles(user_id)
	`
)

type InitialMigration struct {
	BaseMigration
}

func NewInitialMigration() *InitialMigration {
	return &InitialMigration{
		BaseMigration: BaseMigration{
			name:      "initial_migration",
			timestamp: "20250401",
		},
	}
}

func (m *InitialMigration) Apply(db *sql.DB) error {
	if _, err := db.Exec(createSessionsTableSQL); err != nil {
		return fmt.Errorf("failed to create tg_sessions table: %w", err)
	}

	if _, err := db.Exec(createPromptingTemplatesTableSQL); err != nil {
		return fmt.Errorf("failed to create prompting_templates table: %w", err)
	}

	if err := insertDefaultPrompts(db); err != nil {
		return err
	}

	if _, err := db.Exec(createContentsTableSQL); err != nil {
		return fmt.Errorf("failed to create contents table: %w", err)
	}

	if _, err := db.Exec(createTopicsTableSQL); err != nil {
		return fmt.Errorf("failed to create topics table: %w", err)
	}

	if _, err := db.Exec(createUsersTableSQL); err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	if _, err := db.Exec(createProfilesTableSQL); err != nil {
		return fmt.Errorf("failed to create profiles table: %w", err)
	}

	if _, err := db.Exec(createProfilesIndexSQL); err != nil {
		return fmt.Errorf("failed to create profiles index: %w", err)
	}

	return nil
}

func (m *InitialMigration) Rollback(db *sql.DB) error {
	if _, err := db.Exec(`DROP INDEX IF EXISTS profiles_user_id_idx`); err != nil {
		return err
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS profiles`); err != nil {
		return err
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS topics`); err != nil {
		return err
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS users`); err != nil {
		return err
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS contents`); err != nil {
		return err
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS prompting_templates`); err != nil {
		return err
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS tg_sessions`); err != nil {
		return err
	}
	return nil
}

func insertDefaultPrompts(db *sql.DB) error {
	if err := insertDefaultPromptIfNotExists(db, prompts.GetContentPromptKey, prompts.GetContentPromptDefaultValue); err != nil {
		return fmt.Errorf("failed to insert content prompt: %w", err)
	}

	if err := insertDefaultPromptIfNotExists(db, prompts.DailySummarizationPromptKey, prompts.DailySummarizationPromptDefaultValue); err != nil {
		return fmt.Errorf("failed to insert summarization prompt: %w", err)
	}

	if err := insertDefaultPromptIfNotExists(db, prompts.GetToolPromptKey, prompts.GetToolPromptDefaultValue); err != nil {
		return fmt.Errorf("failed to insert tool prompt: %w", err)
	}

	if err := insertDefaultPromptIfNotExists(db, prompts.GetIntroPromptKey, prompts.GetIntroPromptDefaultValue); err != nil {
		return fmt.Errorf("failed to insert intro prompt: %w", err)
	}
	return nil
}

func insertDefaultPromptIfNotExists(db *sql.DB, key, text string) error {
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM prompting_templates WHERE template_key = $1)", key).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if prompt exists: %w", err)
	}

	if !exists {
		_, err = db.Exec("INSERT INTO prompting_templates (template_key, template_text) VALUES ($1, $2)", key, text)
		if err != nil {
			return fmt.Errorf("failed to insert prompt: %w", err)
		}
		log.Printf("Inserted default prompt: %s", key)
	}

	return nil
}
