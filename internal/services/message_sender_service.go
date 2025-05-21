package services

import (
	"evo-bot-go/internal/utils"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

type MessageSenderService struct {
	bot *gotgbot.Bot
}

func NewMessageSenderService(bot *gotgbot.Bot) *MessageSenderService {
	return &MessageSenderService{bot: bot}
}

// Send message to chat
func (s *MessageSenderService) SendWithReturnMessage(chatId int64, text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
	// default link preview options are disabled
	if opts == nil {
		opts = &gotgbot.SendMessageOpts{
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
				IsDisabled: true,
			},
		}
	} else if opts.LinkPreviewOptions == nil {
		opts.LinkPreviewOptions = &gotgbot.LinkPreviewOptions{
			IsDisabled: true,
		}
	}

	sentMsg, err := s.bot.SendMessage(chatId, text, opts)

	if err != nil {
		// If topic is closed, try to reopen it and send again
		if s.isTopicClosedError(err) && opts != nil && opts.MessageThreadId != 0 {
			sentMsg, err = s.handleClosedTopicReturnMessage(chatId, text, opts, "SendWithReturnMessage", err)
		}

		if err != nil {
			log.Printf("%s: Send: Failed to send message: %v", utils.GetCurrentTypeName(), err)
		}
	}

	return sentMsg, err
}

// Send message to chat
func (s *MessageSenderService) Send(chatId int64, text string, opts *gotgbot.SendMessageOpts) error {
	_, err := s.SendWithReturnMessage(chatId, text, opts)
	return err
}

// Send markdown message to chat and return the sent message
func (s *MessageSenderService) SendMarkdownWithReturnMessage(chatId int64, text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
	// default options for markdown messages
	if opts == nil {
		opts = &gotgbot.SendMessageOpts{
			ParseMode: "Markdown",
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
				IsDisabled: true,
			},
		}
	} else {
		opts.ParseMode = "Markdown"
		// default link preview options are disabled
		if opts.LinkPreviewOptions == nil {
			opts.LinkPreviewOptions = &gotgbot.LinkPreviewOptions{
				IsDisabled: true,
			}
		}
	}

	sentMsg, err := s.bot.SendMessage(chatId, text, opts)

	if err != nil {
		// If topic is closed, try to reopen it and send again
		if s.isTopicClosedError(err) && opts != nil && opts.MessageThreadId != 0 {
			sentMsg, err = s.handleClosedTopicReturnMessage(chatId, text, opts, "SendMarkdownWithReturnMessage", err)
		}

		if err != nil {
			log.Printf("%s: SendMarkdownWithReturnMessage: Failed to send message: %v", utils.GetCurrentTypeName(), err)
		}
	}

	return sentMsg, err
}

// Send markdown message to chat
func (s *MessageSenderService) SendMarkdown(chatId int64, text string, opts *gotgbot.SendMessageOpts) error {
	_, err := s.SendMarkdownWithReturnMessage(chatId, text, opts)
	return err
}

// Send html message to chat
func (s *MessageSenderService) SendHtmlWithReturnMessage(chatId int64, text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
	// default options for html messages
	if opts == nil {
		opts = &gotgbot.SendMessageOpts{
			ParseMode: "HTML",
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
				IsDisabled: true,
			},
		}
	} else {
		opts.ParseMode = "HTML"
		// default link preview options are disabled
		if opts.LinkPreviewOptions == nil {
			opts.LinkPreviewOptions = &gotgbot.LinkPreviewOptions{
				IsDisabled: true,
			}
		}
	}

	sentMsg, err := s.bot.SendMessage(chatId, text, opts)

	if err != nil {
		// If topic is closed, try to reopen it and send again
		if s.isTopicClosedError(err) && opts != nil && opts.MessageThreadId != 0 {
			sentMsg, err = s.handleClosedTopicReturnMessage(chatId, text, opts, "SendHtml", err)
		}

		if err != nil {
			log.Printf("%s: SendHtml: Failed to send message: %v", utils.GetCurrentTypeName(), err)
		}
	}

	return sentMsg, err
}

func (s *MessageSenderService) SendHtml(chatId int64, text string, opts *gotgbot.SendMessageOpts) error {
	_, err := s.SendHtmlWithReturnMessage(chatId, text, opts)
	return err
}

// Reply to a message
func (s *MessageSenderService) Reply(msg *gotgbot.Message, replyText string, opts *gotgbot.SendMessageOpts) error {
	_, err := s.ReplyWithReturnMessage(msg, replyText, opts)
	return err
}

// Reply to a message
func (s *MessageSenderService) ReplyWithReturnMessage(msg *gotgbot.Message, replyText string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
	// default link preview options are disabled
	if opts == nil {
		opts = &gotgbot.SendMessageOpts{
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
				IsDisabled: true,
			},
		}
	} else if opts.LinkPreviewOptions == nil {
		opts.LinkPreviewOptions = &gotgbot.LinkPreviewOptions{
			IsDisabled: true,
		}
	}

	sentMsg, err := msg.Reply(s.bot, replyText, opts)

	if err != nil {
		log.Printf("%s: Reply: Failed to send message: %v", utils.GetCurrentTypeName(), err)
	}

	return sentMsg, err
}

// Reply to a message with markdown
func (s *MessageSenderService) ReplyMarkdown(msg *gotgbot.Message, replyText string, opts *gotgbot.SendMessageOpts) error {
	_, err := s.ReplyMarkdownWithReturnMessage(msg, replyText, opts)
	return err
}

// Reply to a message with markdown
func (s *MessageSenderService) ReplyMarkdownWithReturnMessage(msg *gotgbot.Message, replyText string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
	// default options for markdown messages
	if opts == nil {
		opts = &gotgbot.SendMessageOpts{
			ParseMode: "Markdown",
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
				IsDisabled: true,
			},
		}
	} else {
		opts.ParseMode = "Markdown"
		// default link preview options are disabled
		if opts.LinkPreviewOptions == nil {
			opts.LinkPreviewOptions = &gotgbot.LinkPreviewOptions{
				IsDisabled: true,
			}
		}
	}

	sentMsg, err := msg.Reply(s.bot, replyText, opts)

	if err != nil {
		log.Printf("%s: ReplyMarkdown: Failed to send message: %v", utils.GetCurrentTypeName(), err)
	}

	return sentMsg, err
}

// Reply to a message with html
func (s *MessageSenderService) ReplyHtml(msg *gotgbot.Message, replyText string, opts *gotgbot.SendMessageOpts) error {
	// default options for html messages
	if opts == nil {
		opts = &gotgbot.SendMessageOpts{
			ParseMode: "HTML",
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
				IsDisabled: true,
			},
		}
	} else {
		opts.ParseMode = "HTML"
		// default link preview options are disabled
		if opts.LinkPreviewOptions == nil {
			opts.LinkPreviewOptions = &gotgbot.LinkPreviewOptions{
				IsDisabled: true,
			}
		}
	}

	_, err := msg.Reply(s.bot, replyText, opts)

	if err != nil {
		log.Printf("%s: ReplyHtml: Failed to send message: %v", utils.GetCurrentTypeName(), err)
	}

	return err
}

// ReplyWithCleanupAfterDelay replies to a message and then deletes both the reply and the original message after the specified delay
// Returns the sent message and any error that occurred during sending
func (s *MessageSenderService) ReplyWithCleanupAfterDelayWithPing(
	msg *gotgbot.Message,
	text string,
	delaySeconds int,
	opts *gotgbot.SendMessageOpts,
) error {
	sentMsg, err := msg.Reply(s.bot, text, opts)
	if err != nil {
		log.Printf("Failed to send reply: %v", err)
		return err
	}

	// Send a random greeting message
	greetings := []string{
		"Ping!",
		"Hi!",
		"Ку!",
		"Приветы!",
		"Дзень добры!",
		"Пинг!",
	}
	randomGreeting := greetings[rand.Intn(len(greetings))]
	_, err = s.bot.SendMessage(msg.From.Id, randomGreeting, nil)
	if err != nil {
		log.Printf("Failed to send greeting message: %v", err)
	}

	// Start a goroutine to delete both messages after the delay
	go func() {
		time.Sleep(time.Duration(delaySeconds) * time.Second)

		// Delete the reply message
		_, replyErr := sentMsg.Delete(s.bot, nil)
		if replyErr != nil {
			log.Printf("Failed to delete reply message after delay: %v", replyErr)
		}

		// Delete the original message
		_, origErr := msg.Delete(s.bot, nil)
		if origErr != nil {
			log.Printf("Failed to delete original message after delay: %v", origErr)
		}
	}()

	return err
}

// Sends a copy of the original message to the chat
func (s *MessageSenderService) SendCopy(
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
		if utils.Utf16CodeUnitCount(text) > 1000 {
			caption = utils.CutStringByUTF16Units(text, 996)
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
					offsetInTrimmed := entity.Offset - int64(utils.Utf16CodeUnitCount(caption)) + 3 // +3 for "..."
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

// SendTypingAction sends a typing action to the specified chat.
func (s *MessageSenderService) SendTypingAction(chatId int64) error {
	_, err := s.bot.Request("sendChatAction", map[string]string{
		"chat_id": strconv.FormatInt(chatId, 10),
		"action":  "typing",
	}, nil, nil)
	if err != nil {
		log.Printf("%s: SendTypingAction: Failed to send typing action: %v", utils.GetCurrentTypeName(), err)
	}
	return err
}

// RemoveInlineKeyboard removes the inline keyboard from a message
func (s *MessageSenderService) RemoveInlineKeyboard(chatID int64, messageID int64) error {
	if chatID == 0 || messageID == 0 {
		return nil
	}

	_, _, err := s.bot.EditMessageReplyMarkup(&gotgbot.EditMessageReplyMarkupOpts{
		ChatId:      chatID,
		MessageId:   messageID,
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{},
	})

	if err != nil {
		log.Printf("%s: Error removing inline keyboard: %v", utils.GetCurrentTypeName(), err)
	}

	return err
}

// PinMessageWithNotification pins a message with optional notification to all users
func (s *MessageSenderService) PinMessageWithNotification(chatID int64, messageID int64, disableNotification bool) error {
	_, err := s.bot.PinChatMessage(chatID, messageID, &gotgbot.PinChatMessageOpts{
		DisableNotification: disableNotification,
	})

	if err != nil {
		log.Printf("%s: Error pinning message: %v", utils.GetCurrentTypeName(), err)
	}

	return err
}

func (s *MessageSenderService) isTopicClosedError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "TOPIC_CLOSED")
}

func (s *MessageSenderService) handleClosedTopic(chatId int64, text string, opts *gotgbot.SendMessageOpts, methodName string, originalErr error) error {
	_, err := s.handleClosedTopicReturnMessage(chatId, text, opts, methodName, originalErr)
	return err
}

func (s *MessageSenderService) handleClosedTopicReturnMessage(chatId int64, text string, opts *gotgbot.SendMessageOpts, methodName string, originalErr error) (*gotgbot.Message, error) {
	log.Printf("%s: %s: Topic is closed, attempting to reopen it", utils.GetCurrentTypeName(), methodName)

	// Try to reopen the topic
	_, reopenErr := s.bot.ReopenForumTopic(chatId, opts.MessageThreadId, nil)
	if reopenErr != nil {
		log.Printf("%s: %s: Failed to reopen topic: %v", utils.GetCurrentTypeName(), methodName, reopenErr)
		return nil, fmt.Errorf("failed to reopen topic and send message: %w", originalErr)
	}

	// Try sending the message again
	sentMsg, err := s.bot.SendMessage(chatId, text, opts)

	// Close the topic again to maintain its original state
	_, closeErr := s.bot.CloseForumTopic(chatId, opts.MessageThreadId, nil)
	if closeErr != nil {
		log.Printf("%s: %s: Warning: Failed to close topic after sending message: %v", utils.GetCurrentTypeName(), methodName, closeErr)
		// We don't return an error here as the message was sent successfully
	} else {
		log.Printf("%s: %s: Topic closed successfully after sending message", utils.GetCurrentTypeName(), methodName)
	}

	if err != nil {
		log.Printf("%s: %s: Failed to send message after reopening topic: %v", utils.GetCurrentTypeName(), methodName, err)
		return nil, err
	}
	return sentMsg, nil
}
