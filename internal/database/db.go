package database

import (
	"database/sql"
	"evo-bot-go/internal/database/migrations"
	"fmt"

	_ "github.com/lib/pq" // PostgreSQL driver
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

// InitWithMigrations initializes the database and runs any pending migrations
func (db *DB) InitWithMigrations() error {
	// Run pending migrations
	if err := migrations.RunMigrations(db.DB); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}
