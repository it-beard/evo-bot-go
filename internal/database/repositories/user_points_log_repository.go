package repositories

import (
	"database/sql"
	"evo-bot-go/internal/utils"
	"fmt"
	"time"
)

type UserPointsLog struct {
	ID       int64     `db:"id"`
	UserID   int       `db:"user_id"`
	Points   int       `db:"points"`
	Reason   string    `db:"reason"`
	PollID   *int64    `db:"poll_id"`
	CreatedAt time.Time `db:"created_at"`
}

type UserPointsLogRepository struct {
	db *sql.DB
}

func NewUserPointsLogRepository(db *sql.DB) *UserPointsLogRepository {
	return &UserPointsLogRepository{db: db}
}

// AddPoints добавляет очки пользователю и записывает в лог
func (r *UserPointsLogRepository) AddPoints(userID int, points int, reason string, pollID *int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("%s: failed to begin transaction: %w", utils.GetCurrentTypeName(), err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Добавляем запись в лог
	_, err = tx.Exec(`
		INSERT INTO user_points_log (user_id, points, reason, poll_id, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`, userID, points, reason, pollID)
	if err != nil {
		return fmt.Errorf("%s: failed to insert points log: %w", utils.GetCurrentTypeName(), err)
	}

	// Обновляем общий счет пользователя
	_, err = tx.Exec(`
		UPDATE users 
		SET score = score + $1, updated_at = NOW() 
		WHERE id = $2
	`, points, userID)
	if err != nil {
		return fmt.Errorf("%s: failed to update user score: %w", utils.GetCurrentTypeName(), err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%s: failed to commit transaction: %w", utils.GetCurrentTypeName(), err)
	}

	return nil
}

// GetUserPointsHistory получает историю начислений очков для пользователя
func (r *UserPointsLogRepository) GetUserPointsHistory(userID int, limit int) ([]UserPointsLog, error) {
	query := `
		SELECT id, user_id, points, reason, poll_id, created_at
		FROM user_points_log
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`
	
	rows, err := r.db.Query(query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to query points history: %w", utils.GetCurrentTypeName(), err)
	}
	defer rows.Close()

	var logs []UserPointsLog
	for rows.Next() {
		var log UserPointsLog
		err := rows.Scan(
			&log.ID,
			&log.UserID,
			&log.Points,
			&log.Reason,
			&log.PollID,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("%s: failed to scan points log: %w", utils.GetCurrentTypeName(), err)
		}
		logs = append(logs, log)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: error during rows iteration: %w", utils.GetCurrentTypeName(), err)
	}

	return logs, nil
}

// GetPointsForPoll проверяет, получил ли пользователь уже очки за конкретный опрос
func (r *UserPointsLogRepository) GetPointsForPoll(userID int, pollID int64) (*UserPointsLog, error) {
	query := `
		SELECT id, user_id, points, reason, poll_id, created_at
		FROM user_points_log
		WHERE user_id = $1 AND poll_id = $2
		LIMIT 1
	`
	
	var log UserPointsLog
	err := r.db.QueryRow(query, userID, pollID).Scan(
		&log.ID,
		&log.UserID,
		&log.Points,
		&log.Reason,
		&log.PollID,
		&log.CreatedAt,
	)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	
	if err != nil {
		return nil, fmt.Errorf("%s: failed to get points for poll: %w", utils.GetCurrentTypeName(), err)
	}

	return &log, nil
}