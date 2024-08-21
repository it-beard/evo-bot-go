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
		log.Printf("Error deleting reply message: %v", err)
		return err
	}

	originalMessage := ctx.EffectiveMessage.ReplyToMessage
	originalMessageUrl := fmt.Sprintf("https://t.me/c/%s/%d", strconv.FormatInt(originalMessage.Chat.Id, 10)[4:], originalMessage.MessageId)
	originalText := originalMessage.Text
	if originalText == "" {
		originalText = originalMessage.Caption
	}
	bottomText := "[тыц]"
	bottomTextLen := utf8.RuneCountInString(bottomText) + 1
	messageText := fmt.Sprintf("%s\n%s ", originalText, bottomText)
	entities := append(originalMessage.Entities,
		gotgbot.MessageEntity{
			Type:   "italic",
			Offset: int64(utf8.RuneCountInString(messageText) - bottomTextLen),
			Length: int64(bottomTextLen),
		},
		gotgbot.MessageEntity{
			Type:   "text_link",
			Offset: int64(utf8.RuneCountInString(messageText) - bottomTextLen),
			Length: int64(bottomTextLen),
			Url:    originalMessageUrl,
		})

	// Sending the message
	_, err = h.messageSender.Send(ctx.EffectiveUser.Id, messageText, entities, originalMessage)
	if err != nil {
		log.Printf("Error sending message: %v", err)
		return err
	}
	log.Printf("Message sent to user: %v", originalMessageUrl)
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
