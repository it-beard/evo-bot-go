package tasks

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const latestPollQuery = `FROM random_coffee_polls\s+ORDER BY week_start_date DESC`

var latestPollColumns = []string{"id", "message_id", "week_start_date", "telegram_poll_id", "created_at"}

func newRandomCoffeeService(t *testing.T, cfg *config.Config) (*services.RandomCoffeeService, *testutils.MockBotClient, sqlmock.Sqlmock) {
	t.Helper()
	bot, botClient := testutils.NewTestBot()
	db, mock := testutils.NewMockDB(t)
	sender := services.NewMessageSenderService(bot)
	service := services.NewRandomCoffeeService(
		bot,
		cfg,
		services.NewPollSenderService(bot),
		sender,
		repositories.NewRandomCoffeePollRepository(db),
		repositories.NewRandomCoffeeParticipantRepository(db),
		repositories.NewProfileRepository(db),
		repositories.NewRandomCoffeePairRepository(db),
		repositories.NewUserRepository(db),
	)
	return service, botClient, mock
}

func waitForExpectations(t *testing.T, mock sqlmock.Sqlmock) {
	t.Helper()
	require.True(t, waitFor(time.Second, func() bool { return mock.ExpectationsWereMet() == nil }),
		"the scheduled job must run in the background")
}

// assertRescheduled checks that runIfDue returned a fresh future run time instead of the passed one
func assertRescheduled(t *testing.T, previous, got time.Time) {
	t.Helper()
	assert.NotEqual(t, previous, got)
	assert.True(t, got.After(time.Now().UTC()), "next run must be in the future")
}

func TestRandomCoffeePollTask_runIfDue_BeforeScheduledTimeDoesNothing(t *testing.T) {
	// a nil service proves nothing is called
	task := NewRandomCoffeePollTask(&config.Config{}, nil)
	nextRun := time.Now().Add(time.Hour)

	got := task.runIfDue(time.Now(), nextRun)

	assert.Equal(t, nextRun, got)
}

func TestRandomCoffeePollTask_runIfDue_SendsPollWhenDue(t *testing.T) {
	cfg := &config.Config{SuperGroupChatID: 123, RandomCoffeeTopicID: 5, RandomCoffeeIntervalWeeks: 1}
	service, botClient, _ := newRandomCoffeeService(t, cfg)
	// stop SendPoll right after its first API call
	botClient.OnError("sendMessage", "Bad Request: chat not found")
	task := NewRandomCoffeePollTask(cfg, service)
	nextRun := time.Now().Add(-time.Minute)

	got := task.runIfDue(time.Now(), nextRun)

	assertRescheduled(t, nextRun, got)
	require.True(t, waitFor(time.Second, func() bool { return len(botClient.CallsTo("sendMessage")) == 1 }))
	call, _ := botClient.LastCallTo("sendMessage")
	assert.Equal(t, int64(5), call.Int64("message_thread_id"))
}

func TestRandomCoffeePollTask_runIfDue_SkipsWeekWhenPollIsNotDue(t *testing.T) {
	cfg := &config.Config{SuperGroupChatID: 123, RandomCoffeeTopicID: 5, RandomCoffeeIntervalWeeks: 2}
	service, botClient, mock := newRandomCoffeeService(t, cfg)
	now := time.Now().UTC()
	mock.ExpectQuery(latestPollQuery).WillReturnRows(
		sqlmock.NewRows(latestPollColumns).AddRow(1, 10, now, "poll-1", now))
	task := NewRandomCoffeePollTask(cfg, service)
	nextRun := now.Add(-time.Minute)

	got := task.runIfDue(now, nextRun)

	assertRescheduled(t, nextRun, got)
	time.Sleep(20 * time.Millisecond)
	assert.Empty(t, botClient.Calls(), "no poll must be sent when the interval has not passed")
}

func TestRandomCoffeePollTask_runIfDue_SendsPollWhenDueCheckFails(t *testing.T) {
	cfg := &config.Config{SuperGroupChatID: 123, RandomCoffeeTopicID: 5, RandomCoffeeIntervalWeeks: 2}
	service, botClient, mock := newRandomCoffeeService(t, cfg)
	mock.ExpectQuery(latestPollQuery).WillReturnError(errors.New("db down"))
	botClient.OnError("sendMessage", "Bad Request: chat not found")
	task := NewRandomCoffeePollTask(cfg, service)
	nextRun := time.Now().Add(-time.Minute)

	got := task.runIfDue(time.Now(), nextRun)

	assertRescheduled(t, nextRun, got)
	require.True(t, waitFor(time.Second, func() bool { return len(botClient.CallsTo("sendMessage")) == 1 }))
}

func TestRandomCoffeePairsTask_runIfDue_BeforeScheduledTimeDoesNothing(t *testing.T) {
	task := NewRandomCoffeePairsTask(&config.Config{}, nil)
	nextRun := time.Now().Add(time.Hour)

	got := task.runIfDue(time.Now(), nextRun)

	assert.Equal(t, nextRun, got)
}

func TestRandomCoffeePairsTask_runIfDue_GeneratesPairsWhenDue(t *testing.T) {
	cfg := &config.Config{SuperGroupChatID: 123, RandomCoffeeTopicID: 5, RandomCoffeeIntervalWeeks: 1}
	service, _, mock := newRandomCoffeeService(t, cfg)
	// GenerateAndSendPairs starts by loading the latest poll; failing it ends the job
	mock.ExpectQuery(latestPollQuery).WillReturnError(errors.New("db down"))
	task := NewRandomCoffeePairsTask(cfg, service)
	nextRun := time.Now().Add(-time.Minute)

	got := task.runIfDue(time.Now(), nextRun)

	assertRescheduled(t, nextRun, got)
	waitForExpectations(t, mock)
}

func TestRandomCoffeePairsTask_runIfDue_SkipsWeekWhenPairsAreNotDue(t *testing.T) {
	cfg := &config.Config{SuperGroupChatID: 123, RandomCoffeeTopicID: 5, RandomCoffeeIntervalWeeks: 2}
	service, botClient, mock := newRandomCoffeeService(t, cfg)
	// no poll at all means there is nothing to pair this week
	mock.ExpectQuery(latestPollQuery).WillReturnError(sql.ErrNoRows)
	task := NewRandomCoffeePairsTask(cfg, service)
	nextRun := time.Now().Add(-time.Minute)

	got := task.runIfDue(time.Now(), nextRun)

	assertRescheduled(t, nextRun, got)
	time.Sleep(20 * time.Millisecond)
	assert.Empty(t, botClient.Calls())
}

func TestRandomCoffeePairsTask_runIfDue_GeneratesPairsWhenDueCheckFails(t *testing.T) {
	cfg := &config.Config{SuperGroupChatID: 123, RandomCoffeeTopicID: 5, RandomCoffeeIntervalWeeks: 2}
	service, _, mock := newRandomCoffeeService(t, cfg)
	mock.ExpectQuery(latestPollQuery).WillReturnError(errors.New("db down"))
	// the job still runs and queries the latest poll again
	mock.ExpectQuery(latestPollQuery).WillReturnError(errors.New("db down"))
	task := NewRandomCoffeePairsTask(cfg, service)
	nextRun := time.Now().Add(-time.Minute)

	got := task.runIfDue(time.Now(), nextRun)

	assertRescheduled(t, nextRun, got)
	waitForExpectations(t, mock)
}

func TestDailySummarizationTask_runIfDue_BeforeScheduledTimeDoesNothing(t *testing.T) {
	task := NewDailySummarizationTask(&config.Config{}, nil)
	nextRun := time.Now().Add(time.Hour)

	got := task.runIfDue(time.Now(), nextRun)

	assert.Equal(t, nextRun, got)
}

func TestDailySummarizationTask_runIfDue_RunsSummarizationWhenDue(t *testing.T) {
	bot, _ := testutils.NewTestBot()
	db, mock := testutils.NewMockDB(t)
	openaiClient, _ := testutils.NewMockOpenAI(t)
	cfg := &config.Config{MonitoredTopicsIDs: []int{10}, SummaryTime: time.Date(0, 1, 1, 3, 0, 0, 0, time.UTC)}
	service := services.NewSummarizationService(
		cfg,
		openaiClient,
		services.NewMessageSenderService(bot),
		repositories.NewGroupTopicRepository(db),
		repositories.NewPromptingTemplateRepository(db),
		repositories.NewGroupMessageRepository(db),
	)
	// summarization of the monitored topic starts with loading the topic and its messages
	mock.ExpectQuery(`FROM group_topics`).WillReturnError(errors.New("db down"))
	mock.ExpectQuery(`FROM group_messages`).WillReturnError(errors.New("db down"))
	task := NewDailySummarizationTask(cfg, service)
	nextRun := time.Now().Add(-time.Minute)

	got := task.runIfDue(time.Now(), nextRun)

	assertRescheduled(t, nextRun, got)
	waitForExpectations(t, mock)
}
