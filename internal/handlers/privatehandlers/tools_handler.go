package privatehandlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"evo-bot-go/internal/buttons"
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
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
)

const (
	// Conversation states names
	toolsStateStartToolSearch = "tools_state_start_tool_search"

	// Context data keys
	toolsUserCtxDataKeyProcessing        = "tools_user_ctx_data_key_processing"
	toolsUserCtxDataKeyCancelFunc        = "tools_user_ctx_data_key_cancel_func"
	toolsUserCtxDataKeyPreviousMessageID = "tools_user_ctx_data_key_previous_message_id"
	toolsUserCtxDataKeyPreviousChatID    = "tools_user_ctx_data_key_previous_chat_id"

	// Callback data
	toolsCallbackConfirmCancel = "tools_callback_confirm_cancel"
)

type toolsHandler struct {
	config                      *config.Config
	openaiClient                *clients.OpenAiClient
	promptingTemplateRepository *repositories.PromptingTemplateRepository
	messageSenderService        *services.MessageSenderService
	userStore                   *utils.UserDataStore
	permissionsService          *services.PermissionsService
}

func NewToolsHandler(
	config *config.Config,
	openaiClient *clients.OpenAiClient,
	messageSenderService *services.MessageSenderService,
	promptingTemplateRepository *repositories.PromptingTemplateRepository,
	permissionsService *services.PermissionsService,
) ext.Handler {
	h := &toolsHandler{
		config:                      config,
		openaiClient:                openaiClient,
		promptingTemplateRepository: promptingTemplateRepository,
		messageSenderService:        messageSenderService,
		userStore:                   utils.NewUserDataStore(),
		permissionsService:          permissionsService,
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.ToolsCommand, h.startToolSearch),
		},
		map[string][]ext.Handler{
			toolsStateStartToolSearch: {
				handlers.NewMessage(message.All, h.processToolSearch),
				handlers.NewCallback(callbackquery.Equal(toolsCallbackConfirmCancel), h.handleCallbackCancel),
			},
		},
		&handlers.ConversationOpts{
			Exits: []ext.Handler{handlers.NewCommand(constants.CancelCommand, h.handleCancel)},
		},
	)
}

// 1. startToolSearch is the entry point handler for the tool search conversation
func (h *toolsHandler) startToolSearch(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Only proceed if this is a private chat
	if !h.permissionsService.CheckPrivateChatType(msg) {
		return handlers.EndConversation()
	}

	// Check if user is a club member
	if !h.permissionsService.CheckClubMemberPermissions(msg, constants.ToolsCommand) {
		return handlers.EndConversation()
	}

	// Ask user to enter search query
	sentMsg, _ := h.messageSenderService.ReplyWithReturnMessage(
		msg,
		"Введи поисковый запрос по инструментам:",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.CancelButton(toolsCallbackConfirmCancel),
		},
	)

	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
	return handlers.NextConversationState(toolsStateStartToolSearch)
}

// 2. processToolSearch handles the actual tool search
func (h *toolsHandler) processToolSearch(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if we're already processing a request for this user
	if isProcessing, ok := h.userStore.Get(ctx.EffectiveUser.Id, toolsUserCtxDataKeyProcessing); ok && isProcessing.(bool) {
		h.messageSenderService.Reply(
			msg,
			fmt.Sprintf("Пожалуйста, дождись окончания обработки предыдущего запроса, или используй /%s для отмены.", constants.CancelCommand),
			nil,
		)
		return nil // Stay in the same state
	}

	// Get query from user message
	query := strings.TrimSpace(msg.Text)
	if query == "" {
		h.messageSenderService.Reply(
			msg,
			fmt.Sprintf("Поисковый запрос не может быть пустым. Пожалуйста, введи запрос или используй /%s для отмены.", constants.CancelCommand),
			nil,
		)
		return nil // Stay in the same state
	}

	// Mark as processing
	h.userStore.Set(ctx.EffectiveUser.Id, toolsUserCtxDataKeyProcessing, true)

	// Create a cancellable context for this operation
	typingCtx, cancelTyping := context.WithCancel(context.Background())

	// Store cancel function in user store so it can be called from handleCancel
	h.userStore.Set(ctx.EffectiveUser.Id, toolsUserCtxDataKeyCancelFunc, cancelTyping)

	// Make sure we clean up the processing flag in all exit paths
	defer func() {
		h.userStore.Set(ctx.EffectiveUser.Id, toolsUserCtxDataKeyProcessing, false)
		h.userStore.Set(ctx.EffectiveUser.Id, toolsUserCtxDataKeyCancelFunc, nil)
	}()

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)
	// Inform user that search has started
	sentMsg, _ := h.messageSenderService.ReplyWithReturnMessage(msg, fmt.Sprintf("Ищу информацию по запросу: \"%s\"...", query), &gotgbot.SendMessageOpts{
		ReplyMarkup: buttons.CancelButton(toolsCallbackConfirmCancel),
	})
	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)

	// Send typing action using MessageSender.
	h.messageSenderService.SendTypingAction(msg.Chat.Id)

	// Get messages from chat
	//[todo] get messages from chat
	messages := []*repositories.GroupMessage{}

	dataMessages, err := h.prepareTelegramMessages(messages)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при подготовке сообщений для поиска.", nil)
		log.Printf("%s: Error during messages preparation: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	topicLink := fmt.Sprintf("https://t.me/c/%d/%d", h.config.SuperGroupChatID, h.config.ToolTopicID)

	templateText, err := h.promptingTemplateRepository.Get(prompts.GetToolPromptTemplateDbKey)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при получении шаблона для поиска инструментов.", nil)
		log.Printf("%s: Error during template retrieval: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	prompt := fmt.Sprintf(
		templateText,
		topicLink,
		topicLink,
		utils.EscapeMarkdown(string(dataMessages)),
		utils.EscapeMarkdown(query),
	)

	// Save the prompt into a temporary file for logging purposes.
	err = os.WriteFile("last-prompt-log.txt", []byte(prompt), 0644)
	if err != nil {
		log.Printf("%s: Error writing prompt to file: %v", utils.GetCurrentTypeName(), err)
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
		log.Printf("%s: Request was cancelled", utils.GetCurrentTypeName())
		return handlers.EndConversation()
	}

	// Continue only if no errors
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при получении ответа от OpenAI.", nil)
		log.Printf("%s: Error during OpenAI response retrieval: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	err = h.messageSenderService.ReplyMarkdown(msg, responseOpenAi, nil)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при отправке ответа.", nil)
		log.Printf("%s: Error during message sending: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)
	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// handleCallbackCancel processes the cancel button click
func (h *toolsHandler) handleCallbackCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	// Answer the callback query to remove the loading state on the button
	cb := ctx.Update.CallbackQuery
	_, _ = cb.Answer(b, nil)

	return h.handleCancel(b, ctx)
}

// handleCancel handles the /cancel command
func (h *toolsHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if there's an ongoing operation to cancel
	if cancelFunc, ok := h.userStore.Get(ctx.EffectiveUser.Id, toolsUserCtxDataKeyCancelFunc); ok {
		// Call the cancel function to stop any ongoing API calls
		if cf, ok := cancelFunc.(context.CancelFunc); ok {
			cf()
			h.messageSenderService.Reply(msg, "Операция поиска инструментов отменена.", nil)
		}
	} else {
		h.messageSenderService.Reply(msg, "Операция поиска инструментов отменена.", nil)
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

func (h *toolsHandler) prepareTelegramMessages(messages []*repositories.GroupMessage) ([]byte, error) {
	type MessageObject struct {
		ID      int    `json:"id"`
		Message string `json:"message"`
	}

	messageObjects := make([]MessageObject, 0, len(messages))
	for _, message := range messages {
		messageObjects = append(messageObjects, MessageObject{
			ID:      message.ID,
			Message: message.MessageText,
		})
	}

	if len(messageObjects) == 0 {
		return nil, fmt.Errorf("%s: no messages found in chat", utils.GetCurrentTypeName())
	}

	dataMessages, err := json.Marshal(messageObjects)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to marshal messages to JSON: %w", utils.GetCurrentTypeName(), err)
	}

	if string(dataMessages) == "" {
		return nil, fmt.Errorf("%s: no messages found in chat", utils.GetCurrentTypeName())
	}

	return dataMessages, nil
}

func (h *toolsHandler) MessageRemoveInlineKeyboard(b *gotgbot.Bot, userID *int64) {
	var chatID, messageID int64

	// If userID provided, get stored message info using the utility method
	if userID != nil {
		messageID, chatID = h.userStore.GetPreviousMessageInfo(
			*userID,
			toolsUserCtxDataKeyPreviousMessageID,
			toolsUserCtxDataKeyPreviousChatID,
		)
	}

	// Skip if we don't have valid chat and message IDs
	if chatID == 0 || messageID == 0 {
		return
	}

	// Use message sender service to remove the inline keyboard
	_ = h.messageSenderService.RemoveInlineKeyboard(chatID, messageID)
}

func (h *toolsHandler) SavePreviousMessageInfo(userID int64, sentMsg *gotgbot.Message) {
	h.userStore.SetPreviousMessageInfo(userID, sentMsg.MessageId, sentMsg.Chat.Id,
		toolsUserCtxDataKeyPreviousMessageID, toolsUserCtxDataKeyPreviousChatID)
}
