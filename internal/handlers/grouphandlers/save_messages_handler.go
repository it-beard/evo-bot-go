package grouphandlers

import (
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
}

func NewSaveMessagesHandler(
	groupMessageRepository *repositories.GroupMessageRepository,
	userRepository *repositories.UserRepository,
) ext.Handler {
	h := &SaveMessagesHandler{
		groupMessageRepository: groupMessageRepository,
		userRepository:         userRepository,
	}
	return handlers.NewMessage(h.check, h.handle)
}

func (h *SaveMessagesHandler) check(msg *gotgbot.Message) bool {
	if msg == nil {
		return false
	}

	// Handle regular user messages (text, voice, photo, etc.)
	// Skip bot messages and system messages
	if msg.From == nil || msg.From.IsBot {
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
	msg := ctx.EffectiveMessage

	// Handle regular user messages
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
	markdownText := utils.ConvertToMarkdown(messageText, entities)

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

	log.Printf("%s: Successfully saved group message - ID: %d, User: %d, Thread: %v",
		utils.GetCurrentTypeName(), msg.MessageId, msg.From.Id, groupTopicID)

	return nil
}
