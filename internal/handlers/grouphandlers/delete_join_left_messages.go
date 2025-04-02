package grouphandlers

import (
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/handlers"
	"fmt"
	"log"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// todo: refactor to use ext.Handler

type DeleteJoinLeftMessagesHandler struct{}

func NewDeleteJoinLeftMessagesHandler() handlers.Handler {
	return &DeleteJoinLeftMessagesHandler{}
}

func (h *DeleteJoinLeftMessagesHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
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

func (h *DeleteJoinLeftMessagesHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	msg := ctx.EffectiveMessage
	if msg == nil {
		return false
	}

	return msg.NewChatMembers != nil || msg.LeftChatMember != nil
}

func (h *DeleteJoinLeftMessagesHandler) Name() string {
	return constants.DeleteJoinLeftMessagesHandlerName
}
