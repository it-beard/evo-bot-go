package grouphandlersservices

import (
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTopicServiceMsg(threadID int64) *gotgbot.Message {
	return newGroupMsg(testUserID, threadID, "")
}

func expectTopicInsert(mock sqlmock.Sqlmock, topicID int64, name string) *sqlmock.ExpectedQuery {
	return mock.ExpectQuery(sqlInsertGroupTopic).WithArgs(topicID, name)
}

func topicRow(topicID int64, name string) *sqlmock.Rows {
	return sqlmock.NewRows(groupTopicColumns).AddRow(5, topicID, name, testCreatedAt, testCreatedAt)
}

func TestSaveTopicService_IsTopicShouldBeSavedOrUpdated(t *testing.T) {
	s := NewSaveTopicService(nil)

	created := newTopicServiceMsg(50)
	created.ForumTopicCreated = &gotgbot.ForumTopicCreated{Name: "New"}
	edited := newTopicServiceMsg(50)
	edited.ForumTopicEdited = &gotgbot.ForumTopicEdited{Name: "Renamed"}

	assert.True(t, s.IsTopicShouldBeSavedOrUpdated(created))
	assert.True(t, s.IsTopicShouldBeSavedOrUpdated(edited))
	assert.False(t, s.IsTopicShouldBeSavedOrUpdated(newGroupMsg(testUserID, 50, "plain text")))
}

func TestSaveTopicService_SaveOrUpdateTopic_Created(t *testing.T) {
	d := newTestDeps(t)
	s := NewSaveTopicService(d.groupTopicRepo)
	msg := newTopicServiceMsg(50)
	msg.ForumTopicCreated = &gotgbot.ForumTopicCreated{Name: "New topic"}

	expectTopicInsert(d.mock, 50, "New topic").WillReturnRows(topicRow(50, "New topic"))

	require.NoError(t, s.SaveOrUpdateTopic(msg))
}

func TestSaveTopicService_SaveOrUpdateTopic_CreatedInsertError(t *testing.T) {
	d := newTestDeps(t)
	s := NewSaveTopicService(d.groupTopicRepo)
	msg := newTopicServiceMsg(50)
	msg.ForumTopicCreated = &gotgbot.ForumTopicCreated{Name: "New topic"}

	expectTopicInsert(d.mock, 50, "New topic").WillReturnError(errDB)

	err := s.SaveOrUpdateTopic(msg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to save forum topic created")
}

func TestSaveTopicService_SaveOrUpdateTopic_EditedWithNewName(t *testing.T) {
	d := newTestDeps(t)
	s := NewSaveTopicService(d.groupTopicRepo)
	msg := newTopicServiceMsg(50)
	msg.ForumTopicEdited = &gotgbot.ForumTopicEdited{Name: "Renamed"}

	d.mock.ExpectQuery(sqlUpdateGroupTopic).WithArgs("Renamed", int64(50)).WillReturnRows(topicRow(50, "Renamed"))

	require.NoError(t, s.SaveOrUpdateTopic(msg))
}

func TestSaveTopicService_SaveOrUpdateTopic_EditedWithoutNameKeepsExistingName(t *testing.T) {
	d := newTestDeps(t)
	s := NewSaveTopicService(d.groupTopicRepo)
	msg := newTopicServiceMsg(50)
	msg.ForumTopicEdited = &gotgbot.ForumTopicEdited{IconCustomEmojiId: "emoji-id"}

	expectGroupTopic(d.mock, 50, "Existing name")
	d.mock.ExpectQuery(sqlUpdateGroupTopic).WithArgs("Existing name", int64(50)).WillReturnRows(topicRow(50, "Existing name"))

	require.NoError(t, s.SaveOrUpdateTopic(msg))
}

func TestSaveTopicService_SaveOrUpdateTopic_EditedWithoutNameLookupError(t *testing.T) {
	d := newTestDeps(t)
	s := NewSaveTopicService(d.groupTopicRepo)
	msg := newTopicServiceMsg(50)
	msg.ForumTopicEdited = &gotgbot.ForumTopicEdited{IconCustomEmojiId: "emoji-id"}

	expectGroupTopicNotFound(d.mock, 50)

	err := s.SaveOrUpdateTopic(msg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get existing topic for edit")
}

func TestSaveTopicService_SaveOrUpdateTopic_EditedUnknownTopicIsCreated(t *testing.T) {
	d := newTestDeps(t)
	s := NewSaveTopicService(d.groupTopicRepo)
	msg := newTopicServiceMsg(50)
	msg.ForumTopicEdited = &gotgbot.ForumTopicEdited{Name: "Renamed"}

	d.mock.ExpectQuery(sqlUpdateGroupTopic).WithArgs("Renamed", int64(50)).WillReturnError(sql.ErrNoRows)
	expectTopicInsert(d.mock, 50, "Renamed").WillReturnRows(topicRow(50, "Renamed"))

	require.NoError(t, s.SaveOrUpdateTopic(msg))
}

func TestSaveTopicService_SaveOrUpdateTopic_EditedUnknownTopicCreateError(t *testing.T) {
	d := newTestDeps(t)
	s := NewSaveTopicService(d.groupTopicRepo)
	msg := newTopicServiceMsg(50)
	msg.ForumTopicEdited = &gotgbot.ForumTopicEdited{Name: "Renamed"}

	d.mock.ExpectQuery(sqlUpdateGroupTopic).WithArgs("Renamed", int64(50)).WillReturnError(sql.ErrNoRows)
	expectTopicInsert(d.mock, 50, "Renamed").WillReturnError(errDB)

	err := s.SaveOrUpdateTopic(msg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create forum topic during edit")
}

func TestSaveTopicService_SaveOrUpdateTopic_EditedUpdateError(t *testing.T) {
	d := newTestDeps(t)
	s := NewSaveTopicService(d.groupTopicRepo)
	msg := newTopicServiceMsg(50)
	msg.ForumTopicEdited = &gotgbot.ForumTopicEdited{Name: "Renamed"}

	d.mock.ExpectQuery(sqlUpdateGroupTopic).WithArgs("Renamed", int64(50)).WillReturnError(errDB)

	err := s.SaveOrUpdateTopic(msg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update forum topic edited")
	assert.ErrorIs(t, err, errDB)
}

func TestSaveTopicService_SaveOrUpdateTopic_NoTopicEventIsNoop(t *testing.T) {
	d := newTestDeps(t)
	s := NewSaveTopicService(d.groupTopicRepo)

	require.NoError(t, s.SaveOrUpdateTopic(newGroupMsg(testUserID, 50, "plain")))
}
