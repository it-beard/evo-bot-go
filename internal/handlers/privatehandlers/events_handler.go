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

const clubCalendarURL = "https://itbeard.com/s/evo-calendar"

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
		h.messageSenderService.Reply(msg, formatters.EventListErrorText, nil)
		log.Printf("%s: Error during events retrieval: %v", utils.GetCurrentTypeName(), err)
		return nil
	}

	calendarLink := fmt.Sprintf("<a href=\"%s\">клубном календаре</a>", clubCalendarURL)

	if len(events) == 0 {
		h.messageSenderService.ReplyHtml(msg, fmt.Sprintf(
			"Ближайших мероприятий пока нет. Следи за анонсами в %s.", calendarLink,
		), nil)
		return nil
	}

	// Format and display event list
	formattedEvents := formatters.FormatEventListForEventsView(events, "📋 Ближайшие мероприятия")
	formattedEvents += fmt.Sprintf(
		"\nДобавить темы и вопросы — /%s, посмотреть их — /%s.\nБольше информации о мероприятиях — в %s.",
		constants.TopicAddCommand, constants.TopicsCommand, calendarLink,
	)
	h.messageSenderService.ReplyHtml(msg, formattedEvents, nil)

	return nil
}
