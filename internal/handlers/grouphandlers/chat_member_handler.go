package grouphandlers

import (
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/services/grouphandlersservices"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/chatmember"
)

type ChatMemberHandler struct {
	config          *config.Config
	joinLeftService *grouphandlersservices.JoinLeftService
}

func NewChatMemberHandler(config *config.Config, joinLeftService *grouphandlersservices.JoinLeftService) ext.Handler {
	h := &ChatMemberHandler{config: config, joinLeftService: joinLeftService}
	return handlers.NewChatMember(chatmember.All, h.handle)
}

func (h *ChatMemberHandler) handle(b *gotgbot.Bot, ctx *ext.Context) error {
	// chat_member updates carry no message, the chat comes with the update itself
	if ctx.ChatMember == nil || !utils.IsClubSuperGroupChat(ctx.ChatMember.Chat, h.config) {
		return nil
	}

	return h.joinLeftService.HandleJoinLeftMember(b, ctx)
}
