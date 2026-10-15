package privatehandlers

import (
	"testing"

	"evo-bot-go/internal/formatters"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHelpHandler_NonPrivateChatIsRejected(t *testing.T) {
	d := newDeps(t)
	h := d.newHelpHandler()

	require.True(t, d.dispatch(t, h, groupText(testUserID, "/help")))

	assertPrivateOnlyRejection(t, d.client)
	for _, text := range d.client.SentTexts() {
		assert.NotContains(t, text, "Функционал бота")
	}
}

func TestHelpHandler_NonClubMemberIsRejected(t *testing.T) {
	for _, status := range []string{"left", "kicked"} {
		t.Run(status, func(t *testing.T) {
			d := newDeps(t)
			d.client.SetMemberStatus(testUserID, status)

			require.True(t, d.dispatch(t, d.newHelpHandler(), privateText(testUserID, "/help")))

			assertClubOnlyRejection(t, d.client)
		})
	}
}

func TestHelpHandler_ShowsHelpDependingOnRole(t *testing.T) {
	tests := []struct {
		status    string
		wantAdmin bool
	}{
		{status: "member", wantAdmin: false},
		{status: "restricted", wantAdmin: false},
		{status: "administrator", wantAdmin: true},
		{status: "creator", wantAdmin: true},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			d := newDeps(t)
			d.client.SetMemberStatus(testUserID, tt.status)
			msgUpdate := privateText(testUserID, "/help")

			require.True(t, d.dispatch(t, d.newHelpHandler(), msgUpdate))

			sends := d.client.CallsTo("sendMessage")
			require.Len(t, sends, 1)
			help := sends[0]
			assert.Equal(t, testUserID, help.Int64("chat_id"))
			assert.Equal(t, "HTML", help.Params["parse_mode"])
			assert.Contains(t, help.Params["reply_parameters"], `"message_id":`)
			assert.Equal(t, formatters.FormatHelpMessage(tt.wantAdmin, d.cfg), help.Params["text"])
			assert.NotContains(t, help.Params["text"], "└ /code ", "the removed /code command is not listed")
			if tt.wantAdmin {
				assert.Contains(t, help.Params["text"], "Команды администратора")
			} else {
				assert.NotContains(t, help.Params["text"], "Команды администратора")
			}
			// Membership and admin rights are both checked against the club supergroup
			for _, c := range d.client.CallsTo("getChatMember") {
				assert.Equal(t, testFullSuperGroupID, c.Int64("chat_id"))
			}
		})
	}
}

func TestHelpHandler_IgnoresOtherCommands(t *testing.T) {
	d := newDeps(t)

	assert.False(t, d.dispatch(t, d.newHelpHandler(), privateText(testUserID, "/start")))
	assert.False(t, d.dispatch(t, d.newHelpHandler(), privateText(testUserID, "help")))
	assert.Empty(t, d.client.Calls())
}
