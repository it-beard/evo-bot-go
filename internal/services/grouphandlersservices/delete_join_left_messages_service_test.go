package grouphandlersservices

import (
	"testing"

	"evo-bot-go/internal/testutils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func joinMessage() *gotgbot.Message {
	msg := newGroupMsg(testUserID, 0, "")
	msg.NewChatMembers = []gotgbot.User{*testutils.NewUser(testUserID)}
	return msg
}

func leftMessage() *gotgbot.Message {
	msg := newGroupMsg(testUserID, 0, "")
	msg.LeftChatMember = testutils.NewUser(testUserID)
	return msg
}

func TestDeleteJoinLeftMessagesService_IsMessageShouldBeDeleted(t *testing.T) {
	s := NewDeleteJoinLeftMessagesService()

	assert.True(t, s.IsMessageShouldBeDeleted(joinMessage()))
	assert.True(t, s.IsMessageShouldBeDeleted(leftMessage()))
	assert.False(t, s.IsMessageShouldBeDeleted(newGroupMsg(testUserID, 0, "hello")))
}

func TestDeleteJoinLeftMessagesService_DeleteJoinLeftMessages(t *testing.T) {
	tests := []struct {
		name string
		msg  *gotgbot.Message
	}{
		{"join message", joinMessage()},
		{"left message", leftMessage()},
		{"other message", newGroupMsg(testUserID, 0, "hello")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bot, client := testutils.NewTestBot()
			s := NewDeleteJoinLeftMessagesService()

			require.NoError(t, s.DeleteJoinLeftMessages(tt.msg, bot))

			calls := client.Calls()
			require.Len(t, calls, 1)
			assert.Equal(t, "deleteMessage", calls[0].Method)
			assert.Equal(t, testChatID, calls[0].Int64("chat_id"))
			assert.Equal(t, tt.msg.MessageId, calls[0].Int64("message_id"))
		})
	}
}

func TestDeleteJoinLeftMessagesService_DeleteJoinLeftMessages_DeleteError(t *testing.T) {
	bot, client := testutils.NewTestBot()
	client.OnError("deleteMessage", "Bad Request: message can't be deleted")
	s := NewDeleteJoinLeftMessagesService()

	err := s.DeleteJoinLeftMessages(joinMessage(), bot)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Error deleting message")
	assert.Contains(t, err.Error(), "message can't be deleted")
}
