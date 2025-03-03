package privatehandlers

import (
	"context"
	"fmt"
	"log"
	"time"

	"your_module_name/internal/handlers"
	"your_module_name/internal/services"
	"your_module_name/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

const summarizeHandlerName = "summarize_handler"
const summarizeCommand = "/summarize"

// SummarizeHandler handles the summarize command
type SummarizeHandler struct {
	summarizationService *services.SummarizationService
	mainChatID           int64
}

// NewSummarizeHandler creates a new summarize handler
func NewSummarizeHandler(summarizationService *services.SummarizationService, mainChatID int64) handlers.Handler {
	return &SummarizeHandler{
		summarizationService: summarizationService,
		mainChatID:           mainChatID,
	}
}

// HandleUpdate handles the update
func (h *SummarizeHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Send a message indicating that summarization has started
	replyMsg, err := msg.Reply(b, "Запуск процесса создания сводки...", nil)
	if err != nil {
		return err
	}

	// Run summarization in a goroutine to avoid blocking
	go func() {
		// Create a context with timeout
		ctxTimeout, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		// Run the summarization
		err := h.summarizationService.RunDailySummarization(ctxTimeout)

		// Update the reply message with the result
		var resultText string
		if err != nil {
			resultText = fmt.Sprintf("Ошибка при создании сводки: %v", err)
			log.Printf("Error running manual summarization: %v", err)
		} else {
			resultText = "Сводка успешно создана и отправлена в указанный чат."
		}

		_, _, err = b.EditMessageText(resultText, &gotgbot.EditMessageTextOpts{
			ChatId:    msg.Chat.Id,
			MessageId: replyMsg.MessageId,
		})
		if err != nil {
			log.Printf("Error updating summarization result message: %v", err)
		}
	}()

	return nil
}

// CheckUpdate checks if the update should be handled
func (h *SummarizeHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	msg := ctx.EffectiveMessage
	if msg == nil {
		return false
	}

	// Check if the message is the summarize command
	if msg.Text != summarizeCommand {
		return false
	}

	// Check if the user is an admin
	if !utils.IsUserAdminOrCreator(b, msg.From.Id, h.mainChatID) {
		msg.Reply(b, "Эта команда доступна только администраторам.", nil)
		return false
	}

	return true
}

// Name returns the handler name
func (h *SummarizeHandler) Name() string {
	return summarizeHandlerName
}
