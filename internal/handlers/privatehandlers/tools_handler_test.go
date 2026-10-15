package privatehandlers

import (
	"encoding/json"
	"errors"
	"testing"

	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/prompts"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolsHandler_PromptContainsTopicAndCleanedMessages(t *testing.T) {
	d := newDeps(t)
	h := d.newToolsHandler()
	spec := specByName(t, "tools")
	startSearch(t, d, h, spec)

	d.expectGroupMessages(testToolTopicID, groupMessageRows(
		repositories.GroupMessage{ID: 1, MessageID: 4242, MessageText: "GoLand IDE" + constants.CopyrightString, CreatedAt: testTime, UpdatedAt: testTime},
		repositories.GroupMessage{ID: 2, MessageID: 4343, MessageText: "", CreatedAt: testTime, UpdatedAt: testTime},
		repositories.GroupMessage{ID: 3, MessageID: 4444, MessageText: " " + constants.CopyrightString, CreatedAt: testTime, UpdatedAt: testTime},
	))
	d.sql.ExpectQuery(selectGroupTopicSQL).WithArgs(int64(testToolTopicID)).
		WillReturnRows(sqlmock.NewRows(groupTopicColumns).AddRow(1, testToolTopicID, "Тулзы", testTime, testTime))
	d.expectTemplate(prompts.GetToolPromptKey, toolsTestTemplate)

	require.True(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, spec.fastCallback)))

	prompt := d.openaiMock.LastPrompt()
	assert.Equal(t, "https://telegram.me/c/1234567890/12", promptSection(t, prompt, "LINK"))
	assert.Equal(t, "Тулзы", promptSection(t, prompt, "NAME"))
	assert.Equal(t, "https://telegram.me/c/1234567890/12", promptSection(t, prompt, "LINK2"))
	assert.Equal(t, searchQuery, promptSection(t, prompt, "QUERY"))

	var data []map[string]any
	require.NoError(t, json.Unmarshal([]byte(promptSection(t, prompt, "DATA")), &data))
	require.Len(t, data, 1, "blank and copyright-only messages are skipped")
	assert.Equal(t, "GoLand IDE", data[0]["message"])
	assert.NotContains(t, data[0], "date", "tool messages are sent without dates")
	assert.Contains(t, promptSection(t, prompt, "DATA"), "4242")
	assert.NotContains(t, prompt, "4343")
	assert.NotContains(t, prompt, "4444")
}

func TestToolsHandler_UnknownTopicNameFallsBackToDefault(t *testing.T) {
	d := newDeps(t)
	h := d.newToolsHandler()
	spec := specByName(t, "tools")
	startSearch(t, d, h, spec)

	d.expectGroupMessages(testToolTopicID, groupMessageRows(
		repositories.GroupMessage{ID: 1, MessageID: 4242, MessageText: "GoLand IDE", CreatedAt: testTime, UpdatedAt: testTime},
	))
	d.sql.ExpectQuery(selectGroupTopicSQL).WithArgs(int64(testToolTopicID)).
		WillReturnRows(sqlmock.NewRows(groupTopicColumns))
	d.expectTemplate(prompts.GetToolPromptKey, toolsTestTemplate)

	require.True(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, spec.deepCallback)))

	assert.Equal(t, "Инструменты", promptSection(t, d.openaiMock.LastPrompt(), "NAME"))
	assert.Equal(t, "mock completion", lastSend(t, d.client).Params["text"])
}

func TestToolsHandler_DataLoadingErrors(t *testing.T) {
	tests := []struct {
		name     string
		expect   func(d *deps)
		wantText string
	}{
		{
			name: "messages query fails",
			expect: func(d *deps) {
				d.sql.ExpectQuery(selectGroupMsgsSQL).WithArgs(int64(testToolTopicID)).WillReturnError(errors.New("db down"))
			},
			wantText: "Произошла ошибка при получении сообщений из базы данных.",
		},
		{
			name: "no messages in the topic",
			expect: func(d *deps) {
				d.expectGroupMessages(testToolTopicID, groupMessageRows())
			},
			wantText: "Произошла ошибка при подготовке сообщений для поиска.",
		},
		{
			name: "only blank messages",
			expect: func(d *deps) {
				d.expectGroupMessages(testToolTopicID, groupMessageRows(
					repositories.GroupMessage{ID: 1, MessageID: 1, MessageText: "\n", CreatedAt: testTime, UpdatedAt: testTime},
				))
			},
			wantText: "Произошла ошибка при подготовке сообщений для поиска.",
		},
		{
			name: "template query fails",
			expect: func(d *deps) {
				specByName(t, "tools").expectData(d)
				d.sql.ExpectQuery(selectTemplateSQL).WithArgs(prompts.GetToolPromptKey).WillReturnError(errors.New("db down"))
			},
			wantText: "Произошла ошибка при получении шаблона для поиска инструментов.",
		},
		{
			name: "default template cannot be stored",
			expect: func(d *deps) {
				specByName(t, "tools").expectData(d)
				d.sql.ExpectQuery(selectTemplateSQL).WithArgs(prompts.GetToolPromptKey).
					WillReturnRows(sqlmock.NewRows([]string{"template_text"}))
				d.sql.ExpectExec(insertTemplateSQL).
					WithArgs(prompts.GetToolPromptKey, prompts.GetToolPromptDefaultValue).
					WillReturnError(errors.New("read-only"))
			},
			wantText: "Произошла ошибка при получении шаблона для поиска инструментов.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t)
			h := d.newToolsHandler()
			spec := specByName(t, "tools")
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

func TestToolsHandler_MissingTemplateFallsBackToDefault(t *testing.T) {
	d := newDeps(t)
	h := d.newToolsHandler()
	spec := specByName(t, "tools")
	startSearch(t, d, h, spec)

	spec.expectData(d)
	d.sql.ExpectQuery(selectTemplateSQL).WithArgs(prompts.GetToolPromptKey).
		WillReturnRows(sqlmock.NewRows([]string{"template_text"}))
	d.sql.ExpectExec(insertTemplateSQL).
		WithArgs(prompts.GetToolPromptKey, prompts.GetToolPromptDefaultValue).
		WillReturnResult(sqlmock.NewResult(1, 1))

	require.True(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, spec.fastCallback)))

	prompt := d.openaiMock.LastPrompt()
	assert.Contains(t, prompt, "https://telegram.me/c/1234567890/12")
	assert.Contains(t, prompt, "Тулзы")
	assert.Contains(t, prompt, "<request>"+searchQuery+"</request>")
}
