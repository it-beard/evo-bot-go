package privatehandlers

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"your_module_name/internal/handlers"
	"your_module_name/internal/services"
	"your_module_name/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

const summarizeHandlerName = "summarize_handler"
const summarizeCommand = "/summarize"
const dmFlag = "-dm"

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

	// Check if the DM flag is present
	sendToDM := strings.Contains(msg.Text, dmFlag)

	// Send a message indicating that summarization has started
	replyMsg, err := msg.Reply(b, "Запуск процесса создания сводки...", nil)
	if err != nil {
		return err
	}

	// Send typing action using MessageSender.
	sender := services.NewMessageSender(b)
	sender.SendTypingAction(msg.Chat.Id)

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
					sender.SendTypingAction(msg.Chat.Id)
				case <-typingCtx.Done():
					return
				}
			}
		}()
		// Create a context with timeout and user ID for DM
		ctxWithValues := context.WithValue(context.Background(), "userID", msg.From.Id)
		ctxTimeout, cancel := context.WithTimeout(ctxWithValues, 10*time.Minute)
		defer cancel()

		// Run the summarization with the sendToDM flag
		err := h.summarizationService.RunDailySummarization(ctxTimeout, sendToDM)

		// Update the reply message with the result
		var resultText string
		if err != nil {
			resultText = fmt.Sprintf("Ошибка при создании сводки: %v", err)
			log.Printf("Error running manual summarization: %v", err)
		} else {
			if sendToDM {
				resultText = "Сводка успешно создана и отправлена вам в личные сообщения."
			} else {
				resultText = "Сводка успешно создана и отправлена в указанный чат."
			}
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

	return nil
}

// CheckUpdate checks if the update should be handled
func (h *SummarizeHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	msg := ctx.EffectiveMessage
	if msg == nil {
		return false
	}

	// Check if the message is the summarize command (with or without DM flag)
	if !strings.HasPrefix(msg.Text, summarizeCommand) {
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
