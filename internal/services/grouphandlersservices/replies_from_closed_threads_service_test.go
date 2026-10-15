package grouphandlersservices

import (
	"fmt"
	"testing"
	"unicode/utf16"

	"evo-bot-go/internal/testutils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testRepliedPostID int64 = 777

func newRepliesService(d *testDeps) *RepliesFromClosedThreadsService {
	return NewRepliesFromClosedThreadsService(d.cfg, d.sender, d.groupTopicRepo, d.saveUpdate)
}

// utf16Len returns the length Telegram uses for entity offsets/lengths
func utf16Len(s string) int64 {
	return int64(len(utf16.Encode([]rune(s))))
}

// expectedHeader builds the forwarded first line and its entities the way Telegram expects them
func expectedHeader(prefix, postfix string, repliedID int64) (string, []gotgbot.MessageEntity) {
	firstLine := prefix + " " + postfix
	return firstLine, []gotgbot.MessageEntity{
		{Type: "blockquote", Offset: 0, Length: utf16Len(firstLine)},
		{Type: "italic", Offset: 0, Length: utf16Len(firstLine)},
		{Type: "text_link", Offset: utf16Len(prefix) + 1, Length: utf16Len(postfix),
			Url: fmt.Sprintf("https://telegram.me/c/%s/%d", testChatLinkID, repliedID)},
	}
}

func closedTopicReply(text string) *gotgbot.Message {
	return asReplyTo(newGroupMsg(testUserID, testClosedTopicID, text), testRepliedPostID)
}

func TestRepliesFromClosedThreadsService_IsReplyShouldBeForwarded(t *testing.T) {
	anonymousReply := closedTopicReply("hi")
	anonymousReply.From = groupAnonymousBot()

	tests := []struct {
		name     string
		msg      *gotgbot.Message
		expected bool
	}{
		{"not a reply", newGroupMsg(testUserID, testClosedTopicID, "hi"), false},
		{"anonymous admin reply", anonymousReply, false},
		{"reply to a post in closed topic", closedTopicReply("hi"), true},
		{"reply in second closed topic",
			asReplyTo(newGroupMsg(testUserID, testSecondClosedTopicID, "hi"), testRepliedPostID), true},
		{"reply to closed topic root",
			asReplyTo(newGroupMsg(testUserID, testClosedTopicID, "hi"), testClosedTopicID), false},
		{"reply in open topic", asReplyTo(newGroupMsg(testUserID, testOpenTopicID, "hi"), testRepliedPostID), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newTestDeps(t)
			s := newRepliesService(d)

			assert.Equal(t, tt.expected, s.IsReplyShouldBeForwarded(tt.msg, d.bot))
			assert.Empty(t, d.client.Calls())
		})
	}
}

func TestRepliesFromClosedThreadsService_RepliesFromClosedThreads_TextReplyWithTopicName(t *testing.T) {
	d := newTestDeps(t)
	s := newRepliesService(d)
	msg := closedTopicReply("hello world")
	msg.Entities = []gotgbot.MessageEntity{
		{Type: "bold", Offset: 0, Length: 5},
		{Type: "italic", Offset: 3, Length: 100}, // out of bounds -> dropped
	}

	expectGroupTopic(d.mock, testClosedTopicID, "Анонсы")
	expectGroupMessageNotFound(d.mock, msg.MessageId)
	expectExistingUser(d.mock, 7, testUserID)
	expectGroupMessageInsert(d.mock, msg.MessageId, "<strong>hello</strong> world", testRepliedPostID, testUserID, testClosedTopicID)

	err := s.RepliesFromClosedThreads(msg, d.bot, ext.NewContext(d.bot, testutils.MessageUpdate(msg), nil))
	require.NoError(t, err)

	firstLine, headerEntities := expectedHeader(replyPrefix+" @user4242 на сообщение в канале", `"Анонсы"`, testRepliedPostID)

	calls := d.client.Calls()
	require.Len(t, calls, 2)

	forwarded := calls[0]
	assert.Equal(t, "sendMessage", forwarded.Method)
	assert.Equal(t, testChatID, forwarded.Int64("chat_id"))
	assert.Equal(t, int64(testForwardingTopicID), forwarded.Int64("message_thread_id"))
	assert.Equal(t, firstLine+"\nhello world", forwarded.Params["text"])
	expectedEntities := append([]gotgbot.MessageEntity{
		{Type: "bold", Offset: utf16Len(firstLine) + 1, Length: 5},
	}, headerEntities...)
	assert.Equal(t, expectedEntities, decodeEntities(t, forwarded, "entities"))

	deleted := calls[1]
	assert.Equal(t, "deleteMessage", deleted.Method)
	assert.Equal(t, testChatID, deleted.Int64("chat_id"))
	assert.Equal(t, msg.MessageId, deleted.Int64("message_id"))
}

func TestRepliesFromClosedThreadsService_RepliesFromClosedThreads_UnknownOrUnnamedTopic(t *testing.T) {
	tests := []struct {
		name      string
		topicName *string
	}{
		{"topic is not in database", nil},
		{"topic has empty name", new(string)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newTestDeps(t)
			s := newRepliesService(d)
			msg := closedTopicReply("plain")

			if tt.topicName == nil {
				expectGroupTopicNotFound(d.mock, testClosedTopicID)
			} else {
				expectGroupTopic(d.mock, testClosedTopicID, *tt.topicName)
			}
			expectGroupMessageNotFound(d.mock, msg.MessageId)
			expectExistingUser(d.mock, 7, testUserID)
			expectGroupMessageInsert(d.mock, msg.MessageId, "plain", testRepliedPostID, testUserID, testClosedTopicID)

			require.NoError(t, s.RepliesFromClosedThreads(msg, d.bot, ext.NewContext(d.bot, testutils.MessageUpdate(msg), nil)))

			firstLine, headerEntities := expectedHeader(replyPrefix+" @user4242 на", "сообщение", testRepliedPostID)
			forwarded, ok := d.client.LastCallTo("sendMessage")
			require.True(t, ok)
			assert.Equal(t, firstLine+"\nplain", forwarded.Params["text"])
			assert.Equal(t, headerEntities, decodeEntities(t, forwarded, "entities"))
			assert.Len(t, d.client.CallsTo("deleteMessage"), 1)
		})
	}
}

func TestRepliesFromClosedThreadsService_RepliesFromClosedThreads_PhotoWithCaption(t *testing.T) {
	d := newTestDeps(t)
	s := newRepliesService(d)
	msg := closedTopicReply("")
	msg.Caption = "photo caption"
	msg.CaptionEntities = []gotgbot.MessageEntity{
		{Type: "italic", Offset: 0, Length: 5},
		{Type: "bold", Offset: 2, Length: 50}, // out of bounds -> dropped
	}
	msg.Photo = []gotgbot.PhotoSize{{FileId: "small"}, {FileId: "large"}}

	expectGroupTopic(d.mock, testClosedTopicID, "Анонсы")
	expectGroupMessageNotFound(d.mock, msg.MessageId)
	expectExistingUser(d.mock, 7, testUserID)
	expectGroupMessageInsert(d.mock, msg.MessageId, "<em>photo</em> caption", testRepliedPostID, testUserID, testClosedTopicID)

	require.NoError(t, s.RepliesFromClosedThreads(msg, d.bot, ext.NewContext(d.bot, testutils.MessageUpdate(msg), nil)))

	firstLine, headerEntities := expectedHeader(replyPrefix+" @user4242 на сообщение в канале", `"Анонсы"`, testRepliedPostID)

	assert.Empty(t, d.client.CallsTo("sendMessage"))
	photo, ok := d.client.LastCallTo("sendPhoto")
	require.True(t, ok)
	assert.Equal(t, testChatID, photo.Int64("chat_id"))
	assert.Equal(t, int64(testForwardingTopicID), photo.Int64("message_thread_id"))
	assert.Equal(t, "large", photo.Params["photo"])
	assert.Equal(t, firstLine+"\nphoto caption", photo.Params["caption"])
	expectedEntities := append([]gotgbot.MessageEntity{
		{Type: "italic", Offset: utf16Len(firstLine) + 1, Length: 5},
	}, headerEntities...)
	assert.Equal(t, expectedEntities, decodeEntities(t, photo, "caption_entities"))
	assert.Len(t, d.client.CallsTo("deleteMessage"), 1)
}

func TestRepliesFromClosedThreadsService_RepliesFromClosedThreads_ForwardErrorSkipsSaving(t *testing.T) {
	d := newTestDeps(t)
	s := newRepliesService(d)
	msg := closedTopicReply("hello")
	d.client.OnError("sendMessage", "Bad Request: chat not found")

	expectGroupTopic(d.mock, testClosedTopicID, "Анонсы")
	// no group_messages queries are expected: the reply is not saved when forwarding fails

	require.NoError(t, s.RepliesFromClosedThreads(msg, d.bot, ext.NewContext(d.bot, testutils.MessageUpdate(msg), nil)))
	assert.Len(t, d.client.CallsTo("sendMessage"), 1)
}

func TestRepliesFromClosedThreadsService_RepliesFromClosedThreads_DeleteErrorIsSwallowed(t *testing.T) {
	d := newTestDeps(t)
	s := newRepliesService(d)
	msg := closedTopicReply("hello")
	d.client.OnError("deleteMessage", "Bad Request: message can't be deleted")

	expectGroupTopic(d.mock, testClosedTopicID, "Анонсы")
	expectGroupMessageNotFound(d.mock, msg.MessageId)
	expectExistingUser(d.mock, 7, testUserID)
	expectGroupMessageInsert(d.mock, msg.MessageId, "hello", testRepliedPostID, testUserID, testClosedTopicID)

	require.NoError(t, s.RepliesFromClosedThreads(msg, d.bot, ext.NewContext(d.bot, testutils.MessageUpdate(msg), nil)))
	assert.Len(t, d.client.CallsTo("sendMessage"), 1)
	assert.Len(t, d.client.CallsTo("deleteMessage"), 1)
}
