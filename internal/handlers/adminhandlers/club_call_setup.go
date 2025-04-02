package adminhandlers

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

type ClubCallSetupHandler struct {
	contentRepository *repositories.ContentRepository
	config            *config.Config
}

func NewClubCallSetupHandler(
	contentRepository *repositories.ContentRepository,
	config *config.Config,
) handlers.Handler {
	return &ClubCallSetupHandler{
		contentRepository: contentRepository,
		config:            config,
	}
}

func (h *ClubCallSetupHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Extract club call name from command text
	clubCallName := h.extractCommandText(msg)
	if clubCallName == "" {
		_, err := msg.Reply(b, fmt.Sprintf("Пожалуйста, введи название для клубного звонка после команды. Например: %s <название звонка>", constants.ClubCallSetupCommand), nil)
		return err
	}

	// Create content in the database
	id, err := h.contentRepository.CreateContent(clubCallName, "club-call")
	if err != nil {
		log.Printf("Failed to create club call content: %v", err)
		_, replyErr := msg.Reply(b, "Произошла ошибка при создании записи о клубном звонке.", nil)
		return replyErr
	}

	_, err = msg.Reply(b, fmt.Sprintf("Запись о клубном звонке '%s' успешно создана с ID: %d", clubCallName, id), nil)
	return err
}

func (h *ClubCallSetupHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	msg := ctx.EffectiveMessage
	if msg == nil || msg.Text == "" {
		return false
	}

	if strings.HasPrefix(msg.Text, constants.ClubCallSetupCommand) && msg.Chat.Type == constants.PrivateChatType {
		// Check if the user is an admin in the configured supergroup chat
		if !utils.IsUserAdminOrCreator(b, msg.From.Id, h.config.SuperGroupChatID) {
			msg.Reply(b, "Эта команда доступна только администраторам.", nil)
			log.Printf("User %d tried to use /setupClubCall without admin rights.", msg.From.Id)
			return false
		}

		// Check if there is text after the command
		if h.extractCommandText(msg) == "" {
			msg.Reply(b, fmt.Sprintf("Пожалуйста, введи название для клубного звонка после команды. Например: %s <название звонка>", constants.ClubCallSetupCommand), nil)
			return false
		}

		return true
	}

	return false
}

func (h *ClubCallSetupHandler) Name() string {
	return constants.ClubCallSetupHandlerName
}

func (h *ClubCallSetupHandler) extractCommandText(msg *gotgbot.Message) string {
	var commandText string
	if strings.HasPrefix(msg.Text, constants.ClubCallSetupCommand) {
		commandText = strings.TrimPrefix(msg.Text, constants.ClubCallSetupCommand)
	}
	return strings.TrimSpace(commandText)
}
