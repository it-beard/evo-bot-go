package grouphandlers

import (
	"evo-bot-go/internal/services/grouphandlersservices"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/chatmember"
)

type ChatMemberHandler struct {
	joinLeftService *grouphandlersservices.JoinLeftService
}

func NewChatMemberHandler(joinLeftService *grouphandlersservices.JoinLeftService) ext.Handler {
	h := &ChatMemberHandler{joinLeftService: joinLeftService}
	return handlers.NewChatMember(chatmember.All, h.handle)
}

func (h *ChatMemberHandler) handle(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsMessageFromSuperGroupChat(ctx.EffectiveMessage.Chat) {
		return nil
	}

	return h.joinLeftService.HandleJoinLeftMember(b, ctx)
}
