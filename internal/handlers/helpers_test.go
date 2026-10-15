package handlers

import (
	"testing"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/testutils"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/require"
)

const (
	testSuperGroupID       int64 = 1234567890
	testFullSuperGroupID   int64 = -1001234567890
	testRandomCoffeeTopic        = 77
	testUserID             int64 = 501
	testGroupChatIDForTest int64 = -100500
)

type startFixture struct {
	bot     *gotgbot.Bot
	client  *testutils.MockBotClient
	cfg     *config.Config
	handler ext.Handler
}

func newStartFixture(t *testing.T) *startFixture {
	t.Helper()

	bot, client := testutils.NewTestBot()
	cfg := &config.Config{
		SuperGroupChatID:    testSuperGroupID,
		RandomCoffeeTopicID: testRandomCoffeeTopic,
	}
	sender := services.NewMessageSenderService(bot)
	permissions := services.NewPermissionsService(cfg, bot, sender)

	return &startFixture{
		bot:     bot,
		client:  client,
		cfg:     cfg,
		handler: NewStartHandler(cfg, sender, permissions),
	}
}

func (f *startFixture) dispatch(t *testing.T, u *gotgbot.Update) bool {
	t.Helper()
	handled, err := testutils.Dispatch(f.bot, f.handler, u)
	require.NoError(t, err)
	return handled
}

func privateCommand(userID int64, text string) *gotgbot.Update {
	return testutils.MessageUpdate(testutils.NewPrivateMessage(userID, text))
}

// replyKeyboard decodes the reply_markup param of a Bot API call
func replyKeyboard(t *testing.T, call testutils.APICall) gotgbot.InlineKeyboardMarkup {
	t.Helper()
	var markup gotgbot.InlineKeyboardMarkup
	require.NoError(t, call.DecodeJSON("reply_markup", &markup))
	return markup
}
