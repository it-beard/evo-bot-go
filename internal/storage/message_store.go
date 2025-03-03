package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// Message represents a stored message
type Message struct {
	ID        int64
	ChatID    int64
	MessageID int64
	UserID    int64
	Username  string
	Text      string
	CreatedAt time.Time
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

	// Insert message into database
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO messages (chat_id, message_id, user_id, username, message_text, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (chat_id, message_id) DO UPDATE
		SET message_text = $5, username = $4`,
		msg.Chat.Id, msg.MessageId, msg.From.Id, username, text, createdAt,
	)
	if err != nil {
		return fmt.Errorf("failed to store message: %w", err)
	}

	return nil
}

// GetRecentMessages retrieves messages from the past 24 hours for a specific chat
func (s *MessageStore) GetRecentMessages(ctx context.Context, chatID int64, since time.Time) ([]Message, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, chat_id, message_id, user_id, username, message_text, created_at
		FROM messages
		WHERE chat_id = $1 AND created_at >= $2
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
			&msg.ChatID,
			&msg.MessageID,
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

// GetChatName retrieves the chat name for a given chat ID
func (s *MessageStore) GetChatName(ctx context.Context, chatID int64) (string, error) {
	var chatName string
	err := s.db.QueryRowContext(
		ctx,
		`SELECT username FROM messages WHERE chat_id = $1 LIMIT 1`,
		chatID,
	).Scan(&chatName)

	if err == sql.ErrNoRows {
		return fmt.Sprintf("Chat %d", chatID), nil
	} else if err != nil {
		return "", fmt.Errorf("failed to get chat name: %w", err)
	}

	return chatName, nil
}
