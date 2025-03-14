package publichandlers

import (
	"context"
	"log"
	"time"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/handlers"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type MessageCollectorHandler struct {
	config   *config.Config
	messages *repositories.MessageRepository
}

func NewMessageCollectorHandler(messages *repositories.MessageRepository, config *config.Config) handlers.Handler {
	return &MessageCollectorHandler{
		config:   config,
		messages: messages,
	}
}

func (h *MessageCollectorHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Create a context with timeout
	ctxTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Store the message
	if err := h.messages.Store(ctxTimeout, msg); err != nil {
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
	return msg.Chat.Id == formattedSuperGroupChatId &&
		(h.config.IsMonitoredTopic(int(msg.MessageThreadId)) ||
			(h.config.IsMonitoredTopic(0) && !msg.IsTopicMessage)) // small hack for root topic 0 (1 in links)
}

func (h *MessageCollectorHandler) Name() string {
	return constants.MessageCollectorHandlerName
}
