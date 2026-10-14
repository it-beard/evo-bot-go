package handlers

import (
	"fmt"
	"strings"
	"testing"

	"evo-bot-go/internal/formatters"
	"evo-bot-go/internal/testutils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartHandler_NonPrivateChatIsRejected(t *testing.T) {
	f := newStartFixture(t)
	msg := testutils.NewGroupMessage(testUserID, testGroupChatIDForTest, 0, "/start")

	handled := f.dispatch(t, testutils.MessageUpdate(msg))

	require.True(t, handled)
	sends := f.client.CallsTo("sendMessage")
	require.Len(t, sends, 2, "private-only warning in the group + ping in the private chat")

	warning := sends[0]
	assert.Equal(t, testGroupChatIDForTest, warning.Int64("chat_id"))
	assert.Contains(t, warning.Params["text"], "работает только в <i>личной беседе</i>")
	assert.Contains(t, warning.Params["text"], "Напиши мне в ЛС")
	assert.Equal(t, "HTML", warning.Params["parse_mode"])
	assert.Contains(t, warning.Params["reply_parameters"], `"message_id":`)

	assert.Equal(t, testUserID, sends[1].Int64("chat_id"), "the user is pinged in the private chat")
	assert.Empty(t, f.client.CallsTo("getChatMember"), "membership must not be checked outside private chats")

	// The conversation was not started, so the help button is not handled
	assert.False(t, f.dispatch(t, testutils.CallbackUpdate(testUserID, startHandlerCallbackHelp)))
}

func TestStartHandler_ClubMemberGetsGreetingAndHelp(t *testing.T) {
	f := newStartFixture(t)

	require.True(t, f.dispatch(t, privateCommand(testUserID, "/start")))

	memberCall, ok := f.client.LastCallTo("getChatMember")
	require.True(t, ok)
	assert.Equal(t, testFullSuperGroupID, memberCall.Int64("chat_id"))
	assert.Equal(t, testUserID, memberCall.Int64("user_id"))

	greeting, ok := f.client.LastCallTo("sendMessage")
	require.True(t, ok)
	assert.Equal(t, testUserID, greeting.Int64("chat_id"))
	assert.Equal(t, "HTML", greeting.Params["parse_mode"])
	assert.Equal(t, "Приветствую, <b>First</b>! 🎩\n\n"+
		"Я — <b>Дженкинс Вебстер</b>, потомственный дворецкий и верный помощник клуба <i>«Эволюция Кода»</i> 🧐\n\n"+
		"Рад видеть тебя среди участников нашего клуба! Я готов помочь тебе во всех твоих начинаниях. 🤵",
		greeting.Params["text"])
	assert.Contains(t, greeting.Params["reply_parameters"], `"message_id":`)

	markup := replyKeyboard(t, greeting)
	require.Len(t, markup.InlineKeyboard, 1)
	require.Len(t, markup.InlineKeyboard[0], 1)
	assert.Equal(t, "💡 Как пользоваться ботом?", markup.InlineKeyboard[0][0].Text)
	assert.Equal(t, startHandlerCallbackHelp, markup.InlineKeyboard[0][0].CallbackData)

	f.client.Reset()
	require.True(t, f.dispatch(t, testutils.CallbackUpdate(testUserID, startHandlerCallbackHelp)))

	assert.Len(t, f.client.CallsTo("answerCallbackQuery"), 1)
	help, ok := f.client.LastCallTo("sendMessage")
	require.True(t, ok)
	assert.Equal(t, testUserID, help.Int64("chat_id"))
	assert.Equal(t, "HTML", help.Params["parse_mode"])
	assert.Equal(t, formatters.FormatHelpMessage(false, f.cfg), help.Params["text"])
	assert.NotContains(t, help.Params["text"], "Команды администратора")

	// The conversation is over after the help was shown
	assert.False(t, f.dispatch(t, testutils.CallbackUpdate(testUserID, startHandlerCallbackHelp)))
}

func TestStartHandler_HelpForAdmins(t *testing.T) {
	for _, status := range []string{"administrator", "creator"} {
		t.Run(status, func(t *testing.T) {
			f := newStartFixture(t)
			f.client.SetMemberStatus(testUserID, status)

			require.True(t, f.dispatch(t, privateCommand(testUserID, "/start")))
			assert.Contains(t, f.client.SentTexts()[0], "Рад видеть тебя среди участников")

			f.client.Reset()
			require.True(t, f.dispatch(t, testutils.CallbackUpdate(testUserID, startHandlerCallbackHelp)))

			help, ok := f.client.LastCallTo("sendMessage")
			require.True(t, ok)
			assert.Equal(t, formatters.FormatHelpMessage(true, f.cfg), help.Params["text"])
			assert.Contains(t, help.Params["text"], "Команды администратора")
		})
	}
}

func TestStartHandler_NonMemberGetsInvitation(t *testing.T) {
	tests := []struct {
		name  string
		setup func(f *startFixture)
	}{
		{name: "left", setup: func(f *startFixture) { f.client.SetMemberStatus(testUserID, "left") }},
		{name: "kicked", setup: func(f *startFixture) { f.client.SetMemberStatus(testUserID, "kicked") }},
		{name: "getChatMember fails", setup: func(f *startFixture) {
			f.client.OnError("getChatMember", "Bad Request: user not found")
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newStartFixture(t)
			tt.setup(f)

			require.True(t, f.dispatch(t, privateCommand(testUserID, "/start")))

			sends := f.client.CallsTo("sendMessage")
			require.Len(t, sends, 1)
			text := sends[0].Params["text"]
			assert.Equal(t, "Приветствую, <b>First</b>! 🎩\n\n"+
				"Я — <b>Дженкинс Вебстер</b>, потомственный дворецкий и верный помощник клуба <i>«Эволюция Кода»</i> 🧐\n\n"+
				"Позволь предложить тебе присоединиться к нашему изысканному сообществу разработчиков и разработчиц, "+
				"где я буду рад служить тебе всеми своими возможностями и ресурсами.",
				text)
			assert.NotContains(t, text, "Позвольте", "the user is addressed with \"ты\"")
			assert.NotContains(t, text, "Рад видеть тебя среди участников")
			assert.Equal(t, "HTML", sends[0].Params["parse_mode"])

			markup := replyKeyboard(t, sends[0])
			require.Len(t, markup.InlineKeyboard, 1)
			require.Len(t, markup.InlineKeyboard[0], 1)
			button := markup.InlineKeyboard[0][0]
			assert.Equal(t, "💡 Жду тебя в клубе!", button.Text)
			assert.Equal(t, "https://web.tribute.tg/l/ge", button.Url)
			assert.Empty(t, button.CallbackData)
		})
	}
}

func TestStartHandler_GreetingWithoutFirstName(t *testing.T) {
	f := newStartFixture(t)
	msg := testutils.NewPrivateMessage(testUserID, "/start")
	msg.From.FirstName = ""

	require.True(t, f.dispatch(t, testutils.MessageUpdate(msg)))

	texts := f.client.SentTexts()
	require.Len(t, texts, 1)
	assert.True(t, strings.HasPrefix(texts[0], "Приветствую! 🎩\n\n"), "no empty bold name: %q", texts[0])
	assert.NotContains(t, texts[0], "<b></b>")
}

func TestStartHandler_GreetingEscapesFirstName(t *testing.T) {
	tests := []struct {
		firstName string
		greeting  string
	}{
		{firstName: "Anna <3", greeting: "Приветствую, <b>Anna &lt;3</b>! 🎩"},
		{firstName: "Alex_*", greeting: "Приветствую, <b>Alex_*</b>! 🎩"},
		{firstName: "<b>Bob</b> & Co", greeting: "Приветствую, <b>&lt;b&gt;Bob&lt;/b&gt; &amp; Co</b>! 🎩"},
		{firstName: `O'Neil "Jr"`, greeting: "Приветствую, <b>O&#39;Neil &#34;Jr&#34;</b>! 🎩"},
	}

	for _, tt := range tests {
		for _, member := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/member=%v", tt.firstName, member), func(t *testing.T) {
				f := newStartFixture(t)
				if !member {
					f.client.SetMemberStatus(testUserID, "left")
				}
				msg := testutils.NewPrivateMessage(testUserID, "/start")
				msg.From.FirstName = tt.firstName

				require.True(t, f.dispatch(t, testutils.MessageUpdate(msg)))

				sends := f.client.CallsTo("sendMessage")
				require.Len(t, sends, 1)
				assert.Equal(t, "HTML", sends[0].Params["parse_mode"])
				assert.True(t, strings.HasPrefix(sends[0].Params["text"], tt.greeting+"\n\n"),
					"greeting must start with %q, got %q", tt.greeting, sends[0].Params["text"])
			})
		}
	}
}

func TestStartHandler_IgnoresUnrelatedUpdates(t *testing.T) {
	f := newStartFixture(t)

	assert.False(t, f.dispatch(t, privateCommand(testUserID, "/help")))
	assert.False(t, f.dispatch(t, privateCommand(testUserID, "hello")))
	assert.False(t, f.dispatch(t, testutils.CallbackUpdate(testUserID, startHandlerCallbackHelp)),
		"the help button only works inside a started conversation")
	assert.Empty(t, f.client.Calls())
}
