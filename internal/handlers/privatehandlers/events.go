package privatehandlers

import (
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/utils"
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

type eventsHandler struct {
	eventRepository *repositories.EventRepository
	config          *config.Config
}

func NewEventsHandler(
	eventRepository *repositories.EventRepository,
	config *config.Config,
) ext.Handler {
	h := &eventsHandler{
		eventRepository: eventRepository,
		config:          config,
	}

	return handlers.NewCommand(constants.EventsCommand, h.handleCommand)
}

func (h *eventsHandler) handleCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage

	// Only proceed if this is a private chat
	if !utils.CheckPrivateChatType(b, ctx) {
		return nil
	}

	// Check if user is a club member
	if !utils.CheckClubMemberPermissions(b, msg, h.config, constants.EventsCommand) {
		return nil
	}

	// Get actual events to show
	events, err := h.eventRepository.GetLastActualEvents(10) // Fetch last 10 actual events
	if err != nil {
		utils.SendLoggedReply(b, msg, "Ошибка при получении списка событий.", err)
		return nil
	}

	if len(events) == 0 {
		utils.SendLoggedReply(b, msg, "На данный момент нет актуальных событий.", nil)
		return nil
	}

	// Format and display event list
	formattedEvents := utils.FormatEventListForUsersWithoutIds(
		events,
		"📋 Список ближайших событий",
	)
	formattedEvents += fmt.Sprintf("\nИспользуй команду /%s, если хочешь предложить тему или вопросы к этим событиям.", constants.TopicAddCommand)
	formattedEvents += "\n\nА вот ссылка на [клубный календарь](https://itbeard.com/s/evo-calendar), который можно добавить к себе и удобно следить всеми событиями клуба."
	utils.SendLoggedMarkdownReply(b, msg, formattedEvents, nil)

	return nil
}
