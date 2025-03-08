package publichandlers

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/it-beard/evo-bot-go/internal/config"
	"github.com/it-beard/evo-bot-go/internal/constants"
	"github.com/it-beard/evo-bot-go/internal/handlers"
	"github.com/it-beard/evo-bot-go/internal/services"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Constants moved to internal/constants/public_handlers.go

type SaveHandler struct {
	messageSender services.MessageSender
	config        *config.Config
}

func NewSaveHandler(messageSender services.MessageSender, config *config.Config) handlers.Handler {
	return &SaveHandler{
		messageSender: messageSender,
		config:        config,
	}
}

func (h *SaveHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// Delete the reply message before sending the copy
	_, err := msg.Delete(b, nil)
	if err != nil {
		return fmt.Errorf("%s: err >> error deleting reply message: %v", constants.SaveHandlerName, err)
	}

	// Return warning if the reply to message is nil
	if msg.ReplyToMessage == nil {
		return fmt.Errorf("%s: warning >> reply to message is nil", constants.SaveHandlerName)
	}

	// Return warning if it is reply to first message in message thread (not a reply in thread context)
	if msg.ReplyToMessage.MessageId == msg.ReplyToMessage.MessageThreadId {
		return fmt.Errorf("%s: warning >> reply to message not exists", constants.SaveHandlerName)
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
		userId = h.config.AnonymousUserID
	}

	// Sending copied message
	_, err = h.messageSender.SendCopy(userId, nil, originalText, originalMessage.Entities, originalMessage)
	if err != nil {
		return fmt.Errorf("%s: err >> error sending copied message: %v", constants.SaveHandlerName, err)
	}
	log.Printf(
		"%s: Copied message sent: %v\nUsername: %s\nUser ID: %d",
		constants.SaveHandlerName,
		originalMessageUrl,
		msg.From.Username,
		userId)

	// Sending info message
	_, err = h.messageSender.SendCopy(userId, nil, infoMsgText, infoMsgEntities, nil)
	if err != nil {
		return fmt.Errorf("%s: err >> error sending info message: %v", constants.SaveHandlerName, err)
	}
	return nil
}

func (h *SaveHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	msg := ctx.EffectiveMessage
	if msg == nil {
		return false
	}
	return msg.Text != "" && (msg.Text == "@"+b.User.Username ||
		strings.HasPrefix(msg.Text, constants.SaveCommand) ||
		strings.HasPrefix(msg.Text, constants.ForwardCommand))
}

func (h *SaveHandler) Name() string {
	return constants.SaveHandlerName
}
