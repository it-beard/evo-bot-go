package topicshandlers

import (
	"errors"
	"strings"
	"testing"

	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/formatters"
	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const eventListTitle = "📋 Ближайшие мероприятия"

// eventSelectionSpec describes the shared "pick an event by ID" step of /topics and /topicAdd
type eventSelectionSpec struct {
	name              string
	command           string
	actionDescription string
	noEventsText      string
	cancelText        string
	cancelCallback    string
	newHandler        func(d *deps) ext.Handler
}

func eventSelectionSpecs() []eventSelectionSpec {
	return []eventSelectionSpec{
		{
			name:              "topics",
			command:           "/topics",
			actionDescription: "для которого ты хочешь увидеть темы и вопросы",
			noEventsText:      "Ближайших мероприятий пока нет, поэтому и тем с вопросами к ним тоже нет.",
			cancelText:        "Операция просмотра тем отменена.",
			cancelCallback:    topicsCallbackConfirmCancel,
			newHandler:        (*deps).newTopicsHandler,
		},
		{
			name:              "topicAdd",
			command:           "/topicAdd",
			actionDescription: "к которому ты хочешь добавить темы и вопросы",
			noEventsText:      "Ближайших мероприятий пока нет, поэтому добавить к ним темы и вопросы не получится.",
			cancelText:        "Операция добавления темы отменена.",
			cancelCallback:    topicAddCallbackConfirmCancel,
			newHandler:        (*deps).newTopicAddHandler,
		},
	}
}

// openEventList runs the command and returns the id of the event list message
func openEventList(t *testing.T, d *deps, h ext.Handler, spec eventSelectionSpec, events ...repositories.Event) int64 {
	t.Helper()
	if len(events) == 0 {
		events = testEvents
	}
	d.expectActualEvents(events...)
	require.True(t, d.dispatch(t, h, privateText(testUserID, spec.command)))
	return d.sentID(t, spec.actionDescription)
}

func TestEventSelection_NonPrivateChatIsRejected(t *testing.T) {
	for _, spec := range eventSelectionSpecs() {
		t.Run(spec.name, func(t *testing.T) {
			d := newDeps(t)
			h := spec.newHandler(d)

			require.True(t, d.dispatch(t, h, groupText(testUserID, spec.command)))

			assertPrivateOnlyRejection(t, d.client)
			assert.False(t, d.dispatch(t, h, groupText(testUserID, "7")), "no conversation was started")
		})
	}
}

func TestEventSelection_NonClubMemberIsRejected(t *testing.T) {
	for _, spec := range eventSelectionSpecs() {
		t.Run(spec.name, func(t *testing.T) {
			d := newDeps(t)
			d.client.SetMemberStatus(testUserID, "left")
			h := spec.newHandler(d)

			require.True(t, d.dispatch(t, h, privateText(testUserID, spec.command)))

			assertClubOnlyRejection(t, d.client)
			assert.False(t, d.dispatch(t, h, privateText(testUserID, "7")), "no conversation was started")
		})
	}
}

func TestEventSelection_EventsCannotBeListed(t *testing.T) {
	tests := []struct {
		name     string
		expect   func(d *deps)
		wantText func(spec eventSelectionSpec) string
	}{
		{
			name: "database error",
			expect: func(d *deps) {
				d.sql.ExpectQuery(selectActiveEventsSQL).WithArgs("actual", 10).WillReturnError(errors.New("db down"))
			},
			wantText: func(eventSelectionSpec) string {
				return formatters.EventListErrorText
			},
		},
		{
			name:     "no actual events",
			expect:   func(d *deps) { d.expectActualEvents() },
			wantText: func(spec eventSelectionSpec) string { return spec.noEventsText },
		},
	}

	for _, spec := range eventSelectionSpecs() {
		for _, tt := range tests {
			t.Run(spec.name+"/"+tt.name, func(t *testing.T) {
				d := newDeps(t)
				h := spec.newHandler(d)
				tt.expect(d)

				require.True(t, d.dispatch(t, h, privateText(testUserID, spec.command)))

				assert.Equal(t, []string{tt.wantText(spec)}, d.client.SentTexts())
				assert.Contains(t, lastSend(t, d.client).Params["reply_parameters"], `"message_id":`)
				assert.False(t, d.dispatch(t, h, privateText(testUserID, "7")), "conversation is ended")
			})
		}
	}
}

func TestEventSelection_ListsEvents(t *testing.T) {
	for _, spec := range eventSelectionSpecs() {
		t.Run(spec.name, func(t *testing.T) {
			d := newDeps(t)
			h := spec.newHandler(d)
			d.expectActualEvents(testEvents...)
			command := testutils.NewPrivateMessage(testUserID, spec.command)

			require.True(t, d.dispatch(t, h, testutils.MessageUpdate(command)))

			list := lastSend(t, d.client)
			text := list.Params["text"]
			assert.Equal(t, testUserID, list.Int64("chat_id"))
			assert.Equal(t, "HTML", list.Params["parse_mode"])
			assert.Contains(t, list.Params["reply_parameters"], `"message_id":`)
			assert.Equal(t, formatters.FormatEventListForTopicsView(testEvents, eventListTitle, spec.actionDescription), text)
			assert.True(t, strings.HasPrefix(text, "<b>"+eventListTitle+"</b>\n"), "the list starts with the bold title, got %q", text)
			assert.Contains(t, text, "\n🎙 <i>митап</i>: <b>Go Meetup</b>\n└ <i>ID</i> /7, <i>когда</i>: 04.03.2030 в 18:30 UTC\n")
			assert.Contains(t, text, "\n📚 <i>книжный клуб</i>: <b>Book Club</b>\n└ <i>ID</i> /8, <i>когда</i>: не указано\n")
			assert.Contains(t, text, "\n👉 Нажми на ID мероприятия, "+spec.actionDescription+".")
			assert.NotContains(t, text, "_ID_", "no Markdown leftovers")
			assert.Equal(t, []string{spec.cancelCallback}, callbackData(t, list))
		})
	}
}

func TestEventSelection_ListEscapesEventNames(t *testing.T) {
	start := meetupStart
	event := repositories.Event{ID: 11, Name: "C++ <Templates> & Go_Generics", Type: "meetup", Status: "actual",
		StartedAt: &start, CreatedAt: testTime, UpdatedAt: testTime}

	for _, spec := range eventSelectionSpecs() {
		t.Run(spec.name, func(t *testing.T) {
			d := newDeps(t)
			h := spec.newHandler(d)

			openEventList(t, d, h, spec, event)

			list := lastSend(t, d.client)
			assert.Equal(t, "HTML", list.Params["parse_mode"])
			assert.Contains(t, list.Params["text"], "<b>C++ &lt;Templates&gt; &amp; Go_Generics</b>")
			assert.NotContains(t, list.Params["text"], "<Templates>")
		})
	}
}

func TestEventSelection_ListDeliveryFailureDoesNotPanic(t *testing.T) {
	for _, spec := range eventSelectionSpecs() {
		t.Run(spec.name, func(t *testing.T) {
			d := newDeps(t)
			h := spec.newHandler(d)
			d.expectActualEvents(testEvents...)
			d.client.OnError("sendMessage", "Bad Request: can't parse entities")

			require.NotPanics(t, func() {
				require.True(t, d.dispatch(t, h, privateText(testUserID, spec.command)))
			})
			require.Len(t, d.client.CallsTo("sendMessage"), 1, "the list was attempted")

			// No list message was stored, so cancelling has no keyboard to remove
			d.client.On("sendMessage", d.sendMessageResponse)
			d.client.Reset()
			require.True(t, d.dispatch(t, h, privateText(testUserID, "/cancel")))
			assert.Equal(t, []string{spec.cancelText}, d.client.SentTexts())
			assert.Empty(t, d.client.CallsTo("editMessageReplyMarkup"))
		})
	}
}

func TestEventSelection_InvalidOrUnknownEventKeepsWaiting(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expect   func(d *deps)
		wantText string
	}{
		{
			name:     "not a number",
			input:    "мероприятие",
			wantText: formatters.InvalidEventIDText,
		},
		{
			name:  "unknown id",
			input: "/99",
			expect: func(d *deps) {
				d.sql.ExpectQuery(selectEventByIDSQL).WithArgs(99).WillReturnRows(sqlmock.NewRows(eventColumns))
			},
			wantText: formatters.EventNotFoundText(99),
		},
		{
			name:  "database error",
			input: "99",
			expect: func(d *deps) {
				d.sql.ExpectQuery(selectEventByIDSQL).WithArgs(99).WillReturnError(errors.New("db down"))
			},
			wantText: formatters.EventNotFoundText(99),
		},
	}

	for _, spec := range eventSelectionSpecs() {
		for _, tt := range tests {
			t.Run(spec.name+"/"+tt.name, func(t *testing.T) {
				d := newDeps(t)
				h := spec.newHandler(d)
				openEventList(t, d, h, spec)
				d.client.Reset()
				if tt.expect != nil {
					tt.expect(d)
				}

				require.True(t, d.dispatch(t, h, privateText(testUserID, tt.input)))

				assert.Equal(t, []string{tt.wantText}, d.client.SentTexts())
				assert.Contains(t, tt.wantText, "или /cancel для отмены")
				assert.Empty(t, d.client.CallsTo("editMessageReplyMarkup"), "the event list stays usable")

				// Still waiting for an event: cancelling works
				require.True(t, d.dispatch(t, h, privateText(testUserID, "/cancel")))
				assert.Equal(t, spec.cancelText, lastSend(t, d.client).Params["text"])
			})
		}
	}
}

func TestEventSelection_SharedTexts(t *testing.T) {
	assert.Equal(t, "Произошла ошибка при получении списка мероприятий.", formatters.EventListErrorText)
	assert.Equal(t, "Неверный ID. Пожалуйста, нажми на ID мероприятия из списка выше или /cancel для отмены.",
		formatters.InvalidEventIDText)
	assert.Equal(t, "Мероприятие с ID 99 не найдено. Пожалуйста, нажми на ID мероприятия из списка выше или /cancel для отмены.",
		formatters.EventNotFoundText(99))
}

func TestEventSelection_Cancel(t *testing.T) {
	for _, spec := range eventSelectionSpecs() {
		for _, byButton := range []bool{false, true} {
			name := spec.name + "/command"
			if byButton {
				name = spec.name + "/button"
			}
			t.Run(name, func(t *testing.T) {
				d := newDeps(t)
				h := spec.newHandler(d)
				listID := openEventList(t, d, h, spec)
				d.client.Reset()

				update := privateText(testUserID, "/cancel")
				if byButton {
					update = testutils.CallbackUpdate(testUserID, spec.cancelCallback)
				}
				require.True(t, d.dispatch(t, h, update))

				if byButton {
					assert.Len(t, d.client.CallsTo("answerCallbackQuery"), 1)
				}
				assert.Equal(t, []string{spec.cancelText}, d.client.SentTexts())
				assert.Equal(t, testUserID, lastSend(t, d.client).Int64("chat_id"))
				assertKeyboardRemoved(t, d.client, listID)
				assert.False(t, d.dispatch(t, h, privateText(testUserID, "7")), "conversation is ended")
			})
		}
	}
}

func TestEventSelection_IgnoredOutsideConversation(t *testing.T) {
	for _, spec := range eventSelectionSpecs() {
		t.Run(spec.name, func(t *testing.T) {
			d := newDeps(t)
			h := spec.newHandler(d)

			assert.False(t, d.dispatch(t, h, privateText(testUserID, "7")))
			assert.False(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, spec.cancelCallback)))
			assert.False(t, d.dispatch(t, h, privateText(testUserID, "/cancel")))
			assert.Empty(t, d.client.Calls())
		})
	}
}

// ---------- /topics specific ----------

func TestTopicsHandler_ShowsTopicsOfSelectedEvent(t *testing.T) {
	nick := "author"
	topics := []repositories.Topic{
		{ID: 1, Topic: "Как писать тесты?", UserNickname: &nick, EventID: 7, CreatedAt: testTime},
		{ID: 2, Topic: "Что нового в Go?", EventID: 7, CreatedAt: testTime},
	}

	for _, input := range []string{"/7", "7", " 7 "} {
		t.Run(input, func(t *testing.T) {
			d := newDeps(t)
			h := d.newTopicsHandler()
			spec := eventSelectionSpecs()[0]
			listID := openEventList(t, d, h, spec)
			d.client.Reset()
			d.expectEvent(testEvents[0])
			expectTopics(d, 7, topics...)
			selection := testutils.NewPrivateMessage(testUserID, input)

			require.True(t, d.dispatch(t, h, testutils.MessageUpdate(selection)))

			assertKeyboardRemoved(t, d.client, listID)
			reply := lastSend(t, d.client)
			assert.Equal(t, formatters.FormatHtmlTopicListForUsers(topics, "Go Meetup", "meetup"), reply.Params["text"])
			assert.Contains(t, reply.Params["text"], "🎙 <i>митап</i>: <b>Go Meetup</b>\n")
			assert.Contains(t, reply.Params["text"], "Найдено тем и вопросов: <b>2</b>")
			assert.Equal(t, "HTML", reply.Params["parse_mode"])
			assert.Contains(t, reply.Params["reply_parameters"], `"message_id":`)
			assert.False(t, d.dispatch(t, h, privateText(testUserID, "7")), "conversation is ended")
		})
	}
}

func TestTopicsHandler_EventWithoutTopics(t *testing.T) {
	d := newDeps(t)
	h := d.newTopicsHandler()
	openEventList(t, d, h, eventSelectionSpecs()[0])
	d.expectEvent(testEvents[1])
	expectTopics(d, 8)

	require.True(t, d.dispatch(t, h, privateText(testUserID, "8")))

	text := lastSend(t, d.client).Params["text"]
	assert.Contains(t, text, "📚 <i>книжный клуб</i>: <b>Book Club</b>")
	assert.Contains(t, text, "Для этого мероприятия пока нет тем и вопросов.")
}

func TestTopicsHandler_EscapesEventNameInTopicList(t *testing.T) {
	d := newDeps(t)
	h := d.newTopicsHandler()
	event := repositories.Event{ID: 12, Name: "Q&A <live>", Type: "club-call", Status: "actual", CreatedAt: testTime, UpdatedAt: testTime}
	openEventList(t, d, h, eventSelectionSpecs()[0], event)
	d.expectEvent(event)
	expectTopics(d, 12)

	require.True(t, d.dispatch(t, h, privateText(testUserID, "12")))

	reply := lastSend(t, d.client)
	assert.Equal(t, "HTML", reply.Params["parse_mode"])
	assert.Contains(t, reply.Params["text"], "<b>Q&amp;A &lt;live&gt;</b>")
	assert.NotContains(t, reply.Params["text"], "<live>")
}

// TestTopicsHandler_EscapesTopicText documents a SUSPECTED PRODUCTION BUG: /topicAdd stores the raw topic text
// and formatters.FormatHtmlTopicListForUsers puts it into the HTML message unescaped, so a topic like
// "Как работают <T any>?" makes Telegram reject the whole /topics reply ("can't parse entities").
func TestTopicsHandler_EscapesTopicText(t *testing.T) {
	d := newDeps(t)
	h := d.newTopicsHandler()
	openEventList(t, d, h, eventSelectionSpecs()[0])
	d.expectEvent(testEvents[0])
	expectTopics(d, 7, repositories.Topic{ID: 1, Topic: "Как работают <T any> & <b>дженерики</b>?", EventID: 7, CreatedAt: testTime})

	require.True(t, d.dispatch(t, h, privateText(testUserID, "7")))

	text := lastSend(t, d.client).Params["text"]
	assert.Contains(t, text, "Как работают &lt;T any&gt; &amp; &lt;b&gt;дженерики&lt;/b&gt;?")
	assert.NotContains(t, text, "<T any>")
}

func TestTopicsHandler_TopicsCannotBeLoaded(t *testing.T) {
	d := newDeps(t)
	h := d.newTopicsHandler()
	openEventList(t, d, h, eventSelectionSpecs()[0])
	d.client.Reset()
	d.expectEvent(testEvents[0])
	d.sql.ExpectQuery(selectTopicsSQL).WithArgs(7).WillReturnError(errors.New("db down"))

	require.True(t, d.dispatch(t, h, privateText(testUserID, "7")))

	assert.Equal(t, []string{"Произошла ошибка при получении тем и вопросов для выбранного мероприятия."}, d.client.SentTexts())
	assert.False(t, d.dispatch(t, h, privateText(testUserID, "7")), "conversation is ended")
}

func expectTopics(d *deps, eventID int, topics ...repositories.Topic) {
	rows := sqlmock.NewRows(topicColumns)
	for _, tp := range topics {
		rows.AddRow(tp.ID, tp.Topic, tp.UserNickname, tp.EventID, tp.CreatedAt)
	}
	d.sql.ExpectQuery(selectTopicsSQL).WithArgs(eventID).WillReturnRows(rows)
}
