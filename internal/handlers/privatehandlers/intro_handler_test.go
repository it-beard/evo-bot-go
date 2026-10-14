package privatehandlers

import (
	"encoding/json"
	"errors"
	"testing"

	"evo-bot-go/internal/database/prompts"
	"evo-bot-go/internal/testutils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntroHandler_StartMessageRecommendsProfileSearch(t *testing.T) {
	d := newDeps(t)

	require.True(t, d.dispatch(t, d.newIntroHandler(), privateText(testUserID, "/intro")))

	ask := lastSend(t, d.client)
	assert.Equal(t, "HTML", ask.Params["parse_mode"])
	assert.Contains(t, ask.Params["text"],
		"<blockquote> ⚠️ Для быстрого полнотекстового поиска по имени или нику участника лучше использовать менеджер профилей через команду /profile. </blockquote>")
}

func TestIntroHandler_ProfileCommandInsteadOfQueryEndsSearch(t *testing.T) {
	d := newDeps(t)
	h := d.newIntroHandler()
	require.True(t, d.dispatch(t, h, privateText(testUserID, "/intro")))
	d.client.Reset()

	require.True(t, d.dispatch(t, h, privateText(testUserID, "/profile")))

	assert.Equal(t, []string{"Поиск отменён. Нажми /profile ещё разок."}, d.client.SentTexts())
	assert.False(t, d.dispatch(t, h, privateText(testUserID, searchQuery)), "conversation is ended")
}

func TestIntroHandler_PromptContainsProfilesWithBio(t *testing.T) {
	d := newDeps(t)
	h := d.newIntroHandler()
	spec := specByName(t, "intro")
	startSearch(t, d, h, spec)

	d.expectActiveProfiles(
		activeProfileRow{ProfileID: 1, UserID: 10, Bio: "Пишу на Go", PublishedID: int64(555),
			TgID: 1010, Firstname: "Иван", Lastname: "Петров", Username: "ivanpetrov"},
		activeProfileRow{ProfileID: 2, UserID: 20, Bio: "", PublishedID: int64(666),
			TgID: 2020, Firstname: "Пустой", Lastname: "Профиль", Username: "emptybio"},
	)
	d.expectTemplate(prompts.GetIntroPromptKey, introTestTemplate)

	require.True(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, spec.fastCallback)))

	prompt := d.openaiMock.LastPrompt()
	assert.Equal(t, "https://telegram.me/c/1234567890/13", promptSection(t, prompt, "LINK"))
	assert.Equal(t, searchQuery, promptSection(t, prompt, "QUERY"))

	var data []map[string]any
	require.NoError(t, json.Unmarshal([]byte(promptSection(t, prompt, "DATA")), &data))
	require.Len(t, data, 1, "profiles without bio are skipped")
	assert.Equal(t, float64(1), data[0]["id"])
	assert.Equal(t, "Иван", data[0]["firstname"])
	assert.Equal(t, "Петров", data[0]["lastname"])
	assert.Equal(t, "ivanpetrov", data[0]["username"])
	assert.Equal(t, "Пишу на Go", data[0]["bio"])
	assert.Contains(t, promptSection(t, prompt, "DATA"), "555", "published intro message id is included")
	assert.NotContains(t, prompt, "emptybio")
}

func TestIntroHandler_DataLoadingErrors(t *testing.T) {
	tests := []struct {
		name     string
		expect   func(d *deps)
		wantText string
	}{
		{
			name: "profiles query fails",
			expect: func(d *deps) {
				d.sql.ExpectQuery(selectProfilesActive).WillReturnError(errors.New("db down"))
			},
			wantText: "Произошла ошибка при получении данных профилей для обработки.",
		},
		{
			name:     "no profiles",
			expect:   func(d *deps) { d.expectActiveProfiles() },
			wantText: "Произошла ошибка при получении данных профилей для обработки.",
		},
		{
			name: "only profiles without bio",
			expect: func(d *deps) {
				d.expectActiveProfiles(activeProfileRow{ProfileID: 2, UserID: 20, TgID: 2020, Firstname: "A", Lastname: "B"})
			},
			wantText: "Произошла ошибка при получении данных профилей для обработки.",
		},
		{
			name: "template query fails",
			expect: func(d *deps) {
				specByName(t, "intro").expectData(d)
				d.sql.ExpectQuery(selectTemplateSQL).WithArgs(prompts.GetIntroPromptKey).WillReturnError(errors.New("db down"))
			},
			wantText: "Произошла ошибка при получении шаблона для вводной информации.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t)
			h := d.newIntroHandler()
			spec := specByName(t, "intro")
			startSearch(t, d, h, spec)
			d.client.Reset()
			tt.expect(d)

			require.True(t, d.dispatch(t, h, testutils.CallbackUpdate(testUserID, spec.deepCallback)))

			assert.Equal(t, []string{searchingDeepText, tt.wantText}, d.client.SentTexts())
			assert.Empty(t, d.openaiMock.Requests())
			assert.False(t, d.dispatch(t, h, privateText(testUserID, searchQuery)), "conversation is ended")
		})
	}
}

func TestIntroHandler_FailedPromptMessageStillWaitsForQuery(t *testing.T) {
	d := newDeps(t)
	h := d.newIntroHandler()
	d.client.OnError("sendMessage", "Forbidden: bot was blocked by the user")

	require.True(t, d.dispatch(t, h, privateText(testUserID, "/intro")))

	// No previous message was remembered, so nothing is deleted except the query itself
	query := privateText(testUserID, searchQuery)
	require.True(t, d.dispatch(t, h, query))
	assert.Equal(t, []int64{query.Message.MessageId}, deletedMessageIDs(d.client))
	assert.Equal(t, selectTypeText, lastSend(t, d.client).Params["text"])
}
