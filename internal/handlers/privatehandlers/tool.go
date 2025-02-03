package privatehandlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"your_module_name/internal/clients"
	"your_module_name/internal/handlers"
	"your_module_name/internal/handlers/prompts"
	"your_module_name/internal/services"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/gotd/td/tg"
)

type ToolHandler struct {
	openaiClient *clients.OpenAiClient
	chatId       int64 // Add field for chat ID
	topicId      int   // Add field for topic ID
}

func NewToolHandler(openaiClient *clients.OpenAiClient) handlers.Handler {
	// Parse environment variables
	chatIdStr := os.Getenv("TG_EVO_BOT_MAIN_CHAT_ID")
	topicIdStr := os.Getenv("TG_EVO_BOT_TOOL_TOPIC_ID")

	chatId, err := strconv.ParseInt(chatIdStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid TG_EVO_BOT_TOOL_CHAT_ID: %v", err)
	}

	topicId, err := strconv.Atoi(topicIdStr)
	if err != nil {
		log.Fatalf("Invalid TG_EVO_BOT_TOOL_TOPIC_ID: %v", err)
	}

	return &ToolHandler{
		openaiClient: openaiClient,
		chatId:       chatId,
		topicId:      topicId,
	}
}

func (h *ToolHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Extract text after command
	commandText := h.extractCommandText(msg)
	if commandText == "" {
		_, err := msg.Reply(b, fmt.Sprintf("Пожалуйста, введи текст после команды. Пример: %s <текст>", toolCommand), nil)
		return err
	}

	// Send typing action using MessageSender.
	sender := services.NewMessageSender(b)
	sender.SendTypingAction(msg.Chat.Id)

	// Get messages from chat
	messages, err := clients.GetChatMessagesNew(h.chatId, h.topicId) // Get last 100 messages
	if err != nil {
		return fmt.Errorf("failed to get chat messages: %w", err)
	}

	db, err := h.prepareMessagesDB(messages)
	if err != nil {
		return err
	}

	prompt := fmt.Sprintf(prompts.GetToolPromptTemplate, string(db), commandText, h.chatId, h.topicId, h.chatId, h.topicId)

	// Start periodic typing action every 5 seconds while waiting for the OpenAI response.
	typingCtx, cancelTyping := context.WithCancel(context.Background())
	defer cancelTyping() // ensure cancellation if function exits early

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				sender.SendTypingAction(msg.Chat.Id)
			case <-typingCtx.Done():
				return
			}
		}
	}()

	// Get completion from OpenAI using the new context.
	responseOpenAi, err := h.openaiClient.GetCompletion(typingCtx, prompt)
	// Cancel the periodic typing action immediately after getting the response.
	cancelTyping()
	if err != nil {
		return fmt.Errorf("failed to get OpenAI completion: %w", err)
	}

	_, err = msg.Reply(b, responseOpenAi, &gotgbot.SendMessageOpts{
		ParseMode: "Markdown",
	})
	return err
}

func (h *ToolHandler) extractCommandText(msg *gotgbot.Message) string {
	var commandText string
	if strings.HasPrefix(msg.Text, toolsCommand) {
		commandText = strings.TrimPrefix(msg.Text, toolsCommand)
	} else {
		commandText = strings.TrimPrefix(msg.Text, toolCommand)
	}
	return strings.TrimSpace(commandText)
}

func (h *ToolHandler) prepareMessagesDB(messages []tg.Message) ([]byte, error) {
	type MessageObject struct {
		ID      int    `json:"id"`
		Message string `json:"message"`
	}

	messageObjects := make([]MessageObject, 0, len(messages))
	for _, message := range messages {
		messageObjects = append(messageObjects, MessageObject{
			ID:      message.ID,
			Message: message.Message,
		})
	}

	if len(messageObjects) == 0 {
		return nil, fmt.Errorf("no messages found in chat")
	}

	db, err := json.Marshal(messageObjects)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal messages to JSON: %w", err)
	}

	if string(db) == "" {
		return nil, fmt.Errorf("no messages found in chat")
	}

	return db, nil
}

func (h *ToolHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	msg := ctx.EffectiveMessage
	if msg == nil {
		return false
	}

	if msg.Text != "" &&
		(strings.HasPrefix(msg.Text, toolsCommand) || strings.HasPrefix(msg.Text, toolCommand)) &&
		msg.Chat.Type == privateChat {

		if !h.isUserClubMember(b, msg) {
			msg.Reply(b, "Команда доступна только для членов клуба.", nil)
			log.Print("Trying to use /tool command without club membership")
			return false
		}
		return true
	}

	return false
}

func (h *ToolHandler) Name() string {
	return toolHandlerName
}

func (h *ToolHandler) isUserClubMember(b *gotgbot.Bot, msg *gotgbot.Message) bool {
	chatId, err := strconv.ParseInt("-100"+strconv.FormatInt(h.chatId, 10), 10, 64)
	if err != nil {
		log.Printf("Failed to parse chat ID: %v", err)
		return false
	}
	// Check if user is member of target group
	chatMember, err := b.GetChatMember(chatId, msg.From.Id, nil)
	if err != nil {
		log.Printf("Failed to get chat member: %v", err)
		return false
	}

	status := chatMember.GetStatus()
	if status == "left" || status == "kicked" {
		return false
	}
	return true
}
