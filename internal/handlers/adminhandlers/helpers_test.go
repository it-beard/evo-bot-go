package adminhandlers

import (
	"database/sql"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/testutils"
	"evo-bot-go/internal/utils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testAdminID          int64 = 501
	testMemberID         int64 = 502
	testSuperGroupChatID int64 = 1987654321
	testIntroTopicID           = 77

	// Regexps for the SQL issued by the repositories
	sqlUserByID           = `FROM users\s+WHERE id = \$1`
	sqlUserByTgID         = `FROM users\s+WHERE tg_id = \$1`
	sqlUserByUsername     = `FROM users\s+WHERE tg_username = \$1`
	sqlUserByName         = `FROM users\s+WHERE LOWER\(firstname\) = LOWER\(\$1\) AND LOWER\(lastname\) = LOWER\(\$2\)`
	sqlInsertUser         = `INSERT INTO users \(tg_id, firstname, lastname, tg_username, score, has_coffee_ban, is_club_member\)`
	sqlSetCoffeeBan       = `UPDATE users SET has_coffee_ban = \$1, updated_at = NOW\(\) WHERE id = \$2`
	sqlProfileByID        = `FROM profiles\s+WHERE id = \$1`
	sqlProfileByUserID    = `FROM profiles\s+WHERE user_id = \$1`
	sqlInsertProfile      = `INSERT INTO profiles \(user_id, bio\)`
	sqlUpdatePublishedMsg = `UPDATE profiles SET published_message_id = \$1, updated_at = NOW\(\) WHERE id = \$2`
	sqlLastEvents         = `FROM events\s+ORDER BY started_at DESC NULLS LAST\s+LIMIT \$1`
	sqlEventByID          = `FROM events\s+WHERE id = \$1`
	sqlTopicsByEventID    = `FROM topics\s+WHERE event_id = \$1`
	sqlTopicByID          = `FROM topics\s+WHERE id = \$1`
	sqlDeleteTopic        = `DELETE FROM topics WHERE id = \$1`
)

var (
	fixedTime = time.Date(2025, 3, 14, 18, 30, 0, 0, time.UTC)

	userColumns    = []string{"id", "tg_id", "firstname", "lastname", "tg_username", "score", "has_coffee_ban", "is_club_member", "created_at", "updated_at"}
	profileColumns = []string{"id", "user_id", "bio", "published_message_id", "created_at", "updated_at"}
	eventColumns   = []string{"id", "name", "type", "status", "started_at", "created_at", "updated_at"}
	topicColumns   = []string{"id", "topic", "user_nickname", "event_id", "created_at"}
)

// sentMessage is a message the fake Bot API returned for a sendMessage call
type sentMessage struct {
	ChatID    int64
	MessageID int64
	Text      string
}

// testEnv wires a handler to a fake Bot API and a sqlmock database
type testEnv struct {
	t           *testing.T
	bot         *gotgbot.Bot
	client      *testutils.MockBotClient
	db          *sql.DB
	mock        sqlmock.Sqlmock
	cfg         *config.Config
	sender      *services.MessageSenderService
	permissions *services.PermissionsService
	handler     ext.Handler

	mu        sync.Mutex
	nextMsgID int64
	sent      []sentMessage
	// failSend makes sendMessage fail with the returned Telegram error description (empty = success)
	failSend func(chatID int64) string
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	bot, client := testutils.NewTestBot()
	db, mock := testutils.NewMockDB(t)
	cfg := &config.Config{
		SuperGroupChatID: testSuperGroupChatID,
		IntroTopicID:     testIntroTopicID,
	}
	sender := services.NewMessageSenderService(bot)

	e := &testEnv{
		t:           t,
		bot:         bot,
		client:      client,
		db:          db,
		mock:        mock,
		cfg:         cfg,
		sender:      sender,
		permissions: services.NewPermissionsService(cfg, bot, sender),
		nextMsgID:   900000,
	}
	client.SetMemberStatus(testAdminID, "administrator")
	client.On("sendMessage", e.respondToSendMessage)
	return e
}

func newProfilesEnv(t *testing.T) *testEnv {
	t.Helper()
	e := newTestEnv(t)
	e.handler = NewAdminProfilesHandler(
		e.cfg,
		e.sender,
		e.permissions,
		services.NewProfileService(e.bot),
		repositories.NewUserRepository(e.db),
		repositories.NewProfileRepository(e.db),
	)
	return e
}

func newShowTopicsEnv(t *testing.T) *testEnv {
	t.Helper()
	e := newTestEnv(t)
	e.handler = NewShowTopicsHandler(
		e.cfg,
		repositories.NewTopicRepository(e.db),
		repositories.NewEventRepository(e.db),
		e.sender,
		e.permissions,
	)
	return e
}

// respondToSendMessage answers sendMessage with a message whose ID is tracked by the env
func (e *testEnv) respondToSendMessage(call testutils.APICall) (json.RawMessage, error) {
	chatID := call.Int64("chat_id")

	e.mu.Lock()
	failSend := e.failSend
	e.mu.Unlock()
	if failSend != nil {
		if description := failSend(chatID); description != "" {
			return nil, &gotgbot.TelegramError{Method: "sendMessage", Code: 400, Description: description}
		}
	}

	e.mu.Lock()
	e.nextMsgID++
	msg := gotgbot.Message{
		MessageId:       e.nextMsgID,
		Chat:            gotgbot.Chat{Id: chatID, Type: "private"},
		Date:            1700000000,
		Text:            call.Params["text"],
		MessageThreadId: call.Int64("message_thread_id"),
	}
	e.sent = append(e.sent, sentMessage{ChatID: chatID, MessageID: msg.MessageId, Text: msg.Text})
	e.mu.Unlock()

	if chatID < 0 {
		msg.Chat.Type = "supergroup"
	}
	return json.Marshal(msg)
}

func (e *testEnv) setFailSend(fn func(chatID int64) string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.failSend = fn
}

// lastSentID returns the ID of the latest message the bot sent
func (e *testEnv) lastSentID() int64 {
	e.t.Helper()
	e.mu.Lock()
	defer e.mu.Unlock()
	require.NotEmpty(e.t, e.sent, "no messages were sent")
	return e.sent[len(e.sent)-1].MessageID
}

// lastSentText returns the text of the latest message the bot sent (survives client.Reset)
func (e *testEnv) lastSentText() string {
	e.t.Helper()
	e.mu.Lock()
	defer e.mu.Unlock()
	require.NotEmpty(e.t, e.sent, "no messages were sent")
	return e.sent[len(e.sent)-1].Text
}

// nextSentID returns the ID the next successfully sent message will get
func (e *testEnv) nextSentID() int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.nextMsgID + 1
}

// sentIDByText returns the ID of the latest sent message containing substr
func (e *testEnv) sentIDByText(substr string) int64 {
	e.t.Helper()
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := len(e.sent) - 1; i >= 0; i-- {
		if strings.Contains(e.sent[i].Text, substr) {
			return e.sent[i].MessageID
		}
	}
	require.Failf(e.t, "message not found", "no sent message contains %q", substr)
	return 0
}

// dispatch runs the update through the handler and requires that the handler accepted it
func (e *testEnv) dispatch(u *gotgbot.Update) error {
	e.t.Helper()
	handled, err := testutils.Dispatch(e.bot, e.handler, u)
	require.True(e.t, handled, "update was not handled by the conversation")
	return err
}

// notHandled asserts that the conversation does not accept the update (e.g. it has ended)
func (e *testEnv) notHandled(u *gotgbot.Update) {
	e.t.Helper()
	handled, err := testutils.Dispatch(e.bot, e.handler, u)
	assert.NoError(e.t, err)
	assert.False(e.t, handled, "update must not be handled by the conversation")
}

// text sends a private text message from the admin
func (e *testEnv) text(text string) (*gotgbot.Message, error) {
	e.t.Helper()
	msg := testutils.NewPrivateMessage(testAdminID, text)
	return msg, e.dispatch(testutils.MessageUpdate(msg))
}

// mustText sends a private text message from the admin and requires no error
func (e *testEnv) mustText(text string) *gotgbot.Message {
	e.t.Helper()
	msg, err := e.text(text)
	require.NoError(e.t, err)
	return msg
}

// callbackUpdate builds a callback query update whose message is stored by value, like the JSON decoder does.
// ext.NewContext only recognises gotgbot.Message values, so with a *gotgbot.Message EffectiveMessage would be nil.
func callbackUpdate(userID int64, data string) *gotgbot.Update {
	u := testutils.CallbackUpdate(userID, data)
	if m, ok := u.CallbackQuery.Message.(*gotgbot.Message); ok {
		u.CallbackQuery.Message = *m
	}
	return u
}

// click sends a callback query from the admin
func (e *testEnv) click(data string) error {
	e.t.Helper()
	return e.dispatch(callbackUpdate(testAdminID, data))
}

func (e *testEnv) mustClick(data string) {
	e.t.Helper()
	require.NoError(e.t, e.click(data))
}

// lastSendMessage returns the latest sendMessage call
func (e *testEnv) lastSendMessage() testutils.APICall {
	e.t.Helper()
	call, ok := e.client.LastCallTo("sendMessage")
	require.True(e.t, ok, "sendMessage was not called")
	return call
}

// deletedMessageIDs returns IDs of all deleted messages
func (e *testEnv) deletedMessageIDs() []int64 {
	var ids []int64
	for _, c := range e.client.CallsTo("deleteMessage") {
		ids = append(ids, c.Int64("message_id"))
	}
	return ids
}

// markup decodes the reply_markup of a call
func markup(t *testing.T, call testutils.APICall) gotgbot.InlineKeyboardMarkup {
	t.Helper()
	var m gotgbot.InlineKeyboardMarkup
	require.NoError(t, call.DecodeJSON("reply_markup", &m))
	return m
}

// linkPreviewDisabled decodes link_preview_options.is_disabled of a call
func linkPreviewDisabled(t *testing.T, call testutils.APICall) bool {
	t.Helper()
	var opts gotgbot.LinkPreviewOptions
	require.NoError(t, call.DecodeJSON("link_preview_options", &opts))
	return opts.IsDisabled
}

func fullGroupChatID() int64 {
	return utils.ChatIdToFullChatId(testSuperGroupChatID)
}

// ---- DB fixtures ----

func userRows(u repositories.User) *sqlmock.Rows {
	return sqlmock.NewRows(userColumns).AddRow(
		u.ID, u.TgID, u.Firstname, u.Lastname, u.TgUsername, u.Score, u.HasCoffeeBan, u.IsClubMember, fixedTime, fixedTime,
	)
}

func profileRows(p repositories.Profile) *sqlmock.Rows {
	var published any
	if p.PublishedMessageID.Valid {
		published = p.PublishedMessageID.Int64
	}
	return sqlmock.NewRows(profileColumns).AddRow(p.ID, p.UserID, p.Bio, published, fixedTime, fixedTime)
}

func eventRows(events ...repositories.Event) *sqlmock.Rows {
	rows := sqlmock.NewRows(eventColumns)
	for _, ev := range events {
		var startedAt any
		if ev.StartedAt != nil {
			startedAt = *ev.StartedAt
		}
		rows.AddRow(ev.ID, ev.Name, ev.Type, ev.Status, startedAt, fixedTime, fixedTime)
	}
	return rows
}

func topicRows(topics ...repositories.Topic) *sqlmock.Rows {
	rows := sqlmock.NewRows(topicColumns)
	for _, tp := range topics {
		var nickname any
		if tp.UserNickname != nil {
			nickname = *tp.UserNickname
		}
		rows.AddRow(tp.ID, tp.Topic, nickname, tp.EventID, fixedTime)
	}
	return rows
}

func (e *testEnv) expectUserByID(u repositories.User) {
	e.mock.ExpectQuery(sqlUserByID).WithArgs(u.ID).WillReturnRows(userRows(u))
}

func (e *testEnv) expectProfileByID(p repositories.Profile) {
	e.mock.ExpectQuery(sqlProfileByID).WithArgs(p.ID).WillReturnRows(profileRows(p))
}

func (e *testEnv) expectProfileByUserID(p repositories.Profile) {
	e.mock.ExpectQuery(sqlProfileByUserID).WithArgs(p.UserID).WillReturnRows(profileRows(p))
}
