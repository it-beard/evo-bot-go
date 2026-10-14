package testhandlers

import (
	"testing"

	"evo-bot-go/internal/testutils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newLinkToLearnEnv(t *testing.T) *testEnv {
	env := newTestEnv(t)
	env.handler = NewTryLinkToLearnHandler(env.cfg, env.sender, env.permissions)
	return env
}

func TestTryLinkToLearnHandler_AdminGetsKnowledgeBaseButton(t *testing.T) {
	env := newLinkToLearnEnv(t)

	require.True(t, env.sendText(testAdminID, "/tryLinkToLearn"))

	sent := env.lastSent()
	assert.Equal(t, []string{"База знаний Эволюции Кода ➡️"}, env.texts())
	assert.Equal(t, testAdminID, sent.Msg.Chat.Id)
	buttons := inlineButtons(t, sent.Call)
	require.Len(t, buttons, 1)
	assert.Equal(t, "База знаний", buttons[0].Text)
	assert.Equal(t, "https://telegram.me/the_evocoders_learn_bot/base?startapp=app", buttons[0].Url)
}

func TestTryLinkToLearnHandler_IgnoresOtherCommands(t *testing.T) {
	env := newLinkToLearnEnv(t)

	assert.False(t, env.sendText(testAdminID, "/tryLinkToLearnMore"))
	assert.False(t, env.sendText(testAdminID, "tryLinkToLearn"))
	assert.Empty(t, env.client.Calls())
}

func TestTryLinkToLearnHandler_NonAdminIsRejected(t *testing.T) {
	env := newLinkToLearnEnv(t)

	require.True(t, env.sendText(testUserID, "/tryLinkToLearn"))

	assert.Equal(t, []string{adminOnlyText}, env.texts())
	assert.Equal(t, testUserID, env.lastSent().Msg.Chat.Id)
	assert.Len(t, env.client.CallsTo("getChatMember"), 1)
}

func TestTryLinkToLearnHandler_GroupChatIsRejected(t *testing.T) {
	for _, sender := range groupChatSenders {
		t.Run(sender.name, func(t *testing.T) {
			env := newLinkToLearnEnv(t)

			require.True(t, env.sendGroupText(sender.id, "/tryLinkToLearn"))

			env.requirePrivateOnlyNotice(sender.id)
			assert.NotContains(t, env.texts(), "База знаний Эволюции Кода ➡️")
		})
	}
}

func TestTryLinkToLearnHandler_SendFailureIsReturned(t *testing.T) {
	env := newLinkToLearnEnv(t)
	env.failSendWhen(func(testutils.APICall) bool { return true })

	handled, err := env.tryDispatch(testutils.MessageUpdate(testutils.NewPrivateMessage(testAdminID, "/tryLinkToLearn")))

	assert.True(t, handled)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mocked failure")
}
