package eventhandlers

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"evo-bot-go/internal/buttons"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/formatters"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// Steps shared by the event admin commands, so all of them look and behave the same way

const adminEventListTitle = "📋 Последние мероприятия"

// Shown whenever an admin saves an actual event whose start date has already passed
var pastStartDateWarning = fmt.Sprintf(
	"⚠️ Дата начала уже прошла, поэтому мероприятие не показывается в /%s. Если это ошибка, исправь дату через /%s.",
	constants.EventsCommand, constants.EventEditCommand,
)

// pastStartDateWarningFor returns pastStartDateWarning on a new line when startedAt is not in the future
func pastStartDateWarningFor(startedAt time.Time) string {
	if startedAt.After(time.Now()) {
		return ""
	}
	return "\n" + pastStartDateWarning
}

// sendAdminEventList replies with the latest events to pick one by ID.
// It returns false when there is nothing to pick, after telling the admin why.
func sendAdminEventList(
	eventRepository *repositories.EventRepository,
	sender *services.MessageSenderService,
	msg *gotgbot.Message,
	actionDescription string,
	cancelCallback string,
) (*gotgbot.Message, bool) {
	events, err := eventRepository.GetLastEvents(constants.EventEditGetLastLimit)
	if err != nil {
		sender.Reply(msg, formatters.EventListErrorText, nil)
		log.Printf("%s: Error during event retrieval: %v", utils.GetCurrentTypeName(), err)
		return nil, false
	}

	if len(events) == 0 {
		sender.Reply(msg, fmt.Sprintf(
			"Пока нет ни одного мероприятия. Создать новое можно командой /%s.", constants.EventSetupCommand,
		), nil)
		return nil, false
	}

	sentMsg, _ := sender.ReplyHtmlWithReturnMessage(
		msg,
		formatters.FormatEventListForAdmin(events, adminEventListTitle, actionDescription),
		&gotgbot.SendMessageOpts{
			ReplyMarkup: buttons.CancelButton(cancelCallback),
		},
	)
	return sentMsg, true
}

// findSelectedEvent loads the event whose ID the admin sent. On invalid input the admin is asked again
// and false is returned, so the conversation should stay in the same state.
func findSelectedEvent(
	eventRepository *repositories.EventRepository,
	sender *services.MessageSenderService,
	msg *gotgbot.Message,
) (*repositories.Event, bool) {
	eventIDStr := strings.TrimSpace(strings.Replace(msg.Text, "/", "", 1))
	eventID, err := strconv.Atoi(eventIDStr)
	if err != nil {
		sender.Reply(msg, formatters.InvalidEventIDText, nil)
		return nil, false
	}

	event, err := eventRepository.GetEventByID(eventID)
	if err != nil {
		log.Printf("%s: Error during event retrieval for ID %d: %v", utils.GetCurrentTypeName(), eventID, err)
		sender.Reply(msg, formatters.EventNotFoundText(eventID), nil)
		return nil, false
	}

	return event, true
}

// parseEventDate parses a start date typed in formatters.EventDateInputLayout (UTC).
// On invalid input the admin is asked again and false is returned.
func parseEventDate(sender *services.MessageSenderService, msg *gotgbot.Message) (time.Time, bool) {
	startedAt, err := time.Parse(formatters.EventDateInputLayout, strings.TrimSpace(msg.Text))
	if err != nil {
		sender.ReplyHtml(msg, fmt.Sprintf(
			"Неверный формат даты. Пожалуйста, введи дату и время в формате %s %s.",
			formatters.EventDateInputHint, formatters.CancelHint,
		), nil)
		return time.Time{}, false
	}
	return startedAt, true
}

// parseEventType accepts the number of a type listed by formatters.FormatEventTypeOptions or the type
// itself (e.g. "meetup"). On invalid input the admin is asked again and false is returned.
func parseEventType(sender *services.MessageSenderService, msg *gotgbot.Message) (constants.EventType, bool) {
	input := strings.TrimSpace(strings.Replace(msg.Text, "/", "", 1))

	if index, err := strconv.Atoi(input); err == nil && index >= 1 && index <= len(constants.AllEventTypes) {
		return constants.AllEventTypes[index-1], true
	}
	for _, eventType := range constants.AllEventTypes {
		if input == string(eventType) {
			return eventType, true
		}
	}

	sender.Reply(msg, fmt.Sprintf(
		"Неверный тип мероприятия. Пожалуйста, нажми на номер типа от 1 до %d %s.",
		len(constants.AllEventTypes), formatters.CancelHint,
	), nil)
	return "", false
}
