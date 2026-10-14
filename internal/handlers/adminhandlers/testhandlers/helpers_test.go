package testhandlers

import (
	"database/sql/driver"
	"encoding/json"
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
	"github.com/stretchr/testify/require"
)

const (
	testAdminID             int64 = 1001
	testUserID              int64 = 2002
	testSuperGroupID        int64 = 2069889012
	testRandomCoffeeTopicID       = 33
	testIntroTopicID              = 44

	adminOnlyText = "Эта команда доступна только администраторам."
)

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

// testEnv bundles a fake bot with real sender/permission services; tests plug their handler in
type testEnv struct {
	t           *testing.T
	bot         *gotgbot.Bot
	client      *testutils.MockBotClient
	cfg         *config.Config
	log         *messageLog
	sender      *services.MessageSenderService
	permissions *services.PermissionsService
	handler     ext.Handler
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	bot, client := testutils.NewTestBot()
	client.SetMemberStatus(testAdminID, "administrator")
	log := &messageLog{nextID: 5000}
	client.On("sendMessage", log.respond)

	cfg := &config.Config{
		SuperGroupChatID:    testSuperGroupID,
		RandomCoffeeTopicID: testRandomCoffeeTopicID,
		IntroTopicID:        testIntroTopicID,
	}
	sender := services.NewMessageSenderService(bot)

	return &testEnv{
		t:           t,
		bot:         bot,
		client:      client,
		cfg:         cfg,
		log:         log,
		sender:      sender,
		permissions: services.NewPermissionsService(cfg, bot, sender),
	}
}

func fullChatID() int64 {
	return utils.ChatIdToFullChatId(testSuperGroupID)
}

// sendText dispatches a private text message and reports whether the handler accepted it
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

// dispatch runs the handler and requires it to succeed
func (e *testEnv) dispatch(update *gotgbot.Update) bool {
	e.t.Helper()
	handled, err := e.tryDispatch(update)
	require.NoError(e.t, err)
	return handled
}

// tryDispatch runs the handler and returns its error
func (e *testEnv) tryDispatch(update *gotgbot.Update) (bool, error) {
	e.t.Helper()
	require.NotNil(e.t, e.handler, "handler is not set")
	return testutils.Dispatch(e.bot, e.handler, update)
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

// deletedMessages returns "chatID/messageID" pairs of deleteMessage calls
func (e *testEnv) deletedMessages() [][2]int64 {
	var deleted [][2]int64
	for _, call := range e.client.CallsTo("deleteMessage") {
		deleted = append(deleted, [2]int64{call.Int64("chat_id"), call.Int64("message_id")})
	}
	return deleted
}

func msgRef(msg gotgbot.Message) [2]int64 {
	return [2]int64{msg.Chat.Id, msg.MessageId}
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

// futureMonday matches a time.Time argument that is a Monday at 00:00 UTC in the future
type futureMonday struct{}

func (futureMonday) Match(v driver.Value) bool {
	ts, ok := v.(time.Time)
	return ok && ts.Weekday() == time.Monday && ts.Hour() == 0 && ts.Minute() == 0 && ts.After(time.Now())
}

// repositoriesDB bundles the random coffee repositories built over one mock DB
type repositoriesDB struct {
	polls        *repositories.RandomCoffeePollRepository
	participants *repositories.RandomCoffeeParticipantRepository
	profiles     *repositories.ProfileRepository
	pairs        *repositories.RandomCoffeePairRepository
	users        *repositories.UserRepository
}

func newRepositoriesDB(t *testing.T) (*repositoriesDB, sqlmock.Sqlmock) {
	db, mock := testutils.NewMockDB(t)
	return &repositoriesDB{
		polls:        repositories.NewRandomCoffeePollRepository(db),
		participants: repositories.NewRandomCoffeeParticipantRepository(db),
		profiles:     repositories.NewProfileRepository(db),
		pairs:        repositories.NewRandomCoffeePairRepository(db),
		users:        repositories.NewUserRepository(db),
	}, mock
}

// newCoffeeService builds a real RandomCoffeeService on top of the env's fake bot and the given mock DB
func newCoffeeService(env *testEnv, db *repositoriesDB) *services.RandomCoffeeService {
	return services.NewRandomCoffeeService(
		env.bot,
		env.cfg,
		services.NewPollSenderService(env.bot),
		env.sender,
		db.polls,
		db.participants,
		db.profiles,
		db.pairs,
		db.users,
	)
}
