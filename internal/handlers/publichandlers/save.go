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
	// Delete the reply message before sending the copy
	_, err := ctx.EffectiveMessage.Delete(b, nil)
	if err != nil {
		return fmt.Errorf("%s: err >> error deleting reply message: %v", saveHandlerName, err)
	}

	// Return error if the reply to message is nil
	if ctx.EffectiveMessage.ReplyToMessage == nil {
		return fmt.Errorf("%s: warning >> reply to message is nil", saveHandlerName)
	}

	// Return error if the reply to message text is empty
	if ctx.EffectiveMessage.ReplyToMessage.Text == "" {
		return fmt.Errorf("%s: warning >> reply to message text is empty", saveHandlerName)
	}

	// Setting up copy message
	originalMessage := ctx.EffectiveMessage.ReplyToMessage
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

	var userId int64 = ctx.EffectiveUser.Id
	if ctx.EffectiveUser.IsBot && ctx.EffectiveUser.Username == "GroupAnonymousBot" {
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
		ctx.EffectiveUser.Username,
		userId)

	// Sending info message
	_, err = h.messageSender.SendCopy(userId, nil, infoMsgText, infoMsgEntities, nil)
	if err != nil {
		return fmt.Errorf("%s: err >> error sending info message: %v", saveHandlerName, err)
	}
	return nil
}

func (h *SaveHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	if ctx.EffectiveMessage == nil {
		return false
	}
	return ctx.EffectiveMessage.Text != "" && (ctx.EffectiveMessage.Text == "@"+b.User.Username ||
		strings.HasPrefix(ctx.EffectiveMessage.Text, "/save") ||
		strings.HasPrefix(ctx.EffectiveMessage.Text, "/forward"))
}

func (h *SaveHandler) Name() string {
	return saveHandlerName
}
