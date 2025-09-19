package repositories

import (
	"database/sql"
	"evo-bot-go/internal/utils"
	"fmt"
	"time"
)

// GroupMessage represents a row in the group_messages table
type GroupMessage struct {
	ID               int
	MessageID        int64
	MessageText      string
	ReplyToMessageID *int64 // nullable
	UserTgID         int64
	GroupTopicID     int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// GroupMessageRepository handles database operations for group messages
type GroupMessageRepository struct {
	db *sql.DB
}

// NewGroupMessageRepository creates a new GroupMessageRepository
func NewGroupMessageRepository(db *sql.DB) *GroupMessageRepository {
	return &GroupMessageRepository{db: db}
}

// GetDB returns the database connection
func (r *GroupMessageRepository) GetDB() *sql.DB {
	return r.db
}

// Create inserts a new group message record into the database
func (r *GroupMessageRepository) Create(messageID int64, messageText string, replyToMessageID *int64, userTgID int64, groupTopicID int64) (*GroupMessage, error) {
	query := `
		INSERT INTO group_messages (message_id, message_text, reply_to_message_id, user_tg_id, group_topic_id) 
		VALUES ($1, $2, $3, $4, $5) 
		RETURNING id, message_id, message_text, reply_to_message_id, user_tg_id, group_topic_id, created_at, updated_at`

	var message GroupMessage
	err := r.db.QueryRow(query, messageID, messageText, replyToMessageID, userTgID, groupTopicID).Scan(
		&message.ID,
		&message.MessageID,
		&message.MessageText,
		&message.ReplyToMessageID,
		&message.UserTgID,
		&message.GroupTopicID,
		&message.CreatedAt,
		&message.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("%s: failed to insert group message: %w", utils.GetCurrentTypeName(), err)
	}

	return &message, nil
}

// GetByID retrieves a group message by ID
func (r *GroupMessageRepository) GetByID(id int) (*GroupMessage, error) {
	query := `
		SELECT id, message_id, message_text, reply_to_message_id, user_tg_id, group_topic_id, created_at, updated_at
		FROM group_messages
		WHERE id = $1`

	var message GroupMessage
	err := r.db.QueryRow(query, id).Scan(
		&message.ID,
		&message.MessageID,
		&message.MessageText,
		&message.ReplyToMessageID,
		&message.UserTgID,
		&message.GroupTopicID,
		&message.CreatedAt,
		&message.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}

	if err != nil {
		return nil, fmt.Errorf("%s: failed to get group message with ID %d: %w", utils.GetCurrentTypeName(), id, err)
	}

	return &message, nil
}

// GetByMessageID retrieves a group message by message ID
func (r *GroupMessageRepository) GetByMessageID(messageID int64) (*GroupMessage, error) {
	query := `
		SELECT id, message_id, message_text, reply_to_message_id, user_tg_id, group_topic_id, created_at, updated_at
		FROM group_messages
		WHERE message_id = $1`

	var message GroupMessage
	err := r.db.QueryRow(query, messageID).Scan(
		&message.ID,
		&message.MessageID,
		&message.MessageText,
		&message.ReplyToMessageID,
		&message.UserTgID,
		&message.GroupTopicID,
		&message.CreatedAt,
		&message.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}

	if err != nil {
		return nil, fmt.Errorf("%s: failed to get group message with message ID %d: %w", utils.GetCurrentTypeName(), messageID, err)
	}

	return &message, nil
}

// GetByUserTgID retrieves group messages by user telegram ID
func (r *GroupMessageRepository) GetByUserTgID(userTgID int64, limit int, offset int) ([]*GroupMessage, error) {
	query := `
		SELECT id, message_id, message_text, reply_to_message_id, user_tg_id, group_topic_id, created_at, updated_at
		FROM group_messages
		WHERE user_tg_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(query, userTgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to get group messages by user telegram ID %d: %w", utils.GetCurrentTypeName(), userTgID, err)
	}
	defer rows.Close()

	var messages []*GroupMessage
	for rows.Next() {
		var message GroupMessage
		err := rows.Scan(
			&message.ID,
			&message.MessageID,
			&message.MessageText,
			&message.ReplyToMessageID,
			&message.UserTgID,
			&message.GroupTopicID,
			&message.CreatedAt,
			&message.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("%s: failed to scan group message: %w", utils.GetCurrentTypeName(), err)
		}
		messages = append(messages, &message)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: error iterating group message rows: %w", utils.GetCurrentTypeName(), err)
	}

	return messages, nil
}

// GetAllByGroupTopicID retrieves all group messages by group topic ID without limit
func (r *GroupMessageRepository) GetAllByGroupTopicID(groupTopicID int64) ([]*GroupMessage, error) {
	query := `
		SELECT id, message_id, message_text, reply_to_message_id, user_tg_id, group_topic_id, created_at, updated_at
		FROM group_messages
		WHERE group_topic_id = $1
		ORDER BY created_at DESC`

	rows, err := r.db.Query(query, groupTopicID)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to get all group messages by group topic ID %d: %w", utils.GetCurrentTypeName(), groupTopicID, err)
	}
	defer rows.Close()

	var messages []*GroupMessage
	for rows.Next() {
		var message GroupMessage
		err := rows.Scan(
			&message.ID,
			&message.MessageID,
			&message.MessageText,
			&message.ReplyToMessageID,
			&message.UserTgID,
			&message.GroupTopicID,
			&message.CreatedAt,
			&message.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("%s: failed to scan group message: %w", utils.GetCurrentTypeName(), err)
		}
		messages = append(messages, &message)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: error iterating group message rows: %w", utils.GetCurrentTypeName(), err)
	}

	return messages, nil
}

// GetByGroupTopicIDForLastDay retrieves group messages by group topic ID for the last 24 hours
func (r *GroupMessageRepository) GetByGroupTopicIdForpreviousTwentyFourHours(groupTopicID int64) ([]*GroupMessage, error) {
	query := `
		SELECT id, message_id, message_text, reply_to_message_id, user_tg_id, group_topic_id, created_at, updated_at
		FROM group_messages
		WHERE group_topic_id = $1 AND created_at >= NOW() - INTERVAL '24 hours'
		ORDER BY created_at ASC`

	rows, err := r.db.Query(query, groupTopicID)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to get group messages by group topic ID %d for last day: %w", utils.GetCurrentTypeName(), groupTopicID, err)
	}
	defer rows.Close()

	var messages []*GroupMessage
	for rows.Next() {
		var message GroupMessage
		err := rows.Scan(
			&message.ID,
			&message.MessageID,
			&message.MessageText,
			&message.ReplyToMessageID,
			&message.UserTgID,
			&message.GroupTopicID,
			&message.CreatedAt,
			&message.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("%s: failed to scan group message: %w", utils.GetCurrentTypeName(), err)
		}
		messages = append(messages, &message)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: error iterating group message rows: %w", utils.GetCurrentTypeName(), err)
	}

	return messages, nil
}

// Update updates a group message record
func (r *GroupMessageRepository) Update(id int, messageText string) error {
	query := `UPDATE group_messages SET message_text = $1, updated_at = NOW() WHERE id = $2`
	result, err := r.db.Exec(query, messageText, id)
	if err != nil {
		return fmt.Errorf("%s: failed to update group message with ID %d: %w", utils.GetCurrentTypeName(), id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: could not get rows affected after update: %w", utils.GetCurrentTypeName(), err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("%s: no group message found with ID %d to update", utils.GetCurrentTypeName(), id)
	}

	return nil
}

// Delete removes a group message record from the database by its ID
func (r *GroupMessageRepository) Delete(id int) error {
	query := `DELETE FROM group_messages WHERE id = $1`
	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("%s: failed to delete group message with ID %d: %w", utils.GetCurrentTypeName(), id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: could not get rows affected after delete: %w", utils.GetCurrentTypeName(), err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("%s: no group message found with ID %d to delete", utils.GetCurrentTypeName(), id)
	}

	return nil
}

// DeleteByMessageID removes a group message record from the database by message ID
func (r *GroupMessageRepository) DeleteByMessageID(messageID int64) error {
	query := `DELETE FROM group_messages WHERE message_id = $1`
	result, err := r.db.Exec(query, messageID)
	if err != nil {
		return fmt.Errorf("%s: failed to delete group message with message ID %d: %w", utils.GetCurrentTypeName(), messageID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: could not get rows affected after delete: %w", utils.GetCurrentTypeName(), err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("%s: no group message found with message ID %d to delete", utils.GetCurrentTypeName(), messageID)
	}

	return nil
}
