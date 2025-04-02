package contenthandlers

import (
	"fmt"
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
	setupCancelCommand    = "cancel"
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
			Exits: []ext.Handler{handlers.NewCommand(setupCancelCommand, h.handleCancel)},
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

	utils.SendLoggedReply(b, msg, fmt.Sprintf("Пожалуйста, введи название для нового контента или /%s для отмены:", setupCancelCommand), nil)

	return handlers.NextConversationState(stateAskContentName)
}

// 2. handleContentName processes the content name input
func (h *contentSetupHandler) handleContentName(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	contentName := strings.TrimSpace(msg.Text)

	if contentName == "" {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Название не может быть пустым. Пожалуйста, введи название для контента или /%s для отмены:", setupCancelCommand), nil)
		return nil // Stay in the same state
	}

	// Store the content name
	h.userStore.Set(ctx.EffectiveUser.Id, ctxDataKeyContentName, contentName)

	// Ask for content type
	typeOptions := fmt.Sprintf("Выбери тип контента (введи число):\n1. %s\n2. %s\nИли /%s для отмены",
		constants.ContentTypeClubCall,
		constants.ContentTypeMeetup,
		constants.CancelCommand,
	)

	utils.SendLoggedReply(b, msg, typeOptions, nil)

	return handlers.NextConversationState(stateAskContentType)
}

// 3. handleContentType processes the content type selection and creates the content
func (h *contentSetupHandler) handleContentType(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	typeSelection := strings.TrimSpace(msg.Text)

	var contentType string
	switch typeSelection {
	case "1":
		contentType = constants.ContentTypeClubCall
	case "2":
		contentType = constants.ContentTypeMeetup
	default:
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Неверный выбор. Пожалуйста, введи 1 или 2, или /%s для отмены:", setupCancelCommand), nil)
		return nil // Stay in the same state
	}

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
