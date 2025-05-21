package eventhandlers

import (
	"fmt"
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
	eventEditStateSelectEvent   = "event_edit_state_select_event"
	eventEditStateAskEditType   = "event_edit_state_ask_edit_type"
	eventEditStateEditName      = "event_edit_state_edit_name"
	eventEditStateEditStartedAt = "event_edit_state_edit_started_at"
	eventEditStateEditType      = "event_edit_state_edit_type"

	// Context data keys
	eventEditCtxDataKeySelectedEventID   = "event_edit_ctx_data_selected_event_id"
	eventEditCtxDataKeyEditType          = "event_edit_ctx_data_edit_type"
	eventEditCtxDataKeyPreviousMessageID = "event_edit_ctx_data_previous_message_id"
	eventEditCtxDataKeyPreviousChatID    = "event_edit_ctx_data_previous_chat_id"

	// Callback data
	eventEditCallbackConfirmCancel = "event_edit_callback_confirm_cancel"

	// Edit types
	eventEditTypeName      = "name"
	eventEditTypeStartDate = "startDate"
	eventEditTypeType      = "type"
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
	if !h.permissionsService.CheckAdminAndPrivateChat(msg, constants.ShowTopicsCommand) {
		return handlers.EndConversation()
	}

	// Get a list of the last N events
	events, err := h.eventRepository.GetLastEvents(constants.EventEditGetLastLimit)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при получении списка мероприятий.", nil)
		log.Printf("EventEditHandler: Error during event retrieval: %v", err)
		return handlers.EndConversation()
	}

	if len(events) == 0 {
		h.messageSenderService.Reply(msg, "Нет созданных мероприятий для редактирования.", nil)
		return handlers.EndConversation()
	}

	// Create a list of events to display
	title := fmt.Sprintf("Последние %d мероприятия:", len(events))
	actionDescription := "которое ты хочешь отредактировать"
	formattedResponse := formatters.FormatEventListForAdmin(events, title, constants.CancelCommand, actionDescription)

	sentMsg, _ := h.messageSenderService.ReplyMarkdownWithReturnMessage(
		msg,
		formattedResponse,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.CancelButton(eventEditCallbackConfirmCancel),
		},
	)

	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
	return handlers.NextConversationState(eventEditStateSelectEvent)
}

// 2. handleSelectEvent processes the user's selection of an event to edit
func (h *eventEditHandler) handleSelectEvent(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	eventIDStr := strings.TrimSpace(strings.Replace(msg.Text, "/", "", 1))

	eventID, err := strconv.Atoi(eventIDStr)
	if err != nil {
		h.messageSenderService.Reply(msg, fmt.Sprintf("Неверный ID. Пожалуйста, введи числовой ID или используй кнопку для отмены."), nil)
		return nil // Stay in the same state
	}

	// Check if content with this ID exists
	_, err = h.eventRepository.GetEventByID(eventID)
	if err != nil {
		log.Printf("Error checking content with ID %d: %v", eventID, err)
		h.messageSenderService.Reply(
			msg,
			fmt.Sprintf("Мероприятие с ID %d не найдено. Пожалуйста, введи существующий ID или используй кнопку для отмены.", eventID),
			nil,
		)
		return nil // Stay in the same state
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	// Store the selected event ID
	h.userStore.Set(ctx.EffectiveUser.Id, eventEditCtxDataKeySelectedEventID, eventID)

	// Ask what the user wants to edit
	sentMsg, _ := h.messageSenderService.ReplyWithReturnMessage(
		msg,
		fmt.Sprintf("Что ты хочешь отредактировать?\n/1. Название\n/2. Дату начала\n/3. Тип\n\nВведи номер:"),
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

	// Get the selected event ID
	eventIDVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, eventEditCtxDataKeySelectedEventID)
	if !ok {
		h.messageSenderService.Reply(msg, fmt.Sprintf(
			"Произошла ошибка при получении выбранного мероприятия. Пожалуйста, начни заново с /%s",
			constants.EventEditCommand,
		), nil)
		return handlers.EndConversation()
	}

	eventID, ok := eventIDVal.(int)
	if !ok {
		h.messageSenderService.Reply(msg, fmt.Sprintf(
			"Произошла внутренняя ошибка (неверный тип ID). Пожалуйста, начни заново с /%s",
			constants.EventEditCommand,
		), nil)
		return handlers.EndConversation()
	}

	// Get the event details
	event, err := h.eventRepository.GetEventByID(eventID)
	if err != nil {
		h.messageSenderService.Reply(msg, fmt.Sprintf("Ошибка при получении мероприятия с ID %d", eventID), nil)
		log.Printf("EventEditHandler: Error during event retrieval: %v", err)
		return handlers.EndConversation()
	}

	// Parse the selection
	selection, err := strconv.Atoi(selectionText)
	if err != nil || selection < 1 || selection > 3 {
		h.messageSenderService.Reply(msg, fmt.Sprintf(
			"Неверный выбор. Пожалуйста, введи число от 1 до 3, или используй кнопку для отмены",
		), nil)
		return nil // Stay in the same state
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	var editType string
	var nextState string
	var message string

	switch selection {
	case 1:
		editType = eventEditTypeName
		nextState = eventEditStateEditName
		message = fmt.Sprintf("Текущее название: *%s*\n\nВведи новое название:", event.Name)
	case 2:
		editType = eventEditTypeStartDate
		nextState = eventEditStateEditStartedAt
		var currentStartedAt string
		if event.StartedAt != nil {
			currentStartedAt = event.StartedAt.Format("02.01.2006 15:04")
		} else {
			currentStartedAt = "не задана"
		}
		message = fmt.Sprintf(
			"Текущая дата старта: `%s` (UTC)\nВведи новую дату и время в формате DD.MM.YYYY HH:MM (UTC):",
			currentStartedAt,
		)
	case 3:
		editType = eventEditTypeType
		nextState = eventEditStateEditType

		// Prepare available event types for display
		var availableTypes string
		for i, t := range constants.AllEventTypes {
			availableTypes += fmt.Sprintf("/%d. %s %s\n", i+1, formatters.GetTypeEmoji(t), t)
		}

		message = fmt.Sprintf(
			"Текущий тип: *%s*\n\nДоступные типы:\n%s\nВведи новый тип или его номер:",
			event.Type, availableTypes,
		)
	}

	// Store the edit type
	h.userStore.Set(ctx.EffectiveUser.Id, eventEditCtxDataKeyEditType, editType)

	// Prompt for the new value
	sentMsg, _ := h.messageSenderService.ReplyMarkdownWithReturnMessage(
		msg,
		message,
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
			"Название не может быть пустым. Пожалуйста, введи новое название или используй кнопку для отмены:",
		), nil)
		return nil // Stay in the same state
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	// Get the selected event ID
	eventIDVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, eventEditCtxDataKeySelectedEventID)
	if !ok {
		h.messageSenderService.Reply(msg, fmt.Sprintf(
			"Произошла ошибка при получении выбранного мероприятия. Пожалуйста, начни заново с /%s",
			constants.EventEditCommand,
		), nil)
		return handlers.EndConversation()
	}

	eventID, ok := eventIDVal.(int)
	if !ok {
		log.Println("Invalid event ID type:", eventIDVal)
		h.messageSenderService.Reply(msg, fmt.Sprintf(
			"Произошла внутренняя ошибка (неверный тип ID). Пожалуйста, начни заново с /%s",
			constants.EventEditCommand,
		), nil)
		return handlers.EndConversation()
	}

	// Update the event name
	err := h.eventRepository.UpdateEventName(eventID, newName)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при обновлении названия мероприятия.", nil)
		log.Printf("EventEditHandler: Error during event update: %v", err)
		return handlers.EndConversation()
	}

	// Confirmation message
	h.messageSenderService.ReplyMarkdown(
		msg,
		fmt.Sprintf(
			"Название мероприятия с ID `%d` успешно обновлено на *\"%s\"* \n\nДля продолжения редактирования мероприятия используй команду /%s.\nДля просмотра всех команд используй команду /%s",
			eventID, newName, constants.EventEditCommand, constants.HelpCommand,
		),
		nil,
	)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// 4.2. handleEditStartedAt processes the new start date input and updates the event
func (h *eventEditHandler) handleEditStartedAt(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	dateTimeStr := strings.TrimSpace(msg.Text)

	// Parse the start date
	startedAt, err := time.Parse("02.01.2006 15:04", dateTimeStr)
	if err != nil {
		h.messageSenderService.ReplyMarkdown(msg, fmt.Sprintf(
			"Неверный формат даты. Пожалуйста, введи дату и время в формате *DD.MM.YYYY HH:MM* (UTC) или используй кнопку для отмены.",
		), nil)
		return nil // Stay in the same state
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	// Get the selected event ID
	eventIDVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, eventEditCtxDataKeySelectedEventID)
	if !ok {
		h.messageSenderService.Reply(msg, fmt.Sprintf(
			"Произошла ошибка при получении выбранного мероприятия. Пожалуйста, начни заново с /%s",
			constants.EventEditCommand,
		), nil)
		return handlers.EndConversation()
	}

	eventID, ok := eventIDVal.(int)
	if !ok {
		log.Println("Invalid event ID type:", eventIDVal)
		h.messageSenderService.Reply(msg, fmt.Sprintf(
			"Произошла внутренняя ошибка (неверный тип ID). Пожалуйста, начни заново с /%s",
			constants.EventEditCommand,
		), nil)
		return handlers.EndConversation()
	}

	// Update the event start date
	err = h.eventRepository.UpdateEventStartedAt(eventID, startedAt)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при обновлении даты начала мероприятия.", nil)
		log.Printf("EventEditHandler: Error during event update: %v", err)
		return handlers.EndConversation()
	}

	// Confirmation message
	h.messageSenderService.ReplyMarkdown(msg, fmt.Sprintf(
		"Дата начала мероприятия с ID %d успешно обновлена на *%s* \n\nДля продолжения редактирования мероприятия используй команду /%s.\nДля просмотра всех команд используй команду /%s",
		eventID, startedAt.Format("02.01.2006 15:04 UTC"), constants.EventEditCommand, constants.HelpCommand,
	), nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// 4.3. handleEditType processes the new type input and updates the event
func (h *eventEditHandler) handleEditType(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	input := strings.TrimSpace(strings.Replace(msg.Text, "/", "", 1))

	if input == "" {
		h.messageSenderService.Reply(msg, fmt.Sprintf(
			"Тип не может быть пустым. Пожалуйста, введи новый тип или его номер, или используй кнопку для отмены:",
		), nil)
		return nil // Stay in the same state
	}

	// Check if input is a number (index selection)
	index, err := strconv.Atoi(input)
	var validEventType constants.EventType

	if err == nil && index > 0 && index <= len(constants.AllEventTypes) {
		// User selected by index
		validEventType = constants.AllEventTypes[index-1]
	} else {
		// User entered the type directly, validate it
		validEventType = constants.EventType(input)
		isValid := false
		for _, eventType := range constants.AllEventTypes {
			if validEventType == eventType {
				isValid = true
				break
			}
		}

		if !isValid {
			eventTypesStr := []string{}
			for _, t := range constants.AllEventTypes {
				eventTypesStr = append(eventTypesStr, string(t))
			}
			h.messageSenderService.Reply(msg, fmt.Sprintf(
				"Неверный тип мероприятия. Допустимые типы: %s",
				strings.Join(eventTypesStr, ", "),
			), nil)
			return nil // Stay in the same state
		}
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	// Get the selected event ID
	eventIDVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, eventEditCtxDataKeySelectedEventID)
	if !ok {
		h.messageSenderService.Reply(msg, fmt.Sprintf(
			"Произошла ошибка при получении выбранного мероприятия. Пожалуйста, начни заново с /%s",
			constants.EventEditCommand,
		), nil)
		return handlers.EndConversation()
	}

	eventID, ok := eventIDVal.(int)
	if !ok {
		log.Println("Invalid event ID type:", eventIDVal)
		h.messageSenderService.Reply(msg, fmt.Sprintf(
			"Произошла внутренняя ошибка (неверный тип ID). Пожалуйста, начни заново с /%s",
			constants.EventEditCommand,
		), nil)
		return handlers.EndConversation()
	}

	// Update the event type
	err = h.eventRepository.UpdateEventType(eventID, validEventType)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при обновлении типа мероприятия.", nil)
		log.Printf("EventEditHandler: Error during event update: %v", err)
		return handlers.EndConversation()
	}

	// Confirmation message
	h.messageSenderService.ReplyMarkdown(
		msg,
		fmt.Sprintf(
			"Тип мероприятия с ID %d успешно обновлен на %s *'%s'* \n\nДля продолжения редактирования мероприятия используй команду /%s.\nДля просмотра всех команд используй команду /%s",
			eventID,
			formatters.GetTypeEmoji(validEventType),
			validEventType,
			constants.EventEditCommand,
			constants.HelpCommand,
		),
		nil,
	)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
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
	h.userStore.SetPreviousMessageInfo(userID, sentMsg.MessageId, sentMsg.Chat.Id,
		eventEditCtxDataKeyPreviousMessageID, eventEditCtxDataKeyPreviousChatID)
}
