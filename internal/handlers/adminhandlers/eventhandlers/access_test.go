package eventhandlers

import (
	"testing"

	"evo-bot-go/internal/constants"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var allEventHandlers = []struct {
	name    string
	command string
	factory handlerFactory
}{
	{"delete", constants.EventDeleteCommand, NewEventDeleteHandler},
	{"edit", constants.EventEditCommand, NewEventEditHandler},
	{"setup", constants.EventSetupCommand, NewEventSetupHandler},
	{"start", constants.EventStartCommand, NewEventStartHandler},
}

func TestEventHandlers_NonAdminIsRejected(t *testing.T) {
	for _, tc := range allEventHandlers {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t, tc.factory)

			require.True(t, env.sendText(testUserID, "/"+tc.command))

			// Only the rejection is sent, no DB access happens (sqlmock would fail on unexpected queries)
			assert.Equal(t, []string{adminOnlyText}, env.texts())
			assert.Equal(t, testUserID, env.lastSent().Msg.Chat.Id)

			// Admin status is checked against the club supergroup
			memberCall, ok := env.client.LastCallTo("getChatMember")
			require.True(t, ok)
			assert.Equal(t, fullChatID(), memberCall.Int64("chat_id"))
			assert.Equal(t, testUserID, memberCall.Int64("user_id"))

			// The conversation was not started
			assert.False(t, env.sendText(testUserID, "1"))
		})
	}
}

func TestEventHandlers_GroupChatIsRejected(t *testing.T) {
	for _, tc := range allEventHandlers {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t, tc.factory)

			require.True(t, env.sendGroupText(testAdminID, "/"+tc.command))

			sent := env.sentMessages()
			require.Len(t, sent, 2)
			// A markdown notice is posted in the group as a reply ...
			assert.Equal(t, fullChatID(), sent[0].Msg.Chat.Id)
			assert.Equal(t, "Markdown", sent[0].Call.Params["parse_mode"])
			assert.Contains(t, sent[0].Msg.Text, "Эта команда работает только в _личной беседе_ со мной")
			assert.Contains(t, sent[0].Call.Params["reply_parameters"], "message_id")
			// ... and the admin is pinged in private messages
			assert.Equal(t, testAdminID, sent[1].Msg.Chat.Id)

			// The conversation was not started
			assert.False(t, env.sendGroupText(testAdminID, "1"))
		})
	}
}
