package publichandlers

import (
	"context"
	"log"
	"time"

	"github.com/it-beard/evo-bot-go/internal/config"
	"github.com/it-beard/evo-bot-go/internal/constants"
	"github.com/it-beard/evo-bot-go/internal/handlers"
	"github.com/it-beard/evo-bot-go/internal/storage"
	"github.com/it-beard/evo-bot-go/internal/utils"

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

	formattedSuperGroupChatId := utils.ChatIdToFullChatId(h.config.SuperGroupChatID)
	// Check if the chat is in the monitored list and from the supergroup
	return msg.SenderChat != nil && msg.SenderChat.Id == formattedSuperGroupChatId &&
		(h.config.IsMonitoredTopic(int(msg.MessageThreadId)) ||
			(h.config.IsMonitoredTopic(0) && !msg.IsTopicMessage)) // small hack for root topic 0 (1 in links)
}

func (h *MessageCollectorHandler) Name() string {
	return constants.MessageCollectorHandlerName
}
