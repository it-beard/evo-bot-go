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
	groupTopicInsertSQL = `
		INSERT INTO group_topics (topic_id, name)
		VALUES ($1, $2)
		RETURNING id, topic_id, name, created_at, updated_at`
	groupTopicUpdateSQL = `
		UPDATE group_topics
		SET name = $1, updated_at = NOW()
		WHERE topic_id = $2
		RETURNING id, topic_id, name, created_at, updated_at`
	groupTopicByTopicIDSQL = `
		SELECT id, topic_id, name, created_at, updated_at
		FROM group_topics
		WHERE topic_id = $1`
	groupTopicAllSQL = `
		SELECT id, topic_id, name, created_at, updated_at
		FROM group_topics
		ORDER BY created_at ASC`
	groupTopicDeleteSQL = `DELETE FROM group_topics WHERE topic_id = $1`
)

var groupTopicColumns = []string{"id", "topic_id", "name", "created_at", "updated_at"}

const testTopicID int64 = 1234567890123

func TestGroupTopicRepository_AddGroupTopic_Success(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewGroupTopicRepository(db)

	mock.ExpectQuery(exactSQL(groupTopicInsertSQL)).WithArgs(testTopicID, "General").
		WillReturnRows(sqlmock.NewRows(groupTopicColumns).AddRow(1, testTopicID, "General", testCreatedAt, testUpdatedAt))

	gt, err := repo.AddGroupTopic(testTopicID, "General")

	require.NoError(t, err)
	require.NotNil(t, gt)
	assert.Equal(t, GroupTopic{ID: 1, TopicID: testTopicID, Name: "General", CreatedAt: testCreatedAt, UpdatedAt: testUpdatedAt}, *gt)
}

func TestGroupTopicRepository_AddGroupTopic_QueryError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewGroupTopicRepository(db)

	mock.ExpectQuery(exactSQL(groupTopicInsertSQL)).WithArgs(testTopicID, "General").WillReturnError(errDB)

	gt, err := repo.AddGroupTopic(testTopicID, "General")

	require.Error(t, err)
	assert.Nil(t, gt)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to insert group topic")
}

func TestGroupTopicRepository_UpdateGroupTopic_Success(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewGroupTopicRepository(db)

	mock.ExpectQuery(exactSQL(groupTopicUpdateSQL)).WithArgs("Renamed", testTopicID).
		WillReturnRows(sqlmock.NewRows(groupTopicColumns).AddRow(1, testTopicID, "Renamed", testCreatedAt, testUpdatedAt))

	gt, err := repo.UpdateGroupTopic(testTopicID, "Renamed")

	require.NoError(t, err)
	require.NotNil(t, gt)
	assert.Equal(t, GroupTopic{ID: 1, TopicID: testTopicID, Name: "Renamed", CreatedAt: testCreatedAt, UpdatedAt: testUpdatedAt}, *gt)
}

func TestGroupTopicRepository_UpdateGroupTopic_NotFound(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewGroupTopicRepository(db)

	mock.ExpectQuery(exactSQL(groupTopicUpdateSQL)).WithArgs("Renamed", testTopicID).
		WillReturnRows(sqlmock.NewRows(groupTopicColumns))

	gt, err := repo.UpdateGroupTopic(testTopicID, "Renamed")

	require.Error(t, err)
	assert.Nil(t, gt)
	assert.Contains(t, err.Error(), "no group topic found with topic_id 1234567890123")
}

func TestGroupTopicRepository_UpdateGroupTopic_QueryError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewGroupTopicRepository(db)

	mock.ExpectQuery(exactSQL(groupTopicUpdateSQL)).WithArgs("Renamed", testTopicID).WillReturnError(errDB)

	gt, err := repo.UpdateGroupTopic(testTopicID, "Renamed")

	require.Error(t, err)
	assert.Nil(t, gt)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to update group topic with topic_id 1234567890123")
}

func TestGroupTopicRepository_GetGroupTopicByTopicID_Success(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewGroupTopicRepository(db)

	mock.ExpectQuery(exactSQL(groupTopicByTopicIDSQL)).WithArgs(testTopicID).
		WillReturnRows(sqlmock.NewRows(groupTopicColumns).AddRow(4, testTopicID, "Jobs", testCreatedAt, testUpdatedAt))

	gt, err := repo.GetGroupTopicByTopicID(testTopicID)

	require.NoError(t, err)
	require.NotNil(t, gt)
	assert.Equal(t, GroupTopic{ID: 4, TopicID: testTopicID, Name: "Jobs", CreatedAt: testCreatedAt, UpdatedAt: testUpdatedAt}, *gt)
}

func TestGroupTopicRepository_GetGroupTopicByTopicID_NotFound(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewGroupTopicRepository(db)

	mock.ExpectQuery(exactSQL(groupTopicByTopicIDSQL)).WithArgs(testTopicID).
		WillReturnRows(sqlmock.NewRows(groupTopicColumns))

	gt, err := repo.GetGroupTopicByTopicID(testTopicID)

	require.Error(t, err)
	assert.Nil(t, gt)
	assert.Contains(t, err.Error(), "no group topic found with topic_id 1234567890123")
}

func TestGroupTopicRepository_GetGroupTopicByTopicID_QueryError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewGroupTopicRepository(db)

	mock.ExpectQuery(exactSQL(groupTopicByTopicIDSQL)).WithArgs(testTopicID).WillReturnError(errDB)

	gt, err := repo.GetGroupTopicByTopicID(testTopicID)

	require.Error(t, err)
	assert.Nil(t, gt)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to get group topic with topic_id 1234567890123")
}

func TestGroupTopicRepository_GetAllGroupTopics_Success(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewGroupTopicRepository(db)

	mock.ExpectQuery(exactSQL(groupTopicAllSQL)).
		WillReturnRows(sqlmock.NewRows(groupTopicColumns).
			AddRow(1, int64(10), "General", testCreatedAt, testUpdatedAt).
			AddRow(2, int64(20), "Jobs", testUpdatedAt, testUpdatedAt))

	topics, err := repo.GetAllGroupTopics()

	require.NoError(t, err)
	assert.Equal(t, []GroupTopic{
		{ID: 1, TopicID: 10, Name: "General", CreatedAt: testCreatedAt, UpdatedAt: testUpdatedAt},
		{ID: 2, TopicID: 20, Name: "Jobs", CreatedAt: testUpdatedAt, UpdatedAt: testUpdatedAt},
	}, topics)
}

func TestGroupTopicRepository_GetAllGroupTopics_Empty(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewGroupTopicRepository(db)

	mock.ExpectQuery(exactSQL(groupTopicAllSQL)).WillReturnRows(sqlmock.NewRows(groupTopicColumns))

	topics, err := repo.GetAllGroupTopics()

	require.NoError(t, err)
	assert.Empty(t, topics)
}

func TestGroupTopicRepository_GetAllGroupTopics_QueryError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewGroupTopicRepository(db)

	mock.ExpectQuery(exactSQL(groupTopicAllSQL)).WillReturnError(errDB)

	topics, err := repo.GetAllGroupTopics()

	require.Error(t, err)
	assert.Nil(t, topics)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to query group topics")
}

func TestGroupTopicRepository_GetAllGroupTopics_ScanError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewGroupTopicRepository(db)

	mock.ExpectQuery(exactSQL(groupTopicAllSQL)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "topic_id", "name"}).AddRow(1, int64(10), "General"))

	topics, err := repo.GetAllGroupTopics()

	require.Error(t, err)
	assert.Nil(t, topics)
	assert.Contains(t, err.Error(), "failed to scan group topic row")
}

func TestGroupTopicRepository_GetAllGroupTopics_RowsError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewGroupTopicRepository(db)

	mock.ExpectQuery(exactSQL(groupTopicAllSQL)).
		WillReturnRows(sqlmock.NewRows(groupTopicColumns).
			AddRow(1, int64(10), "General", testCreatedAt, testUpdatedAt).
			AddRow(2, int64(20), "Jobs", testCreatedAt, testUpdatedAt).
			RowError(1, errDB))

	topics, err := repo.GetAllGroupTopics()

	require.Error(t, err)
	assert.Nil(t, topics)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "error during rows iteration for group topics")
}

func TestGroupTopicRepository_DeleteGroupTopic(t *testing.T) {
	rowsAffectedErr := errors.New("rows affected unsupported")

	tests := []struct {
		name      string
		setup     func(e *sqlmock.ExpectedExec)
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
			errIs:     errDB,
			errSubstr: "failed to delete group topic with topic_id 1234567890123",
		},
		{
			name:      "not found",
			setup:     func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewResult(0, 0)) },
			errSubstr: "no group topic found with topic_id 1234567890123 to delete",
		},
		{
			name:      "rows affected error is returned",
			setup:     func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewErrorResult(rowsAffectedErr)) },
			errIs:     rowsAffectedErr,
			errSubstr: "could not get rows affected after delete",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewGroupTopicRepository(db)

			tc.setup(mock.ExpectExec(exactSQL(groupTopicDeleteSQL)).WithArgs(testTopicID))

			err := repo.DeleteGroupTopic(testTopicID)

			if tc.errSubstr == "" {
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
