package testhandlers

import (
	"context"
	"log"
	"time"

	"evo-bot-go/internal/buttons"
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
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
	trySummarizeStateProcessCallbacks = "try_summarize_state_process_callbacks"

	// Context data keys
	trySummarizeCtxDataKeyPreviousMessageID = "try_summarize_ctx_data_previous_message_id"
	trySummarizeCtxDataKeyPreviousChatID    = "try_summarize_ctx_data_previous_chat_id"

	// Callback data
	trySummarizeCallbackConfirmYes    = "try_summarize_callback_confirm_yes"
	trySummarizeCallbackConfirmCancel = "try_summarize_callback_confirm_cancel"
)

type trySummarizeHandler struct {
	config               *config.Config
	summarizationService *services.SummarizationService
	messageSenderService *services.MessageSenderService
	userStore            *utils.UserDataStore
	permissionsService   *services.PermissionsService
}

func NewTrySummarizeHandler(
	config *config.Config,
	summarizationService *services.SummarizationService,
	messageSenderService *services.MessageSenderService,
	permissionsService *services.PermissionsService,
) ext.Handler {
	h := &trySummarizeHandler{
		config:               config,
		summarizationService: summarizationService,
		messageSenderService: messageSenderService,
		userStore:            utils.NewUserDataStore(),
		permissionsService:   permissionsService,
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.TrySummarizeCommand, h.startSummarizeConversation),
		},
		map[string][]ext.Handler{
			trySummarizeStateProcessCallbacks: {
				handlers.NewCallback(callbackquery.Equal(trySummarizeCallbackConfirmYes), h.handleCallbackConfirmation),
				handlers.NewCallback(callbackquery.Equal(trySummarizeCallbackConfirmCancel), h.handleCallbackCancel),
				handlers.NewMessage(message.All, h.handleTextDuringConfirmation),
			},
		},
		&handlers.ConversationOpts{
			Exits: []ext.Handler{handlers.NewCommand(constants.CancelCommand, h.handleCancel)},
		},
	)
}

// startSummarizeConversation initiates the summarize conversation
func (h *trySummarizeHandler) startSummarizeConversation(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if user has admin permissions and is in a private chat
	if !h.permissionsService.CheckAdminAndPrivateChat(msg, constants.ShowTopicsCommand) {
		log.Printf("TrySummarizeHandler: User %d (%s) tried to use /%s without admin permissions.",
			ctx.EffectiveUser.Id, ctx.EffectiveUser.Username, constants.ShowTopicsCommand)
		return handlers.EndConversation()
	}

	log.Printf("%s: User %d initiated summarization", utils.GetCurrentTypeName(), msg.From.Id)

	// Check if the user is an admin
	if !utils.IsUserAdminOrCreator(b, msg.From.Id, h.config) {
		msg.Reply(b, "Эта команда доступна только администраторам.", nil)
		return handlers.EndConversation()
	}

	// Ask user to confirm with inline keyboard
	sentMsg, _ := h.messageSenderService.ReplyWithReturnMessage(
		msg,
		"Вы собираетесь запустить процесс тестирования саммаризации общения в клубе. Саммаризация будет отправлена в личные сообщения.\n\nПодтвердите действие, нажав одну из кнопок ниже:",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.ConfirmAndCancelButton(trySummarizeCallbackConfirmYes, trySummarizeCallbackConfirmCancel),
		},
	)

	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
	return handlers.NextConversationState(trySummarizeStateProcessCallbacks)
}

// handleCallbackConfirmation processes the user's callback confirmation
func (h *trySummarizeHandler) handleCallbackConfirmation(b *gotgbot.Bot, ctx *ext.Context) error {
	// Answer the callback query to remove the loading state on the button
	cb := ctx.Update.CallbackQuery
	log.Printf("%s: User %d confirmed summarization", utils.GetCurrentTypeName(), cb.From.Id)
	_, _ = cb.Answer(b, nil)

	return h.startSummarization(b, ctx)
}

// handleCallbackCancel processes the cancel button click
func (h *trySummarizeHandler) handleCallbackCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	// Answer the callback query to remove the loading state on the button
	cb := ctx.Update.CallbackQuery
	_, _ = cb.Answer(b, nil)

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)
	h.messageSenderService.Reply(ctx.EffectiveMessage, "Операция саммаризации отменена.", nil)
	log.Printf("%s: Summarization canceled", utils.GetCurrentTypeName())

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// startSummarization starts the summarization process
func (h *trySummarizeHandler) startSummarization(b *gotgbot.Bot, ctx *ext.Context) error {
	// Get the chat ID from either the message or callback query
	chatId := ctx.EffectiveMessage.Chat.Id

	// Get the handler name using our new utility function
	log.Printf("%s: Starting summarization process", utils.GetCurrentTypeName())

	h.messageSenderService.Reply(ctx.EffectiveMessage, "Запуск процесса саммаризации...", nil)

	// Send typing action using MessageSender.
	h.messageSenderService.SendTypingAction(chatId)

	// Run summarization in a goroutine to avoid blocking
	go func() {
		// Start periodic typing action every 5 seconds while waiting for the OpenAI response.
		typingCtx, cancelTyping := context.WithCancel(context.Background())
		defer cancelTyping() // ensure cancellation if function exits early

		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					h.messageSenderService.SendTypingAction(chatId)
				case <-typingCtx.Done():
					return
				}
			}
		}()

		// Create a context with timeout and user ID for DM
		ctxWithValues := context.WithValue(context.Background(), "userID", ctx.EffectiveUser.Id)
		ctxTimeout, cancel := context.WithTimeout(ctxWithValues, 10*time.Minute)
		defer cancel()

		// Run the summarization with sendToDM=true as default
		err := h.summarizationService.RunDailySummarization(ctxTimeout, true)
		if err != nil {
			h.messageSenderService.Reply(ctx.EffectiveMessage, "Ошибка при создании саммаризации.", nil)
			log.Printf("%s: Error during summarization: %v", utils.GetCurrentTypeName(), err)
		}

		h.messageSenderService.Reply(ctx.EffectiveMessage, "Саммаризация успешно создана.", nil)
		log.Printf("%s: Summarization created successfully", utils.GetCurrentTypeName())

		// Cancel the periodic typing action immediately after getting the response.
		cancelTyping()

	}()

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)
	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// handleCancel handles the /cancel command
func (h *trySummarizeHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	log.Printf("%s: User %d canceled using /cancel command", utils.GetCurrentTypeName(), msg.From.Id)

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)
	h.messageSenderService.Reply(msg, "Операция саммаризации отменена.", nil)
	log.Printf("%s: Summarization canceled", utils.GetCurrentTypeName())

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// handleTextDuringConfirmation handles text messages during the confirmation state
func (h *trySummarizeHandler) handleTextDuringConfirmation(b *gotgbot.Bot, ctx *ext.Context) error {
	log.Printf("%s: User %d sent text during confirmation", utils.GetCurrentTypeName(), ctx.EffectiveUser.Id)

	h.messageSenderService.Reply(
		ctx.EffectiveMessage,
		"Пожалуйста, нажмите на одну из кнопок выше, или используйте кнопку отмены.",
		nil,
	)
	return nil // Stay in the same state
}

func (h *trySummarizeHandler) MessageRemoveInlineKeyboard(b *gotgbot.Bot, userID *int64) {
	var chatID, messageID int64

	// If userID provided, get stored message info using the utility method
	if userID != nil {
		messageID, chatID = h.userStore.GetPreviousMessageInfo(
			*userID,
			trySummarizeCtxDataKeyPreviousMessageID,
			trySummarizeCtxDataKeyPreviousChatID,
		)
	}

	// Skip if we don't have valid chat and message IDs
	if chatID == 0 || messageID == 0 {
		return
	}

	// Use message sender service to remove the inline keyboard
	_ = h.messageSenderService.RemoveInlineKeyboard(chatID, messageID)
}

func (h *trySummarizeHandler) SavePreviousMessageInfo(userID int64, sentMsg *gotgbot.Message) {
	h.userStore.SetPreviousMessageInfo(userID, sentMsg.MessageId, sentMsg.Chat.Id,
		trySummarizeCtxDataKeyPreviousMessageID, trySummarizeCtxDataKeyPreviousChatID)
}
