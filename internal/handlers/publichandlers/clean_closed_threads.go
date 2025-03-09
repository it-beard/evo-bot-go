package publichandlers

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/it-beard/evo-bot-go/internal/config"
	"github.com/it-beard/evo-bot-go/internal/constants"
	"github.com/it-beard/evo-bot-go/internal/handlers"
	"github.com/it-beard/evo-bot-go/internal/services"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type CleanClosedThreadsHandler struct {
	closedTopics         map[int]bool
	messageSenderService services.MessageSenderService
	config               *config.Config
}

func NewCleanClosedThreadsHandler(messageSenderService services.MessageSenderService, config *config.Config) handlers.Handler {
	// Create map of closed topics
	closedTopics := make(map[int]bool)
	for _, id := range config.ClosedTopicsIDs {
		closedTopics[id] = true
	}
	return &CleanClosedThreadsHandler{
		closedTopics:         closedTopics,
		messageSenderService: messageSenderService,
		config:               config,
	}
}

func (h *CleanClosedThreadsHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	_, err := msg.Delete(b, nil)
	if err != nil {
		return fmt.Errorf(
			"%s: error >> failed to delete message: %w",
			constants.CleanClosedThreadsHandlerName,
			err)
	}

	// Prepare messages
	chatIdStr := strconv.FormatInt(msg.Chat.Id, 10)[4:]
	threadUrl := fmt.Sprintf("https://t.me/c/%s/%d", chatIdStr, msg.MessageThreadId)
	messageText := fmt.Sprintf(
		"Ваше сообщение в канале %s было удалено, так как этот канал предназначен только для чтения. "+
			"Но вы можете отвечать на посты из этого канала. Для этого сделайте Replay (ответ) на выбранный пост, "+
			"после чего ваш ответ появиться в чате \"Оффтопчик\".\n\n"+
			"⬇️ _Копия удаленного сообщения_ ⬇️",
		threadUrl,
	)

	// Send message to user about deletion
	_, err = b.SendMessage(msg.From.Id, messageText, &gotgbot.SendMessageOpts{ParseMode: "markdown"})
	if err != nil {
		return fmt.Errorf(
			"%s: error >> failed to send message about deletion: %w",
			constants.CleanClosedThreadsHandlerName,
			err)
	}
	// Send copy of the message to user
	_, err = h.messageSenderService.SendCopy(msg.From.Id, nil, msg.Text, msg.Entities, msg)
	if err != nil {
		return fmt.Errorf(
			"%s: error >> failed to send copy message: %w",
			constants.CleanClosedThreadsHandlerName,
			err)
	}

	// Log the deletion
	log.Printf(
		"%s: Deleted message in topic %s\n"+
			"User ID: %d\n"+
			"Content: \"%s\"",
		constants.CleanClosedThreadsHandlerName, threadUrl, msg.From.Id, msg.Text)

	return nil
}

func (h *CleanClosedThreadsHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	msg := ctx.EffectiveMessage
	if msg == nil {
		return false
	}

	// Check if the topic is in closed topics list
	if !h.closedTopics[int(msg.MessageThreadId)] {
		return false
	}

	// Don't trigger if message from admin
	chatMember, err := b.GetChatMember(msg.Chat.Id, msg.From.Id, nil)
	if err == nil {
		if member, ok := chatMember.(gotgbot.ChatMemberAdministrator); ok {
			return !member.CanDeleteMessages
		}
		if _, ok := chatMember.(gotgbot.ChatMemberOwner); ok {
			return false
		}
	}

	// Don't trigger if message from bot with name "GroupAnonymousBot" (this is anonymous group bot)
	if msg.From.IsBot && msg.From.Username == "GroupAnonymousBot" {
		return false
	}

	// Do not trigger if message is reply to another message in thread (this already handled by RepliesFromThreadsHandler)
	if h.closedTopics[int(msg.MessageThreadId)] &&
		msg.ReplyToMessage != nil &&
		msg.ReplyToMessage.MessageId != msg.MessageThreadId {
		return false
	}

	// Don't trigger if message handled by SaveHandler
	if msg.Text == "@"+b.User.Username ||
		strings.HasPrefix(msg.Text, "/save") ||
		strings.HasPrefix(msg.Text, "/forward") {
		return false
	}

	return true
}

func (h *CleanClosedThreadsHandler) Name() string {
	return constants.CleanClosedThreadsHandlerName
}
