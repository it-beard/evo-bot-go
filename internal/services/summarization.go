package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/it-beard/evo-bot-go/internal/clients"
	"github.com/it-beard/evo-bot-go/internal/config"
	"github.com/it-beard/evo-bot-go/internal/handlers/prompts"
	"github.com/it-beard/evo-bot-go/internal/storage"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// SummarizationService handles the daily summarization of messages
type SummarizationService struct {
	config        *config.Config
	messageStore  *storage.MessageStore
	openaiClient  *clients.OpenAiClient
	messageSender MessageSender
}

// NewSummarizationService creates a new summarization service
func NewSummarizationService(
	config *config.Config,
	messageStore *storage.MessageStore,
	openaiClient *clients.OpenAiClient,
	messageSender MessageSender,
) *SummarizationService {
	return &SummarizationService{
		config:        config,
		messageStore:  messageStore,
		openaiClient:  openaiClient,
		messageSender: messageSender,
	}
}

// RunDailySummarization runs the daily summarization process
func (s *SummarizationService) RunDailySummarization(ctx context.Context, sendToDM bool) error {
	log.Println("Starting daily summarization process")

	// Get the time 24 hours ago
	since := time.Now().Add(-24 * time.Hour)

	// Process each monitored chat
	for _, chatID := range s.config.MonitoredChatIDs {
		if err := s.summarizeChat(ctx, chatID, since, sendToDM); err != nil {
			log.Printf("Error summarizing chat %d: %v", chatID, err)
			// Continue with other chats even if one fails
			continue
		}
	}

	log.Println("Daily summarization process completed")
	return nil
}

// summarizeChat summarizes a single chat
func (s *SummarizationService) summarizeChat(ctx context.Context, chatID int64, since time.Time, sendToDM bool) error {
	// Get chat name
	chatName, err := s.messageStore.GetChatName(ctx, chatID)
	if err != nil {
		return fmt.Errorf("failed to get chat name: %w", err)
	}

	// Get recent messages
	messages, err := s.messageStore.GetRecentMessages(ctx, chatID, since)
	if err != nil {
		return fmt.Errorf("failed to get recent messages: %w", err)
	}

	if len(messages) == 0 {
		log.Printf("No messages found for chat %d since %v", chatID, since)
		return nil
	}

	log.Printf("Found %d messages for chat %d", len(messages), chatID)

	// Build context directly from all messages without using RAG
	context := ""
	for _, msg := range messages {
		context += fmt.Sprintf("[%s] %s: %s\n",
			msg.CreatedAt.Format("2006-01-02 15:04:05"),
			msg.Username,
			msg.Text)
	}

	// Generate summary using OpenAI with the prompt from the prompts package
	prompt := fmt.Sprintf(prompts.FairyTaleSummarizationPromptTemplate, chatName, context)

	summary, err := s.openaiClient.GetCompletion(ctx, prompt)
	if err != nil {
		return fmt.Errorf("failed to generate summary: %w", err)
	}

	// Format the final summary message using the title format from the prompts package
	titleFormat := prompts.FairyTaleSummaryTitleFormat
	title := fmt.Sprintf(titleFormat, chatName)
	finalSummary := fmt.Sprintf("%s\n\n%s", title, summary)

	// Determine the target chat ID
	targetChatID := s.config.SummaryChatID
	if sendToDM {
		// If sendToDM is true, try to get the user ID from context
		if userID, ok := ctx.Value("userID").(int64); ok {
			targetChatID = userID
		} else {
			log.Println("Warning: sendToDM is true but userID not found in context, using SummaryChatID instead")
		}
	}

	// Send the summary to the target chat
	_, err = s.messageSender.SendCopy(
		targetChatID,
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
