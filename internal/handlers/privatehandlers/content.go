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
	"evo-bot-go/internal/database/prompts"
	"evo-bot-go/internal/database/repositories"
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

	// UserStore keys
	contentUserStoreKeyProcessing = "content_is_processing"
	contentUserStoreKeyCancelFunc = "content_cancel_func"
)

type contentHandler struct {
	config                      *config.Config
	openaiClient                *clients.OpenAiClient
	promptingTemplateRepository *repositories.PromptingTemplateRepository
	messageSenderService        services.MessageSenderService
	userStore                   *utils.UserDataStore
}

func NewContentHandler(
	config *config.Config,
	openaiClient *clients.OpenAiClient,
	messageSenderService services.MessageSenderService,
	promptingTemplateRepository *repositories.PromptingTemplateRepository,
) ext.Handler {
	h := &contentHandler{
		config:                      config,
		openaiClient:                openaiClient,
		promptingTemplateRepository: promptingTemplateRepository,
		messageSenderService:        messageSenderService,
		userStore:                   utils.NewUserDataStore(),
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
	h.messageSenderService.Reply(
		b,
		msg,
		fmt.Sprintf("Введите поисковый запрос по контенту или используйте /%s для отмены:", constants.CancelCommand),
		nil,
	)

	return handlers.NextConversationState(stateProcessQuery)
}

// 2. processContentSearch handles the actual content search
func (h *contentHandler) processContentSearch(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if we're already processing a request for this user
	if isProcessing, ok := h.userStore.Get(ctx.EffectiveUser.Id, contentUserStoreKeyProcessing); ok && isProcessing.(bool) {
		h.messageSenderService.Reply(
			b,
			msg,
			fmt.Sprintf("Пожалуйста, дождитесь окончания обработки предыдущего запроса, или используйте /%s для отмены:", constants.CancelCommand),
			nil,
		)
		return nil // Stay in the same state
	}

	// Get query from user message
	query := strings.TrimSpace(msg.Text)
	if query == "" {
		h.messageSenderService.Reply(
			b,
			msg,
			fmt.Sprintf("Поисковый запрос не может быть пустым. Пожалуйста, введите запрос или используйте /%s для отмены:", constants.CancelCommand),
			nil,
		)
		return nil // Stay in the same state
	}

	// Mark as processing
	h.userStore.Set(ctx.EffectiveUser.Id, contentUserStoreKeyProcessing, true)

	// Create a cancellable context for this operation
	typingCtx, cancelTyping := context.WithCancel(context.Background())

	// Store cancel function in user store so it can be called from handleCancel
	h.userStore.Set(ctx.EffectiveUser.Id, contentUserStoreKeyCancelFunc, cancelTyping)

	// Make sure we clean up the processing flag in all exit paths
	defer func() {
		h.userStore.Set(ctx.EffectiveUser.Id, contentUserStoreKeyProcessing, false)
		h.userStore.Set(ctx.EffectiveUser.Id, contentUserStoreKeyCancelFunc, nil)
	}()

	// Inform user that search has started
	h.messageSenderService.Reply(b, msg, fmt.Sprintf("Ищу информацию по запросу: \"%s\"...", query), nil)

	// Send typing action using MessageSender.
	h.messageSenderService.SendTypingAction(msg.Chat.Id)

	// Get messages from chat
	messages, err := clients.GetChatMessages(h.config.SuperGroupChatID, h.config.ContentTopicID)
	if err != nil {
		h.messageSenderService.Reply(b, msg, "Произошла ошибка при получении сообщений из чата.", nil)
		log.Printf("ContentHandler: Error during messages retrieval: %v", err)
		return handlers.EndConversation()
	}

	dataMessages, err := h.prepareTelegramMessages(messages)
	if err != nil {
		h.messageSenderService.Reply(b, msg, "Произошла ошибка при подготовке сообщений для поиска.", nil)
		log.Printf("ContentHandler: Error during messages preparation: %v", err)
		return handlers.EndConversation()
	}

	topicLink := fmt.Sprintf("https://t.me/c/%d/%d", h.config.SuperGroupChatID, h.config.ContentTopicID)

	// Get the prompt template from the database
	templateText, err := h.promptingTemplateRepository.Get(prompts.GetContentPromptTemplateDbKey)
	if err != nil {
		h.messageSenderService.Reply(b, msg, "Произошла ошибка при получении шаблона для поиска контента.", nil)
		log.Printf("ContentHandler: Error during template retrieval: %v", err)
		return handlers.EndConversation()
	}

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
	// Check if context was cancelled
	if typingCtx.Err() != nil {
		log.Printf("Request was cancelled")
		return handlers.EndConversation()
	}

	// Continue only if no errors
	if err != nil {
		h.messageSenderService.Reply(b, msg, "Произошла ошибка при получении ответа от OpenAI.", nil)
		log.Printf("ContentHandler: Error during OpenAI response retrieval: %v", err)
		return handlers.EndConversation()
	}

	h.messageSenderService.ReplyMarkdown(b, msg, responseOpenAi, nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// 3. handleCancel handles the /cancel command
func (h *contentHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if there's an ongoing operation to cancel
	if cancelFunc, ok := h.userStore.Get(ctx.EffectiveUser.Id, contentUserStoreKeyCancelFunc); ok {
		// Call the cancel function to stop any ongoing API calls
		if cf, ok := cancelFunc.(context.CancelFunc); ok {
			cf()
			h.messageSenderService.Reply(b, msg, "Операция поиска контента отменена.", nil)
		}
	} else {
		h.messageSenderService.Reply(b, msg, "Операция поиска контента отменена.", nil)
	}

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
