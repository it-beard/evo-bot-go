package topicshandlers

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
	topicAddStateSelectContent = "topic_add_select_content"
	topicAddStateEnterTopic    = "topic_add_enter_topic"

	// UserStore keys
	topicAddUserStoreKeySelectedContentID   = "topic_add_selected_content_id"
	topicAddUserStoreKeySelectedContentName = "topic_add_selected_content_name"
	topicAddUserStoreKeyCancelFunc          = "topic_add_cancel_func"
)

type topicAddHandler struct {
	topicRepository      *repositories.TopicRepository
	contentRepository    *repositories.ContentRepository
	messageSenderService services.MessageSenderService
	config               *config.Config
	userStore            *utils.UserDataStore
}

func NewTopicAddHandler(
	topicRepository *repositories.TopicRepository,
	contentRepository *repositories.ContentRepository,
	messageSenderService services.MessageSenderService,
	config *config.Config,
) ext.Handler {
	h := &topicAddHandler{
		topicRepository:      topicRepository,
		contentRepository:    contentRepository,
		messageSenderService: messageSenderService,
		config:               config,
		userStore:            utils.NewUserDataStore(),
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.TopicAddCommand, h.startTopicAdd),
		},
		map[string][]ext.Handler{
			topicAddStateSelectContent: {
				handlers.NewMessage(message.All, h.handleContentSelection),
			},
			topicAddStateEnterTopic: {
				handlers.NewMessage(message.All, h.handleTopicEntry),
			},
		},
		&handlers.ConversationOpts{
			Exits: []ext.Handler{handlers.NewCommand(constants.CancelCommand, h.handleCancel)},
		},
	)
}

// 1. startTopicAdd is the entry point handler for adding a topic
func (h *topicAddHandler) startTopicAdd(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Only proceed if this is a private chat
	if !utils.CheckPrivateChatType(b, ctx) {
		return handlers.EndConversation()
	}

	// Check if user is a club member
	if !utils.CheckClubMemberPermissions(b, msg, h.config, constants.TopicAddCommand) {
		return handlers.EndConversation()
	}

	// Get last actual contents to show for selection
	contents, err := h.contentRepository.GetLastActualContents(10)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Ошибка при получении списка контента.", err)
		return handlers.EndConversation()
	}

	if len(contents) == 0 {
		utils.SendLoggedReply(b, msg, "Нет доступного контента для добавления тем.", nil)
		return handlers.EndConversation()
	}

	// Format and display content list for selection
	formattedContents := utils.FormatContentListForUsers(
		contents,
		fmt.Sprintf("Выбери ID мероприятия, на которое ты хочешь закинуть тему или вопросы, либо жми /%s для отмены диалога", constants.CancelCommand),
	)

	utils.SendLoggedMarkdownReply(b, msg, formattedContents, nil)

	return handlers.NextConversationState(topicAddStateSelectContent)
}

// 2. handleContentSelection processes the user's content selection
func (h *topicAddHandler) handleContentSelection(b *gotgbot.Bot, ctx *ext.Context) error {
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

	// Store the selected content ID for later use when creating a new topic
	h.userStore.Set(ctx.EffectiveUser.Id, topicAddUserStoreKeySelectedContentID, contentID)
	h.userStore.Set(ctx.EffectiveUser.Id, topicAddUserStoreKeySelectedContentName, content.Name)

	// Prompt user to enter a topic
	utils.SendLoggedMarkdownReply(
		b,
		msg,
		fmt.Sprintf("Отправь мне темы или вопросы к мероприятию *%s*, либо используй /%s для отмены диалога.", content.Name, constants.CancelCommand),
		nil,
	)

	return handlers.NextConversationState(topicAddStateEnterTopic)
}

// 3. handleTopicEntry handles the user's topic input
func (h *topicAddHandler) handleTopicEntry(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	topicText := strings.TrimSpace(msg.Text)

	if topicText == "" {
		utils.SendLoggedReply(
			b,
			msg,
			"Тема не может быть пустой. Пожалуйста, введи текст темы или /cancel для отмены.",
			nil,
		)
		return nil // Stay in the same state
	}

	// Get the selected content ID from user store
	contentIDInterface, ok := h.userStore.Get(ctx.EffectiveUser.Id, topicAddUserStoreKeySelectedContentID)
	if !ok {
		utils.SendLoggedReply(
			b,
			msg,
			"Произошла ошибка: не найден выбранный контент. Пожалуйста, начни заново.",
			nil,
		)
		return handlers.EndConversation()
	}

	contentID := contentIDInterface.(int)
	userNickname := "не указано"
	if ctx.EffectiveUser.Username != "" {
		userNickname = ctx.EffectiveUser.Username
	}

	// Create the new topic
	_, err := h.topicRepository.CreateTopic(topicText, userNickname, contentID)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Ой! Ошибка записи в базу данных...", err)
		return handlers.EndConversation()
	}

	utils.SendLoggedReply(b, msg,
		fmt.Sprintf("Добавлено! \nИспользуй команду /%s для просмотра всех тем и вопросов к мероприятию.", constants.TopicsShowCommand),
		nil,
	)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// 4. handleCancel handles the /cancel command
func (h *topicAddHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if there's an ongoing operation to cancel
	if cancelFunc, ok := h.userStore.Get(ctx.EffectiveUser.Id, topicAddUserStoreKeyCancelFunc); ok {
		// Call the cancel function to stop any ongoing API calls
		if cf, ok := cancelFunc.(context.CancelFunc); ok {
			cf()
			utils.SendLoggedReply(b, msg, "Операция добавления темы отменена.", nil)
		}
	} else {
		utils.SendLoggedReply(b, msg, "Операция добавления темы отменена.", nil)
	}

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}
