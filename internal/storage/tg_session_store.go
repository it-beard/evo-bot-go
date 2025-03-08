package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/gotd/td/session"
)

// SessionStore is a PostgreSQL implementation of session.Storage interface
// from github.com/gotd/td/session
type SessionStore struct {
	db *DB
}

// Ensure SessionStore implements session.Storage interface
var _ session.Storage = (*SessionStore)(nil)

// NewSessionStore creates a new session store with the given DB connection
func NewSessionStore(db *DB) (*SessionStore, error) {
	return &SessionStore{db: db}, nil
}

// LoadSession loads session data from the database
func (s *SessionStore) LoadSession(ctx context.Context) ([]byte, error) {
	var data []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT data FROM tg_sessions 
		WHERE id = 'telegram_session'
	`).Scan(&data)

	if err == sql.ErrNoRows {
		// No data found, return empty
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	return data, nil
}

// StoreSession saves session data to the database
func (s *SessionStore) StoreSession(ctx context.Context, data []byte) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO tg_sessions (id, data, updated_at)
		VALUES ('telegram_session', $1, $2)
		ON CONFLICT (id) DO UPDATE
		SET data = $1, updated_at = $2
	`, data, time.Now())

	if err != nil {
		return fmt.Errorf("failed to store session: %w", err)
	}

	return nil
}
