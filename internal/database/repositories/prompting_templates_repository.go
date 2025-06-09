package repositories

import (
	"database/sql"
	"evo-bot-go/internal/utils"
	"fmt"
)

// PromptingTemplate represents a stored prompt template in the database
type PromptingTemplate struct {
	TemplateKey  string
	TemplateText string
}

// PromptingTemplateRepository handles prompt template persistence operations
type PromptingTemplateRepository struct {
	db *sql.DB
}

// NewPromptingTemplateRepository creates a new prompting template repository
func NewPromptingTemplateRepository(db *sql.DB) *PromptingTemplateRepository {
	return &PromptingTemplateRepository{db: db}
}

// Get retrieves a prompt template by its key
func (r *PromptingTemplateRepository) Get(templateKey string) (string, error) {
	var templateText string

	err := r.db.QueryRow(
		`SELECT template_text FROM prompting_templates WHERE template_key = $1`,
		templateKey,
	).Scan(&templateText)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // Return empty string if template not found
		}
		return "", fmt.Errorf("%s: failed to get prompting template: %w", utils.GetCurrentTypeName(), err)
	}

	return templateText, nil
}
