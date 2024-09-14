package services

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
)

type MessageSender interface {
	SendCopy(
		chatId int64,
		threadId *int64,
		text string,
		entities []gotgbot.MessageEntity,
		originalMessage *gotgbot.Message,
	) (*gotgbot.Message, error)
}

type TelegramMessageSender struct {
	bot *gotgbot.Bot
}

func NewMessageSender(bot *gotgbot.Bot) MessageSender {
	return &TelegramMessageSender{bot: bot}
}

// Sends a copy of the original message to the chat
func (s *TelegramMessageSender) SendCopy(
	chatId int64,
	threadId *int64,
	text string,
	entities []gotgbot.MessageEntity,
	originalMessage *gotgbot.Message,

) (*gotgbot.Message, error) {
	var opts interface{}
	var method func(int64, interface{}, interface{}) (*gotgbot.Message, error)

	if originalMessage == nil {
		opts = &gotgbot.SendMessageOpts{Entities: entities}
		method = func(chatId int64, text interface{}, opts interface{}) (*gotgbot.Message, error) {
			return s.bot.SendMessage(chatId, text.(string), opts.(*gotgbot.SendMessageOpts))
		}
	} else if originalMessage.Animation != nil {
		opts = &gotgbot.SendAnimationOpts{Caption: text, CaptionEntities: entities}
		method = func(chatId int64, fileId interface{}, opts interface{}) (*gotgbot.Message, error) {
			return s.bot.SendAnimation(chatId, gotgbot.InputFileByID(originalMessage.Animation.FileId), opts.(*gotgbot.SendAnimationOpts))
		}
	} else if originalMessage.Photo != nil {
		opts = &gotgbot.SendPhotoOpts{Caption: text, CaptionEntities: entities}
		method = func(chatId int64, fileId interface{}, opts interface{}) (*gotgbot.Message, error) {
			return s.bot.SendPhoto(chatId, gotgbot.InputFileByID(originalMessage.Photo[len(originalMessage.Photo)-1].FileId), opts.(*gotgbot.SendPhotoOpts))
		}
	} else if originalMessage.Video != nil {
		opts = &gotgbot.SendVideoOpts{Caption: text, CaptionEntities: entities}
		method = func(chatId int64, fileId interface{}, opts interface{}) (*gotgbot.Message, error) {
			return s.bot.SendVideo(chatId, gotgbot.InputFileByID(originalMessage.Video.FileId), opts.(*gotgbot.SendVideoOpts))
		}
	} else {
		opts = &gotgbot.SendMessageOpts{Entities: entities}
		method = func(chatId int64, text interface{}, opts interface{}) (*gotgbot.Message, error) {
			return s.bot.SendMessage(chatId, text.(string), opts.(*gotgbot.SendMessageOpts))
		}
	}

	if threadId != nil {
		switch o := opts.(type) {
		case *gotgbot.SendMessageOpts:
			o.MessageThreadId = *threadId
		case *gotgbot.SendAnimationOpts:
			o.MessageThreadId = *threadId
		case *gotgbot.SendPhotoOpts:
			o.MessageThreadId = *threadId
		case *gotgbot.SendVideoOpts:
			o.MessageThreadId = *threadId
		}
	}

	return method(chatId, text, opts)
}
