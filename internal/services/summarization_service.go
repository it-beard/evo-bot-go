package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
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
	messageSenderService        *MessageSenderService
	promptingTemplateRepository *repositories.PromptingTemplateRepository
}

// NewSummarizationService creates a new summarization service
func NewSummarizationService(
	config *config.Config,
	openaiClient *clients.OpenAiClient,
	messageSenderService *MessageSenderService,
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
	log.Printf("%s: Starting daily summarization process", utils.GetCurrentTypeName())

	// Get the time 24 hours ago
	since := time.Now().Add(-24 * time.Hour)

	// Process each monitored topic
	for _, topicID := range s.config.MonitoredTopicsIDs {
		if err := s.summarizeTopicMessages(ctx, topicID, since, sendToDM); err != nil {
			log.Printf("%s: Error summarizing topic %d: %v", utils.GetCurrentTypeName(), topicID, err)
			// Continue with other chats even if one fails
			continue
		}
	}

	log.Printf("%s: Daily summarization process completed", utils.GetCurrentTypeName())
	return nil
}

// summarizeTopicMessages summarizes a single topic
func (s *SummarizationService) summarizeTopicMessages(ctx context.Context, topicID int, since time.Time, sendToDM bool) error {
	// Get topic name
	topicName, err := utils.GetTopicName(topicID)
	if err != nil {
		return fmt.Errorf("%s: failed to get topic name: %w", utils.GetCurrentTypeName(), err)
	}

	// Calculate hours since the given time
	hoursSince := int(time.Since(since).Hours()) + 1 // Add 1 to ensure we get all messages since 'since' time

	// Get messages directly from Telegram instead of database
	tgMessages, err := clients.GetLastTopicMessagesByTime(s.config.SuperGroupChatID, topicID, hoursSince)
	if err != nil {
		return fmt.Errorf("%s: failed to get messages from Telegram: %w", utils.GetCurrentTypeName(), err)
	}

	if len(tgMessages) == 0 {
		log.Printf("%s: No messages found for topic %d since %v", utils.GetCurrentTypeName(), topicID, since)
		return nil
	}

	log.Printf("%s: Found %d messages for topic %d", utils.GetCurrentTypeName(), len(tgMessages), topicID)

	// Build context directly from all messages without using RAG
	context := ""
	for _, msg := range tgMessages {
		// Convert Unix timestamp to time.Time
		msgTime := time.Unix(int64(msg.Date), 0)

		// Extract username/first name from the message
		userID := int64(0)
		if msg.FromID != nil {
			if user, ok := msg.FromID.(*tg.PeerUser); ok && user != nil {
				// Just use the user ID as a placeholder since we don't have easy access to username
				userID = user.UserID
			}
		}

		replyToMessageId := 0
		if msg.ReplyTo != nil {
			reply, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
			if ok {
				if reply.ReplyToTopID == 0 && topicID != 0 { // Hack for non main topic messages (id = 0)
					replyToMessageId = 0
				} else {
					replyToMessageId = reply.ReplyToMsgID
				}
			}
		}

		replyToMessage := ""

		if replyToMessageId != 0 {
			replyToMessage = fmt.Sprintf("ReplyID: %d\n", replyToMessageId)
		}
		context += fmt.Sprintf("\n---\nMessageID: %d\n%sUserID: user_%d\nTimestamp: %s\nText: %s",
			msg.ID,
			replyToMessage,
			userID,
			msgTime.Format("2006-01-02 15:04:05"),
			msg.Message)

	}

	// Get the prompt template from the database with fallback to default
	templateText, err := s.promptingTemplateRepository.Get(prompts.DailySummarizationPromptTemplateDbKey)
	if err != nil {
		return fmt.Errorf("%s: failed to get prompt template: %w", utils.GetCurrentTypeName(), err)
	}

	dateNow := time.Now().Format("02.01.2006")
	superGroupChatIDStr := strconv.Itoa(int(s.config.SuperGroupChatID))
	topicIDStr := strconv.Itoa(topicID)
	if topicID == 0 {
		topicIDStr = "1" // Hack for non main topic (id = 0)
	}
	// Generate summary using OpenAI with the prompt from the database
	prompt := fmt.Sprintf(
		templateText,
		dateNow,
		superGroupChatIDStr,
		topicIDStr,
		superGroupChatIDStr,
		topicIDStr,
		superGroupChatIDStr,
		topicIDStr,
		superGroupChatIDStr,
		topicIDStr,
		dateNow,
		context,
	)

	// Save the prompt into a temporary file for logging purposes.
	err = os.WriteFile("last-prompt-log.txt", []byte(prompt), 0644)
	if err != nil {
		log.Printf("%s: Error writing prompt to file: %v", utils.GetCurrentTypeName(), err)
	}

	summary, err := s.openaiClient.GetCompletion(ctx, prompt)
	if err != nil {
		return fmt.Errorf("Summarization Service: failed to generate summary: %w", err)
	}

	// Format the final summary message using the title format from the prompts package
	dateNowWithMonth := time.Now().Format("01.02.2006")
	title := fmt.Sprintf("📋 Сводка чата <b>\"%s\"</b> за %s", topicName, dateNowWithMonth)
	finalSummary := fmt.Sprintf("%s\n\n%s", title, summary)

	// Determine the target topic ID
	var targetChatID int64 = int64(s.config.SuperGroupChatID)
	var opts *gotgbot.SendMessageOpts = &gotgbot.SendMessageOpts{
		MessageThreadId: int64(s.config.SummaryTopicID),
	}
	if sendToDM {
		// If sendToDM is true, try to get the user ID from context
		if userID, ok := ctx.Value("userID").(int64); ok {
			targetChatID = userID
			opts = nil
		} else {
			log.Printf("%s: Warning: sendToDM is true but userID not found in context, using SummaryTopicID instead", utils.GetCurrentTypeName())
		}
	}

	// Send the summary to the target chat
	s.messageSenderService.SendHtml(targetChatID, finalSummary, opts)

	log.Printf("%s: Summary sent successfully", utils.GetCurrentTypeName())
	return nil
}
