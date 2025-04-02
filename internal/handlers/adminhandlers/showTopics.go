package adminhandlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
)

const (
	// Conversation states
	showTopicsStateSelectContent = "admin_show_topics_select_content"

	// UserStore keys
	showTopicsUserStoreKeyCancelFunc = "admin_show_topics_cancel_func"
)

type showTopicsHandler struct {
	topicRepository      *repositories.TopicRepository
	contentRepository    *repositories.ContentRepository
	messageSenderService services.MessageSenderService
	config               *config.Config
	userStore            *utils.UserDataStore
}

func NewShowTopicsHandler(
	topicRepository *repositories.TopicRepository,
	contentRepository *repositories.ContentRepository,
	messageSenderService services.MessageSenderService,
	config *config.Config,
) ext.Handler {
	h := &showTopicsHandler{
		topicRepository:      topicRepository,
		contentRepository:    contentRepository,
		messageSenderService: messageSenderService,
		config:               config,
		userStore:            utils.NewUserDataStore(),
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.ShowTopicsCommand, h.startShowTopics),
		},
		map[string][]ext.Handler{
			showTopicsStateSelectContent: {
				handlers.NewMessage(message.All, h.handleContentSelection),
			},
		},
		&handlers.ConversationOpts{
			Exits: []ext.Handler{handlers.NewCommand(constants.CancelCommand, h.handleCancel)},
		},
	)
}

// 1. startShowTopics is the entry point handler for showing topics for admins
func (h *showTopicsHandler) startShowTopics(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Only proceed if this is a private chat
	if !utils.CheckPrivateChatType(b, ctx) {
		return handlers.EndConversation()
	}

	// Check if user is an admin
	superGroupChatID := h.config.SuperGroupChatID
	if !utils.CheckAdminPermissions(b, ctx, superGroupChatID, constants.ShowTopicsCommand) {
		return handlers.EndConversation()
	}

	// Get last contents to show for selection
	contents, err := h.contentRepository.GetLastContents(10)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Ошибка при получении списка контента.", err)
		return handlers.EndConversation()
	}

	if len(contents) == 0 {
		utils.SendLoggedReply(b, msg, "Нет доступного контента для просмотра тем и вопросов.", nil)
		return handlers.EndConversation()
	}

	// Format and display content list for admin
	formattedContents := utils.FormatContentListForAdmin(
		contents,
		"Список мероприятий",
		constants.CancelCommand,
		"для которого ты хочешь увидеть темы и вопросы",
	)

	utils.SendLoggedMarkdownReply(b, msg, formattedContents, nil)

	return handlers.NextConversationState(showTopicsStateSelectContent)
}

// 2. handleContentSelection processes the admin's content selection
func (h *showTopicsHandler) handleContentSelection(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userInput := strings.TrimSpace(msg.Text)

	// Check if the input is a valid content ID
	contentID, err := strconv.Atoi(userInput)
	if err != nil {
		utils.SendLoggedReply(
			b,
			msg,
			fmt.Sprintf("Пожалуйста, отправь корректный ID контента или /%s для отмены.", constants.CancelCommand),
			nil,
		)
		return nil // Stay in the same state
	}

	// Get the content information
	content, err := h.contentRepository.GetContentByID(contentID)
	if err != nil {
		utils.SendLoggedReply(
			b,
			msg,
			fmt.Sprintf("Не удалось найти контент с ID %d. Пожалуйста, проверь ID.", contentID),
			err,
		)
		return nil // Stay in the same state
	}

	// Get topics for this content
	topics, err := h.topicRepository.GetTopicsByContentID(contentID)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Ошибка при получении тем для выбранного контента.", err)
		return handlers.EndConversation()
	}

	// Format and display topics for admin
	formattedTopics := utils.FormatTopicListForAdmin(topics, content.Name, content.Type)
	utils.SendLoggedMarkdownReply(b, msg, formattedTopics, nil)

	return handlers.EndConversation()
}

// 3. handleCancel handles the /cancel command
func (h *showTopicsHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if there's an ongoing operation to cancel
	if cancelFunc, ok := h.userStore.Get(ctx.EffectiveUser.Id, showTopicsUserStoreKeyCancelFunc); ok {
		// Call the cancel function to stop any ongoing API calls
		if cf, ok := cancelFunc.(context.CancelFunc); ok {
			cf()
			utils.SendLoggedReply(b, msg, "Операция просмотра тем отменена.", nil)
		}
	} else {
		utils.SendLoggedReply(b, msg, "Операция просмотра тем отменена.", nil)
	}

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}
