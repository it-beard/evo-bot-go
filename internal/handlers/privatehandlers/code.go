package privatehandlers

import (
	"log"
	"os"
	"strconv"
	"strings"
	"your_module_name/internal/clients"
	"your_module_name/internal/handlers"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type CodeHandler struct {
	chatId int64
}

func NewCodeHandler() handlers.Handler {
	chatIdStr := os.Getenv("TG_EVO_BOT_MAIN_CHAT_ID")
	chatId, err := strconv.ParseInt(chatIdStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid TG_EVO_BOT_MAIN_CHAT_ID: %v", err)
	}
	return &CodeHandler{chatId: chatId}
}

func (h *CodeHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// Extract code from command
	revertedCode := strings.TrimPrefix(msg.Text, codeCommand)
	revertedCode = strings.TrimSpace(revertedCode)
	code := reverseString(revertedCode)
	if code == "" {
		_, err := msg.Reply(b, "Пожалуйста, введи код из сообщения", nil)
		return err
	}

	// Store the code in memory
	clients.SetVerificationCode(code)
	log.Print("Code stored")
	err := clients.TgUserClientKeepSessionAlive() // Refresh session
	if err == nil {
		msg.Reply(b, "Код принят", nil)
	}
	return err
}

func (h *CodeHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	msg := ctx.EffectiveMessage
	if msg == nil {
		return false
	}

	if msg.Text != "" && strings.HasPrefix(msg.Text, codeCommand) && msg.Chat.Type == privateChat {
		if !h.isUserAdminOrCreator(b, msg) {
			msg.Reply(b, "Команда доступна только для администраторов.", nil)
			log.Print("Trying to use /code command without admin rights")
			return false
		}
		return true
	}

	return false
}

func (h *CodeHandler) Name() string {
	return codeHandlerName
}

func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func (h *CodeHandler) isUserAdminOrCreator(b *gotgbot.Bot, msg *gotgbot.Message) bool {
	chatId, err := strconv.ParseInt("-100"+strconv.FormatInt(h.chatId, 10), 10, 64)
	if err != nil {
		log.Printf("Failed to parse chat ID: %v", err)
		return false
	}
	// Check if user is member of target group
	chatMember, err := b.GetChatMember(chatId, msg.From.Id, nil)
	if err != nil {
		log.Printf("Failed to get chat member: %v", err)
		return false
	}

	status := chatMember.GetStatus()
	if status == "administrator" || status == "creator" {
		return true
	}
	return false
}
