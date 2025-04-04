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
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
)

const (
	// Conversation states
	eventSetupStateAskEventName      = "event_setup_ask_event_name"
	eventSetupStateAskEventType      = "event_setup_ask_event_type"
	eventSetupStateAskEventStartedAt = "event_setup_ask_event_started_at"

	// Context data keys
	eventSetupCtxDataKeyEventName = "event_setup_event_name"
	eventSetupCtxDataKeyEventID   = "event_setup_event_id"
)

type eventSetupHandler struct {
	config               *config.Config
	eventRepository      *repositories.EventRepository
	messageSenderService *services.MessageSenderService
	userStore            *utils.UserDataStore
	permissionsService   *services.PermissionsService
}

func NewEventSetupHandler(
	config *config.Config,
	eventRepository *repositories.EventRepository,
	messageSenderService *services.MessageSenderService,
	permissionsService *services.PermissionsService,
) ext.Handler {
	h := &eventSetupHandler{
		config:               config,
		eventRepository:      eventRepository,
		messageSenderService: messageSenderService,
		userStore:            utils.NewUserDataStore(),
		permissionsService:   permissionsService,
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.EventSetupCommand, h.startSetup),
		},
		map[string][]ext.Handler{
			eventSetupStateAskEventName: {
				handlers.NewMessage(message.Text, h.handleEventName),
			},
			eventSetupStateAskEventType: {
				handlers.NewMessage(message.Text, h.handleEventType),
			},
			eventSetupStateAskEventStartedAt: {
				handlers.NewMessage(message.Text, h.handleEventStartedAt),
			},
		},
		&handlers.ConversationOpts{
			Exits: []ext.Handler{handlers.NewCommand(constants.CancelCommand, h.handleCancel)},
		},
	)
}

// 1. startSetup is the entry point handler for the setup conversation
func (h *eventSetupHandler) startSetup(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if user has admin permissions and is in a private chat
	if !h.permissionsService.CheckAdminAndPrivateChat(msg, constants.ShowTopicsCommand) {
		return handlers.EndConversation()
	}

	h.messageSenderService.Reply(
		msg,
		fmt.Sprintf("Пожалуйста, введи название для нового мероприятия или /%s для отмены:", constants.CancelCommand),
		nil,
	)

	return handlers.NextConversationState(eventSetupStateAskEventName)
}

// 2. handleEventName processes the event name input
func (h *eventSetupHandler) handleEventName(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	eventName := strings.TrimSpace(msg.Text)

	if eventName == "" {
		h.messageSenderService.Reply(
			msg,
			fmt.Sprintf("Название не может быть пустым. Пожалуйста, введи название для мероприятия или /%s для отмены:", constants.CancelCommand),
			nil,
		)
		return nil // Stay in the same state
	}

	// Store the event name
	h.userStore.Set(ctx.EffectiveUser.Id, eventSetupCtxDataKeyEventName, eventName)

	// Ask for event type
	eventTypeOptions := []string{}
	for i, eventType := range constants.AllEventTypes {
		eventTypeOptions = append(eventTypeOptions, fmt.Sprintf("%d. %s", i+1, eventType))
	}
	typeOptions := fmt.Sprintf("Выбери тип мероприятия (введи число):\n%s\nИли /%s для отмены",
		strings.Join(eventTypeOptions, "\n"),
		constants.CancelCommand,
	)

	h.messageSenderService.Reply(msg, typeOptions, nil)

	return handlers.NextConversationState(eventSetupStateAskEventType)
}

// 3. handleEventType processes the event type selection and creates the event
func (h *eventSetupHandler) handleEventType(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	typeSelection := strings.TrimSpace(msg.Text)

	var eventType constants.EventType

	// Convert typeSelection to integer
	index, err := strconv.Atoi(typeSelection)
	if err != nil || index < 1 || index > len(constants.AllEventTypes) {
		h.messageSenderService.Reply(
			msg,
			fmt.Sprintf("Неверный выбор. Пожалуйста, введи число от 1 до %d, или /%s для отмены:", len(constants.AllEventTypes), constants.CancelCommand),
			nil,
		)
		return nil // Stay in the same state
	}

	// Arrays are 0-indexed but our options start from 1
	eventType = constants.AllEventTypes[index-1]

	// Get the event name from user data store
	eventNameVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, eventSetupCtxDataKeyEventName)
	if !ok {
		h.messageSenderService.Reply(
			msg,
			fmt.Sprintf("Произошла внутренняя ошибка. Не удалось найти название мероприятия. Попробуй начать заново с /%s.", constants.EventSetupCommand),
			nil,
		)
		return handlers.EndConversation()
	}

	eventName, ok := eventNameVal.(string)
	if !ok {
		h.messageSenderService.Reply(
			msg,
			fmt.Sprintf("Произошла внутренняя ошибка (неверный тип названия). Попробуй начать заново с /%s.", constants.EventSetupCommand),
			nil,
		)
		return handlers.EndConversation()
	}

	// Create event in the database
	id, err := h.eventRepository.CreateEvent(eventName, eventType)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при создании записи о мероприятии.", nil)
		log.Printf("EventSetupHandler: Error during event creation: %v", err)
		return handlers.EndConversation()
	}

	// Store the event ID
	h.userStore.Set(ctx.EffectiveUser.Id, eventSetupCtxDataKeyEventID, id)

	// Ask for start date
	h.messageSenderService.Reply(
		msg,
		fmt.Sprintf("Когда стартует мероприятие? Введи дату и время в формате DD.MM.YYYY HH:MM или /%s для отмены.", constants.CancelCommand),
		nil,
	)

	return handlers.NextConversationState(eventSetupStateAskEventStartedAt)
}

// 4. handleEventStartedAt processes the start date input and updates the event
func (h *eventSetupHandler) handleEventStartedAt(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	dateTimeStr := strings.TrimSpace(msg.Text)

	// Parse the start date
	startedAt, err := time.Parse("02.01.2006 15:04", dateTimeStr)
	if err != nil {
		h.messageSenderService.Reply(
			msg,
			fmt.Sprintf("Неверный формат даты. Пожалуйста, введи дату и время в формате DD.MM.YYYY HH:MM или /%s для отмены.", constants.CancelCommand),
			nil,
		)
		return nil // Stay in the same state
	}

	// Get event ID from user data store
	eventIDVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, eventSetupCtxDataKeyEventID)
	if !ok {
		h.messageSenderService.Reply(
			msg,
			fmt.Sprintf("Произошла внутренняя ошибка. Не удалось найти ID мероприятия. Попробуй начать заново с /%s.", constants.EventSetupCommand),
			nil,
		)
		return handlers.EndConversation()
	}

	eventID, ok := eventIDVal.(int)
	if !ok {
		h.messageSenderService.Reply(
			msg,
			fmt.Sprintf("Произошла внутренняя ошибка (неверный тип ID). Попробуй начать заново с /%s.", constants.EventSetupCommand),
			nil,
		)
		return handlers.EndConversation()
	}

	// Update the started_at field
	err = h.eventRepository.UpdateEventStartedAt(eventID, startedAt)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при обновлении даты начала мероприятия.", nil)
		log.Printf("EventSetupHandler: Error during event update: %v", err)
		return handlers.EndConversation()
	}

	// Get event name for the success message
	eventNameVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, eventSetupCtxDataKeyEventName)
	if !ok {
		h.messageSenderService.Reply(msg, "Мероприятие успешно создано с датой старта.", nil)
		h.userStore.Clear(ctx.EffectiveUser.Id)
		return handlers.EndConversation()
	}

	eventName, ok := eventNameVal.(string)
	if !ok {
		h.messageSenderService.Reply(msg, "Мероприятие успешно создано с датой старта.", nil)
		h.userStore.Clear(ctx.EffectiveUser.Id)
		return handlers.EndConversation()
	}

	// Success message
	h.messageSenderService.Reply(
		msg,
		fmt.Sprintf("Запись о мероприятии '%s' успешно создана с ID: %d и датой старта: %s", eventName, eventID, startedAt.Format("02.01.2006 15:04")),
		nil,
	)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// 5. handleCancel handles the /cancel command
func (h *eventSetupHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	h.messageSenderService.Reply(msg, "Операция создания мероприятия отменена.", nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}
