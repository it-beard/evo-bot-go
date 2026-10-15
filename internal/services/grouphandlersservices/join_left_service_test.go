package grouphandlersservices

import (
	"testing"

	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func chatMemberCtx(bot *gotgbot.Bot, newMember gotgbot.ChatMember) *ext.Context {
	user := testutils.NewUser(testUserID)
	update := &gotgbot.Update{
		UpdateId: 1,
		ChatMember: &gotgbot.ChatMemberUpdated{
			Chat:          gotgbot.Chat{Id: testChatID, Type: testutils.SuperGroupChatType},
			From:          *user,
			Date:          1700000000,
			OldChatMember: gotgbot.ChatMemberLeft{User: *user},
			NewChatMember: newMember,
		},
	}
	return ext.NewContext(bot, update, nil)
}

func memberWithStatus(status string) gotgbot.ChatMember {
	user := *testutils.NewUser(testUserID)
	switch status {
	case "member":
		return gotgbot.ChatMemberMember{User: user}
	case "administrator":
		return gotgbot.ChatMemberAdministrator{User: user}
	case "creator":
		return gotgbot.ChatMemberOwner{User: user}
	case "left":
		return gotgbot.ChatMemberLeft{User: user}
	case "kicked":
		return gotgbot.ChatMemberBanned{User: user}
	case "restricted":
		return gotgbot.ChatMemberRestricted{User: user, IsMember: true}
	case "restricted-left":
		return gotgbot.ChatMemberRestricted{User: user, IsMember: false}
	}
	panic("unknown status " + status)
}

func expectExistingUserWithMembership(mock sqlmock.Sqlmock, dbID int, isClubMember bool) {
	mock.ExpectQuery(sqlSelectUserByTgID).WithArgs(testUserID).WillReturnRows(userRow(dbID, testUserID, false, isClubMember))
	expectExistingProfile(mock, 70, dbID)
}

func TestJoinLeftService_HandleJoinLeftMember_BecameMemberSetsClubMember(t *testing.T) {
	for _, status := range []string{"member", "administrator", "creator", "restricted"} {
		t.Run(status, func(t *testing.T) {
			d := newTestDeps(t)
			s := NewJoinLeftService(d.userRepo)

			expectExistingUserWithMembership(d.mock, 7, false)
			d.mock.ExpectExec(sqlSetClubMember).WithArgs(true, 7).WillReturnResult(sqlmock.NewResult(0, 1))

			require.NoError(t, s.HandleJoinLeftMember(d.bot, chatMemberCtx(d.bot, memberWithStatus(status))))
			assert.Empty(t, d.client.Calls())
		})
	}
}

func TestJoinLeftService_HandleJoinLeftMember_NewUserIsCreatedWithProfile(t *testing.T) {
	d := newTestDeps(t)
	s := NewJoinLeftService(d.userRepo)
	u := testutils.NewUser(testUserID)

	// user does not exist yet -> created
	d.mock.ExpectQuery(sqlSelectUserByTgID).WithArgs(testUserID).WillReturnRows(sqlmock.NewRows(userColumns))
	d.mock.ExpectQuery(sqlInsertUser).WithArgs(testUserID, u.FirstName, u.LastName, u.Username).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
	d.mock.ExpectQuery(sqlSelectUserByID).WithArgs(9).WillReturnRows(userRow(9, testUserID, false, true))
	// profile does not exist yet -> created
	d.mock.ExpectQuery(sqlSelectProfileByUserID).WithArgs(9).WillReturnRows(sqlmock.NewRows(profileColumns))
	d.mock.ExpectQuery(sqlSelectUserByID).WithArgs(9).WillReturnRows(userRow(9, testUserID, false, true))
	d.mock.ExpectQuery(sqlInsertProfile).WithArgs(9, "").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(70))
	expectExistingProfile(d.mock, 70, 9)
	// membership flag
	d.mock.ExpectExec(sqlSetClubMember).WithArgs(true, 9).WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, s.HandleJoinLeftMember(d.bot, chatMemberCtx(d.bot, memberWithStatus("member"))))
}

func TestJoinLeftService_HandleJoinLeftMember_LeftOrKickedClubMemberIsUnset(t *testing.T) {
	// A restricted user who leaves the chat keeps the "restricted" status with is_member = false
	for _, status := range []string{"left", "kicked", "restricted-left"} {
		t.Run(status, func(t *testing.T) {
			d := newTestDeps(t)
			s := NewJoinLeftService(d.userRepo)

			expectExistingUserWithMembership(d.mock, 7, true)
			d.mock.ExpectExec(sqlSetClubMember).WithArgs(false, 7).WillReturnResult(sqlmock.NewResult(0, 1))

			require.NoError(t, s.HandleJoinLeftMember(d.bot, chatMemberCtx(d.bot, memberWithStatus(status))))
		})
	}
}

func TestJoinLeftService_HandleJoinLeftMember_LeftNonClubMemberIsNotUpdated(t *testing.T) {
	d := newTestDeps(t)
	s := NewJoinLeftService(d.userRepo)

	expectExistingUserWithMembership(d.mock, 7, false)

	require.NoError(t, s.HandleJoinLeftMember(d.bot, chatMemberCtx(d.bot, memberWithStatus("left"))))
}

func TestJoinLeftService_HandleJoinLeftMember_Errors(t *testing.T) {
	tests := []struct {
		name        string
		status      string
		setup       func(mock sqlmock.Sqlmock)
		errContains string
	}{
		{
			name:   "member: user lookup fails",
			status: "member",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(sqlSelectUserByTgID).WithArgs(testUserID).WillReturnError(errDB)
			},
			errContains: "failed to get or create user",
		},
		{
			name:   "member: status update fails",
			status: "member",
			setup: func(mock sqlmock.Sqlmock) {
				expectExistingUserWithMembership(mock, 7, false)
				mock.ExpectExec(sqlSetClubMember).WithArgs(true, 7).WillReturnError(errDB)
			},
			errContains: "failed to set club member status to true",
		},
		{
			name:   "member: status update finds no user",
			status: "member",
			setup: func(mock sqlmock.Sqlmock) {
				expectExistingUserWithMembership(mock, 7, false)
				mock.ExpectExec(sqlSetClubMember).WithArgs(true, 7).WillReturnResult(sqlmock.NewResult(0, 0))
			},
			errContains: "failed to set club member status to true",
		},
		{
			name:   "left: user lookup fails",
			status: "left",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(sqlSelectUserByTgID).WithArgs(testUserID).WillReturnError(errDB)
			},
			errContains: "failed to get or create user",
		},
		{
			name:   "kicked: status update fails",
			status: "kicked",
			setup: func(mock sqlmock.Sqlmock) {
				expectExistingUserWithMembership(mock, 7, true)
				mock.ExpectExec(sqlSetClubMember).WithArgs(false, 7).WillReturnError(errDB)
			},
			errContains: "failed to set club member status to false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newTestDeps(t)
			s := NewJoinLeftService(d.userRepo)
			tt.setup(d.mock)

			err := s.HandleJoinLeftMember(d.bot, chatMemberCtx(d.bot, memberWithStatus(tt.status)))

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}
