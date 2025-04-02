package contenthandlers

import (
	"fmt"
	"log"
	"strings"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

type contentsGetLastHandler struct {
	contentRepository *repositories.ContentRepository
	config            *config.Config
}

func NewContentsGetLastHandler(
	contentRepository *repositories.ContentRepository,
	config *config.Config,
) ext.Handler {
	h := &contentsGetLastHandler{
		contentRepository: contentRepository,
		config:            config,
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.ContentsGetLastCommand, h.handleGetLastContents),
		},
		map[string][]ext.Handler{}, // No additional states needed for this simple handler
		&handlers.ConversationOpts{},
	)
}

func (h *contentsGetLastHandler) handleGetLastContents(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check admin permissions
	if !utils.IsUserAdminOrCreator(b, msg.From.Id, h.config.SuperGroupChatID) {
		if _, err := msg.Reply(b, "Эта команда доступна только администраторам.", nil); err != nil {
			log.Printf("Failed to send admin-only message: %v", err)
		}
		log.Printf("User %d tried to use %s without admin rights", msg.From.Id, constants.ContentsGetLastCommand)
		return handlers.EndConversation()
	}

	// Check if the command is used in a private chat
	if msg.Chat.Type != constants.PrivateChatType {
		if _, err := msg.Reply(b, "Эта команда доступна только в личном чате.", nil); err != nil {
			log.Printf("Failed to send private-only message: %v", err)
		}
		return handlers.EndConversation()
	}

	contents, err := h.contentRepository.GetLastContents(constants.ContentsGetLastLimit)
	if err != nil {
		log.Printf("Failed to get last contents: %v", err)
		if _, replyErr := msg.Reply(b, "Произошла ошибка при получении списка контента.", nil); replyErr != nil {
			log.Printf("Failed to send error message: %v", replyErr)
		}
		return handlers.EndConversation()
	}

	if len(contents) == 0 {
		if _, err := msg.Reply(b, "Записи о контенте еще не созданы.", nil); err != nil {
			log.Printf("Failed to send empty contents message: %v", err)
		}
		return handlers.EndConversation()
	}

	var response strings.Builder
	response.WriteString(fmt.Sprintf("Последние %d контента:\n", len(contents)))
	for _, content := range contents {
		response.WriteString(fmt.Sprintf("- ID: %d, Название: %s\n", content.ID, content.Name))
	}

	if _, err := msg.Reply(b, response.String(), nil); err != nil {
		log.Printf("Error sending content list: %v", err)
	}

	return handlers.EndConversation()
}

func (h *contentsGetLastHandler) Name() string {
	return constants.ContentsGetLastHandlerName
}
