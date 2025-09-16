package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChatIdToFullChatId(t *testing.T) {
	tests := []struct {
		name     string
		chatId   int64
		expected int64
	}{
		{
			name:     "Regular positive chat ID",
			chatId:   123456789,
			expected: -1000000000000 - 123456789,
		},
		{
			name:     "Negative chat ID",
			chatId:   -987654321,
			expected: -1000000000000 - (-987654321), // = -1000000000000 + 987654321
		},
		{
			name:     "Zero chat ID",
			chatId:   0,
			expected: -1000000000000,
		},
		{
			name:     "Small positive chat ID",
			chatId:   1,
			expected: -1000000000001,
		},
		{
			name:     "Small negative chat ID",
			chatId:   -1,
			expected: -999999999999,
		},
		{
			name:     "Large positive chat ID",
			chatId:   999999999999,
			expected: -1999999999999, // -1000000000000 - 999999999999
		},
		{
			name:     "Large negative chat ID",
			chatId:   -999999999999,
			expected: -1000000000000 + 999999999999, // = -1000000000001
		},
		{
			name:     "Large boundary test - safe value",
			chatId:   99999999999, // Large but safe value
			expected: -1099999999999, // -1000000000000 - 99999999999
		},
		{
			name:     "Large standard Telegram chat ID",
			chatId:   1234567890,
			expected: -1001234567890,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ChatIdToFullChatId(tt.chatId)
			assert.Equal(t, tt.expected, result, "chat ID conversion should be correct for %s", tt.name)
		})
	}
}
