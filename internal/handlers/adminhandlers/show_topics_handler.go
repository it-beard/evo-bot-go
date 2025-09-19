package adminhandlers

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"evo-bot-go/internal/buttons"
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/formatters"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
)

const (
	// Conversation states names
	showTopicsStateSelectEvent = "admin_show_topics_state_select_event"
	showTopicsStateDeleteTopic = "admin_show_topics_state_delete_topic"

	// Context data keys
	showTopicsCtxDataKeyCancelFunc        = "admin_show_topics_ctx_data_cancel_func"
	showTopicsCtxDataKeyEventID           = "admin_show_topics_ctx_data_event_id"
	showTopicsCtxDataKeyPreviousMessageID = "admin_show_topics_ctx_data_previous_message_id"
	showTopicsCtxDataKeyPreviousChatID    = "admin_show_topics_ctx_data_previous_chat_id"

	// Callback data
	showTopicsCallbackConfirmCancel = "admin_show_topics_callback_confirm_cancel"
)

type showTopicsHandler struct {
	config               *config.Config
	topicRepository      *repositories.TopicRepository
	eventRepository      *repositories.EventRepository
	messageSenderService *services.MessageSenderService
	userStore            *utils.UserDataStore
	permissionsService   *services.PermissionsService
}

func NewShowTopicsHandler(
	config *config.Config,
	topicRepository *repositories.TopicRepository,
	eventRepository *repositories.EventRepository,
	messageSenderService *services.MessageSenderService,
	permissionsService *services.PermissionsService,
) ext.Handler {
	h := &showTopicsHandler{
		config:               config,
		topicRepository:      topicRepository,
		eventRepository:      eventRepository,
		messageSenderService: messageSenderService,
		userStore:            utils.NewUserDataStore(),
		permissionsService:   permissionsService,
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.ShowTopicsCommand, h.startShowTopics),
		},
		map[string][]ext.Handler{
			showTopicsStateSelectEvent: {
				handlers.NewMessage(message.All, h.handleEventSelection),
				handlers.NewCallback(callbackquery.Equal(showTopicsCallbackConfirmCancel), h.handleCallbackCancel),
			},
			showTopicsStateDeleteTopic: {
				handlers.NewMessage(message.All, h.handleTopicDeletion),
				handlers.NewCallback(callbackquery.Equal(showTopicsCallbackConfirmCancel), h.handleCallbackCancel),
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

	// Check if user has admin permissions and is in a private chat
	if !h.permissionsService.CheckAdminAndPrivateChat(msg, constants.ShowTopicsCommand) {
		log.Printf("%s: User %d (%s) tried to use /%s without admin permissions.",
			utils.GetCurrentTypeName(),
			ctx.EffectiveUser.Id,
			ctx.EffectiveUser.Username,
			constants.ShowTopicsCommand,
		)
		return handlers.EndConversation()
	}

	// Get last events to show for selection
	events, err := h.eventRepository.GetLastEvents(10)
	if err != nil {
		h.messageSenderService.Reply(msg, "Ошибка при получении списка мероприятий.", nil)
		log.Printf("%s: Error during event retrieval: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	if len(events) == 0 {
		h.messageSenderService.Reply(msg, "Нет доступных мероприятий для просмотра тем и вопросов.", nil)
		return handlers.EndConversation()
	}

	// Format and display event list for admin
	formattedEvents := formatters.FormatEventListForAdmin(
		events,
		"Список мероприятий",
		constants.CancelCommand,
		"для которого ты хочешь увидеть темы и вопросы",
	)

	sentMsg, _ := h.messageSenderService.ReplyMarkdownWithReturnMessage(
		msg,
		formattedEvents,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.CancelButton(showTopicsCallbackConfirmCancel),
		},
	)

	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
	return handlers.NextConversationState(showTopicsStateSelectEvent)
}

// 2. handleEventSelection processes the admin's event selection
func (h *showTopicsHandler) handleEventSelection(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userInput := strings.TrimSpace(strings.Replace(msg.Text, "/", "", 1))

	// Check if the input is a valid event ID
	eventID, err := strconv.Atoi(userInput)
	if err != nil {
		h.messageSenderService.Reply(
			msg,
			fmt.Sprintf("Пожалуйста, отправь корректный ID мероприятия или используй /%s для отмены.", constants.CancelCommand),
			nil,
		)
		return nil // Stay in the same state
	}

	// Get the event information
	event, err := h.eventRepository.GetEventByID(eventID)
	if err != nil {
		h.messageSenderService.Reply(
			msg,
			fmt.Sprintf("Не удалось найти мероприятие с ID %d. Пожалуйста, проверь ID.", eventID),
			nil,
		)
		log.Printf("%s: Error during event retrieval: %v", utils.GetCurrentTypeName(), err)
		return nil // Stay in the same state
	}

	// Get topics for this event
	topics, err := h.topicRepository.GetTopicsByEventID(eventID)
	if err != nil {
		h.messageSenderService.Reply(msg, "Ошибка при получении тем для выбранного мероприятия.", nil)
		log.Printf("%s: Error during topic retrieval: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	// Store the event ID for use in topic deletion
	h.userStore.Set(ctx.EffectiveUser.Id, showTopicsCtxDataKeyEventID, eventID)

	// Format and display topics for admin
	formattedTopics := formatters.FormatHtmlTopicListForAdmin(topics, event.Name, event.Type)
	h.messageSenderService.ReplyHtml(msg, formattedTopics, nil)
	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	// If there are topics, suggest deletion option
	if len(topics) > 0 {
		sentMsg, _ := h.messageSenderService.SendWithReturnMessage(
			msg.Chat.Id,
			"\nДля удаления темы отправь ID темы, которую нужно удалить:",
			&gotgbot.SendMessageOpts{
				ReplyMarkup: buttons.CancelButton(showTopicsCallbackConfirmCancel),
			},
		)
		h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
		return handlers.NextConversationState(showTopicsStateDeleteTopic)
	}

	return handlers.EndConversation()
}

// 3. handleTopicDeletion processes the admin's selection for topic deletion
func (h *showTopicsHandler) handleTopicDeletion(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userInput := strings.TrimSpace(strings.Replace(msg.Text, "/", "", 1))

	// Check if the input is a valid topic ID
	topicID, err := strconv.Atoi(userInput)
	if err != nil {
		h.messageSenderService.Reply(
			msg,
			fmt.Sprintf("Пожалуйста, отправь корректный ID темы или используй /%s для отмены.", constants.CancelCommand),
			nil,
		)
		return nil // Stay in the same state
	}

	// Get the event ID from user store
	eventIDVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, showTopicsCtxDataKeyEventID)
	if !ok {
		h.messageSenderService.Reply(msg, "Ошибка: не найден ID мероприятия в сессии. Попробуй начать сначала.", nil)
		return handlers.EndConversation()
	}
	eventID, ok := eventIDVal.(int)
	if !ok {
		h.messageSenderService.Reply(msg, "Ошибка при получении ID мероприятия из сессии. Попробуй начать сначала.", nil)
		return handlers.EndConversation()
	}

	// Check if the topic exists
	topic, err := h.topicRepository.GetTopicByID(topicID)
	if err != nil {
		h.messageSenderService.Reply(
			msg,
			fmt.Sprintf("Не удалось найти тему с ID %d. Пожалуйста, проверь ID.", topicID),
			nil,
		)
		log.Printf("%s: Error during topic retrieval: %v", utils.GetCurrentTypeName(), err)
		return nil // Stay in the same state
	}

	// Check if the topic belongs to the selected event
	if topic.EventID != eventID {
		h.messageSenderService.Reply(
			msg,
			fmt.Sprintf(
				"Тема с ID %d относится к другому мероприятию (ID: %d), а не к выбранному (ID: %d).\nПожалуйста, выбери корректный ID темы или используй /%s для отмены.",
				topicID, topic.EventID, eventID, constants.CancelCommand,
			),
			nil,
		)
		return nil // Stay in the same state
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	// Delete the topic
	err = h.topicRepository.DeleteTopic(topicID)
	if err != nil {
		h.messageSenderService.Reply(msg, fmt.Sprintf("Ошибка при удалении темы с ID %d.", topicID), nil)
		log.Printf("%s: Error during topic deletion: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	// Confirmation message
	h.messageSenderService.Reply(msg, fmt.Sprintf("✅ Тема с ID %d успешно удалена.", topicID), nil)

	// Get updated list of topics for this event
	topics, err := h.topicRepository.GetTopicsByEventID(eventID)
	if err != nil {
		h.messageSenderService.Reply(msg, "Ошибка при получении обновленного списка тем.", nil)
		log.Printf("%s: Error during topic retrieval: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	// Get the event information for displaying in the updated list
	event, err := h.eventRepository.GetEventByID(eventID)
	if err != nil {
		h.messageSenderService.Reply(msg, "Ошибка при получении информации о мероприятии.", nil)
		log.Printf("%s: Error during event retrieval: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	// Show updated list of topics
	formattedTopics := formatters.FormatHtmlTopicListForAdmin(topics, event.Name, event.Type)
	h.messageSenderService.ReplyHtml(msg, formattedTopics, nil)

	// If there are still topics, allow for more deletions
	if len(topics) > 0 {
		sentMsg, _ := h.messageSenderService.SendWithReturnMessage(
			msg.Chat.Id,
			"\nДля удаления еще одной темы отправь ID темы:",
			&gotgbot.SendMessageOpts{
				ReplyMarkup: buttons.CancelButton(showTopicsCallbackConfirmCancel),
			},
		)
		h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
		return nil // Stay in the same state to allow more deletions
	}

	// No more topics to delete
	h.messageSenderService.Reply(msg, "Все темы удалены.", nil)
	h.userStore.Clear(ctx.EffectiveUser.Id)
	return handlers.EndConversation()
}

// handleCallbackCancel processes the cancel button click
func (h *showTopicsHandler) handleCallbackCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	// Answer the callback query to remove the loading state on the button
	cb := ctx.Update.CallbackQuery
	_, _ = cb.Answer(b, nil)

	return h.handleCancel(b, ctx)
}

// 4. handleCancel handles the /cancel command
func (h *showTopicsHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if there's an ongoing operation to cancel
	if cancelFunc, ok := h.userStore.Get(ctx.EffectiveUser.Id, showTopicsCtxDataKeyCancelFunc); ok {
		// Call the cancel function to stop any ongoing API calls
		if cf, ok := cancelFunc.(context.CancelFunc); ok {
			cf()
			h.messageSenderService.Reply(msg, "Операция просмотра/удаления тем отменена.", nil)
		}
	} else {
		h.messageSenderService.Reply(msg, "Операция просмотра/удаления тем отменена.", nil)
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

func (h *showTopicsHandler) MessageRemoveInlineKeyboard(b *gotgbot.Bot, userID *int64) {
	var chatID, messageID int64

	// If userID provided, get stored message info using the utility method
	if userID != nil {
		messageID, chatID = h.userStore.GetPreviousMessageInfo(
			*userID,
			showTopicsCtxDataKeyPreviousMessageID,
			showTopicsCtxDataKeyPreviousChatID,
		)
	}

	// Skip if we don't have valid chat and message IDs
	if chatID == 0 || messageID == 0 {
		return
	}

	// Use message sender service to remove the inline keyboard
	_ = h.messageSenderService.RemoveInlineKeyboard(chatID, messageID)
}

func (h *showTopicsHandler) SavePreviousMessageInfo(userID int64, sentMsg *gotgbot.Message) {
	h.userStore.SetPreviousMessageInfo(userID, sentMsg.MessageId, sentMsg.Chat.Id,
		showTopicsCtxDataKeyPreviousMessageID, showTopicsCtxDataKeyPreviousChatID)
}
