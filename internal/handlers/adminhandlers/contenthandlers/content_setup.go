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
	// Conversation states
	stateAskContentName = "ask_content_name"
	stateAskContentType = "ask_content_type"

	// Context data keys
	ctxDataKeyContentName = "content_name"
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
			stateAskContentName: {
				handlers.NewMessage(message.Text, h.handleContentName),
			},
			stateAskContentType: {
				handlers.NewMessage(message.Text, h.handleContentType),
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

	return handlers.NextConversationState(stateAskContentName)
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
	h.userStore.Set(ctx.EffectiveUser.Id, ctxDataKeyContentName, contentName)

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

	return handlers.NextConversationState(stateAskContentType)
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
	contentNameVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, ctxDataKeyContentName)
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

	utils.SendLoggedReply(b, msg, fmt.Sprintf("Запись о контенте '%s' с типом '%s' успешно создана с ID: %d", contentName, contentType, id), nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// 4. handleCancel handles the /cancel command
func (h *contentSetupHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	utils.SendLoggedReply(b, msg, "Операция создания контента отменена.", nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}
