package formatters

import (
	"html"
	"strings"
	"testing"

	"evo-bot-go/internal/constants"

	"github.com/stretchr/testify/assert"
)

func TestConversationTexts(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{name: "CancelHint", text: CancelHint, expected: "или /cancel для отмены"},
		{name: "UseButtonsText", text: UseButtonsText, expected: "Пожалуйста, нажми на одну из кнопок выше или /cancel для отмены."},
		{
			name:     "InvalidEventIDText",
			text:     InvalidEventIDText,
			expected: "Неверный ID. Пожалуйста, нажми на ID мероприятия из списка выше или /cancel для отмены.",
		},
		{
			name:     "EventNotFoundText",
			text:     EventNotFoundText(42),
			expected: "Мероприятие с ID 42 не найдено. Пожалуйста, нажми на ID мероприятия из списка выше или /cancel для отмены.",
		},
		{name: "EventListErrorText", text: EventListErrorText, expected: "Произошла ошибка при получении списка мероприятий."},
		{
			name:     "LostEventText",
			text:     LostEventText(constants.EventEditCommand),
			expected: "Произошла ошибка: не удалось найти выбранное мероприятие. Пожалуйста, начни заново с /eventEdit.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.text)
		})
	}
}

func TestConversationTexts_CancelHintUsesCancelCommand(t *testing.T) {
	assert.Contains(t, CancelHint, "/"+constants.CancelCommand+" ")
}

func TestConversationTexts_TryAgainTextsEndWithCancelHint(t *testing.T) {
	for name, text := range map[string]string{
		"UseButtonsText":     UseButtonsText,
		"InvalidEventIDText": InvalidEventIDText,
		"EventNotFoundText":  EventNotFoundText(1),
	} {
		assert.True(t, strings.HasSuffix(text, " "+CancelHint+"."), "%s must end with the cancel hint: %q", name, text)
	}
}

func TestConversationTexts_ErrorTextsStartWithErrorPrefix(t *testing.T) {
	for name, text := range map[string]string{
		"EventListErrorText": EventListErrorText,
		"LostEventText":      LostEventText(constants.TopicAddCommand),
	} {
		assert.True(t, strings.HasPrefix(text, "Произошла ошибка"), "%s: %q", name, text)
	}
}

// The texts are sent with any parse mode, so they must not contain anything HTML or Markdown would interpret
func TestConversationTexts_ContainNoMarkup(t *testing.T) {
	for name, text := range map[string]string{
		"CancelHint":         CancelHint,
		"UseButtonsText":     UseButtonsText,
		"InvalidEventIDText": InvalidEventIDText,
		"EventNotFoundText":  EventNotFoundText(7),
		"EventListErrorText": EventListErrorText,
		"LostEventText":      LostEventText(constants.EventStartCommand),
	} {
		assert.Equal(t, html.EscapeString(text), text, "%s must not need HTML escaping", name)
		assert.False(t, strings.ContainsAny(text, "*_`["), "%s must not contain Markdown characters: %q", name, text)
	}
}
