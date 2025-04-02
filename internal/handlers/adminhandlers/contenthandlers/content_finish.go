package contenthandlers

import (
	"fmt"
	"strconv"
	"strings"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
)

const (
	// Conversation states for content status change
	stateAskFinishContentID = "ask_finish_content_id"

	// Context data keys
	ctxDataKeyFinishContentID = "finish_content_id"
)

type contentFinishHandler struct {
	contentRepo *repositories.ContentRepository
	config      *config.Config
	userStore   *utils.UserDataStore
}

func NewContentFinishHandler(
	contentRepo *repositories.ContentRepository,
	config *config.Config,
) ext.Handler {
	h := &contentFinishHandler{
		contentRepo: contentRepo,
		config:      config,
		userStore:   utils.NewUserDataStore(),
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.ContentFinishCommand, h.startFinish),
		},
		map[string][]ext.Handler{
			stateAskFinishContentID: {
				handlers.NewMessage(message.Text, h.handleContentID),
			},
		},
		&handlers.ConversationOpts{
			Exits: []ext.Handler{handlers.NewCommand(constants.CancelCommand, h.handleCancel)},
		},
	)
}

// startFinish is the entry point handler for the finish conversation
func (h *contentFinishHandler) startFinish(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check admin permissions and private chat
	if !utils.CheckAdminAndPrivateChat(b, ctx, h.config.SuperGroupChatID, constants.ContentFinishCommand) {
		return handlers.EndConversation()
	}

	// Get contents for changing status
	contents, err := h.contentRepo.GetLastContents(constants.ContentEditGetLastLimit)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Произошла ошибка при получении списка контента.", err)
		return handlers.EndConversation()
	}

	if len(contents) == 0 {
		utils.SendLoggedReply(b, msg, "Нет доступных контента для изменения статуса.", nil)
		return handlers.EndConversation()
	}

	title := fmt.Sprintf("Последние %d контента:", len(contents))
	actionDescription := "статус которого ты хочешь изменить на 'finished'"
	formattedResponse := utils.FormatContentListForAdmin(contents, title, constants.CancelCommand, actionDescription)

	utils.SendLoggedMarkdownReply(b, msg, formattedResponse, nil)

	return handlers.NextConversationState(stateAskFinishContentID)
}

// handleContentID processes the user's selected content ID and changes its status
func (h *contentFinishHandler) handleContentID(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	contentIDStr := strings.TrimSpace(msg.Text)

	contentID, err := strconv.Atoi(contentIDStr)
	if err != nil {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Неверный ID. Пожалуйста, введи числовой ID или /%s для отмены.", constants.CancelCommand), nil)
		return nil // Stay in the same state
	}

	// Update the status in the database to ContentStatusFinished
	err = h.contentRepo.UpdateContentStatus(contentID, constants.ContentStatusFinished)
	if err != nil {
		errorMsg := "Произошла ошибка при обновлении статуса контента в базе данных."
		if strings.Contains(err.Error(), "no content found") {
			errorMsg = fmt.Sprintf("Не удалось найти контент с ID %d для обновления статуса.", contentID)
		}
		utils.SendLoggedReply(b, msg, errorMsg, err)
		return handlers.EndConversation()
	}

	utils.SendLoggedReply(b, msg, fmt.Sprintf("Статус контента с ID %d успешно изменен на '%s'.", contentID, constants.ContentStatusFinished), nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// handleCancel handles the /cancel command
func (h *contentFinishHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	utils.SendLoggedReply(b, msg, "Операция изменения статуса отменена.", nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}
