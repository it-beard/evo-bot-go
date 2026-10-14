package topicshandlers

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
	testSuperGroupID     int64 = 1234567890
	testFullSuperGroupID int64 = -1001234567890
	testAdminUserID      int64 = 9001
	testUserID           int64 = 501
	testGroupChatID      int64 = -100500

	selectActiveEventsSQL = `FROM events\s+WHERE status = \$1 AND \(started_at IS NULL OR started_at > NOW\(\)\)\s+ORDER BY started_at ASC NULLS LAST\s+LIMIT \$2`
	selectEventByIDSQL    = `FROM events\s+WHERE id = \$1`
	selectTopicsSQL       = `FROM topics\s+WHERE event_id = \$1\s+ORDER BY created_at ASC`
	insertTopicSQL        = `INSERT INTO topics \(topic, user_nickname, event_id\) VALUES \(\$1, \$2, \$3\) RETURNING id`
)

var (
	testTime     = time.Date(2024, 5, 1, 10, 0, 0, 0, time.UTC)
	eventColumns = []string{"id", "name", "type", "status", "started_at", "created_at", "updated_at"}
	topicColumns = []string{"id", "topic", "user_nickname", "event_id", "created_at"}

	meetupStart = time.Date(2030, 3, 4, 18, 30, 0, 0, time.UTC)
	testEvents  = []repositories.Event{
		{ID: 7, Name: "Go Meetup", Type: "meetup", Status: "actual", StartedAt: &meetupStart, CreatedAt: testTime, UpdatedAt: testTime},
		{ID: 8, Name: "Book Club", Type: "reading-club", Status: "actual", CreatedAt: testTime, UpdatedAt: testTime},
	}
)

// deps holds the real services/repositories the topics handlers need, built over the test fakes
type deps struct {
	bot         *gotgbot.Bot
	client      *testutils.MockBotClient
	cfg         *config.Config
	db          *sql.DB
	sql         sqlmock.Sqlmock
	sender      *services.MessageSenderService
	permissions *services.PermissionsService

	sentMu sync.Mutex
	sent   []sentMessage
}

type sentMessage struct {
	id   int64
	text string
}

func newDeps(t *testing.T) *deps {
	t.Helper()

	bot, client := testutils.NewTestBot()
	db, mock := testutils.NewMockDB(t)
	cfg := &config.Config{SuperGroupChatID: testSuperGroupID, AdminUserID: testAdminUserID}
	sender := services.NewMessageSenderService(bot)

	d := &deps{
		bot:         bot,
		client:      client,
		cfg:         cfg,
		db:          db,
		sql:         mock,
		sender:      sender,
		permissions: services.NewPermissionsService(cfg, bot, sender),
	}
	client.On("sendMessage", d.sendMessageResponse)
	return d
}

// sendMessageResponse answers sendMessage calls like the default fake, but remembers the id of every
// delivered message so tests can check which message had its keyboard removed
func (d *deps) sendMessageResponse(call testutils.APICall) (json.RawMessage, error) {
	chatID := call.Int64("chat_id")
	chatType := "private"
	if chatID < 0 {
		chatType = "supergroup"
	}

	d.sentMu.Lock()
	id := int64(900001 + len(d.sent))
	d.sent = append(d.sent, sentMessage{id: id, text: call.Params["text"]})
	d.sentMu.Unlock()

	return json.Marshal(gotgbot.Message{
		MessageId: id,
		Chat:      gotgbot.Chat{Id: chatID, Type: chatType},
		Date:      1700000000,
		Text:      call.Params["text"],
	})
}

// sentID returns the id of the first delivered message whose text contains substr
func (d *deps) sentID(t *testing.T, substr string) int64 {
	t.Helper()
	d.sentMu.Lock()
	defer d.sentMu.Unlock()
	for _, m := range d.sent {
		if strings.Contains(m.text, substr) {
			return m.id
		}
	}
	require.Failf(t, "message not sent", "no delivered message contains %q", substr)
	return 0
}

func (d *deps) newTopicsHandler() ext.Handler {
	return NewTopicsHandler(d.cfg, repositories.NewTopicRepository(d.db), repositories.NewEventRepository(d.db), d.sender, d.permissions)
}

func (d *deps) newTopicAddHandler() ext.Handler {
	return NewTopicAddHandler(d.cfg, repositories.NewTopicRepository(d.db), repositories.NewEventRepository(d.db), d.sender, d.permissions)
}

// newTopicAddState builds the /topicAdd state handlers directly, so a test can tamper with the conversation data
func (d *deps) newTopicAddState() *topicAddHandler {
	return &topicAddHandler{
		config:               d.cfg,
		topicRepository:      repositories.NewTopicRepository(d.db),
		eventRepository:      repositories.NewEventRepository(d.db),
		messageSenderService: d.sender,
		userStore:            utils.NewUserDataStore(),
		permissionsService:   d.permissions,
	}
}

func (d *deps) dispatch(t *testing.T, h ext.Handler, u *gotgbot.Update) bool {
	t.Helper()
	handled, err := testutils.Dispatch(d.bot, h, u)
	require.NoError(t, err)
	return handled
}

func (d *deps) expectActualEvents(events ...repositories.Event) {
	rows := sqlmock.NewRows(eventColumns)
	for _, e := range events {
		rows.AddRow(e.ID, e.Name, e.Type, e.Status, e.StartedAt, e.CreatedAt, e.UpdatedAt)
	}
	d.sql.ExpectQuery(selectActiveEventsSQL).WithArgs("actual", 10).WillReturnRows(rows)
}

func (d *deps) expectEvent(e repositories.Event) {
	d.sql.ExpectQuery(selectEventByIDSQL).WithArgs(e.ID).WillReturnRows(
		sqlmock.NewRows(eventColumns).AddRow(e.ID, e.Name, e.Type, e.Status, e.StartedAt, e.CreatedAt, e.UpdatedAt))
}

func privateText(userID int64, text string) *gotgbot.Update {
	return testutils.MessageUpdate(testutils.NewPrivateMessage(userID, text))
}

func groupText(userID int64, text string) *gotgbot.Update {
	return testutils.MessageUpdate(testutils.NewGroupMessage(userID, testGroupChatID, 0, text))
}

func lastSend(t *testing.T, client *testutils.MockBotClient) testutils.APICall {
	t.Helper()
	call, ok := client.LastCallTo("sendMessage")
	require.True(t, ok, "expected a sendMessage call")
	return call
}

func callbackData(t *testing.T, call testutils.APICall) []string {
	t.Helper()
	var markup gotgbot.InlineKeyboardMarkup
	require.NoError(t, call.DecodeJSON("reply_markup", &markup))
	var data []string
	for _, row := range markup.InlineKeyboard {
		for _, b := range row {
			data = append(data, b.CallbackData)
		}
	}
	return data
}

// assertKeyboardRemoved checks that the inline keyboard of exactly the given bot messages was removed
func assertKeyboardRemoved(t *testing.T, client *testutils.MockBotClient, messageIDs ...int64) {
	t.Helper()
	var removed []int64
	for _, c := range client.CallsTo("editMessageReplyMarkup") {
		assert.Equal(t, testUserID, c.Int64("chat_id"))
		var markup gotgbot.InlineKeyboardMarkup
		require.NoError(t, c.DecodeJSON("reply_markup", &markup))
		assert.Empty(t, markup.InlineKeyboard)
		removed = append(removed, c.Int64("message_id"))
	}
	assert.Equal(t, messageIDs, removed)
}

const privateOnlyNoticeText = "<b>Прошу прощения</b> 🧐\n\nЭта команда работает только в <i>личной беседе</i> со мной. " +
	"Напиши мне в ЛС, и я с удовольствием помогу (я тебя там пинганул, если мы общались ранее)." +
	"\n\nЭто сообщение и твоя команда будут автоматически удалены через 10 секунд."

func assertPrivateOnlyRejection(t *testing.T, client *testutils.MockBotClient) {
	t.Helper()
	sends := client.CallsTo("sendMessage")
	require.Len(t, sends, 2, "private-only warning + ping in the private chat")
	assert.Equal(t, testGroupChatID, sends[0].Int64("chat_id"))
	assert.Equal(t, "HTML", sends[0].Params["parse_mode"])
	assert.Equal(t, privateOnlyNoticeText, sends[0].Params["text"])
	assert.Equal(t, testUserID, sends[1].Int64("chat_id"))
	assert.Empty(t, client.CallsTo("getChatMember"))
}

func assertClubOnlyRejection(t *testing.T, client *testutils.MockBotClient) {
	t.Helper()
	assert.Equal(t, []string{"Эта команда доступна только участникам клуба."}, client.SentTexts())
	member, ok := client.LastCallTo("getChatMember")
	require.True(t, ok)
	assert.Equal(t, testFullSuperGroupID, member.Int64("chat_id"))
	assert.Equal(t, testUserID, member.Int64("user_id"))
}
