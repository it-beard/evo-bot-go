package repositories

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"

	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	groupMessageInsertSQL = `
		INSERT INTO group_messages (message_id, message_text, reply_to_message_id, user_tg_id, group_topic_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, message_id, message_text, reply_to_message_id, user_tg_id, group_topic_id, created_at, updated_at`
	groupMessageInsertWithCreatedAtSQL = `
		INSERT INTO group_messages (message_id, message_text, reply_to_message_id, user_tg_id, group_topic_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		RETURNING id, message_id, message_text, reply_to_message_id, user_tg_id, group_topic_id, created_at, updated_at`
	groupMessageSelect = `
		SELECT id, message_id, message_text, reply_to_message_id, user_tg_id, group_topic_id, created_at, updated_at
		FROM group_messages`
	groupMessageByIDSQL          = groupMessageSelect + ` WHERE id = $1`
	groupMessageByMessageIDSQL   = groupMessageSelect + ` WHERE message_id = $1`
	groupMessageByUserTgIDSQL    = groupMessageSelect + ` WHERE user_tg_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	groupMessageByTopicAllSQL    = groupMessageSelect + ` WHERE group_topic_id = $1 ORDER BY created_at DESC`
	groupMessageByTopicLast24h   = groupMessageSelect + ` WHERE group_topic_id = $1 AND created_at >= NOW() - INTERVAL '24 hours' ORDER BY created_at ASC`
	groupMessageUpdateSQL        = `UPDATE group_messages SET message_text = $1, updated_at = NOW() WHERE id = $2`
	groupMessageDeleteSQL        = `DELETE FROM group_messages WHERE id = $1`
	groupMessageDeleteByMsgIDSQL = `DELETE FROM group_messages WHERE message_id = $1`
)

var groupMessageColumns = []string{
	"id", "message_id", "message_text", "reply_to_message_id", "user_tg_id", "group_topic_id", "created_at", "updated_at",
}

func int64Ptr(v int64) *int64 { return &v }

func TestGroupMessageRepository_GetDB(t *testing.T) {
	db, _ := testutils.NewMockDB(t)

	assert.Same(t, db, NewGroupMessageRepository(db).GetDB())
}

func TestGroupMessageRepository_Create(t *testing.T) {
	tests := []struct {
		name      string
		replyTo   *int64
		replyArg  driver.Value
		replyCell driver.Value
	}{
		{name: "with reply", replyTo: int64Ptr(500), replyArg: int64(500), replyCell: int64(500)},
		{name: "without reply", replyTo: nil, replyArg: nil, replyCell: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewGroupMessageRepository(db)

			mock.ExpectQuery(exactSQL(groupMessageInsertSQL)).
				WithArgs(int64(1001), "hello", tc.replyArg, int64(42), int64(77)).
				WillReturnRows(sqlmock.NewRows(groupMessageColumns).
					AddRow(9, int64(1001), "hello", tc.replyCell, int64(42), int64(77), testCreatedAt, testUpdatedAt))

			msg, err := repo.Create(1001, "hello", tc.replyTo, 42, 77)

			require.NoError(t, err)
			require.NotNil(t, msg)
			assert.Equal(t, 9, msg.ID)
			assert.Equal(t, int64(1001), msg.MessageID)
			assert.Equal(t, "hello", msg.MessageText)
			assert.Equal(t, tc.replyTo, msg.ReplyToMessageID)
			assert.Equal(t, int64(42), msg.UserTgID)
			assert.Equal(t, int64(77), msg.GroupTopicID)
			assert.Equal(t, testCreatedAt, msg.CreatedAt)
			assert.Equal(t, testUpdatedAt, msg.UpdatedAt)
		})
	}
}

func TestGroupMessageRepository_Create_QueryError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewGroupMessageRepository(db)

	mock.ExpectQuery(exactSQL(groupMessageInsertSQL)).WillReturnError(errDB)

	msg, err := repo.Create(1001, "hello", nil, 42, 77)

	require.Error(t, err)
	assert.Nil(t, msg)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to insert group message")
}

func TestGroupMessageRepository_CreateWithCreatedAt_Success(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewGroupMessageRepository(db)

	mock.ExpectQuery(exactSQL(groupMessageInsertWithCreatedAtSQL)).
		WithArgs(int64(1001), "hello", int64(500), int64(42), int64(77), testStartedAt).
		WillReturnRows(sqlmock.NewRows(groupMessageColumns).
			AddRow(9, int64(1001), "hello", int64(500), int64(42), int64(77), testStartedAt, testStartedAt))

	msg, err := repo.CreateWithCreatedAt(1001, "hello", int64Ptr(500), 42, 77, testStartedAt)

	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, GroupMessage{
		ID: 9, MessageID: 1001, MessageText: "hello", ReplyToMessageID: int64Ptr(500),
		UserTgID: 42, GroupTopicID: 77, CreatedAt: testStartedAt, UpdatedAt: testStartedAt,
	}, *msg)
}

func TestGroupMessageRepository_CreateWithCreatedAt_QueryError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewGroupMessageRepository(db)

	mock.ExpectQuery(exactSQL(groupMessageInsertWithCreatedAtSQL)).
		WithArgs(int64(1001), "hello", nil, int64(42), int64(77), testStartedAt).
		WillReturnError(errDB)

	msg, err := repo.CreateWithCreatedAt(1001, "hello", nil, 42, 77, testStartedAt)

	require.Error(t, err)
	assert.Nil(t, msg)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to insert group message with created_at")
}

// groupMessageGetCase describes a single-row lookup method
type groupMessageGetCase struct {
	name      string
	sql       string
	arg       driver.Value
	call      func(r *GroupMessageRepository) (*GroupMessage, error)
	errSubstr string
}

func groupMessageGetCases() []groupMessageGetCase {
	return []groupMessageGetCase{
		{
			name:      "GetByID",
			sql:       groupMessageByIDSQL,
			arg:       9,
			call:      func(r *GroupMessageRepository) (*GroupMessage, error) { return r.GetByID(9) },
			errSubstr: "failed to get group message with ID 9",
		},
		{
			name:      "GetByMessageID",
			sql:       groupMessageByMessageIDSQL,
			arg:       int64(1001),
			call:      func(r *GroupMessageRepository) (*GroupMessage, error) { return r.GetByMessageID(1001) },
			errSubstr: "failed to get group message with message ID 1001",
		},
	}
}

func TestGroupMessageRepository_GetSingle_Success(t *testing.T) {
	for _, tc := range groupMessageGetCases() {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewGroupMessageRepository(db)

			mock.ExpectQuery(exactSQL(tc.sql)).WithArgs(tc.arg).
				WillReturnRows(sqlmock.NewRows(groupMessageColumns).
					AddRow(9, int64(1001), "hello", nil, int64(42), int64(77), testCreatedAt, testUpdatedAt))

			msg, err := tc.call(repo)

			require.NoError(t, err)
			require.NotNil(t, msg)
			assert.Equal(t, GroupMessage{
				ID: 9, MessageID: 1001, MessageText: "hello", ReplyToMessageID: nil,
				UserTgID: 42, GroupTopicID: 77, CreatedAt: testCreatedAt, UpdatedAt: testUpdatedAt,
			}, *msg)
		})
	}
}

func TestGroupMessageRepository_GetSingle_NotFoundReturnsErrNoRows(t *testing.T) {
	for _, tc := range groupMessageGetCases() {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewGroupMessageRepository(db)

			mock.ExpectQuery(exactSQL(tc.sql)).WithArgs(tc.arg).WillReturnRows(sqlmock.NewRows(groupMessageColumns))

			msg, err := tc.call(repo)

			assert.Nil(t, msg)
			// Callers compare with == sql.ErrNoRows, so the sentinel must be returned unwrapped
			assert.Equal(t, sql.ErrNoRows, err)
		})
	}
}

func TestGroupMessageRepository_GetSingle_QueryError(t *testing.T) {
	for _, tc := range groupMessageGetCases() {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewGroupMessageRepository(db)

			mock.ExpectQuery(exactSQL(tc.sql)).WithArgs(tc.arg).WillReturnError(errDB)

			msg, err := tc.call(repo)

			require.Error(t, err)
			assert.Nil(t, msg)
			assert.ErrorIs(t, err, errDB)
			assert.Contains(t, err.Error(), tc.errSubstr)
		})
	}
}

func TestGroupMessageRepository_GetSingle_ScanError(t *testing.T) {
	for _, tc := range groupMessageGetCases() {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewGroupMessageRepository(db)

			mock.ExpectQuery(exactSQL(tc.sql)).WithArgs(tc.arg).
				WillReturnRows(sqlmock.NewRows(groupMessageColumns).
					AddRow(9, int64(1001), "hello", "not-a-number", int64(42), int64(77), testCreatedAt, testUpdatedAt))

			msg, err := tc.call(repo)

			require.Error(t, err)
			assert.Nil(t, msg)
			assert.NotEqual(t, sql.ErrNoRows, err)
			assert.Contains(t, err.Error(), tc.errSubstr)
		})
	}
}

// groupMessageListCase describes a multi-row lookup method
type groupMessageListCase struct {
	name         string
	sql          string
	args         []driver.Value
	call         func(r *GroupMessageRepository) ([]*GroupMessage, error)
	queryErrText string
}

func groupMessageListCases() []groupMessageListCase {
	return []groupMessageListCase{
		{
			name:         "GetByUserTgID",
			sql:          groupMessageByUserTgIDSQL,
			args:         []driver.Value{int64(42), 20, 40},
			call:         func(r *GroupMessageRepository) ([]*GroupMessage, error) { return r.GetByUserTgID(42, 20, 40) },
			queryErrText: "failed to get group messages by user telegram ID 42",
		},
		{
			name:         "GetAllByGroupTopicID",
			sql:          groupMessageByTopicAllSQL,
			args:         []driver.Value{int64(77)},
			call:         func(r *GroupMessageRepository) ([]*GroupMessage, error) { return r.GetAllByGroupTopicID(77) },
			queryErrText: "failed to get all group messages by group topic ID 77",
		},
		{
			name: "GetByGroupTopicIdForpreviousTwentyFourHours",
			sql:  groupMessageByTopicLast24h,
			args: []driver.Value{int64(77)},
			call: func(r *GroupMessageRepository) ([]*GroupMessage, error) {
				return r.GetByGroupTopicIdForpreviousTwentyFourHours(77)
			},
			queryErrText: "failed to get group messages by group topic ID 77 for last day",
		},
	}
}

func TestGroupMessageRepository_List_Success(t *testing.T) {
	for _, tc := range groupMessageListCases() {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewGroupMessageRepository(db)

			mock.ExpectQuery(exactSQL(tc.sql)).WithArgs(tc.args...).
				WillReturnRows(sqlmock.NewRows(groupMessageColumns).
					AddRow(1, int64(101), "first", nil, int64(42), int64(77), testCreatedAt, testCreatedAt).
					AddRow(2, int64(102), "second", int64(101), int64(43), int64(77), testUpdatedAt, testUpdatedAt))

			msgs, err := tc.call(repo)

			require.NoError(t, err)
			require.Len(t, msgs, 2)
			// Each element must be a distinct struct (no loop-variable aliasing)
			assert.NotSame(t, msgs[0], msgs[1])
			assert.Equal(t, GroupMessage{
				ID: 1, MessageID: 101, MessageText: "first", ReplyToMessageID: nil,
				UserTgID: 42, GroupTopicID: 77, CreatedAt: testCreatedAt, UpdatedAt: testCreatedAt,
			}, *msgs[0])
			assert.Equal(t, GroupMessage{
				ID: 2, MessageID: 102, MessageText: "second", ReplyToMessageID: int64Ptr(101),
				UserTgID: 43, GroupTopicID: 77, CreatedAt: testUpdatedAt, UpdatedAt: testUpdatedAt,
			}, *msgs[1])
		})
	}
}

func TestGroupMessageRepository_List_Empty(t *testing.T) {
	for _, tc := range groupMessageListCases() {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewGroupMessageRepository(db)

			mock.ExpectQuery(exactSQL(tc.sql)).WithArgs(tc.args...).WillReturnRows(sqlmock.NewRows(groupMessageColumns))

			msgs, err := tc.call(repo)

			require.NoError(t, err)
			assert.Empty(t, msgs)
		})
	}
}

func TestGroupMessageRepository_List_QueryError(t *testing.T) {
	for _, tc := range groupMessageListCases() {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewGroupMessageRepository(db)

			mock.ExpectQuery(exactSQL(tc.sql)).WithArgs(tc.args...).WillReturnError(errDB)

			msgs, err := tc.call(repo)

			require.Error(t, err)
			assert.Nil(t, msgs)
			assert.ErrorIs(t, err, errDB)
			assert.Contains(t, err.Error(), tc.queryErrText)
		})
	}
}

func TestGroupMessageRepository_List_ScanError(t *testing.T) {
	for _, tc := range groupMessageListCases() {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewGroupMessageRepository(db)

			mock.ExpectQuery(exactSQL(tc.sql)).WithArgs(tc.args...).
				WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

			msgs, err := tc.call(repo)

			require.Error(t, err)
			assert.Nil(t, msgs)
			assert.Contains(t, err.Error(), "failed to scan group message")
		})
	}
}

func TestGroupMessageRepository_List_RowsError(t *testing.T) {
	for _, tc := range groupMessageListCases() {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewGroupMessageRepository(db)

			mock.ExpectQuery(exactSQL(tc.sql)).WithArgs(tc.args...).
				WillReturnRows(sqlmock.NewRows(groupMessageColumns).
					AddRow(1, int64(101), "first", nil, int64(42), int64(77), testCreatedAt, testCreatedAt).
					AddRow(2, int64(102), "second", nil, int64(42), int64(77), testCreatedAt, testCreatedAt).
					RowError(1, errDB))

			msgs, err := tc.call(repo)

			require.Error(t, err)
			assert.Nil(t, msgs)
			assert.ErrorIs(t, err, errDB)
			assert.Contains(t, err.Error(), "error iterating group message rows")
		})
	}
}

// groupMessageExecCase describes an UPDATE/DELETE method that requires exactly one affected row
type groupMessageExecCase struct {
	name        string
	sql         string
	args        []driver.Value
	call        func(r *GroupMessageRepository) error
	execErrText string
	notFound    string
	raErrText   string
}

func groupMessageExecCases() []groupMessageExecCase {
	return []groupMessageExecCase{
		{
			name:        "Update",
			sql:         groupMessageUpdateSQL,
			args:        []driver.Value{"edited", 9},
			call:        func(r *GroupMessageRepository) error { return r.Update(9, "edited") },
			execErrText: "failed to update group message with ID 9",
			notFound:    "no group message found with ID 9 to update",
			raErrText:   "could not get rows affected after update",
		},
		{
			name:        "Delete",
			sql:         groupMessageDeleteSQL,
			args:        []driver.Value{9},
			call:        func(r *GroupMessageRepository) error { return r.Delete(9) },
			execErrText: "failed to delete group message with ID 9",
			notFound:    "no group message found with ID 9 to delete",
			raErrText:   "could not get rows affected after delete",
		},
		{
			name:        "DeleteByMessageID",
			sql:         groupMessageDeleteByMsgIDSQL,
			args:        []driver.Value{int64(1001)},
			call:        func(r *GroupMessageRepository) error { return r.DeleteByMessageID(1001) },
			execErrText: "failed to delete group message with message ID 1001",
			notFound:    "no group message found with message ID 1001 to delete",
			raErrText:   "could not get rows affected after delete",
		},
	}
}

func TestGroupMessageRepository_ExecMethods(t *testing.T) {
	rowsAffectedErr := errors.New("rows affected unsupported")

	for _, tc := range groupMessageExecCases() {
		t.Run(tc.name+"/success", func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			mock.ExpectExec(exactSQL(tc.sql)).WithArgs(tc.args...).WillReturnResult(sqlmock.NewResult(0, 1))

			assert.NoError(t, tc.call(NewGroupMessageRepository(db)))
		})

		t.Run(tc.name+"/exec error", func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			mock.ExpectExec(exactSQL(tc.sql)).WithArgs(tc.args...).WillReturnError(errDB)

			err := tc.call(NewGroupMessageRepository(db))

			require.Error(t, err)
			assert.ErrorIs(t, err, errDB)
			assert.Contains(t, err.Error(), tc.execErrText)
		})

		t.Run(tc.name+"/not found", func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			mock.ExpectExec(exactSQL(tc.sql)).WithArgs(tc.args...).WillReturnResult(sqlmock.NewResult(0, 0))

			err := tc.call(NewGroupMessageRepository(db))

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.notFound)
		})

		t.Run(tc.name+"/rows affected error", func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			mock.ExpectExec(exactSQL(tc.sql)).WithArgs(tc.args...).
				WillReturnResult(sqlmock.NewErrorResult(rowsAffectedErr))

			err := tc.call(NewGroupMessageRepository(db))

			require.Error(t, err)
			assert.ErrorIs(t, err, rowsAffectedErr)
			assert.Contains(t, err.Error(), tc.raErrText)
		})
	}
}
