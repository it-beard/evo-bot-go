package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatIdToFullChatId_RegularChatIdPositive(t *testing.T) {
	chatId := int64(123456789)
	expected := int64(-1000000000000 - 123456789)

	result := ChatIdToFullChatId(chatId)
	assert.Equal(t, expected, result, "chat ID conversion should be correct")
}

// TestGetTopicName_OffTopic tests the special case for the default topic (id 0)
func TestGetTopicName_OffTopic(t *testing.T) {
	topicName, err := GetTopicName(0)

	require.NoError(t, err)
	assert.Equal(t, "Оффтопчик", topicName, "should return 'Оффтопчик' for topic ID 0")
}
