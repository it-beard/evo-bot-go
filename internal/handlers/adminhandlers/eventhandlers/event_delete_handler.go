package eventhandlers

import (
	"fmt"
	"log"
	"strconv"
	"strings"

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
	eventDeleteStateSelectEvent = "event_delete_state_select_event"
	eventDeleteStateConfirm     = "event_delete_state_confirm"

	// Context data keys
	eventDeleteCtxDataKeySelectedEventID   = "event_delete_ctx_data_selected_event_id"
	eventDeleteCtxDataKeySelectedEventName = "event_delete_ctx_data_selected_event_name"
	eventDeleteCtxDataKeyPreviousMessageID = "event_delete_ctx_data_previous_message_id"
	eventDeleteCtxDataKeyPreviousChatID    = "event_delete_ctx_data_previous_chat_id"

	// Callback data
	eventDeleteCallbackConfirmCancel = "event_delete_callback_confirm_cancel"
	eventDeleteCallbackConfirmYes    = "event_delete_callback_confirm_yes"
)

type eventDeleteHandler struct {
	config               *config.Config
	eventRepository      *repositories.EventRepository
	messageSenderService *services.MessageSenderService
	userStore            *utils.UserDataStore
	permissionsService   *services.PermissionsService
}

func NewEventDeleteHandler(
	config *config.Config,
	eventRepository *repositories.EventRepository,
	messageSenderService *services.MessageSenderService,
	permissionsService *services.PermissionsService,
) ext.Handler {
	h := &eventDeleteHandler{
		config:               config,
		eventRepository:      eventRepository,
		messageSenderService: messageSenderService,
		userStore:            utils.NewUserDataStore(),
		permissionsService:   permissionsService,
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.EventDeleteCommand, h.startDelete),
		},
		map[string][]ext.Handler{
			eventDeleteStateSelectEvent: {
				handlers.NewMessage(message.Text, h.handleSelectEvent),
				handlers.NewCallback(callbackquery.Equal(eventDeleteCallbackConfirmCancel), h.handleCallbackCancel),
			},
			eventDeleteStateConfirm: {
				handlers.NewCallback(callbackquery.Equal(eventDeleteCallbackConfirmYes), h.handleCallbackConfirmYes),
				handlers.NewCallback(callbackquery.Equal(eventDeleteCallbackConfirmCancel), h.handleCallbackCancel),
				handlers.NewMessage(message.Text, h.handleMessageDuringConfirmation),
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

	// Check if user has admin permissions and is in a private chat
	if !h.permissionsService.CheckAdminAndPrivateChat(msg, constants.ShowTopicsCommand) {
		return handlers.EndConversation()
	}

	// Get a list of the last N events
	events, err := h.eventRepository.GetLastEvents(constants.EventEditGetLastLimit)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при получении списка мероприятий.", nil)
		log.Printf("%s: Error during event retrieval: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	if len(events) == 0 {
		h.messageSenderService.Reply(msg, "Нет созданных мероприятий для удаления.", nil)
		return handlers.EndConversation()
	}

	title := fmt.Sprintf("Последние %d мероприятия:", len(events))
	actionDescription := "которое ты хочешь удалить"
	formattedResponse := formatters.FormatEventListForAdmin(events, title, constants.CancelCommand, actionDescription)

	sentMsg, _ := h.messageSenderService.ReplyMarkdownWithReturnMessage(
		msg,
		formattedResponse,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: formatters.CancelButton(eventDeleteCallbackConfirmCancel),
		},
	)

	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
	return handlers.NextConversationState(eventDeleteStateSelectEvent)
}

// 2. handleSelectEvent processes the user's selection of an event to delete
func (h *eventDeleteHandler) handleSelectEvent(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	eventIDStr := strings.TrimSpace(strings.Replace(msg.Text, "/", "", 1))
	eventID, err := strconv.Atoi(eventIDStr)
	if err != nil {
		h.messageSenderService.Reply(msg, "Некорректный ID. Пожалуйста, введи числовой ID или используй кнопку для отмены.", nil)
		return nil // Stay in the same state
	}

	// Get the last N events
	events, err := h.eventRepository.GetLastEvents(constants.EventEditGetLastLimit)
	if err != nil {
		h.messageSenderService.Reply(msg, "Произошла ошибка при получении списка мероприятий.", nil)
		log.Printf("%s: Error during event retrieval: %v", utils.GetCurrentTypeName(), err)
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
		h.messageSenderService.Reply(
			msg,
			fmt.Sprintf("Мероприятие с ID %d не найдено. Пожалуйста, введи корректный ID или используй кнопку для отмены.", eventID),
			nil,
		)
		return nil // Stay in the same state
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	// Store event ID and name for confirmation
	h.userStore.Set(ctx.EffectiveUser.Id, eventDeleteCtxDataKeySelectedEventID, eventID)
	h.userStore.Set(ctx.EffectiveUser.Id, eventDeleteCtxDataKeySelectedEventName, eventName)

	// Ask for confirmation
	confirmMessage := fmt.Sprintf(
		"Ты действительно хочешь удалить мероприятие '*%s*' (ID: %d)?\n\nЭто также удалит все связанные с ним темы и вопросы.",
		eventName, eventID)

	sentMsg, _ := h.messageSenderService.ReplyMarkdownWithReturnMessage(
		msg,
		confirmMessage,
		&gotgbot.SendMessageOpts{
			ParseMode:   "Markdown",
			ReplyMarkup: formatters.ConfirmAndCancelButton(eventDeleteCallbackConfirmYes, eventDeleteCallbackConfirmCancel),
		},
	)

	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
	return handlers.NextConversationState(eventDeleteStateConfirm)
}

// handleMessageDuringConfirmation handles text messages during the confirmation state
func (h *eventDeleteHandler) handleMessageDuringConfirmation(b *gotgbot.Bot, ctx *ext.Context) error {
	h.messageSenderService.Reply(
		ctx.EffectiveMessage,
		"Пожалуйста, используй кнопки выше для подтверждения или отмены.",
		nil,
	)
	return nil // Stay in the same state
}

// handleCallbackConfirmYes processes the confirmation to delete the event
func (h *eventDeleteHandler) handleCallbackConfirmYes(b *gotgbot.Bot, ctx *ext.Context) error {
	// Answer the callback query to remove the loading state on the button
	cb := ctx.Update.CallbackQuery
	_, _ = cb.Answer(b, nil)

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	// Get the selected event ID
	eventIDVal, ok := h.userStore.Get(ctx.EffectiveUser.Id, eventDeleteCtxDataKeySelectedEventID)
	if !ok {
		h.messageSenderService.Reply(
			ctx.EffectiveMessage,
			fmt.Sprintf(
				"Произошла ошибка при получении выбранного мероприятия. Пожалуйста, начни заново с /%s",
				constants.EventDeleteCommand,
			),
			nil,
		)
		log.Printf("%s: Error during event retrieval.", utils.GetCurrentTypeName())
		return handlers.EndConversation()
	}

	eventID, ok := eventIDVal.(int)
	if !ok {
		log.Println("Invalid event ID type:", eventIDVal)
		h.messageSenderService.Reply(
			ctx.EffectiveMessage,
			fmt.Sprintf(
				"Произошла внутренняя ошибка (неверный тип ID). Пожалуйста, начни заново с /%s",
				constants.EventDeleteCommand,
			),
			nil,
		)
		log.Printf("%s: Invalid event ID type: %v", utils.GetCurrentTypeName(), eventIDVal)
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
		h.messageSenderService.Reply(ctx.EffectiveMessage, "Произошла ошибка при удалении мероприятия.", nil)
		log.Printf("%s: Error during event deletion: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	// Confirmation message
	h.messageSenderService.ReplyMarkdown(
		ctx.EffectiveMessage,
		fmt.Sprintf("Мероприятие '*%s*' успешно удалено. \n\nДля просмотра всех команд используй команду /%s", eventName, constants.HelpCommand),
		&gotgbot.SendMessageOpts{
			ParseMode: "Markdown",
		},
	)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// handleCallbackCancel processes the cancel button click
func (h *eventDeleteHandler) handleCallbackCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	// Answer the callback query to remove the loading state on the button
	cb := ctx.Update.CallbackQuery
	_, _ = cb.Answer(b, nil)

	return h.handleCancel(b, ctx)
}

// 4. handleCancel handles the /cancel command
func (h *eventDeleteHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)
	h.messageSenderService.Reply(msg, "Операция удаления мероприятия отменена.", nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

func (h *eventDeleteHandler) MessageRemoveInlineKeyboard(b *gotgbot.Bot, userID *int64) {
	var chatID, messageID int64

	// If userID provided, try to get stored message info
	if userID != nil {
		if val, ok := h.userStore.Get(*userID, eventDeleteCtxDataKeyPreviousMessageID); ok {
			messageID = val.(int64)
		}
		if val, ok := h.userStore.Get(*userID, eventDeleteCtxDataKeyPreviousChatID); ok {
			chatID = val.(int64)
		}
	}

	// Skip if we don't have valid chat and message IDs
	if chatID == 0 || messageID == 0 {
		return
	}

	// Remove the inline keyboard
	if _, _, err := b.EditMessageReplyMarkup(&gotgbot.EditMessageReplyMarkupOpts{
		ChatId:      chatID,
		MessageId:   messageID,
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{},
	}); err != nil {
		log.Printf("%s: Error removing inline keyboard: %v", utils.GetCurrentTypeName(), err)
	}
}

func (h *eventDeleteHandler) SavePreviousMessageInfo(userID int64, sentMsg *gotgbot.Message) {
	h.userStore.Set(userID, eventDeleteCtxDataKeyPreviousMessageID, sentMsg.MessageId)
	h.userStore.Set(userID, eventDeleteCtxDataKeyPreviousChatID, sentMsg.Chat.Id)
}
