package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChatIdToFullChatId_RegularChatIdPositive(t *testing.T) {
	chatId := int64(123456789)
	expected := int64(-1000000000000 - 123456789)

	result := ChatIdToFullChatId(chatId)
	assert.Equal(t, expected, result, "chat ID conversion should be correct")
}
