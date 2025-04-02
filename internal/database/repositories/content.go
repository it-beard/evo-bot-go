package repositories

import (
	"database/sql"
	"evo-bot-go/internal/constants"
	"fmt"
	"log"
	"time"
)

// Content represents a row in the contents table
type Content struct {
	ID        int
	Name      string
	Type      string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
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
func (r *ContentRepository) CreateContent(name string, contentType constants.ContentType) (int, error) {
	var id int
	query := `INSERT INTO contents (name, type, status) VALUES ($1, $2, $3) RETURNING id`
	err := r.db.QueryRow(query, name, contentType, constants.ContentStatusActual).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to insert content: %w", err)
	}
	return id, nil
}

// GetLastContents retrieves the last N content records
func (r *ContentRepository) GetLastContents(limit int) ([]Content, error) {
	query := `
		SELECT id, name, type, status, created_at, updated_at
		FROM contents
		ORDER BY id DESC 
		LIMIT $1`

	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query last contents: %w", err)
	}
	defer rows.Close()

	var contents []Content
	for rows.Next() {
		var c Content
		if err := rows.Scan(&c.ID, &c.Name, &c.Type, &c.Status, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan content row: %w", err)
		}
		contents = append(contents, c)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration for contents: %w", err)
	}

	return contents, nil
}

// UpdateContentName updates the name of a content record by its ID
func (r *ContentRepository) UpdateContentName(id int, newName string) error {
	query := `UPDATE contents SET name = $1, updated_at = NOW() WHERE id = $2`
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

// UpdateContentStatus updates the status of a content record by its ID
func (r *ContentRepository) UpdateContentStatus(id int, status constants.ContentStatus) error {
	query := `UPDATE contents SET status = $1, updated_at = NOW() WHERE id = $2`
	result, err := r.db.Exec(query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update content status for ID %d: %w", id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Could not get rows affected after update: %v", err)
	} else if rowsAffected == 0 {
		return fmt.Errorf("no content found with ID %d to update status", id)
	}

	return nil
}

// DeleteContent removes a content record from the database by its ID
func (r *ContentRepository) DeleteContent(id int) error {
	query := `DELETE FROM contents WHERE id = $1`
	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete content with ID %d: %w", id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Could not get rows affected after delete: %v", err)
	} else if rowsAffected == 0 {
		return fmt.Errorf("no content found with ID %d to delete", id)
	}

	return nil
}
