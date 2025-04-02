package services

import (
	"context"
	"log"

	"evo-bot-go/internal/constants/prompts"
	"evo-bot-go/internal/database/repositories"
)

type PromptingTemplateService struct {
	repo *repositories.PromptingTemplateRepository
}

func NewPromptingTemplateService(repo *repositories.PromptingTemplateRepository) *PromptingTemplateService {
	return &PromptingTemplateService{repo: repo}
}

func (s *PromptingTemplateService) InitializeDefaultTemplates(ctx context.Context) error {
	log.Println("Initializing default prompting templates...")

	// Initialize summarization prompts
	if err := s.repo.EnsureTemplateExists(ctx, prompts.DailySummarizationPromptTemplateDbKey, prompts.DailySummarizationPromptDefaultTemplate); err != nil {
		return err
	}

	// Initialize content prompts
	if err := s.repo.EnsureTemplateExists(ctx, prompts.GetContentPromptTemplateDbKey, prompts.GetContentPromptDefaultTemplate); err != nil {
		return err
	}

	// Initialize tool prompts
	if err := s.repo.EnsureTemplateExists(ctx, prompts.GetToolPromptTemplateDbKey, prompts.GetToolPromptDefaultTemplate); err != nil {
		return err
	}

	log.Println("Default prompting templates initialized successfully")
	return nil
}

func (s *PromptingTemplateService) GetTemplateWithFallback(ctx context.Context, templateKey string, defaultTemplate string) string {
	templateText, err := s.repo.Get(ctx, templateKey)
	if err != nil {
		log.Printf("Warning: Failed to get prompt template from database, using default: %v", err)
		return defaultTemplate
	}

	// If template is empty (not found in DB), use the default
	if templateText == "" {
		return defaultTemplate
	}

	return templateText
}
