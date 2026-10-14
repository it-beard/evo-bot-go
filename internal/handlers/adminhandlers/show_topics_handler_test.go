package adminhandlers

import (
	"errors"
	"strings"
	"testing"

	"evo-bot-go/internal/buttons"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/formatters"
	"evo-bot-go/internal/testutils"
	"evo-bot-go/internal/utils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	topicsCancelledText = "Операция просмотра и удаления тем отменена."
	deleteTopicPrompt   = "Чтобы удалить тему, отправь её ID или /cancel для отмены."
	deleteMorePrompt    = "Чтобы удалить ещё одну тему, отправь её ID или /cancel для отмены."
	adminEventListTitle = "📋 Последние мероприятия"
	showTopicsAction    = "для которого ты хочешь увидеть темы и вопросы"
	// sampleEventHeader is the escaped topic list header of sampleEvent
	sampleEventHeader = "🎙 <i>митап</i>: <b>Go Meetup</b>\n"
)

func showTopicsCommand() string {
	return "/" + constants.ShowTopicsCommand
}

func sampleEvent() repositories.Event {
	startedAt := fixedTime
	return repositories.Event{ID: 5, Name: "Go Meetup", Type: "meetup", Status: "finished", StartedAt: &startedAt}
}

func sampleTopics() []repositories.Topic {
	nickname := "alice"
	return []repositories.Topic{
		{ID: 7, Topic: "How to test Go?", UserNickname: &nickname, EventID: 5},
		{ID: 8, Topic: "Generics in practice", EventID: 5},
	}
}

func cancelMarkup() gotgbot.InlineKeyboardMarkup {
	return buttons.CancelButton(showTopicsCallbackConfirmCancel)
}

// expectLastEvents expects the query for the events list of /showTopics
func (e *testEnv) expectLastEvents() *sqlmock.ExpectedQuery {
	return e.mock.ExpectQuery(sqlLastEvents).WithArgs(constants.EventEditGetLastLimit)
}

// openEventList starts the conversation with one event in the list and returns the list message ID
func (e *testEnv) openEventList() int64 {
	e.t.Helper()
	e.expectLastEvents().WillReturnRows(eventRows(sampleEvent()))
	e.mustText(showTopicsCommand())
	e.client.Reset()
	return e.lastSentID()
}

// openTopicDeletion selects the sample event with the given topics and returns the deletion prompt message ID
func (e *testEnv) openTopicDeletion(topics ...repositories.Topic) int64 {
	e.t.Helper()
	e.openEventList()
	e.mock.ExpectQuery(sqlEventByID).WithArgs(5).WillReturnRows(eventRows(sampleEvent()))
	e.mock.ExpectQuery(sqlTopicsByEventID).WithArgs(5).WillReturnRows(topicRows(topics...))
	e.mustText("/5")
	e.client.Reset()
	return e.lastSentID()
}

// keyboardRemovedFrom returns the IDs of messages whose inline keyboard was removed
func (e *testEnv) keyboardRemovedFrom() []int64 {
	var ids []int64
	for _, c := range e.client.CallsTo("editMessageReplyMarkup") {
		assert.Equal(e.t, testAdminID, c.Int64("chat_id"))
		ids = append(ids, c.Int64("message_id"))
	}
	return ids
}

// newBareShowTopicsHandler builds the handler without the conversation wrapper, so its stored state can be inspected
func (e *testEnv) newBareShowTopicsHandler() *showTopicsHandler {
	return &showTopicsHandler{
		config:               e.cfg,
		topicRepository:      repositories.NewTopicRepository(e.db),
		eventRepository:      repositories.NewEventRepository(e.db),
		messageSenderService: e.sender,
		userStore:            utils.NewUserDataStore(),
		permissionsService:   e.permissions,
	}
}

// assertShowTopicsStateCleared checks that nothing is left in the handler's store for the admin
func assertShowTopicsStateCleared(t *testing.T, h *showTopicsHandler) {
	t.Helper()
	_, ok := h.userStore.Get(testAdminID, showTopicsCtxDataKeyEventID)
	assert.False(t, ok, "the selected event must be forgotten")
	msgID, chatID := h.userStore.GetPreviousMessageInfo(testAdminID,
		showTopicsCtxDataKeyPreviousMessageID, showTopicsCtxDataKeyPreviousChatID)
	assert.Zero(t, msgID, "the previous message must be forgotten")
	assert.Zero(t, chatID, "the previous message must be forgotten")
}

// assertConversationEnded checks that a handler returned handlers.EndConversation()
func assertConversationEnded(t *testing.T, err error) {
	t.Helper()
	var change *handlers.ConversationStateChange
	require.True(t, errors.As(err, &change), "expected a conversation state change, got %v", err)
	assert.True(t, change.End, "the conversation must end")
}

func TestShowTopicsHandler_RejectsNonAdmin(t *testing.T) {
	e := newShowTopicsEnv(t)

	require.NoError(t, e.dispatch(testutils.MessageUpdate(testutils.NewPrivateMessage(testMemberID, showTopicsCommand()))))

	assert.Equal(t, []string{"Эта команда доступна только администраторам."}, e.client.SentTexts())
	call, ok := e.client.LastCallTo("getChatMember")
	require.True(t, ok)
	assert.Equal(t, fullGroupChatID(), call.Int64("chat_id"))
	e.notHandled(testutils.MessageUpdate(testutils.NewPrivateMessage(testMemberID, "5")))
}

func TestShowTopicsHandler_RejectsGroupChat(t *testing.T) {
	tests := []struct {
		name   string
		userID int64
	}{
		{name: "admin", userID: testAdminID},
		{name: "member gets the same notice instead of the admin-only reply", userID: testMemberID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newShowTopicsEnv(t)
			const groupChatID int64 = -100600
			cmd := testutils.NewGroupMessage(tt.userID, groupChatID, 0, showTopicsCommand())

			require.NoError(t, e.dispatch(testutils.MessageUpdate(cmd)))

			e.assertPrivateOnlyNotice(cmd)
			e.notHandled(testutils.MessageUpdate(testutils.NewGroupMessage(tt.userID, groupChatID, 0, "5")))
		})
	}
}

func TestShowTopicsHandler_StartWithoutEvents(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(e *testEnv)
		wantReply string
	}{
		{
			name: "events query fails",
			setup: func(e *testEnv) {
				e.expectLastEvents().WillReturnError(errDB)
			},
			wantReply: formatters.EventListErrorText,
		},
		{
			name: "no events",
			setup: func(e *testEnv) {
				e.expectLastEvents().WillReturnRows(eventRows())
			},
			wantReply: "Пока нет ни одного мероприятия. Создать новое можно командой /eventSetup.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newShowTopicsEnv(t)
			tt.setup(e)

			cmd := e.mustText(showTopicsCommand())

			assert.Equal(t, []string{tt.wantReply}, e.client.SentTexts())
			var reply gotgbot.ReplyParameters
			require.NoError(t, e.lastSendMessage().DecodeJSON("reply_parameters", &reply))
			assert.Equal(t, cmd.MessageId, reply.MessageId)
			e.notHandled(testutils.MessageUpdate(testutils.NewPrivateMessage(testAdminID, "5")))
		})
	}
}

func TestShowTopicsHandler_StartListsEvents(t *testing.T) {
	e := newShowTopicsEnv(t)
	other := repositories.Event{ID: 6, Name: "Club call", Type: "club-call", Status: "actual"}
	e.expectLastEvents().WillReturnRows(eventRows(sampleEvent(), other))

	cmd := e.mustText(showTopicsCommand())

	call := e.lastSendMessage()
	assert.Equal(t, testAdminID, call.Int64("chat_id"))
	assert.Equal(t, "HTML", call.Params["parse_mode"])
	text := call.Params["text"]
	// The shared admin event list layout is used
	assert.Equal(t, formatters.FormatEventListForAdmin(
		[]repositories.Event{sampleEvent(), other}, adminEventListTitle, showTopicsAction,
	), text)
	assert.True(t, strings.HasPrefix(text, "<b>"+adminEventListTitle+"</b>\n"), text)
	assert.Contains(t, text, "\n✅ 🎙 <i>митап</i>: <b>Go Meetup</b>\n└ <i>ID</i> /5, <i>когда</i>: 14.03.2025 в 18:30 UTC\n")
	assert.Contains(t, text, "\n🔄 💬 <i>клубный созвон</i>: <b>Club call</b>\n└ <i>ID</i> /6, <i>когда</i>: не указано\n")
	assert.Contains(t, text, "<i>🔄 — актуальное, ✅ — завершено</i>")
	assert.True(t, strings.HasSuffix(text, "\n\n👉 Нажми на ID мероприятия, для которого ты хочешь увидеть темы и вопросы."), text)
	assert.Equal(t, cancelMarkup(), markup(t, call))
	var reply gotgbot.ReplyParameters
	require.NoError(t, call.DecodeJSON("reply_parameters", &reply))
	assert.Equal(t, cmd.MessageId, reply.MessageId)
}

func TestShowTopicsHandler_EventNamesAreHTMLEscaped(t *testing.T) {
	e := newShowTopicsEnv(t)
	event := sampleEvent()
	event.Name = "C++ <templates> & Go"
	const escapedName = "<b>C++ &lt;templates&gt; &amp; Go</b>"
	topics := sampleTopics()

	// The event list
	e.expectLastEvents().WillReturnRows(eventRows(event))
	e.mustText(showTopicsCommand())
	listText := e.lastSendMessage().Params["text"]
	assert.Contains(t, listText, escapedName)
	assert.NotContains(t, listText, "<templates>")

	// The topic list header after the event is selected
	e.client.Reset()
	e.mock.ExpectQuery(sqlEventByID).WithArgs(5).WillReturnRows(eventRows(event))
	e.mock.ExpectQuery(sqlTopicsByEventID).WithArgs(5).WillReturnRows(topicRows(topics...))
	e.mustText("/5")
	texts := e.client.SentTexts()
	require.Len(t, texts, 2)
	assert.True(t, strings.HasPrefix(texts[0], "🎙 <i>митап</i>: "+escapedName+"\n"), texts[0])

	// The updated topic list header after a deletion
	e.client.Reset()
	e.mock.ExpectQuery(sqlTopicByID).WithArgs(7).WillReturnRows(topicRows(topics[0]))
	e.mock.ExpectExec(sqlDeleteTopic).WithArgs(7).WillReturnResult(sqlmock.NewResult(0, 1))
	e.mock.ExpectQuery(sqlTopicsByEventID).WithArgs(5).WillReturnRows(topicRows(topics[1]))
	e.mock.ExpectQuery(sqlEventByID).WithArgs(5).WillReturnRows(eventRows(event))
	e.mustText("7")
	texts = e.client.SentTexts()
	require.Len(t, texts, 3)
	assert.True(t, strings.HasPrefix(texts[1], "🎙 <i>митап</i>: "+escapedName+"\n"), texts[1])
}

// BUG: topic texts come from club members (/topicAdd) but FormatHtmlTopicListForAdmin puts them into the HTML
// message unescaped, so a topic like "Generics <T>" makes Telegram reject the whole topic list.
func TestShowTopicsHandler_BUG_TopicTextIsNotHTMLEscaped(t *testing.T) {
	e := newShowTopicsEnv(t)
	e.openEventList()
	topic := repositories.Topic{ID: 7, Topic: "Generics <T> & constraints", EventID: 5}
	e.mock.ExpectQuery(sqlEventByID).WithArgs(5).WillReturnRows(eventRows(sampleEvent()))
	e.mock.ExpectQuery(sqlTopicsByEventID).WithArgs(5).WillReturnRows(topicRows(topic))

	e.mustText("/5")

	texts := e.client.SentTexts()
	require.NotEmpty(t, texts)
	assert.Contains(t, texts[0], "<blockquote expandable>Generics &lt;T&gt; &amp; constraints</blockquote>")
	assert.NotContains(t, texts[0], "<T>")
}

func TestShowTopicsHandler_EventListNotSentStillWaitsForEventID(t *testing.T) {
	e := newShowTopicsEnv(t)
	e.setFailSend(failAdminSends)
	e.expectLastEvents().WillReturnRows(eventRows(sampleEvent()))

	// The list cannot be sent: no panic, nothing is remembered as the previous message
	e.mustText(showTopicsCommand())

	// Telegram is back: the admin can still pick an event
	e.setFailSend(nil)
	e.client.Reset()
	e.mock.ExpectQuery(sqlEventByID).WithArgs(5).WillReturnRows(eventRows(sampleEvent()))
	e.mock.ExpectQuery(sqlTopicsByEventID).WithArgs(5).WillReturnRows(topicRows())
	e.mustText("/5")

	assert.Contains(t, e.lastSentText(), "Для этого мероприятия пока нет тем и вопросов.")
	assert.Empty(t, e.keyboardRemovedFrom())
}

func TestShowTopicsHandler_EventSelectionInvalidInput(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		setup     func(e *testEnv)
		wantReply string
	}{
		{
			name:      "not a number",
			input:     "abc",
			setup:     func(e *testEnv) {},
			wantReply: formatters.InvalidEventIDText,
		},
		{
			name:  "unknown event",
			input: "/42",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlEventByID).WithArgs(42).WillReturnRows(eventRows())
			},
			wantReply: formatters.EventNotFoundText(42),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newShowTopicsEnv(t)
			e.openEventList()

			// The admin stays in the event selection state and can retry
			for attempt := 0; attempt < 2; attempt++ {
				e.client.Reset()
				tt.setup(e)

				input := e.mustText(tt.input)

				assert.Equal(t, []string{tt.wantReply}, e.client.SentTexts(), "attempt %d", attempt)
				var reply gotgbot.ReplyParameters
				require.NoError(t, e.lastSendMessage().DecodeJSON("reply_parameters", &reply))
				assert.Equal(t, input.MessageId, reply.MessageId)
				assert.Empty(t, e.keyboardRemovedFrom())
			}
		})
	}
}

func TestShowTopicsHandler_EventSelectionTopicsQueryFails(t *testing.T) {
	e := newShowTopicsEnv(t)
	e.openEventList()
	e.mock.ExpectQuery(sqlEventByID).WithArgs(5).WillReturnRows(eventRows(sampleEvent()))
	e.mock.ExpectQuery(sqlTopicsByEventID).WithArgs(5).WillReturnError(errDB)

	e.mustText("5")

	assert.Equal(t, []string{"Произошла ошибка при получении тем и вопросов для выбранного мероприятия."}, e.client.SentTexts())
	e.notHandled(testutils.MessageUpdate(testutils.NewPrivateMessage(testAdminID, "5")))
}

func TestShowTopicsHandler_EventWithoutTopics(t *testing.T) {
	e := newShowTopicsEnv(t)
	listID := e.openEventList()
	e.mock.ExpectQuery(sqlEventByID).WithArgs(5).WillReturnRows(eventRows(sampleEvent()))
	e.mock.ExpectQuery(sqlTopicsByEventID).WithArgs(5).WillReturnRows(topicRows())

	e.mustText("/5")

	sends := e.client.CallsTo("sendMessage")
	require.Len(t, sends, 1)
	assert.Equal(t, "HTML", sends[0].Params["parse_mode"])
	assert.Equal(t, sampleEventHeader+"\nДля этого мероприятия пока нет тем и вопросов.", sends[0].Params["text"])
	assert.Equal(t, []int64{listID}, e.keyboardRemovedFrom())
	e.notHandled(testutils.MessageUpdate(testutils.NewPrivateMessage(testAdminID, "7")))
}

func TestShowTopicsHandler_EventWithoutTopicsClearsState(t *testing.T) {
	e := newShowTopicsEnv(t)
	h := e.newBareShowTopicsHandler()
	// State left by the event list step
	h.SavePreviousMessageInfo(testAdminID, &gotgbot.Message{MessageId: 4242, Chat: gotgbot.Chat{Id: testAdminID}})
	e.mock.ExpectQuery(sqlEventByID).WithArgs(5).WillReturnRows(eventRows(sampleEvent()))
	e.mock.ExpectQuery(sqlTopicsByEventID).WithArgs(5).WillReturnRows(topicRows())
	msg := testutils.NewPrivateMessage(testAdminID, "/5")

	err := h.handleEventSelection(e.bot, ext.NewContext(e.bot, testutils.MessageUpdate(msg), nil))

	assertConversationEnded(t, err)
	assertShowTopicsStateCleared(t, h)
	assert.Equal(t, []int64{4242}, e.keyboardRemovedFrom())
}

func TestShowTopicsHandler_TopicDeletionWithoutSelectedEvent(t *testing.T) {
	tests := []struct {
		name    string
		eventID any // nil: nothing stored
	}{
		{name: "no event stored"},
		{name: "event stored with a wrong type", eventID: "5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newShowTopicsEnv(t)
			h := e.newBareShowTopicsHandler()
			h.SavePreviousMessageInfo(testAdminID, &gotgbot.Message{MessageId: 4242, Chat: gotgbot.Chat{Id: testAdminID}})
			if tt.eventID != nil {
				h.userStore.Set(testAdminID, showTopicsCtxDataKeyEventID, tt.eventID)
			}
			msg := testutils.NewPrivateMessage(testAdminID, "7")

			err := h.handleTopicDeletion(e.bot, ext.NewContext(e.bot, testutils.MessageUpdate(msg), nil))

			assertConversationEnded(t, err)
			assert.Equal(t, []string{formatters.LostEventText(constants.ShowTopicsCommand)}, e.client.SentTexts())
			assert.Equal(t, "Произошла ошибка: не удалось найти выбранное мероприятие. Пожалуйста, начни заново с /showTopics.",
				e.lastSentText())
			assertShowTopicsStateCleared(t, h)
		})
	}
}

func TestShowTopicsHandler_DeleteTopicsUntilNoneLeft(t *testing.T) {
	e := newShowTopicsEnv(t)
	topics := sampleTopics()
	listID := e.openEventList()

	// Select the event: topics are listed and deletion is offered
	e.mock.ExpectQuery(sqlEventByID).WithArgs(5).WillReturnRows(eventRows(sampleEvent()))
	e.mock.ExpectQuery(sqlTopicsByEventID).WithArgs(5).WillReturnRows(topicRows(topics...))
	e.mustText("/5")

	sends := e.client.CallsTo("sendMessage")
	require.Len(t, sends, 2)
	assert.Equal(t, "HTML", sends[0].Params["parse_mode"])
	assert.True(t, strings.HasPrefix(sends[0].Params["text"], sampleEventHeader+"📋 Найдено тем и вопросов: <b>2</b>\n\n"), sends[0].Params["text"])
	assert.Contains(t, sends[0].Params["text"], "ID:<code>7</code> / <i>14.03.2025</i> / @alice \n<blockquote expandable>How to test Go?</blockquote>")
	assert.Contains(t, sends[0].Params["text"], "ID:<code>8</code> / <i>14.03.2025</i> / не указано \n<blockquote expandable>Generics in practice</blockquote>")
	assert.Equal(t, deleteTopicPrompt, sends[1].Params["text"])
	assert.Equal(t, testAdminID, sends[1].Int64("chat_id"))
	assert.Equal(t, cancelMarkup(), markup(t, sends[1]))
	assert.Equal(t, []int64{listID}, e.keyboardRemovedFrom())
	promptID := e.lastSentID()

	// Delete the first topic: the remaining one is listed again
	e.client.Reset()
	e.mock.ExpectQuery(sqlTopicByID).WithArgs(7).WillReturnRows(topicRows(topics[0]))
	e.mock.ExpectExec(sqlDeleteTopic).WithArgs(7).WillReturnResult(sqlmock.NewResult(0, 1))
	e.mock.ExpectQuery(sqlTopicsByEventID).WithArgs(5).WillReturnRows(topicRows(topics[1]))
	e.mock.ExpectQuery(sqlEventByID).WithArgs(5).WillReturnRows(eventRows(sampleEvent()))
	e.mustText("7")

	texts := e.client.SentTexts()
	require.Len(t, texts, 3)
	assert.Equal(t, "✅ Тема с ID 7 удалена.", texts[0])
	assert.True(t, strings.HasPrefix(texts[1], sampleEventHeader+"📋 Найдено тем и вопросов: <b>1</b>\n\n"), texts[1])
	assert.NotContains(t, texts[1], "ID:<code>7</code>")
	assert.Contains(t, texts[1], "ID:<code>8</code>")
	assert.Equal(t, deleteMorePrompt, texts[2])
	assert.Equal(t, cancelMarkup(), markup(t, e.lastSendMessage()))
	assert.Equal(t, []int64{promptID}, e.keyboardRemovedFrom())
	promptID = e.lastSentID()

	// Delete the last topic: the conversation ends
	e.client.Reset()
	e.mock.ExpectQuery(sqlTopicByID).WithArgs(8).WillReturnRows(topicRows(topics[1]))
	e.mock.ExpectExec(sqlDeleteTopic).WithArgs(8).WillReturnResult(sqlmock.NewResult(0, 1))
	e.mock.ExpectQuery(sqlTopicsByEventID).WithArgs(5).WillReturnRows(topicRows())
	e.mock.ExpectQuery(sqlEventByID).WithArgs(5).WillReturnRows(eventRows(sampleEvent()))
	e.mustText("/8")

	texts = e.client.SentTexts()
	require.Len(t, texts, 3)
	assert.Equal(t, "✅ Тема с ID 8 удалена.", texts[0])
	assert.Equal(t, sampleEventHeader+"\nДля этого мероприятия пока нет тем и вопросов.", texts[1])
	assert.Equal(t, "✅ Все темы этого мероприятия удалены.", texts[2])
	assert.Equal(t, []int64{promptID}, e.keyboardRemovedFrom())
	e.notHandled(testutils.MessageUpdate(testutils.NewPrivateMessage(testAdminID, "8")))
}

func TestShowTopicsHandler_TopicDeletionInvalidInput(t *testing.T) {
	foreignTopic := repositories.Topic{ID: 9, Topic: "Other", EventID: 6}

	tests := []struct {
		name      string
		input     string
		setup     func(e *testEnv)
		wantReply string
	}{
		{
			name:      "not a number",
			input:     "seven",
			setup:     func(e *testEnv) {},
			wantReply: "Неверный ID темы. Пожалуйста, отправь числовой ID темы из списка выше или /cancel для отмены.",
		},
		{
			name:  "unknown topic",
			input: "99",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlTopicByID).WithArgs(99).WillReturnRows(topicRows())
			},
			wantReply: "Тема с ID 99 не найдена. Пожалуйста, отправь ID темы из списка выше или /cancel для отмены.",
		},
		{
			name:  "topic of another event",
			input: "/9",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlTopicByID).WithArgs(9).WillReturnRows(topicRows(foreignTopic))
			},
			wantReply: "Тема с ID 9 относится к другому мероприятию (ID 6), а не к выбранному (ID 5). " +
				"Пожалуйста, отправь ID темы из списка выше или /cancel для отмены.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Every "try again" message ends with the shared cancel hint
			require.True(t, strings.HasSuffix(tt.wantReply, " "+formatters.CancelHint+"."))
			e := newShowTopicsEnv(t)
			e.openTopicDeletion(sampleTopics()...)

			// Nothing is deleted and the admin can retry
			for attempt := 0; attempt < 2; attempt++ {
				e.client.Reset()
				tt.setup(e)

				e.mustText(tt.input)

				assert.Equal(t, []string{tt.wantReply}, e.client.SentTexts(), "attempt %d", attempt)
				assert.Empty(t, e.keyboardRemovedFrom())
			}
		})
	}
}

func TestShowTopicsHandler_TopicDeletionErrors(t *testing.T) {
	topic := sampleTopics()[0]

	tests := []struct {
		name        string
		setup       func(e *testEnv)
		wantReplies []string
	}{
		{
			name: "delete fails",
			setup: func(e *testEnv) {
				e.mock.ExpectExec(sqlDeleteTopic).WithArgs(7).WillReturnError(errDB)
			},
			wantReplies: []string{"Произошла ошибка при удалении темы с ID 7."},
		},
		{
			name: "topic disappeared before delete",
			setup: func(e *testEnv) {
				e.mock.ExpectExec(sqlDeleteTopic).WithArgs(7).WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantReplies: []string{"Произошла ошибка при удалении темы с ID 7."},
		},
		{
			name: "reloading topics fails",
			setup: func(e *testEnv) {
				e.mock.ExpectExec(sqlDeleteTopic).WithArgs(7).WillReturnResult(sqlmock.NewResult(0, 1))
				e.mock.ExpectQuery(sqlTopicsByEventID).WithArgs(5).WillReturnError(errDB)
			},
			wantReplies: []string{"✅ Тема с ID 7 удалена.", "Произошла ошибка при получении обновлённого списка тем."},
		},
		{
			name: "reloading event fails",
			setup: func(e *testEnv) {
				e.mock.ExpectExec(sqlDeleteTopic).WithArgs(7).WillReturnResult(sqlmock.NewResult(0, 1))
				e.mock.ExpectQuery(sqlTopicsByEventID).WithArgs(5).WillReturnRows(topicRows())
				e.mock.ExpectQuery(sqlEventByID).WithArgs(5).WillReturnError(errDB)
			},
			wantReplies: []string{"✅ Тема с ID 7 удалена.", "Произошла ошибка при получении информации о мероприятии."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newShowTopicsEnv(t)
			promptID := e.openTopicDeletion(sampleTopics()...)
			e.mock.ExpectQuery(sqlTopicByID).WithArgs(7).WillReturnRows(topicRows(topic))
			tt.setup(e)

			e.mustText("7")

			assert.Equal(t, tt.wantReplies, e.client.SentTexts())
			assert.Equal(t, []int64{promptID}, e.keyboardRemovedFrom())
			e.notHandled(testutils.MessageUpdate(testutils.NewPrivateMessage(testAdminID, "7")))
		})
	}
}

func TestShowTopicsHandler_Cancel(t *testing.T) {
	tests := []struct {
		name   string
		open   func(e *testEnv) int64
		cancel func(e *testEnv) *gotgbot.Update
	}{
		{
			name: "cancel button while selecting event",
			open: func(e *testEnv) int64 { return e.openEventList() },
			cancel: func(e *testEnv) *gotgbot.Update {
				return callbackUpdate(testAdminID, showTopicsCallbackConfirmCancel)
			},
		},
		{
			name: "cancel button while deleting topics",
			open: func(e *testEnv) int64 { return e.openTopicDeletion(sampleTopics()...) },
			cancel: func(e *testEnv) *gotgbot.Update {
				return callbackUpdate(testAdminID, showTopicsCallbackConfirmCancel)
			},
		},
		{
			name: "cancel command while selecting event",
			open: func(e *testEnv) int64 { return e.openEventList() },
			cancel: func(e *testEnv) *gotgbot.Update {
				return testutils.MessageUpdate(testutils.NewPrivateMessage(testAdminID, "/"+constants.CancelCommand))
			},
		},
		{
			name: "cancel command while deleting topics",
			open: func(e *testEnv) int64 { return e.openTopicDeletion(sampleTopics()...) },
			cancel: func(e *testEnv) *gotgbot.Update {
				return testutils.MessageUpdate(testutils.NewPrivateMessage(testAdminID, "/"+constants.CancelCommand))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newShowTopicsEnv(t)
			prevID := tt.open(e)
			u := tt.cancel(e)

			require.NoError(t, e.dispatch(u))

			assert.Equal(t, []string{topicsCancelledText}, e.client.SentTexts())
			assert.Equal(t, testAdminID, e.lastSendMessage().Int64("chat_id"))
			assert.Equal(t, []int64{prevID}, e.keyboardRemovedFrom())
			if u.CallbackQuery != nil {
				assert.Equal(t, []string{u.CallbackQuery.Id}, e.answeredCallbackIDs())
			} else {
				assert.Empty(t, e.answeredCallbackIDs())
			}
			e.notHandled(testutils.MessageUpdate(testutils.NewPrivateMessage(testAdminID, "7")))
		})
	}
}
