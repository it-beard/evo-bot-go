package formatters

import (
	"fmt"

	"evo-bot-go/internal/constants"
)

// Texts shared by the step-by-step commands, so the same situation is always phrased the same way.
// They contain no markup, so they can be sent with any parse mode.

// CancelHint ends every "try again" message
var CancelHint = fmt.Sprintf("или /%s для отмены", constants.CancelCommand)

// UseButtonsText is sent when a text arrives while the bot waits for a button press
var UseButtonsText = fmt.Sprintf("Пожалуйста, нажми на одну из кнопок выше %s.", CancelHint)

// InvalidEventIDText is sent when the input is not a number while the bot waits for an event ID
var InvalidEventIDText = fmt.Sprintf("Неверный ID. Пожалуйста, нажми на ID мероприятия из списка выше %s.", CancelHint)

// EventNotFoundText is sent when no event has the entered ID
func EventNotFoundText(eventID int) string {
	return fmt.Sprintf("Мероприятие с ID %d не найдено. Пожалуйста, нажми на ID мероприятия из списка выше %s.", eventID, CancelHint)
}

// EventListErrorText is sent when the list of events cannot be loaded
const EventListErrorText = "Произошла ошибка при получении списка мероприятий."

// LostEventText is sent when the selected event is missing from the conversation data
func LostEventText(command string) string {
	return fmt.Sprintf("Произошла ошибка: не удалось найти выбранное мероприятие. Пожалуйста, начни заново с /%s.", command)
}
