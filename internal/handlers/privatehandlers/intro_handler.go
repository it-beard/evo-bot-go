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
	"evo-bot-go/internal/formatters"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/gotd/td/tg"
)

const (
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
			handlers.NewCommand(constants.IntroCommand, h.handleIntro),
		},
		map[string][]ext.Handler{},
		&handlers.ConversationOpts{
			Exits: []ext.Handler{
				handlers.NewCommand(constants.CancelCommand, h.handleCancel),
				handlers.NewCallback(callbackquery.Equal(introCallbackConfirmCancel), h.handleCallbackCancel),
			},
		},
	)
}

// handleIntro is the handler for the intro command
func (h *introHandler) handleIntro(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Only proceed if this is a private chat
	if !h.permissionsService.CheckPrivateChatType(msg) {
		return nil
	}

	// Check if user is a club member
	if !h.permissionsService.CheckClubMemberPermissions(msg, constants.IntroCommand) {
		return nil
	}

	// Check if we're already processing a request for this user
	if isProcessing, ok := h.userStore.Get(ctx.EffectiveUser.Id, introCtxDataKeyProcessing); ok && isProcessing.(bool) {
		h.messageSenderService.Reply(
			msg,
			fmt.Sprintf("Пожалуйста, дождитесь окончания обработки предыдущего запроса, или используйте /%s для отмены.", constants.CancelCommand),
			nil,
		)
		return nil
	}

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

	// Inform user that processing has started
	sentMsg, _ := h.messageSenderService.ReplyWithReturnMessage(msg, "Генерирую вводную информацию о клубе...", &gotgbot.SendMessageOpts{
		ReplyMarkup: formatters.CancelButton(introCallbackConfirmCancel),
	})
	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)

	// Send typing action using MessageSender.
	h.messageSenderService.SendTypingAction(msg.Chat.Id)

	// Get messages from Invite topic
	messages, err := clients.GetChatMessages(h.config.SuperGroupChatID, h.config.InviteTopicID)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при получении сообщений из чата.", nil)
		log.Printf("IntroHandler: Error during messages retrieval: %v", err)
		return nil
	}

	dataMessages, err := h.prepareTelegramMessages(messages)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при подготовке сообщений для обработки.", nil)
		log.Printf("IntroHandler: Error during messages preparation: %v", err)
		return nil
	}

	// Get the prompt template from the database
	templateText, err := h.promptingTemplateRepository.Get(prompts.GetIntroPromptTemplateDbKey)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при получении шаблона для вводной информации.", nil)
		log.Printf("IntroHandler: Error during template retrieval: %v", err)
		return nil
	}

	prompt := fmt.Sprintf(
		templateText,
		string(dataMessages))

	// Save the prompt into a temporary file for logging purposes.
	err = os.WriteFile("last-intro-prompt-log.txt", []byte(prompt), 0644)
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
		return nil
	}

	// Continue only if no errors
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при получении ответа от OpenAI.", nil)
		log.Printf("IntroHandler: Error during OpenAI response retrieval: %v", err)
		return nil
	}

	h.messageSenderService.ReplyMarkdown(msg, responseOpenAi, nil)

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)
	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return nil
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
			h.messageSenderService.Reply(msg, "Операция генерации вводной информации отменена.", nil)
		}
	} else {
		h.messageSenderService.Reply(msg, "Операция генерации вводной информации отменена.", nil)
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return nil
}

func (h *introHandler) prepareTelegramMessages(messages []tg.Message) ([]byte, error) {
	// Modified MessageObject to have Date as string
	type MessageObject struct {
		ID      int    `json:"id"`
		Message string `json:"message"`
		Date    string `json:"date"` // formatted as "10 february 2024"
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
