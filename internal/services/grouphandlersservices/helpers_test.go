package grouphandlersservices

import (
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/require"
)

const (
	testShortChatID int64 = 1234567890
	testChatID      int64 = -1001234567890
	testChatLinkID        = "1234567890"

	testAdminUserID     int64 = 900001
	testUserID          int64 = 4242
	groupAnonymousBotID int64 = 1087968824

	testForwardingTopicID   = 1
	testContentTopicID      = 10
	testToolTopicID         = 11
	testClosedTopicID       = 20
	testSecondClosedTopicID = 21
	testOpenTopicID         = 30

	// replyPrefix mirrors the prefix used by RepliesFromClosedThreadsService (latin "o" + cyrillic "твет")
	replyPrefix = "↩️ o" + "твет"
)

var (
	testCreatedAt = time.Unix(1700000000, 0).UTC()
	errDB         = errors.New("db is down")
)

// SQL column sets returned by repositories
var (
	userColumns         = []string{"id", "tg_id", "firstname", "lastname", "tg_username", "score", "has_coffee_ban", "is_club_member", "created_at", "updated_at"}
	profileColumns      = []string{"id", "user_id", "bio", "published_message_id", "created_at", "updated_at"}
	groupMessageColumns = []string{"id", "message_id", "message_text", "reply_to_message_id", "user_tg_id", "group_topic_id", "created_at", "updated_at"}
	groupTopicColumns   = []string{"id", "topic_id", "name", "created_at", "updated_at"}
	pollColumns         = []string{"id", "message_id", "week_start_date", "telegram_poll_id", "created_at"}
)

// SQL patterns (sqlmock collapses whitespace before regexp matching)
var (
	sqlSelectUserByTgID        = regexp.QuoteMeta("FROM users WHERE tg_id = $1")
	sqlSelectUserByID          = regexp.QuoteMeta("FROM users WHERE id = $1")
	sqlInsertUser              = regexp.QuoteMeta("INSERT INTO users")
	sqlSetClubMember           = regexp.QuoteMeta("UPDATE users SET is_club_member = $1, updated_at = NOW() WHERE id = $2")
	sqlSelectProfileByUserID   = regexp.QuoteMeta("FROM profiles WHERE user_id = $1")
	sqlInsertProfile           = regexp.QuoteMeta("INSERT INTO profiles (user_id, bio)")
	sqlSelectGroupMessageByMsg = regexp.QuoteMeta("FROM group_messages WHERE message_id = $1")
	sqlInsertGroupMessage      = regexp.QuoteMeta("INSERT INTO group_messages (message_id, message_text, reply_to_message_id, user_tg_id, group_topic_id, created_at, updated_at)")
	sqlUpdateGroupMessage      = regexp.QuoteMeta("UPDATE group_messages SET message_text = $1, updated_at = NOW() WHERE id = $2")
	sqlDeleteGroupMessage      = regexp.QuoteMeta("DELETE FROM group_messages WHERE id = $1")
	sqlSelectGroupTopicByTopic = regexp.QuoteMeta("FROM group_topics WHERE topic_id = $1")
	sqlInsertGroupTopic        = regexp.QuoteMeta("INSERT INTO group_topics (topic_id, name)")
	sqlUpdateGroupTopic        = regexp.QuoteMeta("UPDATE group_topics SET name = $1, updated_at = NOW() WHERE topic_id = $2")
	sqlSelectPollByTelegramID  = regexp.QuoteMeta("FROM random_coffee_polls WHERE telegram_poll_id = $1")
	sqlUpsertCoffeeParticipant = regexp.QuoteMeta("INSERT INTO random_coffee_participants (poll_id, user_id, is_participating, created_at, updated_at)")
	sqlDeleteCoffeeParticipant = regexp.QuoteMeta("DELETE FROM random_coffee_participants WHERE poll_id = $1 AND user_id = $2")
)

func newTestConfig() *config.Config {
	return &config.Config{
		SuperGroupChatID:  testShortChatID,
		AdminUserID:       testAdminUserID,
		ClosedTopicsIDs:   []int{testClosedTopicID, testSecondClosedTopicID},
		ForwardingTopicID: testForwardingTopicID,
		ContentTopicID:    testContentTopicID,
		ToolTopicID:       testToolTopicID,
	}
}

// testDeps bundles real repositories and services built on the fake bot and sqlmock
type testDeps struct {
	bot    *gotgbot.Bot
	client *testutils.MockBotClient
	db     *sql.DB
	mock   sqlmock.Sqlmock
	cfg    *config.Config

	userRepo         *repositories.UserRepository
	groupMessageRepo *repositories.GroupMessageRepository
	groupTopicRepo   *repositories.GroupTopicRepository
	sender           *services.MessageSenderService
	saveUpdate       *SaveUpdateMessageService
}

func newTestDeps(t *testing.T) *testDeps {
	t.Helper()
	bot, client := testutils.NewTestBot()
	db, mock := testutils.NewMockDB(t)
	cfg := newTestConfig()

	d := &testDeps{
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
	d.saveUpdate = NewSaveUpdateMessageService(d.groupMessageRepo, d.userRepo, cfg, bot)
	return d
}

func groupAnonymousBot() *gotgbot.User {
	return &gotgbot.User{Id: groupAnonymousBotID, IsBot: true, FirstName: "Group", Username: "GroupAnonymousBot"}
}

// newGroupMsg builds a text message in the test supergroup inside the given topic
func newGroupMsg(userID int64, threadID int64, text string) *gotgbot.Message {
	return testutils.NewGroupMessage(userID, testChatID, threadID, text)
}

// asReplyTo turns msg into a reply to targetID (which lives in the same topic)
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

// userRow returns a users row matching testutils.NewUser(tgID), so GetOrCreate performs no update
func userRow(dbID int, tgID int64, hasCoffeeBan bool, isClubMember bool) *sqlmock.Rows {
	u := testutils.NewUser(tgID)
	return sqlmock.NewRows(userColumns).
		AddRow(dbID, tgID, u.FirstName, u.LastName, u.Username, 0, hasCoffeeBan, isClubMember, testCreatedAt, testCreatedAt)
}

func expectExistingUser(mock sqlmock.Sqlmock, dbID int, tgID int64) {
	mock.ExpectQuery(sqlSelectUserByTgID).WithArgs(tgID).WillReturnRows(userRow(dbID, tgID, false, true))
}

func expectExistingProfile(mock sqlmock.Sqlmock, profileID int, userID int) {
	mock.ExpectQuery(sqlSelectProfileByUserID).WithArgs(userID).
		WillReturnRows(sqlmock.NewRows(profileColumns).AddRow(profileID, userID, "", nil, testCreatedAt, testCreatedAt))
}

func groupMessageRow(id int, messageID int64, text string, userTgID int64, topicID int64) *sqlmock.Rows {
	return sqlmock.NewRows(groupMessageColumns).
		AddRow(id, messageID, text, nil, userTgID, topicID, testCreatedAt, testCreatedAt)
}

func expectGroupMessageNotFound(mock sqlmock.Sqlmock, messageID int64) {
	mock.ExpectQuery(sqlSelectGroupMessageByMsg).WithArgs(messageID).WillReturnError(sql.ErrNoRows)
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

func expectGroupTopicNotFound(mock sqlmock.Sqlmock, topicID int64) {
	mock.ExpectQuery(sqlSelectGroupTopicByTopic).WithArgs(topicID).WillReturnError(sql.ErrNoRows)
}

// decodeEntities decodes a JSON-encoded entities param of a recorded call
func decodeEntities(t *testing.T, call testutils.APICall, key string) []gotgbot.MessageEntity {
	t.Helper()
	var entities []gotgbot.MessageEntity
	require.NoError(t, call.DecodeJSON(key, &entities))
	return entities
}
