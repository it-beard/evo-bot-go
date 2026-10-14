package adminhandlers

import (
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
		return handlers.EndConversation()
	}

	// Get last events to show for selection
	events, err := h.eventRepository.GetLastEvents(constants.EventEditGetLastLimit)
	if err != nil {
		h.messageSenderService.Reply(msg, formatters.EventListErrorText, nil)
		log.Printf("%s: Error during event retrieval: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	if len(events) == 0 {
		h.messageSenderService.Reply(msg, fmt.Sprintf(
			"Пока нет ни одного мероприятия. Создать новое можно командой /%s.", constants.EventSetupCommand,
		), nil)
		return handlers.EndConversation()
	}

	// Format and display event list for admin
	formattedEvents := formatters.FormatEventListForAdmin(
		events,
		"📋 Последние мероприятия",
		"для которого ты хочешь увидеть темы и вопросы",
	)

	sentMsg, _ := h.messageSenderService.ReplyHtmlWithReturnMessage(
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
		h.messageSenderService.Reply(msg, formatters.InvalidEventIDText, nil)
		return nil // Stay in the same state
	}

	// Get the event information
	event, err := h.eventRepository.GetEventByID(eventID)
	if err != nil {
		h.messageSenderService.Reply(msg, formatters.EventNotFoundText(eventID), nil)
		log.Printf("%s: Error during event retrieval: %v", utils.GetCurrentTypeName(), err)
		return nil // Stay in the same state
	}

	// Get topics for this event
	topics, err := h.topicRepository.GetTopicsByEventID(eventID)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при получении тем и вопросов для выбранного мероприятия.", nil)
		log.Printf("%s: Error during topic retrieval: %v", utils.GetCurrentTypeName(), err)
		h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)
		h.userStore.Clear(ctx.EffectiveUser.Id)
		return handlers.EndConversation()
	}

	// Store the event ID for use in topic deletion
	h.userStore.Set(ctx.EffectiveUser.Id, showTopicsCtxDataKeyEventID, eventID)

	// Format and display topics for admin
	h.replyTopicList(msg, topics, event)
	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	// If there are topics, suggest deletion option
	if len(topics) > 0 {
		sentMsg, _ := h.messageSenderService.SendWithReturnMessage(
			msg.Chat.Id,
			fmt.Sprintf("Чтобы удалить тему, отправь её ID %s.", formatters.CancelHint),
			&gotgbot.SendMessageOpts{
				ReplyMarkup: buttons.CancelButton(showTopicsCallbackConfirmCancel),
			},
		)
		h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
		return handlers.NextConversationState(showTopicsStateDeleteTopic)
	}

	h.userStore.Clear(ctx.EffectiveUser.Id)
	return handlers.EndConversation()
}

// 3. handleTopicDeletion processes the admin's selection for topic deletion
func (h *showTopicsHandler) handleTopicDeletion(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userInput := strings.TrimSpace(strings.Replace(msg.Text, "/", "", 1))

	// Check if the input is a valid topic ID
	topicID, err := strconv.Atoi(userInput)
	if err != nil {
		h.messageSenderService.Reply(msg, fmt.Sprintf(
			"Неверный ID темы. Пожалуйста, отправь числовой ID темы из списка выше %s.", formatters.CancelHint,
		), nil)
		return nil // Stay in the same state
	}

	// Get the event ID from user store
	eventIDVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, showTopicsCtxDataKeyEventID)
	if !ok {
		h.messageSenderService.Reply(msg, formatters.LostEventText(constants.ShowTopicsCommand), nil)
		h.userStore.Clear(ctx.EffectiveUser.Id)
		return handlers.EndConversation()
	}
	eventID, ok := eventIDVal.(int)
	if !ok {
		h.messageSenderService.Reply(msg, formatters.LostEventText(constants.ShowTopicsCommand), nil)
		h.userStore.Clear(ctx.EffectiveUser.Id)
		return handlers.EndConversation()
	}

	// Check if the topic exists
	topic, err := h.topicRepository.GetTopicByID(topicID)
	if err != nil {
		h.messageSenderService.Reply(msg, fmt.Sprintf(
			"Тема с ID %d не найдена. Пожалуйста, отправь ID темы из списка выше %s.", topicID, formatters.CancelHint,
		), nil)
		log.Printf("%s: Error during topic retrieval: %v", utils.GetCurrentTypeName(), err)
		return nil // Stay in the same state
	}

	// Check if the topic belongs to the selected event
	if topic.EventID != eventID {
		h.messageSenderService.Reply(
			msg,
			fmt.Sprintf(
				"Тема с ID %d относится к другому мероприятию (ID %d), а не к выбранному (ID %d). Пожалуйста, отправь ID темы из списка выше %s.",
				topicID, topic.EventID, eventID, formatters.CancelHint,
			),
			nil,
		)
		return nil // Stay in the same state
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	// Delete the topic
	err = h.topicRepository.DeleteTopic(topicID)
	if err != nil {
		h.messageSenderService.Reply(msg, fmt.Sprintf("Произошла ошибка при удалении темы с ID %d.", topicID), nil)
		log.Printf("%s: Error during topic deletion: %v", utils.GetCurrentTypeName(), err)
		h.userStore.Clear(ctx.EffectiveUser.Id)
		return handlers.EndConversation()
	}

	// Confirmation message
	h.messageSenderService.Reply(msg, fmt.Sprintf("✅ Тема с ID %d удалена.", topicID), nil)

	// Get updated list of topics for this event
	topics, err := h.topicRepository.GetTopicsByEventID(eventID)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при получении обновлённого списка тем.", nil)
		log.Printf("%s: Error during topic retrieval: %v", utils.GetCurrentTypeName(), err)
		h.userStore.Clear(ctx.EffectiveUser.Id)
		return handlers.EndConversation()
	}

	// Get the event information for displaying in the updated list
	event, err := h.eventRepository.GetEventByID(eventID)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при получении информации о мероприятии.", nil)
		log.Printf("%s: Error during event retrieval: %v", utils.GetCurrentTypeName(), err)
		h.userStore.Clear(ctx.EffectiveUser.Id)
		return handlers.EndConversation()
	}

	// Show updated list of topics
	h.replyTopicList(msg, topics, event)

	// If there are still topics, allow for more deletions
	if len(topics) > 0 {
		sentMsg, _ := h.messageSenderService.SendWithReturnMessage(
			msg.Chat.Id,
			fmt.Sprintf("Чтобы удалить ещё одну тему, отправь её ID %s.", formatters.CancelHint),
			&gotgbot.SendMessageOpts{
				ReplyMarkup: buttons.CancelButton(showTopicsCallbackConfirmCancel),
			},
		)
		h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
		return nil // Stay in the same state to allow more deletions
	}

	// No more topics to delete
	h.messageSenderService.Reply(msg, "✅ Все темы этого мероприятия удалены.", nil)
	h.userStore.Clear(ctx.EffectiveUser.Id)
	return handlers.EndConversation()
}

// replyTopicList sends the topics of the event, split into several messages when the list is long
func (h *showTopicsHandler) replyTopicList(msg *gotgbot.Message, topics []repositories.Topic, event *repositories.Event) {
	for _, part := range formatters.FormatHtmlTopicListForAdminMessages(topics, event.Name, event.Type) {
		h.messageSenderService.ReplyHtml(msg, part, nil)
	}
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

	h.messageSenderService.Reply(msg, "Операция просмотра и удаления тем отменена.", nil)

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
	if sentMsg == nil {
		return
	}
	h.userStore.SetPreviousMessageInfo(userID, sentMsg.MessageId, sentMsg.Chat.Id,
		showTopicsCtxDataKeyPreviousMessageID, showTopicsCtxDataKeyPreviousChatID)
}
