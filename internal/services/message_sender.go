package services

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
)

type MessageSender interface {
	Send(chatId int64, text string, entities []gotgbot.MessageEntity, originalMessage *gotgbot.Message) (*gotgbot.Message, error)
}

type TelegramMessageSender struct {
	bot *gotgbot.Bot
}

func NewMessageSender(bot *gotgbot.Bot) MessageSender {
	return &TelegramMessageSender{bot: bot}
}

func (s *TelegramMessageSender) Send(chatId int64, text string, entities []gotgbot.MessageEntity, originalMessage *gotgbot.Message) (*gotgbot.Message, error) {
	if originalMessage.Animation != nil {
		return s.bot.SendAnimation(chatId, gotgbot.InputFileByID(originalMessage.Animation.FileId), &gotgbot.SendAnimationOpts{
			Caption:         text,
			CaptionEntities: entities,
		})
	} else if originalMessage.Photo != nil {
		return s.bot.SendPhoto(chatId, gotgbot.InputFileByID(originalMessage.Photo[len(originalMessage.Photo)-1].FileId), &gotgbot.SendPhotoOpts{
			Caption:         text,
			CaptionEntities: entities,
		})
	} else if originalMessage.Video != nil {
		return s.bot.SendVideo(chatId, gotgbot.InputFileByID(originalMessage.Video.FileId), &gotgbot.SendVideoOpts{
			Caption:         text,
			CaptionEntities: entities,
		})
	} else {
		return s.bot.SendMessage(chatId, text, &gotgbot.SendMessageOpts{
			Entities: entities,
		})
	}
}
