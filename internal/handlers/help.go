package handlers

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type HelpHandler struct{}

func NewHelpHandler() Handler {
	return &HelpHandler{}
}

func (h *HelpHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	helpText := "Доступные команды:\n" +
		"/start - Приветственное сообщение\n" +
		"/help - Показывает это сообщение\n" +
		"/forward - Пересылает зареплаенное сообщение в ЛС"
	_, err := ctx.EffectiveMessage.Reply(b, helpText, nil)
	return err
}

func (h *HelpHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	if ctx.EffectiveMessage == nil {
		return false
	}
	return ctx.EffectiveMessage.Text != "" && ctx.EffectiveMessage.Text == "/help"
}

func (h *HelpHandler) Name() string {
	return "help_handler"
}
