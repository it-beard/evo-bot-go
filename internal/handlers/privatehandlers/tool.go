package privatehandlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"evo-bot-go/internal/clients"
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/constants/prompts"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/gotd/td/tg"
)

type toolHandler struct {
	openaiClient             *clients.OpenAiClient
	config                   *config.Config
	promptingTemplateService *services.PromptingTemplateService
	messageSenderService     services.MessageSenderService
}

func NewToolHandler(
	openaiClient *clients.OpenAiClient,
	messageSenderService services.MessageSenderService,
	promptingTemplateService *services.PromptingTemplateService,
	config *config.Config,
) ext.Handler {
	h := &toolHandler{
		openaiClient:             openaiClient,
		config:                   config,
		promptingTemplateService: promptingTemplateService,
		messageSenderService:     messageSenderService,
	}

	return handlers.NewCommand(constants.ToolCommand, h.handleCommand)
}

func (h *toolHandler) handleCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Extract text after command
	commandText := h.extractCommandText(msg)
	if commandText == "" {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Пожалуйста, введи поисковый запрос после команды. Например: /%s <текст>", constants.ToolCommand), nil)
		return handlers.EndConversation()
	}

	// Only proceed if this is a private chat
	if !utils.CheckPrivateChatType(b, ctx) {
		return handlers.EndConversation()
	}

	// Check if user is a club member
	if !utils.IsUserClubMember(b, msg, h.config) {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Команда /%s доступна только для членов клуба.", constants.ToolCommand), nil)
		return handlers.EndConversation()
	}

	// Send typing action using MessageSender.
	h.messageSenderService.SendTypingAction(msg.Chat.Id)

	// Get messages from chat
	messages, err := clients.GetChatMessages(h.config.SuperGroupChatID, h.config.ToolTopicID)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Произошла ошибка при получении сообщений из чата.", err)
		return handlers.EndConversation()
	}

	dataMessages, err := h.prepareTelegramMessages(messages)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Произошла ошибка при подготовке сообщений для поиска.", err)
		return handlers.EndConversation()
	}

	topicLink := fmt.Sprintf("https://t.me/c/%d/%d", h.config.SuperGroupChatID, h.config.ToolTopicID)

	templateText := h.promptingTemplateService.GetTemplateWithFallback(
		context.Background(),
		prompts.GetToolPromptTemplateDbKey,
		prompts.GetToolPromptDefaultTemplate,
	)

	prompt := fmt.Sprintf(
		templateText,
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
		utils.SendLoggedReply(b, msg, "Произошла ошибка при получении ответа от OpenAI.", err)
		return handlers.EndConversation()
	}

	utils.SendLoggedMarkdownReply(b, msg, responseOpenAi, nil)
	return handlers.EndConversation()
}

func (h *toolHandler) extractCommandText(msg *gotgbot.Message) string {
	var commandText string
	if strings.HasPrefix(msg.Text, constants.ToolsCommand) {
		commandText = strings.TrimPrefix(msg.Text, constants.ToolsCommand)
	} else if strings.HasPrefix(msg.Text, constants.ToolCommand) {
		commandText = strings.TrimPrefix(msg.Text, constants.ToolCommand)
	}
	return strings.TrimSpace(commandText)
}

func (h *toolHandler) prepareTelegramMessages(messages []tg.Message) ([]byte, error) {
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
