package repositories

import (
	"database/sql"
	"evo-bot-go/internal/utils"
	"fmt"
	"time"
)

// GroupTopic represents a row in the group_topics table
type GroupTopic struct {
	ID        int
	TopicID   int64
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// GroupTopicRepository handles database operations for group topics
type GroupTopicRepository struct {
	db *sql.DB
}

// NewGroupTopicRepository creates a new GroupTopicRepository
func NewGroupTopicRepository(db *sql.DB) *GroupTopicRepository {
	return &GroupTopicRepository{db: db}
}

// AddGroupTopic inserts a new group topic record into the database
func (r *GroupTopicRepository) AddGroupTopic(topicID int64, name string) (*GroupTopic, error) {
	var groupTopic GroupTopic
	query := `
		INSERT INTO group_topics (topic_id, name) 
		VALUES ($1, $2) 
		RETURNING id, topic_id, name, created_at, updated_at`

	err := r.db.QueryRow(query, topicID, name).Scan(
		&groupTopic.ID,
		&groupTopic.TopicID,
		&groupTopic.Name,
		&groupTopic.CreatedAt,
		&groupTopic.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to insert group topic: %w", utils.GetCurrentTypeName(), err)
	}
	return &groupTopic, nil
}

// UpdateGroupTopic updates an existing group topic's name by topic_id
func (r *GroupTopicRepository) UpdateGroupTopic(topicID int64, name string) (*GroupTopic, error) {
	var groupTopic GroupTopic
	query := `
		UPDATE group_topics 
		SET name = $1, updated_at = NOW() 
		WHERE topic_id = $2
		RETURNING id, topic_id, name, created_at, updated_at`

	err := r.db.QueryRow(query, name, topicID).Scan(
		&groupTopic.ID,
		&groupTopic.TopicID,
		&groupTopic.Name,
		&groupTopic.CreatedAt,
		&groupTopic.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%s: no group topic found with topic_id %d", utils.GetCurrentTypeName(), topicID)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: failed to update group topic with topic_id %d: %w", utils.GetCurrentTypeName(), topicID, err)
	}
	return &groupTopic, nil
}

// GetGroupTopicByTopicID retrieves a group topic by its topic_id
func (r *GroupTopicRepository) GetGroupTopicByTopicID(topicID int64) (*GroupTopic, error) {
	query := `
		SELECT id, topic_id, name, created_at, updated_at
		FROM group_topics
		WHERE topic_id = $1`

	var groupTopic GroupTopic
	err := r.db.QueryRow(query, topicID).Scan(
		&groupTopic.ID,
		&groupTopic.TopicID,
		&groupTopic.Name,
		&groupTopic.CreatedAt,
		&groupTopic.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%s: no group topic found with topic_id %d", utils.GetCurrentTypeName(), topicID)
	}

	if err != nil {
		return nil, fmt.Errorf("%s: failed to get group topic with topic_id %d: %w", utils.GetCurrentTypeName(), topicID, err)
	}

	return &groupTopic, nil
}

// GetAllGroupTopics retrieves all group topics
func (r *GroupTopicRepository) GetAllGroupTopics() ([]GroupTopic, error) {
	query := `
		SELECT id, topic_id, name, created_at, updated_at
		FROM group_topics
		ORDER BY created_at ASC`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to query group topics: %w", utils.GetCurrentTypeName(), err)
	}
	defer rows.Close()

	var groupTopics []GroupTopic
	for rows.Next() {
		var gt GroupTopic
		if err := rows.Scan(&gt.ID, &gt.TopicID, &gt.Name, &gt.CreatedAt, &gt.UpdatedAt); err != nil {
			return nil, fmt.Errorf("%s: failed to scan group topic row: %w", utils.GetCurrentTypeName(), err)
		}
		groupTopics = append(groupTopics, gt)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: error during rows iteration for group topics: %w", utils.GetCurrentTypeName(), err)
	}

	return groupTopics, nil
}

// DeleteGroupTopic removes a group topic by its topic_id
func (r *GroupTopicRepository) DeleteGroupTopic(topicID int64) error {
	query := `DELETE FROM group_topics WHERE topic_id = $1`
	result, err := r.db.Exec(query, topicID)
	if err != nil {
		return fmt.Errorf("%s: failed to delete group topic with topic_id %d: %w", utils.GetCurrentTypeName(), topicID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: could not get rows affected after delete: %w", utils.GetCurrentTypeName(), err)
	} else if rowsAffected == 0 {
		return fmt.Errorf("%s: no group topic found with topic_id %d to delete", utils.GetCurrentTypeName(), topicID)
	}

	return nil
}
