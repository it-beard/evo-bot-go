package services

import (
	"testing"

	"evo-bot-go/internal/testutils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPollSenderService_SendPoll(t *testing.T) {
	answers := []gotgbot.InputPollOption{{Text: "Yes"}, {Text: "No"}}

	t.Run("with options in topic", func(t *testing.T) {
		bot, client := testutils.NewTestBot()
		s := NewPollSenderService(bot)

		msg, err := s.SendPoll(-100, "Coffee?", answers, &gotgbot.SendPollOpts{
			MessageThreadId:       15,
			IsAnonymous:           false,
			AllowsMultipleAnswers: true,
		})

		require.NoError(t, err)
		require.NotNil(t, msg)
		require.NotNil(t, msg.Poll)
		assert.Equal(t, int64(-100), msg.Chat.Id)
		assert.Equal(t, "Coffee?", msg.Poll.Question)

		call, ok := client.LastCallTo("sendPoll")
		require.True(t, ok)
		assert.Equal(t, int64(-100), call.Int64("chat_id"))
		assert.Equal(t, "Coffee?", call.Params["question"])
		assert.Equal(t, int64(15), call.Int64("message_thread_id"))
		assert.Equal(t, "false", call.Params["is_anonymous"])
		assert.Equal(t, "true", call.Params["allows_multiple_answers"])
		var sentAnswers []gotgbot.InputPollOption
		require.NoError(t, call.DecodeJSON("options", &sentAnswers))
		assert.Equal(t, answers, sentAnswers)
	})

	t.Run("nil options", func(t *testing.T) {
		bot, client := testutils.NewTestBot()
		s := NewPollSenderService(bot)

		msg, err := s.SendPoll(-100, "Coffee?", answers, nil)

		require.NoError(t, err)
		require.NotNil(t, msg)
		call, _ := client.LastCallTo("sendPoll")
		_, hasThread := call.Params["message_thread_id"]
		assert.False(t, hasThread)
	})

	t.Run("error", func(t *testing.T) {
		bot, client := testutils.NewTestBot()
		client.OnError("sendPoll", "Bad Request: poll must have at least 2 option")
		s := NewPollSenderService(bot)

		msg, err := s.SendPoll(-100, "Coffee?", answers, nil)

		require.Error(t, err)
		assert.Nil(t, msg)
	})
}

func TestPollSenderService_StopPoll(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		bot, client := testutils.NewTestBot()
		client.OnJSON("stopPoll", gotgbot.Poll{Id: "poll-77", Question: "Coffee?", IsClosed: true, Type: "regular"})
		s := NewPollSenderService(bot)

		poll, err := s.StopPoll(-100, 77, nil)

		require.NoError(t, err)
		require.NotNil(t, poll)
		assert.Equal(t, "poll-77", poll.Id)
		assert.True(t, poll.IsClosed)
		call, ok := client.LastCallTo("stopPoll")
		require.True(t, ok)
		assert.Equal(t, int64(-100), call.Int64("chat_id"))
		assert.Equal(t, int64(77), call.Int64("message_id"))
	})

	t.Run("error", func(t *testing.T) {
		bot, client := testutils.NewTestBot()
		client.OnError("stopPoll", "Bad Request: poll has already been closed")
		s := NewPollSenderService(bot)

		poll, err := s.StopPoll(-100, 77, nil)

		require.Error(t, err)
		assert.Nil(t, poll)
	})
}
