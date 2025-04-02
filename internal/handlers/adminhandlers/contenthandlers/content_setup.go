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
type contentSetupHandler struct {
	contentRepository *repositories.ContentRepository
	config            *config.Config
}

func NewContentSetupHandler(
	contentRepository *repositories.ContentRepository,
	config *config.Config,
) handlers.Handler {
	return &contentSetupHandler{
		contentRepository: contentRepository,
		config:            config,
	}
}

func (h *contentSetupHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Extract content name from command text
	contentName := h.extractCommandText(msg)
	if contentName == "" {
		_, err := msg.Reply(b, fmt.Sprintf("Пожалуйста, введи название для контента после команды. Например: %s <название контента>", constants.ContentSetupCommand), nil)
		return err
	}

	// Create content in the database
	id, err := h.contentRepository.CreateContent(contentName, constants.ContentTypeClubCall)
	if err != nil {
		log.Printf("Failed to create content: %v", err)
		_, replyErr := msg.Reply(b, "Произошла ошибка при создании записи о контенте.", nil)
		return replyErr
	}

	_, err = msg.Reply(b, fmt.Sprintf("Запись о контенте '%s' успешно создана с ID: %d", contentName, id), nil)
	return err
}

func (h *contentSetupHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	msg := ctx.EffectiveMessage
	if msg == nil || msg.Text == "" {
		return false
	}

	if strings.HasPrefix(msg.Text, constants.ContentSetupCommand) && msg.Chat.Type == constants.PrivateChatType {
		// Check if the user is an admin in the configured supergroup chat
		if !utils.IsUserAdminOrCreator(b, msg.From.Id, h.config.SuperGroupChatID) {
			msg.Reply(b, "Эта команда доступна только администраторам.", nil)
			log.Printf("User %d tried to use /setupContent without admin rights.", msg.From.Id)
			return false
		}

		// Check if there is text after the command
		if h.extractCommandText(msg) == "" {
			msg.Reply(b, fmt.Sprintf("Пожалуйста, введи название для контента после команды. Например: %s <название контента>", constants.ContentSetupCommand), nil)
			return false
		}

		return true
	}

	return false
}

func (h *contentSetupHandler) Name() string {
	return constants.ContentSetupHandlerName
}

func (h *contentSetupHandler) extractCommandText(msg *gotgbot.Message) string {
	var commandText string
	if strings.HasPrefix(msg.Text, constants.ContentSetupCommand) {
		commandText = strings.TrimPrefix(msg.Text, constants.ContentSetupCommand)
	}
	return strings.TrimSpace(commandText)
}
