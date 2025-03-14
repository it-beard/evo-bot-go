package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// SQL statements for table creation
const (
	createMessagesTableSQL = `
		CREATE TABLE IF NOT EXISTS messages (
			id SERIAL PRIMARY KEY,
			topic_id INT NOT NULL,
			message_id INT NOT NULL,
			reply_to_message_id INT,
			user_id BIGINT,
			username TEXT,
			message_text TEXT,
			created_at TIMESTAMP WITH TIME ZONE,
			UNIQUE(topic_id, message_id)
		)
	`

	createMessagesChatIdxSQL = `
		CREATE INDEX IF NOT EXISTS idx_messages_topic_id ON messages(topic_id)
	`

	createMessagesTimeIdxSQL = `
		CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at)
	`

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
	if err := db.initMessagesSchema(); err != nil {
		return err
	}

	if err := db.initSessionSchema(); err != nil {
		return err
	}

	if err := db.initPromptingTemplatesSchema(); err != nil {
		return err
	}

	log.Println("All database schemas initialized successfully")
	return nil
}

// initMessagesSchema initializes the messages table schema
func (db *DB) initMessagesSchema() error {
	// Create messages table
	if _, err := db.Exec(createMessagesTableSQL); err != nil {
		return fmt.Errorf("failed to create messages table: %w", err)
	}

	// Create indexes
	if _, err := db.Exec(createMessagesChatIdxSQL); err != nil {
		return fmt.Errorf("failed to create chat_id index: %w", err)
	}

	if _, err := db.Exec(createMessagesTimeIdxSQL); err != nil {
		return fmt.Errorf("failed to create created_at index: %w", err)
	}

	log.Println("Messages schema initialized successfully")
	return nil
}

// initSessionSchema initializes the session storage schema
func (db *DB) initSessionSchema() error {
	if _, err := db.Exec(createSessionsTableSQL); err != nil {
		return fmt.Errorf("failed to create tg_sessions table: %w", err)
	}

	log.Println("Session schema initialized successfully")
	return nil
}

// initPromptingTemplatesSchema initializes the prompting templates schema
func (db *DB) initPromptingTemplatesSchema() error {
	if _, err := db.Exec(createPromptingTemplatesTableSQL); err != nil {
		return fmt.Errorf("failed to create prompting_templates table: %w", err)
	}

	log.Println("Prompting templates schema initialized successfully")
	return nil
}



// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}
