package publichandlers

import (
	"context"
	"log"
	"time"

	"github.com/it-beard/evo-bot-go/internal/config"
	"github.com/it-beard/evo-bot-go/internal/constants"
	"github.com/it-beard/evo-bot-go/internal/handlers"
	"github.com/it-beard/evo-bot-go/internal/storage"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type MessageCollectorHandler struct {
	config       *config.Config
	messageStore *storage.MessageStore
}

func NewMessageCollectorHandler(messageStore *storage.MessageStore, config *config.Config) handlers.Handler {
	return &MessageCollectorHandler{
		config:       config,
		messageStore: messageStore,
	}
}

func (h *MessageCollectorHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Create a context with timeout
	ctxTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Store the message
	if err := h.messageStore.StoreMessage(ctxTimeout, msg); err != nil {
		log.Printf("%s: error storing message: %v", constants.MessageCollectorHandlerName, err)
		return err
	}

	// Don't reply to the message, just collect it
	return nil
}

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

func (h *MessageCollectorHandler) Name() string {
	return constants.MessageCollectorHandlerName
}
