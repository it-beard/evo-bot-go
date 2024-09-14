package publichandlers

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"your_module_name/internal/handlers"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type DeleteMessagesHandler struct {
	threadsForClean map[int64]bool
}

func NewDeleteMessagesHandler() handlers.Handler {
	threadsForClean := make(map[int64]bool)
	threadsStr := os.Getenv("TG_EVO_BOT_THREADS_FOR_CLEAN_IDS")
	for _, chatID := range strings.Split(threadsStr, ",") {
		if id, err := strconv.ParseInt(chatID, 10, 64); err == nil {
			threadsForClean[id] = true
		}
	}
	return &DeleteMessagesHandler{
		threadsForClean: threadsForClean,
	}
}

func (h *DeleteMessagesHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	if h.shouldDeleteMessage(b, ctx) {
		_, err := msg.Delete(b, nil)
		if err != nil {
			log.Printf("Error deleting message: %v", err)
		} else {
			if msg.NewChatMembers != nil {
				log.Printf("New user joined. User ID: %v", msg.NewChatMembers[0].Username)
			} else if msg.LeftChatMember != nil {
				log.Printf("User left. User ID: %v", msg.LeftChatMember.Username)
			} else {
				// send message about deleting message to user
				b.SendMessage(msg.From.Id, fmt.Sprintf("Ваше сообщение в треде _№%d_ сообщества \"_%s_\" было удалено, так как этот тред только для чтения. \n\nОднако, вы можете сохранять сообщения из него с моей помощью. Подробнее по команде /help", msg.MessageThreadId, msg.Chat.Title), &gotgbot.SendMessageOpts{ParseMode: "markdown"})
				log.Printf("Deleted message in topic #%d\nUser ID: %d\nContent\"%s\"", msg.MessageThreadId, msg.From.Id, msg.Text)
			}
		}
	}

	return nil
}

func (h *DeleteMessagesHandler) shouldDeleteMessage(b *gotgbot.Bot, ctx *ext.Context) bool {
	msg := ctx.EffectiveMessage

	// Check if the message is about new user joined or user left
	if msg.NewChatMembers != nil || msg.LeftChatMember != nil {
		return true
	}

	// Check if the thread is in the list for cleaning
	if !h.threadsForClean[msg.MessageThreadId] {
		return false
	}

	// Don't delete messages with specific content
	if msg.Text == "/save" ||
		msg.Text == "@"+b.User.Username ||
		msg.Text == "/save@"+b.User.Username {
		return false
	}

	// Don't delete messages from admins
	chatMember, err := b.GetChatMember(msg.Chat.Id, msg.From.Id, nil)
	if err == nil {
		if member, ok := chatMember.(gotgbot.ChatMemberAdministrator); ok {
			return !member.CanDeleteMessages
		}
		if _, ok := chatMember.(gotgbot.ChatMemberOwner); ok {
			return false
		}
	}

	// Don't delete messages from bot with name "GroupAnonymousBot" (this is anonymous bot)
	if msg.From.IsBot && msg.From.Username == "GroupAnonymousBot" {
		return false
	}

	// Do not delete messages that are replies to messages (this already handled by ForwardRepliesHandler)
	if h.threadsForClean[msg.MessageThreadId] && msg.ReplyToMessage.MessageId != msg.MessageThreadId {
		return false
	}

	return true
}

func (h *DeleteMessagesHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	msg := ctx.EffectiveMessage
	if msg == nil {
		return false
	}

	// Check for new members, left members, or messages in threads for cleaning
	return msg.NewChatMembers != nil ||
		msg.LeftChatMember != nil ||
		h.threadsForClean[msg.MessageThreadId]
}

func (h *DeleteMessagesHandler) Name() string {
	return "delete_messages_handler"
}
