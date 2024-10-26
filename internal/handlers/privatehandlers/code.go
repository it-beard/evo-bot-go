package privatehandlers

import (
	"log"
	"strings"
	"your_module_name/internal/clients"
	"your_module_name/internal/handlers"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

const codeCommand = "/code"

type CodeHandler struct{}

func NewCodeHandler() handlers.Handler {
	return &CodeHandler{}
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
	if ctx.EffectiveMessage == nil {
		return false
	}
	return ctx.EffectiveMessage.Text != "" &&
		strings.HasPrefix(ctx.EffectiveMessage.Text, codeCommand) &&
		ctx.EffectiveMessage.Chat.Type == "private"
}

func (h *CodeHandler) Name() string {
	return "code_handler"
}

func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
