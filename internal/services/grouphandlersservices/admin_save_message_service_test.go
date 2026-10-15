package grouphandlersservices

import (
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testTopicAdminID   int64 = 601
	testTopicCreatorID int64 = 602
	testAdminBotID     int64 = 603
	testOtherBotID     int64 = 604
)

func newAdminSaveService(d *testDeps) *AdminSaveMessageService {
	d.client.SetMemberStatus(testTopicAdminID, "administrator")
	d.client.SetMemberStatus(testTopicCreatorID, "creator")
	d.client.SetMemberStatus(testAdminBotID, "administrator")
	return NewAdminSaveMessageService(d.groupMessageRepo, d.bot, d.cfg, d.saveUpdate, d.sender)
}

// adminCommand builds "<command>" sent by fromID as a reply to a post in the given topic
func adminCommand(from *gotgbot.User, threadID int64, command string) *gotgbot.Message {
	msg := newGroupMsg(from.Id, threadID, command)
	msg.From = from
	return asReplyTo(msg, 888)
}

func humanUser(id int64) *gotgbot.User {
	return &gotgbot.User{Id: id, FirstName: "Admin", Username: "admin"}
}

func TestAdminSaveMessageService_IsMessageShouldBeSavedOrUpdated(t *testing.T) {
	notReply := newGroupMsg(testTopicAdminID, testContentTopicID, "update")

	tests := []struct {
		name     string
		msg      *gotgbot.Message
		expected bool
	}{
		{"not a reply", notReply, false},
		{"reply outside content and tool topics", adminCommand(humanUser(testTopicAdminID), testOpenTopicID, "update"), false},
		{"admin update in content topic", adminCommand(humanUser(testTopicAdminID), testContentTopicID, "update"), true},
		{"admin delete in tool topic", adminCommand(humanUser(testTopicAdminID), testToolTopicID, "delete"), true},
		{"creator update in tool topic", adminCommand(humanUser(testTopicCreatorID), testToolTopicID, "update"), true},
		{"anonymous admin update", adminCommand(groupAnonymousBot(), testContentTopicID, "update"), true},
		{"bot with admin rights", adminCommand(&gotgbot.User{Id: testAdminBotID, IsBot: true, Username: "helper_bot"}, testContentTopicID, "delete"), true},
		{"non-admin bot", adminCommand(&gotgbot.User{Id: testOtherBotID, IsBot: true, Username: "random_bot"}, testContentTopicID, "update"), false},
		{"regular member update in content topic", adminCommand(humanUser(testUserID), testContentTopicID, "update"), false},
		{"regular member delete in tool topic", adminCommand(humanUser(testUserID), testToolTopicID, "delete"), false},
		{"message without sender", func() *gotgbot.Message {
			msg := adminCommand(humanUser(testTopicAdminID), testContentTopicID, "delete")
			msg.From = nil
			return msg
		}(), false},
		{"admin reply with another text", adminCommand(humanUser(testTopicAdminID), testContentTopicID, "nice post"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newTestDeps(t)
			s := newAdminSaveService(d)

			assert.Equal(t, tt.expected, s.IsMessageShouldBeSavedOrUpdated(tt.msg))
		})
	}
}

func TestAdminSaveMessageService_IsMessageShouldBeSavedOrUpdated_RegularMemberIsRejected(t *testing.T) {
	// A regular member replying "delete"/"update" must not be able to delete or index posts in Content/Tools
	d := newTestDeps(t)
	s := newAdminSaveService(d)

	assert.False(t, s.IsMessageShouldBeSavedOrUpdated(adminCommand(humanUser(testUserID), testContentTopicID, "delete")))
}

// expectRepliedPostSaved expects SaveOrUpdate of the post created by asReplyTo (id 888, author testUserID+1)
func expectRepliedPostSaved(mock sqlmock.Sqlmock) {
	expectGroupMessageNotFound(mock, 888)
	expectExistingUser(mock, 8, testUserID+1)
	expectGroupMessageInsert(mock, 888, "original post", nil, testUserID+1, testContentTopicID)
}

func TestAdminSaveMessageService_SaveOrUpdateMessage_UpdateSavesRepliedPostAndNotifiesAdmin(t *testing.T) {
	d := newTestDeps(t)
	s := newAdminSaveService(d)
	msg := adminCommand(humanUser(testTopicAdminID), testContentTopicID, "update")

	expectRepliedPostSaved(d.mock)

	require.NoError(t, s.SaveOrUpdateMessage(msg))

	sent := d.client.CallsTo("sendMessage")
	require.Len(t, sent, 1)
	assert.Equal(t, testTopicAdminID, sent[0].Int64("chat_id"))
	assert.Contains(t, sent[0].Params["text"], "успешно добавлено/обновлено")
	// the command message itself is removed later in background, never synchronously
	assert.Empty(t, d.client.CallsTo("deleteMessage"))
}

func TestAdminSaveMessageService_SaveOrUpdateMessage_AnonymousAdminIsNotifiedViaConfiguredAdmin(t *testing.T) {
	d := newTestDeps(t)
	s := newAdminSaveService(d)
	msg := adminCommand(groupAnonymousBot(), testContentTopicID, "update")

	expectRepliedPostSaved(d.mock)

	require.NoError(t, s.SaveOrUpdateMessage(msg))

	sent, ok := d.client.LastCallTo("sendMessage")
	require.True(t, ok)
	assert.Equal(t, testAdminUserID, sent.Int64("chat_id"))
}

func TestAdminSaveMessageService_SaveOrUpdateMessage_UpdateFailureNotifiesAdmin(t *testing.T) {
	d := newTestDeps(t)
	s := newAdminSaveService(d)
	msg := adminCommand(humanUser(testTopicAdminID), testContentTopicID, "update")

	d.mock.ExpectQuery(sqlSelectGroupMessageByMsg).WithArgs(int64(888)).WillReturnError(errDB)

	err := s.SaveOrUpdateMessage(msg)

	require.Error(t, err)
	assert.ErrorIs(t, err, errDB)
	sent := d.client.CallsTo("sendMessage")
	require.Len(t, sent, 1)
	assert.Equal(t, testTopicAdminID, sent[0].Int64("chat_id"))
	assert.Contains(t, sent[0].Params["text"], "Не удалось обновить сообщение")
}

func TestAdminSaveMessageService_SaveOrUpdateMessage_NotificationFailuresAreSwallowed(t *testing.T) {
	t.Run("success notification", func(t *testing.T) {
		d := newTestDeps(t)
		s := newAdminSaveService(d)
		d.client.OnError("sendMessage", "Forbidden: bot can't initiate conversation with a user")
		expectRepliedPostSaved(d.mock)

		assert.NoError(t, s.SaveOrUpdateMessage(adminCommand(humanUser(testTopicAdminID), testContentTopicID, "update")))
	})

	t.Run("failure notification", func(t *testing.T) {
		d := newTestDeps(t)
		s := newAdminSaveService(d)
		d.client.OnError("sendMessage", "Forbidden: bot can't initiate conversation with a user")
		d.mock.ExpectQuery(sqlSelectGroupMessageByMsg).WithArgs(int64(888)).WillReturnError(errDB)

		assert.ErrorIs(t, s.SaveOrUpdateMessage(adminCommand(humanUser(testTopicAdminID), testContentTopicID, "update")), errDB)
	})
}

func TestAdminSaveMessageService_SaveOrUpdateMessage_DeleteRemovesRepliedPost(t *testing.T) {
	d := newTestDeps(t)
	s := newAdminSaveService(d)
	// the command is normalized (trimmed and lower-cased)
	msg := adminCommand(humanUser(testTopicAdminID), testToolTopicID, "  DELETE ")

	d.mock.ExpectQuery(sqlSelectGroupMessageByMsg).WithArgs(int64(888)).
		WillReturnRows(groupMessageRow(15, 888, "original post", testUserID+1, testToolTopicID))
	d.mock.ExpectExec(sqlDeleteGroupMessage).WithArgs(15).WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, s.SaveOrUpdateMessage(msg))

	deleted := d.client.CallsTo("deleteMessage")
	require.Len(t, deleted, 1, "only the replied post is deleted synchronously")
	assert.Equal(t, testChatID, deleted[0].Int64("chat_id"))
	assert.Equal(t, int64(888), deleted[0].Int64("message_id"))

	sent := d.client.CallsTo("sendMessage")
	require.Len(t, sent, 1)
	assert.Equal(t, testTopicAdminID, sent[0].Int64("chat_id"))
	assert.Contains(t, sent[0].Params["text"], "успешно удалено")
}

func TestAdminSaveMessageService_SaveOrUpdateMessage_DeleteNotificationFailureIsSwallowed(t *testing.T) {
	d := newTestDeps(t)
	s := newAdminSaveService(d)
	d.client.OnError("sendMessage", "Forbidden: bot can't initiate conversation with a user")

	d.mock.ExpectQuery(sqlSelectGroupMessageByMsg).WithArgs(int64(888)).WillReturnError(sql.ErrNoRows)

	assert.NoError(t, s.SaveOrUpdateMessage(adminCommand(humanUser(testTopicAdminID), testToolTopicID, "delete")))
	assert.Len(t, d.client.CallsTo("deleteMessage"), 1)
}

func TestAdminSaveMessageService_SaveOrUpdateMessage_UnknownCommandIsNoop(t *testing.T) {
	d := newTestDeps(t)
	s := newAdminSaveService(d)

	require.NoError(t, s.SaveOrUpdateMessage(adminCommand(humanUser(testTopicAdminID), testContentTopicID, "hello")))
	assert.Empty(t, d.client.Calls())
}
