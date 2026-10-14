package grouphandlersservices

import (
	"testing"

	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveMessageService_IsMessageShouldBeSavedOrUpdated(t *testing.T) {
	s := NewSaveMessageService(nil, nil, newTestConfig())

	media := func(threadID int64, mutate func(m *gotgbot.Message)) *gotgbot.Message {
		m := newGroupMsg(testUserID, threadID, "")
		mutate(m)
		return m
	}

	tests := []struct {
		name     string
		msg      *gotgbot.Message
		expected bool
	}{
		{"content topic reply to a post is skipped",
			asReplyTo(newGroupMsg(testUserID, testContentTopicID, "comment"), 500), false},
		{"content topic message replying to topic root is saved",
			asReplyTo(newGroupMsg(testUserID, testContentTopicID, "post"), testContentTopicID), true},
		{"content topic non-reply is saved",
			newGroupMsg(testUserID, testContentTopicID, "post"), true},
		{"reply in other topic is saved",
			asReplyTo(newGroupMsg(testUserID, testOpenTopicID, "comment"), 500), true},
		{"text", newGroupMsg(testUserID, testOpenTopicID, "text"), true},
		{"caption", media(testOpenTopicID, func(m *gotgbot.Message) { m.Caption = "c" }), true},
		{"voice", media(testOpenTopicID, func(m *gotgbot.Message) { m.Voice = &gotgbot.Voice{} }), true},
		{"audio", media(testOpenTopicID, func(m *gotgbot.Message) { m.Audio = &gotgbot.Audio{} }), true},
		{"document", media(testOpenTopicID, func(m *gotgbot.Message) { m.Document = &gotgbot.Document{} }), true},
		{"photo", media(testOpenTopicID, func(m *gotgbot.Message) { m.Photo = []gotgbot.PhotoSize{{}} }), true},
		{"video", media(testOpenTopicID, func(m *gotgbot.Message) { m.Video = &gotgbot.Video{} }), true},
		{"video note", media(testOpenTopicID, func(m *gotgbot.Message) { m.VideoNote = &gotgbot.VideoNote{} }), true},
		{"sticker", media(testOpenTopicID, func(m *gotgbot.Message) { m.Sticker = &gotgbot.Sticker{} }), true},
		{"animation", media(testOpenTopicID, func(m *gotgbot.Message) { m.Animation = &gotgbot.Animation{} }), true},
		{"service message without content", media(testOpenTopicID, func(m *gotgbot.Message) {
			m.PinnedMessage = gotgbot.Message{MessageId: 1}
		}), false},
		{"location without content", media(testOpenTopicID, func(m *gotgbot.Message) {
			m.Location = &gotgbot.Location{Latitude: 1, Longitude: 2}
		}), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, s.IsMessageShouldBeSavedOrUpdated(tt.msg))
		})
	}
}

func newSaveMessageService(d *testDeps) *SaveMessageService {
	return NewSaveMessageService(d.groupMessageRepo, d.saveUpdate, d.cfg)
}

func TestSaveMessageService_SaveOrUpdateMessage_NewMessageIsInserted(t *testing.T) {
	d := newTestDeps(t)
	s := newSaveMessageService(d)
	msg := newGroupMsg(testUserID, testOpenTopicID, "brand new")

	// Save never looks the message up first
	expectExistingUser(d.mock, 7, testUserID)
	expectGroupMessageInsert(d.mock, msg.MessageId, "brand new", nil, testUserID, testOpenTopicID)

	require.NoError(t, s.SaveOrUpdateMessage(ext.NewContext(d.bot, testutils.MessageUpdate(msg), nil)))
}

func TestSaveMessageService_SaveOrUpdateMessage_EditedMessageIsUpdated(t *testing.T) {
	d := newTestDeps(t)
	s := newSaveMessageService(d)
	msg := newGroupMsg(testUserID, testOpenTopicID, "edited text")

	d.mock.ExpectQuery(sqlSelectGroupMessageByMsg).WithArgs(msg.MessageId).
		WillReturnRows(groupMessageRow(15, msg.MessageId, "original text", testUserID, testOpenTopicID))
	d.mock.ExpectExec(sqlUpdateGroupMessage).WithArgs("edited text", 15).WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, s.SaveOrUpdateMessage(ext.NewContext(d.bot, testutils.EditedMessageUpdate(msg), nil)))
	assert.Empty(t, d.client.CallsTo("deleteMessage"))
}

func TestSaveMessageService_SaveOrUpdateMessage_EditedToDeleteCommandDeletesMessage(t *testing.T) {
	for _, command := range []string{"delete", "удалить"} {
		t.Run(command, func(t *testing.T) {
			d := newTestDeps(t)
			s := newSaveMessageService(d)
			msg := newGroupMsg(testUserID, testOpenTopicID, command)

			d.mock.ExpectQuery(sqlSelectGroupMessageByMsg).WithArgs(msg.MessageId).
				WillReturnRows(groupMessageRow(15, msg.MessageId, "original text", testUserID, testOpenTopicID))
			d.mock.ExpectExec(sqlDeleteGroupMessage).WithArgs(15).WillReturnResult(sqlmock.NewResult(0, 1))

			require.NoError(t, s.SaveOrUpdateMessage(ext.NewContext(d.bot, testutils.EditedMessageUpdate(msg), nil)))

			call, ok := d.client.LastCallTo("deleteMessage")
			require.True(t, ok)
			assert.Equal(t, testChatID, call.Int64("chat_id"))
			assert.Equal(t, msg.MessageId, call.Int64("message_id"))
		})
	}
}

func TestSaveMessageService_SaveOrUpdateMessage_NewMessageWithDeleteTextIsSaved(t *testing.T) {
	d := newTestDeps(t)
	s := newSaveMessageService(d)
	msg := newGroupMsg(testUserID, testOpenTopicID, "delete")

	expectExistingUser(d.mock, 7, testUserID)
	expectGroupMessageInsert(d.mock, msg.MessageId, "delete", nil, testUserID, testOpenTopicID)

	require.NoError(t, s.SaveOrUpdateMessage(ext.NewContext(d.bot, testutils.MessageUpdate(msg), nil)))
	assert.Empty(t, d.client.CallsTo("deleteMessage"))
}
