package grouphandlersservices

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"evo-bot-go/internal/testutils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCleanClosedThreadsService(d *testDeps) *CleanClosedThreadsService {
	return NewCleanClosedThreadsService(d.cfg, d.sender, d.groupTopicRepo)
}

func TestCleanClosedThreadsService_IsTopicShouldBeCleaned(t *testing.T) {
	const adminID, creatorID int64 = 501, 502

	tests := []struct {
		name            string
		msg             *gotgbot.Message
		expected        bool
		checksAdminship bool
	}{
		{"message in open topic", newGroupMsg(testUserID, testOpenTopicID, "hi"), false, false},
		{"message in general chat", newGroupMsg(testUserID, 0, "hi"), false, false},
		{"reply to a post in closed topic is left to replies service",
			asReplyTo(newGroupMsg(testUserID, testClosedTopicID, "hi"), 777), false, false},
		{"regular user posting in closed topic", newGroupMsg(testUserID, testClosedTopicID, "hi"), true, true},
		{"regular user posting in second closed topic", newGroupMsg(testUserID, testSecondClosedTopicID, "hi"), true, true},
		{"regular user replying to closed topic root",
			asReplyTo(newGroupMsg(testUserID, testClosedTopicID, "hi"), testClosedTopicID), true, true},
		{"administrator posting in closed topic", newGroupMsg(adminID, testClosedTopicID, "hi"), false, true},
		{"creator posting in closed topic", newGroupMsg(creatorID, testClosedTopicID, "hi"), false, true},
		{"anonymous admin posting in closed topic", func() *gotgbot.Message {
			m := newGroupMsg(0, testClosedTopicID, "hi")
			m.From = groupAnonymousBot()
			return m
		}(), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newTestDeps(t)
			d.client.SetMemberStatus(adminID, "administrator")
			d.client.SetMemberStatus(creatorID, "creator")
			s := newCleanClosedThreadsService(d)

			assert.Equal(t, tt.expected, s.IsTopicShouldBeCleaned(tt.msg, d.bot))

			memberChecks := d.client.CallsTo("getChatMember")
			if !tt.checksAdminship {
				assert.Empty(t, memberChecks)
				return
			}
			require.Len(t, memberChecks, 1)
			assert.Equal(t, testChatID, memberChecks[0].Int64("chat_id"))
			assert.Equal(t, tt.msg.From.Id, memberChecks[0].Int64("user_id"))
		})
	}
}

func TestCleanClosedThreadsService_CleanClosedThreads_DeletesAndNotifiesUser(t *testing.T) {
	d := newTestDeps(t)
	s := newCleanClosedThreadsService(d)
	msg := newGroupMsg(testUserID, testClosedTopicID, "my hello world")
	msg.Entities = []gotgbot.MessageEntity{{Type: "bold", Offset: 3, Length: 5}}

	expectGroupTopic(d.mock, testClosedTopicID, "Announcements")
	expectGroupTopic(d.mock, testForwardingTopicID, "Main chat")

	require.NoError(t, s.CleanClosedThreads(msg, d.bot))

	calls := d.client.Calls()
	require.Len(t, calls, 3)

	assert.Equal(t, "deleteMessage", calls[0].Method)
	assert.Equal(t, testChatID, calls[0].Int64("chat_id"))
	assert.Equal(t, msg.MessageId, calls[0].Int64("message_id"))

	notification := calls[1]
	assert.Equal(t, "sendMessage", notification.Method)
	assert.Equal(t, testUserID, notification.Int64("chat_id"))
	assert.Equal(t, "HTML", notification.Params["parse_mode"])
	assert.Contains(t, notification.Params["text"],
		fmt.Sprintf(`<a href="https://telegram.me/c/%s/%d">"Announcements"</a>`, testChatLinkID, testClosedTopicID))
	assert.Contains(t, notification.Params["text"], `"<i>Main chat</i>"`)

	cp := calls[2]
	assert.Equal(t, "sendMessage", cp.Method)
	assert.Equal(t, testUserID, cp.Int64("chat_id"))
	assert.Equal(t, "my hello world", cp.Params["text"])
	assert.Empty(t, cp.Params["parse_mode"])
	assert.Equal(t, msg.Entities, decodeEntities(t, cp, "entities"))
}

func TestCleanClosedThreadsService_CleanClosedThreads_UsesDefaultNamesWhenTopicsAreUnknown(t *testing.T) {
	d := newTestDeps(t)
	s := newCleanClosedThreadsService(d)
	msg := newGroupMsg(testUserID, testClosedTopicID, "hello")

	expectGroupTopicNotFound(d.mock, testClosedTopicID)
	d.mock.ExpectQuery(sqlSelectGroupTopicByTopic).WithArgs(int64(testForwardingTopicID)).WillReturnError(errDB)

	require.NoError(t, s.CleanClosedThreads(msg, d.bot))

	texts := d.client.SentTexts()
	require.Len(t, texts, 2)
	assert.Contains(t, texts[0], `"Topic name"</a>`)
	assert.Contains(t, texts[0], `"<i>Main conversation topic name</i>"`)
	assert.Equal(t, "hello", texts[1])
}

func TestCleanClosedThreadsService_CleanClosedThreads_DeleteError(t *testing.T) {
	d := newTestDeps(t)
	s := newCleanClosedThreadsService(d)
	d.client.OnError("deleteMessage", "Bad Request: message can't be deleted")

	err := s.CleanClosedThreads(newGroupMsg(testUserID, testClosedTopicID, "hello"), d.bot)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete message")
	assert.Empty(t, d.client.CallsTo("sendMessage"))
}

func TestCleanClosedThreadsService_CleanClosedThreads_NotificationError(t *testing.T) {
	d := newTestDeps(t)
	s := newCleanClosedThreadsService(d)
	d.client.OnError("sendMessage", "Forbidden: bot was blocked by the user")

	expectGroupTopic(d.mock, testClosedTopicID, "Announcements")
	expectGroupTopic(d.mock, testForwardingTopicID, "Main chat")

	err := s.CleanClosedThreads(newGroupMsg(testUserID, testClosedTopicID, "hello"), d.bot)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send notification message")
	assert.Len(t, d.client.CallsTo("sendMessage"), 1, "copy must not be sent when notification fails")
}

func TestCleanClosedThreadsService_CleanClosedThreads_CopyError(t *testing.T) {
	d := newTestDeps(t)
	s := newCleanClosedThreadsService(d)
	// The HTML notification succeeds, the plain copy fails
	d.client.On("sendMessage", func(call testutils.APICall) (json.RawMessage, error) {
		if call.Params["parse_mode"] == "HTML" {
			return json.Marshal(gotgbot.Message{MessageId: 1, Chat: gotgbot.Chat{Id: testUserID, Type: "private"}})
		}
		return nil, errors.New("copy failed")
	})

	expectGroupTopic(d.mock, testClosedTopicID, "Announcements")
	expectGroupTopic(d.mock, testForwardingTopicID, "Main chat")

	err := s.CleanClosedThreads(newGroupMsg(testUserID, testClosedTopicID, "hello"), d.bot)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send copy of the original message")
	assert.Len(t, d.client.CallsTo("sendMessage"), 2)
}
