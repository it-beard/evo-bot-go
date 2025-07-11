package repositories

import (
	"database/sql"
	"evo-bot-go/internal/utils"
	"fmt"
	"log"
	"time"
)

// Topic represents a row in the topics table
type Topic struct {
	ID           int
	Topic        string
	UserNickname *string
	EventID      int
	CreatedAt    time.Time
}

// TopicRepository handles database operations for topics
type TopicRepository struct {
	db *sql.DB
}

// NewTopicRepository creates a new TopicRepository
func NewTopicRepository(db *sql.DB) *TopicRepository {
	return &TopicRepository{db: db}
}

// CreateTopic inserts a new topic record into the database
func (r *TopicRepository) CreateTopic(topic string, userNickname string, eventID int) (int, error) {
	var id int
	query := `INSERT INTO topics (topic, user_nickname, event_id) VALUES ($1, $2, $3) RETURNING id`
	err := r.db.QueryRow(query, topic, userNickname, eventID).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("%s: failed to insert topic: %w", utils.GetCurrentTypeName(), err)
	}
	return id, nil
}

// GetTopicsByEventID retrieves all topics for a specific event
func (r *TopicRepository) GetTopicsByEventID(eventID int) ([]Topic, error) {
	query := `
		SELECT id, topic, user_nickname, event_id, created_at
		FROM topics
		WHERE event_id = $1
		ORDER BY created_at ASC`

	rows, err := r.db.Query(query, eventID)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to query topics for event ID %d: %w", utils.GetCurrentTypeName(), eventID, err)
	}
	defer rows.Close()

	var topics []Topic
	for rows.Next() {
		var t Topic
		if err := rows.Scan(&t.ID, &t.Topic, &t.UserNickname, &t.EventID, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("%s: failed to scan topic row: %w", utils.GetCurrentTypeName(), err)
		}
		topics = append(topics, t)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: error during rows iteration for topics: %w", utils.GetCurrentTypeName(), err)
	}

	return topics, nil
}

// GetTopicByID retrieves a single topic record by its ID
func (r *TopicRepository) GetTopicByID(id int) (*Topic, error) {
	query := `
		SELECT id, topic, user_nickname, event_id, created_at
		FROM topics
		WHERE id = $1`

	var topic Topic
	err := r.db.QueryRow(query, id).Scan(
		&topic.ID,
		&topic.Topic,
		&topic.UserNickname,
		&topic.EventID,
		&topic.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%s: no topic found with ID %d", utils.GetCurrentTypeName(), id)
	}

	if err != nil {
		return nil, fmt.Errorf("%s: failed to get topic with ID %d: %w", utils.GetCurrentTypeName(), id, err)
	}

	return &topic, nil
}

// DeleteTopic removes a topic record from the database by its ID
func (r *TopicRepository) DeleteTopic(id int) error {
	query := `DELETE FROM topics WHERE id = $1`
	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("%s: failed to delete topic with ID %d: %w", utils.GetCurrentTypeName(), id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("%s: Could not get rows affected after delete: %v", utils.GetCurrentTypeName(), err)
	} else if rowsAffected == 0 {
		return fmt.Errorf("%s: no topic found with ID %d to delete", utils.GetCurrentTypeName(), id)
	}

	return nil
}
