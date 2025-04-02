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
	// Conversation states for content deletion
	stateAskDeleteContentID    = "ask_delete_content_id"
	stateAskDeleteConfirmation = "ask_delete_confirmation"

	// Context data keys
	ctxDataKeyDeleteContentID   = "delete_content_id"
	ctxDataKeyDeleteContentName = "delete_content_name"
)

type contentDeleteHandler struct {
	contentRepo *repositories.ContentRepository
	config      *config.Config
	userStore   *utils.UserDataStore
}

func NewContentDeleteHandler(
	contentRepo *repositories.ContentRepository,
	config *config.Config,
) ext.Handler {
	h := &contentDeleteHandler{
		contentRepo: contentRepo,
		config:      config,
		userStore:   utils.NewUserDataStore(),
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.ContentDeleteCommand, h.startDelete),
		},
		map[string][]ext.Handler{
			stateAskDeleteContentID: {
				handlers.NewMessage(message.Text, h.handleContentID),
			},
			stateAskDeleteConfirmation: {
				handlers.NewMessage(message.Text, h.handleConfirmation),
			},
		},
		&handlers.ConversationOpts{
			Exits: []ext.Handler{handlers.NewCommand(constants.CancelCommand, h.handleCancel)},
		},
	)
}

// 1. startDelete is the entry point handler for the delete conversation
func (h *contentDeleteHandler) startDelete(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check admin permissions and private chat
	if !utils.CheckAdminAndPrivateChat(b, ctx, h.config.SuperGroupChatID, constants.ContentDeleteCommand) {
		return handlers.EndConversation()
	}

	// Get contents for deletion
	contents, err := h.contentRepo.GetLastContents(constants.ContentEditGetLastLimit)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Произошла ошибка при получении списка контента.", err)
		return handlers.EndConversation()
	}

	if len(contents) == 0 {
		utils.SendLoggedReply(b, msg, "Нет доступных контента для удаления.", nil)
		return handlers.EndConversation()
	}

	// Build response with available contents
	var response strings.Builder
	response.WriteString(fmt.Sprintf("Последние %d контента:\n", len(contents)))
	for _, content := range contents {
		response.WriteString(fmt.Sprintf("- ID: %d, Название: %s _(%s)_\n", content.ID, content.Name, content.Type))
	}
	response.WriteString(fmt.Sprintf("\nПожалуйста, отправь ID контента, который ты хочешь удалить, или /%s для отмены.", constants.CancelCommand))
	utils.SendLoggedMarkdownReply(b, msg, response.String(), nil)

	return handlers.NextConversationState(stateAskDeleteContentID)
}

// 2. handleContentID processes the user's selected content ID
func (h *contentDeleteHandler) handleContentID(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	contentIDStr := strings.TrimSpace(msg.Text)

	contentID, err := strconv.Atoi(contentIDStr)
	if err != nil {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Некорректный ID. Пожалуйста, введи числовой ID или /%s для отмены.", constants.CancelCommand), nil)
		return nil // Stay in the same state
	}

	// Get the content to confirm deletion
	contents, err := h.contentRepo.GetLastContents(constants.ContentEditGetLastLimit)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Произошла ошибка при получении списка контента.", err)
		return handlers.EndConversation()
	}

	// Find the content with the given ID
	var found bool
	var contentName, contentType string
	for _, content := range contents {
		if content.ID == contentID {
			found = true
			contentName = content.Name
			contentType = content.Type
			break
		}
	}

	if !found {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Контент с ID %d не найден. Пожалуйста, введи корректный ID или /%s для отмены.", contentID, constants.CancelCommand), nil)
		return nil // Stay in the same state
	}

	// Store content ID and name for confirmation
	h.userStore.Set(ctx.EffectiveUser.Id, ctxDataKeyDeleteContentID, contentID)
	h.userStore.Set(ctx.EffectiveUser.Id, ctxDataKeyDeleteContentName, contentName)

	// Ask for confirmation
	confirmMessage := fmt.Sprintf(
		"Ты собираешься удалить контент:\nID: %d\nНазвание: %s\nТип: %s\n\nПожалуйста, подтверди удаление, отправив 'да' или 'нет', или /%s для отмены.",
		contentID, contentName, contentType, constants.CancelCommand)

	utils.SendLoggedReply(b, msg, confirmMessage, nil)

	return handlers.NextConversationState(stateAskDeleteConfirmation)
}

// 3. handleConfirmation processes the user's confirmation of content deletion
func (h *contentDeleteHandler) handleConfirmation(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	confirmation := strings.ToLower(strings.TrimSpace(msg.Text))

	if confirmation != "да" && confirmation != "нет" {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Пожалуйста, отправь 'да' для подтверждения или 'нет' для отмены (или /%s).", constants.CancelCommand), nil)
		return nil // Stay in the same state
	}

	if confirmation == "нет" {
		utils.SendLoggedReply(b, msg, "Операция удаления контента отменена.", nil)
		// Clean up user data
		h.userStore.Clear(ctx.EffectiveUser.Id)
		return handlers.EndConversation()
	}

	// Get content ID from user data store
	contentIDVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, ctxDataKeyDeleteContentID)
	if !ok {
		utils.SendLoggedReply(
			b,
			msg,
			fmt.Sprintf("Произошла внутренняя ошибка. Не удалось найти ID контента. Попробуй начать заново с /%s.", constants.ContentDeleteCommand),
			nil)
		return handlers.EndConversation()
	}

	contentID, ok := contentIDVal.(int)
	if !ok {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Произошла внутренняя ошибка (неверный тип ID). Попробуй начать заново с /%s.", constants.ContentDeleteCommand), nil)
		return handlers.EndConversation()
	}

	// Get content name for the response message
	contentNameVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, ctxDataKeyDeleteContentName)
	if !ok {
		// Not critical, we can proceed without the name
		contentNameVal = "неизвестно"
	}
	contentName, _ := contentNameVal.(string)

	// Delete content from the database
	err := h.contentRepo.DeleteContent(contentID)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Произошла ошибка при удалении контента.", err)
		return handlers.EndConversation()
	}

	utils.SendLoggedReply(b, msg, fmt.Sprintf("Контент '%s' (ID: %d) успешно удален.", contentName, contentID), nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// 4. handleCancel handles the /cancel command
func (h *contentDeleteHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	utils.SendLoggedReply(b, msg, "Операция удаления контента отменена.", nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}
