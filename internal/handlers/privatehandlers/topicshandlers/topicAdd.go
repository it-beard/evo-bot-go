package topicshandlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

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
	topicAddStateSelectEvent = "topic_add_select_event"
	topicAddStateEnterTopic  = "topic_add_enter_topic"

	// UserStore keys
	topicAddUserStoreKeySelectedEventID   = "topic_add_selected_event_id"
	topicAddUserStoreKeySelectedEventName = "topic_add_selected_event_name"
	topicAddUserStoreKeyCancelFunc        = "topic_add_cancel_func"
)

type topicAddHandler struct {
	topicRepository      *repositories.TopicRepository
	eventRepository      *repositories.EventRepository
	messageSenderService services.MessageSenderService
	config               *config.Config
	userStore            *utils.UserDataStore
}

func NewTopicAddHandler(
	topicRepository *repositories.TopicRepository,
	eventRepository *repositories.EventRepository,
	messageSenderService services.MessageSenderService,
	config *config.Config,
) ext.Handler {
	h := &topicAddHandler{
		topicRepository:      topicRepository,
		eventRepository:      eventRepository,
		messageSenderService: messageSenderService,
		config:               config,
		userStore:            utils.NewUserDataStore(),
	}

	return handlers.NewConversation(
		[]ext.Handler{
			handlers.NewCommand(constants.TopicAddCommand, h.startTopicAdd),
		},
		map[string][]ext.Handler{
			topicAddStateSelectEvent: {
				handlers.NewMessage(message.All, h.handleEventSelection),
			},
			topicAddStateEnterTopic: {
				handlers.NewMessage(message.All, h.handleTopicEntry),
			},
		},
		&handlers.ConversationOpts{
			Exits: []ext.Handler{handlers.NewCommand(constants.CancelCommand, h.handleCancel)},
		},
	)
}

// 1. startTopicAdd is the entry point handler for adding a topic
func (h *topicAddHandler) startTopicAdd(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Only proceed if this is a private chat
	if !utils.CheckPrivateChatType(b, ctx) {
		return handlers.EndConversation()
	}

	// Check if user is a club member
	if !utils.CheckClubMemberPermissions(b, msg, h.config, constants.TopicAddCommand) {
		return handlers.EndConversation()
	}

	// Get last actual events to show for selection
	events, err := h.eventRepository.GetLastActualEvents(10)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Ошибка при получении списка событий.", err)
		return handlers.EndConversation()
	}

	if len(events) == 0 {
		utils.SendLoggedReply(b, msg, "Нет доступных событий для добавления тем.", nil)
		return handlers.EndConversation()
	}

	// Format and display event list for selection
	formattedEvents := utils.FormatEventListForUsers(
		events,
		fmt.Sprintf("Выбери ID события, к которому ты хочешь закинуть темы или вопросы, либо жми /%s для отмены диалога", constants.CancelCommand),
	)

	utils.SendLoggedMarkdownReply(b, msg, formattedEvents, nil)

	return handlers.NextConversationState(topicAddStateSelectEvent)
}

// 2. handleEventSelection processes the user's event selection
func (h *topicAddHandler) handleEventSelection(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	userInput := strings.TrimSpace(msg.Text)

	// Check if the input is a valid event ID
	eventID, err := strconv.Atoi(userInput)
	if err != nil {
		utils.SendLoggedReply(
			b,
			msg,
			fmt.Sprintf("Пожалуйста, отправь корректный ID события или жми /%s для отмены.", constants.CancelCommand),
			nil,
		)
		return nil // Stay in the same state
	}

	// Get the event information
	event, err := h.eventRepository.GetEventByID(eventID)
	if err != nil {
		utils.SendLoggedReply(
			b,
			msg,
			fmt.Sprintf("Не удалось найти событие с ID %d. Пожалуйста, проверь ID.", eventID),
			err,
		)
		return nil // Stay in the same state
	}

	// Store the selected event ID for later use when creating a new topic
	h.userStore.Set(ctx.EffectiveUser.Id, topicAddUserStoreKeySelectedEventID, eventID)
	h.userStore.Set(ctx.EffectiveUser.Id, topicAddUserStoreKeySelectedEventName, event.Name)

	// Prompt user to enter a topic
	utils.SendLoggedMarkdownReply(
		b,
		msg,
		fmt.Sprintf("Отправь мне темы или вопросы к событию *%s*, либо используй /%s для отмены диалога.", event.Name, constants.CancelCommand),
		nil,
	)

	return handlers.NextConversationState(topicAddStateEnterTopic)
}

// 3. handleTopicEntry handles the user's topic input
func (h *topicAddHandler) handleTopicEntry(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	topicText := strings.TrimSpace(msg.Text)

	if topicText == "" {
		utils.SendLoggedReply(
			b,
			msg,
			"Тема не может быть пустой. Пожалуйста, введи текст темы или /cancel для отмены.",
			nil,
		)
		return nil // Stay in the same state
	}

	// Get the selected event ID from user store
	eventIDInterface, ok := h.userStore.Get(ctx.EffectiveUser.Id, topicAddUserStoreKeySelectedEventID)
	if !ok {
		utils.SendLoggedReply(
			b,
			msg,
			"Произошла ошибка: не найден выбранное событие. Пожалуйста, начни заново.",
			nil,
		)
		return handlers.EndConversation()
	}

	eventID := eventIDInterface.(int)
	userNickname := "не указано"
	if ctx.EffectiveUser.Username != "" {
		userNickname = ctx.EffectiveUser.Username
	}

	// Create the new topic
	_, err := h.topicRepository.CreateTopic(topicText, userNickname, eventID)
	if err != nil {
		utils.SendLoggedReply(b, msg, "Ой! Ошибка записи в базу данных...", err)
		return handlers.EndConversation()
	}

	// Send notification to admin about new topic
	eventName, _ := h.userStore.Get(ctx.EffectiveUser.Id, topicAddUserStoreKeySelectedEventName)
	adminChatID := h.config.AdminUserID

	adminMsg := fmt.Sprintf(
		"🔔 *Новая тема добавлена*\n\n"+
			"_Событие:_ %s\n"+
			"_Автор:_ @%s\n"+
			"_Топик:_ %s",
		eventName,
		userNickname,
		topicText,
	)

	_, err = h.messageSenderService.SendMessageToUser(adminChatID, adminMsg, nil)
	if err != nil {
		// Just log the error, don't interrupt the user flow
		fmt.Printf("Error sending admin notification about new topic: %v\n", err)
	}

	utils.SendLoggedReply(b, msg,
		fmt.Sprintf("Добавлено! \nИспользуй команду /%s для просмотра всех тем и вопросов к событию.", constants.TopicsCommand),
		nil,
	)

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}

// 4. handleCancel handles the /cancel command
func (h *topicAddHandler) handleCancel(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Check if there's an ongoing operation to cancel
	if cancelFunc, ok := h.userStore.Get(ctx.EffectiveUser.Id, topicAddUserStoreKeyCancelFunc); ok {
		// Call the cancel function to stop any ongoing API calls
		if cf, ok := cancelFunc.(context.CancelFunc); ok {
			cf()
			utils.SendLoggedReply(b, msg, "Операция добавления темы отменена.", nil)
		}
	} else {
		utils.SendLoggedReply(b, msg, "Операция добавления темы отменена.", nil)
	}

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}
