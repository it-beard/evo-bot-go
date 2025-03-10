package repositories

import (
	"context"
	"database/sql"
	"fmt"

	"evo-bot-go/internal/database"
)

// PromptingTemplate represents a stored prompt template in the database
type PromptingTemplate struct {
	TemplateKey  string
	TemplateText string
}

// PromptingTemplateRepository handles prompt template persistence operations
type PromptingTemplateRepository struct {
	db *database.DB
}

// NewPromptingTemplateRepository creates a new prompting template repository
func NewPromptingTemplateRepository(db *database.DB) *PromptingTemplateRepository {
	return &PromptingTemplateRepository{db: db}
}

// Store persists a prompt template to the database
func (r *PromptingTemplateRepository) Store(ctx context.Context, templateKey, templateText string) error {
	// Insert or update template in database
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO prompting_templates (template_key, template_text)
		VALUES ($1, $2)
		ON CONFLICT (template_key) DO UPDATE
		SET template_text = $2`,
		templateKey, templateText,
	)
	if err != nil {
		return fmt.Errorf("failed to store prompting template: %w", err)
	}

	return nil
}

// Get retrieves a prompt template by its key
func (r *PromptingTemplateRepository) Get(ctx context.Context, templateKey string) (string, error) {
	var templateText string

	err := r.db.QueryRowContext(
		ctx,
		`SELECT template_text FROM prompting_templates WHERE template_key = $1`,
		templateKey,
	).Scan(&templateText)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // Return empty string if template not found
		}
		return "", fmt.Errorf("failed to get prompting template: %w", err)
	}

	return templateText, nil
}

// EnsureTemplateExists checks if a template exists and adds it if it doesn't
func (r *PromptingTemplateRepository) EnsureTemplateExists(ctx context.Context, templateKey, templateText string) error {
	// Check if template exists
	existing, err := r.Get(ctx, templateKey)
	if err != nil {
		return err
	}

	// If template doesn't exist or is empty, store it
	if existing == "" {
		return r.Store(ctx, templateKey, templateText)
	}

	return nil
}
