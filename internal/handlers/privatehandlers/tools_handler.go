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
	"github.com/openai/openai-go/v2"
)

const (
	// Conversation states names
	toolsStateStartToolSearch   = "tools_state_start_tool_search"
	toolsStateSelectSearchType  = "tools_state_select_search_type"
	toolsStateProcessToolSearch = "tools_state_process_tool_search"

	// Context data keys
	toolsUserCtxDataKeyProcessing        = "tools_user_ctx_data_key_processing"
	toolsUserCtxDataKeyCancelFunc        = "tools_user_ctx_data_key_cancel_func"
	toolsUserCtxDataKeyPreviousMessageID = "tools_user_ctx_data_key_previous_message_id"
	toolsUserCtxDataKeyPreviousChatID    = "tools_user_ctx_data_key_previous_chat_id"
	toolsUserCtxDataKeySearchQuery       = "tools_user_ctx_data_key_search_query"
	toolsUserCtxDataKeySearchType        = "tools_user_ctx_data_key_search_type"

	// Callback data
	toolsCallbackConfirmCancel = "tools_callback_confirm_cancel"
	toolsCallbackFastSearch    = "tools_callback_fast_search"
	toolsCallbackDeepSearch    = "tools_callback_deep_search"
)

type toolsHandler struct {
	config                      *config.Config
	openaiClient                *clients.OpenAiClient
	promptingTemplateRepository *repositories.PromptingTemplateRepository
	groupMessageRepository      *repositories.GroupMessageRepository
	groupTopicRepository        *repositories.GroupTopicRepository
	messageSenderService        *services.MessageSenderService
	userStore                   *utils.UserDataStore
	permissionsService          *services.PermissionsService
}

func NewToolsHandler(
	config *config.Config,
	openaiClient *clients.OpenAiClient,
	messageSenderService *services.MessageSenderService,
	promptingTemplateRepository *repositories.PromptingTemplateRepository,
	groupMessageRepository *repositories.GroupMessageRepository,
	groupTopicRepository *repositories.GroupTopicRepository,
	permissionsService *services.PermissionsService,
) ext.Handler {
	h := &toolsHandler{
		config:                      config,
		openaiClient:                openaiClient,
		promptingTemplateRepository: promptingTemplateRepository,
		groupMessageRepository:      groupMessageRepository,
		messageSenderService:        messageSenderService,
		groupTopicRepository:        groupTopicRepository,
		userStore:                   utils.NewUserDataStore(),
		permissionsService:          permissionsService,
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.ToolsCommand, h.startToolSearch),
		},
		map[string][]ext.Handler{
			toolsStateStartToolSearch: {
				handlers.NewMessage(message.All, h.selectSearchType),
				handlers.NewCallback(callbackquery.Equal(toolsCallbackConfirmCancel), h.handleCallbackCancel),
			},
			toolsStateSelectSearchType: {
				handlers.NewCallback(callbackquery.Equal(toolsCallbackFastSearch), h.handleFastSearchSelection),
				handlers.NewCallback(callbackquery.Equal(toolsCallbackDeepSearch), h.handleDeepSearchSelection),
				handlers.NewCallback(callbackquery.Equal(toolsCallbackConfirmCancel), h.handleCallbackCancel),
				handlers.NewMessage(message.All, h.processToolSearchWithType),
			},
			toolsStateProcessToolSearch: {
				handlers.NewMessage(message.All, h.processToolSearchWithType),
				handlers.NewCallback(callbackquery.Equal(toolsCallbackConfirmCancel), h.handleCallbackCancel),
			},
		},
		&handlers.ConversationOpts{
			Exits: []ext.Handler{
				handlers.NewCommand(constants.CancelCommand, h.handleCancel),
				handlers.NewCallback(callbackquery.Equal(toolsCallbackConfirmCancel), h.handleCallbackCancel),
			},
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
	sentMsg, _ := h.messageSenderService.SendWithReturnMessage(
		msg.Chat.Id,
		"Пришли мне поисковый запрос по инструментам:",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.CancelButton(toolsCallbackConfirmCancel),
		},
	)

	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
	return handlers.NextConversationState(toolsStateStartToolSearch)
}

// 2. selectSearchType handles user input and shows search type selection
func (h *toolsHandler) selectSearchType(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Get query from user message
	query := strings.TrimSpace(msg.Text)
	if query == "" {
		h.messageSenderService.Send(
			msg.Chat.Id,
			fmt.Sprintf("Поисковый запрос не может быть пустым. Пожалуйста, введи запрос или используй /%s для отмены.",
				constants.CancelCommand),
			nil,
		)
		return nil // Stay in the same state
	}

	// Store the search query for later use
	h.userStore.Set(ctx.EffectiveUser.Id, toolsUserCtxDataKeySearchQuery, query)

	// Remove search query message
	msg.Delete(b, nil)
	// Delete previous message before showing new one
	h.RemovePreviousMessage(b, &ctx.EffectiveUser.Id)

	// Show search type selection
	sentMsg, _ := h.messageSenderService.SendWithReturnMessage(
		msg.Chat.Id,
		fmt.Sprintf("Запрос: \"%s\"\n\nВыбери тип поиска:", query),
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.SearchTypeSelectionButton(
				toolsCallbackFastSearch,
				toolsCallbackDeepSearch,
				toolsCallbackConfirmCancel,
			),
		},
	)

	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
	return handlers.NextConversationState(toolsStateSelectSearchType)
}

// 2.1 handleFastSearchSelection handles fast search type selection
func (h *toolsHandler) handleFastSearchSelection(b *gotgbot.Bot, ctx *ext.Context) error {
	// Answer the callback query
	cb := ctx.Update.CallbackQuery
	_, _ = cb.Answer(b, nil)

	// Store search type
	h.userStore.Set(ctx.EffectiveUser.Id, toolsUserCtxDataKeySearchType, constants.SearchTypeFast)

	// Proceed to processing
	return h.processToolSearchWithType(b, ctx)
}

// 2.2handleDeepSearchSelection handles deep search type selection
func (h *toolsHandler) handleDeepSearchSelection(b *gotgbot.Bot, ctx *ext.Context) error {
	// Answer the callback query
	cb := ctx.Update.CallbackQuery
	_, _ = cb.Answer(b, nil)

	// Store search type
	h.userStore.Set(ctx.EffectiveUser.Id, toolsUserCtxDataKeySearchType, constants.SearchTypeDeep)
	// Proceed to processing
	return h.processToolSearchWithType(b, ctx)
}

// 3. processToolSearchWithType processes the search with the selected type
func (h *toolsHandler) processToolSearchWithType(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userId := ctx.EffectiveUser.Id

	// Check if we're already processing a request for this user
	if isProcessing, ok := h.userStore.Get(userId, toolsUserCtxDataKeyProcessing); ok && isProcessing.(bool) {
		h.RemovePreviousMessage(b, &userId)
		msg.Delete(b, nil)
		warningMsg, _ := h.messageSenderService.SendWithReturnMessage(
			msg.Chat.Id,
			fmt.Sprintf("Пожалуйста, дождись окончания обработки предыдущего запроса, или используй /%s для отмены.",
				constants.CancelCommand),
			&gotgbot.SendMessageOpts{
				ReplyMarkup: buttons.CancelButton(toolsCallbackConfirmCancel),
			},
		)
		h.SavePreviousMessageInfo(userId, warningMsg)
		return nil // Stay in the same state
	}

	// Get stored query and search type
	queryInterface, _ := h.userStore.Get(userId, toolsUserCtxDataKeySearchQuery)
	query, _ := queryInterface.(string)
	searchTypeInterface, hasSearchType := h.userStore.Get(userId, toolsUserCtxDataKeySearchType)
	searchType, okType := searchTypeInterface.(string)

	// If search type isn't chosen yet, don't start processing; prompt the user
	if !hasSearchType || !okType || strings.TrimSpace(searchType) == "" {
		// If we're mid-processing, the early return above would have handled it; here we just remind user to choose type
		h.messageSenderService.Send(msg.Chat.Id, "Сначала выбери тип поиска кнопками выше!", nil)
		return nil
	}

	// Mark as processing
	h.userStore.Set(userId, toolsUserCtxDataKeyProcessing, true)

	// Create a cancellable context for this operation
	typingCtx, cancelTyping := context.WithCancel(context.Background())

	// Store cancel function in user store so it can be called from handleCancel
	h.userStore.Set(userId, toolsUserCtxDataKeyCancelFunc, cancelTyping)

	// Make sure we clean up the processing flag in all exit paths
	defer func() {
		h.userStore.Set(userId, toolsUserCtxDataKeyProcessing, false)
		h.userStore.Set(userId, toolsUserCtxDataKeyCancelFunc, nil)
	}()

	h.RemovePreviousMessage(b, &userId)

	// Inform user that search has started with search type info
	searchTypeText := "быстрый"
	if searchType == constants.SearchTypeDeep {
		searchTypeText = "глубокий"
	}

	sentMsg, _ := h.messageSenderService.SendWithReturnMessage(
		msg.Chat.Id,
		fmt.Sprintf("Ищу информацию по запросу: \"%s\" (%s поиск)...", query, searchTypeText),
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.CancelButton(toolsCallbackConfirmCancel),
		},
	)

	h.SavePreviousMessageInfo(userId, sentMsg)

	// Send typing action using MessageSender.
	h.messageSenderService.SendTypingAction(msg.Chat.Id)

	// Get messages from chat
	messages, err := h.groupMessageRepository.GetAllByGroupTopicID(int64(h.config.ToolTopicID))
	if err != nil {
		h.messageSenderService.Send(msg.Chat.Id, "Произошла ошибка при получении сообщений из базы данных.", nil)
		log.Printf("%s: Error during message retrieval: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	dataMessages, err := h.preprocessingMessages(messages)
	if err != nil {
		h.messageSenderService.Send(msg.Chat.Id, "Произошла ошибка при подготовке сообщений для поиска.", nil)
		log.Printf("%s: Error during messages preparation: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	topicLink := fmt.Sprintf("https://t.me/c/%d/%d", h.config.SuperGroupChatID, h.config.ToolTopicID)
	topicName := "Инструменты"
	topic, err := h.groupTopicRepository.GetGroupTopicByTopicID(int64(h.config.ToolTopicID))
	if err != nil {
		log.Printf("%s: Error during topic information retrieval: %v", utils.GetCurrentTypeName(), err)
	} else {
		topicName = topic.Name
	}

	templateText, err := h.promptingTemplateRepository.Get(prompts.GetToolPromptKey, prompts.GetToolPromptDefaultValue)
	if err != nil {
		h.messageSenderService.Send(msg.Chat.Id, "Произошла ошибка при получении шаблона для поиска инструментов.", nil)
		log.Printf("%s: Error during template retrieval: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	prompt := fmt.Sprintf(
		templateText,
		topicLink,
		topicName,
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

	// Get completion from OpenAI using the new context with specified reasoning effort
	var responseOpenAi string
	if searchType == constants.SearchTypeFast {
		responseOpenAi, err = h.openaiClient.GetCompletionWithReasoning(typingCtx, prompt, openai.ReasoningEffortMinimal)
	} else {
		responseOpenAi, err = h.openaiClient.GetCompletionWithReasoning(typingCtx, prompt, openai.ReasoningEffortMedium)
	}

	// Check if context was cancelled
	if typingCtx.Err() != nil {
		log.Printf("%s: Request was cancelled", utils.GetCurrentTypeName())
		return handlers.EndConversation()
	}

	// Continue only if no errors
	if err != nil {
		h.messageSenderService.Send(msg.Chat.Id, "Произошла ошибка при получении ответа от OpenAI.", nil)
		log.Printf("%s: Error during OpenAI response retrieval: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	err = h.messageSenderService.SendHtml(msg.Chat.Id, responseOpenAi, nil)
	if err != nil {
		h.messageSenderService.Send(msg.Chat.Id, "Произошла ошибка при отправке ответа.", nil)
		log.Printf("%s: Error during message sending: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	} else {
		h.RemovePreviousMessage(b, &userId)
		h.userStore.Clear(userId)
	}

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
			h.messageSenderService.Send(msg.Chat.Id, "Операция поиска инструментов отменена.", nil)
		}
	} else {
		h.messageSenderService.Send(msg.Chat.Id, "Операция поиска инструментов отменена.", nil)
	}

	h.RemovePreviousMessage(b, &ctx.EffectiveUser.Id)
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

func (h *toolsHandler) preprocessingMessages(messages []*repositories.GroupMessage) ([]byte, error) {
	// MessageObject represents the format for tool search processing
	type MessageObject struct {
		MessageID int64  `json:"message_id"` // Telegram message ID
		Message   string `json:"message"`    // Message content (HTML formatted)
	}

	if len(messages) == 0 {
		return nil, fmt.Errorf("%s: no messages found for processing", utils.GetCurrentTypeName())
	}

	messageObjects := make([]MessageObject, 0, len(messages))
	for _, message := range messages {
		// Skip empty messages
		if strings.TrimSpace(message.MessageText) == "" {
			continue
		}

		// Clean message text by removing copyright string
		cleanedMessage := strings.ReplaceAll(message.MessageText, constants.CopyrightString, "")
		cleanedMessage = strings.TrimSpace(cleanedMessage)

		// Skip if message becomes empty after cleaning
		if cleanedMessage == "" {
			continue
		}

		messageObjects = append(messageObjects, MessageObject{
			MessageID: message.MessageID,
			Message:   cleanedMessage, // Use cleaned message text
		})
	}

	if len(messageObjects) == 0 {
		return nil, fmt.Errorf("%s: no valid messages found after filtering", utils.GetCurrentTypeName())
	}

	// Marshal to JSON for AI processing
	dataMessages, err := json.Marshal(messageObjects)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to marshal messages to JSON: %w", utils.GetCurrentTypeName(), err)
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

func (h *toolsHandler) RemovePreviousMessage(b *gotgbot.Bot, userID *int64) {
	var chatID, messageID int64

	if userID != nil {
		messageID, chatID = h.userStore.GetPreviousMessageInfo(
			*userID,
			toolsUserCtxDataKeyPreviousMessageID,
			toolsUserCtxDataKeyPreviousChatID,
		)
	}

	if chatID == 0 || messageID == 0 {
		return
	}

	b.DeleteMessage(chatID, messageID, nil)
}

func (h *toolsHandler) SavePreviousMessageInfo(userID int64, sentMsg *gotgbot.Message) {
	h.userStore.SetPreviousMessageInfo(userID, sentMsg.MessageId, sentMsg.Chat.Id,
		toolsUserCtxDataKeyPreviousMessageID, toolsUserCtxDataKeyPreviousChatID)
}
