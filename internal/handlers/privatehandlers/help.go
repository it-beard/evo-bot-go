package privatehandlers

import (
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/formatters"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

type helpHandler struct {
	config               *config.Config
	messageSenderService *services.MessageSenderService
	permissionsService   *services.PermissionsService
}

func NewHelpHandler(
	config *config.Config,
	messageSenderService *services.MessageSenderService,
	permissionsService *services.PermissionsService,
) ext.Handler {
	h := &helpHandler{
		config:               config,
		messageSenderService: messageSenderService,
		permissionsService:   permissionsService,
	}

	return handlers.NewCommand(constants.HelpCommand, h.handleCommand)
}

func (h *helpHandler) handleCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Only proceed if this is a private chat
	if !h.permissionsService.CheckPrivateChatType(msg) {
		return nil
	}

	// Check if user is a club member
	if !h.permissionsService.CheckClubMemberPermissions(msg, constants.HelpCommand) {
		return nil
	}

	user := ctx.EffectiveUser
	isAdmin := utils.IsUserAdminOrCreator(b, user.Id, h.config)
	helpText := formatters.FormatHelpMessage(isAdmin)

	h.messageSenderService.ReplyHtml(msg, helpText, nil)

	return nil
}
