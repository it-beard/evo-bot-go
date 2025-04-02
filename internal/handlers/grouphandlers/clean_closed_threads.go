package grouphandlers

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/services"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

type CleanClosedThreadsHandler struct {
	closedTopics         map[int]bool
	messageSenderService services.MessageSenderService
	config               *config.Config
	botUsername          string
}

func NewCleanClosedThreadsHandler(messageSenderService services.MessageSenderService, config *config.Config) ext.Handler {
	// Create map of closed topics
	closedTopics := make(map[int]bool)
	for _, id := range config.ClosedTopicsIDs {
		closedTopics[id] = true
	}
	h := &CleanClosedThreadsHandler{
		closedTopics:         closedTopics,
		messageSenderService: messageSenderService,
		config:               config,
	}

	return handlers.NewMessage(h.check, h.handle)
}

func (h *CleanClosedThreadsHandler) check(msg *gotgbot.Message) bool {
	if msg == nil {
		return false
	}

	// Check if the topic is in closed topics list
	if !h.closedTopics[int(msg.MessageThreadId)] {
		return false
	}

	// Don't trigger if message is reply to another message in thread (this already handled by RepliesFromThreadsHandler)
	if h.closedTopics[int(msg.MessageThreadId)] &&
		msg.ReplyToMessage != nil &&
		msg.ReplyToMessage.MessageId != msg.MessageThreadId {
		return false
	}

	// Don't trigger if message handled by SaveHandler
	// Note: we'll check the exact username in HandleUpdate since we need the bot instance
	if strings.HasPrefix(msg.Text, "/save") ||
		strings.HasPrefix(msg.Text, "/forward") {
		return false
	}

	return true
}

func (h *CleanClosedThreadsHandler) handle(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Set bot username if not set
	if h.botUsername == "" {
		h.botUsername = b.User.Username
	}

	// Don't trigger if message handled by SaveHandler with exact bot username
	if msg.Text == "@"+h.botUsername {
		return nil
	}

	// Don't trigger if message from admin (must be checked here since we need the bot instance)
	chatMember, err := b.GetChatMember(msg.Chat.Id, msg.From.Id, nil)
	if err == nil {
		if member, ok := chatMember.(gotgbot.ChatMemberAdministrator); ok && member.CanDeleteMessages {
			return nil
		}
		if _, ok := chatMember.(gotgbot.ChatMemberOwner); ok {
			return nil
		}
	}

	// Don't trigger if message from bot with name "GroupAnonymousBot" (this is anonymous group bot)
	if msg.From.IsBot && msg.From.Username == "GroupAnonymousBot" {
		return nil
	}

	_, err = msg.Delete(b, nil)
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
		"*Приношу свои извинения* 🧐\n\n"+
			"Твоё сообщение в канале %s было удалено, поскольку этот канал предназначен только для чтения. "+
			"Однако ты можешь присоединиться к обсуждению, используя функцию *Reply* (ответ) на интересующий тебя пост. "+
			"Твой ответ автоматически появится в чате \"_Оффтопчик_\" 👌\n\n"+
			"⬇️ _Копия твоего сообщения_ ⬇️",
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

func (h *CleanClosedThreadsHandler) Name() string {
	return constants.CleanClosedThreadsHandlerName
}
