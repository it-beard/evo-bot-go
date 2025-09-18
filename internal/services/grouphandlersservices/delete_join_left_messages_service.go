package grouphandlersservices

import (
	"evo-bot-go/internal/utils"
	"fmt"
	"log"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

type DeleteJoinLeftMessagesService struct {
}

func NewDeleteJoinLeftMessagesService() *DeleteJoinLeftMessagesService {
	return &DeleteJoinLeftMessagesService{}
}

func (h *DeleteJoinLeftMessagesService) DeleteJoinLeftMessages(msg *gotgbot.Message, b *gotgbot.Bot) error {
	// Delete message
	_, err := msg.Delete(b, nil)
	if err != nil {
		return fmt.Errorf("%s: Error deleting message: %v", utils.GetCurrentTypeName(), err)
	}

	// Log the deletion
	if msg.NewChatMembers != nil {
		log.Printf("%s: New user joined. Username: %v. User ID: %v", utils.GetCurrentTypeName(), msg.NewChatMembers[0].Username, msg.NewChatMembers[0].Id)
	} else if msg.LeftChatMember != nil {
		log.Printf("%s: User left. Username: %v. User ID: %v", utils.GetCurrentTypeName(), msg.LeftChatMember.Username, msg.LeftChatMember.Id)
	}

	return nil
}

func (h *DeleteJoinLeftMessagesService) IsMessageShouldBeDeleted(msg *gotgbot.Message) bool {
	return msg.NewChatMembers != nil || msg.LeftChatMember != nil
}
