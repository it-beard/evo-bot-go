package grouphandlers

import (
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"
	"fmt"
	"log"
	"strconv"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

type RepliesFromClosedThreadsHandler struct {
	config               *config.Config
	closedTopics         map[int]bool
	messageSenderService *services.MessageSenderService
	groupTopicRepository *repositories.GroupTopicRepository
}

func NewRepliesFromClosedThreadsHandler(
	config *config.Config,
	messageSenderService *services.MessageSenderService,
	groupTopicRepository *repositories.GroupTopicRepository,
) ext.Handler {
	// Create map of closed topics
	closedTopics := make(map[int]bool)
	for _, id := range config.ClosedTopicsIDs {
		closedTopics[id] = true
	}

	h := &RepliesFromClosedThreadsHandler{
		closedTopics:         closedTopics,
		messageSenderService: messageSenderService,
		config:               config,
		groupTopicRepository: groupTopicRepository,
	}

	return handlers.NewMessage(h.check, h.handle)
}

func (h *RepliesFromClosedThreadsHandler) check(msg *gotgbot.Message) bool {
	if msg == nil || msg.ReplyToMessage == nil {
		return false
	}

	// Skip private chats
	if msg.Chat.Type == constants.PrivateChatType {
		return false
	}

	// Don't forward frow GroupAnonymousBot
	if msg.From.IsBot && msg.From.Username == "GroupAnonymousBot" {
		return false
	}

	// Trigger if message is in closed topics and not reply to itself
	return h.closedTopics[int(msg.MessageThreadId)] && msg.ReplyToMessage.MessageId != msg.MessageThreadId
}

func (h *RepliesFromClosedThreadsHandler) handle(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	if msg.ReplyToMessage != nil &&
		h.closedTopics[int(msg.MessageThreadId)] &&
		msg.ReplyToMessage.MessageId != msg.MessageThreadId {
		if err := h.forwardReplyMessage(ctx, b); err != nil {
			log.Printf(
				"%s: error >> failed to forward reply message: %v",
				utils.GetCurrentTypeName(),
				err)
		}
		_, err := msg.Delete(b, nil)
		if err != nil {
			log.Printf(
				"%s: error >> failed to delete original message after forwarding: %v",
				utils.GetCurrentTypeName(),
				err)
		}
	}

	return nil
}

func (h *RepliesFromClosedThreadsHandler) forwardReplyMessage(ctx *ext.Context, b *gotgbot.Bot) error {
	msg := ctx.EffectiveMessage
	replyToMessageUrl := fmt.Sprintf(
		"https://t.me/c/%s/%d",
		strconv.FormatInt(msg.ReplyToMessage.Chat.Id, 10)[4:],
		msg.ReplyToMessage.MessageId)

	// Get the topic name
	// [todo] get topic name from database
	groupTopic, err := h.groupTopicRepository.GetGroupTopicByTopicID(msg.MessageThreadId)
	if err != nil {
		log.Printf(
			"%s: error >> failed to get topic name: %v",
			utils.GetCurrentTypeName(),
			err)
	}

	// Prepare the text with the topic name and user mention
	prefixText, postfixText := "", ""
	if groupTopic != nil && groupTopic.Name != "" {
		prefixText = fmt.Sprintf(
			"↩️ oтвет @%s на сообщение в канале",
			msg.From.Username)
		postfixText = fmt.Sprintf("\"%s\"", groupTopic.Name)
	} else {
		prefixText = fmt.Sprintf(
			"↩️ oтвет @%s на",
			msg.From.Username)
		postfixText = "сообщение"
	}

	prefixLength := utf8.RuneCountInString(prefixText)
	postfixLength := utf8.RuneCountInString(postfixText)

	firstLine := fmt.Sprintf(
		"%s %s",
		prefixText,
		postfixText)
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
			Length: int64(postfixLength),
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
	_, err = h.messageSenderService.SendCopy(msg.Chat.Id, &h.config.ForwardingTopicID, finalMessage, updatedEntities, msg)
	if err != nil {
		return fmt.Errorf("%s: error >> failed to forward reply message: %w", utils.GetCurrentTypeName(), err)
	}

	return nil
}
