package privatehandlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"evo-bot-go/internal/clients"
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/conversation"
	"github.com/openai/openai-go/v2/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSuperGroupID     int64 = 1234567890
	testFullSuperGroupID int64 = -1001234567890
	testAdminUserID      int64 = 9001
	testContentTopicID         = 11
	testToolTopicID            = 12
	testIntroTopicID           = 13
	testCoffeeTopicID          = 14
	testUserID           int64 = 501
	testGroupChatID      int64 = -100500
)

// The AI search handlers write the last prompt into "last-prompt-log.txt" in the working directory.
// Run the package tests from a temporary directory so the source tree stays clean.
func TestMain(m *testing.M) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	dir, err := os.MkdirTemp("", "privatehandlers-test-")
	if err != nil {
		panic(err)
	}
	if err := os.Chdir(dir); err != nil {
		panic(err)
	}

	code := m.Run()

	_ = os.Chdir(wd)
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// deps holds every real service/repository a private handler needs, built over the test fakes
type deps struct {
	bot         *gotgbot.Bot
	client      *testutils.MockBotClient
	cfg         *config.Config
	db          *sql.DB
	sql         sqlmock.Sqlmock
	openai      *clients.OpenAiClient
	openaiMock  *testutils.MockOpenAI
	sender      *services.MessageSenderService
	permissions *services.PermissionsService

	sentMu   sync.Mutex
	sent     []sentMessage
	failHTML atomic.Bool
}

// sentMessage is a message delivered through the fake sendMessage endpoint
type sentMessage struct {
	id     int64
	chatID int64
	text   string
}

func newDeps(t *testing.T) *deps {
	t.Helper()

	bot, client := testutils.NewTestBot()
	db, mock := testutils.NewMockDB(t)
	openaiClient, openaiMock := testutils.NewMockOpenAI(t)
	cfg := &config.Config{
		SuperGroupChatID:    testSuperGroupID,
		AdminUserID:         testAdminUserID,
		ContentTopicID:      testContentTopicID,
		ToolTopicID:         testToolTopicID,
		IntroTopicID:        testIntroTopicID,
		RandomCoffeeTopicID: testCoffeeTopicID,
	}
	sender := services.NewMessageSenderService(bot)

	d := &deps{
		bot:         bot,
		client:      client,
		cfg:         cfg,
		db:          db,
		sql:         mock,
		openai:      openaiClient,
		openaiMock:  openaiMock,
		sender:      sender,
		permissions: services.NewPermissionsService(cfg, bot, sender),
	}
	client.On("sendMessage", d.sendMessageResponse)
	return d
}

// sendMessageResponse answers sendMessage calls like the default fake, but remembers the id of every
// delivered message (ids survive client.Reset()) and can reject HTML messages (see failHTMLSends).
func (d *deps) sendMessageResponse(call testutils.APICall) (json.RawMessage, error) {
	if d.failHTML.Load() && call.Params["parse_mode"] == "HTML" {
		return nil, &gotgbot.TelegramError{Method: "sendMessage", Code: 400, Description: "Bad Request: can't parse entities"}
	}

	chatID := call.Int64("chat_id")
	chatType := "private"
	if chatID < 0 {
		chatType = "supergroup"
	}

	d.sentMu.Lock()
	id := int64(900001 + len(d.sent))
	d.sent = append(d.sent, sentMessage{id: id, chatID: chatID, text: call.Params["text"]})
	d.sentMu.Unlock()

	return json.Marshal(gotgbot.Message{
		MessageId:       id,
		Chat:            gotgbot.Chat{Id: chatID, Type: chatType},
		Date:            1700000000,
		Text:            call.Params["text"],
		MessageThreadId: call.Int64("message_thread_id"),
	})
}

// failHTMLSends makes every HTML sendMessage fail while other messages are still delivered
func (d *deps) failHTMLSends() {
	d.failHTML.Store(true)
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

// lastSentID returns the id of the last delivered message whose text contains substr
func (d *deps) lastSentID(t *testing.T, substr string) int64 {
	t.Helper()
	d.sentMu.Lock()
	defer d.sentMu.Unlock()
	for i := len(d.sent) - 1; i >= 0; i-- {
		if strings.Contains(d.sent[i].text, substr) {
			return d.sent[i].id
		}
	}
	require.Failf(t, "message not sent", "no delivered message contains %q", substr)
	return 0
}

func (d *deps) newHelpHandler() ext.Handler {
	return NewHelpHandler(d.cfg, d.sender, d.permissions)
}

func (d *deps) newEventsHandler() ext.Handler {
	return NewEventsHandler(d.cfg, repositories.NewEventRepository(d.db), d.sender, d.permissions)
}

func (d *deps) newContentHandler() ext.Handler {
	return NewContentHandler(
		d.cfg,
		d.openai,
		d.sender,
		repositories.NewPromptingTemplateRepository(d.db),
		repositories.NewGroupMessageRepository(d.db),
		d.permissions,
	)
}

func (d *deps) newToolsHandler() ext.Handler {
	return NewToolsHandler(
		d.cfg,
		d.openai,
		d.sender,
		repositories.NewPromptingTemplateRepository(d.db),
		repositories.NewGroupMessageRepository(d.db),
		repositories.NewGroupTopicRepository(d.db),
		d.permissions,
	)
}

func (d *deps) newIntroHandler() ext.Handler {
	return NewIntroHandler(
		d.cfg,
		d.openai,
		d.sender,
		repositories.NewPromptingTemplateRepository(d.db),
		repositories.NewProfileRepository(d.db),
		d.permissions,
	)
}

func (d *deps) newProfileHandler() ext.Handler {
	return NewProfileHandler(
		d.cfg,
		d.sender,
		d.permissions,
		services.NewProfileService(d.bot),
		repositories.NewUserRepository(d.db),
		repositories.NewProfileRepository(d.db),
		repositories.NewPromptingTemplateRepository(d.db),
		d.openai,
	)
}

// dispatch runs the update through the handler and fails the test on a handler error
func (d *deps) dispatch(t *testing.T, h ext.Handler, u *gotgbot.Update) bool {
	t.Helper()
	handled, err := testutils.Dispatch(d.bot, h, u)
	require.NoError(t, err)
	return handled
}

// ---------- update builders ----------

func privateText(userID int64, text string) *gotgbot.Update {
	return testutils.MessageUpdate(testutils.NewPrivateMessage(userID, text))
}

func groupText(userID int64, text string) *gotgbot.Update {
	return testutils.MessageUpdate(testutils.NewGroupMessage(userID, testGroupChatID, 0, text))
}

// ---------- Bot API assertions ----------

func lastSend(t *testing.T, client *testutils.MockBotClient) testutils.APICall {
	t.Helper()
	call, ok := client.LastCallTo("sendMessage")
	require.True(t, ok, "expected a sendMessage call")
	return call
}

// sendWithText returns the first sendMessage call whose text equals the given text
func sendWithText(t *testing.T, client *testutils.MockBotClient, text string) testutils.APICall {
	t.Helper()
	for _, c := range client.CallsTo("sendMessage") {
		if c.Params["text"] == text {
			return c
		}
	}
	require.Failf(t, "sendMessage not found", "no sendMessage with text %q in %q", text, client.SentTexts())
	return testutils.APICall{}
}

func keyboard(t *testing.T, call testutils.APICall) gotgbot.InlineKeyboardMarkup {
	t.Helper()
	var markup gotgbot.InlineKeyboardMarkup
	require.NoError(t, call.DecodeJSON("reply_markup", &markup))
	return markup
}

// callbackData flattens all callback_data values of an inline keyboard
func callbackData(t *testing.T, call testutils.APICall) []string {
	t.Helper()
	var data []string
	for _, row := range keyboard(t, call).InlineKeyboard {
		for _, b := range row {
			data = append(data, b.CallbackData)
		}
	}
	return data
}

// deletedMessageIDs returns the message ids of all deleteMessage calls
func deletedMessageIDs(client *testutils.MockBotClient) []int64 {
	var ids []int64
	for _, c := range client.CallsTo("deleteMessage") {
		ids = append(ids, c.Int64("message_id"))
	}
	return ids
}

// privateOnlyText is the auto-deleted notice sent when a command is used outside of a private chat
const privateOnlyText = "<b>Прошу прощения</b> 🧐\n\nЭта команда работает только в <i>личной беседе</i> со мной. " +
	"Напиши мне в ЛС, и я с удовольствием помогу (я тебя там пинганул, если мы общались ранее)." +
	"\n\nЭто сообщение и твоя команда будут автоматически удалены через 10 секунд."

// assertPrivateOnlyRejection checks the reply sent for a command used outside of a private chat
func assertPrivateOnlyRejection(t *testing.T, client *testutils.MockBotClient) {
	t.Helper()
	sends := client.CallsTo("sendMessage")
	require.Len(t, sends, 2, "private-only warning + ping in the private chat")
	assert.Equal(t, testGroupChatID, sends[0].Int64("chat_id"))
	assert.Equal(t, privateOnlyText, sends[0].Params["text"])
	assert.Equal(t, "HTML", sends[0].Params["parse_mode"])
	assert.Equal(t, testUserID, sends[1].Int64("chat_id"))
	assert.Empty(t, client.CallsTo("getChatMember"))
}

// conversationState returns the conversation state testUserID is in within their private chat,
// or "" when no conversation is running
func conversationState(t *testing.T, d *deps, h ext.Handler) string {
	t.Helper()
	conv, ok := h.(handlers.Conversation)
	require.True(t, ok, "the handler is a conversation")
	state, err := conv.StateStorage.Get(ext.NewContext(d.bot, privateText(testUserID, "state probe"), nil))
	if errors.Is(err, conversation.ErrKeyNotFound) {
		return ""
	}
	require.NoError(t, err)
	return state.Key
}

// assertClubOnlyRejection checks the reply sent to a user who is not a club member
func assertClubOnlyRejection(t *testing.T, client *testutils.MockBotClient) {
	t.Helper()
	assert.Equal(t, []string{"Эта команда доступна только участникам клуба."}, client.SentTexts())
	member, ok := client.LastCallTo("getChatMember")
	require.True(t, ok)
	assert.Equal(t, testFullSuperGroupID, member.Int64("chat_id"))
	assert.Equal(t, testUserID, member.Int64("user_id"))
}

// ---------- SQL fixtures ----------

var (
	testTime          = time.Date(2024, 5, 1, 10, 0, 0, 0, time.UTC)
	eventColumns      = []string{"id", "name", "type", "status", "started_at", "created_at", "updated_at"}
	groupMsgColumns   = []string{"id", "message_id", "message_text", "reply_to_message_id", "user_tg_id", "group_topic_id", "created_at", "updated_at"}
	groupTopicColumns = []string{"id", "topic_id", "name", "created_at", "updated_at"}
	userColumns       = []string{"id", "tg_id", "firstname", "lastname", "tg_username", "score", "has_coffee_ban", "is_club_member", "created_at", "updated_at"}
	profileColumns    = []string{"id", "user_id", "bio", "published_message_id", "created_at", "updated_at"}
)

const (
	selectTemplateSQL    = `SELECT template_text FROM prompting_templates WHERE template_key = \$1`
	insertTemplateSQL    = `INSERT INTO prompting_templates \(template_key, template_text\) VALUES \(\$1, \$2\) ON CONFLICT \(template_key\) DO NOTHING`
	selectGroupMsgsSQL   = `FROM group_messages\s+WHERE group_topic_id = \$1\s+ORDER BY created_at DESC`
	selectGroupTopicSQL  = `FROM group_topics\s+WHERE topic_id = \$1`
	selectActiveEvents   = `FROM events\s+WHERE status = \$1 AND \(started_at IS NULL OR started_at > NOW\(\)\)\s+ORDER BY started_at ASC NULLS LAST\s+LIMIT \$2`
	selectProfilesActive = `FROM profiles p\s+INNER JOIN users u ON p.user_id = u.id`
	selectUserByTgID     = `FROM users\s+WHERE tg_id = \$1`
	selectUserByUsername = `FROM users\s+WHERE tg_username = \$1`
	selectUserByName     = `FROM users\s+WHERE LOWER\(firstname\) = LOWER\(\$1\) AND LOWER\(lastname\) = LOWER\(\$2\)`
	selectProfileByUser  = `FROM profiles\s+WHERE user_id = \$1`
)

func (d *deps) expectTemplate(key string, template string) {
	d.sql.ExpectQuery(selectTemplateSQL).WithArgs(key).
		WillReturnRows(sqlmock.NewRows([]string{"template_text"}).AddRow(template))
}

func (d *deps) expectGroupMessages(topicID int, rows *sqlmock.Rows) {
	d.sql.ExpectQuery(selectGroupMsgsSQL).WithArgs(int64(topicID)).WillReturnRows(rows)
}

func groupMessageRows(messages ...repositories.GroupMessage) *sqlmock.Rows {
	rows := sqlmock.NewRows(groupMsgColumns)
	for _, m := range messages {
		rows.AddRow(m.ID, m.MessageID, m.MessageText, nil, m.UserTgID, m.GroupTopicID, m.CreatedAt, m.UpdatedAt)
	}
	return rows
}

type testDBUser struct {
	ID        int
	TgID      int64
	Firstname string
	Lastname  string
	Username  string
}

type testDBProfile struct {
	ID          int
	UserID      int
	Bio         string
	PublishedID any // nil or int64
}

func userRows(u testDBUser) *sqlmock.Rows {
	return sqlmock.NewRows(userColumns).
		AddRow(u.ID, u.TgID, u.Firstname, u.Lastname, u.Username, 0, false, true, testTime, testTime)
}

func profileRows(p testDBProfile) *sqlmock.Rows {
	return sqlmock.NewRows(profileColumns).
		AddRow(p.ID, p.UserID, p.Bio, p.PublishedID, testTime, testTime)
}

// ---------- blocking OpenAI server ----------

// blockingOpenAI is a fake OpenAI server that holds chat completion requests until released
// (or until the client cancels the request). It lets tests observe handler behaviour while
// a search is still in progress.
type blockingOpenAI struct {
	started   chan struct{}
	release   chan struct{}
	cancelled chan struct{}
	once      sync.Once
}

func newBlockingOpenAI(t *testing.T, completion string) (*clients.OpenAiClient, *blockingOpenAI) {
	t.Helper()

	b := &blockingOpenAI{
		started:   make(chan struct{}, 10),
		release:   make(chan struct{}),
		cancelled: make(chan struct{}, 10),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Consume the body so the server notices when the client aborts the request
		_, _ = io.ReadAll(r.Body)
		b.started <- struct{}{}
		select {
		case <-b.release:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":      "chatcmpl-test",
				"object":  "chat.completion",
				"created": 1700000000,
				"model":   "gpt-5-mini",
				"choices": []any{map[string]any{
					"index":         0,
					"finish_reason": "stop",
					"message":       map[string]any{"role": "assistant", "content": completion},
				}},
			})
		case <-r.Context().Done():
			b.cancelled <- struct{}{}
		}
	}))
	t.Cleanup(server.Close)
	t.Cleanup(b.Release) // runs before server.Close

	client := clients.NewOpenAiClientWithOptions(
		option.WithAPIKey("test-key"),
		option.WithBaseURL(server.URL),
		option.WithMaxRetries(0),
	)
	return client, b
}

// Release lets all pending and future requests complete
func (b *blockingOpenAI) Release() {
	b.once.Do(func() { close(b.release) })
}

func (b *blockingOpenAI) waitStarted(t *testing.T) {
	t.Helper()
	select {
	case <-b.started:
	case <-time.After(5 * time.Second):
		t.Fatal("OpenAI request was not started")
	}
}

func (b *blockingOpenAI) waitCancelled(t *testing.T) {
	t.Helper()
	select {
	case <-b.cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("OpenAI request was not cancelled")
	}
}

// dispatchAsync dispatches an update in a goroutine and returns a channel with the handler error
func (d *deps) dispatchAsync(h ext.Handler, u *gotgbot.Update) <-chan error {
	done := make(chan error, 1)
	go func() {
		_, err := testutils.Dispatch(d.bot, h, u)
		done <- err
	}()
	return done
}

func waitDone(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("handler did not finish")
		return nil
	}
}

// activeProfileRow is a row of ProfileRepository.GetAllActiveWithUserInfo
type activeProfileRow struct {
	ProfileID   int
	UserID      int
	Bio         string
	PublishedID any // nil or int64
	TgID        int64
	Firstname   string
	Lastname    string
	Username    string
}

func (d *deps) expectActiveProfiles(profiles ...activeProfileRow) {
	rows := sqlmock.NewRows([]string{
		"p_id", "p_user_id", "p_bio", "p_published_message_id", "p_created_at", "p_updated_at",
		"u_id", "u_tg_id", "u_firstname", "u_lastname", "u_tg_username", "u_score", "u_has_coffee_ban",
		"u_is_club_member", "u_created_at", "u_updated_at",
	})
	for _, p := range profiles {
		rows.AddRow(p.ProfileID, p.UserID, p.Bio, p.PublishedID, testTime, testTime,
			p.UserID, p.TgID, p.Firstname, p.Lastname, p.Username, 0, false, true, testTime, testTime)
	}
	d.sql.ExpectQuery(selectProfilesActive).WillReturnRows(rows)
}
