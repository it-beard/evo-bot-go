package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// Message represents a stored message
type Message struct {
	ID               int64
	TopicID          int
	MessageID        int
	ReplyToMessageID *int
	UserID           int64
	Username         string
	Text             string
	CreatedAt        time.Time
}

// MessageStore handles message storage operations
type MessageStore struct {
	db *DB
}

// NewMessageStore creates a new message store
func NewMessageStore(db *DB) *MessageStore {
	return &MessageStore{db: db}
}

// StoreMessage stores a message in the database
func (s *MessageStore) StoreMessage(ctx context.Context, msg *gotgbot.Message) error {
	// Extract message text (could be in Caption for media messages)
	text := msg.Text
	if text == "" {
		text = msg.Caption
	}

	// Skip empty messages
	if text == "" {
		return nil
	}

	// Get username or first name if username is empty
	username := msg.From.Username
	if username == "" {
		username = msg.From.FirstName
	}

	// Convert Unix timestamp to time.Time
	createdAt := time.Unix(int64(msg.Date), 0)

	// if msg.ReplyToMessage is nil set reply_to_message_id to null
	var replyToMessageId *int64
	if msg.ReplyToMessage != nil {
		replyToMessageId = &msg.ReplyToMessage.MessageId
	}

	// if replyToMessageId is the same as MessageThreadId, set it to null
	if replyToMessageId != nil && *replyToMessageId == msg.MessageThreadId {
		replyToMessageId = nil
	}

	//small hack for root topic 0 (1 in links)
	var messageTopicId int
	if !msg.IsTopicMessage {
		messageTopicId = 0
	} else {
		messageTopicId = int(msg.MessageThreadId)
	}

	// Insert message into database
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO messages (topic_id, message_id, reply_to_message_id, user_id, username, message_text, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (topic_id, message_id) DO UPDATE
		SET message_text = $6, username = $5`,
		messageTopicId, msg.MessageId, replyToMessageId, msg.From.Id, username, text, createdAt,
	)
	if err != nil {
		return fmt.Errorf("failed to store message: %w", err)
	}

	return nil
}

// GetRecentMessages retrieves messages from the past 24 hours for a specific topic
func (s *MessageStore) GetRecentMessages(ctx context.Context, chatID int, since time.Time) ([]Message, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, topic_id, message_id, reply_to_message_id, user_id, username, message_text, created_at
		FROM messages
		WHERE topic_id = $1 AND created_at >= $2
		ORDER BY created_at ASC`,
		chatID, since,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		if err := rows.Scan(
			&msg.ID,
			&msg.TopicID,
			&msg.MessageID,
			&msg.ReplyToMessageID,
			&msg.UserID,
			&msg.Username,
			&msg.Text,
			&msg.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan message row: %w", err)
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating message rows: %w", err)
	}

	return messages, nil
}
