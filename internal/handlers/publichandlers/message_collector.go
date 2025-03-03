package publichandlers

import (
	"context"
	"log"
	"time"

	"your_module_name/internal/config"
	"your_module_name/internal/handlers"
	"your_module_name/internal/storage"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

const messageCollectorHandlerName = "message_collector_handler"

// MessageCollectorHandler collects messages from monitored chats
type MessageCollectorHandler struct {
	config       *config.Config
	messageStore *storage.MessageStore
}

// NewMessageCollectorHandler creates a new message collector handler
func NewMessageCollectorHandler(config *config.Config, messageStore *storage.MessageStore) handlers.Handler {
	return &MessageCollectorHandler{
		config:       config,
		messageStore: messageStore,
	}
}

// HandleUpdate handles the update
func (h *MessageCollectorHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Create a context with timeout
	ctxTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Store the message
	if err := h.messageStore.StoreMessage(ctxTimeout, msg); err != nil {
		log.Printf("%s: error storing message: %v", messageCollectorHandlerName, err)
		return err
	}

	// Don't reply to the message, just collect it
	return nil
}

// CheckUpdate checks if the update should be handled
func (h *MessageCollectorHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	msg := ctx.EffectiveMessage
	if msg == nil {
		return false
	}

	// Only process text messages from monitored chats
	if msg.Text == "" && msg.Caption == "" {
		return false
	}

	// Check if the chat is in the monitored list
	return h.config.IsMonitoredChat(msg.Chat.Id)
}

// Name returns the handler name
func (h *MessageCollectorHandler) Name() string {
	return messageCollectorHandlerName
}
