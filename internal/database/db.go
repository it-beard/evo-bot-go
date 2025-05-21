package database

import (
	"database/sql"
	"evo-bot-go/internal/database/migrations"
	"evo-bot-go/internal/database/prompts"
	"fmt"
	"log"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// SQL statements for table creation
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

	createMigrationsTableSQL = `
		CREATE TABLE IF NOT EXISTS migrations (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			timestamp TEXT NOT NULL,
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

// DB represents a database connection
type DB struct {
	*sql.DB
}

// NewDB creates a new database connection
func NewDB(connectionString string) (*DB, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{db}, nil
}

// InitSchema initializes all database schemas
func (db *DB) InitSchema() error {
	if err := db.initSessionSchema(); err != nil {
		return err
	}

	if err := db.initPromptingTemplatesSchema(); err != nil {
		return err
	}

	if err := db.initContentsSchema(); err != nil {
		return err
	}

	if err := db.initTopicsSchema(); err != nil {
		return err
	}

	if err := db.initMigrationsSchema(); err != nil {
		return err
	}

	if err := db.initUsersSchema(); err != nil {
		return err
	}

	if err := db.initProfilesSchema(); err != nil {
		return err
	}

	return nil
}

// InitWithMigrations initializes the database and runs any pending migrations
func (db *DB) InitWithMigrations() error {
	// First initialize the base schema
	if err := db.InitSchema(); err != nil {
		return err
	}

	// Run pending migrations
	if err := migrations.RunMigrations(db.DB); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// initSessionSchema initializes the session storage schema
func (db *DB) initSessionSchema() error {
	if _, err := db.Exec(createSessionsTableSQL); err != nil {
		return fmt.Errorf("failed to create tg_sessions table: %w", err)
	}

	return nil
}

// initPromptingTemplatesSchema initializes the prompting templates schema
func (db *DB) initPromptingTemplatesSchema() error {
	if _, err := db.Exec(createPromptingTemplatesTableSQL); err != nil {
		return fmt.Errorf("failed to create prompting_templates table: %w", err)
	}

	// Insert default prompts if they don't exist
	if err := db.insertDefaultPromptIfNotExists(prompts.GetContentPromptTemplateDbKey, prompts.GetContentPromptDefaultTemplate); err != nil {
		return fmt.Errorf("failed to insert content prompt: %w", err)
	}

	if err := db.insertDefaultPromptIfNotExists(prompts.DailySummarizationPromptTemplateDbKey, prompts.DailySummarizationPromptDefaultTemplate); err != nil {
		return fmt.Errorf("failed to insert summarization prompt: %w", err)
	}

	if err := db.insertDefaultPromptIfNotExists(prompts.GetToolPromptTemplateDbKey, prompts.GetToolPromptDefaultTemplate); err != nil {
		return fmt.Errorf("failed to insert tool prompt: %w", err)
	}

	if err := db.insertDefaultPromptIfNotExists(prompts.GetIntroPromptTemplateDbKey, prompts.GetIntroPromptDefaultTemplate); err != nil {
		return fmt.Errorf("failed to insert intro prompt: %w", err)
	}

	return nil
}

// insertDefaultPromptIfNotExists inserts a default prompt if it doesn't already exist
func (db *DB) insertDefaultPromptIfNotExists(key, text string) error {
	// Check if the prompt already exists
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM prompting_templates WHERE template_key = $1)", key).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if prompt exists: %w", err)
	}

	// If it doesn't exist, insert it
	if !exists {
		_, err = db.Exec("INSERT INTO prompting_templates (template_key, template_text) VALUES ($1, $2)", key, text)
		if err != nil {
			return fmt.Errorf("failed to insert prompt: %w", err)
		}
		log.Printf("Inserted default prompt: %s", key)
	}

	return nil
}

// initContentsSchema initializes the contents schema
func (db *DB) initContentsSchema() error {
	if _, err := db.Exec(createContentsTableSQL); err != nil {
		return fmt.Errorf("failed to create contents table: %w", err)
	}

	return nil
}

// initTopicsSchema initializes the topics schema
func (db *DB) initTopicsSchema() error {
	if _, err := db.Exec(createTopicsTableSQL); err != nil {
		return fmt.Errorf("failed to create topics table: %w", err)
	}

	return nil
}

// initMigrationsSchema initializes the migrations table schema
func (db *DB) initMigrationsSchema() error {
	if _, err := db.Exec(createMigrationsTableSQL); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	return nil
}

// initUsersSchema initializes the users schema
func (db *DB) initUsersSchema() error {
	if _, err := db.Exec(createUsersTableSQL); err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	return nil
}

// initProfilesSchema initializes the profiles schema
func (db *DB) initProfilesSchema() error {
	// First create the profiles table
	if _, err := db.Exec(createProfilesTableSQL); err != nil {
		return fmt.Errorf("failed to create profiles table: %w", err)
	}

	// Then create the index
	if _, err := db.Exec(createProfilesIndexSQL); err != nil {
		return fmt.Errorf("failed to create profiles index: %w", err)
	}

	return nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}
