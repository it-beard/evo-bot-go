package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"evo-bot-go/internal/database"
)

// Message represents a stored message in the database
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

// MessageRepository handles message persistence operations
type MessageRepository struct {
	db *database.DB
}

// NewMessageRepository creates a new message repository
func NewMessageRepository(db *database.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

// Store persists a Telegram message to the database
func (r *MessageRepository) Store(ctx context.Context, msg *gotgbot.Message) error {
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

	// Handle reply references
	var replyToMessageID *int64
	if msg.ReplyToMessage != nil {
		replyToMessageID = &msg.ReplyToMessage.MessageId
	}

	// If reply is to the topic starter message, treat as no reply
	if replyToMessageID != nil && *replyToMessageID == msg.MessageThreadId {
		replyToMessageID = nil
	}

	// Determine topic ID (0 for non-topic messages)
	topicID := 0
	if msg.IsTopicMessage {
		topicID = int(msg.MessageThreadId)
	}

	// Insert or update message in database
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO messages (topic_id, message_id, reply_to_message_id, user_id, username, message_text, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (topic_id, message_id) DO UPDATE
		SET message_text = $6, username = $5`,
		topicID, msg.MessageId, replyToMessageID, msg.From.Id, username, text, createdAt,
	)
	if err != nil {
		return fmt.Errorf("failed to store message: %w", err)
	}

	return nil
}

// GetRecent retrieves messages from a specific topic since the provided time
func (r *MessageRepository) GetRecent(ctx context.Context, topicID int, since time.Time) ([]Message, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, topic_id, message_id, reply_to_message_id, user_id, username, message_text, created_at
		FROM messages
		WHERE topic_id = $1 AND created_at >= $2
		ORDER BY created_at ASC`,
		topicID, since,
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
