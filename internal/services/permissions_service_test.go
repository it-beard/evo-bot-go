package services

import (
	"testing"

	"evo-bot-go/internal/testutils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	adminOnlyText   = "Эта команда доступна только администраторам."
	clubOnlyText    = "Эта команда доступна только участникам клуба."
	privateOnlyText = "<b>Прошу прощения</b> 🧐\n\nЭта команда работает только в <i>личной беседе</i> со мной. " +
		"Напиши мне в ЛС, и я с удовольствием помогу (я тебя там пинганул, если мы общались ранее)." +
		"\n\nЭто сообщение и твоя команда будут автоматически удалены через 10 секунд."
)

func newPermissionsFixture() (*PermissionsService, *testutils.MockBotClient) {
	bot, client := testutils.NewTestBot()
	return NewPermissionsService(newTestConfig(), bot, NewMessageSenderService(bot)), client
}

func assertRepliedTo(t *testing.T, client *testutils.MockBotClient, msg *gotgbot.Message, text string) {
	t.Helper()
	call, ok := client.LastCallTo("sendMessage")
	require.True(t, ok, "a reply must be sent")
	assert.Equal(t, msg.Chat.Id, call.Int64("chat_id"))
	assert.Equal(t, text, call.Params["text"])
	var rp gotgbot.ReplyParameters
	require.NoError(t, call.DecodeJSON("reply_parameters", &rp))
	assert.Equal(t, msg.MessageId, rp.MessageId)
}

func TestPermissionsService_CheckAdminPermissions(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{status: "administrator", want: true},
		{status: "creator", want: true},
		{status: "member", want: false},
		{status: "restricted", want: false},
		{status: "left", want: false},
		{status: "kicked", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			s, client := newPermissionsFixture()
			client.SetMemberStatus(10, tt.status)
			msg := testutils.NewPrivateMessage(10, "/cmd")

			got := s.CheckAdminPermissions(msg, "cmd")

			assert.Equal(t, tt.want, got)
			member, ok := client.LastCallTo("getChatMember")
			require.True(t, ok)
			assert.Equal(t, testFullChatID, member.Int64("chat_id"), "membership is checked in the club supergroup")
			assert.Equal(t, int64(10), member.Int64("user_id"))
			if tt.want {
				assert.Empty(t, client.CallsTo("sendMessage"))
			} else {
				assertRepliedTo(t, client, msg, adminOnlyText)
			}
		})
	}
}

func TestPermissionsService_CheckAdminPermissions_GetChatMemberError(t *testing.T) {
	s, client := newPermissionsFixture()
	client.OnError("getChatMember", "Bad Request: user not found")
	msg := testutils.NewPrivateMessage(10, "/cmd")

	assert.False(t, s.CheckAdminPermissions(msg, "cmd"))
	assertRepliedTo(t, client, msg, adminOnlyText)
}

func TestPermissionsService_CheckAdminPermissions_ReplyErrorStillDenies(t *testing.T) {
	s, client := newPermissionsFixture()
	client.OnError("sendMessage", "Forbidden: bot was blocked by the user")

	assert.False(t, s.CheckAdminPermissions(testutils.NewPrivateMessage(10, "/cmd"), "cmd"))
}

func TestPermissionsService_CheckClubMemberPermissions(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{status: "member", want: true},
		{status: "administrator", want: true},
		{status: "creator", want: true},
		{status: "restricted", want: true},
		{status: "left", want: false},
		{status: "kicked", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			s, client := newPermissionsFixture()
			client.SetMemberStatus(20, tt.status)
			msg := testutils.NewPrivateMessage(20, "/cmd")

			got := s.CheckClubMemberPermissions(msg, "cmd")

			assert.Equal(t, tt.want, got)
			member, ok := client.LastCallTo("getChatMember")
			require.True(t, ok)
			assert.Equal(t, testFullChatID, member.Int64("chat_id"))
			assert.Equal(t, int64(20), member.Int64("user_id"))
			if tt.want {
				assert.Empty(t, client.CallsTo("sendMessage"))
			} else {
				assertRepliedTo(t, client, msg, clubOnlyText)
			}
		})
	}

	t.Run("getChatMember error denies", func(t *testing.T) {
		s, client := newPermissionsFixture()
		client.OnError("getChatMember", "Bad Request: user not found")
		msg := testutils.NewPrivateMessage(20, "/cmd")

		assert.False(t, s.CheckClubMemberPermissions(msg, "cmd"))
		assertRepliedTo(t, client, msg, clubOnlyText)
	})

	t.Run("reply error still denies", func(t *testing.T) {
		s, client := newPermissionsFixture()
		client.SetMemberStatus(20, "left")
		client.OnError("sendMessage", "Forbidden: bot was blocked by the user")

		assert.False(t, s.CheckClubMemberPermissions(testutils.NewPrivateMessage(20, "/cmd"), "cmd"))
	})
}

func TestPermissionsService_CheckPrivateChatType_Private(t *testing.T) {
	s, client := newPermissionsFixture()

	assert.True(t, s.CheckPrivateChatType(testutils.NewPrivateMessage(30, "/cmd")))
	assert.Empty(t, client.Calls())
}

func TestPermissionsService_CheckPrivateChatType_Group(t *testing.T) {
	s, client := newPermissionsFixture()
	msg := testutils.NewGroupMessage(30, testFullChatID, 5, "/cmd")

	assert.False(t, s.CheckPrivateChatType(msg))

	sends := client.CallsTo("sendMessage")
	require.Len(t, sends, 2, "a reply in the group and a ping in private")
	reply := sends[0]
	assert.Equal(t, testFullChatID, reply.Int64("chat_id"))
	assert.Equal(t, "HTML", reply.Params["parse_mode"])
	assert.Equal(t, privateOnlyText, reply.Params["text"])
	var rp gotgbot.ReplyParameters
	require.NoError(t, reply.DecodeJSON("reply_parameters", &rp))
	assert.Equal(t, msg.MessageId, rp.MessageId)
	assert.Equal(t, int64(30), sends[1].Int64("chat_id"), "the user is pinged in the private chat")
	// Both messages are deleted after a hard-coded 10 second delay; that is covered by the
	// MessageSenderService.ReplyWithCleanupAfterDelayWithPing tests with a zero delay.
	assert.Empty(t, client.CallsTo("deleteMessage"))
}

func TestPermissionsService_CheckPrivateChatType_ReplyErrorStillDenies(t *testing.T) {
	s, client := newPermissionsFixture()
	client.OnError("sendMessage", "Bad Request: chat not found")

	assert.False(t, s.CheckPrivateChatType(testutils.NewGroupMessage(30, testFullChatID, 0, "/cmd")))
	assert.Len(t, client.CallsTo("sendMessage"), 1)
}

func TestPermissionsService_CheckAdminAndPrivateChat_GroupChatGetsPrivateOnlyNotice(t *testing.T) {
	// The chat type is checked first: whatever the user's role, a command sent to a group only gets the
	// auto-deleted "private chat only" notice, and the membership is not even looked up.
	for _, status := range []string{"member", "administrator", "creator", "left"} {
		t.Run(status, func(t *testing.T) {
			s, client := newPermissionsFixture()
			client.SetMemberStatus(40, status)
			msg := testutils.NewGroupMessage(40, testFullChatID, 0, "/cmd")

			assert.False(t, s.CheckAdminAndPrivateChat(msg, "cmd"))

			sends := client.CallsTo("sendMessage")
			require.Len(t, sends, 2, "the private-only notice in the group and a ping in private")
			assert.Equal(t, testFullChatID, sends[0].Int64("chat_id"))
			assert.Equal(t, privateOnlyText, sends[0].Params["text"])
			assert.Equal(t, "HTML", sends[0].Params["parse_mode"])
			assert.Equal(t, int64(40), sends[1].Int64("chat_id"), "the user is pinged in the private chat")
			assert.NotContains(t, client.SentTexts(), adminOnlyText, "no permanent \"only admins\" reply in the group")
			assert.Empty(t, client.CallsTo("getChatMember"), "admin rights must not be checked outside private chats")
		})
	}
}

func TestPermissionsService_CheckAdminAndPrivateChat_PrivateChat(t *testing.T) {
	t.Run("non admin is rejected", func(t *testing.T) {
		s, client := newPermissionsFixture()
		client.SetMemberStatus(40, "member")
		msg := testutils.NewPrivateMessage(40, "/cmd")

		assert.False(t, s.CheckAdminAndPrivateChat(msg, "cmd"))
		assert.Equal(t, []string{adminOnlyText}, client.SentTexts())
		assertRepliedTo(t, client, msg, adminOnlyText)
		assert.Len(t, client.CallsTo("getChatMember"), 1)
	})

	for _, status := range []string{"administrator", "creator"} {
		t.Run(status+" passes", func(t *testing.T) {
			s, client := newPermissionsFixture()
			client.SetMemberStatus(40, status)

			assert.True(t, s.CheckAdminAndPrivateChat(testutils.NewPrivateMessage(40, "/cmd"), "cmd"))
			assert.Empty(t, client.CallsTo("sendMessage"))
			assert.Len(t, client.CallsTo("getChatMember"), 1)
		})
	}
}
