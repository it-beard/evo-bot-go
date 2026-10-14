package adminhandlers

import (
	"testing"

	"evo-bot-go/internal/buttons"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	topicsCancelledText = "Операция просмотра/удаления тем отменена."
	deleteTopicPrompt   = "\nДля удаления темы отправь ID темы, которую нужно удалить:"
	deleteMorePrompt    = "\nДля удаления еще одной темы отправь ID темы:"
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

// openEventList starts the conversation with one event in the list and returns the list message ID
func (e *testEnv) openEventList() int64 {
	e.t.Helper()
	e.mock.ExpectQuery(sqlLastEvents).WithArgs(10).WillReturnRows(eventRows(sampleEvent()))
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
	e := newShowTopicsEnv(t)
	const groupChatID int64 = -100600

	require.NoError(t, e.dispatch(testutils.MessageUpdate(testutils.NewGroupMessage(testAdminID, groupChatID, 0, showTopicsCommand()))))

	sends := e.client.CallsTo("sendMessage")
	require.Len(t, sends, 2)
	assert.Equal(t, groupChatID, sends[0].Int64("chat_id"))
	assert.Contains(t, sends[0].Params["text"], "Эта команда работает только в _личной беседе_ со мной")
	assert.Equal(t, testAdminID, sends[1].Int64("chat_id"))
	e.notHandled(testutils.MessageUpdate(testutils.NewGroupMessage(testAdminID, groupChatID, 0, "5")))
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
				e.mock.ExpectQuery(sqlLastEvents).WithArgs(10).WillReturnError(errDB)
			},
			wantReply: "Ошибка при получении списка мероприятий.",
		},
		{
			name: "no events",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlLastEvents).WithArgs(10).WillReturnRows(eventRows())
			},
			wantReply: "Нет доступных мероприятий для просмотра тем и вопросов.",
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
	e.mock.ExpectQuery(sqlLastEvents).WithArgs(10).WillReturnRows(eventRows(sampleEvent(), other))

	cmd := e.mustText(showTopicsCommand())

	call := e.lastSendMessage()
	assert.Equal(t, testAdminID, call.Int64("chat_id"))
	assert.Equal(t, "Markdown", call.Params["parse_mode"])
	text := call.Params["text"]
	assert.Contains(t, text, "*Список мероприятий*")
	assert.Contains(t, text, "🎙 ID /5: *Go Meetup*\n└ ✅ _когда_: *14.03.2025 в 18:30 UTC*")
	assert.Contains(t, text, "💬 ID /6: *Club call*\n└ 🔄 _когда_: *не указано*")
	assert.Contains(t, text, "Пожалуйста, отправь ID мероприятия, для которого ты хочешь увидеть темы и вопросы.")
	assert.Equal(t, cancelMarkup(), markup(t, call))
	var reply gotgbot.ReplyParameters
	require.NoError(t, call.DecodeJSON("reply_parameters", &reply))
	assert.Equal(t, cmd.MessageId, reply.MessageId)
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
			wantReply: "Пожалуйста, отправь корректный ID мероприятия или используй /cancel для отмены.",
		},
		{
			name:  "unknown event",
			input: "/42",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlEventByID).WithArgs(42).WillReturnRows(eventRows())
			},
			wantReply: "Не удалось найти мероприятие с ID 42. Пожалуйста, проверь ID.",
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

	assert.Equal(t, []string{"Ошибка при получении тем для выбранного мероприятия."}, e.client.SentTexts())
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
	assert.Contains(t, sends[0].Params["text"], "🎙 <i>Мероприятие (митап):</i> Go Meetup")
	assert.Contains(t, sends[0].Params["text"], "Для этого мероприятия пока нет тем и вопросов.")
	assert.Equal(t, []int64{listID}, e.keyboardRemovedFrom())
	e.notHandled(testutils.MessageUpdate(testutils.NewPrivateMessage(testAdminID, "7")))
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
	assert.Equal(t, "✅ Тема с ID 7 успешно удалена.", texts[0])
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
	assert.Equal(t, "✅ Тема с ID 8 успешно удалена.", texts[0])
	assert.Contains(t, texts[1], "Для этого мероприятия пока нет тем и вопросов.")
	assert.Equal(t, "Все темы удалены.", texts[2])
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
			wantReply: "Пожалуйста, отправь корректный ID темы или используй /cancel для отмены.",
		},
		{
			name:  "unknown topic",
			input: "99",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlTopicByID).WithArgs(99).WillReturnRows(topicRows())
			},
			wantReply: "Не удалось найти тему с ID 99. Пожалуйста, проверь ID.",
		},
		{
			name:  "topic of another event",
			input: "/9",
			setup: func(e *testEnv) {
				e.mock.ExpectQuery(sqlTopicByID).WithArgs(9).WillReturnRows(topicRows(foreignTopic))
			},
			wantReply: "Тема с ID 9 относится к другому мероприятию (ID: 6), а не к выбранному (ID: 5).\n" +
				"Пожалуйста, выбери корректный ID темы или используй /cancel для отмены.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			wantReplies: []string{"Ошибка при удалении темы с ID 7."},
		},
		{
			name: "topic disappeared before delete",
			setup: func(e *testEnv) {
				e.mock.ExpectExec(sqlDeleteTopic).WithArgs(7).WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantReplies: []string{"Ошибка при удалении темы с ID 7."},
		},
		{
			name: "reloading topics fails",
			setup: func(e *testEnv) {
				e.mock.ExpectExec(sqlDeleteTopic).WithArgs(7).WillReturnResult(sqlmock.NewResult(0, 1))
				e.mock.ExpectQuery(sqlTopicsByEventID).WithArgs(5).WillReturnError(errDB)
			},
			wantReplies: []string{"✅ Тема с ID 7 успешно удалена.", "Ошибка при получении обновленного списка тем."},
		},
		{
			name: "reloading event fails",
			setup: func(e *testEnv) {
				e.mock.ExpectExec(sqlDeleteTopic).WithArgs(7).WillReturnResult(sqlmock.NewResult(0, 1))
				e.mock.ExpectQuery(sqlTopicsByEventID).WithArgs(5).WillReturnRows(topicRows())
				e.mock.ExpectQuery(sqlEventByID).WithArgs(5).WillReturnError(errDB)
			},
			wantReplies: []string{"✅ Тема с ID 7 успешно удалена.", "Ошибка при получении информации о мероприятии."},
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
			answers := e.client.CallsTo("answerCallbackQuery")
			if u.CallbackQuery != nil {
				require.Len(t, answers, 1)
				assert.Equal(t, u.CallbackQuery.Id, answers[0].Params["callback_query_id"])
			} else {
				assert.Empty(t, answers)
			}
			e.notHandled(testutils.MessageUpdate(testutils.NewPrivateMessage(testAdminID, "7")))
		})
	}
}
