package grouphandlers_test

import (
	"testing"

	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageHandler_IgnoresNonSupergroupChats(t *testing.T) {
	tests := []struct {
		name string
		msg  *gotgbot.Message
	}{
		{"private chat", testutils.NewPrivateMessage(testUserID, "hello")},
		{"basic group", testutils.NewMessage(testUserID, -555, "group", "hello")},
		{"private join-like message", func() *gotgbot.Message {
			m := testutils.NewPrivateMessage(testUserID, "")
			m.NewChatMembers = []gotgbot.User{*testutils.NewUser(testUserID)}
			return m
		}()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newHandlerEnv(t)

			handled, err := testutils.Dispatch(e.bot, e.messageHandler(), testutils.MessageUpdate(tt.msg))

			require.NoError(t, err)
			assert.True(t, handled)
			assert.Empty(t, e.client.Calls())
		})
	}
}

func TestMessageHandler_DoesNotHandleNonMessageUpdates(t *testing.T) {
	e := newHandlerEnv(t)

	handled, err := testutils.Dispatch(e.bot, e.messageHandler(), testutils.PollAnswerUpdate(testUserID, "poll", []int64{0}))

	require.NoError(t, err)
	assert.False(t, handled)
}

func TestMessageHandler_JoinAndLeftMessagesAreDeletedBeforeAnythingElse(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(m *gotgbot.Message)
	}{
		{"join", func(m *gotgbot.Message) { m.NewChatMembers = []gotgbot.User{*testutils.NewUser(testUserID)} }},
		{"left", func(m *gotgbot.Message) { m.LeftChatMember = testutils.NewUser(testUserID) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newHandlerEnv(t)
			// in a closed topic and with text, so every later step would also match
			msg := newGroupMsg(testUserID, testClosedTopicID, "welcome")
			tt.mutate(msg)

			handled, err := testutils.Dispatch(e.bot, e.messageHandler(), testutils.MessageUpdate(msg))

			require.NoError(t, err)
			assert.True(t, handled)
			assert.Equal(t, []string{"deleteMessage"}, methods(e.client))
			call, _ := e.client.LastCallTo("deleteMessage")
			assert.Equal(t, testChatID, call.Int64("chat_id"))
			assert.Equal(t, msg.MessageId, call.Int64("message_id"))
		})
	}
}

func TestMessageHandler_JoinMessageDeleteErrorIsReturned(t *testing.T) {
	e := newHandlerEnv(t)
	e.client.OnError("deleteMessage", "Bad Request: message can't be deleted")
	msg := newGroupMsg(testUserID, 0, "")
	msg.NewChatMembers = []gotgbot.User{*testutils.NewUser(testUserID)}

	_, err := testutils.Dispatch(e.bot, e.messageHandler(), testutils.MessageUpdate(msg))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "message can't be deleted")
}

func TestMessageHandler_PostInClosedTopicIsCleaned(t *testing.T) {
	e := newHandlerEnv(t)
	msg := newGroupMsg(testUserID, testClosedTopicID, "my thoughts")

	expectGroupTopic(e.mock, testClosedTopicID, "Announcements")
	expectGroupTopic(e.mock, testForwardingTopicID, "Main chat")
	// nothing is saved to group_messages

	handled, err := testutils.Dispatch(e.bot, e.messageHandler(), testutils.MessageUpdate(msg))

	require.NoError(t, err)
	assert.True(t, handled)
	assert.Equal(t, []string{"getChatMember", "deleteMessage", "sendMessage", "sendMessage"}, methods(e.client))
	texts := e.client.SentTexts()
	assert.Contains(t, texts[0], "Announcements")
	assert.Equal(t, "my thoughts", texts[1])
	for _, c := range e.client.CallsTo("sendMessage") {
		assert.Equal(t, testUserID, c.Int64("chat_id"), "notification and copy go to the author's DM")
	}
}

func TestMessageHandler_ReplyInClosedTopicIsForwardedAndSaved(t *testing.T) {
	e := newHandlerEnv(t)
	msg := asReplyTo(newGroupMsg(testUserID, testClosedTopicID, "great news"), 777)

	expectGroupTopic(e.mock, testClosedTopicID, "Announcements")
	e.mock.ExpectQuery(sqlSelectGroupMessageByMsg).WithArgs(msg.MessageId).WillReturnRows(sqlmock.NewRows(groupMessageColumns))
	expectExistingUser(e.mock, 7, testUserID)
	expectGroupMessageInsert(e.mock, msg.MessageId, "great news", int64(777), testUserID, testClosedTopicID)

	handled, err := testutils.Dispatch(e.bot, e.messageHandler(), testutils.MessageUpdate(msg))

	require.NoError(t, err)
	assert.True(t, handled)
	assert.Equal(t, []string{"sendMessage", "deleteMessage"}, methods(e.client), "replies skip the admin check of the cleaner")

	forwarded, _ := e.client.LastCallTo("sendMessage")
	assert.Equal(t, testChatID, forwarded.Int64("chat_id"))
	assert.Equal(t, int64(testForwardingTopicID), forwarded.Int64("message_thread_id"))
	assert.Contains(t, forwarded.Params["text"], "@user4242")
	assert.Contains(t, forwarded.Params["text"], "\ngreat news")
}

func TestMessageHandler_AdminPostInClosedTopicIsSaved(t *testing.T) {
	e := newHandlerEnv(t)
	msg := newGroupMsg(testTopicAdminID, testClosedTopicID, "official announcement")

	expectExistingUser(e.mock, 3, testTopicAdminID)
	expectGroupMessageInsert(e.mock, msg.MessageId, "official announcement", nil, testTopicAdminID, testClosedTopicID)

	handled, err := testutils.Dispatch(e.bot, e.messageHandler(), testutils.MessageUpdate(msg))

	require.NoError(t, err)
	assert.True(t, handled)
	assert.Equal(t, []string{"getChatMember"}, methods(e.client), "admin post must not be deleted")
}

func TestMessageHandler_ForumTopicCreatedIsSavedAsTopic(t *testing.T) {
	e := newHandlerEnv(t)
	msg := newGroupMsg(testUserID, 50, "")
	msg.ForumTopicCreated = &gotgbot.ForumTopicCreated{Name: "Brand new topic"}

	e.mock.ExpectQuery(sqlInsertGroupTopic).WithArgs(int64(50), "Brand new topic").
		WillReturnRows(sqlmock.NewRows(groupTopicColumns).AddRow(1, 50, "Brand new topic", testCreatedAt, testCreatedAt))

	handled, err := testutils.Dispatch(e.bot, e.messageHandler(), testutils.MessageUpdate(msg))

	require.NoError(t, err)
	assert.True(t, handled)
	assert.Empty(t, e.client.Calls())
}

func TestMessageHandler_ForumTopicSaveErrorIsReturned(t *testing.T) {
	e := newHandlerEnv(t)
	msg := newGroupMsg(testUserID, 50, "")
	msg.ForumTopicCreated = &gotgbot.ForumTopicCreated{Name: "Brand new topic"}

	e.mock.ExpectQuery(sqlInsertGroupTopic).WillReturnError(assert.AnError)

	_, err := testutils.Dispatch(e.bot, e.messageHandler(), testutils.MessageUpdate(msg))

	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestMessageHandler_AdminUpdateCommandSavesRepliedPostInsteadOfCommand(t *testing.T) {
	e := newHandlerEnv(t)
	msg := asReplyTo(newGroupMsg(testTopicAdminID, testContentTopicID, "update"), 888)

	// only the replied post is stored, the "update" command itself is not
	e.mock.ExpectQuery(sqlSelectGroupMessageByMsg).WithArgs(int64(888)).WillReturnRows(sqlmock.NewRows(groupMessageColumns))
	expectExistingUser(e.mock, 8, testUserID+1)
	expectGroupMessageInsert(e.mock, 888, "original post", nil, testUserID+1, testContentTopicID)

	handled, err := testutils.Dispatch(e.bot, e.messageHandler(), testutils.MessageUpdate(msg))

	require.NoError(t, err)
	assert.True(t, handled)
	sent := e.client.CallsTo("sendMessage")
	require.Len(t, sent, 1)
	assert.Equal(t, testTopicAdminID, sent[0].Int64("chat_id"))
	assert.Contains(t, sent[0].Params["text"], "успешно добавлено/обновлено")
}

func TestMessageHandler_AnonymousAdminDeleteCommandRemovesRepliedPost(t *testing.T) {
	e := newHandlerEnv(t)
	msg := asReplyTo(newGroupMsg(0, testToolTopicID, "delete"), 888)
	msg.From = groupAnonymousBot()

	e.mock.ExpectQuery(sqlSelectGroupMessageByMsg).WithArgs(int64(888)).
		WillReturnRows(groupMessageRow(15, 888, "original post", testUserID+1, testToolTopicID))
	e.mock.ExpectExec(sqlDeleteGroupMessage).WithArgs(15).WillReturnResult(sqlmock.NewResult(0, 1))

	handled, err := testutils.Dispatch(e.bot, e.messageHandler(), testutils.MessageUpdate(msg))

	require.NoError(t, err)
	assert.True(t, handled)
	deleted := e.client.CallsTo("deleteMessage")
	require.Len(t, deleted, 1)
	assert.Equal(t, int64(888), deleted[0].Int64("message_id"))
	sent, ok := e.client.LastCallTo("sendMessage")
	require.True(t, ok)
	assert.Equal(t, testAdminUserID, sent.Int64("chat_id"))
}

func TestMessageHandler_CommentOnContentPostIsNotSaved(t *testing.T) {
	e := newHandlerEnv(t)
	msg := asReplyTo(newGroupMsg(testUserID, testContentTopicID, "nice post"), 888)

	handled, err := testutils.Dispatch(e.bot, e.messageHandler(), testutils.MessageUpdate(msg))

	require.NoError(t, err)
	assert.True(t, handled)
	assert.Empty(t, e.client.CallsTo("sendMessage"))
	assert.Empty(t, e.client.CallsTo("deleteMessage"))
}

func TestMessageHandler_RegularMessageIsSaved(t *testing.T) {
	e := newHandlerEnv(t)
	msg := newGroupMsg(testUserID, testOpenTopicID, "hello club")

	expectExistingUser(e.mock, 7, testUserID)
	expectGroupMessageInsert(e.mock, msg.MessageId, "hello club", nil, testUserID, testOpenTopicID)

	handled, err := testutils.Dispatch(e.bot, e.messageHandler(), testutils.MessageUpdate(msg))

	require.NoError(t, err)
	assert.True(t, handled)
	assert.Empty(t, e.client.Calls())
}

func TestMessageHandler_SaveErrorIsReturned(t *testing.T) {
	e := newHandlerEnv(t)
	msg := newGroupMsg(testUserID, testOpenTopicID, "hello club")

	expectExistingUser(e.mock, 7, testUserID)
	e.mock.ExpectQuery(sqlInsertGroupMessage).WillReturnError(assert.AnError)

	_, err := testutils.Dispatch(e.bot, e.messageHandler(), testutils.MessageUpdate(msg))

	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestMessageHandler_EditedMessageIsUpdated(t *testing.T) {
	e := newHandlerEnv(t)
	msg := newGroupMsg(testUserID, testOpenTopicID, "hello club (edited)")

	e.mock.ExpectQuery(sqlSelectGroupMessageByMsg).WithArgs(msg.MessageId).
		WillReturnRows(groupMessageRow(15, msg.MessageId, "hello club", testUserID, testOpenTopicID))
	e.mock.ExpectExec(sqlUpdateGroupMessage).WithArgs("hello club (edited)", 15).WillReturnResult(sqlmock.NewResult(0, 1))

	handled, err := testutils.Dispatch(e.bot, e.messageHandler(), testutils.EditedMessageUpdate(msg))

	require.NoError(t, err)
	assert.True(t, handled)
	assert.Empty(t, e.client.Calls())
}

func TestMessageHandler_MessageEditedToDeleteCommandIsRemoved(t *testing.T) {
	e := newHandlerEnv(t)
	msg := newGroupMsg(testUserID, testOpenTopicID, "удалить")

	e.mock.ExpectQuery(sqlSelectGroupMessageByMsg).WithArgs(msg.MessageId).
		WillReturnRows(groupMessageRow(15, msg.MessageId, "hello club", testUserID, testOpenTopicID))
	e.mock.ExpectExec(sqlDeleteGroupMessage).WithArgs(15).WillReturnResult(sqlmock.NewResult(0, 1))

	handled, err := testutils.Dispatch(e.bot, e.messageHandler(), testutils.EditedMessageUpdate(msg))

	require.NoError(t, err)
	assert.True(t, handled)
	assert.Equal(t, []string{"deleteMessage"}, methods(e.client))
	call, _ := e.client.LastCallTo("deleteMessage")
	assert.Equal(t, msg.MessageId, call.Int64("message_id"))
}

func TestMessageHandler_ServiceMessageWithoutContentIsIgnored(t *testing.T) {
	e := newHandlerEnv(t)
	msg := newGroupMsg(testUserID, testOpenTopicID, "")
	msg.PinnedMessage = gotgbot.Message{MessageId: 5}

	handled, err := testutils.Dispatch(e.bot, e.messageHandler(), testutils.MessageUpdate(msg))

	require.NoError(t, err)
	assert.True(t, handled)
	assert.Empty(t, e.client.Calls())
}
