package repositories

import (
	"database/sql"
	"evo-bot-go/internal/constants"
	"fmt"
	"log"
	"time"
)

// Event represents a row in the events table
type Event struct {
	ID        int
	Name      string
	Type      string
	Status    string
	StartedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// EventRepository handles database operations for events
type EventRepository struct {
	db              *sql.DB
	topicRepository *TopicRepository
}

// NewEventRepository creates a new EventRepository
func NewEventRepository(db *sql.DB) *EventRepository {
	return &EventRepository{
		db:              db,
		topicRepository: NewTopicRepository(db),
	}
}

// CreateEvent inserts a new event record into the database
func (r *EventRepository) CreateEvent(name string, eventType constants.EventType) (int, error) {
	var id int
	query := `INSERT INTO events (name, type, status) VALUES ($1, $2, $3) RETURNING id`
	err := r.db.QueryRow(query, name, eventType, constants.EventStatusActual).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to insert event: %w", err)
	}
	return id, nil
}

// CreateEventWithStartedAt inserts a new event record with a started_at value into the database
func (r *EventRepository) CreateEventWithStartedAt(name string, eventType constants.EventType, startedAt time.Time) (int, error) {
	var id int
	query := `INSERT INTO events (name, type, status, started_at) VALUES ($1, $2, $3, $4) RETURNING id`
	err := r.db.QueryRow(query, name, eventType, constants.EventStatusActual, startedAt).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to insert event with started_at: %w", err)
	}
	return id, nil
}

// GetLastActualEvents retrieves the last N actual event records
func (r *EventRepository) GetLastActualEvents(limit int) ([]Event, error) {
	query := `
		SELECT id, name, type, status, started_at, created_at, updated_at
		FROM events
		WHERE status = $1
		ORDER BY started_at ASC NULLS LAST
		LIMIT $2`

	rows, err := r.db.Query(query, constants.EventStatusActual, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query last events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Name, &e.Type, &e.Status, &e.StartedAt, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan event row: %w", err)
		}
		events = append(events, e)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration for events: %w", err)
	}

	return events, nil
}

// GetLastEvents retrieves the last N event records
func (r *EventRepository) GetLastEvents(limit int) ([]Event, error) {
	query := `
		SELECT id, name, type, status, started_at, created_at, updated_at
		FROM events
		ORDER BY started_at ASC NULLS LAST
		LIMIT $1`

	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query last events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Name, &e.Type, &e.Status, &e.StartedAt, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan event row: %w", err)
		}
		events = append(events, e)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration for events: %w", err)
	}

	return events, nil
}

// UpdateEventName updates the name of an event record by its ID
func (r *EventRepository) UpdateEventName(id int, newName string) error {
	query := `UPDATE events SET name = $1, updated_at = NOW() WHERE id = $2`
	result, err := r.db.Exec(query, newName, id)
	if err != nil {
		return fmt.Errorf("failed to update event name for ID %d: %w", id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		// Log the error but don't fail the operation if rowsAffected can't be retrieved
		log.Printf("Could not get rows affected after update: %v", err)
	} else if rowsAffected == 0 {
		return fmt.Errorf("no event found with ID %d to update", id)
	}

	return nil
}

// UpdateEventStatus updates the status of an event record by its ID
func (r *EventRepository) UpdateEventStatus(id int, status constants.EventStatus) error {
	query := `UPDATE events SET status = $1, updated_at = NOW() WHERE id = $2`
	result, err := r.db.Exec(query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update event status for ID %d: %w", id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Could not get rows affected after update: %v", err)
	} else if rowsAffected == 0 {
		return fmt.Errorf("no event found with ID %d to update status", id)
	}

	return nil
}

// UpdateEventStartedAt updates the started_at field of an event record by its ID
func (r *EventRepository) UpdateEventStartedAt(id int, startedAt time.Time) error {
	query := `UPDATE events SET started_at = $1, updated_at = NOW() WHERE id = $2`
	result, err := r.db.Exec(query, startedAt, id)
	if err != nil {
		return fmt.Errorf("failed to update event started_at for ID %d: %w", id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Could not get rows affected after update: %v", err)
	} else if rowsAffected == 0 {
		return fmt.Errorf("no event found with ID %d to update started_at", id)
	}

	return nil
}

// UpdateEventType updates the type of an event record by its ID
func (r *EventRepository) UpdateEventType(id int, eventType constants.EventType) error {
	query := `UPDATE events SET type = $1, updated_at = NOW() WHERE id = $2`
	result, err := r.db.Exec(query, string(eventType), id)
	if err != nil {
		return fmt.Errorf("failed to update event type for ID %d: %w", id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Could not get rows affected after update: %v", err)
	} else if rowsAffected == 0 {
		return fmt.Errorf("no event found with ID %d to update type", id)
	}

	return nil
}

// DeleteEvent removes an event record from the database by its ID
func (r *EventRepository) DeleteEvent(id int) error {
	// First, get all topics related to this event
	topics, err := r.topicRepository.GetTopicsByEventID(id)
	if err != nil {
		return fmt.Errorf("failed to get topics for event ID %d: %w", id, err)
	}

	// Delete all related topics
	for _, topic := range topics {
		if err := r.topicRepository.DeleteTopic(topic.ID); err != nil {
			return fmt.Errorf("failed to delete related topic with ID %d: %w", topic.ID, err)
		}
	}

	// Now delete the event itself
	query := `DELETE FROM events WHERE id = $1`
	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete event with ID %d: %w", id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Could not get rows affected after delete: %v", err)
	} else if rowsAffected == 0 {
		return fmt.Errorf("no event found with ID %d to delete", id)
	}

	return nil
}

// GetEventByID retrieves a single event record by its ID
func (r *EventRepository) GetEventByID(id int) (*Event, error) {
	query := `
		SELECT id, name, type, status, started_at, created_at, updated_at
		FROM events
		WHERE id = $1`

	var event Event
	err := r.db.QueryRow(query, id).Scan(
		&event.ID,
		&event.Name,
		&event.Type,
		&event.Status,
		&event.StartedAt,
		&event.CreatedAt,
		&event.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no event found with ID %d", id)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get event with ID %d: %w", id, err)
	}

	return &event, nil
}
