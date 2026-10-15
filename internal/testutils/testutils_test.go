package testutils

import (
	"context"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockBotClient_SendMessageRecordsCallAndReturnsMessage(t *testing.T) {
	bot, client := NewTestBot()

	msg, err := bot.SendMessage(42, "hello", &gotgbot.SendMessageOpts{MessageThreadId: 7, ParseMode: "HTML"})

	require.NoError(t, err)
	assert.Equal(t, int64(42), msg.Chat.Id)
	assert.Equal(t, "hello", msg.Text)
	assert.Equal(t, int64(7), msg.MessageThreadId)
	assert.NotZero(t, msg.MessageId)

	call, ok := client.LastCallTo("sendMessage")
	require.True(t, ok)
	assert.Equal(t, int64(42), call.Int64("chat_id"))
	assert.Equal(t, "HTML", call.Params["parse_mode"])
	assert.Equal(t, []string{"hello"}, client.SentTexts())
}

func TestMockBotClient_OnErrorReturnsTelegramError(t *testing.T) {
	bot, client := NewTestBot()
	client.OnError("sendMessage", "Bad Request: TOPIC_CLOSED")

	_, err := bot.SendMessage(1, "x", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "TOPIC_CLOSED")
}

func TestMockBotClient_ChatMemberStatus(t *testing.T) {
	bot, client := NewTestBot()
	client.SetMemberStatus(1, "administrator")
	client.SetDefaultMemberStatus("left")

	admin, err := bot.GetChatMember(-100, 1, nil)
	require.NoError(t, err)
	assert.Equal(t, "administrator", admin.GetStatus())

	other, err := bot.GetChatMember(-100, 2, nil)
	require.NoError(t, err)
	assert.Equal(t, "left", other.GetStatus())
}

func TestMockBotClient_DefaultResponses(t *testing.T) {
	bot, client := NewTestBot()

	ok, err := bot.DeleteMessage(1, 2, nil)
	require.NoError(t, err)
	assert.True(t, ok)

	poll, err := bot.SendPoll(-100, "q?", []gotgbot.InputPollOption{{Text: "a"}, {Text: "b"}}, nil)
	require.NoError(t, err)
	require.NotNil(t, poll.Poll)
	assert.NotEmpty(t, poll.Poll.Id)

	copied, err := bot.CopyMessage(1, 2, 3, nil)
	require.NoError(t, err)
	assert.NotZero(t, copied.MessageId)

	edited, _, err := bot.EditMessageText("new", &gotgbot.EditMessageTextOpts{ChatId: 5, MessageId: 9})
	require.NoError(t, err)
	assert.Equal(t, int64(9), edited.MessageId)

	assert.Len(t, client.Calls(), 4)
	client.Reset()
	assert.Empty(t, client.Calls())
}

func TestDispatch_CommandHandler(t *testing.T) {
	bot, _ := NewTestBot()
	var got string
	h := handlers.NewCommand("help", func(b *gotgbot.Bot, ctx *ext.Context) error {
		got = ctx.EffectiveMessage.Text
		return nil
	})

	handled, err := Dispatch(bot, h, MessageUpdate(NewPrivateMessage(10, "/help")))
	require.NoError(t, err)
	assert.True(t, handled)
	assert.Equal(t, "/help", got)

	handled, err = Dispatch(bot, h, MessageUpdate(NewPrivateMessage(10, "plain text")))
	require.NoError(t, err)
	assert.False(t, handled)
}

func TestMockOpenAI_CompletionAndEmbeddings(t *testing.T) {
	client, mock := NewMockOpenAI(t)
	mock.SetCompletion("answer")
	mock.SetEmbeddings([][]float64{{1, 2}, {3, 4}})

	text, err := client.GetCompletion(context.Background(), "prompt text")
	require.NoError(t, err)
	assert.Equal(t, "answer", text)
	assert.Equal(t, "prompt text", mock.LastPrompt())

	vectors, err := client.GetBatchEmbeddings(context.Background(), []string{"a", "b"})
	require.NoError(t, err)
	assert.Equal(t, [][]float64{{1, 2}, {3, 4}}, vectors)

	mock.FailWith(http.StatusInternalServerError)
	_, err = client.GetCompletion(context.Background(), "prompt")
	assert.Error(t, err)
	assert.Len(t, mock.Requests(), 3)
}

func TestNewMockDB(t *testing.T) {
	db, mock := NewMockDB(t)
	mock.ExpectExec("DELETE FROM users").WillReturnResult(sqlmock.NewResult(0, 1))

	_, err := db.Exec("DELETE FROM users WHERE id = $1", 1)
	require.NoError(t, err)
}

func TestCallbackUpdate_PopulatesEffectiveMessage(t *testing.T) {
	bot, _ := NewTestBot()

	ctx := ext.NewContext(bot, CallbackUpdate(55, "action"), nil)

	require.NotNil(t, ctx.EffectiveMessage)
	assert.Equal(t, int64(55), ctx.EffectiveMessage.Chat.Id)
	assert.Equal(t, int64(55), ctx.EffectiveUser.Id)
	assert.Equal(t, "action", ctx.CallbackQuery.Data)
}
