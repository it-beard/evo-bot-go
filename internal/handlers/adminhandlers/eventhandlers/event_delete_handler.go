package eventhandlers

import (
	"fmt"
	"log"

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
	if !h.permissionsService.CheckAdminAndPrivateChat(msg, constants.EventDeleteCommand) {
		return handlers.EndConversation()
	}

	sentMsg, ok := sendAdminEventList(h.eventRepository, h.messageSenderService, msg,
		"которое ты хочешь удалить", eventDeleteCallbackConfirmCancel)
	if !ok {
		return handlers.EndConversation()
	}

	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
	return handlers.NextConversationState(eventDeleteStateSelectEvent)
}

// 2. handleSelectEvent processes the user's selection of an event to delete
func (h *eventDeleteHandler) handleSelectEvent(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	event, ok := findSelectedEvent(h.eventRepository, h.messageSenderService, msg)
	if !ok {
		return nil // Stay in the same state
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	// Store event ID and name for confirmation
	h.userStore.Set(ctx.EffectiveUser.Id, eventDeleteCtxDataKeySelectedEventID, event.ID)
	h.userStore.Set(ctx.EffectiveUser.Id, eventDeleteCtxDataKeySelectedEventName, event.Name)

	// Ask for confirmation
	sentMsg, _ := h.messageSenderService.ReplyHtmlWithReturnMessage(
		msg,
		fmt.Sprintf(
			"🗑 <b>Удалить мероприятие?</b>\n\n%s\n\n⚠️ Вместе с ним будут удалены все его темы и вопросы. Это действие нельзя отменить.",
			formatters.FormatEventSummary(*event),
		),
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.ConfirmAndCancelButton(eventDeleteCallbackConfirmYes, eventDeleteCallbackConfirmCancel),
		},
	)

	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
	return handlers.NextConversationState(eventDeleteStateConfirm)
}

// handleMessageDuringConfirmation handles text messages during the confirmation state
func (h *eventDeleteHandler) handleMessageDuringConfirmation(b *gotgbot.Bot, ctx *ext.Context) error {
	h.messageSenderService.Reply(ctx.EffectiveMessage, formatters.UseButtonsText, nil)
	return nil // Stay in the same state
}

// handleCallbackConfirmYes processes the confirmation to delete the event
func (h *eventDeleteHandler) handleCallbackConfirmYes(b *gotgbot.Bot, ctx *ext.Context) error {
	// Answer the callback query to remove the loading state on the button
	cb := ctx.Update.CallbackQuery
	_, _ = cb.Answer(b, nil)

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	// Get the selected event ID
	eventIDVal, _ := h.userStore.Get(ctx.EffectiveUser.Id, eventDeleteCtxDataKeySelectedEventID)
	eventID, ok := eventIDVal.(int)
	if !ok {
		log.Printf("%s: Selected event ID is missing or invalid: %v", utils.GetCurrentTypeName(), eventIDVal)
		h.messageSenderService.Reply(ctx.EffectiveMessage, formatters.LostEventText(constants.EventDeleteCommand), nil)
		h.userStore.Clear(ctx.EffectiveUser.Id)
		return handlers.EndConversation()
	}

	// The name is only needed for the success message
	eventNameVal, _ := h.userStore.Get(ctx.EffectiveUser.Id, eventDeleteCtxDataKeySelectedEventName)
	eventName, _ := eventNameVal.(string)

	// Delete the event
	err := h.eventRepository.DeleteEvent(eventID)
	if err != nil {
		h.messageSenderService.Reply(ctx.EffectiveMessage, "Произошла ошибка при удалении мероприятия.", nil)
		log.Printf("%s: Error during event deletion: %v", utils.GetCurrentTypeName(), err)
		h.userStore.Clear(ctx.EffectiveUser.Id)
		return handlers.EndConversation()
	}

	// Confirmation message
	h.messageSenderService.ReplyHtml(
		ctx.EffectiveMessage,
		fmt.Sprintf("✅ Мероприятие %s (ID %d) удалено.\n\nСписок всех команд — /%s.",
			formatters.FormatEventName(eventName), eventID, constants.HelpCommand),
		nil,
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

	// If userID provided, get stored message info using the utility method
	if userID != nil {
		messageID, chatID = h.userStore.GetPreviousMessageInfo(
			*userID,
			eventDeleteCtxDataKeyPreviousMessageID,
			eventDeleteCtxDataKeyPreviousChatID,
		)
	}

	// Skip if we don't have valid chat and message IDs
	if chatID == 0 || messageID == 0 {
		return
	}

	// Use message sender service to remove the inline keyboard
	_ = h.messageSenderService.RemoveInlineKeyboard(chatID, messageID)
}

func (h *eventDeleteHandler) SavePreviousMessageInfo(userID int64, sentMsg *gotgbot.Message) {
	if sentMsg == nil {
		return
	}
	h.userStore.SetPreviousMessageInfo(userID, sentMsg.MessageId, sentMsg.Chat.Id,
		eventDeleteCtxDataKeyPreviousMessageID, eventDeleteCtxDataKeyPreviousChatID)
}
