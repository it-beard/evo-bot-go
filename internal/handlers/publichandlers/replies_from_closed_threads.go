package publichandlers

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"unicode/utf8"
	"github.com/it-beard/evo-bot-go/internal/clients"
	"github.com/it-beard/evo-bot-go/internal/config"
	"github.com/it-beard/evo-bot-go/internal/constants"
	"github.com/it-beard/evo-bot-go/internal/handlers"
	"github.com/it-beard/evo-bot-go/internal/services"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type RepliesFromClosedThreadsHandler struct {
	closedThreads map[int64]bool
	messageSender services.MessageSender
	config        *config.Config
}

func NewRepliesFromClosedThreadsHandler(messageSender services.MessageSender, config *config.Config) handlers.Handler {
	// Create map of closed threads
	closedThreads := make(map[int64]bool)
	for _, id := range config.ClosedThreadsIDs {
		closedThreads[id] = true
	}

	return &RepliesFromClosedThreadsHandler{
		closedThreads: closedThreads,
		messageSender: messageSender,
		config:        config,
	}
}

func (h *RepliesFromClosedThreadsHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	if msg.ReplyToMessage != nil &&
		h.closedThreads[msg.MessageThreadId] &&
		msg.ReplyToMessage.MessageId != msg.MessageThreadId {
		if err := h.forwardReplyMessage(ctx); err != nil {
			log.Printf(
				"%s: error >> failed to forward reply message: %v",
				constants.RepliesFromClosedThreadsHandlerName,
				err)
		}
		_, err := msg.Delete(b, nil)
		if err != nil {
			log.Printf(
				"%s: error >> failed to delete original message after forwarding: %v",
				constants.RepliesFromClosedThreadsHandlerName,
				err)
		}
	}

	return nil
}

// getTopicName retrieves the topic name from the thread ID using the Telegram API
func (h *RepliesFromClosedThreadsHandler) getTopicName(threadId int64, chatId int64) (string, error) {
	// Convert threadId to int since GetChatMessageById expects an int
	messageId := int(threadId)

	// Remove "-100" prefix from chatId if present
	chatIdStr := strconv.FormatInt(chatId, 10)
	if strings.HasPrefix(chatIdStr, "-100") {
		chatIdStr = chatIdStr[4:] // Remove the first 4 characters ("-100")
		chatId, _ = strconv.ParseInt(chatIdStr, 10, 64)
	}

	// Get the topic message by ID
	message, err := clients.GetChatMessageById(chatId, messageId)
	if err != nil {
		return "Topic", fmt.Errorf("failed to get thread message: %w", err)
	}

	// Extract and truncate the topic name if needed
	topicName := message.Message
	if topicName == "" {
		topicName = "Topic"
	} else if len(topicName) > 30 {
		topicName = topicName[:27] + "..."
	}

	return topicName, nil
}

func (h *RepliesFromClosedThreadsHandler) forwardReplyMessage(ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	replyToMessageUrl := fmt.Sprintf(
		"https://t.me/c/%s/%d",
		strconv.FormatInt(msg.ReplyToMessage.Chat.Id, 10)[4:],
		msg.ReplyToMessage.MessageId)

	// Get the topic name
	topicName, topicErr := h.getTopicName(msg.MessageThreadId, msg.Chat.Id)
	if topicErr != nil {
		log.Printf(
			"%s: warning >> failed to get topic name: %v",
			constants.RepliesFromClosedThreadsHandlerName,
			topicErr)
		// Continue with a default topic name
		topicName = "Topic"
	}

	// Prepare the text with the topic name and user mention
	username := msg.From.Username
	prefixText := fmt.Sprintf(
		"↩️ oтвет от @%s в канале ",
		username)
	prefixLength := utf8.RuneCountInString(prefixText)
	topicNameLength := utf8.RuneCountInString(topicName)

	firstLine := fmt.Sprintf(
		"%s\"%s\"",
		prefixText,
		topicName)
	firstLineLength := utf8.RuneCountInString(firstLine)
	firstLineEntities := []gotgbot.MessageEntity{
		{
			Type:   "blockquote",
			Offset: 0,
			Length: int64(firstLineLength),
		},
		{
			Type:   "italic",
			Offset: 0,
			Length: int64(firstLineLength),
		},
		{
			Type:   "text_link",
			Offset: int64(prefixLength) + 1,
			Length: int64(topicNameLength),
			Url:    replyToMessageUrl,
		},
	}

	var finalMessage string
	var updatedEntities []gotgbot.MessageEntity

	// Check if we're dealing with caption or text
	if msg.Caption != "" {
		// Handle caption
		// Increase offset for all original caption entities
		for _, entity := range msg.CaptionEntities {
			// Check if the entity is within the bounds of the caption
			if int(entity.Offset)+int(entity.Length) <= len(msg.Caption) {
				newEntity := entity
				newEntity.Offset += int64(firstLineLength) + 1 // +1 for the newline character
				updatedEntities = append(updatedEntities, newEntity)
			}
		}
		updatedEntities = append(updatedEntities, firstLineEntities...)
		finalMessage = firstLine + "\n" + msg.Caption
	} else {
		// Handle regular text
		// Increase offset for all original entities
		for _, entity := range msg.Entities {
			// Check if the entity is within the bounds of the new text
			if int(entity.Offset)+int(entity.Length) <= len(msg.Text) {
				newEntity := entity
				newEntity.Offset += int64(firstLineLength) + 1 // +1 for the newline character
				updatedEntities = append(updatedEntities, newEntity)
			}
		}
		updatedEntities = append(updatedEntities, firstLineEntities...)
		finalMessage = firstLine + "\n" + msg.Text
	}

	// Forward the message
	_, err := h.messageSender.SendCopy(msg.Chat.Id, &h.config.ForwardingThreadID, finalMessage, updatedEntities, msg)
	if err != nil {
		return fmt.Errorf("%s: error >> failed to forward reply message: %w", constants.RepliesFromClosedThreadsHandlerName, err)
	}

	return nil
}

func (h *RepliesFromClosedThreadsHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	msg := ctx.EffectiveMessage
	if msg == nil || msg.ReplyToMessage == nil {
		return false
	}

	// Don't forward frow GroupAnonymousBot
	if msg.From.IsBot && msg.From.Username == "GroupAnonymousBot" {
		return false
	}

	// Trigger if message is in closed threads and not reply to itself
	return h.closedThreads[msg.MessageThreadId] && msg.ReplyToMessage.MessageId != msg.MessageThreadId
}

func (h *RepliesFromClosedThreadsHandler) Name() string {
	return constants.RepliesFromClosedThreadsHandlerName
}
