package eventhandlers

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/formatters"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
)

const (
	// Conversation states
	eventEditStateSelectEvent   = "event_edit_select_event"
	eventEditStateAskEditType   = "event_edit_ask_edit_type"
	eventEditStateEditName      = "event_edit_edit_name"
	eventEditStateEditStartedAt = "event_edit_edit_started_at"

	// Context data keys
	eventEditCtxDataKeySelectedEventID = "event_edit_selected_event_id"
	eventEditCtxDataKeyEditType        = "event_edit_edit_type"
)

// Edit types
const (
	eventEditTypeName      = "name"
	eventEditTypeStartDate = "startDate"
)

type eventEditHandler struct {
	config               *config.Config
	eventRepository      *repositories.EventRepository
	messageSenderService services.MessageSenderService
	userStore            *utils.UserDataStore
	permissionsService   *services.PermissionsService
}

func NewEventEditHandler(
	config *config.Config,
	eventRepository *repositories.EventRepository,
	messageSenderService services.MessageSenderService,
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
			},
			eventEditStateAskEditType: {
				handlers.NewMessage(message.Text, h.handleSelectEditType),
			},
			eventEditStateEditName: {
				handlers.NewMessage(message.Text, h.handleEditName),
			},
			eventEditStateEditStartedAt: {
				handlers.NewMessage(message.Text, h.handleEditStartedAt),
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
	if !h.permissionsService.CheckAdminAndPrivateChat(b, ctx, constants.ShowTopicsCommand) {
		return handlers.EndConversation()
	}

	// Get a list of the last N events
	events, err := h.eventRepository.GetLastEvents(constants.EventEditGetLastLimit)
	if err != nil {
		h.messageSenderService.Reply(b, msg, "Произошла ошибка при получении списка мероприятий.", nil)
		log.Printf("EventEditHandler: Error during event retrieval: %v", err)
		return handlers.EndConversation()
	}

	if len(events) == 0 {
		h.messageSenderService.Reply(b, msg, "Нет созданных мероприятий для редактирования.", nil)
		return handlers.EndConversation()
	}

	// Create a list of events to display
	title := fmt.Sprintf("Последние %d мероприятия:", len(events))
	actionDescription := "которое ты хочешь отредактировать"
	formattedResponse := formatters.FormatEventListForAdmin(events, title, constants.CancelCommand, actionDescription)

	h.messageSenderService.ReplyMarkdown(b, msg, formattedResponse, nil)

	return handlers.NextConversationState(eventEditStateSelectEvent)
}

// 2. handleSelectEvent processes the user's selection of an event to edit
func (h *eventEditHandler) handleSelectEvent(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	eventIDStr := strings.TrimSpace(msg.Text)

	eventID, err := strconv.Atoi(eventIDStr)
	if err != nil {
		h.messageSenderService.Reply(b, msg, fmt.Sprintf("Неверный ID. Пожалуйста, введи числовой ID или /%s для отмены.", constants.CancelCommand), nil)
		return nil // Stay in the same state
	}

	// Check if content with this ID exists
	_, err = h.eventRepository.GetEventByID(eventID)
	if err != nil {
		log.Printf("Error checking content with ID %d: %v", eventID, err)
		h.messageSenderService.Reply(
			b,
			msg,
			fmt.Sprintf("Мероприятие с ID %d не найдено. Пожалуйста, введи существующий ID или /%s для отмены.", eventID, constants.CancelCommand),
			nil,
		)
		return nil // Stay in the same state
	}

	// Store the selected event ID
	h.userStore.Set(ctx.EffectiveUser.Id, eventEditCtxDataKeySelectedEventID, eventID)

	// Ask what the user wants to edit
	h.messageSenderService.Reply(
		b,
		msg,
		fmt.Sprintf(
			"Что ты хочешь отредактировать?\n1. Название\n2. Дату начала\n\nВведи номер или используй /%s для отмены:",
			constants.CancelCommand,
		),
		nil,
	)

	return handlers.NextConversationState(eventEditStateAskEditType)
}

// 3. handleSelectEditType processes the user's selection of what to edit
func (h *eventEditHandler) handleSelectEditType(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	selectionText := strings.TrimSpace(msg.Text)

	// Get the selected event ID
	eventIDVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, eventEditCtxDataKeySelectedEventID)
	if !ok {
		h.messageSenderService.Reply(b, msg, fmt.Sprintf(
			"Произошла ошибка при получении выбранного мероприятия. Пожалуйста, начни заново с /%s",
			constants.EventEditCommand,
		), nil)
		return handlers.EndConversation()
	}

	eventID, ok := eventIDVal.(int)
	if !ok {
		h.messageSenderService.Reply(b, msg, fmt.Sprintf(
			"Произошла внутренняя ошибка (неверный тип ID). Пожалуйста, начни заново с /%s",
			constants.EventEditCommand,
		), nil)
		return handlers.EndConversation()
	}

	// Get the event details
	event, err := h.eventRepository.GetEventByID(eventID)
	if err != nil {
		h.messageSenderService.Reply(b, msg, fmt.Sprintf("Ошибка при получении мероприятия с ID %d", eventID), nil)
		log.Printf("EventEditHandler: Error during event retrieval: %v", err)
		return handlers.EndConversation()
	}

	// Parse the selection
	selection, err := strconv.Atoi(selectionText)
	if err != nil || selection < 1 || selection > 2 {
		h.messageSenderService.Reply(b, msg, fmt.Sprintf(
			"Неверный выбор. Пожалуйста, введи число от 1 до 2, или используй /%s для отмены",
			constants.CancelCommand,
		), nil)
		return nil // Stay in the same state
	}

	var editType string
	var nextState string
	var promptMessage string

	switch selection {
	case 1:
		editType = eventEditTypeName
		nextState = eventEditStateEditName
		promptMessage = fmt.Sprintf("Текущее название: %s\nВведи новое название или используй /%s для отмены:", event.Name, constants.CancelCommand)
	case 2:
		editType = eventEditTypeStartDate
		nextState = eventEditStateEditStartedAt
		var currentStartedAt string
		if event.StartedAt != nil {
			currentStartedAt = event.StartedAt.Format("02.01.2006 15:04")
		} else {
			currentStartedAt = "не задана"
		}
		promptMessage = fmt.Sprintf(
			"Текущая дата старта: %s\nВведи новую дату и время в формате DD.MM.YYYY HH:MM или используй /%s для отмены:",
			currentStartedAt, constants.CancelCommand,
		)
	}

	// Store the edit type
	h.userStore.Set(ctx.EffectiveUser.Id, eventEditCtxDataKeyEditType, editType)

	// Prompt for the new value
	h.messageSenderService.Reply(b, msg, promptMessage, nil)

	return handlers.NextConversationState(nextState)
}

// 4.1. handleEditName processes the new name input and updates the event
func (h *eventEditHandler) handleEditName(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	newName := strings.TrimSpace(msg.Text)

	if newName == "" {
		h.messageSenderService.Reply(b, msg, fmt.Sprintf(
			"Название не может быть пустым. Пожалуйста, введи новое название или используй /%s для отмены:",
			constants.CancelCommand,
		), nil)
		return nil // Stay in the same state
	}

	// Get the selected event ID
	eventIDVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, eventEditCtxDataKeySelectedEventID)
	if !ok {
		h.messageSenderService.Reply(b, msg, fmt.Sprintf(
			"Произошла ошибка при получении выбранного мероприятия. Пожалуйста, начни заново с /%s",
			constants.EventEditCommand,
		), nil)
		return handlers.EndConversation()
	}

	eventID, ok := eventIDVal.(int)
	if !ok {
		log.Println("Invalid event ID type:", eventIDVal)
		h.messageSenderService.Reply(b, msg, fmt.Sprintf(
			"Произошла внутренняя ошибка (неверный тип ID). Пожалуйста, начни заново с /%s",
			constants.EventEditCommand,
		), nil)
		return handlers.EndConversation()
	}

	// Update the event name
	err := h.eventRepository.UpdateEventName(eventID, newName)
	if err != nil {
		h.messageSenderService.Reply(b, msg, "Произошла ошибка при обновлении названия мероприятия.", nil)
		log.Printf("EventEditHandler: Error during event update: %v", err)
		return handlers.EndConversation()
	}

	// Confirmation message
	h.messageSenderService.Reply(b, msg, fmt.Sprintf("Название мероприятия с ID %d успешно обновлено на '%s'", eventID, newName), nil)

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
		h.messageSenderService.Reply(b, msg, fmt.Sprintf(
			"Неверный формат даты. Пожалуйста, введи дату и время в формате DD.MM.YYYY HH:MM или используй /%s для отмены.",
			constants.CancelCommand,
		), nil)
		return nil // Stay in the same state
	}

	// Get the selected event ID
	eventIDVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, eventEditCtxDataKeySelectedEventID)
	if !ok {
		h.messageSenderService.Reply(b, msg, fmt.Sprintf(
			"Произошла ошибка при получении выбранного мероприятия. Пожалуйста, начни заново с /%s",
			constants.EventEditCommand,
		), nil)
		return handlers.EndConversation()
	}

	eventID, ok := eventIDVal.(int)
	if !ok {
		log.Println("Invalid event ID type:", eventIDVal)
		h.messageSenderService.Reply(b, msg, fmt.Sprintf(
			"Произошла внутренняя ошибка (неверный тип ID). Пожалуйста, начни заново с /%s",
			constants.EventEditCommand,
		), nil)
		return handlers.EndConversation()
	}

	// Update the event start date
	err = h.eventRepository.UpdateEventStartedAt(eventID, startedAt)
	if err != nil {
		h.messageSenderService.Reply(b, msg, "Произошла ошибка при обновлении даты начала мероприятия.", nil)
		log.Printf("EventEditHandler: Error during event update: %v", err)
		return handlers.EndConversation()
	}

	// Confirmation message
	h.messageSenderService.Reply(b, msg, fmt.Sprintf(
		"Дата начала мероприятия с ID %d успешно обновлена на %s",
		eventID, startedAt.Format("02.01.2006 15:04"),
	), nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// 5. handleCancel handles the /cancel command
func (h *eventEditHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	h.messageSenderService.Reply(b, msg, "Операция редактирования мероприятия отменена.", nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}
