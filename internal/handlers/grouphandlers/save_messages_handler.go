package grouphandlers

import (
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/services"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

type SaveMessagesHandler struct {
	saveUpdateMessageService *services.SaveUpdateMessageService
}

func NewSaveMessagesHandler(
	saveUpdateMessageService *services.SaveUpdateMessageService,
) ext.Handler {
	h := &SaveMessagesHandler{
		saveUpdateMessageService: saveUpdateMessageService,
	}
	return handlers.NewMessage(h.check, h.handle).SetAllowEdited(true)
}

func (h *SaveMessagesHandler) check(msg *gotgbot.Message) bool {
	if msg == nil {
		return false
	}

	// Skip private chats
	if msg.Chat.Type == constants.PrivateChatType {
		return false
	}

	// Skip forum topic created or edited messages (handled by SaveTopicsHandler)
	if msg.ForumTopicCreated != nil || msg.ForumTopicEdited != nil {
		return false
	}

	// Check if this is a regular message with content
	return msg.Text != "" || msg.Caption != "" || msg.Voice != nil || msg.Audio != nil ||
		msg.Document != nil || msg.Photo != nil || msg.Video != nil || msg.VideoNote != nil ||
		msg.Sticker != nil || msg.Animation != nil
}

func (h *SaveMessagesHandler) handle(b *gotgbot.Bot, ctx *ext.Context) error {
	// Check if this is an edited message
	if ctx.Update.EditedMessage != nil {
		return h.saveUpdateMessageService.UpdateMessage(ctx.Update.EditedMessage)
	}

	// Handle regular new messages
	msg := ctx.EffectiveMessage
	return h.saveUpdateMessageService.SaveMessage(msg)
}
