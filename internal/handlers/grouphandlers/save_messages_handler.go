package grouphandlers

import (
	"database/sql"
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/utils"
	"fmt"
	"log"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

type SaveMessagesHandler struct {
	groupMessageRepository *repositories.GroupMessageRepository
	userRepository         *repositories.UserRepository
	config                 *config.Config
	bot                    *gotgbot.Bot
}

func NewSaveMessagesHandler(
	groupMessageRepository *repositories.GroupMessageRepository,
	userRepository *repositories.UserRepository,
	config *config.Config,
	bot *gotgbot.Bot,
) ext.Handler {
	h := &SaveMessagesHandler{
		groupMessageRepository: groupMessageRepository,
		userRepository:         userRepository,
		config:                 config,
		bot:                    bot,
	}
	return handlers.NewMessage(h.check, h.handle).SetAllowEdited(true)
}

func (h *SaveMessagesHandler) check(msg *gotgbot.Message) bool {
	if msg == nil {
		return false
	}

	// Skip private chats
	if msg.Chat.Type == constants.PrivateChatType {
		return false
	}

	// Skip forum topic created or edited messages (handled by SaveTopicsHandler)
	if msg.ForumTopicCreated != nil || msg.ForumTopicEdited != nil {
		return false
	}

	// Check if this is a regular message with content
	return msg.Text != "" || msg.Caption != "" || msg.Voice != nil || msg.Audio != nil ||
		msg.Document != nil || msg.Photo != nil || msg.Video != nil || msg.VideoNote != nil ||
		msg.Sticker != nil || msg.Animation != nil
}

func (h *SaveMessagesHandler) handle(b *gotgbot.Bot, ctx *ext.Context) error {
	// Check if this is an edited message
	if ctx.Update.EditedMessage != nil {
		return h.handleEditedMessage(ctx.Update.EditedMessage)
	}

	// Handle regular new messages
	msg := ctx.EffectiveMessage
	return h.handleUserMessage(msg)
}

// handleUserMessage processes regular user messages and saves them to database
func (h *SaveMessagesHandler) handleUserMessage(msg *gotgbot.Message) error {
	// Get message content
	messageText := ""
	entities := []gotgbot.MessageEntity{}

	if msg.Text != "" {
		messageText = msg.Text
		entities = msg.Entities
	} else if msg.Caption != "" {
		messageText = msg.Caption
		entities = msg.CaptionEntities
	} else {
		// For media messages without caption, create a descriptive text
		if msg.Photo != nil {
			messageText = "[Photo]"
		} else if msg.Video != nil {
			messageText = "[Video]"
		} else if msg.Voice != nil {
			messageText = "[Voice message]"
		} else if msg.Audio != nil {
			messageText = "[Audio]"
		} else if msg.Document != nil {
			messageText = "[Document]"
		} else if msg.VideoNote != nil {
			messageText = "[Video note]"
		} else if msg.Sticker != nil {
			messageText = "[Sticker]"
		} else if msg.Animation != nil {
			messageText = "[GIF]"
		} else {
			messageText = "[Media]"
		}
	}

	// Convert to markdown
	markdownText := utils.ConvertToHTML(messageText, entities)

	// Get replied message ID if exists
	var replyToMessageID *int64
	if msg.ReplyToMessage != nil {
		repliedID := int64(msg.ReplyToMessage.MessageId)
		replyToMessageID = &repliedID
	}

	// Get thread ID
	var groupTopicID int64
	if msg.MessageThreadId != 0 {
		groupTopicID = msg.MessageThreadId
		if !msg.IsTopicMessage {
			groupTopicID = 0 // If message is not a topic message, set thread ID to 0
		}
	}

	// If replied message ID is the same as thread ID, set replied message ID to nil
	if replyToMessageID != nil && *replyToMessageID == groupTopicID {
		replyToMessageID = nil
	}

	// Check if user exists and create/update if needed
	_, err := h.userRepository.GetOrCreate(msg.From)
	if err != nil {
		log.Printf("%s: failed to get or create user %d: %v", utils.GetCurrentTypeName(), msg.From.Id, err)
		// Continue even if user operations fail, but log the error
	}

	// FromId for "GroupAnonymousBot" should be admin
	if msg.From.IsBot && msg.From.Username == "GroupAnonymousBot" {
		msg.From.Id = h.config.AdminUserID
	}
	// Save the message
	_, err = h.groupMessageRepository.Create(
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

// handleEditedMessage processes edited messages
func (h *SaveMessagesHandler) handleEditedMessage(msg *gotgbot.Message) error {
	// Check if the message indicates deletion first
	messageText := h.getMessageText(msg)
	if h.isMessageForDeletion(messageText) {
		return h.handleMessageDeletion(msg)
	}

	// Update the message in the database
	return h.handleMessageUpdate(msg)
}

// handleMessageUpdate updates an existing message in the database
func (h *SaveMessagesHandler) handleMessageUpdate(msg *gotgbot.Message) error {
	// Get existing message from database
	existingMessage, err := h.groupMessageRepository.GetByMessageID(msg.MessageId)
	if err != nil {
		if err == sql.ErrNoRows {
			// If message doesn't exist, save it as a new message
			log.Printf("%s: Edited message not found in database, saving as new message - ID: %d",
				utils.GetCurrentTypeName(), msg.MessageId)
			return h.handleUserMessage(msg)
		}
		return fmt.Errorf("%s: failed to get existing message: %w", utils.GetCurrentTypeName(), err)
	}

	// Get updated message content
	messageText := ""
	entities := []gotgbot.MessageEntity{}

	if msg.Text != "" {
		messageText = msg.Text
		entities = msg.Entities
	} else if msg.Caption != "" {
		messageText = msg.Caption
		entities = msg.CaptionEntities
	} else {
		// For media messages without caption, keep the descriptive text
		if msg.Photo != nil {
			messageText = "[Photo]"
		} else if msg.Video != nil {
			messageText = "[Video]"
		} else if msg.Voice != nil {
			messageText = "[Voice message]"
		} else if msg.Audio != nil {
			messageText = "[Audio]"
		} else if msg.Document != nil {
			messageText = "[Document]"
		} else if msg.VideoNote != nil {
			messageText = "[Video note]"
		} else if msg.Sticker != nil {
			messageText = "[Sticker]"
		} else if msg.Animation != nil {
			messageText = "[GIF]"
		} else {
			messageText = "[Media]"
		}
	}

	// Convert to markdown
	markdownText := utils.ConvertToHTML(messageText, entities)

	// Only update if content actually changed
	if markdownText != existingMessage.MessageText {
		// Update the message
		err = h.groupMessageRepository.Update(existingMessage.ID, markdownText)
		if err != nil {
			return fmt.Errorf("%s: failed to update group message: %w", utils.GetCurrentTypeName(), err)
		}

		log.Printf("%s: Successfully updated group message - ID: %d, User: %d",
			utils.GetCurrentTypeName(), msg.MessageId, msg.From.Id)
	}

	return nil
}

// getMessageText extracts text content from a message
func (h *SaveMessagesHandler) getMessageText(msg *gotgbot.Message) string {
	if msg.Text != "" {
		return msg.Text
	}
	if msg.Caption != "" {
		return msg.Caption
	}
	return ""
}

// isMessageForDeletion checks if a message indicates deletion
func (h *SaveMessagesHandler) isMessageForDeletion(messageText string) bool {
	return messageText == "[delete]" || messageText == "[удалить]"
}

// handleMessageDeletion deletes a message from both Telegram and the database
func (h *SaveMessagesHandler) handleMessageDeletion(msg *gotgbot.Message) error {
	// First, get the existing message from database to get the internal ID
	existingMessage, err := h.groupMessageRepository.GetByMessageID(msg.MessageId)
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
	_, err = h.bot.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)
	if err != nil {
		log.Printf("%s: Failed to delete message from Telegram - ID: %d, Error: %v",
			utils.GetCurrentTypeName(), msg.MessageId, err)
		// Continue with database deletion even if Telegram deletion fails
	}

	// Delete from database if we found it
	if existingMessage != nil {
		err = h.groupMessageRepository.Delete(existingMessage.ID)
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
