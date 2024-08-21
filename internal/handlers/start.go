package handlers

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type StartHandler struct{}

func NewStartHandler() Handler {
	return &StartHandler{}
}

func (h *StartHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	_, err := ctx.EffectiveMessage.Reply(b, "Привет! Я дворецкий бот клуба \"Эволюция Кода\". Используй /help, что бы увидеть мои возможности.", nil)
	return err
}

func (h *StartHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	if ctx.EffectiveMessage == nil {
		return false
	}
	return ctx.EffectiveMessage.Text != "" && ctx.EffectiveMessage.Text == "/start"
}

func (h *StartHandler) Name() string {
	return "start_handler"
}
