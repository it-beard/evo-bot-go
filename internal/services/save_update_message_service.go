package services

import (
	"database/sql"
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/utils"
	"fmt"
	"log"

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

// SaveMessage processes regular user messages and saves them to database
func (s *SaveUpdateMessageService) SaveMessage(msg *gotgbot.Message) error {
	// Extract message content and convert to HTML
	markdownText := s.extractAndFormatMessageContent(msg)

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
	// Save the message
	_, err = s.groupMessageRepository.Create(
		msg.MessageId,
		markdownText,
		replyToMessageID,
		msg.From.Id,
		groupTopicID,
	)
	if err != nil {
		return fmt.Errorf("%s: failed to save group message: %w", utils.GetCurrentTypeName(), err)
	}

	return nil
}

// UpdateMessage processes edited messages
func (s *SaveUpdateMessageService) UpdateMessage(msg *gotgbot.Message) error {
	// Check if the message indicates deletion first
	messageText := s.getMessageText(msg)
	if s.isMessageForDeletion(messageText) {
		return s.handleMessageDeletion(msg)
	}

	// Update the message in the database
	return s.handleMessageUpdate(msg)
}

// handleMessageUpdate updates an existing message in the database
func (s *SaveUpdateMessageService) handleMessageUpdate(msg *gotgbot.Message) error {
	// Get existing message from database
	existingMessage, err := s.groupMessageRepository.GetByMessageID(msg.MessageId)
	if err != nil {
		if err == sql.ErrNoRows {
			// If message doesn't exist, save it as a new message
			log.Printf("%s: Edited message not found in database, saving as new message - ID: %d",
				utils.GetCurrentTypeName(), msg.MessageId)
			return s.SaveMessage(msg)
		}
		return fmt.Errorf("%s: failed to get existing message: %w", utils.GetCurrentTypeName(), err)
	}

	// Extract updated message content and convert to HTML
	markdownText := s.extractAndFormatMessageContent(msg)

	// Only update if content actually changed
	if markdownText != existingMessage.MessageText {
		// Update the message
		err = s.groupMessageRepository.Update(existingMessage.ID, markdownText)
		if err != nil {
			return fmt.Errorf("%s: failed to update group message: %w", utils.GetCurrentTypeName(), err)
		}

		log.Printf("%s: Successfully updated group message - ID: %d, User: %d",
			utils.GetCurrentTypeName(), msg.MessageId, msg.From.Id)
	}

	return nil
}

// handleMessageDeletion deletes a message from both Telegram and the database
func (s *SaveUpdateMessageService) handleMessageDeletion(msg *gotgbot.Message) error {
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

// isMessageForDeletion checks if a message indicates deletion
func (s *SaveUpdateMessageService) isMessageForDeletion(messageText string) bool {
	return messageText == "[delete]" || messageText == "[удалить]"
}
