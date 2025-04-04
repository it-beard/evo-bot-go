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
	config               *config.Config
	topicRepository      *repositories.TopicRepository
	eventRepository      *repositories.EventRepository
	messageSenderService services.MessageSenderService
	userStore            *utils.UserDataStore
}

func NewTopicAddHandler(
	config *config.Config,
	topicRepository *repositories.TopicRepository,
	eventRepository *repositories.EventRepository,
	messageSenderService services.MessageSenderService,
) ext.Handler {
	h := &topicAddHandler{
		config:               config,
		topicRepository:      topicRepository,
		eventRepository:      eventRepository,
		messageSenderService: messageSenderService,
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
		h.messageSenderService.Reply(b, msg, "Ошибка при получении списка мероприятий.", nil)
		log.Printf("TopicAddHandler: Error during events retrieval: %v", err)
		return handlers.EndConversation()
	}

	if len(events) == 0 {
		h.messageSenderService.Reply(b, msg, "Нет доступных мероприятий для добавления тем и вопросов.", nil)
		return handlers.EndConversation()
	}

	// Format and display event list for selection
	formattedEvents := utils.FormatEventListForUsers(
		events,
		fmt.Sprintf("Выбери ID мероприятия, к которому ты хочешь закинуть темы или вопросы, либо жми /%s для отмены диалога", constants.CancelCommand),
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
		h.messageSenderService.Reply(
			b,
			msg,
			fmt.Sprintf("Пожалуйста, отправь корректный ID мероприятия или жми /%s для отмены.", constants.CancelCommand),
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
		log.Printf("TopicAddHandler: Error during event retrieval: %v", err)
		return nil // Stay in the same state
	}

	// Store the selected event ID for later use when creating a new topic
	h.userStore.Set(ctx.EffectiveUser.Id, topicAddUserStoreKeySelectedEventID, eventID)
	h.userStore.Set(ctx.EffectiveUser.Id, topicAddUserStoreKeySelectedEventName, event.Name)

	// Prompt user to enter a topic
	utils.SendLoggedMarkdownReply(
		b,
		msg,
		fmt.Sprintf("Отправь мне темы и вопросы к мероприятию *%s*, либо используй /%s для отмены диалога.", event.Name, constants.CancelCommand),
		nil,
	)

	return handlers.NextConversationState(topicAddStateEnterTopic)
}

// 3. handleTopicEntry handles the user's topic input
func (h *topicAddHandler) handleTopicEntry(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	topicText := strings.TrimSpace(msg.Text)

	if topicText == "" {
		h.messageSenderService.Reply(
			b,
			msg,
			"Тема не может быть пустой. Пожалуйста, введи текст темы или /cancel для отмены.",
			nil,
		)
		log.Printf("TopicAddHandler: Empty topic text")
		return nil // Stay in the same state
	}

	// Get the selected event ID from user store
	eventIDInterface, ok := h.userStore.Get(ctx.EffectiveUser.Id, topicAddUserStoreKeySelectedEventID)
	if !ok {
		h.messageSenderService.Reply(
			b,
			msg,
			"Произошла ошибка: не найден выбранное мероприятие. Пожалуйста, начни заново.",
			nil,
		)
		log.Printf("TopicAddHandler: Event ID not found in user store")
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
		h.messageSenderService.Reply(b, msg, "Ой! Что-то пошло не так...", nil)
		log.Printf("TopicAddHandler: Error during topic creation in database: %v", err)
		return handlers.EndConversation()
	}

	// Send notification to admin about new topic
	eventName, _ := h.userStore.Get(ctx.EffectiveUser.Id, topicAddUserStoreKeySelectedEventName)
	adminChatID := h.config.AdminUserID

	adminMsg := fmt.Sprintf(
		"🔔 *Новая тема добавлена*\n\n"+
			"_Мероприятие:_ %s\n"+
			"_Автор:_ @%s\n"+
			"_Топик:_ %s",
		eventName,
		userNickname,
		topicText,
	)

	h.messageSenderService.Send(adminChatID, adminMsg, nil)

	h.messageSenderService.Reply(
		b,
		msg,
		fmt.Sprintf("Добавлено! \nИспользуй команду /%s для просмотра всех тем и вопросов к мероприятию.", constants.TopicsCommand),
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
			h.messageSenderService.Reply(b, msg, "Операция добавления темы отменена.", nil)
		}
	} else {
		h.messageSenderService.Reply(b, msg, "Операция добавления темы отменена.", nil)
	}

	// Clean up user data
	h.userStore.Clear(ctx.EffectiveUser.Id)

	return handlers.EndConversation()
}
