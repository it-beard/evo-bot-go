package privatehandlers

import (
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/utils"
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

type showContentHandler struct {
	contentRepository *repositories.ContentRepository
	config            *config.Config
}

func NewShowContentHandler(
	contentRepository *repositories.ContentRepository,
	config *config.Config,
) ext.Handler {
	h := &showContentHandler{
		contentRepository: contentRepository,
		config:            config,
	}

	return handlers.NewCommand(constants.ContentShowCommand, h.handleCommand)
}

func (h *showContentHandler) handleCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Only proceed if this is a private chat
	if !utils.CheckPrivateChatType(b, ctx) {
		return nil
	}

	// Check if user is a club member
	if !utils.CheckClubMemberPermissions(b, msg, h.config, constants.ContentShowCommand) {
		return nil
	}

	// Get actual contents to show
	contents, err := h.contentRepository.GetLastActualContents(10) // Fetch last 10 actual contents
	if err != nil {
		utils.SendLoggedReply(b, msg, "Ошибка при получении списка контента.", err)
		return nil
	}

	if len(contents) == 0 {
		utils.SendLoggedReply(b, msg, "На данный момент нет актуального контента.", nil)
		return nil
	}

	// Format and display content list
	formattedContents := utils.FormatContentListForUsersWithoutIds(
		contents,
		"📋 Список ближайших мероприятий",
	)
	formattedContents += fmt.Sprintf("\nИспользуй команду /%s, если хочешь предложить тему или вопросы к этим мероприятиям.", constants.TopicAddCommand)
	utils.SendLoggedMarkdownReply(b, msg, formattedContents, nil)

	return nil
}
