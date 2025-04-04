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
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
)

const (
	// Conversation states
	topicsStateSelectEvent = "topics_select_event"

	// UserStore keys
	topicsUserStoreKeyCancelFunc = "topics_cancel_func"
)

type topicsHandler struct {
	config               *config.Config
	topicRepository      *repositories.TopicRepository
	eventRepository      *repositories.EventRepository
	messageSenderService services.MessageSenderService
	userStore            *utils.UserDataStore
}

func NewTopicsHandler(
	config *config.Config,
	topicRepository *repositories.TopicRepository,
	eventRepository *repositories.EventRepository,
	messageSenderService services.MessageSenderService,
) ext.Handler {
	h := &topicsHandler{
		config:               config,
		topicRepository:      topicRepository,
		eventRepository:      eventRepository,
		messageSenderService: messageSenderService,
		userStore:            utils.NewUserDataStore(),
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.TopicsCommand, h.startTopics),
		},
		map[string][]ext.Handler{
			topicsStateSelectEvent: {
				handlers.NewMessage(message.All, h.handleEventSelection),
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
	if !utils.CheckPrivateChatType(b, ctx) {
		return handlers.EndConversation()
	}

	// Check if user is a club member
	if !utils.CheckClubMemberPermissions(b, msg, h.config, constants.TopicsCommand) {
		return handlers.EndConversation()
	}

	// Get last actual events to show for selection
	events, err := h.eventRepository.GetLastActualEvents(10)
	if err != nil {
		h.messageSenderService.Reply(b, msg, "Ошибка при получении списка мероприятий.", nil)
		log.Printf("TopicsHandler: Error during events retrieval: %v", err)
		return handlers.EndConversation()
	}

	if len(events) == 0 {
		h.messageSenderService.Reply(b, msg, "Нет доступных мероприятий для просмотра тем и вопросов.", nil)
		return handlers.EndConversation()
	}

	// Format and display event list for selection
	formattedEvents := formatters.FormatEventListForUsers(
		events,
		fmt.Sprintf("Выбери ID мероприятия, для которого ты хочешь увидеть темы и вопросы, либо жми /%s для отмены диалога", constants.CancelCommand),
	)

	h.messageSenderService.ReplyMarkdown(b, msg, formattedEvents, nil)

	return handlers.NextConversationState(topicsStateSelectEvent)
}

// 2. handleEventSelection processes the user's event selection
func (h *topicsHandler) handleEventSelection(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userInput := strings.TrimSpace(msg.Text)

	// Check if the input is a valid event ID
	eventID, err := strconv.Atoi(userInput)
	if err != nil {
		h.messageSenderService.Reply(
			b,
			msg,
			fmt.Sprintf("Пожалуйста, отправь корректный ID мероприятия или /%s для отмены.", constants.CancelCommand),
			nil,
		)
		return nil // Stay in the same state
	}

	// Get the event information
	event, err := h.eventRepository.GetEventByID(eventID)
	if err != nil {
		h.messageSenderService.Reply(
			b,
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
		h.messageSenderService.Reply(b, msg, "Ошибка при получении тем и вопросов для выбранного мероприятия.", nil)
		log.Printf("TopicsHandler: Error during topics retrieval: %v", err)
		return handlers.EndConversation()
	}

	// Format and display topics
	formattedTopics := formatters.FormatTopicListForUsers(topics, event.Name, event.Type)
	h.messageSenderService.ReplyMarkdown(b, msg, formattedTopics, nil)

	return handlers.EndConversation()
}

// 3. handleCancel handles the /cancel command
func (h *topicsHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if there's an ongoing operation to cancel
	if cancelFunc, ok := h.userStore.Get(ctx.EffectiveUser.Id, topicsUserStoreKeyCancelFunc); ok {
		// Call the cancel function to stop any ongoing API calls
		if cf, ok := cancelFunc.(context.CancelFunc); ok {
			cf()
			h.messageSenderService.Reply(b, msg, "Операция просмотра тем отменена.", nil)
		}
	} else {
		h.messageSenderService.Reply(b, msg, "Операция просмотра тем отменена.", nil)
	}

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}
