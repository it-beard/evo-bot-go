package privatehandlers

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

const (
	GetLastClubCallsHandlerName = "get_last_club_calls_handler"
	lastClubCallsLimit          = 10
)

type GetLastClubCallsHandler struct {
	contentRepository *repositories.ContentRepository
	config            *config.Config
}

func NewGetLastClubCallsHandler(
	contentRepository *repositories.ContentRepository,
	config *config.Config,
) handlers.Handler {
	return &GetLastClubCallsHandler{
		contentRepository: contentRepository,
		config:            config,
	}
}

func (h *GetLastClubCallsHandler) HandleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	clubCalls, err := h.contentRepository.GetLastClubCalls(lastClubCallsLimit)
	if err != nil {
		log.Printf("Failed to get last club calls: %v", err)
		_, replyErr := msg.Reply(b, "Произошла ошибка при получении списка клубных звонков.", nil)
		return replyErr
	}

	if len(clubCalls) == 0 {
		_, err := msg.Reply(b, "Записи о клубных звонках еще не созданы.", nil)
		return err
	}

	var response strings.Builder
	response.WriteString(fmt.Sprintf("Последние %d клубных звонков:\n", len(clubCalls)))
	for _, call := range clubCalls {
		response.WriteString(fmt.Sprintf("- ID: %d, Название: %s\n", call.ID, call.Name))
	}

	_, err = msg.Reply(b, response.String(), nil)
	return err
}

func (h *GetLastClubCallsHandler) CheckUpdate(b *gotgbot.Bot, ctx *ext.Context) bool {
	msg := ctx.EffectiveMessage
	if msg == nil || msg.Text == "" {
		return false
	}

	if strings.HasPrefix(msg.Text, constants.GetLastClubCallsCommand) && msg.Chat.Type == constants.PrivateChat {
		// Check if the user is an admin in the configured supergroup chat
		if !utils.IsUserAdminOrCreator(b, msg.From.Id, h.config.SuperGroupChatID) {
			msg.Reply(b, "Эта команда доступна только администраторам.", nil)
			log.Printf("User %d tried to use /getlastclubcalls without admin rights.", msg.From.Id)
			return false
		}
		return true
	}

	return false
}

func (h *GetLastClubCallsHandler) Name() string {
	return GetLastClubCallsHandlerName
}
