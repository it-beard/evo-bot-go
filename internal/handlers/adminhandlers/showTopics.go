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
	showTopicsStateDeleteTopic   = "admin_show_topics_delete_topic"

	// UserStore keys
	showTopicsUserStoreKeyCancelFunc = "admin_show_topics_cancel_func"
	showTopicsUserStoreContentID     = "admin_show_topics_content_id"
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
			showTopicsStateDeleteTopic: {
				handlers.NewMessage(message.All, h.handleTopicDeletion),
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

	// Store the content ID for use in topic deletion
	h.userStore.Set(ctx.EffectiveUser.Id, showTopicsUserStoreContentID, contentID)

	// Format and display topics for admin
	formattedTopics := utils.FormatTopicListForAdmin(topics, content.Name, content.Type)
	utils.SendLoggedMarkdownReply(b, msg, formattedTopics, nil)

	// If there are topics, suggest deletion option
	if len(topics) > 0 {
		suggestionMsg := fmt.Sprintf("\nДля удаления темы отправь ID темы, которую нужно удалить, или /%s для отмены.", constants.CancelCommand)
		utils.SendLoggedReply(b, msg, suggestionMsg, nil)
		return handlers.NextConversationState(showTopicsStateDeleteTopic)
	}

	return handlers.EndConversation()
}

// 3. handleTopicDeletion processes the admin's selection for topic deletion
func (h *showTopicsHandler) handleTopicDeletion(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userInput := strings.TrimSpace(msg.Text)

	// Check if the input is a valid topic ID
	topicID, err := strconv.Atoi(userInput)
	if err != nil {
		utils.SendLoggedReply(
			b,
			msg,
			fmt.Sprintf("Пожалуйста, отправь корректный ID темы или /%s для отмены.", constants.CancelCommand),
			nil,
		)
		return nil // Stay in the same state
	}

	// Get the content ID from user store
	contentIDVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, showTopicsUserStoreContentID)
	if !ok {
		utils.SendLoggedReply(b, msg, "Ошибка: не найден ID контента в сессии. Попробуй начать сначала.", nil)
		return handlers.EndConversation()
	}
	contentID, ok := contentIDVal.(int)
	if !ok {
		utils.SendLoggedReply(b, msg, "Ошибка при получении ID контента из сессии. Попробуй начать сначала.", nil)
		return handlers.EndConversation()
	}

	// Check if the topic exists
	topic, err := h.topicRepository.GetTopicByID(topicID)
	if err != nil {
		utils.SendLoggedReply(
			b,
			msg,
			fmt.Sprintf("Не удалось найти тему с ID %d. Пожалуйста, проверь ID.", topicID),
			err,
		)
		return nil // Stay in the same state
	}

	// Check if the topic belongs to the selected content
	if topic.ContentID != contentID {
		utils.SendLoggedReply(
			b,
			msg,
			fmt.Sprintf(
				"Тема с ID %d относится к другому мероприятию (контент ID: %d), а не к выбранному (контент ID: %d).\nПожалуйста, выбери корректный ID темы или /%s для отмены.",
				topicID, topic.ContentID, contentID, constants.CancelCommand,
			),
			nil,
		)
		return nil // Stay in the same state
	}

	// Delete the topic
	err = h.topicRepository.DeleteTopic(topicID)
	if err != nil {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Ошибка при удалении темы с ID %d.", topicID), err)
		return handlers.EndConversation()
	}

	// Confirmation message
	utils.SendLoggedReply(b, msg, fmt.Sprintf("✅ Тема с ID %d успешно удалена.", topicID), nil)

	// Get updated list of topics for this content
	topics, err := h.topicRepository.GetTopicsByContentID(contentID)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Ошибка при получении обновленного списка тем.", err)
		return handlers.EndConversation()
	}

	// Get the content information for displaying in the updated list
	content, err := h.contentRepository.GetContentByID(contentID)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Ошибка при получении информации о контенте.", err)
		return handlers.EndConversation()
	}

	// Show updated list of topics
	formattedTopics := utils.FormatTopicListForAdmin(topics, content.Name, content.Type)
	utils.SendLoggedMarkdownReply(b, msg, formattedTopics, nil)

	// If there are still topics, allow for more deletions
	if len(topics) > 0 {
		suggestionMsg := fmt.Sprintf("\nДля удаления еще одной темы отправь ID темы, или /%s для завершения.", constants.CancelCommand)
		utils.SendLoggedReply(b, msg, suggestionMsg, nil)
		return nil // Stay in the same state to allow more deletions
	}

	// No more topics to delete
	utils.SendLoggedReply(b, msg, "Все темы удалены.", nil)
	return handlers.EndConversation()
}

// 4. handleCancel handles the /cancel command
func (h *showTopicsHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if there's an ongoing operation to cancel
	if cancelFunc, ok := h.userStore.Get(ctx.EffectiveUser.Id, showTopicsUserStoreKeyCancelFunc); ok {
		// Call the cancel function to stop any ongoing API calls
		if cf, ok := cancelFunc.(context.CancelFunc); ok {
			cf()
			utils.SendLoggedReply(b, msg, "Операция просмотра/удаления тем отменена.", nil)
		}
	} else {
		utils.SendLoggedReply(b, msg, "Операция просмотра/удаления тем отменена.", nil)
	}

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}
