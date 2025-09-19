package grouphandlersservices

import (
	"database/sql"
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/utils"
	"fmt"
	"log"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// SaveUpdateMessageService handles saving and updating messages in the database
type SaveUpdateMessageService struct {
	groupMessageRepository *repositories.GroupMessageRepository
	userRepository         *repositories.UserRepository
	config                 *config.Config
	bot                    *gotgbot.Bot
}

// NewSaveUpdateMessageService creates a new save update message service
func NewSaveUpdateMessageService(
	groupMessageRepository *repositories.GroupMessageRepository,
	userRepository *repositories.UserRepository,
	config *config.Config,
	bot *gotgbot.Bot,
) *SaveUpdateMessageService {
	return &SaveUpdateMessageService{
		groupMessageRepository: groupMessageRepository,
		userRepository:         userRepository,
		config:                 config,
		bot:                    bot,
	}
}
func (s *SaveUpdateMessageService) Save(msg *gotgbot.Message) error {
	return s.handleSaveOrUpdate(msg, true)
}

func (s *SaveUpdateMessageService) SaveOrUpdate(msg *gotgbot.Message) error {
	return s.handleSaveOrUpdate(msg, false)
}

// SaveOrUpdate saves a new message or updates an existing one in the database
func (s *SaveUpdateMessageService) handleSaveOrUpdate(msg *gotgbot.Message, isSaveOnly bool) error {
	// Extract message content and convert to HTML
	markdownText := s.extractAndFormatMessageContent(msg)

	if !isSaveOnly {
		// First, try to get the message from the database
		existingMessage, err := s.groupMessageRepository.GetByMessageID(msg.MessageId)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("%s: failed to get existing message: %w", utils.GetCurrentTypeName(), err)
		}

		// If message exists in database, update it
		if err != sql.ErrNoRows && existingMessage != nil {
			// Only update if content actually changed
			if markdownText != existingMessage.MessageText {
				err = s.groupMessageRepository.Update(existingMessage.ID, markdownText)
				if err != nil {
					return fmt.Errorf("%s: failed to update group message: %w", utils.GetCurrentTypeName(), err)
				}

				log.Printf("%s: Successfully updated group message - ID: %d, User: %d",
					utils.GetCurrentTypeName(), msg.MessageId, msg.From.Id)
			}
			return nil
		}

		// Message doesn't exist, save it as new
		log.Printf("%s: Message not found in database, saving as new message - ID: %d",
			utils.GetCurrentTypeName(), msg.MessageId)
	}

	// Get replied message ID if exists
	replyToMessageID := s.extractReplyToMessageID(msg)

	// Get thread ID
	groupTopicID := s.extractGroupTopicID(msg)

	// If replied message ID is the same as thread ID, set replied message ID to nil
	if replyToMessageID != nil && *replyToMessageID == groupTopicID {
		replyToMessageID = nil
	}

	// Check if user exists and create/update if needed
	_, err := s.userRepository.GetOrCreate(msg.From)
	if err != nil {
		log.Printf("%s: failed to get or create user %d: %v", utils.GetCurrentTypeName(), msg.From.Id, err)
		// Continue even if user operations fail, but log the error
	}

	// FromId for "GroupAnonymousBot" should be admin
	if msg.From.IsBot && msg.From.Username == "GroupAnonymousBot" {
		msg.From.Id = s.config.AdminUserID
	}

	// Save the message with original creation time from Telegram
	createdAt := time.Unix(int64(msg.Date), 0).UTC()
	_, err = s.groupMessageRepository.CreateWithCreatedAt(
		msg.MessageId,
		markdownText,
		replyToMessageID,
		msg.From.Id,
		groupTopicID,
		createdAt,
	)
	if err != nil {
		return fmt.Errorf("%s: failed to save group message: %w", utils.GetCurrentTypeName(), err)
	}

	return nil
}

// Delete deletes a message from both Telegram and the database
func (s *SaveUpdateMessageService) Delete(msg *gotgbot.Message) error {
	// First, get the existing message from database to get the internal ID
	existingMessage, err := s.groupMessageRepository.GetByMessageID(msg.MessageId)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("%s: Message not found in database for deletion - ID: %d",
				utils.GetCurrentTypeName(), msg.MessageId)
			// Still try to delete from Telegram even if not in database
		} else {
			return fmt.Errorf("%s: failed to get existing message for deletion: %w", utils.GetCurrentTypeName(), err)
		}
	}

	// Delete from Telegram first
	_, err = s.bot.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
	if err != nil {
		log.Printf("%s: Failed to delete message from Telegram - ID: %d, Error: %v",
			utils.GetCurrentTypeName(), msg.MessageId, err)
		// Continue with database deletion even if Telegram deletion fails
	}

	// Delete from database if we found it
	if existingMessage != nil {
		err = s.groupMessageRepository.Delete(existingMessage.ID)
		if err != nil {
			return fmt.Errorf("%s: failed to delete message from database: %w", utils.GetCurrentTypeName(), err)
		}
		log.Printf("%s: Successfully deleted message from database - Message ID: %d, Internal ID: %d",
			utils.GetCurrentTypeName(), msg.MessageId, existingMessage.ID)
	}

	log.Printf("%s: Successfully processed message deletion - Message ID: %d",
		utils.GetCurrentTypeName(), msg.MessageId)

	return nil
}

// extractAndFormatMessageContent extracts text and entities from a message and formats it as HTML
func (s *SaveUpdateMessageService) extractAndFormatMessageContent(msg *gotgbot.Message) string {
	messageText, entities := s.extractMessageTextAndEntities(msg)
	return utils.ConvertToHTML(messageText, entities)
}

// extractMessageTextAndEntities extracts text content and entities from a message
func (s *SaveUpdateMessageService) extractMessageTextAndEntities(msg *gotgbot.Message) (string, []gotgbot.MessageEntity) {
	if msg.Text != "" {
		return msg.Text, msg.Entities
	}
	if msg.Caption != "" {
		return msg.Caption, msg.CaptionEntities
	}
	return s.getMediaTypeDescription(msg), []gotgbot.MessageEntity{}
}

// getMediaTypeDescription returns a descriptive text for media messages without caption
func (s *SaveUpdateMessageService) getMediaTypeDescription(msg *gotgbot.Message) string {
	if msg.Photo != nil {
		return "[Photo]"
	}
	if msg.Video != nil {
		return "[Video]"
	}
	if msg.Voice != nil {
		return "[Voice message]"
	}
	if msg.Audio != nil {
		return "[Audio]"
	}
	if msg.Document != nil {
		return "[Document]"
	}
	if msg.VideoNote != nil {
		return "[Video note]"
	}
	if msg.Sticker != nil {
		return "[Sticker]"
	}
	if msg.Animation != nil {
		return "[GIF]"
	}
	return "[Media]"
}

// extractReplyToMessageID extracts reply-to message ID from a message
func (s *SaveUpdateMessageService) extractReplyToMessageID(msg *gotgbot.Message) *int64 {
	if msg.ReplyToMessage != nil {
		repliedID := int64(msg.ReplyToMessage.MessageId)
		return &repliedID
	}
	return nil
}

// extractGroupTopicID extracts group topic ID from a message
func (s *SaveUpdateMessageService) extractGroupTopicID(msg *gotgbot.Message) int64 {
	if msg.MessageThreadId != 0 && msg.IsTopicMessage {
		return msg.MessageThreadId
	}
	return 0
}

// getMessageText extracts text content from a message
func (s *SaveUpdateMessageService) getMessageText(msg *gotgbot.Message) string {
	messageText, _ := s.extractMessageTextAndEntities(msg)
	return messageText
}
