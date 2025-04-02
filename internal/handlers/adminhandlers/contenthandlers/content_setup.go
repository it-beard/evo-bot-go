package contenthandlers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

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
	// Conversation states
	contentSetupStateAskContentName      = "content_setup_ask_content_name"
	contentSetupStateAskContentType      = "content_setup_ask_content_type"
	contentSetupStateAskContentStartedAt = "content_setup_ask_content_started_at"

	// Context data keys
	contentSetupCtxDataKeyContentName = "content_setup_content_name"
	contentSetupCtxDataKeyContentID   = "content_setup_content_id"
)

type contentSetupHandler struct {
	contentRepository *repositories.ContentRepository
	config            *config.Config
	userStore         *utils.UserDataStore
}

func NewContentSetupHandler(
	contentRepository *repositories.ContentRepository,
	config *config.Config,
) ext.Handler {
	h := &contentSetupHandler{
		contentRepository: contentRepository,
		config:            config,
		userStore:         utils.NewUserDataStore(),
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.ContentSetupCommand, h.startSetup),
		},
		map[string][]ext.Handler{
			contentSetupStateAskContentName: {
				handlers.NewMessage(message.Text, h.handleContentName),
			},
			contentSetupStateAskContentType: {
				handlers.NewMessage(message.Text, h.handleContentType),
			},
			contentSetupStateAskContentStartedAt: {
				handlers.NewMessage(message.Text, h.handleContentStartedAt),
			},
		},
		&handlers.ConversationOpts{
			Exits: []ext.Handler{handlers.NewCommand(constants.CancelCommand, h.handleCancel)},
		},
	)
}

// 1. startSetup is the entry point handler for the setup conversation
func (h *contentSetupHandler) startSetup(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check admin permissions and private chat
	if !utils.CheckAdminAndPrivateChat(b, ctx, h.config.SuperGroupChatID, constants.ContentSetupCommand) {
		return handlers.EndConversation()
	}

	utils.SendLoggedReply(b, msg, fmt.Sprintf("Пожалуйста, введи название для нового контента или /%s для отмены:", constants.CancelCommand), nil)

	return handlers.NextConversationState(contentSetupStateAskContentName)
}

// 2. handleContentName processes the content name input
func (h *contentSetupHandler) handleContentName(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	contentName := strings.TrimSpace(msg.Text)

	if contentName == "" {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Название не может быть пустым. Пожалуйста, введи название для контента или /%s для отмены:", constants.CancelCommand), nil)
		return nil // Stay in the same state
	}

	// Store the content name
	h.userStore.Set(ctx.EffectiveUser.Id, contentSetupCtxDataKeyContentName, contentName)

	// Ask for content type
	contentTypeOptions := []string{}
	for i, contentType := range constants.AllContentTypes {
		contentTypeOptions = append(contentTypeOptions, fmt.Sprintf("%d. %s", i+1, contentType))
	}
	typeOptions := fmt.Sprintf("Выбери тип контента (введи число):\n%s\nИли /%s для отмены",
		strings.Join(contentTypeOptions, "\n"),
		constants.CancelCommand,
	)

	utils.SendLoggedReply(b, msg, typeOptions, nil)

	return handlers.NextConversationState(contentSetupStateAskContentType)
}

// 3. handleContentType processes the content type selection and creates the content
func (h *contentSetupHandler) handleContentType(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	typeSelection := strings.TrimSpace(msg.Text)

	var contentType constants.ContentType

	// Convert typeSelection to integer
	index, err := strconv.Atoi(typeSelection)
	if err != nil || index < 1 || index > len(constants.AllContentTypes) {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Неверный выбор. Пожалуйста, введи число от 1 до %d, или /%s для отмены:", len(constants.AllContentTypes), constants.CancelCommand), nil)
		return nil // Stay in the same state
	}

	// Arrays are 0-indexed but our options start from 1
	contentType = constants.AllContentTypes[index-1]

	// Get the content name from user data store
	contentNameVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, contentSetupCtxDataKeyContentName)
	if !ok {
		utils.SendLoggedReply(
			b,
			msg,
			fmt.Sprintf("Произошла внутренняя ошибка. Не удалось найти название контента. Попробуй начать заново с /%s.", constants.ContentSetupCommand),
			nil)
		return handlers.EndConversation()
	}

	contentName, ok := contentNameVal.(string)
	if !ok {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Произошла внутренняя ошибка (неверный тип названия). Попробуй начать заново с /%s.", constants.ContentSetupCommand), nil)
		return handlers.EndConversation()
	}

	// Create content in the database
	id, err := h.contentRepository.CreateContent(contentName, contentType)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Произошла ошибка при создании записи о контенте.", err)
		return handlers.EndConversation()
	}

	// Store the content ID
	h.userStore.Set(ctx.EffectiveUser.Id, contentSetupCtxDataKeyContentID, id)

	// Ask for start date
	utils.SendLoggedReply(b, msg, fmt.Sprintf("Когда стартует контент? Введи дату и время в формате DD.MM.YYYY HH:MM или /%s для отмены.", constants.CancelCommand), nil)

	return handlers.NextConversationState(contentSetupStateAskContentStartedAt)
}

// 4. handleContentStartedAt processes the start date input and updates the content
func (h *contentSetupHandler) handleContentStartedAt(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	dateTimeStr := strings.TrimSpace(msg.Text)

	// Parse the start date
	startedAt, err := time.Parse("02.01.2006 15:04", dateTimeStr)
	if err != nil {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Неверный формат даты. Пожалуйста, введи дату и время в формате DD.MM.YYYY HH:MM или /%s для отмены.", constants.CancelCommand), nil)
		return nil // Stay in the same state
	}

	// Get content ID from user data store
	contentIDVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, contentSetupCtxDataKeyContentID)
	if !ok {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Произошла внутренняя ошибка. Не удалось найти ID контента. Попробуй начать заново с /%s.", constants.ContentSetupCommand), nil)
		return handlers.EndConversation()
	}

	contentID, ok := contentIDVal.(int)
	if !ok {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Произошла внутренняя ошибка (неверный тип ID). Попробуй начать заново с /%s.", constants.ContentSetupCommand), nil)
		return handlers.EndConversation()
	}

	// Update the started_at field
	err = h.contentRepository.UpdateContentStartedAt(contentID, startedAt)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Произошла ошибка при обновлении даты начала контента.", err)
		return handlers.EndConversation()
	}

	// Get content name for the success message
	contentNameVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, contentSetupCtxDataKeyContentName)
	if !ok {
		utils.SendLoggedReply(b, msg, "Контент успешно создан с датой старта.", nil)
		h.userStore.Clear(ctx.EffectiveUser.Id)
		return handlers.EndConversation()
	}

	contentName, ok := contentNameVal.(string)
	if !ok {
		utils.SendLoggedReply(b, msg, "Контент успешно создан с датой старта.", nil)
		h.userStore.Clear(ctx.EffectiveUser.Id)
		return handlers.EndConversation()
	}

	// Success message
	utils.SendLoggedReply(b, msg, fmt.Sprintf("Запись о контенте '%s' успешно создана с ID: %d и датой старта: %s", contentName, contentID, startedAt.Format("02.01.2006 15:04")), nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// 5. handleCancel handles the /cancel command
func (h *contentSetupHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	utils.SendLoggedReply(b, msg, "Операция создания контента отменена.", nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}
