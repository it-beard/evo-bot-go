package grouphandlers_test

import (
	"regexp"
	"testing"

	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func chatMemberUpdate(chatType string, newMember gotgbot.ChatMember) *gotgbot.Update {
	return chatMemberUpdateIn(testChatID, chatType, newMember)
}

func chatMemberUpdateIn(chatID int64, chatType string, newMember gotgbot.ChatMember) *gotgbot.Update {
	user := testutils.NewUser(testUserID)
	return &gotgbot.Update{
		UpdateId: 1,
		ChatMember: &gotgbot.ChatMemberUpdated{
			Chat:          gotgbot.Chat{Id: chatID, Type: chatType},
			From:          *user,
			Date:          1700000000,
			OldChatMember: gotgbot.ChatMemberLeft{User: *user},
			NewChatMember: newMember,
		},
	}
}

func TestChatMemberHandler_CheckUpdate(t *testing.T) {
	e := newHandlerEnv(t)
	h := e.chatMemberHandler()
	member := gotgbot.ChatMemberMember{User: *testutils.NewUser(testUserID)}

	assert.True(t, h.CheckUpdate(e.bot, ext.NewContext(e.bot, chatMemberUpdate(testutils.SuperGroupChatType, member), nil)))
	assert.False(t, h.CheckUpdate(e.bot, ext.NewContext(e.bot, testutils.MessageUpdate(newGroupMsg(testUserID, 0, "hi")), nil)))
	assert.False(t, h.CheckUpdate(e.bot, ext.NewContext(e.bot, testutils.PollAnswerUpdate(testUserID, "p", []int64{0}), nil)))
}

func TestChatMemberHandler_SupergroupJoinMarksUserAsClubMember(t *testing.T) {
	e := newHandlerEnv(t)
	member := gotgbot.ChatMemberMember{User: *testutils.NewUser(testUserID)}

	expectExistingUser(e.mock, 7, testUserID)
	e.mock.ExpectQuery(regexp.QuoteMeta("FROM profiles WHERE user_id = $1")).WithArgs(7).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "bio", "published_message_id", "created_at", "updated_at"}).
			AddRow(70, 7, "", nil, testCreatedAt, testCreatedAt))
	e.mock.ExpectExec(regexp.QuoteMeta("UPDATE users SET is_club_member = $1")).WithArgs(true, 7).
		WillReturnResult(sqlmock.NewResult(0, 1))

	handled, err := testutils.Dispatch(e.bot, e.chatMemberHandler(), chatMemberUpdate(testutils.SuperGroupChatType, member))

	require.NoError(t, err)
	assert.True(t, handled)
}

func TestChatMemberHandler_NonSupergroupUpdateIsIgnored(t *testing.T) {
	e := newHandlerEnv(t)
	member := gotgbot.ChatMemberMember{User: *testutils.NewUser(testUserID)}

	handled, err := testutils.Dispatch(e.bot, e.chatMemberHandler(), chatMemberUpdate("group", member))

	require.NoError(t, err)
	assert.True(t, handled)
}

func TestChatMemberHandler_LeaveMarksUserAsNotClubMember(t *testing.T) {
	e := newHandlerEnv(t)
	left := gotgbot.ChatMemberLeft{User: *testutils.NewUser(testUserID)}

	expectExistingUser(e.mock, 7, testUserID)
	e.mock.ExpectQuery(regexp.QuoteMeta("FROM profiles WHERE user_id = $1")).WithArgs(7).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "bio", "published_message_id", "created_at", "updated_at"}).
			AddRow(70, 7, "", nil, testCreatedAt, testCreatedAt))
	e.mock.ExpectExec(regexp.QuoteMeta("UPDATE users SET is_club_member = $1")).WithArgs(false, 7).
		WillReturnResult(sqlmock.NewResult(0, 1))

	handled, err := testutils.Dispatch(e.bot, e.chatMemberHandler(), chatMemberUpdate(testutils.SuperGroupChatType, left))

	require.NoError(t, err)
	assert.True(t, handled)
}

func TestChatMemberHandler_OtherSupergroupIsIgnored(t *testing.T) {
	e := newHandlerEnv(t)
	member := gotgbot.ChatMemberMember{User: *testutils.NewUser(testUserID)}

	// The bot was added to someone else's supergroup: nothing may be written to the club's database
	handled, err := testutils.Dispatch(e.bot, e.chatMemberHandler(),
		chatMemberUpdateIn(-1009999999999, testutils.SuperGroupChatType, member))

	require.NoError(t, err)
	assert.True(t, handled)
	require.NoError(t, e.mock.ExpectationsWereMet())
}
