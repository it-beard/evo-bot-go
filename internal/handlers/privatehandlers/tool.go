package privatehandlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/it-beard/evo-bot-go/internal/clients"
	"github.com/it-beard/evo-bot-go/internal/config"
	"github.com/it-beard/evo-bot-go/internal/constants"
	"github.com/it-beard/evo-bot-go/internal/handlers"
	"github.com/it-beard/evo-bot-go/internal/handlers/prompts"
	"github.com/it-beard/evo-bot-go/internal/services"
	"github.com/it-beard/evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/gotd/td/tg"
)

type ToolHandler struct {
	openaiClient         *clients.OpenAiClient
	config               *config.Config
	messageSenderService services.MessageSenderService
}

func NewToolHandler(
	openaiClient *clients.OpenAiClient,
	messageSenderService services.MessageSenderService,
	config *config.Config,
) handlers.Handler {
	return &ToolHandler{
		openaiClient:         openaiClient,
		config:               config,
		messageSenderService: messageSenderService,
	}
}

func (h *ToolHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Extract text after command
	commandText := h.extractCommandText(msg)
	if commandText == "" {
		_, err := msg.Reply(b, fmt.Sprintf("Пожалуйста, введи поисковый запрос после команды. Например: %s <текст>", constants.ToolCommand), nil)
		return err
	}

	// Send typing action using MessageSender.
	h.messageSenderService.SendTypingAction(msg.Chat.Id)

	// Get messages from chat
	messages, err := clients.GetChatMessages(h.config.SuperGroupChatID, h.config.ToolTopicID) // Get last 100 messages
	if err != nil {
		return fmt.Errorf("failed to get chat messages: %w", err)
	}

	dataMessages, err := h.prepareTelegramMessages(messages)
	if err != nil {
		return err
	}

	topicLink := fmt.Sprintf("https://t.me/c/%d/%d", h.config.SuperGroupChatID, h.config.ToolTopicID)
	prompt := fmt.Sprintf(
		prompts.GetToolPromptTemplate,
		topicLink,
		topicLink,
		string(dataMessages),
		commandText)

	// Save the prompt into a temporary file for logging purposes.
	err = os.WriteFile("last-prompt-log.txt", []byte(prompt), 0644)
	if err != nil {
		log.Printf("Error writing prompt to file: %v", err)
	}

	// Start periodic typing action every 5 seconds while waiting for the OpenAI response.
	typingCtx, cancelTyping := context.WithCancel(context.Background())
	defer cancelTyping() // ensure cancellation if function exits early

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				h.messageSenderService.SendTypingAction(msg.Chat.Id)
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

func (h *ToolHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	msg := ctx.EffectiveMessage
	if msg == nil {
		return false
	}

	if msg.Text != "" &&
		(strings.HasPrefix(msg.Text, constants.ToolsCommand) || strings.HasPrefix(msg.Text, constants.ToolCommand)) &&
		msg.Chat.Type == constants.PrivateChat {

		if !utils.IsUserClubMember(b, msg, h.config) {
			msg.Reply(b, "Команда доступна только для членов клуба.", nil)
			log.Print("Trying to use /tool command without club membership")
			return false
		}
		return true
	}

	return false
}

func (h *ToolHandler) Name() string {
	return constants.ToolHandlerName
}

func (h *ToolHandler) extractCommandText(msg *gotgbot.Message) string {
	var commandText string
	if strings.HasPrefix(msg.Text, constants.ToolsCommand) {
		commandText = strings.TrimPrefix(msg.Text, constants.ToolsCommand)
	} else {
		commandText = strings.TrimPrefix(msg.Text, constants.ToolCommand)
	}
	return strings.TrimSpace(commandText)
}

func (h *ToolHandler) prepareTelegramMessages(messages []tg.Message) ([]byte, error) {
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

	dataMessages, err := json.Marshal(messageObjects)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal messages to JSON: %w", err)
	}

	if string(dataMessages) == "" {
		return nil, fmt.Errorf("no messages found in chat")
	}

	return dataMessages, nil
}
