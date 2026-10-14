package topicshandlers

import (
	"fmt"
	"html"
	"log"
	"strconv"
	"strings"
	"time"

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
	topicAddStateSelectEvent = "topic_add_state_select_event"
	topicAddStateEnterTopic  = "topic_add_state_enter_topic"

	// Context data keys
	topicAddCtxDataKeySelectedEventID   = "topic_add_ctx_data_selected_event_id"
	topicAddCtxDataKeySelectedEventName = "topic_add_ctx_data_selected_event_name"
	topicAddCtxDataKeyPreviousMessageID = "topic_add_ctx_data_previous_message_id"
	topicAddCtxDataKeyPreviousChatID    = "topic_add_ctx_data_previous_chat_id"

	// Callback data
	topicAddCallbackConfirmCancel = "topic_add_callback_confirm_cancel"
)

type topicAddHandler struct {
	config               *config.Config
	topicRepository      *repositories.TopicRepository
	eventRepository      *repositories.EventRepository
	messageSenderService *services.MessageSenderService
	userStore            *utils.UserDataStore
	permissionsService   *services.PermissionsService
}

func NewTopicAddHandler(
	config *config.Config,
	topicRepository *repositories.TopicRepository,
	eventRepository *repositories.EventRepository,
	messageSenderService *services.MessageSenderService,
	permissionsService *services.PermissionsService,
) ext.Handler {
	h := &topicAddHandler{
		config:               config,
		topicRepository:      topicRepository,
		eventRepository:      eventRepository,
		messageSenderService: messageSenderService,
		userStore:            utils.NewUserDataStore(),
		permissionsService:   permissionsService,
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.TopicAddCommand, h.startTopicAdd),
		},
		map[string][]ext.Handler{
			topicAddStateSelectEvent: {
				handlers.NewMessage(message.All, h.handleEventSelection),
				handlers.NewCallback(callbackquery.Equal(topicAddCallbackConfirmCancel), h.handleCallbackCancel),
			},
			topicAddStateEnterTopic: {
				handlers.NewMessage(message.All, h.handleTopicEntry),
				handlers.NewCallback(callbackquery.Equal(topicAddCallbackConfirmCancel), h.handleCallbackCancel),
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
	if !h.permissionsService.CheckPrivateChatType(msg) {
		return handlers.EndConversation()
	}

	// Check if user is a club member
	if !h.permissionsService.CheckClubMemberPermissions(msg, constants.TopicAddCommand) {
		return handlers.EndConversation()
	}

	// Get upcoming events to show for selection
	events, err := h.eventRepository.GetUpcomingEvents(10)
	if err != nil {
		h.messageSenderService.Reply(msg, formatters.EventListErrorText, nil)
		log.Printf("%s: Error during events retrieval: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	if len(events) == 0 {
		h.messageSenderService.Reply(msg, "Ближайших мероприятий пока нет, поэтому добавить к ним темы и вопросы не получится.", nil)
		return handlers.EndConversation()
	}

	// Format and display event list for selection
	formattedEvents := formatters.FormatEventListForTopicsView(
		events,
		"📋 Ближайшие мероприятия",
		"к которому ты хочешь добавить темы и вопросы",
	)

	sentMsg, _ := h.messageSenderService.ReplyHtmlWithReturnMessage(
		msg,
		formattedEvents,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.CancelButton(topicAddCallbackConfirmCancel),
		},
	)

	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
	return handlers.NextConversationState(topicAddStateSelectEvent)
}

// 2. handleEventSelection processes the user's event selection
func (h *topicAddHandler) handleEventSelection(b *gotgbot.Bot, ctx *ext.Context) error {
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

	// Topics can be added only to events that have not taken place yet
	if !event.IsUpcoming(time.Now()) {
		h.messageSenderService.Reply(msg, fmt.Sprintf(
			"Мероприятие с ID %d уже прошло, к нему нельзя добавить темы и вопросы. Пожалуйста, нажми на ID мероприятия из списка выше %s.",
			eventID, formatters.CancelHint,
		), nil)
		return nil // Stay in the same state
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	// Store the selected event ID for later use when creating a new topic
	h.userStore.Set(ctx.EffectiveUser.Id, topicAddCtxDataKeySelectedEventID, eventID)
	h.userStore.Set(ctx.EffectiveUser.Id, topicAddCtxDataKeySelectedEventName, event.Name)

	// Prompt user to enter a topic
	sentMsg, _ := h.messageSenderService.ReplyHtmlWithReturnMessage(
		msg,
		fmt.Sprintf("Отправь мне темы и вопросы к мероприятию %s:", formatters.FormatEventName(event.Name)),
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.CancelButton(topicAddCallbackConfirmCancel),
		},
	)

	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
	return handlers.NextConversationState(topicAddStateEnterTopic)
}

// 3. handleTopicEntry handles the user's topic input
func (h *topicAddHandler) handleTopicEntry(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	topicText := strings.TrimSpace(msg.Text)

	if topicText == "" {
		h.messageSenderService.Reply(msg, fmt.Sprintf(
			"Тема не может быть пустой. Пожалуйста, отправь текст темы или вопроса %s.", formatters.CancelHint,
		), nil)
		return nil // Stay in the same state
	}

	// Get the selected event ID from user store
	eventIDVal, _ := h.userStore.Get(ctx.EffectiveUser.Id, topicAddCtxDataKeySelectedEventID)
	eventID, ok := eventIDVal.(int)
	if !ok {
		log.Printf("%s: Selected event ID is missing or invalid: %v", utils.GetCurrentTypeName(), eventIDVal)
		h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)
		h.messageSenderService.Reply(msg, formatters.LostEventText(constants.TopicAddCommand), nil)
		h.userStore.Clear(ctx.EffectiveUser.Id)
		return handlers.EndConversation()
	}

	userNickname := "не указано"
	author := userNickname
	if ctx.EffectiveUser.Username != "" {
		userNickname = ctx.EffectiveUser.Username
		author = "@" + userNickname
	}

	// Create the new topic
	_, err := h.topicRepository.CreateTopic(topicText, userNickname, eventID)
	if err != nil {
		h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)
		h.messageSenderService.Reply(msg, "Произошла ошибка при сохранении темы. Пожалуйста, попробуй ещё раз позже.", nil)
		log.Printf("%s: Error during topic creation in database: %v", utils.GetCurrentTypeName(), err)
		h.userStore.Clear(ctx.EffectiveUser.Id)
		return handlers.EndConversation()
	}

	// Send notification to admin about new topic
	eventNameVal, _ := h.userStore.Get(ctx.EffectiveUser.Id, topicAddCtxDataKeySelectedEventName)
	eventName, _ := eventNameVal.(string)

	adminMsg := fmt.Sprintf(
		"🔔 <b>Новая тема добавлена</b>\n\n"+
			"<i>Мероприятие:</i> %s\n"+
			"<i>Автор:</i> %s\n"+
			"<i>Тема:</i> %s",
		html.EscapeString(eventName),
		html.EscapeString(author),
		html.EscapeString(topicText),
	)

	h.messageSenderService.SendHtml(h.config.AdminUserID, adminMsg, nil)

	h.messageSenderService.Reply(
		msg,
		fmt.Sprintf(
			"✅ Добавлено!\n\nПосмотреть все темы и вопросы — /%s, добавить ещё — /%s.",
			constants.TopicsCommand,
			constants.TopicAddCommand,
		),
		nil,
	)

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)
	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// handleCallbackCancel processes the cancel button click
func (h *topicAddHandler) handleCallbackCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	// Answer the callback query to remove the loading state on the button
	cb := ctx.Update.CallbackQuery
	_, _ = cb.Answer(b, nil)

	return h.handleCancel(b, ctx)
}

// 4. handleCancel handles the /cancel command
func (h *topicAddHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	h.messageSenderService.Reply(msg, "Операция добавления темы отменена.", nil)

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

func (h *topicAddHandler) MessageRemoveInlineKeyboard(b *gotgbot.Bot, userID *int64) {
	var chatID, messageID int64

	// If userID provided, get stored message info using the utility method
	if userID != nil {
		messageID, chatID = h.userStore.GetPreviousMessageInfo(
			*userID,
			topicAddCtxDataKeyPreviousMessageID,
			topicAddCtxDataKeyPreviousChatID,
		)
	}

	// Skip if we don't have valid chat and message IDs
	if chatID == 0 || messageID == 0 {
		return
	}

	// Use message sender service to remove the inline keyboard
	_ = h.messageSenderService.RemoveInlineKeyboard(chatID, messageID)
}

func (h *topicAddHandler) SavePreviousMessageInfo(userID int64, sentMsg *gotgbot.Message) {
	if sentMsg == nil {
		return
	}
	h.userStore.SetPreviousMessageInfo(userID, sentMsg.MessageId, sentMsg.Chat.Id,
		topicAddCtxDataKeyPreviousMessageID, topicAddCtxDataKeyPreviousChatID)
}
