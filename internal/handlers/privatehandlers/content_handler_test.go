package privatehandlers

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/prompts"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func specByName(t *testing.T, name string) aiSearchSpec {
	t.Helper()
	for _, s := range aiSearchSpecs() {
		if s.name == name {
			return s
		}
	}
	t.Fatalf("unknown spec %q", name)
	return aiSearchSpec{}
}

// promptSection returns the value of a "KEY=value" line of a test prompt template
func promptSection(t *testing.T, prompt string, key string) string {
	t.Helper()
	for _, line := range strings.Split(prompt, "\n") {
		if value, ok := strings.CutPrefix(line, key+"="); ok {
			return value
		}
	}
	t.Fatalf("prompt has no %s section: %q", key, prompt)
	return ""
}

func TestContentHandler_PromptContainsCleanedTopicMessages(t *testing.T) {
	d := newDeps(t)
	h := d.newContentHandler()
	spec := specByName(t, "content")
	startSearch(t, d, h, spec)

	moscow := time.FixedZone("MSK", 3*60*60)
	d.expectGroupMessages(testContentTopicID, groupMessageRows(
		repositories.GroupMessage{ID: 1, MessageID: 4242, MessageText: "  Видео про Go" + constants.CopyrightString + "  ",
			CreatedAt: time.Date(2024, 5, 2, 1, 30, 0, 0, moscow), UpdatedAt: testTime},
		repositories.GroupMessage{ID: 2, MessageID: 4343, MessageText: "   ", CreatedAt: testTime, UpdatedAt: testTime},
		repositories.GroupMessage{ID: 3, MessageID: 4444, MessageText: constants.CopyrightString, CreatedAt: testTime, UpdatedAt: testTime},
	))
	d.expectTemplate(prompts.GetContentPromptKey, contentTestTemplate)

	require.True(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, spec.fastCallback)))

	prompt := d.openaiMock.LastPrompt()
	assert.Equal(t, "https://telegram.me/c/1234567890/11", promptSection(t, prompt, "LINK"))
	assert.Equal(t, searchQuery, promptSection(t, prompt, "QUERY"))

	var data []map[string]any
	require.NoError(t, json.Unmarshal([]byte(promptSection(t, prompt, "DATA")), &data))
	require.Len(t, data, 1, "blank and copyright-only messages are skipped")
	assert.Equal(t, "Видео про Go", data[0]["message"], "copyright is stripped and text trimmed")
	assert.Equal(t, "2024.05.01", data[0]["date"], "dates are formatted in UTC")
	assert.Contains(t, promptSection(t, prompt, "DATA"), "4242")
	assert.NotContains(t, prompt, "4343")
	assert.NotContains(t, prompt, "4444")
}

func TestContentHandler_DataLoadingErrors(t *testing.T) {
	tests := []struct {
		name     string
		expect   func(d *deps)
		wantText string
	}{
		{
			name: "messages query fails",
			expect: func(d *deps) {
				d.sql.ExpectQuery(selectGroupMsgsSQL).WithArgs(int64(testContentTopicID)).WillReturnError(errors.New("db down"))
			},
			wantText: "Произошла ошибка при получении сообщений из базы данных.",
		},
		{
			name: "no messages in the topic",
			expect: func(d *deps) {
				d.expectGroupMessages(testContentTopicID, groupMessageRows())
			},
			wantText: "Произошла ошибка при подготовке сообщений для поиска.",
		},
		{
			name: "only blank messages",
			expect: func(d *deps) {
				d.expectGroupMessages(testContentTopicID, groupMessageRows(
					repositories.GroupMessage{ID: 1, MessageID: 1, MessageText: " ", CreatedAt: testTime, UpdatedAt: testTime},
					repositories.GroupMessage{ID: 2, MessageID: 2, MessageText: constants.CopyrightString, CreatedAt: testTime, UpdatedAt: testTime},
				))
			},
			wantText: "Произошла ошибка при подготовке сообщений для поиска.",
		},
		{
			name: "template query fails",
			expect: func(d *deps) {
				specByName(t, "content").expectData(d)
				d.sql.ExpectQuery(selectTemplateSQL).WithArgs(prompts.GetContentPromptKey).WillReturnError(errors.New("db down"))
			},
			wantText: "Произошла ошибка при получении шаблона для поиска контента.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t)
			h := d.newContentHandler()
			spec := specByName(t, "content")
			startSearch(t, d, h, spec)
			d.client.Reset()
			tt.expect(d)

			require.True(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, spec.fastCallback)))

			assert.Equal(t, []string{searchingFastText, tt.wantText}, d.client.SentTexts())
			assert.Empty(t, d.openaiMock.Requests())
			assert.False(t, d.dispatch(t, h, privateText(testUserID, searchQuery)), "conversation is ended")
		})
	}
}

func TestContentHandler_MissingTemplateFallsBackToDefault(t *testing.T) {
	d := newDeps(t)
	h := d.newContentHandler()
	spec := specByName(t, "content")
	startSearch(t, d, h, spec)

	specByName(t, "content").expectData(d)
	d.sql.ExpectQuery(selectTemplateSQL).WithArgs(prompts.GetContentPromptKey).
		WillReturnRows(sqlmock.NewRows([]string{"template_text"}))
	d.sql.ExpectExec(insertTemplateSQL).
		WithArgs(prompts.GetContentPromptKey, prompts.GetContentPromptDefaultValue).
		WillReturnResult(sqlmock.NewResult(1, 1))

	require.True(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, spec.deepCallback)))

	prompt := d.openaiMock.LastPrompt()
	assert.Contains(t, prompt, "https://telegram.me/c/1234567890/11")
	assert.Contains(t, prompt, "<request>"+searchQuery+"</request>")
	assert.Equal(t, "mock completion", lastSend(t, d.client).Params["text"])
}
