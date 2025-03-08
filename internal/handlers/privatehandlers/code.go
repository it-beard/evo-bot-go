package privatehandlers

import (
	"log"
	"strings"
	"github.com/it-beard/evo-bot-go/internal/clients"
	"github.com/it-beard/evo-bot-go/internal/config"
	"github.com/it-beard/evo-bot-go/internal/constants"
	"github.com/it-beard/evo-bot-go/internal/handlers"
	"github.com/it-beard/evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type CodeHandler struct {
	config *config.Config
}

func NewCodeHandler(config *config.Config) handlers.Handler {
	return &CodeHandler{
		config: config,
	}
}

func (h *CodeHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	// Extract code from command
	revertedCode := strings.TrimPrefix(msg.Text, constants.CodeCommand)
	revertedCode = strings.TrimSpace(revertedCode)
	code := reverseString(revertedCode)
	if code == "" {
		_, err := msg.Reply(b, "Пожалуйста, введи код из сообщения", nil)
		return err
	}

	// Store the code in memory
	clients.SetVerificationCode(code)
	log.Print("Code stored")
	err := clients.KeepSessionAlive() // Refresh session
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

	if msg.Text != "" && strings.HasPrefix(msg.Text, constants.CodeCommand) && msg.Chat.Type == constants.PrivateChat {
		if !utils.IsUserAdminOrCreator(b, msg.From.Id, h.config.MainChatID) {
			msg.Reply(b, "Команда доступна только для администраторов.", nil)
			log.Print("Trying to use /code command without admin rights")
			return false
		}
		return true
	}

	return false
}

func (h *CodeHandler) Name() string {
	return constants.CodeHandlerName
}

func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
