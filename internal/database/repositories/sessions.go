package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/gotd/td/session"
	"github.com/it-beard/evo-bot-go/internal/database"
)

// SessionRepository implements the session.Storage interface for
// persisting Telegram session data in PostgreSQL
type SessionRepository struct {
	db *database.DB
}

// Ensure SessionRepository implements session.Storage interface
var _ session.Storage = (*SessionRepository)(nil)

// NewSessionRepository creates a new session repository
func NewSessionRepository(db *database.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// LoadSession retrieves session data from the database
func (r *SessionRepository) LoadSession(ctx context.Context) ([]byte, error) {
	var data []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT data FROM tg_sessions 
		WHERE id = 'telegram_session'
	`).Scan(&data)

	if err == sql.ErrNoRows {
		// No session found, return empty data
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	return data, nil
}

// StoreSession saves session data to the database
func (r *SessionRepository) StoreSession(ctx context.Context, data []byte) error {
	_, err := r.db.ExecContext(ctx, `
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
