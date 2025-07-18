package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"evo-bot-go/internal/database"
	"evo-bot-go/internal/utils"

	"github.com/gotd/td/session"
)

// SessionRepository implements the session.Storage interface for
// persisting Telegram session data in PostgreSQL
type TgSessionRepository struct {
	db *database.DB
}

// Ensure TgSessionRepository implements session.Storage interface
var _ session.Storage = (*TgSessionRepository)(nil)

// NewTgSessionRepository creates a new session repository
func NewTgSessionRepository(db *database.DB) *TgSessionRepository {
	return &TgSessionRepository{db: db}
}

// LoadSession retrieves session data from the database
func (r *TgSessionRepository) LoadSession(ctx context.Context) ([]byte, error) {
	var data []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT data FROM tg_sessions 
		WHERE id = 'telegram_session'
	`).Scan(&data)

	if err == sql.ErrNoRows {
		// No session found, return empty data
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("%s: failed to load session: %w", utils.GetCurrentTypeName(), err)
	}

	return data, nil
}

// StoreSession saves session data to the database
func (r *TgSessionRepository) StoreSession(ctx context.Context, data []byte) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO tg_sessions (id, data, updated_at)
		VALUES ('telegram_session', $1, $2)
		ON CONFLICT (id) DO UPDATE
		SET data = $1, updated_at = $2
	`, data, time.Now())

	if err != nil {
		return fmt.Errorf("%s: failed to store session: %w", utils.GetCurrentTypeName(), err)
	}

	return nil
}
