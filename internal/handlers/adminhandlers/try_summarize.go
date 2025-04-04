package adminhandlers

import (
	"context"
	"fmt"
	"log"
	"time"

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
	trySummarizeHandlerStateProcessCallbacks = "try_summarize_handler_state_process_callbacks"
	// Callbacks names
	trySummarizeCallbackConfirmYes    = "try_summarize_callback_confirm_yes"
	trySummarizeCallbackConfirmCancel = "try_summarize_callback_confirm_cancel"
)

type trySummarizeHandler struct {
	config               *config.Config
	summarizationService *services.SummarizationService
	messageSenderService services.MessageSenderService
	userStore            *utils.UserDataStore
}

func NewTrySummarizeHandler(
	config *config.Config,
	summarizationService *services.SummarizationService,
	messageSenderService services.MessageSenderService,
) ext.Handler {
	h := &trySummarizeHandler{
		config:               config,
		summarizationService: summarizationService,
		messageSenderService: messageSenderService,
		userStore:            utils.NewUserDataStore(),
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.TrySummarizeCommand, h.startSummarizeConversation),
		},
		map[string][]ext.Handler{
			trySummarizeHandlerStateProcessCallbacks: {
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
	log.Printf("%s: User %d initiated summarization", utils.GetCurrentHandlerName(), msg.From.Id)

	// Check if the user is an admin
	if !utils.IsUserAdminOrCreator(b, msg.From.Id, h.config.SuperGroupChatID) {
		msg.Reply(b, "Эта команда доступна только администраторам.", nil)
		return handlers.EndConversation()
	}

	// Create an inline keyboard with "Да" and "Отмена" buttons
	inlineKeyboard := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         "✅ Да",
					CallbackData: trySummarizeCallbackConfirmYes,
				},
				{
					Text:         "❌ Отмена",
					CallbackData: trySummarizeCallbackConfirmCancel,
				},
			},
		},
	}

	// Ask user to confirm with inline keyboard
	h.messageSenderService.Reply(
		b,
		msg,
		"Вы собираетесь запустить процесс тестирования саммаризации общения в клубе. Саммаризация будет отправлена в личные сообщения.\n\nПодтвердите действие, нажав одну из кнопок ниже:",
		&gotgbot.SendMessageOpts{
			ReplyMarkup: inlineKeyboard,
		})

	return handlers.NextConversationState(trySummarizeHandlerStateProcessCallbacks)
}

// handleCallbackConfirmation processes the user's callback confirmation
func (h *trySummarizeHandler) handleCallbackConfirmation(b *gotgbot.Bot, ctx *ext.Context) error {
	// Answer the callback query to remove the loading state on the button
	cb := ctx.Update.CallbackQuery
	log.Printf("%s: User %d confirmed summarization", utils.GetCurrentHandlerName(), cb.From.Id)
	_, _ = cb.Answer(b, nil)

	return h.startSummarization(b, ctx)
}

// handleCallbackCancel processes the cancel button click
func (h *trySummarizeHandler) handleCallbackCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	// Answer the callback query to remove the loading state on the button
	cb := ctx.Update.CallbackQuery
	_, _ = cb.Answer(b, nil)

	h.messageSenderService.Reply(b, ctx.EffectiveMessage, "Операция саммаризации отменена.", nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// startSummarization starts the summarization process
func (h *trySummarizeHandler) startSummarization(b *gotgbot.Bot, ctx *ext.Context) error {
	// Get the chat ID from either the message or callback query
	chatId := ctx.EffectiveMessage.Chat.Id

	// Get the handler name using our new utility function
	log.Printf("%s: Starting summarization process", utils.GetCurrentHandlerName())

	h.messageSenderService.Reply(b, ctx.EffectiveMessage, "Запуск процесса саммаризации...", nil)

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
			h.messageSenderService.Reply(b, ctx.EffectiveMessage, "Ошибка при создании саммаризации.", nil)
			log.Printf("%s: Error during summarization: %v", utils.GetCurrentHandlerName(), err)
		}

		// Cancel the periodic typing action immediately after getting the response.
		cancelTyping()

	}()

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// handleCancel handles the /cancel command
func (h *trySummarizeHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	log.Printf("%s: User %d canceled using /cancel command", utils.GetCurrentHandlerName(), msg.From.Id)

	h.messageSenderService.Reply(b, msg, "Операция саммаризации отменена.", nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// handleTextDuringConfirmation handles text messages during the confirmation state
func (h *trySummarizeHandler) handleTextDuringConfirmation(b *gotgbot.Bot, ctx *ext.Context) error {
	log.Printf("%s: User %d sent text during confirmation", utils.GetCurrentHandlerName(), ctx.EffectiveUser.Id)

	h.messageSenderService.Reply(
		b,
		ctx.EffectiveMessage,
		fmt.Sprintf("Пожалуйста, нажмите на одну из кнопок выше, или используйте /%s для отмены.", constants.CancelCommand),
		nil,
	)
	return nil // Stay in the same state
}
