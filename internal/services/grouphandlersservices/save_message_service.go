package grouphandlersservices

import (
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type SaveMessageService struct {
	groupMessageRepository   *repositories.GroupMessageRepository
	saveUpdateMessageService *SaveUpdateMessageService
}

func NewSaveMessageService(
	groupMessageRepository *repositories.GroupMessageRepository,
	saveUpdateMessageService *SaveUpdateMessageService,
) *SaveMessageService {
	return &SaveMessageService{
		groupMessageRepository:   groupMessageRepository,
		saveUpdateMessageService: saveUpdateMessageService,
	}
}

func (s *SaveMessageService) SaveOrUpdateMessage(ctx *ext.Context) error {
	// Handle edited messages
	updatedMessage := ctx.Update.EditedMessage
	if ctx.Update.EditedMessage != nil {
		if s.isMessageForDeletion(updatedMessage.Text) {
			return s.saveUpdateMessageService.Delete(updatedMessage)
		} else {
			return s.saveUpdateMessageService.SaveOrUpdate(updatedMessage)
		}
	}

	// Handle regular new messages
	msg := ctx.EffectiveMessage
	return s.saveUpdateMessageService.Save(msg)
}

func (s *SaveMessageService) IsMessageShouldBeSavedOrUpdated(msg *gotgbot.Message) bool {
	// Check if this is a regular message with content
	return msg.Text != "" || msg.Caption != "" || msg.Voice != nil || msg.Audio != nil ||
		msg.Document != nil || msg.Photo != nil || msg.Video != nil || msg.VideoNote != nil ||
		msg.Sticker != nil || msg.Animation != nil
}

func (s *SaveMessageService) isMessageForDeletion(messageText string) bool {
	return messageText == constants.ServiceSaveMessage_MessageDeleteEnCommand ||
		messageText == constants.ServiceSaveMessage_MessageDeleteRuCommand
}
