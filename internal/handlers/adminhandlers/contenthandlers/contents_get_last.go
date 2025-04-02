package contenthandlers

import (
	"fmt"
	"log"
	"strings"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/handlers"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// todo: refactor to use conversation

type contentsGetLastHandler struct {
	contentRepository *repositories.ContentRepository
	config            *config.Config
}

func NewContentsGetLastHandler(
	contentRepository *repositories.ContentRepository,
	config *config.Config,
) handlers.Handler {
	return &contentsGetLastHandler{
		contentRepository: contentRepository,
		config:            config,
	}
}

func (h *contentsGetLastHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	contents, err := h.contentRepository.GetLastContents(constants.ContentsGetLastLimit)
	if err != nil {
		log.Printf("Failed to get last contents: %v", err)
		_, replyErr := msg.Reply(b, "Произошла ошибка при получении списка контента.", nil)
		return replyErr
	}

	if len(contents) == 0 {
		_, err := msg.Reply(b, "Записи о контенте еще не созданы.", nil)
		return err
	}

	var response strings.Builder
	response.WriteString(fmt.Sprintf("Последние %d контента:\n", len(contents)))
	for _, content := range contents {
		response.WriteString(fmt.Sprintf("- ID: %d, Название: %s\n", content.ID, content.Name))
	}

	_, err = msg.Reply(b, response.String(), nil)
	return err
}

func (h *contentsGetLastHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	msg := ctx.EffectiveMessage
	if msg == nil || msg.Text == "" {
		return false
	}

	if strings.HasPrefix(msg.Text, constants.ContentsGetLastCommand) && msg.Chat.Type == constants.PrivateChatType {
		// Check if the user is an admin in the configured supergroup chat
		if !utils.IsUserAdminOrCreator(b, msg.From.Id, h.config.SuperGroupChatID) {
			msg.Reply(b, "Эта команда доступна только администраторам.", nil)
			log.Printf("User %d tried to use /getlastcontents without admin rights.", msg.From.Id)
			return false
		}
		return true
	}

	return false
}

func (h *contentsGetLastHandler) Name() string {
	return constants.ContentsGetLastHandlerName
}
