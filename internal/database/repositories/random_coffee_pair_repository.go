package repositories

import (
	"database/sql"
	"fmt"
	"time"
)

type RandomCoffeePair struct {
	ID        int
	PollID    int
	User1ID   int64
	User2ID   int64
	CreatedAt time.Time
}

type RandomCoffeePairRepository struct {
	db *sql.DB
}

func NewRandomCoffeePairRepository(db *sql.DB) *RandomCoffeePairRepository {
	return &RandomCoffeePairRepository{db: db}
}

func (r *RandomCoffeePairRepository) CreatePair(pollID int, user1ID, user2ID int) error {
	query := `
		INSERT INTO random_coffee_pairs (poll_id, user1_id, user2_id)
		VALUES ($1, $2, $3)
	`
	_, err := r.db.Exec(query, pollID, user1ID, user2ID)
	if err != nil {
		return fmt.Errorf("error creating random coffee pair: %w", err)
	}
	return nil
}
