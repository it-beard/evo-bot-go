package grouphandlers

import (
	"evo-bot-go/internal/constants"
	"fmt"
	"log"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

type deleteJoinLeftMessagesHandler struct{}

func NewDeleteJoinLeftMessagesHandler() ext.Handler {
	h := &deleteJoinLeftMessagesHandler{}

	return handlers.NewMessage(h.check, h.handle)
}

func (h *deleteJoinLeftMessagesHandler) check(msg *gotgbot.Message) bool {
	if msg == nil {
		return false
	}

	return msg.NewChatMembers != nil || msg.LeftChatMember != nil
}

func (h *deleteJoinLeftMessagesHandler) handle(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	_, err := msg.Delete(b, nil)
	if err != nil {
		return fmt.Errorf("%s: Error deleting message: %v", constants.DeleteJoinLeftMessagesHandlerName, err)
	}

	if msg.NewChatMembers != nil {
		log.Printf("%s: New user joined. Username: %v. User ID: %v", constants.DeleteJoinLeftMessagesHandlerName, msg.NewChatMembers[0].Username, msg.NewChatMembers[0].Id)
	} else if msg.LeftChatMember != nil {
		log.Printf("%s: User left. Username: %v. User ID: %v", constants.DeleteJoinLeftMessagesHandlerName, msg.LeftChatMember.Username, msg.LeftChatMember.Id)
	}

	return nil
}
