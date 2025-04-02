package handlers

import (
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

type startHandler struct{}

func NewStartHandler() ext.Handler {
	h := &startHandler{}

	return handlers.NewCommand(constants.StartCommand, h.handleCommand)
}

func (h *startHandler) handleCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	// Only proceed if this is a private chat
	if !utils.CheckPrivateChatType(b, ctx) {
		return handlers.EndConversation()
	}

	message := "Привет! Я *Дженкинс Вебстер*, дворецкий бот клуба _\"Эволюция Кода\"_.\n" +
		"Используй /help для просмотра возможностей."

	utils.SendLoggedMarkdownReply(b, ctx.EffectiveMessage, message, nil)

	return handlers.EndConversation()
}
