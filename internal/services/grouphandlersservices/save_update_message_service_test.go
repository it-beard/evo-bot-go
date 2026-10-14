package grouphandlersservices

import (
	"database/sql"
	"testing"

	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveUpdateMessageService_Save_TextWithEntitiesInTopic(t *testing.T) {
	d := newTestDeps(t)
	msg := newGroupMsg(testUserID, testOpenTopicID, "hello world")
	msg.Entities = []gotgbot.MessageEntity{{Type: "bold", Offset: 0, Length: 5}}

	expectExistingUser(d.mock, 7, testUserID)
	expectGroupMessageInsert(d.mock, msg.MessageId, "<strong>hello</strong> world", nil, testUserID, testOpenTopicID)

	err := d.saveUpdate.Save(msg)

	require.NoError(t, err)
	assert.Empty(t, d.client.Calls(), "saving must not call Telegram")
}

func TestSaveUpdateMessageService_Save_MediaDescriptions(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(m *gotgbot.Message)
		expected string
	}{
		{"caption is used when there is no text", func(m *gotgbot.Message) {
			m.Caption = "a <caption>"
			m.Photo = []gotgbot.PhotoSize{{FileId: "p"}}
		}, "a &lt;caption&gt;"},
		{"caption entities are applied", func(m *gotgbot.Message) {
			m.Caption = "styled caption"
			m.CaptionEntities = []gotgbot.MessageEntity{{Type: "italic", Offset: 0, Length: 6}}
			m.Video = &gotgbot.Video{FileId: "v"}
		}, "<em>styled</em> caption"},
		{"photo", func(m *gotgbot.Message) { m.Photo = []gotgbot.PhotoSize{{FileId: "p"}} }, "[Photo]"},
		{"video", func(m *gotgbot.Message) { m.Video = &gotgbot.Video{FileId: "v"} }, "[Video]"},
		{"voice", func(m *gotgbot.Message) { m.Voice = &gotgbot.Voice{FileId: "vo"} }, "[Voice message]"},
		{"audio", func(m *gotgbot.Message) { m.Audio = &gotgbot.Audio{FileId: "a"} }, "[Audio]"},
		{"document", func(m *gotgbot.Message) { m.Document = &gotgbot.Document{FileId: "d"} }, "[Document]"},
		{"video note", func(m *gotgbot.Message) { m.VideoNote = &gotgbot.VideoNote{FileId: "vn"} }, "[Video note]"},
		{"sticker", func(m *gotgbot.Message) { m.Sticker = &gotgbot.Sticker{FileId: "s"} }, "[Sticker]"},
		{"animation", func(m *gotgbot.Message) { m.Animation = &gotgbot.Animation{FileId: "g"} }, "[GIF]"},
		{"unknown media", func(m *gotgbot.Message) { m.Location = &gotgbot.Location{Latitude: 1, Longitude: 2} }, "[Media]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newTestDeps(t)
			msg := newGroupMsg(testUserID, testOpenTopicID, "")
			tt.mutate(msg)

			expectExistingUser(d.mock, 7, testUserID)
			expectGroupMessageInsert(d.mock, msg.MessageId, tt.expected, nil, testUserID, testOpenTopicID)

			require.NoError(t, d.saveUpdate.Save(msg))
		})
	}
}

func TestSaveUpdateMessageService_Save_ReplyAndTopicIDs(t *testing.T) {
	t.Run("reply to another message keeps reply id", func(t *testing.T) {
		d := newTestDeps(t)
		msg := asReplyTo(newGroupMsg(testUserID, testOpenTopicID, "answer"), 555)

		expectExistingUser(d.mock, 7, testUserID)
		expectGroupMessageInsert(d.mock, msg.MessageId, "answer", int64(555), testUserID, testOpenTopicID)

		require.NoError(t, d.saveUpdate.Save(msg))
	})

	t.Run("reply to the topic root is not stored as reply", func(t *testing.T) {
		d := newTestDeps(t)
		msg := asReplyTo(newGroupMsg(testUserID, testOpenTopicID, "in topic"), testOpenTopicID)

		expectExistingUser(d.mock, 7, testUserID)
		expectGroupMessageInsert(d.mock, msg.MessageId, "in topic", nil, testUserID, testOpenTopicID)

		require.NoError(t, d.saveUpdate.Save(msg))
	})

	t.Run("thread id without topic flag is stored as general topic", func(t *testing.T) {
		d := newTestDeps(t)
		msg := newGroupMsg(testUserID, 0, "general")
		msg.MessageThreadId = 77 // reply thread in a non-forum context
		msg.IsTopicMessage = false

		expectExistingUser(d.mock, 7, testUserID)
		expectGroupMessageInsert(d.mock, msg.MessageId, "general", nil, testUserID, 0)

		require.NoError(t, d.saveUpdate.Save(msg))
	})
}

func TestSaveUpdateMessageService_Save_GroupAnonymousBotIsSavedAsAdmin(t *testing.T) {
	d := newTestDeps(t)
	msg := newGroupMsg(0, testOpenTopicID, "anonymous admin post")
	msg.From = groupAnonymousBot()

	d.mock.ExpectQuery(sqlSelectUserByTgID).WithArgs(groupAnonymousBotID).
		WillReturnRows(sqlmock.NewRows(userColumns).
			AddRow(3, groupAnonymousBotID, "Group", "", "GroupAnonymousBot", 0, false, false, testCreatedAt, testCreatedAt))
	expectGroupMessageInsert(d.mock, msg.MessageId, "anonymous admin post", nil, testAdminUserID, testOpenTopicID)

	require.NoError(t, d.saveUpdate.Save(msg))
	assert.Equal(t, testAdminUserID, msg.From.Id)
}

func TestSaveUpdateMessageService_Save_UserLookupFailureDoesNotBlockSaving(t *testing.T) {
	d := newTestDeps(t)
	msg := newGroupMsg(testUserID, testOpenTopicID, "still saved")

	d.mock.ExpectQuery(sqlSelectUserByTgID).WithArgs(testUserID).WillReturnError(errDB)
	expectGroupMessageInsert(d.mock, msg.MessageId, "still saved", nil, testUserID, testOpenTopicID)

	require.NoError(t, d.saveUpdate.Save(msg))
}

func TestSaveUpdateMessageService_Save_InsertError(t *testing.T) {
	d := newTestDeps(t)
	msg := newGroupMsg(testUserID, testOpenTopicID, "boom")

	expectExistingUser(d.mock, 7, testUserID)
	d.mock.ExpectQuery(sqlInsertGroupMessage).WillReturnError(errDB)

	err := d.saveUpdate.Save(msg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to save group message")
	assert.ErrorIs(t, err, errDB)
}

func TestSaveUpdateMessageService_SaveOrUpdate_UpdatesChangedMessage(t *testing.T) {
	d := newTestDeps(t)
	msg := newGroupMsg(testUserID, testOpenTopicID, "new text")

	d.mock.ExpectQuery(sqlSelectGroupMessageByMsg).WithArgs(msg.MessageId).
		WillReturnRows(groupMessageRow(15, msg.MessageId, "old text", testUserID, testOpenTopicID))
	d.mock.ExpectExec(sqlUpdateGroupMessage).WithArgs("new text", 15).WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, d.saveUpdate.SaveOrUpdate(msg))
}

func TestSaveUpdateMessageService_SaveOrUpdate_UnchangedMessageIsNotUpdated(t *testing.T) {
	d := newTestDeps(t)
	msg := newGroupMsg(testUserID, testOpenTopicID, "same text")

	d.mock.ExpectQuery(sqlSelectGroupMessageByMsg).WithArgs(msg.MessageId).
		WillReturnRows(groupMessageRow(15, msg.MessageId, "same text", testUserID, testOpenTopicID))

	require.NoError(t, d.saveUpdate.SaveOrUpdate(msg))
}

func TestSaveUpdateMessageService_SaveOrUpdate_MissingMessageIsInserted(t *testing.T) {
	d := newTestDeps(t)
	msg := newGroupMsg(testUserID, testOpenTopicID, "fresh")

	expectGroupMessageNotFound(d.mock, msg.MessageId)
	expectExistingUser(d.mock, 7, testUserID)
	expectGroupMessageInsert(d.mock, msg.MessageId, "fresh", nil, testUserID, testOpenTopicID)

	require.NoError(t, d.saveUpdate.SaveOrUpdate(msg))
}

func TestSaveUpdateMessageService_SaveOrUpdate_Errors(t *testing.T) {
	t.Run("lookup error", func(t *testing.T) {
		d := newTestDeps(t)
		msg := newGroupMsg(testUserID, testOpenTopicID, "x")
		d.mock.ExpectQuery(sqlSelectGroupMessageByMsg).WithArgs(msg.MessageId).WillReturnError(errDB)

		err := d.saveUpdate.SaveOrUpdate(msg)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get existing message")
	})

	t.Run("update error", func(t *testing.T) {
		d := newTestDeps(t)
		msg := newGroupMsg(testUserID, testOpenTopicID, "changed")
		d.mock.ExpectQuery(sqlSelectGroupMessageByMsg).WithArgs(msg.MessageId).
			WillReturnRows(groupMessageRow(15, msg.MessageId, "original", testUserID, testOpenTopicID))
		d.mock.ExpectExec(sqlUpdateGroupMessage).WithArgs("changed", 15).WillReturnError(errDB)

		err := d.saveUpdate.SaveOrUpdate(msg)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update group message")
	})
}

func TestSaveUpdateMessageService_Delete_ExistingMessage(t *testing.T) {
	d := newTestDeps(t)
	msg := newGroupMsg(testUserID, testOpenTopicID, "to delete")

	d.mock.ExpectQuery(sqlSelectGroupMessageByMsg).WithArgs(msg.MessageId).
		WillReturnRows(groupMessageRow(15, msg.MessageId, "to delete", testUserID, testOpenTopicID))
	d.mock.ExpectExec(sqlDeleteGroupMessage).WithArgs(15).WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, d.saveUpdate.Delete(msg))

	call, ok := d.client.LastCallTo("deleteMessage")
	require.True(t, ok)
	assert.Equal(t, testChatID, call.Int64("chat_id"))
	assert.Equal(t, msg.MessageId, call.Int64("message_id"))
}

func TestSaveUpdateMessageService_Delete_MessageNotInDatabaseIsStillDeletedFromTelegram(t *testing.T) {
	d := newTestDeps(t)
	msg := newGroupMsg(testUserID, testOpenTopicID, "unknown")

	d.mock.ExpectQuery(sqlSelectGroupMessageByMsg).WithArgs(msg.MessageId).WillReturnError(sql.ErrNoRows)

	require.NoError(t, d.saveUpdate.Delete(msg))
	assert.Len(t, d.client.CallsTo("deleteMessage"), 1)
}

func TestSaveUpdateMessageService_Delete_TelegramFailureStillDeletesFromDatabase(t *testing.T) {
	d := newTestDeps(t)
	msg := newGroupMsg(testUserID, testOpenTopicID, "to delete")
	d.client.OnError("deleteMessage", "Bad Request: message to delete not found")

	d.mock.ExpectQuery(sqlSelectGroupMessageByMsg).WithArgs(msg.MessageId).
		WillReturnRows(groupMessageRow(15, msg.MessageId, "to delete", testUserID, testOpenTopicID))
	d.mock.ExpectExec(sqlDeleteGroupMessage).WithArgs(15).WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, d.saveUpdate.Delete(msg))
}

func TestSaveUpdateMessageService_Delete_Errors(t *testing.T) {
	t.Run("lookup error aborts before Telegram delete", func(t *testing.T) {
		d := newTestDeps(t)
		msg := newGroupMsg(testUserID, testOpenTopicID, "x")
		d.mock.ExpectQuery(sqlSelectGroupMessageByMsg).WithArgs(msg.MessageId).WillReturnError(errDB)

		err := d.saveUpdate.Delete(msg)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get existing message for deletion")
		assert.Empty(t, d.client.CallsTo("deleteMessage"))
	})

	t.Run("database delete error", func(t *testing.T) {
		d := newTestDeps(t)
		msg := newGroupMsg(testUserID, testOpenTopicID, "x")
		d.mock.ExpectQuery(sqlSelectGroupMessageByMsg).WithArgs(msg.MessageId).
			WillReturnRows(groupMessageRow(15, msg.MessageId, "x", testUserID, testOpenTopicID))
		d.mock.ExpectExec(sqlDeleteGroupMessage).WithArgs(15).WillReturnError(errDB)

		err := d.saveUpdate.Delete(msg)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete message from database")
		assert.Len(t, d.client.CallsTo("deleteMessage"), 1)
	})
}

func TestSaveUpdateMessageService_getMessageText(t *testing.T) {
	s := NewSaveUpdateMessageService(nil, nil, newTestConfig(), nil)

	assert.Equal(t, "text", s.getMessageText(&gotgbot.Message{Text: "text", Caption: "caption"}))
	assert.Equal(t, "caption", s.getMessageText(&gotgbot.Message{Caption: "caption"}))
	assert.Equal(t, "[Sticker]", s.getMessageText(&gotgbot.Message{Sticker: &gotgbot.Sticker{FileId: "s"}}))
	assert.Equal(t, "[Media]", s.getMessageText(testutils.NewGroupMessage(1, testChatID, 0, "")))
}
