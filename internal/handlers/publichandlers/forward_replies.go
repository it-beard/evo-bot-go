package publichandlers

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
	"your_module_name/internal/handlers"
	"your_module_name/internal/services"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type ForwardRepliesHandler struct {
	threadsForClean map[int64]bool
	mainThreadId    int64
	messageSender   services.MessageSender
}

func NewForwardRepliesHandler(messageSender services.MessageSender) handlers.Handler {
	threadsForClean := make(map[int64]bool)
	threadsStr := os.Getenv("TG_EVO_BOT_THREADS_FOR_CLEAN_IDS")
	for _, chatID := range strings.Split(threadsStr, ",") {
		if id, err := strconv.ParseInt(chatID, 10, 64); err == nil {
			threadsForClean[id] = true
		}
	}
	mainThreadStr := os.Getenv("TG_EVO_BOT_MAIN_THREAD_ID")
	mainThreadId, err := strconv.ParseInt(mainThreadStr, 10, 64)
	if err != nil {
		log.Printf("Error parsing main thread ID: %v", err)
	}
	return &ForwardRepliesHandler{
		threadsForClean: threadsForClean,
		messageSender:   messageSender,
		mainThreadId:    mainThreadId,
	}
}

func (h *ForwardRepliesHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	if msg.ReplyToMessage != nil && h.threadsForClean[msg.MessageThreadId] && msg.ReplyToMessage.MessageId != msg.MessageThreadId {
		if err := h.forwardReplyMessage(ctx); err != nil {
			log.Printf("Error forwarding reply message: %v", err)
		}
		_, err := msg.Delete(b, nil)
		if err != nil {
			log.Printf("Error deleting original message after forwarding: %v", err)
		}
	}

	return nil
}

func (h *ForwardRepliesHandler) forwardReplyMessage(ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	replyToMessageUrl := fmt.Sprintf("https://t.me/c/%s/%d", strconv.FormatInt(msg.ReplyToMessage.Chat.Id, 10)[4:], msg.ReplyToMessage.MessageId)

	// Prepare the text with the user mention
	firstLine := fmt.Sprintf("oтвет на %s от @%s", replyToMessageUrl, msg.From.Username)
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
	}

	// Increase offset for all original entities
	var updatedEntities []gotgbot.MessageEntity
	for _, entity := range msg.Entities {
		// Check if the entity is within the bounds of the new text
		if int(entity.Offset)+int(entity.Length) <= len(msg.Text) {
			newEntity := entity
			newEntity.Offset += int64(firstLineLength) + 1 // +1 for the newline character
			updatedEntities = append(updatedEntities, newEntity)
		}
	}
	updatedEntities = append(updatedEntities, firstLineEntities...)

	finalMessage := firstLine + "\n" + msg.Text
	// Forward the message
	_, err := h.messageSender.SendCopy(msg.Chat.Id, &h.mainThreadId, finalMessage, updatedEntities, msg)
	if err != nil {
		return fmt.Errorf("failed to forward reply message: %w", err)
	}

	return nil
}

func (h *ForwardRepliesHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	msg := ctx.EffectiveMessage
	if msg == nil || msg.ReplyToMessage == nil {
		return false
	}

	// Don't forward frow GroupAnonymousBot
	if msg.From.IsBot && msg.From.Username == "GroupAnonymousBot" {
		return false
	}

	return h.threadsForClean[msg.MessageThreadId] && msg.ReplyToMessage.MessageId != msg.MessageThreadId
}

func (h *ForwardRepliesHandler) Name() string {
	return "forward_replies_handler"
}
