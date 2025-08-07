package implementations

import (
	"database/sql"
	"evo-bot-go/internal/database/prompts"
	"fmt"
	"log"
)

type AddProfileSearchPromptMigration struct {
	BaseMigration
}

func NewAddProfileSearchPromptMigration() *AddProfileSearchPromptMigration {
	return &AddProfileSearchPromptMigration{
		BaseMigration: BaseMigration{
			name:      "add_profile_search_prompt",
			timestamp: "20250807",
		},
	}
}

func (m *AddProfileSearchPromptMigration) Apply(db *sql.DB) error {
	if err := m.insertPromptIfNotExists(db, prompts.GetProfilePromptTemplateDbKey, prompts.GetProfilePromptDefaultTemplate); err != nil {
		return fmt.Errorf("failed to insert profile search prompt: %w", err)
	}

	log.Printf("Migration %s applied successfully", m.name)
	return nil
}

func (m *AddProfileSearchPromptMigration) Rollback(db *sql.DB) error {
	_, err := db.Exec("DELETE FROM prompting_templates WHERE template_key = $1", prompts.GetProfilePromptTemplateDbKey)
	if err != nil {
		return fmt.Errorf("failed to remove profile search prompt: %w", err)
	}

	log.Printf("Migration %s rolled back successfully", m.name)
	return nil
}

func (m *AddProfileSearchPromptMigration) insertPromptIfNotExists(db *sql.DB, key, text string) error {
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM prompting_templates WHERE template_key = $1)", key).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if prompt exists: %w", err)
	}

	if !exists {
		_, err = db.Exec("INSERT INTO prompting_templates (template_key, template_text) VALUES ($1, $2)", key, text)
		if err != nil {
			return fmt.Errorf("failed to insert prompt: %w", err)
		}
		log.Printf("Inserted profile search prompt: %s", key)
	} else {
		log.Printf("Profile search prompt already exists: %s", key)
	}

	return nil
}
