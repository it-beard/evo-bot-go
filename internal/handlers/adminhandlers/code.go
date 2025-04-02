package adminhandlers

import (
	"log"
	"strings"

	"evo-bot-go/internal/clients"
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

type codeHandler struct {
	config *config.Config
}

func NewCodeHandler(config *config.Config) ext.Handler {
	h := &codeHandler{
		config: config,
	}

	return handlers.NewCommand(constants.CodeCommand, h.handleCode)
}

func (h *codeHandler) handleCode(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check admin permissions and private chat
	if !utils.CheckAdminAndPrivateChat(b, ctx, h.config.SuperGroupChatID, constants.CodeCommand) {
		return nil
	}

	// Extract code from command
	revertedCode := strings.TrimPrefix(msg.Text, "/"+constants.CodeCommand)
	revertedCode = strings.TrimSpace(revertedCode)
	code := reverseString(revertedCode)
	if code == "" {
		utils.SendLoggedReply(b, msg, "Пожалуйста, введи код из сообщения", nil)
		return nil
	}

	// Store the code in memory
	clients.TgSetVerificationCode(code)
	log.Print("Code stored")
	err := clients.TgKeepSessionAlive() // Refresh session
	if err == nil {
		utils.SendLoggedReply(b, msg, "Код принят", nil)
	} else {
		utils.SendLoggedReply(b, msg, "Произошла ошибка при сохранении кода", err)
	}

	return nil
}

func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
