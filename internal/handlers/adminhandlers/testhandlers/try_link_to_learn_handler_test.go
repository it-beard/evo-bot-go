package testhandlers

import (
	"testing"

	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/testutils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	linkToLearnChooseLanguageText = "На каком языке отправить сообщение со ссылкой на базу знаний?"
	linkToLearnUseButtonsText     = "Пожалуйста, нажми на одну из кнопок выше или /cancel для отмены."
	linkToLearnCanceledText       = "Отправка ссылки на базу знаний отменена."
	linkToLearnURL                = "https://t.me/the_evocoders_learn_bot/base?startapp=app"
)

// knowledgeBaseLanguages lists the language buttons with the message each of them sends
var knowledgeBaseLanguages = []struct {
	name       string
	callback   string
	text       string
	buttonText string
}{
	{"russian", constants.TryLinkToLearnRussianCallback, "База знаний Эволюции Кода ➡️", "База знаний"},
	{"belarusian", constants.TryLinkToLearnBelarusianCallback, "База ведаў Эвалюцыі Кода ➡️", "База ведаў"},
}

func newLinkToLearnEnv(t *testing.T) *testEnv {
	env := newTestEnv(t)
	env.handler = NewTryLinkToLearnHandler(env.cfg, env.sender, env.permissions)
	return env
}

// openLanguageSelection runs the command and returns the language selection message
func openLanguageSelection(t *testing.T, env *testEnv) gotgbot.Message {
	t.Helper()
	require.True(t, env.sendText(testAdminID, "/tryLinkToLearn"))
	return env.lastSent().Msg
}

func TestTryLinkToLearnHandler_AdminIsAskedForLanguage(t *testing.T) {
	env := newLinkToLearnEnv(t)

	prompt := openLanguageSelection(t, env)
	call := env.lastSent().Call

	assert.Equal(t, testAdminID, prompt.Chat.Id)
	assert.Equal(t, []string{linkToLearnChooseLanguageText}, env.texts())
	assert.Equal(t, []string{
		constants.TryLinkToLearnRussianCallback,
		constants.TryLinkToLearnBelarusianCallback,
		constants.TryLinkToLearnCancelCallback,
	}, callbackData(t, call))
	buttons := inlineButtons(t, call)
	assert.Equal(t, "Русский", buttons[0].Text)
	assert.Equal(t, "Беларуская", buttons[1].Text)
	assert.Empty(t, env.deletedMessages())
}

func TestTryLinkToLearnHandler_LanguageChoiceSendsKnowledgeBaseLink(t *testing.T) {
	for _, lang := range knowledgeBaseLanguages {
		t.Run(lang.name, func(t *testing.T) {
			env := newLinkToLearnEnv(t)
			prompt := openLanguageSelection(t, env)
			env.reset()

			update := testutils.CallbackUpdate(testAdminID, lang.callback)
			require.True(t, env.dispatch(update))

			assert.Equal(t, []string{update.CallbackQuery.Id}, env.answeredCallbacks(), "the button press is answered")
			assert.Equal(t, [][2]int64{msgRef(prompt)}, env.deletedMessages(), "the language selection is removed")

			sent := env.lastSent()
			assert.Equal(t, []string{lang.text}, env.texts())
			assert.Equal(t, testAdminID, sent.Msg.Chat.Id)
			buttons := inlineButtons(t, sent.Call)
			require.Len(t, buttons, 1)
			assert.Equal(t, lang.buttonText, buttons[0].Text)
			assert.Equal(t, linkToLearnURL, buttons[0].Url)
			assert.Empty(t, buttons[0].CallbackData)

			// The conversation is over
			assert.False(t, env.click(testAdminID, lang.callback))
			assert.False(t, env.sendText(testAdminID, "hello"))
		})
	}
}

func TestTryLinkToLearnHandler_TextDuringSelectionAsksForButtons(t *testing.T) {
	env := newLinkToLearnEnv(t)
	openLanguageSelection(t, env)
	env.reset()

	require.True(t, env.sendText(testAdminID, "русский"))

	assert.Equal(t, []string{linkToLearnUseButtonsText}, env.texts())
	assert.Empty(t, env.deletedMessages())

	// The language can still be chosen
	env.reset()
	require.True(t, env.click(testAdminID, constants.TryLinkToLearnRussianCallback))
	assert.Equal(t, []string{knowledgeBaseLanguages[0].text}, env.texts())
}

func TestTryLinkToLearnHandler_Cancel(t *testing.T) {
	tests := []struct {
		name       string
		cancel     func(env *testEnv) bool
		isCallback bool
	}{
		{"cancel button", func(env *testEnv) bool { return env.click(testAdminID, constants.TryLinkToLearnCancelCallback) }, true},
		{"cancel command", func(env *testEnv) bool { return env.sendText(testAdminID, "/cancel") }, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newLinkToLearnEnv(t)
			prompt := openLanguageSelection(t, env)
			env.reset()

			require.True(t, tc.cancel(env))

			if tc.isCallback {
				assert.Len(t, env.answeredCallbacks(), 1)
			} else {
				assert.Empty(t, env.answeredCallbacks())
			}
			assert.Equal(t, [][2]int64{msgRef(prompt)}, env.deletedMessages())
			assert.Equal(t, []string{linkToLearnCanceledText}, env.texts())
			assert.Equal(t, testAdminID, env.lastSent().Msg.Chat.Id)
			assert.False(t, env.click(testAdminID, constants.TryLinkToLearnRussianCallback), "conversation must be over")
		})
	}
}

func TestTryLinkToLearnHandler_CancelSendFailureStillEndsConversation(t *testing.T) {
	env := newLinkToLearnEnv(t)
	prompt := openLanguageSelection(t, env)
	env.reset()
	env.failSendWhen(func(call testutils.APICall) bool { return call.Params["text"] == linkToLearnCanceledText })

	require.True(t, env.click(testAdminID, constants.TryLinkToLearnCancelCallback))

	assert.Len(t, env.answeredCallbacks(), 1)
	assert.Equal(t, [][2]int64{msgRef(prompt)}, env.deletedMessages())
	assert.Empty(t, env.texts())
	assert.False(t, env.click(testAdminID, constants.TryLinkToLearnRussianCallback), "conversation must be over")
}

func TestTryLinkToLearnHandler_LanguageSelectionSendFailureIsReturned(t *testing.T) {
	env := newLinkToLearnEnv(t)
	env.failSendWhen(func(call testutils.APICall) bool { return call.Params["text"] == linkToLearnChooseLanguageText })

	handled, err := env.tryDispatch(testutils.MessageUpdate(testutils.NewPrivateMessage(testAdminID, "/tryLinkToLearn")))

	assert.True(t, handled)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send language selection")
	assert.Contains(t, err.Error(), "mocked failure")
	assert.False(t, env.click(testAdminID, constants.TryLinkToLearnRussianCallback), "conversation must not be started")
}

func TestTryLinkToLearnHandler_LinkSendFailureKeepsLanguageSelection(t *testing.T) {
	env := newLinkToLearnEnv(t)
	prompt := openLanguageSelection(t, env)
	env.reset()
	env.failSendWhen(func(call testutils.APICall) bool { return call.Params["text"] == knowledgeBaseLanguages[1].text })

	update := testutils.CallbackUpdate(testAdminID, constants.TryLinkToLearnBelarusianCallback)
	handled, err := env.tryDispatch(update)

	assert.True(t, handled)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send knowledge base link")
	assert.Contains(t, err.Error(), "mocked failure")
	assert.Equal(t, []string{update.CallbackQuery.Id}, env.answeredCallbacks(), "the button press is still answered")
	assert.Empty(t, env.deletedMessages(), "the language buttons stay for another try")

	// Another press succeeds once Telegram answers again
	env.reset()
	env.failSendWhen(nil)
	require.True(t, env.click(testAdminID, constants.TryLinkToLearnBelarusianCallback))
	assert.Equal(t, []string{knowledgeBaseLanguages[1].text}, env.texts())
	assert.Equal(t, [][2]int64{msgRef(prompt)}, env.deletedMessages())
}

func TestTryLinkToLearnHandler_IgnoresOtherCommands(t *testing.T) {
	env := newLinkToLearnEnv(t)

	assert.False(t, env.sendText(testAdminID, "/tryLinkToLearnMore"))
	assert.False(t, env.sendText(testAdminID, "tryLinkToLearn"))
	assert.False(t, env.click(testAdminID, constants.TryLinkToLearnRussianCallback), "buttons work only after the command")
	assert.Empty(t, env.client.Calls())
}

func TestTryLinkToLearnHandler_NonAdminIsRejected(t *testing.T) {
	env := newLinkToLearnEnv(t)

	require.True(t, env.sendText(testUserID, "/tryLinkToLearn"))

	assert.Equal(t, []string{adminOnlyText}, env.texts())
	assert.Equal(t, testUserID, env.lastSent().Msg.Chat.Id)
	assert.Len(t, env.client.CallsTo("getChatMember"), 1)
	assert.False(t, env.click(testUserID, constants.TryLinkToLearnRussianCallback), "conversation must not be started")
}

func TestTryLinkToLearnHandler_GroupChatIsRejected(t *testing.T) {
	for _, sender := range groupChatSenders {
		t.Run(sender.name, func(t *testing.T) {
			env := newLinkToLearnEnv(t)

			require.True(t, env.sendGroupText(sender.id, "/tryLinkToLearn"))

			env.requirePrivateOnlyNotice(sender.id)
			assert.NotContains(t, env.texts(), linkToLearnChooseLanguageText)
			assert.False(t, env.sendGroupText(sender.id, "hello"), "conversation must not be started")
		})
	}
}
