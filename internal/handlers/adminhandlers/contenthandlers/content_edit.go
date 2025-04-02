package contenthandlers

import (
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/utils"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
)

const (
	// Conversation states
	stateAskContentID = "ask_content_id"
	stateAskNewName   = "ask_new_name"

	// Context data keys
	ctxDataKeyContentID = "content_id"
)

type contentEditHandler struct {
	contentRepo *repositories.ContentRepository
	config      *config.Config
	userStore   *utils.UserDataStore
}

func NewContentEditHandler(
	contentRepo *repositories.ContentRepository,
	config *config.Config,
) ext.Handler {
	h := &contentEditHandler{
		contentRepo: contentRepo,
		config:      config,
		userStore:   utils.NewUserDataStore(),
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.ContentEditCommand, h.startEdit),
		},
		map[string][]ext.Handler{
			stateAskContentID: {
				handlers.NewMessage(message.Text, h.handleContentID),
			},
			stateAskNewName: {
				handlers.NewMessage(message.Text, h.handleNewName),
			},
		},
		&handlers.ConversationOpts{
			Exits: []ext.Handler{handlers.NewCommand(constants.CancelCommand, h.handleCancel)},
		},
	)
}

// 1. startEdit is the entry point handler for the edit conversation
func (h *contentEditHandler) startEdit(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check admin permissions and private chat
	if !utils.CheckAdminAndPrivateChat(b, ctx, h.config.SuperGroupChatID, constants.ContentEditCommand) {
		return handlers.EndConversation()
	}

	// Get contents for editing
	contents, err := h.contentRepo.GetLastContents(constants.ContentEditGetLastLimit)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Произошла ошибка при получении списка контента.", err)
		return handlers.EndConversation()
	}

	if len(contents) == 0 {
		utils.SendLoggedReply(b, msg, "Нет доступных контента для редактирования.", nil)
		return handlers.EndConversation()
	}

	// Build response with available contents
	var response strings.Builder
	response.WriteString(fmt.Sprintf("Последние %d контента:\n", len(contents)))
	for _, content := range contents {
		response.WriteString(fmt.Sprintf("- ID: %d, Название: %s _(%s)_\n", content.ID, content.Name, content.Type))
	}
	response.WriteString(fmt.Sprintf("\nПожалуйста, отправь ID контента, который ты хочешь отредактировать, или /%s для отмены.", constants.CancelCommand))
	utils.SendLoggedMarkdownReply(b, msg, response.String(), nil)

	return handlers.NextConversationState(stateAskContentID)
}

// 2. handleContentID processes the user's selected content ID
func (h *contentEditHandler) handleContentID(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	contentIDStr := strings.TrimSpace(msg.Text)

	contentID, err := strconv.Atoi(contentIDStr)
	if err != nil {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Неверный ID. Пожалуйста, введи числовой ID или /%s для отмены.", constants.CancelCommand), nil)
		return nil // Stay in the same state
	}

	h.userStore.Set(ctx.EffectiveUser.Id, ctxDataKeyContentID, contentID)

	if _, err := msg.Reply(b, fmt.Sprintf("Хорошо. Теперь введи новое название для этого контента, или /%s для отмены.", constants.CancelCommand), nil); err != nil {
		log.Printf("Error asking for new name: %v", err)
	}

	return handlers.NextConversationState(stateAskNewName)
}

// 3. handleNewName processes the new name and updates the content
func (h *contentEditHandler) handleNewName(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	newName := strings.TrimSpace(msg.Text)

	if newName == "" {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Название не может быть пустым. Попробуй еще раз или /%s для отмены.", constants.CancelCommand), nil)
		return nil // Stay in the same state
	}

	// Get the content ID from user data store
	contentIDVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, ctxDataKeyContentID)
	if !ok {
		utils.SendLoggedReply(
			b,
			msg,
			fmt.Sprintf("Произошла внутренняя ошибка. Не удалось найти ID контента. Попробуй начать заново с /%s.", constants.ContentEditCommand),
			nil,
		)
		return handlers.EndConversation()
	}

	contentID, ok := contentIDVal.(int)
	if !ok {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Произошла внутренняя ошибка (неверный тип ID). Попробуй начать заново с /%s.", constants.ContentEditCommand), nil)
		return handlers.EndConversation()
	}

	// Update the name in the database
	err := h.contentRepo.UpdateContentName(contentID, newName)
	if err != nil {
		log.Printf("Failed to update content name for ID %d: %v", contentID, err)
		errorMsg := "Произошла ошибка при обновлении названия контента в базе данных."
		if strings.Contains(err.Error(), "no content found") {
			errorMsg = fmt.Sprintf("Не удалось найти контент с ID %d для обновления.", contentID)
		}
		utils.SendLoggedReply(b, msg, errorMsg, err)
		return handlers.EndConversation()
	}

	utils.SendLoggedReply(b, msg, fmt.Sprintf("Название контента с ID %d успешно обновлено на '%s'.", contentID, newName), nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// 4. handleCancel handles the /cancel command
func (h *contentEditHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	utils.SendLoggedReply(b, msg, "Операция редактирования отменена.", nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}
