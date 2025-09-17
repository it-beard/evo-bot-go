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

// Get retrieves a prompt by its key
func (r *PromptingTemplateRepository) Get(key string, defaultValue string) (string, error) {
	var templateText string

	err := r.db.QueryRow(
		`SELECT template_text FROM prompting_templates WHERE template_key = $1`,
		key,
	).Scan(&templateText)

	if err != nil {
		if err == sql.ErrNoRows {
			// Insert default value when not found and return it
			_, insertErr := r.db.Exec(
				`INSERT INTO prompting_templates (template_key, template_text) VALUES ($1, $2) ON CONFLICT (template_key) DO NOTHING`,
				key,
				defaultValue,
			)
			if insertErr != nil {
				return "", fmt.Errorf("%s: failed to insert default prompting template: %w", utils.GetCurrentTypeName(), insertErr)
			}
			return defaultValue, nil
		}
		return "", fmt.Errorf("%s: failed to get prompting template: %w", utils.GetCurrentTypeName(), err)
	}

	return templateText, nil
}
