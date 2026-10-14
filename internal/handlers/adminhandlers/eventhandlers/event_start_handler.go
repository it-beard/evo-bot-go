package eventhandlers

import (
	"fmt"
	"html"
	"log"
	"net/url"
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
	eventStartStateSelectEvent = "event_start_state_select_event"
	eventStartStateEnterLink   = "event_start_state_enter_link"
	eventStartStateConfirm     = "event_start_state_confirm"

	// Context data keys
	eventStartCtxDataKeySelectedEventID   = "event_start_ctx_data_key_selected_event_id"
	eventStartCtxDataKeyEventLink         = "event_start_ctx_data_key_event_link"
	eventStartCtxDataKeyPreviousMessageID = "event_start_ctx_data_key_previous_message_id"
	eventStartCtxDataKeyPreviousChatID    = "event_start_ctx_data_key_previous_chat_id"

	// Callbacks names
	eventStartCallbackConfirmYes    = "event_start_callback_confirm_yes"
	eventStartCallbackConfirmCancel = "event_start_callback_confirm_cancel"
)

type eventStartHandler struct {
	config               *config.Config
	eventRepository      *repositories.EventRepository
	messageSenderService *services.MessageSenderService
	userStore            *utils.UserDataStore
	permissionsService   *services.PermissionsService
}

func NewEventStartHandler(
	config *config.Config,
	eventRepository *repositories.EventRepository,
	messageSenderService *services.MessageSenderService,
	permissionsService *services.PermissionsService,
) ext.Handler {
	h := &eventStartHandler{
		config:               config,
		eventRepository:      eventRepository,
		messageSenderService: messageSenderService,
		userStore:            utils.NewUserDataStore(),
		permissionsService:   permissionsService,
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.EventStartCommand, h.startEvent),
		},
		map[string][]ext.Handler{
			eventStartStateSelectEvent: {
				handlers.NewMessage(message.Text, h.handleSelectEvent),
				handlers.NewCallback(callbackquery.Equal(eventStartCallbackConfirmCancel), h.handleCallbackCancel),
			},
			eventStartStateEnterLink: {
				handlers.NewMessage(message.Text, h.handleEnterLink),
				handlers.NewCallback(callbackquery.Equal(eventStartCallbackConfirmCancel), h.handleCallbackCancel),
			},
			eventStartStateConfirm: {
				handlers.NewCallback(callbackquery.Equal(eventStartCallbackConfirmYes), h.handleCallbackYes),
				handlers.NewCallback(callbackquery.Equal(eventStartCallbackConfirmCancel), h.handleCallbackCancel),
				handlers.NewMessage(message.All, h.handleTextDuringConfirmation),
			},
		},
		&handlers.ConversationOpts{
			Exits: []ext.Handler{handlers.NewCommand(constants.CancelCommand, h.handleCancel)},
		},
	)
}

// 1. startEvent is the entry point handler for the start conversation
func (h *eventStartHandler) startEvent(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if user has admin permissions and is in a private chat
	if !h.permissionsService.CheckAdminAndPrivateChat(msg, constants.EventStartCommand) {
		return handlers.EndConversation()
	}

	sentMsg, ok := sendAdminEventList(h.eventRepository, h.messageSenderService, msg,
		"которое ты хочешь запустить", eventStartCallbackConfirmCancel)
	if !ok {
		return handlers.EndConversation()
	}

	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
	return handlers.NextConversationState(eventStartStateSelectEvent)
}

// 2. handleSelectEvent processes the user's selection of an event to start
func (h *eventStartHandler) handleSelectEvent(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	event, ok := findSelectedEvent(h.eventRepository, h.messageSenderService, msg)
	if !ok {
		return nil // Stay in the same state
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	// Store the selected event ID
	h.userStore.Set(ctx.EffectiveUser.Id, eventStartCtxDataKeySelectedEventID, event.ID)

	sentMsg, _ := h.messageSenderService.ReplyHtmlWithReturnMessage(
		msg,
		fmt.Sprintf(
			"🚀 <b>Запуск мероприятия</b>\n\n%s\n\n🔗 Отправь мне ссылку на мероприятие, она будет опубликована в чате объявлений:",
			formatters.FormatEventSummary(*event),
		),
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.CancelButton(eventStartCallbackConfirmCancel),
		},
	)

	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
	return handlers.NextConversationState(eventStartStateEnterLink)
}

// 3. handleEnterLink processes the user's input of event link
func (h *eventStartHandler) handleEnterLink(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	eventLink := strings.TrimSpace(msg.Text)

	// The link becomes an inline button, so it has to be a valid absolute http(s) URL
	if !isValidEventLink(eventLink) {
		h.messageSenderService.Reply(msg, fmt.Sprintf(
			"Некорректная ссылка. Пожалуйста, отправь полную ссылку вида https://… %s.",
			formatters.CancelHint,
		), nil)
		return nil // Stay in the same state
	}

	eventID, ok := h.selectedEventID(ctx)
	if !ok {
		return handlers.EndConversation()
	}

	// Get event details to show in the confirmation message
	event, err := h.eventRepository.GetEventByID(eventID)
	if err != nil {
		h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)
		h.messageSenderService.Reply(msg, fmt.Sprintf("Произошла ошибка при получении мероприятия с ID %d.", eventID), nil)
		log.Printf("%s: Error during event retrieval: %v", utils.GetCurrentTypeName(), err)
		h.userStore.Clear(ctx.EffectiveUser.Id)
		return handlers.EndConversation()
	}

	// Store the event link
	h.userStore.Set(ctx.EffectiveUser.Id, eventStartCtxDataKeyEventLink, eventLink)

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	sentMsg, _ := h.messageSenderService.ReplyHtmlWithReturnMessage(msg, fmt.Sprintf(
		"🚀 <b>Запустить мероприятие?</b>\n\n%s\n\n🔗 Ссылка: <code>%s</code>\n\n"+
			"Объявление со ссылкой будет опубликовано и закреплено в чате объявлений, а мероприятие получит статус «завершено».",
		formatters.FormatEventSummary(*event), html.EscapeString(eventLink),
	), &gotgbot.SendMessageOpts{
		ReplyMarkup: buttons.ConfirmAndCancelButton(eventStartCallbackConfirmYes, eventStartCallbackConfirmCancel),
	})

	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
	return handlers.NextConversationState(eventStartStateConfirm)
}

// handleCallbackYes processes the confirmation button click
func (h *eventStartHandler) handleCallbackYes(b *gotgbot.Bot, ctx *ext.Context) error {
	// Answer the callback query to remove the loading state on the button
	cb := ctx.Update.CallbackQuery
	_, _ = cb.Answer(b, nil)

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)
	defer h.userStore.Clear(ctx.EffectiveUser.Id)

	eventID, ok := h.selectedEventID(ctx)
	if !ok {
		return handlers.EndConversation()
	}

	// Get the event link
	eventLinkVal, _ := h.userStore.Get(ctx.EffectiveUser.Id, eventStartCtxDataKeyEventLink)
	eventLink, ok := eventLinkVal.(string)
	if !ok {
		log.Printf("%s: Event link is missing or invalid: %v", utils.GetCurrentTypeName(), eventLinkVal)
		h.messageSenderService.Reply(ctx.EffectiveMessage, fmt.Sprintf(
			"Произошла ошибка: не удалось найти ссылку на мероприятие. Пожалуйста, начни заново с /%s.",
			constants.EventStartCommand,
		), nil)
		return handlers.EndConversation()
	}

	// Get the event details for the announcement
	event, err := h.eventRepository.GetEventByID(eventID)
	if err != nil {
		h.messageSenderService.Reply(ctx.EffectiveMessage, fmt.Sprintf("Произошла ошибка при получении мероприятия с ID %d.", eventID), nil)
		log.Printf("%s: Error during event retrieval: %v", utils.GetCurrentTypeName(), err)
		return handlers.EndConversation()
	}

	buttonWithLink := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text: "🔗 Ссылка на Zoom",
					Url:  eventLink,
				},
			},
		},
	}

	// Send announcement message with the event link to the announcement topic
	announcementMsg := fmt.Sprintf(
		"🔴 <b>НАЧИНАЕМ ИВЕНТ!</b> 🔴\n\n%s %s\n",
		formatters.GetTypeEmoji(constants.EventType(event.Type)),
		formatters.FormatEventName(event.Name),
	)

	if event.Type == string(constants.EventTypeClubCall) {
		announcementMsg += "💡 <a href=\"https://telegram.me/c/2069889012/127/33823\">Про формат и правила клубных созвонов</a>\n"
	}

	announcementMsg += "\nИспользуй кнопку ниже, чтобы присоединиться ⬇️"

	sentAnnouncementMsg, err := h.messageSenderService.SendHtmlWithReturnMessage(
		utils.ChatIdToFullChatId(h.config.SuperGroupChatID),
		announcementMsg,
		&gotgbot.SendMessageOpts{
			MessageThreadId: int64(h.config.AnnouncementTopicID),
			ReplyMarkup:     buttonWithLink,
		},
	)
	if err != nil || sentAnnouncementMsg == nil {
		// Nothing was announced, so the event stays actual and can be started again
		log.Printf("%s: Error sending announcement message: %v", utils.GetCurrentTypeName(), err)
		h.messageSenderService.Reply(ctx.EffectiveMessage, fmt.Sprintf(
			"Не удалось опубликовать объявление в чате объявлений, мероприятие не запущено. Проверь ссылку и попробуй ещё раз: /%s.",
			constants.EventStartCommand,
		), nil)
		return handlers.EndConversation()
	}

	// Pin the announcement message with notification for all users
	if err := h.messageSenderService.PinMessage(sentAnnouncementMsg.Chat.Id, sentAnnouncementMsg.MessageId, true); err != nil {
		log.Printf("%s: Error pinning announcement message: %v", utils.GetCurrentTypeName(), err)
	}

	// A started event is over for the bot: it disappears from /events, /topics and /topicAdd
	if err := h.eventRepository.UpdateEventStatus(eventID, constants.EventStatusFinished); err != nil {
		log.Printf("%s: Error during event update: %v", utils.GetCurrentTypeName(), err)
		h.messageSenderService.Reply(ctx.EffectiveMessage, fmt.Sprintf(
			"⚠️ Объявление опубликовано, но статус мероприятия обновить не удалось. Заверши его вручную через /%s.",
			constants.EventEditCommand,
		), nil)
		return handlers.EndConversation()
	}

	// Confirmation message
	h.messageSenderService.ReplyHtml(
		ctx.EffectiveMessage,
		fmt.Sprintf("✅ Мероприятие %s (ID %d) запущено!\n\n📢 Объявление со ссылкой опубликовано в чате объявлений.",
			formatters.FormatEventName(event.Name), event.ID),
		nil,
	)

	return handlers.EndConversation()
}

// handleTextDuringConfirmation handles text messages during the confirmation state
func (h *eventStartHandler) handleTextDuringConfirmation(b *gotgbot.Bot, ctx *ext.Context) error {
	h.messageSenderService.Reply(ctx.EffectiveMessage, formatters.UseButtonsText, nil)
	return nil // Stay in the same state
}

// selectedEventID returns the event chosen earlier in the conversation, or reports the problem to the user
func (h *eventStartHandler) selectedEventID(ctx *ext.Context) (int, bool) {
	eventIDVal, _ := h.userStore.Get(ctx.EffectiveUser.Id, eventStartCtxDataKeySelectedEventID)
	eventID, ok := eventIDVal.(int)
	if !ok {
		log.Printf("%s: Selected event ID is missing or invalid: %v", utils.GetCurrentTypeName(), eventIDVal)
		h.messageSenderService.Reply(ctx.EffectiveMessage, formatters.LostEventText(constants.EventStartCommand), nil)
		h.userStore.Clear(ctx.EffectiveUser.Id)
		return 0, false
	}
	return eventID, true
}

// isValidEventLink reports whether link is an absolute http(s) URL that Telegram accepts for a button
func isValidEventLink(link string) bool {
	parsed, err := url.Parse(link)
	if err != nil {
		return false
	}
	return (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

// handleCallbackCancel processes the cancel button click
func (h *eventStartHandler) handleCallbackCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	// Answer the callback query to remove the loading state on the button
	cb := ctx.Update.CallbackQuery
	_, _ = cb.Answer(b, nil)

	return h.handleCancel(b, ctx)
}

// handleCancel handles the /cancel command
func (h *eventStartHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	h.messageSenderService.Reply(msg, "Операция запуска мероприятия отменена.", nil)
	log.Printf("%s: Event start canceled", utils.GetCurrentTypeName())

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

func (h *eventStartHandler) MessageRemoveInlineKeyboard(b *gotgbot.Bot, userID *int64) {
	var chatID, messageID int64

	// If userID provided, get stored message info using the utility method
	if userID != nil {
		messageID, chatID = h.userStore.GetPreviousMessageInfo(
			*userID,
			eventStartCtxDataKeyPreviousMessageID,
			eventStartCtxDataKeyPreviousChatID,
		)
	}

	// Skip if we don't have valid chat and message IDs
	if chatID == 0 || messageID == 0 {
		return
	}

	// Use message sender service to remove the inline keyboard
	_ = h.messageSenderService.RemoveInlineKeyboard(chatID, messageID)
}

func (h *eventStartHandler) SavePreviousMessageInfo(userID int64, sentMsg *gotgbot.Message) {
	if sentMsg == nil {
		return
	}
	h.userStore.SetPreviousMessageInfo(userID, sentMsg.MessageId, sentMsg.Chat.Id,
		eventStartCtxDataKeyPreviousMessageID, eventStartCtxDataKeyPreviousChatID)
}
