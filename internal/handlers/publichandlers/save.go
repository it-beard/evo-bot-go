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

const saveHandlerName = "save_handler"

type SaveHandler struct {
	messageSender   services.MessageSender
	anonymousUserId int64
}

func NewSaveHandler(messageSender services.MessageSender) handlers.Handler {
	anonymousUserIdString := os.Getenv("TG_EVO_BOT_ANONYMOUS_USER_ID")
	anonymousUserId, err := strconv.ParseInt(anonymousUserIdString, 10, 64)
	if err != nil {
		log.Printf("%s: error >> error parsing anonymous user ID from env: %v", saveHandlerName, err)
	}

	return &SaveHandler{
		messageSender:   messageSender,
		anonymousUserId: anonymousUserId,
	}
}

func (h *SaveHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// Delete the reply message before sending the copy
	_, err := msg.Delete(b, nil)
	if err != nil {
		return fmt.Errorf("%s: err >> error deleting reply message: %v", saveHandlerName, err)
	}

	// Return warning if the reply to message is nil
	if msg.ReplyToMessage == nil {
		return fmt.Errorf("%s: warning >> reply to message is nil", saveHandlerName)
	}

	// Return warning if it is reply to first message in message thread (not a reply in thread context)
	if msg.ReplyToMessage.MessageId == msg.ReplyToMessage.MessageThreadId {
		return fmt.Errorf("%s: warning >> reply to message not exists", saveHandlerName)
	}

	// Setting up copy message
	originalMessage := msg.ReplyToMessage
	originalMessageUrl := fmt.Sprintf(
		"https://t.me/c/%s/%d",
		strconv.FormatInt(originalMessage.Chat.Id, 10)[4:],
		originalMessage.MessageId)
	originalText := originalMessage.Text
	if originalText == "" {
		originalText = originalMessage.Caption
	}
	infoMsgText := "⬆️ ссылка на оригинал ⬆️"
	infoMsgLen := utf8.RuneCountInString(infoMsgText)
	infoMsgEntities := []gotgbot.MessageEntity{
		{
			Type:   "italic",
			Offset: 0,
			Length: int64(infoMsgLen),
		},
		{
			Type:   "text_link",
			Offset: 0,
			Length: int64(infoMsgLen),
			Url:    originalMessageUrl,
		},
	}

	var userId int64 = msg.From.Id
	if msg.From.IsBot && msg.From.Username == "GroupAnonymousBot" {
		userId = h.anonymousUserId
	}

	// Sending copied message
	_, err = h.messageSender.SendCopy(userId, nil, originalText, originalMessage.Entities, originalMessage)
	if err != nil {
		return fmt.Errorf("%s: err >> error sending copied message: %v", saveHandlerName, err)
	}
	log.Printf(
		"%s: Copied message sent: %v\nUsername: %s\nUser ID: %d",
		saveHandlerName,
		originalMessageUrl,
		msg.From.Username,
		userId)

	// Sending info message
	_, err = h.messageSender.SendCopy(userId, nil, infoMsgText, infoMsgEntities, nil)
	if err != nil {
		return fmt.Errorf("%s: err >> error sending info message: %v", saveHandlerName, err)
	}
	return nil
}

func (h *SaveHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	msg := ctx.EffectiveMessage
	if msg == nil {
		return false
	}
	return msg.Text != "" && (msg.Text == "@"+b.User.Username ||
		strings.HasPrefix(msg.Text, "/save") ||
		strings.HasPrefix(msg.Text, "/forward"))
}

func (h *SaveHandler) Name() string {
	return saveHandlerName
}
