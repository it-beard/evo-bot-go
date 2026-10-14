package privatehandlers

import (
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/formatters"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"
	"fmt"
	"log"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

type eventsHandler struct {
	config               *config.Config
	eventRepository      *repositories.EventRepository
	messageSenderService *services.MessageSenderService
	permissionsService   *services.PermissionsService
}

func NewEventsHandler(
	config *config.Config,
	eventRepository *repositories.EventRepository,
	messageSenderService *services.MessageSenderService,
	permissionsService *services.PermissionsService,
) ext.Handler {
	h := &eventsHandler{
		config:               config,
		eventRepository:      eventRepository,
		messageSenderService: messageSenderService,
		permissionsService:   permissionsService,
	}

	return handlers.NewCommand(constants.EventsCommand, h.handleCommand)
}

func (h *eventsHandler) handleCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Only proceed if this is a private chat
	if !h.permissionsService.CheckPrivateChatType(msg) {
		return nil
	}

	// Check if user is a club member
	if !h.permissionsService.CheckClubMemberPermissions(msg, constants.EventsCommand) {
		return nil
	}

	// Get upcoming events to show
	events, err := h.eventRepository.GetUpcomingEvents(10) // Fetch the 10 nearest upcoming events
	if err != nil {
		h.messageSenderService.Reply(msg, "Ошибка при получении списка мероприятий.", nil)
		log.Printf("%s: Error during events retrieval: %v", utils.GetCurrentTypeName(), err)
		return nil
	}

	if len(events) == 0 {
		h.messageSenderService.Reply(msg, "На данный момент нет актуальных мероприятий.", nil)
		return nil
	}

	// Format and display event list
	formattedEvents := formatters.FormatEventListForEventsView(
		events,
		"📋 Список ближайших мероприятий",
	)
	formattedEvents += fmt.Sprintf("\nДобавить темы и вопросы /%s. ", constants.TopicAddCommand)
	formattedEvents += fmt.Sprintf("Просмотреть темы и вопросы /%s. ", constants.TopicsCommand)
	formattedEvents += "Больше информации о мероприятиях смотри в [клубном календаре](https://itbeard.com/s/evo-calendar)."
	h.messageSenderService.ReplyMarkdown(msg, formattedEvents, nil)

	return nil
}
