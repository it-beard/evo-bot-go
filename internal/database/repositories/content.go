package repositories

import (
	"database/sql"
	"fmt"
)

// Content represents a row in the contents table
type Content struct {
	ID   int
	Name string
	Type string
}

// ContentRepository handles database operations for contents
type ContentRepository struct {
	db *sql.DB
}

// NewContentRepository creates a new ContentRepository
func NewContentRepository(db *sql.DB) *ContentRepository {
	return &ContentRepository{db: db}
}

// CreateContent inserts a new content record into the database
func (r *ContentRepository) CreateContent(name, contentType string) (int, error) {
	var id int
	query := `INSERT INTO contents (name, type) VALUES ($1, $2) RETURNING id`
	err := r.db.QueryRow(query, name, contentType).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to insert content: %w", err)
	}
	return id, nil
} 