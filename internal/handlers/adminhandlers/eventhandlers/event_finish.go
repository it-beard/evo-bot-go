package eventhandlers

import (
	"fmt"
	"log"
	"strconv"
	"strings"

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
	eventFinishStateSelectEvent = "event_finish_select_event"
	eventFinishStateConfirm     = "event_finish_confirm"

	// Context data keys
	eventFinishCtxDataKeySelectedEventID = "event_finish_selected_event_id"
)

// Confirmation message options
const (
	eventFinishConfirmYes = "да"
	eventFinishConfirmNo  = "нет"
)

type eventFinishHandler struct {
	eventRepository *repositories.EventRepository
	config          *config.Config
	userStore       *utils.UserDataStore
}

func NewEventFinishHandler(
	eventRepository *repositories.EventRepository,
	config *config.Config,
) ext.Handler {
	h := &eventFinishHandler{
		eventRepository: eventRepository,
		config:          config,
		userStore:       utils.NewUserDataStore(),
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.EventFinishCommand, h.startFinish),
		},
		map[string][]ext.Handler{
			eventFinishStateSelectEvent: {
				handlers.NewMessage(message.Text, h.handleSelectEvent),
			},
			eventFinishStateConfirm: {
				handlers.NewMessage(message.Text, h.handleConfirmation),
			},
		},
		&handlers.ConversationOpts{
			Exits: []ext.Handler{handlers.NewCommand(constants.CancelCommand, h.handleCancel)},
		},
	)
}

// 1. startFinish is the entry point handler for the finish conversation
func (h *eventFinishHandler) startFinish(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check admin permissions and private chat
	if !utils.CheckAdminAndPrivateChat(b, ctx, h.config.SuperGroupChatID, constants.EventFinishCommand) {
		return handlers.EndConversation()
	}

	// Get a list of active events
	events, err := h.eventRepository.GetLastActualEvents(constants.EventEditGetLastLimit)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Произошла ошибка при получении списка актуальных событий.", err)
		return handlers.EndConversation()
	}

	if len(events) == 0 {
		utils.SendLoggedReply(b, msg, "Нет активных событий для завершения.", nil)
		return handlers.EndConversation()
	}

	title := fmt.Sprintf("Последние %d события:", len(events))
	actionDescription := "которое ты хочешь завершить"
	formattedResponse := utils.FormatEventListForAdmin(events, title, constants.CancelCommand, actionDescription)

	utils.SendLoggedMarkdownReply(b, msg, formattedResponse, nil)

	return handlers.NextConversationState(eventFinishStateSelectEvent)
}

// 2. handleSelectEvent processes the user's selection of an event to finish
func (h *eventFinishHandler) handleSelectEvent(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	eventIDStr := strings.TrimSpace(msg.Text)

	eventID, err := strconv.Atoi(eventIDStr)
	if err != nil {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Неверный ID. Пожалуйста, введи числовой ID или /%s для отмены.", constants.CancelCommand), nil)
		return nil // Stay in the same state
	}

	// Store the selected event ID
	h.userStore.Set(ctx.EffectiveUser.Id, eventFinishCtxDataKeySelectedEventID, eventID)

	// Ask for confirmation
	utils.SendLoggedReply(b, msg, fmt.Sprintf(
		"Ты действительно хочешь завершить это событие? Это пометит его как неактуальное.\n\nВведи 'да' для подтверждения или 'нет' для отмены (или используй /%s):",
		constants.CancelCommand,
	), nil)

	return handlers.NextConversationState(eventFinishStateConfirm)
}

// 3. handleConfirmation processes the user's confirmation to finish the event
func (h *eventFinishHandler) handleConfirmation(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	confirmationText := strings.ToLower(strings.TrimSpace(msg.Text))

	// Check the confirmation
	if confirmationText != eventFinishConfirmYes && confirmationText != eventFinishConfirmNo {
		utils.SendLoggedReply(b, msg, fmt.Sprintf(
			"Пожалуйста, введи 'да' для подтверждения или 'нет' для отмены (или используй /%s):",
			constants.CancelCommand,
		), nil)
		return nil // Stay in the same state
	}

	// If user said "no", cancel the operation
	if confirmationText == eventFinishConfirmNo {
		utils.SendLoggedReply(b, msg, "Операция завершения события отменена.", nil)
		h.userStore.Clear(ctx.EffectiveUser.Id)
		return handlers.EndConversation()
	}

	// Get the selected event ID
	eventIDVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, eventFinishCtxDataKeySelectedEventID)
	if !ok {
		utils.SendLoggedReply(b, msg, fmt.Sprintf(
			"Произошла ошибка при получении выбранного события. Пожалуйста, начни заново с /%s",
			constants.EventFinishCommand,
		), nil)
		return handlers.EndConversation()
	}

	eventID, ok := eventIDVal.(int)
	if !ok {
		log.Println("Invalid event ID type:", eventIDVal)
		utils.SendLoggedReply(b, msg, fmt.Sprintf(
			"Произошла внутренняя ошибка (неверный тип ID). Пожалуйста, начни заново с /%s",
			constants.EventFinishCommand,
		), nil)
		return handlers.EndConversation()
	}

	// Get the event details for the success message
	event, err := h.eventRepository.GetEventByID(eventID)
	if err != nil {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Ошибка при получении события с ID %d", eventID), err)
		return handlers.EndConversation()
	}

	// Update the event status to finished
	err = h.eventRepository.UpdateEventStatus(eventID, constants.EventStatusFinished)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Произошла ошибка при обновлении статуса события.", err)
		return handlers.EndConversation()
	}

	// Confirmation message
	utils.SendLoggedReply(b, msg, fmt.Sprintf("Событие '%s' (ID: %d) успешно завершено.", event.Name, event.ID), nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// 4. handleCancel handles the /cancel command
func (h *eventFinishHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	utils.SendLoggedReply(b, msg, "Операция завершения события отменена.", nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}
