package handlers

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"unicode/utf8"

	"your_module_name/internal/services"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type ForwardHandler struct {
	messageSender services.MessageSender
}

func NewForwardHandler(messageSender services.MessageSender) Handler {
	return &ForwardHandler{messageSender: messageSender}
}

func (h *ForwardHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	// Delete the reply message before sending the copy
	_, err := ctx.EffectiveMessage.Delete(b, nil)
	if err != nil {
		return fmt.Errorf("err >> error deleting reply message: %v", err)
	}

	// Return error if the reply to message is nil
	if ctx.EffectiveMessage.ReplyToMessage == nil {
		return fmt.Errorf("err >> reply to message is nil")
	}

	// Setting up copy message
	originalMessage := ctx.EffectiveMessage.ReplyToMessage
	originalMessageUrl := fmt.Sprintf("https://t.me/c/%s/%d", strconv.FormatInt(originalMessage.Chat.Id, 10)[4:], originalMessage.MessageId)
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

	// Sending copied message
	_, err = h.messageSender.Send(ctx.EffectiveUser.Id, originalText, originalMessage.Entities, originalMessage)
	if err != nil {
		return fmt.Errorf("err >> error sending copied message: %v", err)
	}
	log.Printf("Copied message sent: %v", originalMessageUrl)

	// Sending info message
	_, err = h.messageSender.Send(ctx.EffectiveUser.Id, infoMsgText, infoMsgEntities, nil)
	if err != nil {
		return fmt.Errorf("err >> error sending info message: %v", err)
	}
	log.Printf("Info message sent: %v", originalMessageUrl)
	return nil
}

func (h *ForwardHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	if ctx.EffectiveMessage == nil {
		return false
	}
	return ctx.EffectiveMessage.Text != "" && (ctx.EffectiveMessage.Text == "@"+b.User.Username || strings.HasPrefix(ctx.EffectiveMessage.Text, "/forward"))
}

func (h *ForwardHandler) Name() string {
	return "forward_handler"
}
