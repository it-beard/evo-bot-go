package topicshandlers

import (
	"context"
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
	topicsStateSelectEvent = "topics_state_select_event"

	// Context data keys
	topicsCtxDataKeyCancelFunc        = "topics_ctx_data_cancel_func"
	topicsCtxDataKeyPreviousMessageID = "topics_ctx_data_previous_message_id"
	topicsCtxDataKeyPreviousChatID    = "topics_ctx_data_previous_chat_id"

	// Callback data
	topicsCallbackConfirmCancel = "topics_callback_confirm_cancel"
)

type topicsHandler struct {
	config               *config.Config
	topicRepository      *repositories.TopicRepository
	eventRepository      *repositories.EventRepository
	messageSenderService *services.MessageSenderService
	userStore            *utils.UserDataStore
	permissionsService   *services.PermissionsService
}

func NewTopicsHandler(
	config *config.Config,
	topicRepository *repositories.TopicRepository,
	eventRepository *repositories.EventRepository,
	messageSenderService *services.MessageSenderService,
	permissionsService *services.PermissionsService,
) ext.Handler {
	h := &topicsHandler{
		config:               config,
		topicRepository:      topicRepository,
		eventRepository:      eventRepository,
		messageSenderService: messageSenderService,
		userStore:            utils.NewUserDataStore(),
		permissionsService:   permissionsService,
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.TopicsCommand, h.startTopics),
		},
		map[string][]ext.Handler{
			topicsStateSelectEvent: {
				handlers.NewMessage(message.All, h.handleEventSelection),
				handlers.NewCallback(callbackquery.Equal(topicsCallbackConfirmCancel), h.handleCallbackCancel),
			},
		},
		&handlers.ConversationOpts{
			Exits: []ext.Handler{handlers.NewCommand(constants.CancelCommand, h.handleCancel)},
		},
	)
}

// 1. startTopics is the entry point handler for showing topics
func (h *topicsHandler) startTopics(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Only proceed if this is a private chat
	if !h.permissionsService.CheckPrivateChatType(msg) {
		return handlers.EndConversation()
	}

	// Check if user is a club member
	if !h.permissionsService.CheckClubMemberPermissions(msg, constants.TopicsCommand) {
		return handlers.EndConversation()
	}

	// Get last actual events to show for selection
	events, err := h.eventRepository.GetLastActualEvents(10)
	if err != nil {
		h.messageSenderService.Reply(msg, "Ошибка при получении списка мероприятий.", nil)
		log.Printf("TopicsHandler: Error during events retrieval: %v", err)
		return handlers.EndConversation()
	}

	if len(events) == 0 {
		h.messageSenderService.Reply(msg, "Нет доступных мероприятий для просмотра тем и вопросов.", nil)
		return handlers.EndConversation()
	}

	// Format and display event list for selection
	formattedEvents := formatters.FormatEventListForUsers(
		events,
		fmt.Sprintf("Выбери ID мероприятия, для которого ты хочешь увидеть темы и вопросы"),
	)

	sentMsg, _ := h.messageSenderService.ReplyMarkdownWithReturnMessage(
		msg,
		formattedEvents,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: formatters.CancelButton(topicsCallbackConfirmCancel),
		},
	)

	h.SavePreviousMessageInfo(ctx.EffectiveUser.Id, sentMsg)
	return handlers.NextConversationState(topicsStateSelectEvent)
}

// 2. handleEventSelection processes the user's event selection
func (h *topicsHandler) handleEventSelection(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userInput := strings.TrimSpace(strings.Replace(msg.Text, "/", "", 1))

	// Check if the input is a valid event ID
	eventID, err := strconv.Atoi(userInput)
	if err != nil {
		h.messageSenderService.Reply(
			msg,
			fmt.Sprintf("Пожалуйста, отправь корректный ID мероприятия или используй /%s для отмены.", constants.CancelCommand),
			nil,
		)
		return nil // Stay in the same state
	}

	// Get the event information
	event, err := h.eventRepository.GetEventByID(eventID)
	if err != nil {
		h.messageSenderService.Reply(
			msg,
			fmt.Sprintf("Не удалось найти мероприятие с ID %d. Пожалуйста, проверь ID.", eventID),
			nil,
		)
		log.Printf("TopicsHandler: Error during event retrieval: %v", err)
		return nil // Stay in the same state
	}

	// Get topics for this event
	topics, err := h.topicRepository.GetTopicsByEventID(eventID)
	if err != nil {
		h.messageSenderService.Reply(msg, "Ошибка при получении тем и вопросов для выбранного мероприятия.", nil)
		log.Printf("TopicsHandler: Error during topics retrieval: %v", err)
		return handlers.EndConversation()
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)
	// Format and display topics
	formattedTopics := formatters.FormatTopicListForUsers(topics, event.Name, event.Type)
	h.messageSenderService.ReplyMarkdown(msg, formattedTopics, nil)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// handleCallbackCancel processes the cancel button click
func (h *topicsHandler) handleCallbackCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	// Answer the callback query to remove the loading state on the button
	cb := ctx.Update.CallbackQuery
	_, _ = cb.Answer(b, nil)

	return h.handleCancel(b, ctx)
}

// 3. handleCancel handles the /cancel command
func (h *topicsHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if there's an ongoing operation to cancel
	if cancelFunc, ok := h.userStore.Get(ctx.EffectiveUser.Id, topicsCtxDataKeyCancelFunc); ok {
		// Call the cancel function to stop any ongoing API calls
		if cf, ok := cancelFunc.(context.CancelFunc); ok {
			cf()
			h.messageSenderService.Reply(msg, "Операция просмотра тем отменена.", nil)
		}
	} else {
		h.messageSenderService.Reply(msg, "Операция просмотра тем отменена.", nil)
	}

	h.MessageRemoveInlineKeyboard(b, &ctx.EffectiveUser.Id)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

func (h *topicsHandler) MessageRemoveInlineKeyboard(b *gotgbot.Bot, userID *int64) {
	var chatID, messageID int64

	// If userID provided, try to get stored message info
	if userID != nil {
		if val, ok := h.userStore.Get(*userID, topicsCtxDataKeyPreviousMessageID); ok {
			messageID = val.(int64)
		}
		if val, ok := h.userStore.Get(*userID, topicsCtxDataKeyPreviousChatID); ok {
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

func (h *topicsHandler) SavePreviousMessageInfo(userID int64, sentMsg *gotgbot.Message) {
	h.userStore.Set(userID, topicsCtxDataKeyPreviousMessageID, sentMsg.MessageId)
	h.userStore.Set(userID, topicsCtxDataKeyPreviousChatID, sentMsg.Chat.Id)
}
