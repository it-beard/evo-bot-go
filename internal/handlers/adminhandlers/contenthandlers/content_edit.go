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
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
)

const (
	// Conversation states
	contentEditStateAskContentID    = "content_edit_ask_content_id"
	contentEditStateAskEditField    = "content_edit_ask_edit_field"
	contentEditStateAskNewName      = "content_edit_ask_new_name"
	contentEditStateAskNewStartedAt = "content_edit_ask_new_started_at"

	// Context data keys
	contentEditCtxDataKeyContentID = "content_edit_content_id"
	contentEditCtxDataKeyEditField = "content_edit_field"
)

// Edit field options
const (
	editFieldName      = "name"
	editFieldStartedAt = "started_at"
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
			contentEditStateAskContentID: {
				handlers.NewMessage(message.Text, h.handleContentID),
			},
			contentEditStateAskEditField: {
				handlers.NewMessage(message.Text, h.handleEditField),
			},
			contentEditStateAskNewName: {
				handlers.NewMessage(message.Text, h.handleNewName),
			},
			contentEditStateAskNewStartedAt: {
				handlers.NewMessage(message.Text, h.handleNewStartedAt),
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

	title := fmt.Sprintf("Последние %d контента:", len(contents))
	actionDescription := "который ты хочешь отредактировать"
	formattedResponse := utils.FormatContentList(contents, title, constants.CancelCommand, actionDescription)

	utils.SendLoggedMarkdownReply(b, msg, formattedResponse, nil)

	return handlers.NextConversationState(contentEditStateAskContentID)
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

	// Check if content with this ID exists
	_, err = h.contentRepo.GetContentByID(contentID)
	if err != nil {
		log.Printf("Error checking content with ID %d: %v", contentID, err)
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Контент с ID %d не найден. Пожалуйста, введи существующий ID или /%s для отмены.", contentID, constants.CancelCommand), nil)
		return nil // Stay in the same state
	}

	h.userStore.Set(ctx.EffectiveUser.Id, contentEditCtxDataKeyContentID, contentID)

	// Ask what field to edit
	if _, err := msg.Reply(b, fmt.Sprintf("Что ты хочешь отредактировать?\n1. Название\n2. Дату старта\nВведи номер или /%s для отмены.", constants.CancelCommand), nil); err != nil {
		log.Printf("Error asking what to edit: %v", err)
	}

	return handlers.NextConversationState(contentEditStateAskEditField)
}

// 3. handleEditField processes the user's choice of what field to edit
func (h *contentEditHandler) handleEditField(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	choice := strings.TrimSpace(msg.Text)

	switch choice {
	case "1":
		h.userStore.Set(ctx.EffectiveUser.Id, contentEditCtxDataKeyEditField, editFieldName)
		if _, err := msg.Reply(b, fmt.Sprintf("Введи новое название для этого контента, или /%s для отмены.", constants.CancelCommand), nil); err != nil {
			log.Printf("Error asking for new name: %v", err)
		}
		return handlers.NextConversationState(contentEditStateAskNewName)
	case "2":
		h.userStore.Set(ctx.EffectiveUser.Id, contentEditCtxDataKeyEditField, editFieldStartedAt)
		if _, err := msg.Reply(b, fmt.Sprintf("Введи новую дату и время старта в формате DD.MM.YYYY HH:MM или /%s для отмены.", constants.CancelCommand), nil); err != nil {
			log.Printf("Error asking for new start date: %v", err)
		}
		return handlers.NextConversationState(contentEditStateAskNewStartedAt)
	default:
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Неверный выбор. Пожалуйста, введи 1 или 2, или /%s для отмены.", constants.CancelCommand), nil)
		return nil // Stay in the same state
	}
}

// 4. handleNewName processes the new name and updates the content
func (h *contentEditHandler) handleNewName(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	newName := strings.TrimSpace(msg.Text)

	if newName == "" {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Название не может быть пустым. Попробуй еще раз или /%s для отмены.", constants.CancelCommand), nil)
		return nil // Stay in the same state
	}

	// Get the content ID from user data store
	contentIDVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, contentEditCtxDataKeyContentID)
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

// 5. handleNewStartedAt processes the new start date and updates the content
func (h *contentEditHandler) handleNewStartedAt(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	dateTimeStr := strings.TrimSpace(msg.Text)

	// Parse the start date
	startedAt, err := time.Parse("02.01.2006 15:04", dateTimeStr)
	if err != nil {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Неверный формат даты. Пожалуйста, введи дату и время в формате DD.MM.YYYY HH:MM или /%s для отмены.", constants.CancelCommand), nil)
		return nil // Stay in the same state
	}

	// Get the content ID from user data store
	contentIDVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, contentEditCtxDataKeyContentID)
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

	// Update the started_at field in the database
	err = h.contentRepo.UpdateContentStartedAt(contentID, startedAt)
	if err != nil {
		log.Printf("Failed to update content started_at for ID %d: %v", contentID, err)
		errorMsg := "Произошла ошибка при обновлении даты старта контента в базе данных."
		if strings.Contains(err.Error(), "no content found") {
			errorMsg = fmt.Sprintf("Не удалось найти контент с ID %d для обновления.", contentID)
		}
		utils.SendLoggedReply(b, msg, errorMsg, err)
		return handlers.EndConversation()
	}

	utils.SendLoggedReply(b, msg, fmt.Sprintf("Дата старта контента с ID %d успешно обновлена на '%s'.", contentID, startedAt.Format("02.01.2006 15:04")), nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// 6. handleCancel handles the /cancel command
func (h *contentEditHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	utils.SendLoggedReply(b, msg, "Операция редактирования отменена.", nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}
