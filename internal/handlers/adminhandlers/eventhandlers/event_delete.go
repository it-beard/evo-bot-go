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
	eventDeleteStateSelectEvent = "event_delete_select_event"
	eventDeleteStateConfirm     = "event_delete_confirm"

	// Context data keys
	eventDeleteCtxDataKeySelectedEventID   = "event_delete_selected_event_id"
	eventDeleteCtxDataKeySelectedEventName = "event_delete_selected_event_name"
)

// Confirmation message options
const (
	eventDeleteConfirmYes = "да"
	eventDeleteConfirmNo  = "нет"
)

type eventDeleteHandler struct {
	eventRepository *repositories.EventRepository
	config          *config.Config
	userStore       *utils.UserDataStore
}

func NewEventDeleteHandler(
	eventRepository *repositories.EventRepository,
	config *config.Config,
) ext.Handler {
	h := &eventDeleteHandler{
		eventRepository: eventRepository,
		config:          config,
		userStore:       utils.NewUserDataStore(),
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.EventDeleteCommand, h.startDelete),
		},
		map[string][]ext.Handler{
			eventDeleteStateSelectEvent: {
				handlers.NewMessage(message.Text, h.handleSelectEvent),
			},
			eventDeleteStateConfirm: {
				handlers.NewMessage(message.Text, h.handleConfirmation),
			},
		},
		&handlers.ConversationOpts{
			Exits: []ext.Handler{handlers.NewCommand(constants.CancelCommand, h.handleCancel)},
		},
	)
}

// 1. startDelete is the entry point handler for the delete conversation
func (h *eventDeleteHandler) startDelete(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check admin permissions and private chat
	if !utils.CheckAdminAndPrivateChat(b, ctx, h.config.SuperGroupChatID, constants.EventDeleteCommand) {
		return handlers.EndConversation()
	}

	// Get a list of the last N events
	events, err := h.eventRepository.GetLastEvents(constants.EventEditGetLastLimit)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Произошла ошибка при получении списка событий.", err)
		return handlers.EndConversation()
	}

	if len(events) == 0 {
		utils.SendLoggedReply(b, msg, "Нет созданных событий для удаления.", nil)
		return handlers.EndConversation()
	}

	title := fmt.Sprintf("Последние %d события:", len(events))
	actionDescription := "которое ты хочешь удалить"
	formattedResponse := utils.FormatEventListForAdmin(events, title, constants.CancelCommand, actionDescription)

	utils.SendLoggedMarkdownReply(b, msg, formattedResponse, nil)

	return handlers.NextConversationState(eventDeleteStateSelectEvent)
}

// 2. handleSelectEvent processes the user's selection of an event to delete
func (h *eventDeleteHandler) handleSelectEvent(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	eventIDStr := strings.TrimSpace(msg.Text)
	eventID, err := strconv.Atoi(eventIDStr)
	if err != nil {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Некорректный ID. Пожалуйста, введи числовой ID или /%s для отмены.", constants.CancelCommand), nil)
		return nil // Stay in the same state
	}

	// Get the last N events
	events, err := h.eventRepository.GetLastEvents(constants.EventEditGetLastLimit)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Произошла ошибка при получении списка событий.", err)
		return handlers.EndConversation()
	}

	// Find the event with the given ID
	var found bool
	var eventName string
	for _, event := range events {
		if event.ID == eventID {
			found = true
			eventName = event.Name
			break
		}
	}

	if !found {
		utils.SendLoggedReply(b, msg, fmt.Sprintf("Событие с ID %d не найдено. Пожалуйста, введи корректный ID или /%s для отмены.", eventID, constants.CancelCommand), nil)
		return nil // Stay in the same state
	}

	// Store event ID and name for confirmation
	h.userStore.Set(ctx.EffectiveUser.Id, eventDeleteCtxDataKeySelectedEventID, eventID)
	h.userStore.Set(ctx.EffectiveUser.Id, eventDeleteCtxDataKeySelectedEventName, eventName)

	// Ask for confirmation
	confirmMessage := fmt.Sprintf(
		"Ты действительно хочешь удалить событие '%s' (ID: %d)? Это также удалит все связанные с ним темы.\n\nВведи 'да' для подтверждения или 'нет' для отмены (или используй /%s):",
		eventName, eventID, constants.CancelCommand)

	utils.SendLoggedReply(b, msg, confirmMessage, nil)

	return handlers.NextConversationState(eventDeleteStateConfirm)
}

// 3. handleConfirmation processes the user's confirmation to delete the event
func (h *eventDeleteHandler) handleConfirmation(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	confirmationText := strings.ToLower(strings.TrimSpace(msg.Text))

	// Check the confirmation
	if confirmationText != eventDeleteConfirmYes && confirmationText != eventDeleteConfirmNo {
		utils.SendLoggedReply(b, msg, fmt.Sprintf(
			"Пожалуйста, введи 'да' для подтверждения или 'нет' для отмены (или используй /%s):",
			constants.CancelCommand,
		), nil)
		return nil // Stay in the same state
	}

	// If user said "no", cancel the operation
	if confirmationText == eventDeleteConfirmNo {
		utils.SendLoggedReply(b, msg, "Операция удаления события отменена.", nil)
		h.userStore.Clear(ctx.EffectiveUser.Id)
		return handlers.EndConversation()
	}

	// Get the selected event ID
	eventIDVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, eventDeleteCtxDataKeySelectedEventID)
	if !ok {
		utils.SendLoggedReply(b, msg, fmt.Sprintf(
			"Произошла ошибка при получении выбранного события. Пожалуйста, начни заново с /%s",
			constants.EventDeleteCommand,
		), nil)
		return handlers.EndConversation()
	}

	eventID, ok := eventIDVal.(int)
	if !ok {
		log.Println("Invalid event ID type:", eventIDVal)
		utils.SendLoggedReply(b, msg, fmt.Sprintf(
			"Произошла внутренняя ошибка (неверный тип ID). Пожалуйста, начни заново с /%s",
			constants.EventDeleteCommand,
		), nil)
		return handlers.EndConversation()
	}

	// Get the event details for the success message
	eventNameVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, eventDeleteCtxDataKeySelectedEventName)
	if !ok {
		// Not critical, we can proceed without the name
		eventNameVal = "неизвестно"
	}
	eventName, _ := eventNameVal.(string)

	// Delete the event
	err := h.eventRepository.DeleteEvent(eventID)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Произошла ошибка при удалении события.", err)
		return handlers.EndConversation()
	}

	// Confirmation message
	utils.SendLoggedReply(b, msg, fmt.Sprintf("Событие '%s' успешно удалено.", eventName), nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// 4. handleCancel handles the /cancel command
func (h *eventDeleteHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	utils.SendLoggedReply(b, msg, "Операция удаления события отменена.", nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}
