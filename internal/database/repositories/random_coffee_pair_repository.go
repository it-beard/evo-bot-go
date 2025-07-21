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

// GetPairsHistoryForUsers returns historical pairs for specified users from last N polls
func (r *RandomCoffeePairRepository) GetPairsHistoryForUsers(userIDs []int, lastNPolls int) (map[string][]int, error) {
	if len(userIDs) == 0 {
		return make(map[string][]int), nil
	}

	// Convert userIDs to a format suitable for SQL IN clause
	placeholders := ""
	args := make([]interface{}, len(userIDs)+1)
	for i, userID := range userIDs {
		if i > 0 {
			placeholders += ","
		}
		placeholders += fmt.Sprintf("$%d", i+2)
		args[i+1] = userID
	}
	args[0] = lastNPolls

	query := fmt.Sprintf(`
		SELECT p.user1_id, p.user2_id, poll.id as poll_id
		FROM random_coffee_pairs p
		JOIN random_coffee_polls poll ON p.poll_id = poll.id
		WHERE (p.user1_id IN (%s) OR p.user2_id IN (%s))
		ORDER BY poll.week_start_date DESC
		LIMIT (SELECT COUNT(*) FROM random_coffee_pairs pairs 
		       JOIN random_coffee_polls polls ON pairs.poll_id = polls.id 
		       WHERE polls.id IN (
		           SELECT id FROM random_coffee_polls 
		           ORDER BY week_start_date DESC 
		           LIMIT $1
		       ))
	`, placeholders, placeholders)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("error getting pairs history: %w", err)
	}
	defer rows.Close()

	pairHistory := make(map[string][]int)

	for rows.Next() {
		var user1ID, user2ID, pollID int
		if err := rows.Scan(&user1ID, &user2ID, &pollID); err != nil {
			return nil, fmt.Errorf("error scanning pair history row: %w", err)
		}

		// Create keys for both directions (user1-user2 and user2-user1)
		key1 := fmt.Sprintf("%d-%d", user1ID, user2ID)
		key2 := fmt.Sprintf("%d-%d", user2ID, user1ID)

		pairHistory[key1] = append(pairHistory[key1], pollID)
		pairHistory[key2] = append(pairHistory[key2], pollID)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration for pair history: %w", err)
	}

	return pairHistory, nil
}

// GetMostRecentPairPoll returns the most recent poll ID where two users were paired, or 0 if never paired
func (r *RandomCoffeePairRepository) GetMostRecentPairPoll(user1ID, user2ID int, lastNPolls int) (int, error) {
	query := `
		SELECT poll.id
		FROM random_coffee_pairs p
		JOIN random_coffee_polls poll ON p.poll_id = poll.id
		WHERE ((p.user1_id = $1 AND p.user2_id = $2) OR (p.user1_id = $2 AND p.user2_id = $1))
		AND poll.id IN (
			SELECT id FROM random_coffee_polls 
			ORDER BY week_start_date DESC 
			LIMIT $3
		)
		ORDER BY poll.week_start_date DESC
		LIMIT 1
	`

	var pollID int
	err := r.db.QueryRow(query, user1ID, user2ID, lastNPolls).Scan(&pollID)
	if err == sql.ErrNoRows {
		return 0, nil // Never paired
	}
	if err != nil {
		return 0, fmt.Errorf("error getting most recent pair poll: %w", err)
	}

	return pollID, nil
}
