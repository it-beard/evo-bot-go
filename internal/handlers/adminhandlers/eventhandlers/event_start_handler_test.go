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
	startCanceledText = "Операция запуска мероприятия отменена."
	testEventLink     = "https://zoom.us/j/123"
	invalidLinkText   = "Некорректная ссылка. Пожалуйста, отправь полную ссылку вида https://… или /cancel для отмены."
	clubCallRulesLink = "💡 <a href=\"https://telegram.me/c/2069889012/127/33823\">Про формат и правила клубных созвонов</a>\n"
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
	return enterStartLinkValue(t, env, ev, testEventLink)
}

// enterStartLinkValue enters the given event link and returns the confirmation message
func enterStartLinkValue(t *testing.T, env *testEnv, ev repositories.Event, link string) gotgbot.Message {
	t.Helper()
	selectEventForStart(t, env, ev)
	env.expectGetEventByID(ev.ID, &ev)
	require.True(t, env.sendText(testAdminID, link))
	return env.lastSent().Msg
}

func expectStatusFinished(mock sqlmock.Sqlmock) {
	mock.ExpectExec(regexp.QuoteMeta(sqlUpdateStatus)).
		WithArgs(constants.EventStatusFinished, 7).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

// isAnnouncement reports whether a sendMessage call goes to the club supergroup
func isAnnouncement(call testutils.APICall) bool {
	return call.Int64("chat_id") == fullChatID()
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
			wantText: eventListErrorText,
		},
		{
			name:     "no events",
			setup:    func(env *testEnv) { env.expectGetLastEvents() },
			wantText: noEventsText,
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
		wantSummary      string
		wantAnnouncement string
	}{
		{
			name:        "club call includes rules link",
			event:       startTestEvent(constants.EventTypeClubCall, "Club Call"),
			wantSummary: "💬 <i>клубный созвон</i>: <b>Club Call</b>\n└ <i>ID</i> 7, <i>когда</i>: не указано",
			wantAnnouncement: "🔴 <b>НАЧИНАЕМ ИВЕНТ!</b> 🔴\n\n💬 <b>Club Call</b>\n" +
				clubCallRulesLink +
				"\nИспользуй кнопку ниже, чтобы присоединиться ⬇️",
		},
		{
			name:             "meetup has no rules link",
			event:            startTestEvent(constants.EventTypeMeetup, "Go Meetup"),
			wantSummary:      "🎙 <i>митап</i>: <b>Go Meetup</b>\n└ <i>ID</i> 7, <i>когда</i>: не указано",
			wantAnnouncement: "🔴 <b>НАЧИНАЕМ ИВЕНТ!</b> 🔴\n\n🎙 <b>Go Meetup</b>\n\nИспользуй кнопку ниже, чтобы присоединиться ⬇️",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t, NewEventStartHandler)
			ev := tc.event

			// 1. Event list
			listMsg := startStart(t, env, ev)
			listCall := env.lastSent().Call
			assert.Equal(t, "HTML", listCall.Params["parse_mode"])
			assert.Contains(t, listMsg.Text, adminListTitle)
			assert.Contains(t, listMsg.Text, ": <b>"+ev.Name+"</b>\n└ <i>ID</i> /7, <i>когда</i>: не указано\n")
			assert.Contains(t, listMsg.Text, "👉 Нажми на ID мероприятия, которое ты хочешь запустить.")
			assert.Equal(t, []string{eventStartCallbackConfirmCancel}, callbackData(t, listCall))

			// 2. Non-numeric id keeps the state
			env.reset()
			require.True(t, env.sendText(testAdminID, "abc"))
			assert.Equal(t, []string{invalidEventIDText}, env.texts())

			// 3. Selecting the event asks for the link
			env.reset()
			env.expectGetEventByID(7, &ev)
			require.True(t, env.sendText(testAdminID, "/7"))
			env.requireKeyboardRemoved(listMsg)
			linkPrompt := env.lastSent()
			assert.Equal(t,
				"🚀 <b>Запуск мероприятия</b>\n\n"+tc.wantSummary+"\n\n"+
					"🔗 Отправь мне ссылку на мероприятие, она будет опубликована в чате объявлений:",
				linkPrompt.Msg.Text)
			assert.Equal(t, "HTML", linkPrompt.Call.Params["parse_mode"])
			assert.Equal(t, []string{eventStartCallbackConfirmCancel}, callbackData(t, linkPrompt.Call))

			// 4. Something that is not a link keeps the state
			env.reset()
			require.True(t, env.sendText(testAdminID, "zoom.us/j/123"))
			assert.Equal(t, []string{invalidLinkText}, env.texts())

			// 5. A link asks for confirmation
			env.reset()
			env.expectGetEventByID(7, &ev)
			require.True(t, env.sendText(testAdminID, "  "+testEventLink+" "))
			env.requireKeyboardRemoved(linkPrompt.Msg)
			confirm := env.lastSent()
			assert.Equal(t,
				"🚀 <b>Запустить мероприятие?</b>\n\n"+tc.wantSummary+"\n\n"+
					"🔗 Ссылка: <code>"+testEventLink+"</code>\n\n"+
					"Объявление со ссылкой будет опубликовано и закреплено в чате объявлений, "+
					"а мероприятие получит статус «завершено».",
				confirm.Msg.Text)
			assert.Equal(t, "HTML", confirm.Call.Params["parse_mode"])
			assert.Equal(t, []string{eventStartCallbackConfirmYes, eventStartCallbackConfirmCancel}, callbackData(t, confirm.Call))

			// 6. Any message during confirmation (text or not) asks to use the buttons
			env.reset()
			require.True(t, env.sendText(testAdminID, "ok"))
			sticker := testutils.NewPrivateMessage(testAdminID, "")
			sticker.Sticker = &gotgbot.Sticker{FileId: "sticker", FileUniqueId: "sticker", Type: "regular", Width: 1, Height: 1}
			require.True(t, env.dispatch(testutils.MessageUpdate(sticker)))
			assert.Equal(t, []string{useButtonsText, useButtonsText}, env.texts())

			// 7. Confirmation announces the event, pins the announcement and only then marks the event finished
			env.reset()
			env.expectGetEventByID(7, &ev)
			expectStatusFinished(env.mock)
			announcedBeforeStatusUpdate := false
			env.onSend(func(call testutils.APICall) {
				if isAnnouncement(call) {
					// The status UPDATE is the only expectation still pending at this point
					announcedBeforeStatusUpdate = env.mock.ExpectationsWereMet() != nil
				}
			})
			require.True(t, env.click(testAdminID, eventStartCallbackConfirmYes))

			assert.True(t, announcedBeforeStatusUpdate, "the announcement must be published before the status is updated")
			require.NoError(t, env.mock.ExpectationsWereMet())
			assert.Len(t, env.client.CallsTo("answerCallbackQuery"), 1)
			sent := env.sentMessages()
			require.Len(t, sent, 2)

			announcement := sent[0]
			assert.Equal(t, fullChatID(), announcement.Msg.Chat.Id)
			assert.Equal(t, int64(testAnnouncementTopicID), announcement.Call.Int64("message_thread_id"))
			assert.Equal(t, "HTML", announcement.Call.Params["parse_mode"])
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
				"✅ Мероприятие <b>"+ev.Name+"</b> (ID 7) запущено!\n\n📢 Объявление со ссылкой опубликовано в чате объявлений.",
				sent[1].Msg.Text)
			assert.Equal(t, "HTML", sent[1].Call.Params["parse_mode"])
			env.requireKeyboardRemoved(confirm.Msg)

			// 8. The conversation is over
			assert.False(t, env.click(testAdminID, eventStartCallbackConfirmYes))
		})
	}
}

func TestEventStartHandler_RulesLinkOnlyForClubCalls(t *testing.T) {
	for _, eventType := range constants.AllEventTypes {
		t.Run(string(eventType), func(t *testing.T) {
			env := newTestEnv(t, NewEventStartHandler)
			ev := startTestEvent(eventType, "Event")
			enterStartLink(t, env, ev)
			env.reset()

			env.expectGetEventByID(7, &ev)
			expectStatusFinished(env.mock)
			require.True(t, env.click(testAdminID, eventStartCallbackConfirmYes))

			announcement := env.sentMessages()[0]
			require.True(t, isAnnouncement(announcement.Call))
			if eventType == constants.EventTypeClubCall {
				assert.Contains(t, announcement.Msg.Text, clubCallRulesLink)
			} else {
				assert.NotContains(t, announcement.Msg.Text, "Про формат и правила клубных созвонов")
				assert.NotContains(t, announcement.Msg.Text, "<a href=")
			}
		})
	}
}

func TestEventStartHandler_EscapesEventName(t *testing.T) {
	env := newTestEnv(t, NewEventStartHandler)
	ev := startTestEvent(constants.EventTypeMeetup, htmlEventName)

	listMsg := startStart(t, env, ev)
	assert.Contains(t, listMsg.Text, "🔄 🎙 <i>митап</i>: "+escapedEventName+"\n")
	assert.NotContains(t, listMsg.Text, htmlEventName)

	env.reset()
	env.expectGetEventByID(7, &ev)
	require.True(t, env.sendText(testAdminID, "/7"))
	assert.Contains(t, env.lastSent().Msg.Text, "🎙 <i>митап</i>: "+escapedEventName+"\n└ <i>ID</i> 7")
	assert.NotContains(t, env.lastSent().Msg.Text, htmlEventName)

	env.reset()
	env.expectGetEventByID(7, &ev)
	require.True(t, env.sendText(testAdminID, testEventLink))
	assert.Contains(t, env.lastSent().Msg.Text, "🎙 <i>митап</i>: "+escapedEventName+"\n└ <i>ID</i> 7")
	assert.NotContains(t, env.lastSent().Msg.Text, htmlEventName)

	env.reset()
	env.expectGetEventByID(7, &ev)
	expectStatusFinished(env.mock)
	require.True(t, env.click(testAdminID, eventStartCallbackConfirmYes))

	sent := env.sentMessages()
	require.Len(t, sent, 2)
	assert.Equal(t,
		"🔴 <b>НАЧИНАЕМ ИВЕНТ!</b> 🔴\n\n🎙 "+escapedEventName+"\n\nИспользуй кнопку ниже, чтобы присоединиться ⬇️",
		sent[0].Msg.Text)
	assert.Equal(t,
		"✅ Мероприятие "+escapedEventName+" (ID 7) запущено!\n\n📢 Объявление со ссылкой опубликовано в чате объявлений.",
		sent[1].Msg.Text)
}

func TestEventStartHandler_AnnouncementEscapesSpecialCharacters(t *testing.T) {
	env := newTestEnv(t, NewEventStartHandler)
	ev := startTestEvent(constants.EventTypeClubCall, "R&D <Talks>")
	enterStartLink(t, env, ev)
	env.reset()

	env.expectGetEventByID(7, &ev)
	expectStatusFinished(env.mock)
	require.True(t, env.click(testAdminID, eventStartCallbackConfirmYes))

	announcement := env.sentMessages()[0]
	assert.Equal(t, "HTML", announcement.Call.Params["parse_mode"])
	assert.Equal(t,
		"🔴 <b>НАЧИНАЕМ ИВЕНТ!</b> 🔴\n\n💬 <b>R&amp;D &lt;Talks&gt;</b>\n"+clubCallRulesLink+
			"\nИспользуй кнопку ниже, чтобы присоединиться ⬇️",
		announcement.Msg.Text)
}

func TestEventStartHandler_LinkValidation(t *testing.T) {
	ev := startTestEvent(constants.EventTypeMeetup, "Go Meetup")

	t.Run("invalid links keep waiting", func(t *testing.T) {
		env := newTestEnv(t, NewEventStartHandler)
		linkPrompt := selectEventForStart(t, env, ev)

		invalid := []string{
			"zoom.us/j/123",
			"https://",
			"http://",
			"http:/x",
			"https:///path",
			"ftp://example.com/file",
			"mailto:admin@example.com",
			"javascript:alert(1)",
			"tg://resolve?domain=evocoders",
			"https://exa mple.com",
		}
		for _, link := range invalid {
			env.reset()
			// No event lookup happens for an invalid link (sqlmock fails on unexpected queries)
			require.True(t, env.sendText(testAdminID, link))
			assert.Equal(t, []string{invalidLinkText}, env.texts(), "link %q", link)
			assert.Empty(t, env.client.CallsTo("editMessageReplyMarkup"), "link %q", link)
		}

		// The conversation still waits for a link
		env.reset()
		env.expectGetEventByID(7, &ev)
		require.True(t, env.sendText(testAdminID, "http://example.com"))
		env.requireKeyboardRemoved(linkPrompt)
		assert.Contains(t, env.lastSent().Msg.Text, "🔗 Ссылка: <code>http://example.com</code>")
	})

	t.Run("valid links are accepted", func(t *testing.T) {
		for _, link := range []string{"http://example.com", "https://zoom.us/j/123?pwd=abc", "HTTPS://Meet.Example.com/room"} {
			env := newTestEnv(t, NewEventStartHandler)
			confirm := enterStartLinkValue(t, env, ev, link)
			assert.Contains(t, confirm.Text, "🚀 <b>Запустить мероприятие?</b>", "link %q", link)
		}
	})

	t.Run("link is escaped in confirmation and kept as is in the button", func(t *testing.T) {
		const link = "https://zoom.us/j/123?pwd=a<b&uname=x"
		env := newTestEnv(t, NewEventStartHandler)
		confirm := enterStartLinkValue(t, env, ev, link)
		assert.Contains(t, confirm.Text, "🔗 Ссылка: <code>https://zoom.us/j/123?pwd=a&lt;b&amp;uname=x</code>")

		env.reset()
		env.expectGetEventByID(7, &ev)
		expectStatusFinished(env.mock)
		require.True(t, env.click(testAdminID, eventStartCallbackConfirmYes))

		buttons := inlineButtons(t, env.sentMessages()[0].Call)
		require.Len(t, buttons, 1)
		assert.Equal(t, link, buttons[0].Url)
	})
}

func TestEventStartHandler_SelectUnknownEventKeepsWaiting(t *testing.T) {
	env := newTestEnv(t, NewEventStartHandler)
	ev := startTestEvent(constants.EventTypeMeetup, "Go Meetup")
	listMsg := startStart(t, env, ev)
	env.reset()

	env.expectGetEventByID(99, nil)
	require.True(t, env.sendText(testAdminID, "/99"))

	assert.Equal(t, []string{eventNotFoundText(99)}, env.texts())
	assert.Empty(t, env.client.CallsTo("editMessageReplyMarkup"))

	// The conversation still waits for an event id
	env.reset()
	env.expectGetEventByID(7, &ev)
	require.True(t, env.sendText(testAdminID, "/7"))
	env.requireKeyboardRemoved(listMsg)
	assert.Contains(t, env.lastSent().Msg.Text, "🚀 <b>Запуск мероприятия</b>")
}

func TestEventStartHandler_EnterLinkLookupFailureEndsConversation(t *testing.T) {
	env := newTestEnv(t, NewEventStartHandler)
	selectEventForStart(t, env, startTestEvent(constants.EventTypeMeetup, "Go Meetup"))
	env.reset()

	env.mock.ExpectQuery(regexp.QuoteMeta(sqlGetEventByID)).WithArgs(7).WillReturnError(errors.New("db down"))
	require.True(t, env.sendText(testAdminID, testEventLink))

	assert.Equal(t, []string{"Произошла ошибка при получении мероприятия с ID 7."}, env.texts())
	assert.False(t, env.sendText(testAdminID, testEventLink), "conversation must be over")
}

func TestEventStartHandler_EventLookupFailureDoesNotAnnounce(t *testing.T) {
	env := newTestEnv(t, NewEventStartHandler)
	confirmMsg := enterStartLink(t, env, startTestEvent(constants.EventTypeMeetup, "Go Meetup"))
	env.reset()

	env.mock.ExpectQuery(regexp.QuoteMeta(sqlGetEventByID)).WithArgs(7).WillReturnError(errors.New("db down"))
	require.True(t, env.click(testAdminID, eventStartCallbackConfirmYes))

	assert.Equal(t, []string{"Произошла ошибка при получении мероприятия с ID 7."}, env.texts())
	assert.Equal(t, testAdminID, env.lastSent().Msg.Chat.Id)
	assert.Empty(t, env.client.CallsTo("pinChatMessage"))
	env.requireKeyboardRemoved(confirmMsg)
	assert.False(t, env.click(testAdminID, eventStartCallbackConfirmYes), "conversation must be over")
}

func TestEventStartHandler_AnnouncementFailureKeepsEventActual(t *testing.T) {
	env := newTestEnv(t, NewEventStartHandler)
	ev := startTestEvent(constants.EventTypeMeetup, "Go Meetup")
	confirmMsg := enterStartLink(t, env, ev)
	env.reset()

	env.failSendWhen(isAnnouncement)
	// Only the event lookup is expected: an UPDATE of the status would hit sqlmock unexpectedly
	env.expectGetEventByID(7, &ev)
	require.True(t, env.click(testAdminID, eventStartCallbackConfirmYes))

	// The announcement was attempted ...
	var announcementAttempts int
	for _, call := range env.client.CallsTo("sendMessage") {
		if isAnnouncement(call) {
			announcementAttempts++
		}
	}
	assert.Equal(t, 1, announcementAttempts)
	// ... but nothing was pinned, the status is untouched and the admin learns that the event was not started
	assert.Empty(t, env.client.CallsTo("pinChatMessage"))
	require.NoError(t, env.mock.ExpectationsWereMet())
	assert.Equal(t, []string{
		"Не удалось опубликовать объявление в чате объявлений, мероприятие не запущено. " +
			"Проверь ссылку и попробуй ещё раз: /eventStart.",
	}, env.texts())
	assert.Equal(t, testAdminID, env.lastSent().Msg.Chat.Id)
	env.requireKeyboardRemoved(confirmMsg)
	assert.False(t, env.click(testAdminID, eventStartCallbackConfirmYes), "conversation must be over")
}

func TestEventStartHandler_StatusUpdateFailureAfterAnnouncementWarnsAdmin(t *testing.T) {
	env := newTestEnv(t, NewEventStartHandler)
	ev := startTestEvent(constants.EventTypeMeetup, "Go Meetup")
	confirmMsg := enterStartLink(t, env, ev)
	env.reset()

	env.expectGetEventByID(7, &ev)
	env.mock.ExpectExec(regexp.QuoteMeta(sqlUpdateStatus)).
		WithArgs(constants.EventStatusFinished, 7).
		WillReturnError(errors.New("db down"))
	require.True(t, env.click(testAdminID, eventStartCallbackConfirmYes))

	sent := env.sentMessages()
	require.Len(t, sent, 2)
	// The announcement is out and pinned ...
	assert.True(t, isAnnouncement(sent[0].Call))
	_, pinned := env.client.LastCallTo("pinChatMessage")
	assert.True(t, pinned)
	// ... so the admin is asked to finish the event manually instead of getting a plain error
	assert.Equal(t, testAdminID, sent[1].Msg.Chat.Id)
	assert.Equal(t,
		"⚠️ Объявление опубликовано, но статус мероприятия обновить не удалось. Заверши его вручную через /eventEdit.",
		sent[1].Msg.Text)
	env.requireKeyboardRemoved(confirmMsg)
	assert.False(t, env.click(testAdminID, eventStartCallbackConfirmYes), "conversation must be over")
}

func TestEventStartHandler_PinFailureStillStartsEvent(t *testing.T) {
	env := newTestEnv(t, NewEventStartHandler)
	ev := startTestEvent(constants.EventTypeMeetup, "Go Meetup")
	confirmMsg := enterStartLink(t, env, ev)
	env.reset()

	env.client.OnError("pinChatMessage", "Bad Request: not enough rights")
	env.expectGetEventByID(7, &ev)
	expectStatusFinished(env.mock)
	require.True(t, env.click(testAdminID, eventStartCallbackConfirmYes))

	assert.Len(t, env.client.CallsTo("pinChatMessage"), 1)
	require.NoError(t, env.mock.ExpectationsWereMet())
	sent := env.sentMessages()
	require.Len(t, sent, 2)
	assert.True(t, isAnnouncement(sent[0].Call))
	assert.Equal(t, testAdminID, sent[1].Msg.Chat.Id)
	assert.Contains(t, sent[1].Msg.Text, "✅ Мероприятие <b>Go Meetup</b> (ID 7) запущено!")
	env.requireKeyboardRemoved(confirmMsg)
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
