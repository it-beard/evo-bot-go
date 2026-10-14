package services

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"evo-bot-go/internal/database/prompts"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPromptTemplate has exactly the 9 placeholders the service fills
const testPromptTemplate = "chat=%s topic=%s|%s/%s|%s/%s|%s/%s|CONTEXT:%s"

type summarizationFixture struct {
	svc    *SummarizationService
	client *testutils.MockBotClient
	mock   sqlmock.Sqlmock
	ai     *testutils.MockOpenAI
	dir    string
}

func newSummarizationFixture(t *testing.T, topics ...int) *summarizationFixture {
	t.Helper()
	dir := chdirTemp(t) // the service writes last-prompt-log.txt into the working directory

	bot, client := testutils.NewTestBot()
	db, mock := testutils.NewMockDB(t)
	aiClient, ai := testutils.NewMockOpenAI(t)
	cfg := newTestConfig()
	cfg.MonitoredTopicsIDs = topics

	svc := NewSummarizationService(
		cfg,
		aiClient,
		NewMessageSenderService(bot),
		repositories.NewGroupTopicRepository(db),
		repositories.NewPromptingTemplateRepository(db),
		repositories.NewGroupMessageRepository(db),
	)
	return &summarizationFixture{svc: svc, client: client, mock: mock, ai: ai, dir: dir}
}

type testGroupMessage struct {
	messageID int64
	userTgID  int64
	text      string
	createdAt time.Time
}

func (f *summarizationFixture) expectTopicName(topicID int64, name string) {
	now := time.Now()
	f.mock.ExpectQuery(groupTopicQuery).WithArgs(topicID).
		WillReturnRows(sqlmock.NewRows(groupTopicColumns).AddRow(1, topicID, name, now, now))
}

func (f *summarizationFixture) expectMessages(topicID int64, msgs ...testGroupMessage) {
	rows := sqlmock.NewRows(groupMsgColumns)
	for i, m := range msgs {
		rows.AddRow(i+1, m.messageID, m.text, nil, m.userTgID, topicID, m.createdAt, m.createdAt)
	}
	f.mock.ExpectQuery(lastDayMessages).WithArgs(topicID).WillReturnRows(rows)
}

func (f *summarizationFixture) expectTemplate(text string) {
	f.mock.ExpectQuery(templateQuery).WithArgs(prompts.DailySummarizationPromptKey).
		WillReturnRows(sqlmock.NewRows([]string{"template_text"}).AddRow(text))
}

// todayDates returns the accepted summary dates (handles a run across midnight)
func summaryDates(before, after time.Time) []string {
	return []string{before.Format("02.01.2006"), after.Format("02.01.2006")}
}

func TestSummarizationService_RunDailySummarization_SendsSummaryToTopic(t *testing.T) {
	f := newSummarizationFixture(t, 5)
	ts := time.Date(2026, 3, 4, 10, 20, 30, 0, time.UTC)
	f.expectTopicName(5, "Общий чат")
	f.expectMessages(5,
		testGroupMessage{messageID: 11, userTgID: 501, text: "hello there", createdAt: ts},
		testGroupMessage{messageID: 12, userTgID: 502, text: "general kenobi", createdAt: ts.Add(time.Minute)},
	)
	f.expectTemplate(testPromptTemplate)
	f.ai.SetCompletion("SUMMARY BODY")

	before := time.Now()
	err := f.svc.RunDailySummarization(context.Background(), false)
	after := time.Now()

	require.NoError(t, err)

	// Prompt sent to OpenAI
	reqs := f.ai.Requests()
	require.Len(t, reqs, 1)
	assert.Equal(t, "/chat/completions", reqs[0].Path)
	prompt := f.ai.LastPrompt()
	assert.True(t, strings.HasPrefix(prompt, "chat=1234567890 topic=5|1234567890/5|1234567890/5|1234567890/5|CONTEXT:"), prompt)
	assert.Contains(t, prompt, "MessageID: 11\nUserID: user_501\nTimestamp: "+time.Unix(ts.Unix(), 0).Format("2006-01-02 15:04:05")+"\nText: hello there")
	assert.Contains(t, prompt, "MessageID: 12\nUserID: user_502\n")
	assert.Less(t, strings.Index(prompt, "hello there"), strings.Index(prompt, "general kenobi"), "messages keep chronological order")

	// The prompt is logged to a file in the working directory
	logged, err := os.ReadFile(filepath.Join(f.dir, "last-prompt-log.txt"))
	require.NoError(t, err)
	assert.Equal(t, prompt, string(logged))

	// Summary sent to the summary topic
	calls := f.client.CallsTo("sendMessage")
	require.Len(t, calls, 1)
	call := calls[0]
	assert.Equal(t, testFullChatID, call.Int64("chat_id"))
	assert.Equal(t, int64(testSummaryTopicID), call.Int64("message_thread_id"))
	assert.Equal(t, "HTML", call.Params["parse_mode"])
	text := call.Params["text"]
	assert.True(t, strings.HasPrefix(text, "📋 Сводка чата <b>\"Общий чат\"</b> за "), text)
	assert.True(t, strings.HasSuffix(text, "\n\nSUMMARY BODY"), text)
	dateOK := false
	for _, d := range summaryDates(before, after) {
		dateOK = dateOK || strings.Contains(text, " за "+d+"\n")
	}
	assert.True(t, dateOK, "summary title must contain today's date: %s", text)
}

func TestSummarizationService_RunDailySummarization_SendToDM(t *testing.T) {
	t.Run("with user id in context", func(t *testing.T) {
		f := newSummarizationFixture(t, 5)
		f.expectTopicName(5, "Topic")
		f.expectMessages(5, testGroupMessage{messageID: 1, userTgID: 2, text: "hi", createdAt: time.Now()})
		f.expectTemplate(testPromptTemplate)
		//nolint:staticcheck // the service reads a plain string context key
		ctx := context.WithValue(context.Background(), "userID", int64(9999))

		require.NoError(t, f.svc.RunDailySummarization(ctx, true))

		call, ok := f.client.LastCallTo("sendMessage")
		require.True(t, ok)
		assert.Equal(t, int64(9999), call.Int64("chat_id"))
		assert.Equal(t, "HTML", call.Params["parse_mode"])
		_, hasThread := call.Params["message_thread_id"]
		assert.False(t, hasThread, "DM summaries are not sent to a topic")
	})

	t.Run("without user id falls back to summary topic", func(t *testing.T) {
		f := newSummarizationFixture(t, 5)
		f.expectTopicName(5, "Topic")
		f.expectMessages(5, testGroupMessage{messageID: 1, userTgID: 2, text: "hi", createdAt: time.Now()})
		f.expectTemplate(testPromptTemplate)

		require.NoError(t, f.svc.RunDailySummarization(context.Background(), true))

		call, ok := f.client.LastCallTo("sendMessage")
		require.True(t, ok)
		assert.Equal(t, testFullChatID, call.Int64("chat_id"))
		assert.Equal(t, int64(testSummaryTopicID), call.Int64("message_thread_id"))
	})
}

func TestSummarizationService_RunDailySummarization_MainTopicUsesIDOne(t *testing.T) {
	f := newSummarizationFixture(t, 0)
	f.expectTopicName(0, "Main")
	f.expectMessages(0, testGroupMessage{messageID: 1, userTgID: 2, text: "hi", createdAt: time.Now()})
	f.expectTemplate(testPromptTemplate)

	require.NoError(t, f.svc.RunDailySummarization(context.Background(), false))

	assert.True(t, strings.HasPrefix(f.ai.LastPrompt(), "chat=1234567890 topic=1|1234567890/1|"), f.ai.LastPrompt())
}

func TestSummarizationService_RunDailySummarization_TopicNameFallback(t *testing.T) {
	f := newSummarizationFixture(t, 5)
	f.mock.ExpectQuery(groupTopicQuery).WithArgs(int64(5)).WillReturnError(sql.ErrNoRows)
	f.expectMessages(5, testGroupMessage{messageID: 1, userTgID: 2, text: "hi", createdAt: time.Now()})
	f.expectTemplate(testPromptTemplate)

	require.NoError(t, f.svc.RunDailySummarization(context.Background(), false))

	call, ok := f.client.LastCallTo("sendMessage")
	require.True(t, ok)
	assert.Contains(t, call.Params["text"], "Topic name\"</b>")
}

func TestSummarizationService_RunDailySummarization_NoMessages(t *testing.T) {
	f := newSummarizationFixture(t, 5)
	f.expectTopicName(5, "Topic")
	f.expectMessages(5)

	require.NoError(t, f.svc.RunDailySummarization(context.Background(), false))

	assert.Empty(t, f.ai.Requests(), "nothing to summarize")
	assert.Empty(t, f.client.Calls())
}

func TestSummarizationService_RunDailySummarization_NoMonitoredTopics(t *testing.T) {
	f := newSummarizationFixture(t)

	require.NoError(t, f.svc.RunDailySummarization(context.Background(), false))

	assert.Empty(t, f.ai.Requests())
	assert.Empty(t, f.client.Calls())
}

func TestSummarizationService_RunDailySummarization_ContinuesAfterTopicFailure(t *testing.T) {
	f := newSummarizationFixture(t, 5, 6)
	f.expectTopicName(5, "Broken")
	f.mock.ExpectQuery(lastDayMessages).WithArgs(int64(5)).WillReturnError(errors.New("db is down"))
	f.expectTopicName(6, "Working")
	f.expectMessages(6, testGroupMessage{messageID: 1, userTgID: 2, text: "hi", createdAt: time.Now()})
	f.expectTemplate(testPromptTemplate)

	require.NoError(t, f.svc.RunDailySummarization(context.Background(), false))

	texts := f.client.SentTexts()
	require.Len(t, texts, 1)
	assert.Contains(t, texts[0], "\"Working\"")
}

func TestSummarizationService_RunDailySummarization_DefaultTemplateIsStored(t *testing.T) {
	f := newSummarizationFixture(t, 5)
	f.expectTopicName(5, "Topic")
	f.expectMessages(5, testGroupMessage{messageID: 1, userTgID: 2, text: "hi", createdAt: time.Now()})
	f.mock.ExpectQuery(templateQuery).WithArgs(prompts.DailySummarizationPromptKey).WillReturnError(sql.ErrNoRows)
	f.mock.ExpectExec(templateInsert).
		WithArgs(prompts.DailySummarizationPromptKey, prompts.DailySummarizationPromptDefaultValue).
		WillReturnResult(sqlmock.NewResult(1, 1))

	require.NoError(t, f.svc.RunDailySummarization(context.Background(), false))

	prompt := f.ai.LastPrompt()
	defaultPrefix := strings.SplitN(prompts.DailySummarizationPromptDefaultValue, "%", 2)[0]
	assert.True(t, strings.HasPrefix(prompt, defaultPrefix), "the default prompt template is used")
	assert.Contains(t, prompt, "Text: hi")
	assert.Len(t, f.client.CallsTo("sendMessage"), 1)
}

func TestSummarizationService_RunDailySummarization_TemplateError(t *testing.T) {
	f := newSummarizationFixture(t, 5)
	f.expectTopicName(5, "Topic")
	f.expectMessages(5, testGroupMessage{messageID: 1, userTgID: 2, text: "hi", createdAt: time.Now()})
	f.mock.ExpectQuery(templateQuery).WillReturnError(errors.New("db is down"))

	require.NoError(t, f.svc.RunDailySummarization(context.Background(), false))

	assert.Empty(t, f.ai.Requests())
	assert.Empty(t, f.client.Calls())
}

func TestSummarizationService_RunDailySummarization_OpenAIError(t *testing.T) {
	f := newSummarizationFixture(t, 5)
	f.expectTopicName(5, "Topic")
	f.expectMessages(5, testGroupMessage{messageID: 1, userTgID: 2, text: "hi", createdAt: time.Now()})
	f.expectTemplate(testPromptTemplate)
	f.ai.FailWith(http.StatusInternalServerError)

	require.NoError(t, f.svc.RunDailySummarization(context.Background(), false))

	assert.Len(t, f.ai.Requests(), 1)
	assert.Empty(t, f.client.Calls(), "nothing is sent when the summary could not be generated")
}

func TestSummarizationService_RunDailySummarization_PromptLogWriteFailureIsTolerated(t *testing.T) {
	f := newSummarizationFixture(t, 5)
	// A directory with the log file name makes the write fail
	require.NoError(t, os.Mkdir(filepath.Join(f.dir, "last-prompt-log.txt"), 0o755))
	f.expectTopicName(5, "Topic")
	f.expectMessages(5, testGroupMessage{messageID: 1, userTgID: 2, text: "hi", createdAt: time.Now()})
	f.expectTemplate(testPromptTemplate)

	require.NoError(t, f.svc.RunDailySummarization(context.Background(), false))

	assert.Len(t, f.ai.Requests(), 1)
	assert.Len(t, f.client.CallsTo("sendMessage"), 1)
}
