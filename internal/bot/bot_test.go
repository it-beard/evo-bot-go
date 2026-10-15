package bot

import (
	"testing"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/database"
	"evo-bot-go/internal/tasks"
	"evo-bot-go/internal/testutils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTask struct {
	started int
	stopped int
}

func (f *fakeTask) Start() { f.started++ }
func (f *fakeTask) Stop()  { f.stopped++ }

func newTestClient(t *testing.T) (*TgBotClient, *testutils.MockBotClient) {
	t.Helper()

	bot, botClient := testutils.NewTestBot()
	sqlDB, _ := testutils.NewMockDB(t)
	openaiClient, _ := testutils.NewMockOpenAI(t)
	appConfig := &config.Config{
		SuperGroupChatID:          1234567890,
		RandomCoffeeIntervalWeeks: 1,
	}

	return newTgBotClient(bot, &database.DB{DB: sqlDB}, openaiClient, appConfig), botClient
}

func TestNewTgBotClient_WiresBotDatabaseAndTasks(t *testing.T) {
	client, _ := newTestClient(t)

	require.NotNil(t, client.bot)
	require.NotNil(t, client.db)
	assert.NotNil(t, client.dispatcher)
	assert.NotNil(t, client.updater)
	assert.Len(t, client.tasks, 3)
}

func TestNewTgBotClient_RegisteredHandlersProcessUpdates(t *testing.T) {
	client, botClient := newTestClient(t)

	err := client.dispatcher.ProcessUpdate(client.bot, testutils.MessageUpdate(testutils.NewPrivateMessage(42, "/help")), nil)

	require.NoError(t, err)
	reply, ok := botClient.LastCallTo("sendMessage")
	require.True(t, ok, "help command should be answered")
	assert.Equal(t, int64(42), reply.Int64("chat_id"))
	assert.Equal(t, "HTML", reply.Params["parse_mode"])
	assert.NotEmpty(t, reply.Params["text"])
}

func TestTgBotClient_Close_StopsTasksAndClosesDatabase(t *testing.T) {
	bot, _ := testutils.NewTestBot()
	sqlDB, mock := testutils.NewMockDB(t)
	mock.ExpectClose()
	task := &fakeTask{}
	client := &TgBotClient{bot: bot, db: &database.DB{DB: sqlDB}, tasks: []tasks.Task{task, task}}

	err := client.Close()

	require.NoError(t, err)
	assert.Equal(t, 2, task.stopped)
}

func TestSetupDatabase_ReturnsErrorWhenDatabaseIsUnreachable(t *testing.T) {
	db, err := setupDatabase("postgres://user:pass@127.0.0.1:1/db?sslmode=disable&connect_timeout=1")

	assert.Nil(t, db)
	assert.Error(t, err)
}
