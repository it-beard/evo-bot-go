package repositories

import (
	"errors"
	"testing"

	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	topicInsertSQL = `INSERT INTO topics (topic, user_nickname, event_id) VALUES ($1, $2, $3) RETURNING id`
	topicByIDSQL   = `
		SELECT id, topic, user_nickname, event_id, created_at
		FROM topics
		WHERE id = $1`
)

func TestTopicRepository_CreateTopic_Success(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewTopicRepository(db)

	mock.ExpectQuery(exactSQL(topicInsertSQL)).
		WithArgs("Generics in Go", "gopher", 5).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(31))

	id, err := repo.CreateTopic("Generics in Go", "gopher", 5)

	require.NoError(t, err)
	assert.Equal(t, 31, id)
}

func TestTopicRepository_CreateTopic_QueryError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewTopicRepository(db)

	mock.ExpectQuery(exactSQL(topicInsertSQL)).
		WithArgs("Generics in Go", "gopher", 5).
		WillReturnError(errDB)

	id, err := repo.CreateTopic("Generics in Go", "gopher", 5)

	require.Error(t, err)
	assert.Zero(t, id)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to insert topic")
}

func TestTopicRepository_GetTopicsByEventID_Success(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewTopicRepository(db)

	mock.ExpectQuery(exactSQL(topicsByEventIDSQL)).WithArgs(5).
		WillReturnRows(sqlmock.NewRows(topicColumns).
			AddRow(1, "Topic A", "alice", 5, testCreatedAt).
			AddRow(2, "Topic B", nil, 5, testUpdatedAt))

	topics, err := repo.GetTopicsByEventID(5)

	require.NoError(t, err)
	require.Len(t, topics, 2)

	assert.Equal(t, 1, topics[0].ID)
	assert.Equal(t, "Topic A", topics[0].Topic)
	require.NotNil(t, topics[0].UserNickname)
	assert.Equal(t, "alice", *topics[0].UserNickname)
	assert.Equal(t, 5, topics[0].EventID)
	assert.Equal(t, testCreatedAt, topics[0].CreatedAt)

	assert.Equal(t, 2, topics[1].ID)
	assert.Equal(t, "Topic B", topics[1].Topic)
	assert.Nil(t, topics[1].UserNickname)
	assert.Equal(t, 5, topics[1].EventID)
	assert.Equal(t, testUpdatedAt, topics[1].CreatedAt)
}

func TestTopicRepository_GetTopicsByEventID_Empty(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewTopicRepository(db)

	mock.ExpectQuery(exactSQL(topicsByEventIDSQL)).WithArgs(5).WillReturnRows(sqlmock.NewRows(topicColumns))

	topics, err := repo.GetTopicsByEventID(5)

	require.NoError(t, err)
	assert.Empty(t, topics)
}

func TestTopicRepository_GetTopicsByEventID_QueryError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewTopicRepository(db)

	mock.ExpectQuery(exactSQL(topicsByEventIDSQL)).WithArgs(5).WillReturnError(errDB)

	topics, err := repo.GetTopicsByEventID(5)

	require.Error(t, err)
	assert.Nil(t, topics)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to query topics for event ID 5")
}

func TestTopicRepository_GetTopicsByEventID_ScanError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewTopicRepository(db)

	mock.ExpectQuery(exactSQL(topicsByEventIDSQL)).WithArgs(5).
		WillReturnRows(sqlmock.NewRows(topicColumns).AddRow("x", "Topic A", "alice", 5, testCreatedAt))

	topics, err := repo.GetTopicsByEventID(5)

	require.Error(t, err)
	assert.Nil(t, topics)
	assert.Contains(t, err.Error(), "failed to scan topic row")
}

func TestTopicRepository_GetTopicsByEventID_RowsError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewTopicRepository(db)

	mock.ExpectQuery(exactSQL(topicsByEventIDSQL)).WithArgs(5).
		WillReturnRows(sqlmock.NewRows(topicColumns).
			AddRow(1, "Topic A", "alice", 5, testCreatedAt).
			RowError(0, errDB))

	topics, err := repo.GetTopicsByEventID(5)

	require.Error(t, err)
	assert.Nil(t, topics)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "error during rows iteration for topics")
}

func TestTopicRepository_GetTopicByID_Success(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewTopicRepository(db)

	mock.ExpectQuery(exactSQL(topicByIDSQL)).WithArgs(7).
		WillReturnRows(sqlmock.NewRows(topicColumns).AddRow(7, "Topic", "carol", 2, testCreatedAt))

	topic, err := repo.GetTopicByID(7)

	require.NoError(t, err)
	require.NotNil(t, topic)
	require.NotNil(t, topic.UserNickname)
	assert.Equal(t, Topic{ID: 7, Topic: "Topic", UserNickname: topic.UserNickname, EventID: 2, CreatedAt: testCreatedAt}, *topic)
	assert.Equal(t, "carol", *topic.UserNickname)
}

func TestTopicRepository_GetTopicByID_NullNickname(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewTopicRepository(db)

	mock.ExpectQuery(exactSQL(topicByIDSQL)).WithArgs(7).
		WillReturnRows(sqlmock.NewRows(topicColumns).AddRow(7, "Topic", nil, 2, testCreatedAt))

	topic, err := repo.GetTopicByID(7)

	require.NoError(t, err)
	require.NotNil(t, topic)
	assert.Nil(t, topic.UserNickname)
}

func TestTopicRepository_GetTopicByID_NotFound(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewTopicRepository(db)

	mock.ExpectQuery(exactSQL(topicByIDSQL)).WithArgs(7).WillReturnRows(sqlmock.NewRows(topicColumns))

	topic, err := repo.GetTopicByID(7)

	require.Error(t, err)
	assert.Nil(t, topic)
	assert.Contains(t, err.Error(), "no topic found with ID 7")
}

func TestTopicRepository_GetTopicByID_QueryError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewTopicRepository(db)

	mock.ExpectQuery(exactSQL(topicByIDSQL)).WithArgs(7).WillReturnError(errDB)

	topic, err := repo.GetTopicByID(7)

	require.Error(t, err)
	assert.Nil(t, topic)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to get topic with ID 7")
}

func TestTopicRepository_DeleteTopic(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(e *sqlmock.ExpectedExec)
		wantErr   bool
		errIs     error
		errSubstr string
	}{
		{
			name:  "success",
			setup: func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewResult(0, 1)) },
		},
		{
			name:      "exec error",
			setup:     func(e *sqlmock.ExpectedExec) { e.WillReturnError(errDB) },
			wantErr:   true,
			errIs:     errDB,
			errSubstr: "failed to delete topic with ID 7",
		},
		{
			name:      "not found",
			setup:     func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewResult(0, 0)) },
			wantErr:   true,
			errSubstr: "no topic found with ID 7 to delete",
		},
		{
			name: "rows affected error is only logged",
			setup: func(e *sqlmock.ExpectedExec) {
				e.WillReturnResult(sqlmock.NewErrorResult(errors.New("rows affected unsupported")))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewTopicRepository(db)

			tc.setup(mock.ExpectExec(exactSQL(topicDeleteSQL)).WithArgs(7))

			err := repo.DeleteTopic(7)

			if !tc.wantErr {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			if tc.errIs != nil {
				assert.ErrorIs(t, err, tc.errIs)
			}
			assert.Contains(t, err.Error(), tc.errSubstr)
		})
	}
}
