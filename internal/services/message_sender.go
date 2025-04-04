package services

import (
	"log"
	"strconv"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

type MessageSenderService interface {
	SendLoggedMarkdownMessage(chatId int64, text string, err error)
	SendCopy(
		chatId int64,
		topicId *int,
		text string,
		entities []gotgbot.MessageEntity,
		originalMessage *gotgbot.Message,
	) (*gotgbot.Message, error)
	SendTypingAction(chatId int64) error
	SendMessageToUser(
		userId int64,
		text string,
		opts *gotgbot.SendMessageOpts,
	) (*gotgbot.Message, error)
}

type TelegramMessageSender struct {
	bot *gotgbot.Bot
}

func NewMessageSenderService(bot *gotgbot.Bot) MessageSenderService {
	return &TelegramMessageSender{bot: bot}
}

// SendLoggedReply sends a reply to the user with proper logging
func (s *TelegramMessageSender) SendLoggedMarkdownMessage(chatId int64, text string, err error) {
	if _, replyErr := s.bot.SendMessage(chatId, text, &gotgbot.SendMessageOpts{
		ParseMode: "Markdown",
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
			IsDisabled: true,
		},
	}); replyErr != nil {
		log.Printf("Failed to send error message: %v", replyErr)
	}
	if err != nil {
		log.Printf("Error: %v", err)
	}
}

// Sends a copy of the original message to the chat
func (s *TelegramMessageSender) SendCopy(
	chatId int64,
	topicId *int,
	text string,
	entities []gotgbot.MessageEntity,
	originalMessage *gotgbot.Message,

) (*gotgbot.Message, error) {
	var opts interface{}
	var method func(int64, interface{}, interface{}) (*gotgbot.Message, error)

	//Prepare caption
	var caption string
	var trimmedPartOfCaption = ""
	var captionEntities []gotgbot.MessageEntity
	var trimmedPartOfCaptionEntities []gotgbot.MessageEntity
	if originalMessage != nil {
		if utf16CodeUnitCount(text) > 1000 {
			caption = CutStringByUTF16Units(text, 996)
			trimmedPartOfCaption = "..." + text[len(caption):]

			// Adjust entities for the caption
			for _, entity := range originalMessage.CaptionEntities {
				// Entity ends before the cutoff
				if entity.Offset+entity.Length <= 996 {
					captionEntities = append(captionEntities, entity)
				} else if entity.Offset < 996 {
					// Entity spans the cutoff; adjust the length
					clippedEntity := entity
					clippedEntity.Length = 996 - entity.Offset
					captionEntities = append(captionEntities, clippedEntity)
				}
			}

			// Adjust entities for the trimmed part
			for _, entity := range originalMessage.CaptionEntities {
				if entity.Offset+entity.Length > 996 {
					offsetInTrimmed := entity.Offset - int64(utf16CodeUnitCount(caption)) + 3 // +3 for "..."
					trimmedEntity := gotgbot.MessageEntity{
						Type:   entity.Type,
						Offset: offsetInTrimmed,
						Length: entity.Length,
						Url:    entity.Url,
					}
					trimmedPartOfCaptionEntities = append(trimmedPartOfCaptionEntities, trimmedEntity)
				}
			}
		} else {
			// Set caption even when text is within limits
			caption = text
			captionEntities = entities
		}
	}

	if originalMessage == nil {
		opts = &gotgbot.SendMessageOpts{Entities: entities}
		method = func(chatId int64, text interface{}, opts interface{}) (*gotgbot.Message, error) {
			return s.bot.SendMessage(chatId, text.(string), opts.(*gotgbot.SendMessageOpts))
		}
	} else if originalMessage.Animation != nil {
		opts = &gotgbot.SendAnimationOpts{Caption: caption, CaptionEntities: captionEntities}
		method = func(chatId int64, fileId interface{}, opts interface{}) (*gotgbot.Message, error) {
			return s.bot.SendAnimation(chatId, gotgbot.InputFileByID(originalMessage.Animation.FileId), opts.(*gotgbot.SendAnimationOpts))
		}
	} else if originalMessage.Photo != nil {
		opts = &gotgbot.SendPhotoOpts{Caption: caption, CaptionEntities: captionEntities}
		method = func(chatId int64, fileId interface{}, opts interface{}) (*gotgbot.Message, error) {
			return s.bot.SendPhoto(chatId, gotgbot.InputFileByID(originalMessage.Photo[len(originalMessage.Photo)-1].FileId), opts.(*gotgbot.SendPhotoOpts))
		}
	} else if originalMessage.Video != nil {
		opts = &gotgbot.SendVideoOpts{Caption: caption, CaptionEntities: captionEntities}
		method = func(chatId int64, fileId interface{}, opts interface{}) (*gotgbot.Message, error) {
			return s.bot.SendVideo(chatId, gotgbot.InputFileByID(originalMessage.Video.FileId), opts.(*gotgbot.SendVideoOpts))
		}
	} else {
		opts = &gotgbot.SendMessageOpts{Entities: entities}
		method = func(chatId int64, text interface{}, opts interface{}) (*gotgbot.Message, error) {
			return s.bot.SendMessage(chatId, text.(string), opts.(*gotgbot.SendMessageOpts))
		}
	}

	if topicId != nil {
		switch o := opts.(type) {
		case *gotgbot.SendMessageOpts:
			o.MessageThreadId = int64(*topicId)
		case *gotgbot.SendAnimationOpts:
			o.MessageThreadId = int64(*topicId)
		case *gotgbot.SendPhotoOpts:
			o.MessageThreadId = int64(*topicId)
		case *gotgbot.SendVideoOpts:
			o.MessageThreadId = int64(*topicId)
		}
	}

	sentMessage, err := method(chatId, text, opts)
	if err != nil {
		return nil, err
	}
	// send trimmed part of caption as a separate message ((works only for SaveHandler))
	if trimmedPartOfCaption != "" &&
		originalMessage != nil &&
		(originalMessage.Animation != nil ||
			originalMessage.Photo != nil ||
			originalMessage.Video != nil) {
		_, err := s.bot.SendMessage(chatId, trimmedPartOfCaption, &gotgbot.SendMessageOpts{Entities: trimmedPartOfCaptionEntities})
		if err != nil {
			return sentMessage, err
		}
	}

	return sentMessage, nil
}

// CutStringByUTF16Units cuts the string s so that its length in UTF-16 code units is at most limit.
// It returns the prefix of s that satisfies this condition.
func CutStringByUTF16Units(s string, limit int) string {
	var cuCount int   // Cumulative UTF-16 code units
	var byteIndex int // Byte index in the string
	for i, r := range s {
		// Determine the number of UTF-16 code units for this rune
		cuLen := 0
		if r <= 0xFFFF {
			cuLen = 1
		} else {
			cuLen = 2
		}

		// Check if adding this rune exceeds the limit
		if cuCount+cuLen > limit {
			break
		}

		cuCount += cuLen
		byteIndex = i + len(string(r))
	}

	return s[:byteIndex]
}

func utf16CodeUnitCount(s string) int {
	count := 0
	for _, r := range s {
		if r <= 0xFFFF {
			count += 1
		} else {
			count += 2
		}
	}
	return count
}

// SendTypingAction sends a typing action to the specified chat.
func (s *TelegramMessageSender) SendTypingAction(chatId int64) error {
	_, err := s.bot.Request("sendChatAction", map[string]string{
		"chat_id": strconv.FormatInt(chatId, 10),
		"action":  "typing",
	}, nil, nil)
	if err != nil {
		log.Printf("failed to send typing action: %v", err)
	}
	return err
}

// SendMessageToUser sends a text message to a specific user by their user ID
func (s *TelegramMessageSender) SendMessageToUser(
	userId int64,
	text string,
	opts *gotgbot.SendMessageOpts,
) (*gotgbot.Message, error) {
	if opts == nil {
		opts = &gotgbot.SendMessageOpts{
			ParseMode: "Markdown",
		}
	}

	sentMessage, err := s.bot.SendMessage(userId, text, opts)
	if err != nil {
		log.Printf("failed to send message to user %d: %v", userId, err)
		return nil, err
	}

	return sentMessage, nil
}
