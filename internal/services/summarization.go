package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"evo-bot-go/internal/clients"
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/database/prompts"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/gotd/td/tg"
)

// SummarizationService handles the daily summarization of messages
type SummarizationService struct {
	config                      *config.Config
	openaiClient                *clients.OpenAiClient
	messageSenderService        MessageSenderService
	promptingTemplateRepository *repositories.PromptingTemplateRepository
}

// NewSummarizationService creates a new summarization service
func NewSummarizationService(
	config *config.Config,
	openaiClient *clients.OpenAiClient,
	messageSenderService MessageSenderService,
	promptingTemplateRepository *repositories.PromptingTemplateRepository,
) *SummarizationService {
	return &SummarizationService{
		config:                      config,
		openaiClient:                openaiClient,
		messageSenderService:        messageSenderService,
		promptingTemplateRepository: promptingTemplateRepository,
	}
}

// RunDailySummarization runs the daily summarization process
func (s *SummarizationService) RunDailySummarization(ctx context.Context, sendToDM bool) error {
	log.Println("Starting daily summarization process")

	// Get the time 24 hours ago
	since := time.Now().Add(-24 * time.Hour)

	// Process each monitored topic
	for _, topicID := range s.config.MonitoredTopicsIDs {
		if err := s.summarizeTopicMessages(ctx, topicID, since, sendToDM); err != nil {
			log.Printf("Error summarizing topic %d: %v", topicID, err)
			// Continue with other chats even if one fails
			continue
		}
	}

	log.Println("Daily summarization process completed")
	return nil
}

// summarizeTopicMessages summarizes a single topic
func (s *SummarizationService) summarizeTopicMessages(ctx context.Context, topicID int, since time.Time, sendToDM bool) error {
	// Get topic name
	topicName, err := utils.GetTopicName(topicID)
	if err != nil {
		return fmt.Errorf("failed to get topic name: %w", err)
	}

	// Calculate hours since the given time
	hoursSince := int(time.Since(since).Hours()) + 1 // Add 1 to ensure we get all messages since 'since' time

	// Get messages directly from Telegram instead of database
	tgMessages, err := clients.GetLastTopicMessagesByTime(s.config.SuperGroupChatID, topicID, hoursSince)
	if err != nil {
		return fmt.Errorf("failed to get messages from Telegram: %w", err)
	}

	if len(tgMessages) == 0 {
		log.Printf("No messages found for topic %d since %v", topicID, since)
		return nil
	}

	log.Printf("Found %d messages for topic %d", len(tgMessages), topicID)

	// Build context directly from all messages without using RAG
	context := ""
	for _, msg := range tgMessages {
		// Convert Unix timestamp to time.Time
		msgTime := time.Unix(int64(msg.Date), 0)

		// Extract username/first name from the message
		username := "Unknown"
		if msg.FromID != nil {
			if userID, ok := msg.FromID.(*tg.PeerUser); ok && userID != nil {
				// Just use the user ID as a placeholder since we don't have easy access to username
				username = fmt.Sprintf("User %d", userID.UserID)
			}
		}

		context += fmt.Sprintf("[%s] %s: %s\n",
			msgTime.Format("2006-01-02 15:04:05"),
			username,
			msg.Message)
	}

	// Get the prompt template from the database with fallback to default
	templateText, err := s.promptingTemplateRepository.Get(prompts.DailySummarizationPromptTemplateDbKey)
	if err != nil {
		return fmt.Errorf("failed to get prompt template: %w", err)
	}

	// Generate summary using OpenAI with the prompt from the database
	prompt := fmt.Sprintf(templateText, topicName, context)

	summary, err := s.openaiClient.GetCompletion(ctx, prompt)
	if err != nil {
		return fmt.Errorf("failed to generate summary: %w", err)
	}

	// Format the final summary message using the title format from the prompts package
	titleFormat := "Сводка чата %s"
	title := fmt.Sprintf(titleFormat, topicName)
	finalSummary := fmt.Sprintf("%s\n\n%s", title, summary)

	// Determine the target topic ID
	var targetTopicID int64 = int64(s.config.SummaryTopicID)
	if sendToDM {
		// If sendToDM is true, try to get the user ID from context
		if userID, ok := ctx.Value("userID").(int64); ok {
			targetTopicID = userID
		} else {
			log.Println("Warning: sendToDM is true but userID not found in context, using SummaryTopicID instead")
		}
	}

	// Send the summary to the target chat
	_, err = s.messageSenderService.SendCopy(
		targetTopicID,
		nil,
		finalSummary,
		[]gotgbot.MessageEntity{
			{
				Type:   "bold",
				Offset: 0,
				Length: int64(len(title)),
			},
		},
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to send summary: %w", err)
	}

	log.Printf("Summary sent successfully")
	return nil
}
