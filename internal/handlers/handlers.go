package handlers

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type Handler interface {
	HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error
	CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool
	Name() string
}
