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
	"github.com/gotd/td/tg"
)

const (
	// Conversation states names
	introStateProcessQuery = "intro_state_process_query"

	// UserStore keys
	introCtxDataKeyProcessing        = "intro_ctx_data_processing"
	introCtxDataKeyCancelFunc        = "intro_ctx_data_cancel_func"
	introCtxDataKeyPreviousMessageID = "intro_ctx_data_previous_message_id"
	introCtxDataKeyPreviousChatID    = "intro_ctx_data_previous_chat_id"

	// Callback data
	introCallbackConfirmCancel = "intro_callback_confirm_cancel"
)

type introHandler struct {
	config                      *config.Config
	openaiClient                *clients.OpenAiClient
	promptingTemplateRepository *repositories.PromptingTemplateRepository
	messageSenderService        *services.MessageSenderService
	userStore                   *utils.UserDataStore
	permissionsService          *services.PermissionsService
}

func NewIntroHandler(
	config *config.Config,
	openaiClient *clients.OpenAiClient,
	messageSenderService *services.MessageSenderService,
	promptingTemplateRepository *repositories.PromptingTemplateRepository,
	permissionsService *services.PermissionsService,
) ext.Handler {
	h := &introHandler{
		config:                      config,
		openaiClient:                openaiClient,
		promptingTemplateRepository: promptingTemplateRepository,
		messageSenderService:        messageSenderService,
		userStore:                   utils.NewUserDataStore(),
		permissionsService:          permissionsService,
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.IntroCommand, h.startIntroSearch),
		},
		map[string][]ext.Handler{
			introStateProcessQuery: {
				handlers.NewMessage(message.All, h.processIntroSearch),
				handlers.NewCallback(callbackquery.Equal(introCallbackConfirmCancel), h.handleCallbackCancel),
			},
		},
		&handlers.ConversationOpts{
			Exits: []ext.Handler{
				handlers.NewCommand(constants.CancelCommand, h.handleCancel),
				handlers.NewCallback(callbackquery.Equal(introCallbackConfirmCancel), h.handleCallbackCancel),
			},
		},
	)
}

// startIntroSearch is the entry point handler for the intro search conversation
func (h *introHandler) startIntroSearch(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Only proceed if this is a private chat
	if !h.permissionsService.CheckPrivateChatType(msg) {
		return handlers.EndConversation()
	}

	// Check if user is a club member
	if !h.permissionsService.CheckClubMemberPermissions(msg, constants.IntroCommand) {
		return handlers.EndConversation()
	}

	// Ask user to enter search query
	sentMsg, _ := h.messageSenderService.ReplyWithReturnMessage(
		msg,
		fmt.Sprintf("Введите поисковый запрос по участникам клуба или нажмите /%s для получения общей информации:", constants.CancelCommand),
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.CancelButton(introCallbackConfirmCancel),
		},
	)

	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
	return handlers.NextConversationState(introStateProcessQuery)
}

// processIntroSearch handles the actual intro search
func (h *introHandler) processIntroSearch(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if we're already processing a request for this user
	if isProcessing, ok := h.userStore.Get(ctx.EffectiveUser.Id, introCtxDataKeyProcessing); ok && isProcessing.(bool) {
		h.messageSenderService.Reply(
			msg,
			fmt.Sprintf("Пожалуйста, дождитесь окончания обработки предыдущего запроса, или используйте /%s для отмены.", constants.CancelCommand),
			nil,
		)
		return nil // Stay in the same state
	}

	// Get query from user message
	query := strings.TrimSpace(msg.Text)

	// Mark as processing
	h.userStore.Set(ctx.EffectiveUser.Id, introCtxDataKeyProcessing, true)

	// Create a cancellable context for this operation
	typingCtx, cancelTyping := context.WithCancel(context.Background())

	// Store cancel function in user store so it can be called from handleCancel
	h.userStore.Set(ctx.EffectiveUser.Id, introCtxDataKeyCancelFunc, cancelTyping)

	// Make sure we clean up the processing flag in all exit paths
	defer func() {
		h.userStore.Set(ctx.EffectiveUser.Id, introCtxDataKeyProcessing, false)
		h.userStore.Set(ctx.EffectiveUser.Id, introCtxDataKeyCancelFunc, nil)
	}()

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	// Inform user that processing has started
	var sentMsg *gotgbot.Message
	if query == "" {
		sentMsg, _ = h.messageSenderService.ReplyWithReturnMessage(msg, "Генерирую вводную информацию о клубе...", &gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.CancelButton(introCallbackConfirmCancel),
		})
	} else {
		sentMsg, _ = h.messageSenderService.ReplyWithReturnMessage(msg, fmt.Sprintf("Ищу информацию по запросу: \"%s\"...", query), &gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.CancelButton(introCallbackConfirmCancel),
		})
	}
	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)

	// Send typing action using MessageSender.
	h.messageSenderService.SendTypingAction(msg.Chat.Id)

	// Get messages from Intro topic
	messages, err := clients.GetChatMessages(h.config.SuperGroupChatID, h.config.IntroTopicID)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при получении сообщений из чата.", nil)
		log.Printf("IntroHandler: Error during messages retrieval: %v", err)
		return handlers.EndConversation()
	}

	dataMessages, err := h.prepareTelegramMessages(messages)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при подготовке сообщений для обработки.", nil)
		log.Printf("IntroHandler: Error during messages preparation: %v", err)
		return handlers.EndConversation()
	}

	// Get the prompt template from the database
	templateText, err := h.promptingTemplateRepository.Get(prompts.GetIntroPromptTemplateDbKey)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при получении шаблона для вводной информации.", nil)
		log.Printf("IntroHandler: Error during template retrieval: %v", err)
		return handlers.EndConversation()
	}

	topicLink := fmt.Sprintf("https://t.me/c/%d/%d", h.config.SuperGroupChatID, h.config.IntroTopicID)

	prompt := fmt.Sprintf(
		templateText,
		topicLink,
		utils.EscapeMarkdown(string(dataMessages)),
		utils.EscapeMarkdown(query),
	)

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
		h.messageSenderService.Reply(msg, "Произошла ошибка при получении ответа от OpenAI.", nil)
		log.Printf("IntroHandler: Error during OpenAI response retrieval: %v", err)
		return handlers.EndConversation()
	}

	err = h.messageSenderService.ReplyMarkdown(msg, responseOpenAi, nil)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при отправке ответа.", nil)
		log.Printf("IntroHandler: Error during message sending: %v", err)
		return handlers.EndConversation()
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)
	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// handleCallbackCancel processes the cancel button click
func (h *introHandler) handleCallbackCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	// Answer the callback query to remove the loading state on the button
	cb := ctx.Update.CallbackQuery
	_, _ = cb.Answer(b, nil)

	return h.handleCancel(b, ctx)
}

// handleCancel handles the /cancel command
func (h *introHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if there's an ongoing operation to cancel
	if cancelFunc, ok := h.userStore.Get(ctx.EffectiveUser.Id, introCtxDataKeyCancelFunc); ok {
		// Call the cancel function to stop any ongoing API calls
		if cf, ok := cancelFunc.(context.CancelFunc); ok {
			cf()
			h.messageSenderService.Reply(msg, "Операция поиска вводной информации отменена.", nil)
		}
	} else {
		h.messageSenderService.Reply(msg, "Операция поиска вводной информации отменена.", nil)
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

func (h *introHandler) prepareTelegramMessages(messages []tg.Message) ([]byte, error) {

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

func (h *introHandler) MessageRemoveInlineKeyboard(b *gotgbot.Bot, userID *int64) {
	var chatID, messageID int64

	// If userID provided, get stored message info using the utility method
	if userID != nil {
		messageID, chatID = h.userStore.GetPreviousMessageInfo(
			*userID,
			introCtxDataKeyPreviousMessageID,
			introCtxDataKeyPreviousChatID,
		)
	}

	// Skip if we don't have valid chat and message IDs
	if chatID == 0 || messageID == 0 {
		return
	}

	// Use message sender service to remove the inline keyboard
	_ = h.messageSenderService.RemoveInlineKeyboard(chatID, messageID)
}

func (h *introHandler) SavePreviousMessageInfo(userID int64, sentMsg *gotgbot.Message) {
	h.userStore.SetPreviousMessageInfo(userID, sentMsg.MessageId, sentMsg.Chat.Id,
		introCtxDataKeyPreviousMessageID, introCtxDataKeyPreviousChatID)
}
