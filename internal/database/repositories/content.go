package repositories

import (
	"database/sql"
	"fmt"
	"log"
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

// GetLastClubCalls retrieves the last 10 content records of type 'club-call'
func (r *ContentRepository) GetLastClubCalls(limit int) ([]Content, error) {
	query := `
		SELECT id, name, type 
		FROM contents 
		WHERE type = $1 
		ORDER BY id DESC 
		LIMIT $2`

	rows, err := r.db.Query(query, "club-call", limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query last club calls: %w", err)
	}
	defer rows.Close()

	var contents []Content
	for rows.Next() {
		var c Content
		if err := rows.Scan(&c.ID, &c.Name, &c.Type); err != nil {
			return nil, fmt.Errorf("failed to scan club call row: %w", err)
		}
		contents = append(contents, c)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration for club calls: %w", err)
	}

	return contents, nil
}

// UpdateContentName updates the name of a content record by its ID
func (r *ContentRepository) UpdateContentName(id int, newName string) error {
	query := `UPDATE contents SET name = $1 WHERE id = $2`
	result, err := r.db.Exec(query, newName, id)
	if err != nil {
		return fmt.Errorf("failed to update content name for ID %d: %w", id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		// Log the error but don't fail the operation if rowsAffected can't be retrieved
		log.Printf("Could not get rows affected after update: %v", err)
	} else if rowsAffected == 0 {
		return fmt.Errorf("no content found with ID %d to update", id)
	}

	return nil
}
