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
	const privateOnlyNotice = "<b>Прошу прощения</b> 🧐\n\n" +
		"Эта команда работает только в <i>личной беседе</i> со мной. " +
		"Напиши мне в ЛС, и я с удовольствием помогу (я тебя там пинганул, если мы общались ранее)." +
		"\n\nЭто сообщение и твоя команда будут автоматически удалены через 10 секунд."

	users := []struct {
		name   string
		userID int64
	}{
		{"admin", testAdminID},
		{"non-admin", testUserID},
	}

	for _, tc := range allEventHandlers {
		for _, user := range users {
			t.Run(tc.name+"/"+user.name, func(t *testing.T) {
				env := newTestEnv(t, tc.factory)

				require.True(t, env.sendGroupText(user.userID, "/"+tc.command))

				sent := env.sentMessages()
				require.Len(t, sent, 2)
				// An HTML notice is posted in the group as a reply ...
				assert.Equal(t, fullChatID(), sent[0].Msg.Chat.Id)
				assert.Equal(t, "HTML", sent[0].Call.Params["parse_mode"])
				assert.Equal(t, privateOnlyNotice, sent[0].Msg.Text)
				assert.Contains(t, sent[0].Call.Params["reply_parameters"], "message_id")
				// ... and the user is pinged in private messages
				assert.Equal(t, user.userID, sent[1].Msg.Chat.Id)

				// The chat type is checked first, so admin rights are not even looked up
				assert.Empty(t, env.client.CallsTo("getChatMember"))
				assert.NotContains(t, env.texts(), adminOnlyText)

				// The conversation was not started
				assert.False(t, env.sendGroupText(user.userID, "1"))
			})
		}
	}
}
