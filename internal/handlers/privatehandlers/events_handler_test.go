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

	assert.Equal(t, []string{"Произошла ошибка при получении списка мероприятий."}, d.client.SentTexts())
}

func TestEventsHandler_NoUpcomingEventsPointsToCalendar(t *testing.T) {
	d := newDeps(t)
	d.sql.ExpectQuery(selectActiveEvents).WithArgs("actual", 10).WillReturnRows(sqlmock.NewRows(eventColumns))

	require.True(t, d.dispatch(t, d.newEventsHandler(), privateText(testUserID, "/events")))

	sends := d.client.CallsTo("sendMessage")
	require.Len(t, sends, 1)
	reply := sends[0]
	assert.Equal(t, testUserID, reply.Int64("chat_id"))
	assert.Equal(t, "HTML", reply.Params["parse_mode"])
	assert.Contains(t, reply.Params["reply_parameters"], `"message_id":`)
	assert.Equal(t,
		"Ближайших мероприятий пока нет. Следи за анонсами в <a href=\"https://itbeard.com/s/evo-calendar\">клубном календаре</a>.",
		reply.Params["text"])
}

func TestEventsHandler_ListsUpcomingEvents(t *testing.T) {
	d := newDeps(t)
	startedAt := time.Date(2030, 3, 4, 18, 30, 0, 0, time.UTC)
	d.sql.ExpectQuery(selectActiveEvents).WithArgs("actual", 10).WillReturnRows(
		sqlmock.NewRows(eventColumns).
			AddRow(1, "Go Meetup", "meetup", "actual", startedAt, testTime, testTime).
			AddRow(2, "Book Club", "reading-club", "actual", nil, testTime, testTime),
	)

	require.True(t, d.dispatch(t, d.newEventsHandler(), privateText(testUserID, "/events")))

	sends := d.client.CallsTo("sendMessage")
	require.Len(t, sends, 1)
	reply := sends[0]
	assert.Equal(t, testUserID, reply.Int64("chat_id"))
	assert.Equal(t, "HTML", reply.Params["parse_mode"])
	assert.Contains(t, reply.Params["reply_parameters"], `"message_id":`)

	// Members see neither event IDs nor statuses
	assert.Equal(t, "<b>📋 Ближайшие мероприятия</b>\n"+
		"\n🎙 <i>митап</i>: <b>Go Meetup</b>\n"+
		"└ <i>когда</i>: 04.03.2030 в 18:30 UTC\n"+
		"\n📚 <i>книжный клуб</i>: <b>Book Club</b>\n"+
		"└ <i>когда</i>: не указано\n"+
		"\nДобавить темы и вопросы — /topicAdd, посмотреть их — /topics."+
		"\nБольше информации о мероприятиях — в <a href=\"https://itbeard.com/s/evo-calendar\">клубном календаре</a>.",
		reply.Params["text"])
}

func TestEventsHandler_EventNamesAreEscaped(t *testing.T) {
	d := newDeps(t)
	d.sql.ExpectQuery(selectActiveEvents).WithArgs("actual", 10).WillReturnRows(
		sqlmock.NewRows(eventColumns).
			AddRow(1, "Generics <T> & snake_case_names", "meetup", "actual", nil, testTime, testTime),
	)

	require.True(t, d.dispatch(t, d.newEventsHandler(), privateText(testUserID, "/events")))

	reply := lastSend(t, d.client)
	text := reply.Params["text"]
	assert.Equal(t, "HTML", reply.Params["parse_mode"])
	assert.Contains(t, text, "🎙 <i>митап</i>: <b>Generics &lt;T&gt; &amp; snake_case_names</b>\n")
	assert.NotContains(t, text, "<T>")
}

func TestEventsHandler_ShowsCountdownForEventsWithinAWeek(t *testing.T) {
	d := newDeps(t)
	// 2 days and 30 minutes ahead, so the countdown is "2д 0ч" even if the test runs slowly
	startedAt := time.Now().UTC().Add(48*time.Hour + 30*time.Minute)
	d.sql.ExpectQuery(selectActiveEvents).WithArgs("actual", 10).WillReturnRows(
		sqlmock.NewRows(eventColumns).
			AddRow(1, "Club Call", "club-call", "actual", startedAt, testTime, testTime),
	)

	require.True(t, d.dispatch(t, d.newEventsHandler(), privateText(testUserID, "/events")))

	assert.Contains(t, lastSend(t, d.client).Params["text"],
		"└ <i>когда</i>: "+startedAt.Format("02.01.2006 в 15:04 UTC")+" <i>(через 2д 0ч)</i>\n")
}
