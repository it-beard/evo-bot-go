package handlers

import (
	"log"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type DeleteMessagesHandler struct{}

func NewDeleteMessagesHandler() Handler {
	return &DeleteMessagesHandler{}
}

func (h *DeleteMessagesHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	_, err := ctx.EffectiveMessage.Delete(b, nil)
	if err != nil {
		log.Printf("Error deleting message: %v", err)
	} else {
		if ctx.EffectiveMessage.NewChatMembers != nil {
			log.Printf("New user joined. User ID: %v", ctx.EffectiveMessage.NewChatMembers[0].Username)
		} else if ctx.EffectiveMessage.LeftChatMember != nil {
			log.Printf("User left. User ID: %v", ctx.EffectiveMessage.LeftChatMember.Username)
		}
	}
	return nil
}

func (h *DeleteMessagesHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	return ctx.EffectiveMessage.NewChatMembers != nil || ctx.EffectiveMessage.LeftChatMember != nil
}

func (h *DeleteMessagesHandler) Name() string {
	return "delete_messages_handler"
}
