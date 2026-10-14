package repositories

import (
	"database/sql/driver"
	"errors"
	"testing"

	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	eventInsertSQL              = `INSERT INTO events (name, type, status) VALUES ($1, $2, $3) RETURNING id`
	eventInsertWithStartedAtSQL = `INSERT INTO events (name, type, status, started_at) VALUES ($1, $2, $3, $4) RETURNING id`
	eventLastActualSQL          = `
		SELECT id, name, type, status, started_at, created_at, updated_at
		FROM events
		WHERE status = $1
		ORDER BY started_at ASC NULLS LAST
		LIMIT $2`
	eventLastSQL = `
		SELECT id, name, type, status, started_at, created_at, updated_at
		FROM events
		ORDER BY started_at DESC NULLS LAST
		LIMIT $1`
	eventByIDSQL = `
		SELECT id, name, type, status, started_at, created_at, updated_at
		FROM events
		WHERE id = $1`
	eventDeleteSQL       = `DELETE FROM events WHERE id = $1`
	topicsByEventIDSQL   = `SELECT id, topic, user_nickname, event_id, created_at FROM topics WHERE event_id = $1 ORDER BY created_at ASC`
	topicDeleteSQL       = `DELETE FROM topics WHERE id = $1`
	eventUpdateNameSQL   = `UPDATE events SET name = $1, updated_at = NOW() WHERE id = $2`
	eventUpdateStatusSQL = `UPDATE events SET status = $1, updated_at = NOW() WHERE id = $2`
	eventUpdateStartSQL  = `UPDATE events SET started_at = $1, updated_at = NOW() WHERE id = $2`
	eventUpdateTypeSQL   = `UPDATE events SET type = $1, updated_at = NOW() WHERE id = $2`
)

var (
	eventColumns = []string{"id", "name", "type", "status", "started_at", "created_at", "updated_at"}
	topicColumns = []string{"id", "topic", "user_nickname", "event_id", "created_at"}
)

func TestNewEventRepository_WiresTopicRepository(t *testing.T) {
	db, _ := testutils.NewMockDB(t)

	repo := NewEventRepository(db)

	require.NotNil(t, repo.topicRepository)
	assert.Same(t, db, repo.db)
	assert.Same(t, db, repo.topicRepository.db)
}

func TestEventRepository_CreateEvent_Success(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewEventRepository(db)

	mock.ExpectQuery(exactSQL(eventInsertSQL)).
		WithArgs("Go meetup", "meetup", "actual").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(17))

	id, err := repo.CreateEvent("Go meetup", constants.EventTypeMeetup)

	require.NoError(t, err)
	assert.Equal(t, 17, id)
}

func TestEventRepository_CreateEvent_QueryError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewEventRepository(db)

	mock.ExpectQuery(exactSQL(eventInsertSQL)).
		WithArgs("Go meetup", "meetup", "actual").
		WillReturnError(errDB)

	id, err := repo.CreateEvent("Go meetup", constants.EventTypeMeetup)

	require.Error(t, err)
	assert.Zero(t, id)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to insert event")
}

func TestEventRepository_CreateEvent_ScanError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewEventRepository(db)

	mock.ExpectQuery(exactSQL(eventInsertSQL)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("not-a-number"))

	id, err := repo.CreateEvent("Go meetup", constants.EventTypeMeetup)

	require.Error(t, err)
	assert.Zero(t, id)
	assert.Contains(t, err.Error(), "failed to insert event")
}

func TestEventRepository_CreateEventWithStartedAt_Success(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewEventRepository(db)

	mock.ExpectQuery(exactSQL(eventInsertWithStartedAtSQL)).
		WithArgs("Club call #5", "club-call", "actual", testStartedAt).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))

	id, err := repo.CreateEventWithStartedAt("Club call #5", constants.EventTypeClubCall, testStartedAt)

	require.NoError(t, err)
	assert.Equal(t, 3, id)
}

func TestEventRepository_CreateEventWithStartedAt_QueryError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewEventRepository(db)

	mock.ExpectQuery(exactSQL(eventInsertWithStartedAtSQL)).
		WithArgs("Club call #5", "club-call", "actual", testStartedAt).
		WillReturnError(errDB)

	id, err := repo.CreateEventWithStartedAt("Club call #5", constants.EventTypeClubCall, testStartedAt)

	require.Error(t, err)
	assert.Zero(t, id)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to insert event with started_at")
}

// eventListCase describes one of the list methods returning []Event
type eventListCase struct {
	name string
	sql  string
	args []driver.Value
	call func(r *EventRepository) ([]Event, error)
}

func eventListCases() []eventListCase {
	return []eventListCase{
		{
			name: "GetLastActualEvents",
			sql:  eventLastActualSQL,
			args: []driver.Value{"actual", 5},
			call: func(r *EventRepository) ([]Event, error) { return r.GetLastActualEvents(5) },
		},
		{
			name: "GetLastEvents",
			sql:  eventLastSQL,
			args: []driver.Value{5},
			call: func(r *EventRepository) ([]Event, error) { return r.GetLastEvents(5) },
		},
	}
}

func TestEventRepository_ListEvents_Success(t *testing.T) {
	for _, tc := range eventListCases() {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewEventRepository(db)

			rows := sqlmock.NewRows(eventColumns).
				AddRow(1, "Club call", "club-call", "actual", testStartedAt, testCreatedAt, testUpdatedAt).
				AddRow(2, "Workshop", "workshop", "finished", nil, testCreatedAt, testUpdatedAt)
			mock.ExpectQuery(exactSQL(tc.sql)).WithArgs(tc.args...).WillReturnRows(rows)

			events, err := tc.call(repo)

			require.NoError(t, err)
			require.Len(t, events, 2)

			require.NotNil(t, events[0].StartedAt)
			assert.Equal(t, Event{
				ID: 1, Name: "Club call", Type: "club-call", Status: "actual",
				StartedAt: events[0].StartedAt, CreatedAt: testCreatedAt, UpdatedAt: testUpdatedAt,
			}, events[0])
			assert.True(t, testStartedAt.Equal(*events[0].StartedAt))

			assert.Equal(t, Event{
				ID: 2, Name: "Workshop", Type: "workshop", Status: "finished",
				StartedAt: nil, CreatedAt: testCreatedAt, UpdatedAt: testUpdatedAt,
			}, events[1])
		})
	}
}

func TestEventRepository_ListEvents_Empty(t *testing.T) {
	for _, tc := range eventListCases() {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewEventRepository(db)

			mock.ExpectQuery(exactSQL(tc.sql)).WithArgs(tc.args...).WillReturnRows(sqlmock.NewRows(eventColumns))

			events, err := tc.call(repo)

			require.NoError(t, err)
			assert.Empty(t, events)
		})
	}
}

func TestEventRepository_ListEvents_QueryError(t *testing.T) {
	for _, tc := range eventListCases() {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewEventRepository(db)

			mock.ExpectQuery(exactSQL(tc.sql)).WithArgs(tc.args...).WillReturnError(errDB)

			events, err := tc.call(repo)

			require.Error(t, err)
			assert.Nil(t, events)
			assert.ErrorIs(t, err, errDB)
			assert.Contains(t, err.Error(), "failed to query last events")
		})
	}
}

func TestEventRepository_ListEvents_ScanError(t *testing.T) {
	for _, tc := range eventListCases() {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewEventRepository(db)

			// Too few columns for the 7 scan destinations
			mock.ExpectQuery(exactSQL(tc.sql)).WithArgs(tc.args...).
				WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "Club call"))

			events, err := tc.call(repo)

			require.Error(t, err)
			assert.Nil(t, events)
			assert.Contains(t, err.Error(), "failed to scan event row")
		})
	}
}

func TestEventRepository_ListEvents_RowsError(t *testing.T) {
	for _, tc := range eventListCases() {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewEventRepository(db)

			rows := sqlmock.NewRows(eventColumns).
				AddRow(1, "Club call", "club-call", "actual", nil, testCreatedAt, testUpdatedAt).
				AddRow(2, "Workshop", "workshop", "actual", nil, testCreatedAt, testUpdatedAt).
				RowError(1, errDB)
			mock.ExpectQuery(exactSQL(tc.sql)).WithArgs(tc.args...).WillReturnRows(rows)

			events, err := tc.call(repo)

			require.Error(t, err)
			assert.Nil(t, events)
			assert.ErrorIs(t, err, errDB)
			assert.Contains(t, err.Error(), "error during rows iteration for events")
		})
	}
}

// eventUpdateCase describes one of the single-column UPDATE methods
type eventUpdateCase struct {
	name        string
	sql         string
	args        []driver.Value
	call        func(r *EventRepository) error
	execErrMsg  string
	notFoundMsg string
}

func eventUpdateCases() []eventUpdateCase {
	return []eventUpdateCase{
		{
			name:        "UpdateEventName",
			sql:         eventUpdateNameSQL,
			args:        []driver.Value{"New name", 9},
			call:        func(r *EventRepository) error { return r.UpdateEventName(9, "New name") },
			execErrMsg:  "failed to update event name for ID 9",
			notFoundMsg: "no event found with ID 9 to update",
		},
		{
			name:        "UpdateEventStatus",
			sql:         eventUpdateStatusSQL,
			args:        []driver.Value{"finished", 9},
			call:        func(r *EventRepository) error { return r.UpdateEventStatus(9, constants.EventStatusFinished) },
			execErrMsg:  "failed to update event status for ID 9",
			notFoundMsg: "no event found with ID 9 to update status",
		},
		{
			name:        "UpdateEventStartedAt",
			sql:         eventUpdateStartSQL,
			args:        []driver.Value{testStartedAt, 9},
			call:        func(r *EventRepository) error { return r.UpdateEventStartedAt(9, testStartedAt) },
			execErrMsg:  "failed to update event started_at for ID 9",
			notFoundMsg: "no event found with ID 9 to update started_at",
		},
		{
			name:        "UpdateEventType",
			sql:         eventUpdateTypeSQL,
			args:        []driver.Value{"reading-club", 9},
			call:        func(r *EventRepository) error { return r.UpdateEventType(9, constants.EventTypeReadingClub) },
			execErrMsg:  "failed to update event type for ID 9",
			notFoundMsg: "no event found with ID 9 to update type",
		},
	}
}

func TestEventRepository_UpdateMethods_Success(t *testing.T) {
	for _, tc := range eventUpdateCases() {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewEventRepository(db)

			mock.ExpectExec(exactSQL(tc.sql)).WithArgs(tc.args...).WillReturnResult(sqlmock.NewResult(0, 1))

			assert.NoError(t, tc.call(repo))
		})
	}
}

func TestEventRepository_UpdateMethods_ExecError(t *testing.T) {
	for _, tc := range eventUpdateCases() {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewEventRepository(db)

			mock.ExpectExec(exactSQL(tc.sql)).WithArgs(tc.args...).WillReturnError(errDB)

			err := tc.call(repo)

			require.Error(t, err)
			assert.ErrorIs(t, err, errDB)
			assert.Contains(t, err.Error(), tc.execErrMsg)
		})
	}
}

func TestEventRepository_UpdateMethods_NotFound(t *testing.T) {
	for _, tc := range eventUpdateCases() {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewEventRepository(db)

			mock.ExpectExec(exactSQL(tc.sql)).WithArgs(tc.args...).WillReturnResult(sqlmock.NewResult(0, 0))

			err := tc.call(repo)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.notFoundMsg)
		})
	}
}

func TestEventRepository_UpdateMethods_RowsAffectedErrorIsIgnored(t *testing.T) {
	for _, tc := range eventUpdateCases() {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutils.NewMockDB(t)
			repo := NewEventRepository(db)

			mock.ExpectExec(exactSQL(tc.sql)).WithArgs(tc.args...).
				WillReturnResult(sqlmock.NewErrorResult(errors.New("rows affected unsupported")))

			// The repository only logs RowsAffected failures and treats the update as successful
			assert.NoError(t, tc.call(repo))
		})
	}
}

func TestEventRepository_DeleteEvent_DeletesTopicsThenEvent(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewEventRepository(db)

	mock.ExpectQuery(exactSQL(topicsByEventIDSQL)).WithArgs(4).
		WillReturnRows(sqlmock.NewRows(topicColumns).
			AddRow(11, "Topic A", "alice", 4, testCreatedAt).
			AddRow(12, "Topic B", nil, 4, testCreatedAt))
	mock.ExpectExec(exactSQL(topicDeleteSQL)).WithArgs(11).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(exactSQL(topicDeleteSQL)).WithArgs(12).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(exactSQL(eventDeleteSQL)).WithArgs(4).WillReturnResult(sqlmock.NewResult(0, 1))

	assert.NoError(t, repo.DeleteEvent(4))
}

func TestEventRepository_DeleteEvent_NoTopics(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewEventRepository(db)

	mock.ExpectQuery(exactSQL(topicsByEventIDSQL)).WithArgs(4).WillReturnRows(sqlmock.NewRows(topicColumns))
	mock.ExpectExec(exactSQL(eventDeleteSQL)).WithArgs(4).WillReturnResult(sqlmock.NewResult(0, 1))

	assert.NoError(t, repo.DeleteEvent(4))
}

func TestEventRepository_DeleteEvent_GetTopicsError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewEventRepository(db)

	mock.ExpectQuery(exactSQL(topicsByEventIDSQL)).WithArgs(4).WillReturnError(errDB)

	err := repo.DeleteEvent(4)

	require.Error(t, err)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to get topics for event ID 4")
}

func TestEventRepository_DeleteEvent_DeleteTopicError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewEventRepository(db)

	mock.ExpectQuery(exactSQL(topicsByEventIDSQL)).WithArgs(4).
		WillReturnRows(sqlmock.NewRows(topicColumns).
			AddRow(11, "Topic A", "alice", 4, testCreatedAt).
			AddRow(12, "Topic B", "bob", 4, testCreatedAt))
	mock.ExpectExec(exactSQL(topicDeleteSQL)).WithArgs(11).WillReturnError(errDB)

	err := repo.DeleteEvent(4)

	require.Error(t, err)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to delete related topic with ID 11")
}

func TestEventRepository_DeleteEvent_ExecError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewEventRepository(db)

	mock.ExpectQuery(exactSQL(topicsByEventIDSQL)).WithArgs(4).WillReturnRows(sqlmock.NewRows(topicColumns))
	mock.ExpectExec(exactSQL(eventDeleteSQL)).WithArgs(4).WillReturnError(errDB)

	err := repo.DeleteEvent(4)

	require.Error(t, err)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to delete event with ID 4")
}

func TestEventRepository_DeleteEvent_NotFound(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewEventRepository(db)

	mock.ExpectQuery(exactSQL(topicsByEventIDSQL)).WithArgs(4).WillReturnRows(sqlmock.NewRows(topicColumns))
	mock.ExpectExec(exactSQL(eventDeleteSQL)).WithArgs(4).WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.DeleteEvent(4)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no event found with ID 4 to delete")
}

func TestEventRepository_DeleteEvent_RowsAffectedErrorIsIgnored(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewEventRepository(db)

	mock.ExpectQuery(exactSQL(topicsByEventIDSQL)).WithArgs(4).WillReturnRows(sqlmock.NewRows(topicColumns))
	mock.ExpectExec(exactSQL(eventDeleteSQL)).WithArgs(4).
		WillReturnResult(sqlmock.NewErrorResult(errors.New("rows affected unsupported")))

	assert.NoError(t, repo.DeleteEvent(4))
}

func TestEventRepository_GetEventByID_Success(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewEventRepository(db)

	mock.ExpectQuery(exactSQL(eventByIDSQL)).WithArgs(8).
		WillReturnRows(sqlmock.NewRows(eventColumns).
			AddRow(8, "Conf", "conference", "actual", testStartedAt, testCreatedAt, testUpdatedAt))

	event, err := repo.GetEventByID(8)

	require.NoError(t, err)
	require.NotNil(t, event)
	assert.Equal(t, 8, event.ID)
	assert.Equal(t, "Conf", event.Name)
	assert.Equal(t, "conference", event.Type)
	assert.Equal(t, "actual", event.Status)
	require.NotNil(t, event.StartedAt)
	assert.True(t, testStartedAt.Equal(*event.StartedAt))
	assert.Equal(t, testCreatedAt, event.CreatedAt)
	assert.Equal(t, testUpdatedAt, event.UpdatedAt)
}

func TestEventRepository_GetEventByID_NullStartedAt(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewEventRepository(db)

	mock.ExpectQuery(exactSQL(eventByIDSQL)).WithArgs(8).
		WillReturnRows(sqlmock.NewRows(eventColumns).
			AddRow(8, "Conf", "conference", "actual", nil, testCreatedAt, testUpdatedAt))

	event, err := repo.GetEventByID(8)

	require.NoError(t, err)
	require.NotNil(t, event)
	assert.Nil(t, event.StartedAt)
}

func TestEventRepository_GetEventByID_NotFound(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewEventRepository(db)

	mock.ExpectQuery(exactSQL(eventByIDSQL)).WithArgs(8).WillReturnRows(sqlmock.NewRows(eventColumns))

	event, err := repo.GetEventByID(8)

	require.Error(t, err)
	assert.Nil(t, event)
	assert.Contains(t, err.Error(), "no event found with ID 8")
}

func TestEventRepository_GetEventByID_QueryError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewEventRepository(db)

	mock.ExpectQuery(exactSQL(eventByIDSQL)).WithArgs(8).WillReturnError(errDB)

	event, err := repo.GetEventByID(8)

	require.Error(t, err)
	assert.Nil(t, event)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to get event with ID 8")
}

func TestEventRepository_GetEventByID_ScanError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewEventRepository(db)

	mock.ExpectQuery(exactSQL(eventByIDSQL)).WithArgs(8).
		WillReturnRows(sqlmock.NewRows(eventColumns).
			AddRow(8, "Conf", "conference", "actual", "not-a-time", testCreatedAt, testUpdatedAt))

	event, err := repo.GetEventByID(8)

	require.Error(t, err)
	assert.Nil(t, event)
	assert.Contains(t, err.Error(), "failed to get event with ID 8")
}
