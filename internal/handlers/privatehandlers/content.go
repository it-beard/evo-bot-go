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
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	"github.com/gotd/td/tg"
)

const (
	// Conversation states
	stateProcessQuery = "process_query"

	// Context data keys
	ctxDataKeyQuery = "query"
)

type contentHandler struct {
	openaiClient             *clients.OpenAiClient
	promptingTemplateService *services.PromptingTemplateService
	messageSenderService     services.MessageSenderService
	config                   *config.Config
	userStore                *utils.UserDataStore
}

func NewContentHandler(
	openaiClient *clients.OpenAiClient,
	messageSenderService services.MessageSenderService,
	promptingTemplateService *services.PromptingTemplateService,
	config *config.Config,
) ext.Handler {
	h := &contentHandler{
		openaiClient:             openaiClient,
		promptingTemplateService: promptingTemplateService,
		messageSenderService:     messageSenderService,
		config:                   config,
		userStore:                utils.NewUserDataStore(),
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.ContentCommand, h.startContentSearch),
		},
		map[string][]ext.Handler{
			stateProcessQuery: {
				handlers.NewMessage(message.All, h.processContentSearch),
			},
		},
		&handlers.ConversationOpts{
			Exits: []ext.Handler{handlers.NewCommand(constants.CancelCommand, h.handleCancel)},
		},
	)
}

// 1. startContentSearch is the entry point handler for the content search conversation
func (h *contentHandler) startContentSearch(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Only proceed if this is a private chat
	if !utils.CheckPrivateChatType(b, ctx) {
		return handlers.EndConversation()
	}

	// Check if user is a club member
	if !utils.CheckClubMemberPermissions(b, msg, h.config, constants.ContentCommand) {
		return handlers.EndConversation()
	}

	// Ask user to enter search query
	utils.SendLoggedReply(b, msg, fmt.Sprintf("Введите поисковый запрос по контенту или используйте /%s для отмены:", constants.CancelCommand), nil)

	return handlers.NextConversationState(stateProcessQuery)
}

// 2. processContentSearch handles the actual content search
func (h *contentHandler) processContentSearch(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Get query from user message
	query := strings.TrimSpace(msg.Text)
	if query == "" {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Поисковый запрос не может быть пустым. Пожалуйста, введите запрос или используйте /%s для отмены:", constants.CancelCommand), nil)
		return nil // Stay in the same state
	}

	// Send typing action using MessageSender.
	h.messageSenderService.SendTypingAction(msg.Chat.Id)

	// Get messages from chat
	messages, err := clients.GetChatMessages(h.config.SuperGroupChatID, h.config.ContentTopicID)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Произошла ошибка при получении сообщений из чата.", err)
		return handlers.EndConversation()
	}

	dataMessages, err := h.prepareTelegramMessages(messages)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Произошла ошибка при подготовке сообщений для поиска.", err)
		return handlers.EndConversation()
	}

	topicLink := fmt.Sprintf("https://t.me/c/%d/%d", h.config.SuperGroupChatID, h.config.ContentTopicID)

	// Get the prompt template from the database
	templateText := h.promptingTemplateService.GetTemplateWithFallback(
		context.Background(),
		prompts.GetContentPromptTemplateDbKey,
		prompts.GetContentPromptDefaultTemplate,
	)

	prompt := fmt.Sprintf(
		templateText,
		topicLink,
		topicLink,
		string(dataMessages),
		query)

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

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// 3. handleCancel handles the /cancel command
func (h *contentHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	utils.SendLoggedReply(b, msg, "Операция поиска контента отменена.", nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

func (h *contentHandler) prepareTelegramMessages(messages []tg.Message) ([]byte, error) {
	// Modified MessageObject to have Date as string
	type MessageObject struct {
		ID      int    `json:"id"`
		Message string `json:"message"`
		Date    string `json:"date"` // now formatted as "10 february 2024"
	}

	// Load CET location
	loc, err := time.LoadLocation("CET")
	if err != nil {
		return nil, fmt.Errorf("failed to load CET location: %w", err)
	}

	messageObjects := make([]MessageObject, 0, len(messages))
	for _, message := range messages {
		// Convert Unix timestamp to CET time
		t := time.Unix(int64(message.Date), 0).In(loc)
		// Format date as "day month year" and convert to lowercase
		dateFormatted := strings.ToLower(t.Format("2 January 2006"))

		messageObjects = append(messageObjects, MessageObject{
			ID:      message.ID,
			Message: message.Message,
			Date:    dateFormatted,
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
