package eventhandlers

import (
	"errors"
	"regexp"
	"testing"

	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	startCanceledText = "Операция старта мероприятия отменена."
	testEventLink     = "https://zoom.us/j/123"
)

func startTestEvent(eventType constants.EventType, name string) repositories.Event {
	return repositories.Event{ID: 7, Name: name, Type: string(eventType), Status: "actual"}
}

// startStart runs /eventStart and returns the event list message
func startStart(t *testing.T, env *testEnv, ev repositories.Event) gotgbot.Message {
	t.Helper()
	env.expectGetLastEvents(ev)
	require.True(t, env.sendText(testAdminID, "/eventStart"))
	return env.lastSent().Msg
}

// selectEventForStart selects the event and returns the link prompt
func selectEventForStart(t *testing.T, env *testEnv, ev repositories.Event) gotgbot.Message {
	t.Helper()
	startStart(t, env, ev)
	env.expectGetEventByID(ev.ID, &ev)
	require.True(t, env.sendText(testAdminID, "/7"))
	return env.lastSent().Msg
}

// enterStartLink enters the event link and returns the confirmation message
func enterStartLink(t *testing.T, env *testEnv, ev repositories.Event) gotgbot.Message {
	t.Helper()
	selectEventForStart(t, env, ev)
	env.expectGetEventByID(ev.ID, &ev)
	require.True(t, env.sendText(testAdminID, testEventLink))
	return env.lastSent().Msg
}

func expectStatusFinished(mock sqlmock.Sqlmock) {
	mock.ExpectExec(regexp.QuoteMeta(sqlUpdateStatus)).
		WithArgs(constants.EventStatusFinished, 7).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestEventStartHandler_StartFailures(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(env *testEnv)
		wantText string
	}{
		{
			name: "database error",
			setup: func(env *testEnv) {
				env.mock.ExpectQuery(regexp.QuoteMeta(sqlGetLastEvents)).WillReturnError(errors.New("db down"))
			},
			wantText: "Произошла ошибка при получении списка актуальных мероприятий.",
		},
		{
			name:     "no events",
			setup:    func(env *testEnv) { env.expectGetLastEvents() },
			wantText: "Нет мероприятий для старта.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t, NewEventStartHandler)
			tc.setup(env)

			require.True(t, env.sendText(testAdminID, "/eventStart"))

			assert.Equal(t, []string{tc.wantText}, env.texts())
			assert.False(t, env.sendText(testAdminID, "/7"), "conversation must not be started")
		})
	}
}

func TestEventStartHandler_FullFlowAnnouncesAndPinsEvent(t *testing.T) {
	tests := []struct {
		name             string
		event            repositories.Event
		wantAnnouncement string
	}{
		{
			name:  "club call includes rules link",
			event: startTestEvent(constants.EventTypeClubCall, "Club Call"),
			wantAnnouncement: "🔴 *НАЧИНАЕМ ИВЕНТ!* 🔴\n\n💬 *Club Call*\n" +
				"💡 [Про формат и правила клубных созвонов](https://telegram.me/c/2069889012/127/33823)\n" +
				"\nИспользуй кнопку ниже, чтобы присоединиться ⬇️",
		},
		{
			name:             "meetup has no rules link",
			event:            startTestEvent(constants.EventTypeMeetup, "Go Meetup"),
			wantAnnouncement: "🔴 *НАЧИНАЕМ ИВЕНТ!* 🔴\n\n🎙 *Go Meetup*\n\nИспользуй кнопку ниже, чтобы присоединиться ⬇️",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t, NewEventStartHandler)
			ev := tc.event

			// 1. Event list
			listMsg := startStart(t, env, ev)
			listCall := env.lastSent().Call
			assert.Equal(t, "Markdown", listCall.Params["parse_mode"])
			assert.Contains(t, listMsg.Text, "ID /7: *"+ev.Name+"*")
			assert.Contains(t, listMsg.Text, "которое ты хочешь запустить")
			assert.Equal(t, []string{eventStartCallbackConfirmCancel}, callbackData(t, listCall))

			// 2. Non-numeric id keeps the state
			env.reset()
			require.True(t, env.sendText(testAdminID, "abc"))
			assert.Equal(t, []string{"Неверный ID. Пожалуйста, введи числовой ID или /cancel для отмены."}, env.texts())

			// 3. Selecting the event asks for the link
			env.reset()
			env.expectGetEventByID(7, &ev)
			require.True(t, env.sendText(testAdminID, "/7"))
			env.requireKeyboardRemoved(listMsg)
			linkPrompt := env.lastSent()
			assert.Equal(t,
				"🔗 Отправь мне ссылку на мероприятие '"+ev.Name+"' (ID: 7)\nЭта ссылка будет отправлена в чат объявлений.",
				linkPrompt.Msg.Text)
			assert.Equal(t, "Markdown", linkPrompt.Call.Params["parse_mode"])
			assert.Equal(t, []string{eventStartCallbackConfirmCancel}, callbackData(t, linkPrompt.Call))

			// 4. Something that is not a link keeps the state
			env.reset()
			require.True(t, env.sendText(testAdminID, "zoom.us/j/123"))
			assert.Equal(t, []string{
				"Пожалуйста, введи корректную ссылку, начинающуюся с http:// или https:// (или используй /cancel для отмены):",
			}, env.texts())

			// 5. A link asks for confirmation
			env.reset()
			env.expectGetEventByID(7, &ev)
			require.True(t, env.sendText(testAdminID, "  "+testEventLink+" "))
			env.requireKeyboardRemoved(linkPrompt.Msg)
			confirm := env.lastSent()
			assert.Equal(t,
				"*Подтверждение запуска мероприятия*\n\n🎯 *"+ev.Name+"* _(ID: 7)_\n\n🔗 Ссылка: `"+testEventLink+"`"+
					"\n\nЭта ссылка будет отправлена в чат объявлений.\n\nНажми на кнопку ниже для подтверждения или отмены",
				confirm.Msg.Text)
			assert.Equal(t, "Markdown", confirm.Call.Params["parse_mode"])
			assert.Equal(t, []string{eventStartCallbackConfirmYes, eventStartCallbackConfirmCancel}, callbackData(t, confirm.Call))

			// 6. Any message during confirmation (text or not) asks to use the buttons
			pleaseUseButtons := "Пожалуйста, нажмите на одну из кнопок выше, или используйте /cancel для отмены."
			env.reset()
			require.True(t, env.sendText(testAdminID, "ok"))
			sticker := testutils.NewPrivateMessage(testAdminID, "")
			sticker.Sticker = &gotgbot.Sticker{FileId: "sticker", FileUniqueId: "sticker", Type: "regular", Width: 1, Height: 1}
			require.True(t, env.dispatch(testutils.MessageUpdate(sticker)))
			assert.Equal(t, []string{pleaseUseButtons, pleaseUseButtons}, env.texts())

			// 7. Confirmation marks the event finished, announces it and pins the announcement
			env.reset()
			env.expectGetEventByID(7, &ev)
			expectStatusFinished(env.mock)
			require.True(t, env.click(testAdminID, eventStartCallbackConfirmYes))

			assert.Len(t, env.client.CallsTo("answerCallbackQuery"), 1)
			sent := env.sentMessages()
			require.Len(t, sent, 2)

			announcement := sent[0]
			assert.Equal(t, fullChatID(), announcement.Msg.Chat.Id)
			assert.Equal(t, int64(testAnnouncementTopicID), announcement.Call.Int64("message_thread_id"))
			assert.Equal(t, "Markdown", announcement.Call.Params["parse_mode"])
			assert.Equal(t, tc.wantAnnouncement, announcement.Msg.Text)
			buttons := inlineButtons(t, announcement.Call)
			require.Len(t, buttons, 1)
			assert.Equal(t, "🔗 Ссылка на Zoom", buttons[0].Text)
			assert.Equal(t, testEventLink, buttons[0].Url)

			pin, ok := env.client.LastCallTo("pinChatMessage")
			require.True(t, ok)
			assert.Equal(t, fullChatID(), pin.Int64("chat_id"))
			assert.Equal(t, announcement.Msg.MessageId, pin.Int64("message_id"))
			assert.Equal(t, "false", pin.Params["disable_notification"], "pin must notify members")

			assert.Equal(t, testAdminID, sent[1].Msg.Chat.Id)
			assert.Equal(t,
				"✅ *Мероприятие успешно запущено!*\n\n🎯 *"+ev.Name+"* _(ID: 7)_\n\n📢 Ссылка отправлена в чат объявлений.",
				sent[1].Msg.Text)
			assert.Equal(t, "Markdown", sent[1].Call.Params["parse_mode"])
			env.requireKeyboardRemoved(confirm.Msg)

			// 8. The conversation is over
			assert.False(t, env.click(testAdminID, eventStartCallbackConfirmYes))
		})
	}
}

func TestEventStartHandler_SelectUnknownEventEndsConversation(t *testing.T) {
	env := newTestEnv(t, NewEventStartHandler)
	startStart(t, env, startTestEvent(constants.EventTypeMeetup, "Go Meetup"))
	env.reset()

	env.expectGetEventByID(99, nil)
	require.True(t, env.sendText(testAdminID, "/99"))

	assert.Equal(t, []string{"Ошибка при получении мероприятия с ID 99"}, env.texts())
	assert.Empty(t, env.client.CallsTo("editMessageReplyMarkup"))
	assert.False(t, env.sendText(testAdminID, testEventLink), "conversation must be over")
}

func TestEventStartHandler_EnterLinkLookupFailureEndsConversation(t *testing.T) {
	env := newTestEnv(t, NewEventStartHandler)
	selectEventForStart(t, env, startTestEvent(constants.EventTypeMeetup, "Go Meetup"))
	env.reset()

	env.mock.ExpectQuery(regexp.QuoteMeta(sqlGetEventByID)).WithArgs(7).WillReturnError(errors.New("db down"))
	require.True(t, env.sendText(testAdminID, testEventLink))

	assert.Equal(t, []string{"Ошибка при получении мероприятия с ID 7"}, env.texts())
	assert.False(t, env.sendText(testAdminID, testEventLink), "conversation must be over")
}

func TestEventStartHandler_ConfirmFailuresDoNotAnnounce(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(env *testEnv, ev repositories.Event)
		wantText string
	}{
		{
			name: "event lookup fails",
			setup: func(env *testEnv, _ repositories.Event) {
				env.mock.ExpectQuery(regexp.QuoteMeta(sqlGetEventByID)).WithArgs(7).WillReturnError(errors.New("db down"))
			},
			wantText: "Ошибка при получении мероприятия с ID 7",
		},
		{
			name: "status update fails",
			setup: func(env *testEnv, ev repositories.Event) {
				env.expectGetEventByID(7, &ev)
				env.mock.ExpectExec(regexp.QuoteMeta(sqlUpdateStatus)).WillReturnError(errors.New("db down"))
			},
			wantText: "Произошла ошибка при обновлении статуса мероприятия.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t, NewEventStartHandler)
			ev := startTestEvent(constants.EventTypeMeetup, "Go Meetup")
			enterStartLink(t, env, ev)
			env.reset()

			tc.setup(env, ev)
			require.True(t, env.click(testAdminID, eventStartCallbackConfirmYes))

			assert.Equal(t, []string{tc.wantText}, env.texts())
			assert.Empty(t, env.client.CallsTo("pinChatMessage"))
			assert.False(t, env.click(testAdminID, eventStartCallbackConfirmYes), "conversation must be over")
		})
	}
}

func TestEventStartHandler_AnnouncementProblemsStillConfirmToAdmin(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(env *testEnv)
		wantPins  int
		wantSends int
	}{
		{
			name: "announcement cannot be sent",
			setup: func(env *testEnv) {
				env.failSendWhen(func(call testutils.APICall) bool { return call.Int64("chat_id") == fullChatID() })
			},
			wantPins:  0,
			wantSends: 1,
		},
		{
			name:      "announcement cannot be pinned",
			setup:     func(env *testEnv) { env.client.OnError("pinChatMessage", "Bad Request: not enough rights") },
			wantPins:  1,
			wantSends: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t, NewEventStartHandler)
			ev := startTestEvent(constants.EventTypeMeetup, "Go Meetup")
			confirmMsg := enterStartLink(t, env, ev)
			env.reset()

			tc.setup(env)
			env.expectGetEventByID(7, &ev)
			expectStatusFinished(env.mock)
			require.True(t, env.click(testAdminID, eventStartCallbackConfirmYes))

			assert.Len(t, env.client.CallsTo("pinChatMessage"), tc.wantPins)
			sent := env.sentMessages()
			require.Len(t, sent, tc.wantSends)
			last := sent[len(sent)-1]
			assert.Equal(t, testAdminID, last.Msg.Chat.Id)
			assert.Contains(t, last.Msg.Text, "✅ *Мероприятие успешно запущено!*")
			env.requireKeyboardRemoved(confirmMsg)
		})
	}
}

func TestEventStartHandler_Cancel(t *testing.T) {
	ev := startTestEvent(constants.EventTypeMeetup, "Go Meetup")
	states := []struct {
		name       string
		reachState func(t *testing.T, env *testEnv, ev repositories.Event) gotgbot.Message
	}{
		{"select event", startStart},
		{"enter link", selectEventForStart},
		{"confirm", enterStartLink},
	}

	for _, state := range states {
		for _, viaButton := range []bool{true, false} {
			name := state.name + "/command"
			if viaButton {
				name = state.name + "/button"
			}
			t.Run(name, func(t *testing.T) {
				env := newTestEnv(t, NewEventStartHandler)
				keyboardMsg := state.reachState(t, env, ev)
				env.reset()

				if viaButton {
					require.True(t, env.click(testAdminID, eventStartCallbackConfirmCancel))
					assert.Len(t, env.client.CallsTo("answerCallbackQuery"), 1)
				} else {
					require.True(t, env.sendText(testAdminID, "/cancel"))
				}

				assert.Equal(t, []string{startCanceledText}, env.texts())
				env.requireKeyboardRemoved(keyboardMsg)
				assert.Empty(t, env.client.CallsTo("pinChatMessage"))
				assert.False(t, env.click(testAdminID, eventStartCallbackConfirmYes), "conversation must be over")
			})
		}
	}
}
