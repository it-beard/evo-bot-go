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
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
)

const (
	// Conversation states
	trySummarizeHandlerStateConfirmation = "try_summarize_handler_confirmation"
)

type trySummarizeHandler struct {
	summarizationService *services.SummarizationService
	messageSenderService services.MessageSenderService
	config               *config.Config
	userStore            *utils.UserDataStore
}

// NewTrySummarizeHandler creates a new try summarize handler
func NewTrySummarizeHandler(
	summarizationService *services.SummarizationService,
	messageSenderService services.MessageSenderService,
	config *config.Config,
) ext.Handler {
	h := &trySummarizeHandler{
		summarizationService: summarizationService,
		messageSenderService: messageSenderService,
		config:               config,
		userStore:            utils.NewUserDataStore(),
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.TrySummarizeCommand, h.startSummarizeConversation),
		},
		map[string][]ext.Handler{
			trySummarizeHandlerStateConfirmation: {
				handlers.NewMessage(message.All, h.handleConfirmation),
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

	// Check if the user is an admin
	if !utils.IsUserAdminOrCreator(b, msg.From.Id, h.config.SuperGroupChatID) {
		msg.Reply(b, "Эта команда доступна только администраторам.", nil)
		return handlers.EndConversation()
	}

	// Ask user to confirm
	utils.SendLoggedReply(b, msg,
		fmt.Sprintf("Вы собираетесь запустить процесс тестирования саммаризации общения в клубе. Саммаризация будет отправлена в личные сообщения.\n\nПодтвердите действие набрав 'да' или используйте /%s для отмены:",
			constants.CancelCommand), nil)

	return handlers.NextConversationState(trySummarizeHandlerStateConfirmation)
}

// handleConfirmation processes the user's confirmation
func (h *trySummarizeHandler) handleConfirmation(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	confirmation := msg.Text
	if confirmation != "да" && confirmation != "Да" {
		utils.SendLoggedReply(b, msg,
			fmt.Sprintf("Действие не подтверждено. Пожалуйста, введите 'да' для подтверждения или используйте /%s для отмены:",
				constants.CancelCommand), nil)
		return nil // Stay in the same state
	}

	// Send a message indicating that summarization has started
	replyMsg, err := msg.Reply(b, "Запуск процесса саммаризации...", nil)
	if err != nil {
		return handlers.EndConversation()
	}

	// Send typing action using MessageSender.
	h.messageSenderService.SendTypingAction(msg.Chat.Id)

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
					h.messageSenderService.SendTypingAction(msg.Chat.Id)
				case <-typingCtx.Done():
					return
				}
			}
		}()

		// Create a context with timeout and user ID for DM
		ctxWithValues := context.WithValue(context.Background(), "userID", msg.From.Id)
		ctxTimeout, cancel := context.WithTimeout(ctxWithValues, 10*time.Minute)
		defer cancel()

		// Run the summarization with sendToDM=true as default
		err := h.summarizationService.RunDailySummarization(ctxTimeout, true)

		// Update the reply message with the result
		var resultText string
		if err != nil {
			resultText = fmt.Sprintf("Ошибка при создании саммаризации: %v", err)
			log.Printf("Error running manual summarization: %v", err)
		} else {
			resultText = "Саммаризация общения в клубе успешно завершена, результат отправлен вам в личные сообщения."
		}

		// Cancel the periodic typing action immediately after getting the response.
		cancelTyping()
		_, _, err = b.EditMessageText(resultText, &gotgbot.EditMessageTextOpts{
			ChatId:    msg.Chat.Id,
			MessageId: replyMsg.MessageId,
		})
		if err != nil {
			log.Printf("Error updating summarization result message: %v", err)
		}
	}()

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// handleCancel handles the /cancel command
func (h *trySummarizeHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	utils.SendLoggedReply(b, msg, "Операция саммаризации отменена.", nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}
