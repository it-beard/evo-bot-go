package eventhandlers

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
	eventEditStateSelectEvent   = "event_edit_state_select_event"
	eventEditStateAskEditType   = "event_edit_state_ask_edit_type"
	eventEditStateEditName      = "event_edit_state_edit_name"
	eventEditStateEditStartedAt = "event_edit_state_edit_started_at"
	eventEditStateEditType      = "event_edit_state_edit_type"

	// Context data keys
	eventEditCtxDataKeySelectedEventID   = "event_edit_ctx_data_selected_event_id"
	eventEditCtxDataKeyPreviousMessageID = "event_edit_ctx_data_previous_message_id"
	eventEditCtxDataKeyPreviousChatID    = "event_edit_ctx_data_previous_chat_id"

	// Callback data
	eventEditCallbackConfirmCancel = "event_edit_callback_confirm_cancel"

	// Edit menu options
	eventEditOptionName      = 1
	eventEditOptionStartDate = 2
	eventEditOptionType      = 3
	eventEditOptionStatus    = 4
)

type eventEditHandler struct {
	config               *config.Config
	eventRepository      *repositories.EventRepository
	messageSenderService *services.MessageSenderService
	userStore            *utils.UserDataStore
	permissionsService   *services.PermissionsService
}

func NewEventEditHandler(
	config *config.Config,
	eventRepository *repositories.EventRepository,
	messageSenderService *services.MessageSenderService,
	permissionsService *services.PermissionsService,
) ext.Handler {
	h := &eventEditHandler{
		config:               config,
		eventRepository:      eventRepository,
		messageSenderService: messageSenderService,
		userStore:            utils.NewUserDataStore(),
		permissionsService:   permissionsService,
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.EventEditCommand, h.startEdit),
		},
		map[string][]ext.Handler{
			eventEditStateSelectEvent: {
				handlers.NewMessage(message.Text, h.handleSelectEvent),
				handlers.NewCallback(callbackquery.Equal(eventEditCallbackConfirmCancel), h.handleCallbackCancel),
			},
			eventEditStateAskEditType: {
				handlers.NewMessage(message.Text, h.handleSelectEditType),
				handlers.NewCallback(callbackquery.Equal(eventEditCallbackConfirmCancel), h.handleCallbackCancel),
			},
			eventEditStateEditName: {
				handlers.NewMessage(message.Text, h.handleEditName),
				handlers.NewCallback(callbackquery.Equal(eventEditCallbackConfirmCancel), h.handleCallbackCancel),
			},
			eventEditStateEditStartedAt: {
				handlers.NewMessage(message.Text, h.handleEditStartedAt),
				handlers.NewCallback(callbackquery.Equal(eventEditCallbackConfirmCancel), h.handleCallbackCancel),
			},
			eventEditStateEditType: {
				handlers.NewMessage(message.Text, h.handleEditType),
				handlers.NewCallback(callbackquery.Equal(eventEditCallbackConfirmCancel), h.handleCallbackCancel),
			},
		},
		&handlers.ConversationOpts{
			Exits: []ext.Handler{handlers.NewCommand(constants.CancelCommand, h.handleCancel)},
		},
	)
}

// 1. startEdit is the entry point handler for the edit conversation
func (h *eventEditHandler) startEdit(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if user has admin permissions and is in a private chat
	if !h.permissionsService.CheckAdminAndPrivateChat(msg, constants.EventEditCommand) {
		return handlers.EndConversation()
	}

	sentMsg, ok := sendAdminEventList(h.eventRepository, h.messageSenderService, msg,
		"которое ты хочешь отредактировать", eventEditCallbackConfirmCancel)
	if !ok {
		return handlers.EndConversation()
	}

	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
	return handlers.NextConversationState(eventEditStateSelectEvent)
}

// 2. handleSelectEvent processes the user's selection of an event to edit
func (h *eventEditHandler) handleSelectEvent(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	event, ok := findSelectedEvent(h.eventRepository, h.messageSenderService, msg)
	if !ok {
		return nil // Stay in the same state
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	// Store the selected event ID
	h.userStore.Set(ctx.EffectiveUser.Id, eventEditCtxDataKeySelectedEventID, event.ID)

	// Ask what the user wants to edit
	sentMsg, _ := h.messageSenderService.ReplyHtmlWithReturnMessage(
		msg,
		fmt.Sprintf(
			"✏️ <b>Редактирование мероприятия</b>\n\n%s\n\nЧто ты хочешь изменить?\n/%d. Название\n/%d. Дату начала\n/%d. Тип\n/%d. %s\n\n👉 Нажми на номер пункта.",
			formatters.FormatEventSummary(*event),
			eventEditOptionName, eventEditOptionStartDate, eventEditOptionType, eventEditOptionStatus,
			statusToggleLabel(event),
		),
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.CancelButton(eventEditCallbackConfirmCancel),
		},
	)

	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
	return handlers.NextConversationState(eventEditStateAskEditType)
}

// 3. handleSelectEditType processes the user's selection of what to edit
func (h *eventEditHandler) handleSelectEditType(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	selectionText := strings.TrimSpace(strings.Replace(msg.Text, "/", "", 1))

	eventID, ok := h.selectedEventID(ctx)
	if !ok {
		return handlers.EndConversation()
	}

	// Get the event details
	event, err := h.eventRepository.GetEventByID(eventID)
	if err != nil {
		h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)
		h.messageSenderService.Reply(msg, fmt.Sprintf("Произошла ошибка при получении мероприятия с ID %d.", eventID), nil)
		log.Printf("%s: Error during event retrieval: %v", utils.GetCurrentTypeName(), err)
		h.userStore.Clear(ctx.EffectiveUser.Id)
		return handlers.EndConversation()
	}

	// Parse the selection
	selection, err := strconv.Atoi(selectionText)
	if err != nil || selection < eventEditOptionName || selection > eventEditOptionStatus {
		h.messageSenderService.Reply(msg, fmt.Sprintf(
			"Неверный выбор. Пожалуйста, нажми на номер пункта от %d до %d %s.",
			eventEditOptionName, eventEditOptionStatus, formatters.CancelHint,
		), nil)
		return nil // Stay in the same state
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	var nextState string
	var prompt string

	switch selection {
	case eventEditOptionName:
		nextState = eventEditStateEditName
		prompt = fmt.Sprintf("Текущее название: %s\n\nВведи новое название:", formatters.FormatEventName(event.Name))
	case eventEditOptionStartDate:
		nextState = eventEditStateEditStartedAt
		prompt = fmt.Sprintf(
			"Текущая дата начала: <b>%s</b>\n\nВведи новую дату и время в формате %s:",
			formatters.FormatEventDate(event.StartedAt), formatters.EventDateInputHint,
		)
	case eventEditOptionType:
		nextState = eventEditStateEditType
		prompt = fmt.Sprintf(
			"Текущий тип: %s\n\nВыбери новый тип:\n%s\n\n👉 Нажми на номер типа.",
			formatters.FormatEventType(constants.EventType(event.Type)), formatters.FormatEventTypeOptions(),
		)
	case eventEditOptionStatus:
		// Changing the status needs no extra input, so it is applied right away
		return h.toggleStatus(ctx, event)
	}

	// Prompt for the new value
	sentMsg, _ := h.messageSenderService.ReplyHtmlWithReturnMessage(
		msg,
		prompt,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.CancelButton(eventEditCallbackConfirmCancel),
		},
	)

	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
	return handlers.NextConversationState(nextState)
}

// 4.1. handleEditName processes the new name input and updates the event
func (h *eventEditHandler) handleEditName(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	newName := strings.TrimSpace(msg.Text)

	if newName == "" {
		h.messageSenderService.Reply(msg, fmt.Sprintf(
			"Название не может быть пустым. Пожалуйста, введи новое название %s.", formatters.CancelHint,
		), nil)
		return nil // Stay in the same state
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	eventID, ok := h.selectedEventID(ctx)
	if !ok {
		return handlers.EndConversation()
	}

	// Update the event name
	err := h.eventRepository.UpdateEventName(eventID, newName)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при обновлении названия мероприятия.", nil)
		log.Printf("%s: Error during event update: %v", utils.GetCurrentTypeName(), err)
		h.userStore.Clear(ctx.EffectiveUser.Id)
		return handlers.EndConversation()
	}

	h.replySuccess(msg, fmt.Sprintf(
		"✅ Название мероприятия (ID %d) изменено на %s.", eventID, formatters.FormatEventName(newName),
	))

	h.userStore.Clear(ctx.EffectiveUser.Id)
	return handlers.EndConversation()
}

// 4.2. handleEditStartedAt processes the new start date input and updates the event
func (h *eventEditHandler) handleEditStartedAt(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	startedAt, ok := parseEventDate(h.messageSenderService, msg)
	if !ok {
		return nil // Stay in the same state
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	eventID, ok := h.selectedEventID(ctx)
	if !ok {
		return handlers.EndConversation()
	}

	// Update the event start date
	err := h.eventRepository.UpdateEventStartedAt(eventID, startedAt)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при обновлении даты начала мероприятия.", nil)
		log.Printf("%s: Error during event update: %v", utils.GetCurrentTypeName(), err)
		h.userStore.Clear(ctx.EffectiveUser.Id)
		return handlers.EndConversation()
	}

	h.replySuccess(msg, fmt.Sprintf(
		"✅ Дата начала мероприятия (ID %d) изменена на <b>%s</b>.%s",
		eventID, formatters.FormatEventDate(&startedAt), pastStartDateWarningFor(startedAt),
	))

	h.userStore.Clear(ctx.EffectiveUser.Id)
	return handlers.EndConversation()
}

// 4.3. handleEditType processes the new type input and updates the event
func (h *eventEditHandler) handleEditType(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	newType, ok := parseEventType(h.messageSenderService, msg)
	if !ok {
		return nil // Stay in the same state
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	eventID, ok := h.selectedEventID(ctx)
	if !ok {
		return handlers.EndConversation()
	}

	// Update the event type
	err := h.eventRepository.UpdateEventType(eventID, newType)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при обновлении типа мероприятия.", nil)
		log.Printf("%s: Error during event update: %v", utils.GetCurrentTypeName(), err)
		h.userStore.Clear(ctx.EffectiveUser.Id)
		return handlers.EndConversation()
	}

	h.replySuccess(msg, fmt.Sprintf(
		"✅ Тип мероприятия (ID %d) изменён на %s.", eventID, formatters.FormatEventType(newType),
	))

	h.userStore.Clear(ctx.EffectiveUser.Id)
	return handlers.EndConversation()
}

// 4.4. toggleStatus finishes an actual event or makes a finished event actual again.
// Events that pass without the bot (no /eventStart) stay actual until an admin finishes them here.
func (h *eventEditHandler) toggleStatus(ctx *ext.Context, event *repositories.Event) error {
	msg := ctx.EffectiveMessage

	newStatus := constants.EventStatusFinished
	if event.Status == string(constants.EventStatusFinished) {
		newStatus = constants.EventStatusActual
	}

	if err := h.eventRepository.UpdateEventStatus(event.ID, newStatus); err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при обновлении статуса мероприятия.", nil)
		log.Printf("%s: Error during event update: %v", utils.GetCurrentTypeName(), err)
		h.userStore.Clear(ctx.EffectiveUser.Id)
		return handlers.EndConversation()
	}

	var result string
	if newStatus == constants.EventStatusFinished {
		result = fmt.Sprintf(
			"%s Мероприятие %s (ID %d) завершено.\nОно больше не показывается в /%s, /%s и /%s.",
			formatters.GetStatusEmoji(newStatus), formatters.FormatEventName(event.Name), event.ID,
			constants.EventsCommand, constants.TopicsCommand, constants.TopicAddCommand,
		)
	} else {
		result = fmt.Sprintf(
			"%s Мероприятие %s (ID %d) снова актуально.",
			formatters.GetStatusEmoji(newStatus), formatters.FormatEventName(event.Name), event.ID,
		)
		if event.StartedAt != nil {
			result += pastStartDateWarningFor(*event.StartedAt)
		}
	}

	h.replySuccess(msg, result)

	h.userStore.Clear(ctx.EffectiveUser.Id)
	return handlers.EndConversation()
}

// statusToggleLabel returns the edit menu entry that switches the event to the opposite status
func statusToggleLabel(event *repositories.Event) string {
	if event.Status == string(constants.EventStatusFinished) {
		return fmt.Sprintf("Вернуть в актуальные %s", formatters.GetStatusEmoji(constants.EventStatusActual))
	}
	return fmt.Sprintf("Завершить %s", formatters.GetStatusEmoji(constants.EventStatusFinished))
}

// replySuccess sends a success message followed by the hint on what to do next
func (h *eventEditHandler) replySuccess(msg *gotgbot.Message, text string) {
	h.messageSenderService.ReplyHtml(msg, text+fmt.Sprintf(
		"\n\nПродолжить редактирование — /%s, список всех команд — /%s.",
		constants.EventEditCommand, constants.HelpCommand,
	), nil)
}

// selectedEventID returns the event chosen earlier in the conversation, or reports the problem to the user
func (h *eventEditHandler) selectedEventID(ctx *ext.Context) (int, bool) {
	eventIDVal, _ := h.userStore.Get(ctx.EffectiveUser.Id, eventEditCtxDataKeySelectedEventID)
	eventID, ok := eventIDVal.(int)
	if !ok {
		log.Printf("%s: Selected event ID is missing or invalid: %v", utils.GetCurrentTypeName(), eventIDVal)
		h.messageSenderService.Reply(ctx.EffectiveMessage, formatters.LostEventText(constants.EventEditCommand), nil)
		h.userStore.Clear(ctx.EffectiveUser.Id)
		return 0, false
	}
	return eventID, true
}

// handleCallbackCancel processes the cancel button click
func (h *eventEditHandler) handleCallbackCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	// Answer the callback query to remove the loading state on the button
	cb := ctx.Update.CallbackQuery
	_, _ = cb.Answer(b, nil)

	return h.handleCancel(b, ctx)
}

// 5. handleCancel handles the /cancel command
func (h *eventEditHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)
	h.messageSenderService.Reply(msg, "Операция редактирования мероприятия отменена.", nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

func (h *eventEditHandler) MessageRemoveInlineKeyboard(b *gotgbot.Bot, userID *int64) {
	var chatID, messageID int64

	// If userID provided, get stored message info using the utility method
	if userID != nil {
		messageID, chatID = h.userStore.GetPreviousMessageInfo(
			*userID,
			eventEditCtxDataKeyPreviousMessageID,
			eventEditCtxDataKeyPreviousChatID,
		)
	}

	// Skip if we don't have valid chat and message IDs
	if chatID == 0 || messageID == 0 {
		return
	}

	// Use message sender service to remove the inline keyboard
	_ = h.messageSenderService.RemoveInlineKeyboard(chatID, messageID)
}

func (h *eventEditHandler) SavePreviousMessageInfo(userID int64, sentMsg *gotgbot.Message) {
	if sentMsg == nil {
		return
	}
	h.userStore.SetPreviousMessageInfo(userID, sentMsg.MessageId, sentMsg.Chat.Id,
		eventEditCtxDataKeyPreviousMessageID, eventEditCtxDataKeyPreviousChatID)
}
