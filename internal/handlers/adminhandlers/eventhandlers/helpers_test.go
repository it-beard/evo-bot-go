package eventhandlers

import (
	"database/sql/driver"
	"encoding/json"
	"regexp"
	"sync"
	"testing"
	"time"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/testutils"
	"evo-bot-go/internal/utils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/require"
)

const (
	testAdminID             int64 = 1001
	testUserID              int64 = 2002
	testSuperGroupID        int64 = 2069889012
	testAnnouncementTopicID       = 77
)

const (
	sqlGetLastEvents = "SELECT id, name, type, status, started_at, created_at, updated_at FROM events " +
		"ORDER BY started_at DESC NULLS LAST LIMIT $1"
	sqlGetEventByID  = "SELECT id, name, type, status, started_at, created_at, updated_at FROM events WHERE id = $1"
	sqlCreateEvent   = "INSERT INTO events (name, type, status, started_at) VALUES ($1, $2, $3, $4) RETURNING id"
	sqlUpdateName    = "UPDATE events SET name = $1, updated_at = NOW() WHERE id = $2"
	sqlUpdateStatus  = "UPDATE events SET status = $1, updated_at = NOW() WHERE id = $2"
	sqlUpdateStart   = "UPDATE events SET started_at = $1, updated_at = NOW() WHERE id = $2"
	sqlUpdateType    = "UPDATE events SET type = $1, updated_at = NOW() WHERE id = $2"
	sqlDeleteEvent   = "DELETE FROM events WHERE id = $1"
	sqlTopicsByEvent = "SELECT id, topic, user_nickname, event_id, created_at FROM topics WHERE event_id = $1"
	sqlDeleteTopic   = "DELETE FROM topics WHERE id = $1"

	adminOnlyText = "Эта команда доступна только администраторам."
)

var (
	eventColumns = []string{"id", "name", "type", "status", "started_at", "created_at", "updated_at"}
	topicColumns = []string{"id", "topic", "user_nickname", "event_id", "created_at"}
	fixedTime    = time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
)

type handlerFactory func(
	*config.Config,
	*repositories.EventRepository,
	*services.MessageSenderService,
	*services.PermissionsService,
) ext.Handler

// sentMessage is a message the bot successfully sent through sendMessage
type sentMessage struct {
	Call testutils.APICall
	Msg  gotgbot.Message
}

// messageLog answers sendMessage calls with predictable message ids and remembers the sent messages
type messageLog struct {
	mu     sync.Mutex
	nextID int64
	sent   []sentMessage
	fail   func(call testutils.APICall) bool
}

func (l *messageLog) respond(call testutils.APICall) (json.RawMessage, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.fail != nil && l.fail(call) {
		return nil, &gotgbot.TelegramError{Method: call.Method, Code: 400, Description: "Bad Request: mocked failure"}
	}

	l.nextID++
	chatID := call.Int64("chat_id")
	chatType := testutils.PrivateChatType
	if chatID < 0 {
		chatType = testutils.SuperGroupChatType
	}
	msg := gotgbot.Message{
		MessageId:       l.nextID,
		Chat:            gotgbot.Chat{Id: chatID, Type: chatType},
		Date:            1700000000,
		Text:            call.Params["text"],
		MessageThreadId: call.Int64("message_thread_id"),
	}
	l.sent = append(l.sent, sentMessage{Call: call, Msg: msg})
	return json.Marshal(msg)
}

type testEnv struct {
	t       *testing.T
	bot     *gotgbot.Bot
	client  *testutils.MockBotClient
	mock    sqlmock.Sqlmock
	cfg     *config.Config
	log     *messageLog
	handler ext.Handler
}

func newTestEnv(t *testing.T, factory handlerFactory) *testEnv {
	t.Helper()

	bot, client := testutils.NewTestBot()
	client.SetMemberStatus(testAdminID, "administrator")
	log := &messageLog{nextID: 5000}
	client.On("sendMessage", log.respond)

	db, mock := testutils.NewMockDB(t)
	cfg := &config.Config{
		SuperGroupChatID:    testSuperGroupID,
		AnnouncementTopicID: testAnnouncementTopicID,
	}
	sender := services.NewMessageSenderService(bot)
	permissions := services.NewPermissionsService(cfg, bot, sender)

	return &testEnv{
		t:       t,
		bot:     bot,
		client:  client,
		mock:    mock,
		cfg:     cfg,
		log:     log,
		handler: factory(cfg, repositories.NewEventRepository(db), sender, permissions),
	}
}

func fullChatID() int64 {
	return utils.ChatIdToFullChatId(testSuperGroupID)
}

// sendText dispatches a private text message and reports whether the conversation accepted it
func (e *testEnv) sendText(userID int64, text string) bool {
	e.t.Helper()
	return e.dispatch(testutils.MessageUpdate(testutils.NewPrivateMessage(userID, text)))
}

// sendGroupText dispatches a text message written in the club supergroup
func (e *testEnv) sendGroupText(userID int64, text string) bool {
	e.t.Helper()
	return e.dispatch(testutils.MessageUpdate(testutils.NewGroupMessage(userID, fullChatID(), 0, text)))
}

// click dispatches an inline button press in the user's private chat
func (e *testEnv) click(userID int64, data string) bool {
	e.t.Helper()
	return e.dispatch(testutils.CallbackUpdate(userID, data))
}

func (e *testEnv) dispatch(update *gotgbot.Update) bool {
	e.t.Helper()
	handled, err := testutils.Dispatch(e.bot, e.handler, update)
	require.NoError(e.t, err)
	return handled
}

// reset forgets recorded Bot API calls and sent messages
func (e *testEnv) reset() {
	e.client.Reset()
	e.log.mu.Lock()
	e.log.sent = nil
	e.log.mu.Unlock()
}

// failSendWhen makes sendMessage calls matching the predicate fail
func (e *testEnv) failSendWhen(pred func(call testutils.APICall) bool) {
	e.log.mu.Lock()
	defer e.log.mu.Unlock()
	e.log.fail = pred
}

func (e *testEnv) sentMessages() []sentMessage {
	e.log.mu.Lock()
	defer e.log.mu.Unlock()
	return append([]sentMessage(nil), e.log.sent...)
}

// lastSent returns the last message the bot sent
func (e *testEnv) lastSent() sentMessage {
	e.t.Helper()
	sent := e.sentMessages()
	require.NotEmpty(e.t, sent, "expected the bot to send a message")
	return sent[len(sent)-1]
}

// texts returns the texts of all messages sent since the last reset
func (e *testEnv) texts() []string {
	var texts []string
	for _, s := range e.sentMessages() {
		texts = append(texts, s.Msg.Text)
	}
	return texts
}

// requireKeyboardRemoved asserts that the inline keyboard of the given bot message was removed
func (e *testEnv) requireKeyboardRemoved(msg gotgbot.Message) {
	e.t.Helper()
	for _, call := range e.client.CallsTo("editMessageReplyMarkup") {
		if call.Int64("chat_id") == msg.Chat.Id && call.Int64("message_id") == msg.MessageId {
			return
		}
	}
	e.t.Fatalf("inline keyboard of message %d in chat %d was not removed", msg.MessageId, msg.Chat.Id)
}

func (e *testEnv) expectGetLastEvents(events ...repositories.Event) {
	e.mock.ExpectQuery(regexp.QuoteMeta(sqlGetLastEvents)).
		WithArgs(constants.EventEditGetLastLimit).
		WillReturnRows(eventRows(events...))
}

// expectGetEventByID expects an event lookup; a nil event means "not found"
func (e *testEnv) expectGetEventByID(id int, event *repositories.Event) {
	if event == nil {
		e.mock.ExpectQuery(regexp.QuoteMeta(sqlGetEventByID)).WithArgs(id).WillReturnRows(eventRows())
		return
	}
	e.mock.ExpectQuery(regexp.QuoteMeta(sqlGetEventByID)).WithArgs(id).WillReturnRows(eventRows(*event))
}

func eventRows(events ...repositories.Event) *sqlmock.Rows {
	rows := sqlmock.NewRows(eventColumns)
	for _, ev := range events {
		var startedAt driver.Value
		if ev.StartedAt != nil {
			startedAt = *ev.StartedAt
		}
		rows.AddRow(ev.ID, ev.Name, ev.Type, ev.Status, startedAt, fixedTime, fixedTime)
	}
	return rows
}

func timePtr(t time.Time) *time.Time {
	return &t
}

// inlineButtons decodes the inline keyboard of a sendMessage call
func inlineButtons(t *testing.T, call testutils.APICall) []gotgbot.InlineKeyboardButton {
	t.Helper()
	var markup gotgbot.InlineKeyboardMarkup
	require.NoError(t, call.DecodeJSON("reply_markup", &markup))
	var buttons []gotgbot.InlineKeyboardButton
	for _, row := range markup.InlineKeyboard {
		buttons = append(buttons, row...)
	}
	return buttons
}

// callbackData returns the callback data of all inline buttons of a sendMessage call
func callbackData(t *testing.T, call testutils.APICall) []string {
	t.Helper()
	var data []string
	for _, b := range inlineButtons(t, call) {
		data = append(data, b.CallbackData)
	}
	return data
}
