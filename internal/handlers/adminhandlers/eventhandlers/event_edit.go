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
	eventRepository *repositories.EventRepository
	config          *config.Config
	userStore       *utils.UserDataStore
}

func NewEventEditHandler(
	eventRepository *repositories.EventRepository,
	config *config.Config,
) ext.Handler {
	h := &eventEditHandler{
		eventRepository: eventRepository,
		config:          config,
		userStore:       utils.NewUserDataStore(),
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

	// Check admin permissions and private chat
	if !utils.CheckAdminAndPrivateChat(b, ctx, h.config.SuperGroupChatID, constants.EventEditCommand) {
		return handlers.EndConversation()
	}

	// Get a list of the last N events
	events, err := h.eventRepository.GetLastEvents(constants.EventEditGetLastLimit)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Произошла ошибка при получении списка событий.", err)
		return handlers.EndConversation()
	}

	if len(events) == 0 {
		utils.SendLoggedReply(b, msg, "Нет созданных событий для редактирования.", nil)
		return handlers.EndConversation()
	}

	// Create a list of events to display
	title := fmt.Sprintf("Последние %d события:", len(events))
	actionDescription := "которое ты хочешь отредактировать"
	formattedResponse := utils.FormatEventListForAdmin(events, title, constants.CancelCommand, actionDescription)

	utils.SendLoggedMarkdownReply(b, msg, formattedResponse, nil)

	return handlers.NextConversationState(eventEditStateSelectEvent)
}

// 2. handleSelectEvent processes the user's selection of an event to edit
func (h *eventEditHandler) handleSelectEvent(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	eventIDStr := strings.TrimSpace(msg.Text)

	eventID, err := strconv.Atoi(eventIDStr)
	if err != nil {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Неверный ID. Пожалуйста, введи числовой ID или /%s для отмены.", constants.CancelCommand), nil)
		return nil // Stay in the same state
	}

	// Check if content with this ID exists
	_, err = h.eventRepository.GetEventByID(eventID)
	if err != nil {
		log.Printf("Error checking content with ID %d: %v", eventID, err)
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Событие с ID %d не найдено. Пожалуйста, введи существующий ID или /%s для отмены.", eventID, constants.CancelCommand), nil)
		return nil // Stay in the same state
	}

	// Store the selected event ID
	h.userStore.Set(ctx.EffectiveUser.Id, eventEditCtxDataKeySelectedEventID, eventID)

	// Ask what the user wants to edit
	utils.SendLoggedReply(b, msg, fmt.Sprintf(
		"Что ты хочешь отредактировать?\n1. Название\n2. Дату начала\n\nВведи номер или используй /%s для отмены:", constants.CancelCommand,
	), nil)

	return handlers.NextConversationState(eventEditStateAskEditType)
}

// 3. handleSelectEditType processes the user's selection of what to edit
func (h *eventEditHandler) handleSelectEditType(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	selectionText := strings.TrimSpace(msg.Text)

	// Get the selected event ID
	eventIDVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, eventEditCtxDataKeySelectedEventID)
	if !ok {
		utils.SendLoggedReply(b, msg, fmt.Sprintf(
			"Произошла ошибка при получении выбранного события. Пожалуйста, начни заново с /%s",
			constants.EventEditCommand,
		), nil)
		return handlers.EndConversation()
	}

	eventID, ok := eventIDVal.(int)
	if !ok {
		utils.SendLoggedReply(b, msg, fmt.Sprintf(
			"Произошла внутренняя ошибка (неверный тип ID). Пожалуйста, начни заново с /%s",
			constants.EventEditCommand,
		), nil)
		return handlers.EndConversation()
	}

	// Get the event details
	event, err := h.eventRepository.GetEventByID(eventID)
	if err != nil {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Ошибка при получении события с ID %d", eventID), err)
		return handlers.EndConversation()
	}

	// Parse the selection
	selection, err := strconv.Atoi(selectionText)
	if err != nil || selection < 1 || selection > 2 {
		utils.SendLoggedReply(b, msg, fmt.Sprintf(
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
	utils.SendLoggedReply(b, msg, promptMessage, nil)

	return handlers.NextConversationState(nextState)
}

// 4.1. handleEditName processes the new name input and updates the event
func (h *eventEditHandler) handleEditName(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	newName := strings.TrimSpace(msg.Text)

	if newName == "" {
		utils.SendLoggedReply(b, msg, fmt.Sprintf(
			"Название не может быть пустым. Пожалуйста, введи новое название или используй /%s для отмены:",
			constants.CancelCommand,
		), nil)
		return nil // Stay in the same state
	}

	// Get the selected event ID
	eventIDVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, eventEditCtxDataKeySelectedEventID)
	if !ok {
		utils.SendLoggedReply(b, msg, fmt.Sprintf(
			"Произошла ошибка при получении выбранного события. Пожалуйста, начни заново с /%s",
			constants.EventEditCommand,
		), nil)
		return handlers.EndConversation()
	}

	eventID, ok := eventIDVal.(int)
	if !ok {
		log.Println("Invalid event ID type:", eventIDVal)
		utils.SendLoggedReply(b, msg, fmt.Sprintf(
			"Произошла внутренняя ошибка (неверный тип ID). Пожалуйста, начни заново с /%s",
			constants.EventEditCommand,
		), nil)
		return handlers.EndConversation()
	}

	// Update the event name
	err := h.eventRepository.UpdateEventName(eventID, newName)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Произошла ошибка при обновлении названия события.", err)
		return handlers.EndConversation()
	}

	// Confirmation message
	utils.SendLoggedReply(b, msg, fmt.Sprintf("Название события с ID %d успешно обновлено на '%s'", eventID, newName), nil)

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
		utils.SendLoggedReply(b, msg, fmt.Sprintf(
			"Неверный формат даты. Пожалуйста, введи дату и время в формате DD.MM.YYYY HH:MM или используй /%s для отмены.",
			constants.CancelCommand,
		), nil)
		return nil // Stay in the same state
	}

	// Get the selected event ID
	eventIDVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, eventEditCtxDataKeySelectedEventID)
	if !ok {
		utils.SendLoggedReply(b, msg, fmt.Sprintf(
			"Произошла ошибка при получении выбранного события. Пожалуйста, начни заново с /%s",
			constants.EventEditCommand,
		), nil)
		return handlers.EndConversation()
	}

	eventID, ok := eventIDVal.(int)
	if !ok {
		log.Println("Invalid event ID type:", eventIDVal)
		utils.SendLoggedReply(b, msg, fmt.Sprintf(
			"Произошла внутренняя ошибка (неверный тип ID). Пожалуйста, начни заново с /%s",
			constants.EventEditCommand,
		), nil)
		return handlers.EndConversation()
	}

	// Update the event start date
	err = h.eventRepository.UpdateEventStartedAt(eventID, startedAt)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Произошла ошибка при обновлении даты начала события.", err)
		return handlers.EndConversation()
	}

	// Confirmation message
	utils.SendLoggedReply(b, msg, fmt.Sprintf(
		"Дата начала события с ID %d успешно обновлена на %s",
		eventID, startedAt.Format("02.01.2006 15:04"),
	), nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// 5. handleCancel handles the /cancel command
func (h *eventEditHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	utils.SendLoggedReply(b, msg, "Операция редактирования события отменена.", nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}
