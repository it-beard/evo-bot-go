package privatehandlers

import (
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventsHandler_NonPrivateChatIsRejected(t *testing.T) {
	d := newDeps(t)

	require.True(t, d.dispatch(t, d.newEventsHandler(), groupText(testUserID, "/events")))

	assertPrivateOnlyRejection(t, d.client)
}

func TestEventsHandler_NonClubMemberIsRejected(t *testing.T) {
	d := newDeps(t)
	d.client.SetMemberStatus(testUserID, "left")

	require.True(t, d.dispatch(t, d.newEventsHandler(), privateText(testUserID, "/events")))

	assertClubOnlyRejection(t, d.client)
}

func TestEventsHandler_DatabaseError(t *testing.T) {
	d := newDeps(t)
	d.sql.ExpectQuery(selectActiveEvents).WithArgs("actual", 10).WillReturnError(errors.New("db down"))

	require.True(t, d.dispatch(t, d.newEventsHandler(), privateText(testUserID, "/events")))

	assert.Equal(t, []string{"Ошибка при получении списка мероприятий."}, d.client.SentTexts())
}

func TestEventsHandler_NoActualEvents(t *testing.T) {
	d := newDeps(t)
	d.sql.ExpectQuery(selectActiveEvents).WithArgs("actual", 10).WillReturnRows(sqlmock.NewRows(eventColumns))

	require.True(t, d.dispatch(t, d.newEventsHandler(), privateText(testUserID, "/events")))

	assert.Equal(t, []string{"На данный момент нет актуальных мероприятий."}, d.client.SentTexts())
}

func TestEventsHandler_ListsActualEvents(t *testing.T) {
	d := newDeps(t)
	startedAt := time.Date(2020, 3, 4, 18, 30, 0, 0, time.UTC)
	d.sql.ExpectQuery(selectActiveEvents).WithArgs("actual", 10).WillReturnRows(
		sqlmock.NewRows(eventColumns).
			AddRow(1, "Go Meetup", "meetup", "actual", startedAt, testTime, testTime).
			AddRow(2, "Book Club", "reading-club", "actual", nil, testTime, testTime),
	)

	require.True(t, d.dispatch(t, d.newEventsHandler(), privateText(testUserID, "/events")))

	sends := d.client.CallsTo("sendMessage")
	require.Len(t, sends, 1)
	reply := sends[0]
	text := reply.Params["text"]
	assert.Equal(t, testUserID, reply.Int64("chat_id"))
	assert.Equal(t, "Markdown", reply.Params["parse_mode"])
	assert.Contains(t, reply.Params["reply_parameters"], `"message_id":`)

	assert.Contains(t, text, "📋 Список ближайших мероприятий:")
	assert.Contains(t, text, "🎙 _митап_: *Go Meetup*")
	assert.Contains(t, text, "_когда_: 04.03.2020 в 18:30 UTC")
	assert.Contains(t, text, "📚 _книжный клуб_: *Book Club*")
	assert.Contains(t, text, "_когда_: не указано")
	assert.Contains(t, text, "Добавить темы и вопросы /topicAdd.")
	assert.Contains(t, text, "Просмотреть темы и вопросы /topics.")
	assert.Contains(t, text, "[клубном календаре](https://itbeard.com/s/evo-calendar)")
}
