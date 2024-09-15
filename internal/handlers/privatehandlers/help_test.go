package privatehandlers

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/mock"
)

// MockBot is a mock implementation of gotgbot.Bot
type MockBot struct {
	mock.Mock
}

// MockMessage is a mock implementation of gotgbot.Message
type MockMessage struct {
	mock.Mock
}

func (m *MockMessage) Reply(b *gotgbot.Bot, text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
	args := m.Called(b, text, opts)
	return args.Get(0).(*gotgbot.Message), args.Error(1)
}

func TestNewHelpHandler(t *testing.T) {
	handler := NewHelpHandler()
	if handler == nil {
		t.Error("NewHelpHandler returned nil")
	}
}

func TestHelpHandler_Name(t *testing.T) {
	handler := NewHelpHandler()
	if handler.Name() != "help_handler" {
		t.Errorf("Expected name to be 'help_handler', got '%s'", handler.Name())
	}
}

func TestHelpHandler_CheckUpdate(t *testing.T) {
	handler := NewHelpHandler()
	bot := &gotgbot.Bot{}

	tests := []struct {
		name     string
		message  *gotgbot.Message
		expected bool
	}{
		{
			name: "Valid help command in private chat",
			message: &gotgbot.Message{
				Text: "/help",
				Chat: gotgbot.Chat{Type: "private"},
			},
			expected: true,
		},
		{
			name: "Help command in non-private chat",
			message: &gotgbot.Message{
				Text: "/help",
				Chat: gotgbot.Chat{Type: "group"},
			},
			expected: false,
		},
		{
			name: "Non-help command in private chat",
			message: &gotgbot.Message{
				Text: "/start",
				Chat: gotgbot.Chat{Type: "private"},
			},
			expected: false,
		},
		{
			name:     "Nil message",
			message:  nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &ext.Context{
				EffectiveMessage: tt.message,
			}
			result := handler.CheckUpdate(bot, ctx)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
