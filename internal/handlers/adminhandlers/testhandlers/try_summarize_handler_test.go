package testhandlers

import (
	"encoding/json"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"evo-bot-go/internal/database/prompts"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	summarizeConfirmText = "Вы собираетесь запустить процесс тестирования саммаризации общения в клубе. " +
		"Саммаризация будет отправлена в личные сообщения.\n\nПодтвердите действие, нажав одну из кнопок ниже:"
	summarizeCanceledText = "Операция саммаризации отменена."
	summarizeStartedText  = "Запуск процесса саммаризации..."
	summarizeDoneText     = "Саммаризация успешно создана."

	sqlGroupTopic      = "SELECT id, topic_id, name, created_at, updated_at FROM group_topics WHERE topic_id = $1"
	sqlRecentMessages  = "FROM group_messages WHERE group_topic_id = $1 AND created_at >= NOW() - INTERVAL '24 hours'"
	sqlPromptTemplate  = "SELECT template_text FROM prompting_templates WHERE template_key = $1"
	testMonitoredTopic = 5
)

func newSummarizeEnv(t *testing.T) (*testEnv, sqlmock.Sqlmock, *testutils.MockOpenAI) {
	env := newTestEnv(t)
	db, mock := testutils.NewMockDB(t)
	openai, openaiMock := testutils.NewMockOpenAI(t)
	summarization := services.NewSummarizationService(
		env.cfg,
		openai,
		env.sender,
		repositories.NewGroupTopicRepository(db),
		repositories.NewPromptingTemplateRepository(db),
		repositories.NewGroupMessageRepository(db),
	)
	env.handler = NewTrySummarizeHandler(env.cfg, summarization, env.sender, env.permissions)
	return env, mock, openaiMock
}

// openSummarizeConfirmation runs the command and returns the confirmation message
func openSummarizeConfirmation(t *testing.T, env *testEnv) gotgbot.Message {
	t.Helper()
	require.True(t, env.sendText(testAdminID, "/trySummarize"))
	return env.lastSent().Msg
}

// waitForText waits until the bot sends a message with the given text
func waitForText(t *testing.T, env *testEnv, text string) {
	t.Helper()
	require.Eventually(t, func() bool { return slices.Contains(env.texts(), text) },
		time.Second, 5*time.Millisecond, "bot never sent %q", text)
}

// chdirTemp runs the test inside a temp dir: the summarization service writes last-prompt-log.txt into the working dir
func chdirTemp(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(wd) })
}

func TestTrySummarizeHandler_AsksForConfirmation(t *testing.T) {
	env, _, _ := newSummarizeEnv(t)

	confirm := openSummarizeConfirmation(t, env)

	assert.Equal(t, testAdminID, confirm.Chat.Id)
	assert.Equal(t, summarizeConfirmText, confirm.Text)
	assert.Equal(t, []string{trySummarizeCallbackConfirmYes, trySummarizeCallbackConfirmCancel}, callbackData(t, env.lastSent().Call))

	// Any message while waiting asks to use the buttons and keeps the state
	env.reset()
	require.True(t, env.sendText(testAdminID, "go"))
	assert.Equal(t, []string{"Пожалуйста, нажмите на одну из кнопок выше, или используйте кнопку отмены."}, env.texts())
	assert.True(t, env.sendText(testAdminID, "still here"))
}

func TestTrySummarizeHandler_ConfirmWithoutMonitoredTopics(t *testing.T) {
	env, _, openai := newSummarizeEnv(t)
	confirm := openSummarizeConfirmation(t, env)
	env.reset()

	require.True(t, env.click(testAdminID, trySummarizeCallbackConfirmYes))

	// Synchronous part: callback answered, start notice, typing action, keyboard removed, conversation over
	assert.Len(t, env.client.CallsTo("answerCallbackQuery"), 1)
	assert.Equal(t, summarizeStartedText, env.sentMessages()[0].Msg.Text)
	typing, ok := env.client.LastCallTo("sendChatAction")
	require.True(t, ok)
	assert.Equal(t, testAdminID, typing.Int64("chat_id"))
	assert.Equal(t, "typing", typing.Params["action"])
	env.requireKeyboardRemoved(confirm)
	assert.False(t, env.sendText(testAdminID, "go"), "conversation must be over")

	// Asynchronous part: nothing to summarise, the admin is told it is done
	waitForText(t, env, summarizeDoneText)
	assert.Equal(t, []string{summarizeStartedText, summarizeDoneText}, env.texts())
	assert.Empty(t, openai.Requests())
}

func TestTrySummarizeHandler_ConfirmSendsSummaryToAdminDM(t *testing.T) {
	chdirTemp(t)
	env, mock, openai := newSummarizeEnv(t)
	env.cfg.MonitoredTopicsIDs = []int{testMonitoredTopic}
	env.cfg.SummaryTopicID = 99
	openai.SetCompletion("• Обсудили Go")

	openSummarizeConfirmation(t, env)
	env.reset()

	created := time.Date(2025, 4, 7, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(sqlGroupTopic)).WithArgs(int64(testMonitoredTopic)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "topic_id", "name", "created_at", "updated_at"}).
			AddRow(1, testMonitoredTopic, "General", created, created))
	mock.ExpectQuery(regexp.QuoteMeta(sqlRecentMessages)).WithArgs(int64(testMonitoredTopic)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "message_id", "message_text", "reply_to_message_id", "user_tg_id", "group_topic_id", "created_at", "updated_at",
		}).
			AddRow(1, 501, "Hello from Alice", nil, 111, testMonitoredTopic, created, created).
			AddRow(2, 502, "Hi Bob", nil, 222, testMonitoredTopic, created, created))
	mock.ExpectQuery(regexp.QuoteMeta(sqlPromptTemplate)).WithArgs(prompts.DailySummarizationPromptKey).
		WillReturnRows(sqlmock.NewRows([]string{"template_text"}).AddRow("chat=%s topic=%s|%s|%s|%s|%s|%s|%s\nCONTEXT:%s"))

	require.True(t, env.click(testAdminID, trySummarizeCallbackConfirmYes))
	waitForText(t, env, summarizeDoneText)

	// The prompt is built from the topic messages
	prompt := openai.LastPrompt()
	assert.True(t, strings.HasPrefix(prompt, "chat=2069889012 topic=5|2069889012|5|"), prompt)
	assert.Contains(t, prompt, "MessageID: 501\nUserID: user_111")
	assert.Contains(t, prompt, "Text: Hello from Alice")
	assert.Contains(t, prompt, "Text: Hi Bob")

	// The summary goes to the admin's DM, not to the summary topic
	sent := env.sentMessages()
	require.Len(t, sent, 3)
	assert.Equal(t, summarizeStartedText, sent[0].Msg.Text)
	summary := sent[1]
	assert.Equal(t, testAdminID, summary.Msg.Chat.Id)
	assert.Zero(t, summary.Call.Int64("message_thread_id"))
	assert.Equal(t, "HTML", summary.Call.Params["parse_mode"])
	assert.True(t, strings.HasPrefix(summary.Msg.Text, "📋 Сводка чата <b>\"General\"</b> за "), summary.Msg.Text)
	assert.True(t, strings.HasSuffix(summary.Msg.Text, "\n\n• Обсудили Go"), summary.Msg.Text)
	assert.Equal(t, summarizeDoneText, sent[2].Msg.Text)
}

func TestTrySummarizeHandler_Cancel(t *testing.T) {
	tests := []struct {
		name       string
		cancel     func(env *testEnv) bool
		isCallback bool
	}{
		{"cancel button", func(env *testEnv) bool { return env.click(testAdminID, trySummarizeCallbackConfirmCancel) }, true},
		{"cancel command", func(env *testEnv) bool { return env.sendText(testAdminID, "/cancel") }, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env, _, openai := newSummarizeEnv(t)
			confirm := openSummarizeConfirmation(t, env)
			env.reset()

			require.True(t, tc.cancel(env))

			if tc.isCallback {
				assert.Len(t, env.client.CallsTo("answerCallbackQuery"), 1)
			}
			assert.Equal(t, []string{summarizeCanceledText}, env.texts())
			env.requireKeyboardRemoved(confirm)
			assert.Empty(t, env.client.CallsTo("sendChatAction"))
			assert.Empty(t, openai.Requests())
			assert.False(t, env.click(testAdminID, trySummarizeCallbackConfirmYes), "conversation must be over")
		})
	}
}

func TestTrySummarizeHandler_AccessIsRestricted(t *testing.T) {
	t.Run("non-admin", func(t *testing.T) {
		env, _, _ := newSummarizeEnv(t)

		require.True(t, env.sendText(testUserID, "/trySummarize"))

		assert.Equal(t, []string{adminOnlyText}, env.texts())
		assert.False(t, env.click(testUserID, trySummarizeCallbackConfirmYes), "conversation must not be started")
	})

	t.Run("group chat", func(t *testing.T) {
		env, _, _ := newSummarizeEnv(t)

		require.True(t, env.sendGroupText(testAdminID, "/trySummarize"))

		sent := env.sentMessages()
		require.Len(t, sent, 2)
		assert.Equal(t, fullChatID(), sent[0].Msg.Chat.Id)
		assert.Contains(t, sent[0].Msg.Text, "Эта команда работает только в _личной беседе_ со мной")
		assert.Equal(t, testAdminID, sent[1].Msg.Chat.Id)
		assert.False(t, env.sendGroupText(testAdminID, "go"), "conversation must not be started")
	})

	t.Run("admin rights lost between checks", func(t *testing.T) {
		env, _, _ := newSummarizeEnv(t)
		var checks atomic.Int32
		env.client.On("getChatMember", func(call testutils.APICall) (json.RawMessage, error) {
			status := "member"
			if checks.Add(1) == 1 {
				status = "administrator"
			}
			return json.Marshal(map[string]any{
				"status": status,
				"user":   map[string]any{"id": call.Int64("user_id"), "is_bot": false, "first_name": "Admin"},
			})
		})

		require.True(t, env.sendText(testAdminID, "/trySummarize"))

		assert.Equal(t, int32(2), checks.Load())
		assert.Equal(t, []string{adminOnlyText}, env.texts())
		assert.False(t, env.click(testAdminID, trySummarizeCallbackConfirmYes), "conversation must not be started")
	})
}
