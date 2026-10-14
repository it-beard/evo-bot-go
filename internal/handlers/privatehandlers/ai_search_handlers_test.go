package privatehandlers

// Shared conversation tests for the three AI search handlers (/content, /tools, /intro).
// They share the same flow: command -> query -> search type (fast/deep) -> OpenAI answer,
// with cancel/back handling and a guard against parallel searches.
// Handler-specific data loading and prompt building is covered in the per-handler test files.

import (
	"net/http"
	"os"
	"testing"

	"evo-bot-go/internal/database/prompts"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	searchQuery          = "golang"
	selectTypeText       = "Запрос: \"golang\"\n\nВыбери тип поиска:"
	emptyQueryText       = "Поисковый запрос не может быть пустым. Пожалуйста, введи запрос или используй /cancel для отмены."
	chooseTypeFirstText  = "Сначала выбери тип поиска кнопками выше!"
	busyText             = "Пожалуйста, дождись окончания обработки предыдущего запроса, или используй /cancel для отмены."
	openAIErrorText      = "Произошла ошибка при получении ответа от OpenAI."
	sendAnswerErrorText  = "Произошла ошибка при отправке ответа."
	searchingFastText    = "Ищу информацию по запросу: \"golang\" (быстрый поиск)..."
	searchingDeepText    = "Ищу информацию по запросу: \"golang\" (глубокий поиск)..."
	contentTestTemplate  = "LINK=%s\nDATA=%s\nQUERY=%s"
	toolsTestTemplate    = "LINK=%s\nNAME=%s\nLINK2=%s\nDATA=%s\nQUERY=%s"
	introTestTemplate    = "LINK=%s\nDATA=%s\nQUERY=%s"
	testOpenAIAnswerHTML = "<b>Найдено:</b> всё про golang"
)

type aiSearchSpec struct {
	name           string
	command        string
	fastCallback   string
	deepCallback   string
	cancelCallback string
	startText      string // substring of the message asking for a query
	startParseMode string
	cancelText     string
	newHandler     func(d *deps) ext.Handler
	expectData     func(d *deps) // DB expectations for loading the search data (before the template)
	templateKey    string
	testTemplate   string
}

// expectSearchSQL sets the DB expectations of a search that reaches OpenAI
func (s aiSearchSpec) expectSearchSQL(d *deps) {
	s.expectData(d)
	d.expectTemplate(s.templateKey, s.testTemplate)
}

func aiSearchSpecs() []aiSearchSpec {
	return []aiSearchSpec{
		{
			name:           "content",
			command:        "/content",
			fastCallback:   contentCallbackFastSearch,
			deepCallback:   contentCallbackDeepSearch,
			cancelCallback: contentCallbackConfirmCancel,
			startText:      "Пришли мне поисковый запрос по контенту:",
			cancelText:     "Операция поиска контента отменена.",
			newHandler:     (*deps).newContentHandler,
			expectData: func(d *deps) {
				d.expectGroupMessages(testContentTopicID, groupMessageRows(
					repositories.GroupMessage{ID: 1, MessageID: 4242, MessageText: "Видео про Go", CreatedAt: testTime, UpdatedAt: testTime},
				))
			},
			templateKey:  prompts.GetContentPromptKey,
			testTemplate: contentTestTemplate,
		},
		{
			name:           "tools",
			command:        "/tools",
			fastCallback:   toolsCallbackFastSearch,
			deepCallback:   toolsCallbackDeepSearch,
			cancelCallback: toolsCallbackConfirmCancel,
			startText:      "Пришли мне поисковый запрос по инструментам:",
			cancelText:     "Операция поиска инструментов отменена.",
			newHandler:     (*deps).newToolsHandler,
			expectData: func(d *deps) {
				d.expectGroupMessages(testToolTopicID, groupMessageRows(
					repositories.GroupMessage{ID: 1, MessageID: 4242, MessageText: "GoLand IDE", CreatedAt: testTime, UpdatedAt: testTime},
				))
				d.sql.ExpectQuery(selectGroupTopicSQL).WithArgs(int64(testToolTopicID)).
					WillReturnRows(sqlmock.NewRows(groupTopicColumns).AddRow(1, testToolTopicID, "Тулзы", testTime, testTime))
			},
			templateKey:  prompts.GetToolPromptKey,
			testTemplate: toolsTestTemplate,
		},
		{
			name:           "intro",
			command:        "/intro",
			fastCallback:   introCallbackFastSearch,
			deepCallback:   introCallbackDeepSearch,
			cancelCallback: introCallbackConfirmCancel,
			startText:      "Пришли мне поисковый запрос для интеллектуального поиска по участникам клуба",
			startParseMode: "HTML",
			cancelText:     "Поиск отменён.",
			newHandler:     (*deps).newIntroHandler,
			expectData: func(d *deps) {
				d.expectActiveProfiles(activeProfileRow{ProfileID: 1, UserID: 10, Bio: "Пишу на Go", PublishedID: int64(555),
					TgID: 1010, Firstname: "Иван", Lastname: "Петров", Username: "ivanpetrov"})
			},
			templateKey:  prompts.GetIntroPromptKey,
			testTemplate: introTestTemplate,
		},
	}
}

// startSearch runs the command and sends the search query
func startSearch(t *testing.T, d *deps, h ext.Handler, spec aiSearchSpec) {
	t.Helper()
	require.True(t, d.dispatch(t, h, privateText(testUserID, spec.command)))
	require.True(t, d.dispatch(t, h, privateText(testUserID, searchQuery)))
}

func TestAISearchHandlers_NonPrivateChatIsRejected(t *testing.T) {
	for _, spec := range aiSearchSpecs() {
		t.Run(spec.name, func(t *testing.T) {
			d := newDeps(t)
			h := spec.newHandler(d)

			require.True(t, d.dispatch(t, h, groupText(testUserID, spec.command)))

			assertPrivateOnlyRejection(t, d.client)
			// No conversation was started in the group
			assert.False(t, d.dispatch(t, h, groupText(testUserID, searchQuery)))
		})
	}
}

func TestAISearchHandlers_NonClubMemberIsRejected(t *testing.T) {
	for _, spec := range aiSearchSpecs() {
		t.Run(spec.name, func(t *testing.T) {
			d := newDeps(t)
			d.client.SetMemberStatus(testUserID, "left")
			h := spec.newHandler(d)

			require.True(t, d.dispatch(t, h, privateText(testUserID, spec.command)))

			assertClubOnlyRejection(t, d.client)
			assert.False(t, d.dispatch(t, h, privateText(testUserID, searchQuery)))
		})
	}
}

func TestAISearchHandlers_AsksForQueryAndSearchType(t *testing.T) {
	for _, spec := range aiSearchSpecs() {
		t.Run(spec.name, func(t *testing.T) {
			d := newDeps(t)
			h := spec.newHandler(d)

			require.True(t, d.dispatch(t, h, privateText(testUserID, spec.command)))

			ask := lastSend(t, d.client)
			assert.Equal(t, testUserID, ask.Int64("chat_id"))
			assert.Contains(t, ask.Params["text"], spec.startText)
			assert.Equal(t, spec.startParseMode, ask.Params["parse_mode"])
			assert.Equal(t, []string{spec.cancelCallback}, callbackData(t, ask))
			askID := d.sentID(t, spec.startText)

			// An empty query keeps the conversation waiting for a query
			require.True(t, d.dispatch(t, h, privateText(testUserID, "   ")))
			assert.Equal(t, emptyQueryText, lastSend(t, d.client).Params["text"])
			assert.Empty(t, d.client.CallsTo("deleteMessage"))

			queryMsg := testutils.NewPrivateMessage(testUserID, "  "+searchQuery+"  ")
			require.True(t, d.dispatch(t, h, testutils.MessageUpdate(queryMsg)))

			assert.ElementsMatch(t, []int64{queryMsg.MessageId, askID}, deletedMessageIDs(d.client),
				"both the query and the previous prompt are removed")
			selectMsg := lastSend(t, d.client)
			assert.Equal(t, selectTypeText, selectMsg.Params["text"])
			assert.Equal(t, []string{spec.fastCallback, spec.deepCallback, spec.cancelCallback}, callbackData(t, selectMsg))

			// Typing text instead of pressing a button only reminds to choose the type
			d.client.Reset()
			require.True(t, d.dispatch(t, h, privateText(testUserID, "ещё текст")))
			assert.Equal(t, []string{chooseTypeFirstText}, d.client.SentTexts())
			assert.Empty(t, d.openaiMock.Requests())
		})
	}
}

func TestAISearchHandlers_CancelByCommand(t *testing.T) {
	for _, spec := range aiSearchSpecs() {
		t.Run(spec.name, func(t *testing.T) {
			d := newDeps(t)
			h := spec.newHandler(d)
			require.True(t, d.dispatch(t, h, privateText(testUserID, spec.command)))
			askID := d.sentID(t, spec.startText)
			d.client.Reset()

			require.True(t, d.dispatch(t, h, privateText(testUserID, "/cancel")))

			assert.Equal(t, []string{spec.cancelText}, d.client.SentTexts())
			assert.Equal(t, []int64{askID}, deletedMessageIDs(d.client))
			// The conversation is over: a plain text is not treated as a query anymore
			assert.False(t, d.dispatch(t, h, privateText(testUserID, searchQuery)))
		})
	}
}

func TestAISearchHandlers_CancelByButton(t *testing.T) {
	for _, spec := range aiSearchSpecs() {
		t.Run(spec.name, func(t *testing.T) {
			d := newDeps(t)
			h := spec.newHandler(d)
			startSearch(t, d, h, spec)
			selectID := d.sentID(t, selectTypeText)
			d.client.Reset()

			require.True(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, spec.cancelCallback)))

			assert.Len(t, d.client.CallsTo("answerCallbackQuery"), 1)
			assert.Equal(t, []string{spec.cancelText}, d.client.SentTexts())
			assert.Equal(t, testUserID, lastSend(t, d.client).Int64("chat_id"))
			assert.Equal(t, []int64{selectID}, deletedMessageIDs(d.client))
			assert.False(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, spec.fastCallback)))
			assert.Empty(t, d.openaiMock.Requests())
		})
	}
}

func TestAISearchHandlers_ButtonsOutsideConversationAreIgnored(t *testing.T) {
	for _, spec := range aiSearchSpecs() {
		t.Run(spec.name, func(t *testing.T) {
			d := newDeps(t)
			h := spec.newHandler(d)

			assert.False(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, spec.fastCallback)))
			assert.False(t, d.dispatch(t, h, privateText(testUserID, searchQuery)))
			assert.Empty(t, d.client.Calls())
		})
	}
}

func TestAISearchHandlers_SuccessfulSearch(t *testing.T) {
	searchTypes := []struct {
		name            string
		callback        func(spec aiSearchSpec) string
		searchingText   string
		reasoningEffort string
	}{
		{name: "fast", callback: func(s aiSearchSpec) string { return s.fastCallback }, searchingText: searchingFastText, reasoningEffort: "minimal"},
		{name: "deep", callback: func(s aiSearchSpec) string { return s.deepCallback }, searchingText: searchingDeepText, reasoningEffort: "medium"},
	}

	for _, spec := range aiSearchSpecs() {
		for _, st := range searchTypes {
			t.Run(spec.name+"/"+st.name, func(t *testing.T) {
				d := newDeps(t)
				d.openaiMock.SetCompletion(testOpenAIAnswerHTML)
				h := spec.newHandler(d)
				startSearch(t, d, h, spec)
				selectID := d.sentID(t, selectTypeText)
				d.client.Reset()
				spec.expectSearchSQL(d)

				require.True(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, st.callback(spec))))

				assert.Len(t, d.client.CallsTo("answerCallbackQuery"), 1)
				assert.Equal(t, []string{st.searchingText, testOpenAIAnswerHTML}, d.client.SentTexts())

				searching := sendWithText(t, d.client, st.searchingText)
				assert.Equal(t, []string{spec.cancelCallback}, callbackData(t, searching))
				searchingID := d.sentID(t, st.searchingText)

				typing, ok := d.client.LastCallTo("sendChatAction")
				require.True(t, ok)
				assert.Equal(t, "typing", typing.Params["action"])
				assert.Equal(t, testUserID, typing.Int64("chat_id"))

				answer := lastSend(t, d.client)
				assert.Equal(t, testUserID, answer.Int64("chat_id"))
				assert.Equal(t, "HTML", answer.Params["parse_mode"])

				// The type selection and the "searching" status messages are cleaned up
				assert.Equal(t, []int64{selectID, searchingID}, deletedMessageIDs(d.client))

				requests := d.openaiMock.Requests()
				require.Len(t, requests, 1)
				assert.Equal(t, "/chat/completions", requests[0].Path)
				assert.Equal(t, "gpt-5-mini", requests[0].Body["model"])
				assert.Equal(t, st.reasoningEffort, requests[0].Body["reasoning_effort"])
				prompt := d.openaiMock.LastPrompt()
				assert.Contains(t, prompt, "QUERY="+searchQuery)

				logged, err := os.ReadFile("last-prompt-log.txt")
				require.NoError(t, err)
				assert.Equal(t, prompt, string(logged), "the last prompt is logged to a file")

				// The conversation is finished
				assert.False(t, d.dispatch(t, h, privateText(testUserID, searchQuery)))
			})
		}
	}
}

func TestAISearchHandlers_OpenAIError(t *testing.T) {
	for _, spec := range aiSearchSpecs() {
		t.Run(spec.name, func(t *testing.T) {
			d := newDeps(t)
			d.openaiMock.FailWith(http.StatusInternalServerError)
			h := spec.newHandler(d)
			startSearch(t, d, h, spec)
			d.client.Reset()
			spec.expectSearchSQL(d)

			require.True(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, spec.fastCallback)))

			assert.Equal(t, []string{searchingFastText, openAIErrorText}, d.client.SentTexts())
			assert.Len(t, d.openaiMock.Requests(), 1)
			assert.False(t, d.dispatch(t, h, privateText(testUserID, searchQuery)), "conversation is ended")
		})
	}
}

func TestAISearchHandlers_SendingAnswerFails(t *testing.T) {
	for _, spec := range aiSearchSpecs() {
		t.Run(spec.name, func(t *testing.T) {
			d := newDeps(t)
			h := spec.newHandler(d)
			startSearch(t, d, h, spec)
			d.client.Reset()
			d.failHTMLSends()
			spec.expectSearchSQL(d)

			require.True(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, spec.deepCallback)))

			assert.Equal(t, []string{searchingDeepText, "mock completion", sendAnswerErrorText}, d.client.SentTexts())
			assert.False(t, d.dispatch(t, h, privateText(testUserID, searchQuery)), "conversation is ended")
		})
	}
}

func TestAISearchHandlers_BusyWhileSearchIsRunning(t *testing.T) {
	for _, spec := range aiSearchSpecs() {
		t.Run(spec.name, func(t *testing.T) {
			d := newDeps(t)
			blockingClient, server := newBlockingOpenAI(t, testOpenAIAnswerHTML)
			d.openai = blockingClient
			h := spec.newHandler(d)
			startSearch(t, d, h, spec)
			spec.expectSearchSQL(d)

			done := d.dispatchAsync(h, testutils.CallbackUpdate(testUserID, spec.fastCallback))
			server.waitStarted(t)
			searchingID := d.sentID(t, searchingFastText)

			// A second request while the first one is still running is rejected with a warning
			d.client.Reset()
			another := testutils.NewPrivateMessage(testUserID, "ещё запрос")
			require.True(t, d.dispatch(t, h, testutils.MessageUpdate(another)))

			assert.Equal(t, []string{busyText}, d.client.SentTexts())
			assert.Equal(t, []string{spec.cancelCallback}, callbackData(t, lastSend(t, d.client)))
			assert.ElementsMatch(t, []int64{searchingID, another.MessageId}, deletedMessageIDs(d.client))
			warningID := d.sentID(t, busyText)

			// The first search still completes normally
			server.Release()
			require.NoError(t, waitDone(t, done))
			assert.Equal(t, testOpenAIAnswerHTML, lastSend(t, d.client).Params["text"])
			assert.Contains(t, deletedMessageIDs(d.client), warningID)
			assert.False(t, d.dispatch(t, h, privateText(testUserID, searchQuery)), "conversation is ended")
		})
	}
}

func TestAISearchHandlers_CancelWhileSearchIsRunning(t *testing.T) {
	for _, spec := range aiSearchSpecs() {
		t.Run(spec.name, func(t *testing.T) {
			d := newDeps(t)
			blockingClient, server := newBlockingOpenAI(t, testOpenAIAnswerHTML)
			d.openai = blockingClient
			h := spec.newHandler(d)
			startSearch(t, d, h, spec)
			spec.expectSearchSQL(d)

			done := d.dispatchAsync(h, testutils.CallbackUpdate(testUserID, spec.deepCallback))
			server.waitStarted(t)
			searchingID := d.sentID(t, searchingDeepText)
			d.client.Reset()

			require.True(t, d.dispatch(t, h, privateText(testUserID, "/cancel")))

			server.waitCancelled(t)
			require.NoError(t, waitDone(t, done))
			assert.Equal(t, []string{spec.cancelText}, d.client.SentTexts(),
				"neither an answer nor an OpenAI error is sent for a cancelled search")
			assert.Equal(t, []int64{searchingID}, deletedMessageIDs(d.client))
			assert.False(t, d.dispatch(t, h, privateText(testUserID, searchQuery)), "conversation is ended")
		})
	}
}
