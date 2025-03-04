package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"your_module_name/internal/clients"
	"your_module_name/internal/config"
	"your_module_name/internal/rag"
	"your_module_name/internal/storage"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// SummarizationService handles the daily summarization of messages
type SummarizationService struct {
	config           *config.Config
	messageStore     *storage.MessageStore
	openaiClient     *clients.OpenAiClient
	embeddingService *rag.EmbeddingService
	retriever        *rag.Retriever
	messageSender    MessageSender
}

// NewSummarizationService creates a new summarization service
func NewSummarizationService(
	config *config.Config,
	messageStore *storage.MessageStore,
	openaiClient *clients.OpenAiClient,
	messageSender MessageSender,
) *SummarizationService {
	embeddingService := rag.NewEmbeddingService(openaiClient)
	retriever := rag.NewRetriever(embeddingService)

	return &SummarizationService{
		config:           config,
		messageStore:     messageStore,
		openaiClient:     openaiClient,
		embeddingService: embeddingService,
		retriever:        retriever,
		messageSender:    messageSender,
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

	// Use RAG to find the most relevant messages
	// For summarization, we'll use a generic query
	query := "What are the main topics and important points discussed in this chat?"
	topK := 20 // Number of most relevant messages to include

	relevantMessages, err := s.retriever.RetrieveRelevantMessages(ctx, messages, query, topK)
	if err != nil {
		return fmt.Errorf("failed to retrieve relevant messages: %w", err)
	}

	// Build context from relevant messages
	context := rag.BuildContextFromMessages(relevantMessages)

	// Generate summary using OpenAI
	prompt := fmt.Sprintf(
		"You are tasked with creating a daily summary of a chat conversation. "+
			"Below are the most relevant messages from the chat '%s' in the last 24 hours.\n\n"+
			"%s\n\n"+
			"Please provide a concise summary of the main topics and important points discussed. "+
			"Organize the summary in a clear and readable format. "+
			"If there are distinct topics, separate them with headings. "+
			"Include any important decisions, questions, or action items that emerged.",
		chatName, context)

	summary, err := s.openaiClient.GetCompletion(ctx, prompt)
	if err != nil {
		return fmt.Errorf("failed to generate summary: %w", err)
	}

	// Format the final summary message
	finalSummary := fmt.Sprintf("📋 *Daily Summary: %s*\n\n%s", chatName, summary)

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
				Length: int64(len(fmt.Sprintf("📋 Daily Summary: %s", chatName))),
			},
		},
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to send summary: %w", err)
	}

	log.Printf("Summary for chat %d sent successfully", chatID)
	return nil
}
