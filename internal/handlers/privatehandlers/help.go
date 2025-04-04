package privatehandlers

import (
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

type helpHandler struct {
	config *config.Config
}

func NewHelpHandler(config *config.Config) ext.Handler {
	h := &helpHandler{
		config: config,
	}

	return handlers.NewCommand(constants.HelpCommand, h.handleCommand)
}

func (h *helpHandler) handleCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Only proceed if this is a private chat
	if !utils.CheckPrivateChatType(b, ctx) {
		return nil
	}

	// Check if user is a club member
	if !utils.CheckClubMemberPermissions(b, msg, h.config, constants.HelpCommand) {
		return nil
	}

	isAdmin := utils.IsUserAdminOrCreator(b, msg.From.Id, h.config.SuperGroupChatID)
	helpText := utils.FormatHelpMessage(isAdmin)

	utils.SendLoggedHtmlReply(b, ctx.EffectiveMessage, helpText, nil)

	return nil
}
