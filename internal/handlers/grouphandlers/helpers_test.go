package grouphandlers_test

import (
	"database/sql"
	"regexp"
	"testing"
	"time"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/handlers/grouphandlers"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/services/grouphandlersservices"
	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

const (
	testShortChatID int64 = 1234567890
	testChatID      int64 = -1001234567890

	testAdminUserID     int64 = 900001
	testUserID          int64 = 4242
	testTopicAdminID    int64 = 601
	groupAnonymousBotID int64 = 1087968824

	testForwardingTopicID = 1
	testContentTopicID    = 10
	testToolTopicID       = 11
	testClosedTopicID     = 20
	testOpenTopicID       = 30
)

var testCreatedAt = time.Unix(1700000000, 0).UTC()

var (
	userColumns         = []string{"id", "tg_id", "firstname", "lastname", "tg_username", "score", "has_coffee_ban", "is_club_member", "created_at", "updated_at"}
	groupMessageColumns = []string{"id", "message_id", "message_text", "reply_to_message_id", "user_tg_id", "group_topic_id", "created_at", "updated_at"}
	groupTopicColumns   = []string{"id", "topic_id", "name", "created_at", "updated_at"}
	pollColumns         = []string{"id", "message_id", "week_start_date", "telegram_poll_id", "created_at"}
)

var (
	sqlSelectUserByTgID        = regexp.QuoteMeta("FROM users WHERE tg_id = $1")
	sqlSelectGroupMessageByMsg = regexp.QuoteMeta("FROM group_messages WHERE message_id = $1")
	sqlInsertGroupMessage      = regexp.QuoteMeta("INSERT INTO group_messages (message_id, message_text, reply_to_message_id, user_tg_id, group_topic_id, created_at, updated_at)")
	sqlUpdateGroupMessage      = regexp.QuoteMeta("UPDATE group_messages SET message_text = $1, updated_at = NOW() WHERE id = $2")
	sqlDeleteGroupMessage      = regexp.QuoteMeta("DELETE FROM group_messages WHERE id = $1")
	sqlSelectGroupTopicByTopic = regexp.QuoteMeta("FROM group_topics WHERE topic_id = $1")
	sqlInsertGroupTopic        = regexp.QuoteMeta("INSERT INTO group_topics (topic_id, name)")
	sqlSelectPollByTelegramID  = regexp.QuoteMeta("FROM random_coffee_polls WHERE telegram_poll_id = $1")
	sqlUpsertCoffeeParticipant = regexp.QuoteMeta("INSERT INTO random_coffee_participants (poll_id, user_id, is_participating, created_at, updated_at)")
	sqlDeleteCoffeeParticipant = regexp.QuoteMeta("DELETE FROM random_coffee_participants WHERE poll_id = $1 AND user_id = $2")
)

func newTestConfig() *config.Config {
	return &config.Config{
		SuperGroupChatID:  testShortChatID,
		AdminUserID:       testAdminUserID,
		ClosedTopicsIDs:   []int{testClosedTopicID},
		ForwardingTopicID: testForwardingTopicID,
		ContentTopicID:    testContentTopicID,
		ToolTopicID:       testToolTopicID,
	}
}

// handlerEnv wires real repositories and services over the fake bot client and sqlmock
type handlerEnv struct {
	bot    *gotgbot.Bot
	client *testutils.MockBotClient
	db     *sql.DB
	mock   sqlmock.Sqlmock
	cfg    *config.Config

	userRepo         *repositories.UserRepository
	groupMessageRepo *repositories.GroupMessageRepository
	groupTopicRepo   *repositories.GroupTopicRepository
	sender           *services.MessageSenderService
	saveUpdate       *grouphandlersservices.SaveUpdateMessageService
}

func newHandlerEnv(t *testing.T) *handlerEnv {
	t.Helper()
	bot, client := testutils.NewTestBot()
	db, mock := testutils.NewMockDB(t)
	cfg := newTestConfig()
	client.SetMemberStatus(testTopicAdminID, "administrator")

	e := &handlerEnv{
		bot:              bot,
		client:           client,
		db:               db,
		mock:             mock,
		cfg:              cfg,
		userRepo:         repositories.NewUserRepository(db),
		groupMessageRepo: repositories.NewGroupMessageRepository(db),
		groupTopicRepo:   repositories.NewGroupTopicRepository(db),
		sender:           services.NewMessageSenderService(bot),
	}
	e.saveUpdate = grouphandlersservices.NewSaveUpdateMessageService(e.groupMessageRepo, e.userRepo, cfg, bot)
	return e
}

func (e *handlerEnv) messageHandler() ext.Handler {
	return grouphandlers.NewMessageHandler(
		e.cfg,
		e.sender,
		grouphandlersservices.NewCleanClosedThreadsService(e.cfg, e.sender, e.groupTopicRepo),
		grouphandlersservices.NewRepliesFromClosedThreadsService(e.cfg, e.sender, e.groupTopicRepo, e.saveUpdate),
		grouphandlersservices.NewDeleteJoinLeftMessagesService(),
		grouphandlersservices.NewSaveTopicService(e.groupTopicRepo),
		grouphandlersservices.NewAdminSaveMessageService(e.groupMessageRepo, e.bot, e.cfg, e.saveUpdate, e.sender),
		grouphandlersservices.NewSaveMessageService(e.groupMessageRepo, e.saveUpdate, e.cfg),
	)
}

func (e *handlerEnv) pollAnswerHandler() ext.Handler {
	return grouphandlers.NewPollAnswerHandler(grouphandlersservices.NewRandomCoffeePollAnswersService(
		e.sender,
		e.cfg,
		repositories.NewRandomCoffeePollRepository(e.db),
		repositories.NewRandomCoffeeParticipantRepository(e.db),
		e.userRepo,
	))
}

func (e *handlerEnv) chatMemberHandler() ext.Handler {
	return grouphandlers.NewChatMemberHandler(e.cfg, grouphandlersservices.NewJoinLeftService(e.userRepo))
}

func groupAnonymousBot() *gotgbot.User {
	return &gotgbot.User{Id: groupAnonymousBotID, IsBot: true, FirstName: "Group", Username: "GroupAnonymousBot"}
}

func newGroupMsg(userID int64, threadID int64, text string) *gotgbot.Message {
	return testutils.NewGroupMessage(userID, testChatID, threadID, text)
}

// asReplyTo turns msg into a reply to a post (authored by testUserID+1) in the same topic
func asReplyTo(msg *gotgbot.Message, targetID int64) *gotgbot.Message {
	msg.ReplyToMessage = &gotgbot.Message{
		MessageId:       targetID,
		Chat:            msg.Chat,
		MessageThreadId: msg.MessageThreadId,
		IsTopicMessage:  msg.IsTopicMessage,
		From:            testutils.NewUser(testUserID + 1),
		Date:            1700000000,
		Text:            "original post",
	}
	return msg
}

func userRow(dbID int, tgID int64, hasCoffeeBan bool) *sqlmock.Rows {
	u := testutils.NewUser(tgID)
	return sqlmock.NewRows(userColumns).
		AddRow(dbID, tgID, u.FirstName, u.LastName, u.Username, 0, hasCoffeeBan, true, testCreatedAt, testCreatedAt)
}

func expectExistingUser(mock sqlmock.Sqlmock, dbID int, tgID int64) {
	mock.ExpectQuery(sqlSelectUserByTgID).WithArgs(tgID).WillReturnRows(userRow(dbID, tgID, false))
}

func expectGroupMessageInsert(mock sqlmock.Sqlmock, messageID int64, text string, replyTo any, userTgID int64, topicID int64) {
	mock.ExpectQuery(sqlInsertGroupMessage).
		WithArgs(messageID, text, replyTo, userTgID, topicID, testCreatedAt).
		WillReturnRows(sqlmock.NewRows(groupMessageColumns).
			AddRow(1, messageID, text, nil, userTgID, topicID, testCreatedAt, testCreatedAt))
}

func expectGroupTopic(mock sqlmock.Sqlmock, topicID int64, name string) {
	mock.ExpectQuery(sqlSelectGroupTopicByTopic).WithArgs(topicID).
		WillReturnRows(sqlmock.NewRows(groupTopicColumns).AddRow(int(topicID)+100, topicID, name, testCreatedAt, testCreatedAt))
}

func groupMessageRow(id int, messageID int64, text string, userTgID int64, topicID int64) *sqlmock.Rows {
	return sqlmock.NewRows(groupMessageColumns).
		AddRow(id, messageID, text, nil, userTgID, topicID, testCreatedAt, testCreatedAt)
}

// methods returns the Bot API methods called, in order
func methods(client *testutils.MockBotClient) []string {
	var result []string
	for _, c := range client.Calls() {
		result = append(result, c.Method)
	}
	return result
}
