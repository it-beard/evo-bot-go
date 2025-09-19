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
	contentStateStartSearch   = "content_state_start_search"
	contentStateSelectSearch  = "content_state_select_search"
	contentStateProcessSearch = "content_state_process_search"

	// UserStore keys
	contentCtxDataKeyProcessing        = "content_ctx_data_processing"
	contentCtxDataKeyCancelFunc        = "content_ctx_data_cancel_func"
	contentCtxDataKeyPreviousMessageID = "content_ctx_data_previous_message_id"
	contentCtxDataKeyPreviousChatID    = "content_ctx_data_previous_chat_id"
	contentCtxDataKeySearchQuery       = "content_ctx_data_search_query"
	contentCtxDataKeySearchType        = "content_ctx_data_search_type"

	// Callback data
	contentCallbackConfirmCancel = "content_callback_confirm_cancel"
	contentCallbackFastSearch    = "content_callback_fast_search"
	contentCallbackDeepSearch    = "content_callback_deep_search"
)

type contentHandler struct {
	config                      *config.Config
	openaiClient                *clients.OpenAiClient
	promptingTemplateRepository *repositories.PromptingTemplateRepository
	groupMessageRepository      *repositories.GroupMessageRepository
	messageSenderService        *services.MessageSenderService
	userStore                   *utils.UserDataStore
	permissionsService          *services.PermissionsService
}

func NewContentHandler(
	config *config.Config,
	openaiClient *clients.OpenAiClient,
	messageSenderService *services.MessageSenderService,
	promptingTemplateRepository *repositories.PromptingTemplateRepository,
	groupMessageRepository *repositories.GroupMessageRepository,
	permissionsService *services.PermissionsService,
) ext.Handler {
	h := &contentHandler{
		config:                      config,
		openaiClient:                openaiClient,
		promptingTemplateRepository: promptingTemplateRepository,
		groupMessageRepository:      groupMessageRepository,
		messageSenderService:        messageSenderService,
		userStore:                   utils.NewUserDataStore(),
		permissionsService:          permissionsService,
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.ContentCommand, h.startContentSearch),
		},
		map[string][]ext.Handler{
			contentStateStartSearch: {
				handlers.NewMessage(message.All, h.selectSearchType),
				handlers.NewCallback(callbackquery.Equal(contentCallbackConfirmCancel), h.handleCallbackCancel),
			},
			contentStateSelectSearch: {
				handlers.NewCallback(callbackquery.Equal(contentCallbackFastSearch), h.handleFastSearchSelection),
				handlers.NewCallback(callbackquery.Equal(contentCallbackDeepSearch), h.handleDeepSearchSelection),
				handlers.NewCallback(callbackquery.Equal(contentCallbackConfirmCancel), h.handleCallbackCancel),
				handlers.NewMessage(message.All, h.processContentSearchWithType),
			},
			contentStateProcessSearch: {
				handlers.NewMessage(message.All, h.processContentSearchWithType),
				handlers.NewCallback(callbackquery.Equal(contentCallbackConfirmCancel), h.handleCallbackCancel),
			},
		},
		&handlers.ConversationOpts{
			Exits: []ext.Handler{
				handlers.NewCommand(constants.CancelCommand, h.handleCancel),
				handlers.NewCallback(callbackquery.Equal(contentCallbackConfirmCancel), h.handleCallbackCancel),
			},
		},
	)
}

// 1. startContentSearch is the entry point handler for the content search conversation
func (h *contentHandler) startContentSearch(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	if !h.permissionsService.CheckPrivateChatType(msg) {
		return handlers.EndConversation()
	}

	if !h.permissionsService.CheckClubMemberPermissions(msg, constants.ContentCommand) {
		return handlers.EndConversation()
	}

	sentMsg, _ := h.messageSenderService.SendWithReturnMessage(
		msg.Chat.Id,
		"Пришли мне поисковый запрос по контенту:",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.CancelButton(contentCallbackConfirmCancel),
		},
	)

	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
	return handlers.NextConversationState(contentStateStartSearch)
}

// 2. processContentSearch handles the actual content search
func (h *contentHandler) selectSearchType(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	query := strings.TrimSpace(msg.Text)
	if query == "" {
		h.messageSenderService.Send(
			msg.Chat.Id,
			fmt.Sprintf("Поисковый запрос не может быть пустым. Пожалуйста, введи запрос или используй /%s для отмены.",
				constants.CancelCommand),
			nil,
		)
		return nil
	}

	h.userStore.Set(ctx.EffectiveUser.Id, contentCtxDataKeySearchQuery, query)

	msg.Delete(b, nil)
	h.RemovePreviousMessage(b, &ctx.EffectiveUser.Id)

	sentMsg, _ := h.messageSenderService.SendWithReturnMessage(
		msg.Chat.Id,
		fmt.Sprintf("Запрос: \"%s\"\n\nВыбери тип поиска:", query),
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.SearchTypeSelectionButton(
				contentCallbackFastSearch,
				contentCallbackDeepSearch,
				contentCallbackConfirmCancel,
			),
		},
	)

	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
	return handlers.NextConversationState(contentStateSelectSearch)
}

func (h *contentHandler) handleFastSearchSelection(b *gotgbot.Bot, ctx *ext.Context) error {
	cb := ctx.Update.CallbackQuery
	_, _ = cb.Answer(b, nil)
	h.userStore.Set(ctx.EffectiveUser.Id, contentCtxDataKeySearchType, constants.SearchTypeFast)
	return h.processContentSearchWithType(b, ctx)
}

func (h *contentHandler) handleDeepSearchSelection(b *gotgbot.Bot, ctx *ext.Context) error {
	cb := ctx.Update.CallbackQuery
	_, _ = cb.Answer(b, nil)
	h.userStore.Set(ctx.EffectiveUser.Id, contentCtxDataKeySearchType, constants.SearchTypeDeep)
	return h.processContentSearchWithType(b, ctx)
}

func (h *contentHandler) processContentSearchWithType(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userId := ctx.EffectiveUser.Id

	if isProcessing, ok := h.userStore.Get(userId, contentCtxDataKeyProcessing); ok && isProcessing.(bool) {
		h.RemovePreviousMessage(b, &userId)
		msg.Delete(b, nil)
		warningMsg, _ := h.messageSenderService.SendWithReturnMessage(
			msg.Chat.Id,
			fmt.Sprintf("Пожалуйста, дождись окончания обработки предыдущего запроса, или используй /%s для отмены.",
				constants.CancelCommand),
			&gotgbot.SendMessageOpts{ReplyMarkup: buttons.CancelButton(contentCallbackConfirmCancel)},
		)
		h.SavePreviousMessageInfo(userId, warningMsg)
		return nil
	}

	queryInterface, _ := h.userStore.Get(userId, contentCtxDataKeySearchQuery)
	query, _ := queryInterface.(string)
	searchTypeInterface, hasSearchType := h.userStore.Get(userId, contentCtxDataKeySearchType)
	searchType, okType := searchTypeInterface.(string)

	if !hasSearchType || !okType || strings.TrimSpace(searchType) == "" {
		h.messageSenderService.Send(msg.Chat.Id, "Сначала выбери тип поиска кнопками выше!", nil)
		return nil
	}

	h.userStore.Set(userId, contentCtxDataKeyProcessing, true)
	typingCtx, cancelTyping := context.WithCancel(context.Background())
	h.userStore.Set(userId, contentCtxDataKeyCancelFunc, cancelTyping)
	defer func() {
		h.userStore.Set(userId, contentCtxDataKeyProcessing, false)
		h.userStore.Set(userId, contentCtxDataKeyCancelFunc, nil)
	}()

	h.RemovePreviousMessage(b, &userId)

	searchTypeText := "быстрый"
	if searchType == constants.SearchTypeDeep {
		searchTypeText = "глубокий"
	}

	sentMsg, _ := h.messageSenderService.SendWithReturnMessage(
		msg.Chat.Id,
		fmt.Sprintf("Ищу информацию по запросу: \"%s\" (%s поиск)...", query, searchTypeText),
		&gotgbot.SendMessageOpts{ReplyMarkup: buttons.CancelButton(contentCallbackConfirmCancel)},
	)
	h.SavePreviousMessageInfo(userId, sentMsg)

	h.messageSenderService.SendTypingAction(msg.Chat.Id)

	messages, err := h.groupMessageRepository.GetAllByGroupTopicID(int64(h.config.ContentTopicID))
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

	topicLink := fmt.Sprintf("https://t.me/c/%d/%d", h.config.SuperGroupChatID, h.config.ContentTopicID)

	templateText, err := h.promptingTemplateRepository.Get(prompts.GetContentPromptKey, prompts.GetContentPromptDefaultValue)
	if err != nil {
		h.messageSenderService.Send(msg.Chat.Id, "Произошла ошибка при получении шаблона для поиска контента.", nil)
		log.Printf("%s: Error during template retrieval: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	prompt := fmt.Sprintf(
		templateText,
		topicLink,
		utils.EscapeMarkdown(string(dataMessages)),
		utils.EscapeMarkdown(query),
	)

	if err = os.WriteFile("last-prompt-log.txt", []byte(prompt), 0644); err != nil {
		log.Printf("%s: Error writing prompt to file: %v", utils.GetCurrentTypeName(), err)
	}

	defer cancelTyping()
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

	var responseOpenAi string
	if searchType == constants.SearchTypeFast {
		responseOpenAi, err = h.openaiClient.GetCompletionWithReasoning(typingCtx, prompt, openai.ReasoningEffortMinimal)
	} else {
		responseOpenAi, err = h.openaiClient.GetCompletionWithReasoning(typingCtx, prompt, openai.ReasoningEffortMedium)
	}

	if typingCtx.Err() != nil {
		log.Printf("%s: Request was cancelled", utils.GetCurrentTypeName())
		return handlers.EndConversation()
	}

	if err != nil {
		h.messageSenderService.Send(msg.Chat.Id, "Произошла ошибка при получении ответа от OpenAI.", nil)
		log.Printf("%s: Error during OpenAI response retrieval: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	if err = h.messageSenderService.SendHtml(msg.Chat.Id, responseOpenAi, nil); err != nil {
		h.messageSenderService.Send(msg.Chat.Id, "Произошла ошибка при отправке ответа.", nil)
		log.Printf("%s: Error during message sending: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	h.RemovePreviousMessage(b, &userId)
	h.userStore.Clear(userId)

	return handlers.EndConversation()
}

// handleCallbackCancel processes the cancel button click
func (h *contentHandler) handleCallbackCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	// Answer the callback query to remove the loading state on the button
	cb := ctx.Update.CallbackQuery
	_, _ = cb.Answer(b, nil)

	return h.handleCancel(b, ctx)
}

// 3. handleCancel handles the /cancel command
func (h *contentHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if there's an ongoing operation to cancel
	if cancelFunc, ok := h.userStore.Get(ctx.EffectiveUser.Id, contentCtxDataKeyCancelFunc); ok {
		// Call the cancel function to stop any ongoing API calls
		if cf, ok := cancelFunc.(context.CancelFunc); ok {
			cf()
			h.messageSenderService.Send(msg.Chat.Id, "Операция поиска контента отменена.", nil)
		}
	} else {
		h.messageSenderService.Send(msg.Chat.Id, "Операция поиска контента отменена.", nil)
	}

	h.RemovePreviousMessage(b, &ctx.EffectiveUser.Id)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

func (h *contentHandler) preprocessingMessages(messages []*repositories.GroupMessage) ([]byte, error) {
	// MessageObject represents the structured format for AI processing
	type MessageObject struct {
		MessageID int64  `json:"message_id"` // Telegram message ID
		Message   string `json:"message"`    // Message content (HTML formatted)
		Date      string `json:"date"`       // Formatted date
	}

	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages found for processing")
	}

	// Load UTC location for consistent date formatting
	loc, err := time.LoadLocation("UTC")
	if err != nil {
		return nil, fmt.Errorf("failed to load UTC location: %w", err)
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

		// Format date for readability: "2 January 2006"
		dateFormatted := message.CreatedAt.In(loc).Format("2006.01.28")

		messageObjects = append(messageObjects, MessageObject{
			MessageID: message.MessageID,
			Message:   cleanedMessage, // Use cleaned message text
			Date:      dateFormatted,
		})
	}

	if len(messageObjects) == 0 {
		return nil, fmt.Errorf("no valid messages found after filtering")
	}

	// Marshal to JSON for AI processing
	dataMessages, err := json.Marshal(messageObjects)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal messages to JSON: %w", err)
	}

	return dataMessages, nil
}

func (h *contentHandler) MessageRemoveInlineKeyboard(b *gotgbot.Bot, userID *int64) {
	var chatID, messageID int64

	// If userID provided, get stored message info using the utility method
	if userID != nil {
		messageID, chatID = h.userStore.GetPreviousMessageInfo(
			*userID,
			contentCtxDataKeyPreviousMessageID,
			contentCtxDataKeyPreviousChatID,
		)
	}

	// Skip if we don't have valid chat and message IDs
	if chatID == 0 || messageID == 0 {
		return
	}

	// Use message sender service to remove the inline keyboard
	_ = h.messageSenderService.RemoveInlineKeyboard(chatID, messageID)
}

func (h *contentHandler) RemovePreviousMessage(b *gotgbot.Bot, userID *int64) {
	var chatID, messageID int64

	if userID != nil {
		messageID, chatID = h.userStore.GetPreviousMessageInfo(
			*userID,
			contentCtxDataKeyPreviousMessageID,
			contentCtxDataKeyPreviousChatID,
		)
	}

	if chatID == 0 || messageID == 0 {
		return
	}

	b.DeleteMessage(chatID, messageID, nil)
}

func (h *contentHandler) SavePreviousMessageInfo(userID int64, sentMsg *gotgbot.Message) {
	h.userStore.SetPreviousMessageInfo(userID, sentMsg.MessageId, sentMsg.Chat.Id,
		contentCtxDataKeyPreviousMessageID, contentCtxDataKeyPreviousChatID)
}
