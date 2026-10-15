package formatters

import (
	"database/sql"
	"testing"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/database/repositories"

	"github.com/stretchr/testify/assert"
)

func fullUser() *repositories.User {
	return &repositories.User{
		ID:         1,
		TgID:       123456,
		Firstname:  "Ivan",
		Lastname:   "Petrov",
		TgUsername: "ivanp",
		Score:      42,
	}
}

func minimalUser() *repositories.User {
	return &repositories.User{ID: 2, TgID: 777, Firstname: "Anna"}
}

func TestFormatProfileView_NilProfile(t *testing.T) {
	result := FormatProfileView(fullUser(), nil, true)

	assert.Equal(t, "Твой профиль не найден.\n\nСоздай профиль через кнопку \"Редактировать мой профиль\".", result)
}

func TestFormatProfileView_FullProfile(t *testing.T) {
	profile := &repositories.Profile{Bio: "Backend developer"}

	result := FormatProfileView(fullUser(), profile, false)

	expected := "🖐 <b><a href=\"tg://user?id=123456\">Ivan Petrov</a></b> (@ivanp)\n" +
		"\n<blockquote>О себе</blockquote>\nBackend developer\n"
	assert.Equal(t, expected, result)
}

func TestFormatProfileView_MinimalUserWithoutBio(t *testing.T) {
	result := FormatProfileView(minimalUser(), &repositories.Profile{}, false)

	assert.Equal(t, "🖐 <b><a href=\"tg://user?id=777\">Anna</a></b> \n", result)
}

func TestFormatProfileView_EscapesAngleBracketsInBio(t *testing.T) {
	profile := &repositories.Profile{Bio: "I write <b>Go</b> and <script>"}

	result := FormatProfileView(minimalUser(), profile, false)

	assert.Contains(t, result, "\nI write &lt;b&gt;Go&lt;/b&gt; and &lt;script&gt;\n")
	assert.NotContains(t, result, "<script>")
}

func TestFormatProfileView_Score(t *testing.T) {
	tests := []struct {
		name      string
		score     int
		showScore bool
		visible   bool
	}{
		{name: "hidden when showScore is false", score: 500, showScore: false, visible: false},
		{name: "hidden at threshold", score: 100, showScore: true, visible: false},
		{name: "hidden below threshold", score: 5, showScore: true, visible: false},
		{name: "shown above threshold", score: 101, showScore: true, visible: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := fullUser()
			user.Score = tt.score

			result := FormatProfileView(user, &repositories.Profile{Bio: "bio"}, tt.showScore)

			if tt.visible {
				assert.Contains(t, result, "\n<b>101</b> <i>(что это? хм...)</i>\n")
			} else {
				assert.NotContains(t, result, "(что это? хм...)")
			}
		})
	}
}

func TestFormatProfileManagerView_FullProfilePublished(t *testing.T) {
	cfg := &config.Config{SuperGroupChatID: 2069889012, IntroTopicID: 5}
	profile := &repositories.Profile{
		Bio:                "Likes <Go>",
		PublishedMessageID: sql.NullInt64{Int64: 999, Valid: true},
	}

	result := FormatProfileManagerView(fullUser(), profile, false, cfg)

	expected := "🖐 <b><a href=\"tg://user?id=123456\">Ivan Petrov</a></b> (@ivanp)\n" +
		"\n<i>О себе:</i><blockquote expandable>Likes &lt;Go&gt;</blockquote>" +
		"\n\n<i>Карма:</i> <b>42</b>" +
		"\n<i>Кофейные встречи:</i> ✅ Разрешено" +
		"\n<i>Telegram ID:</i> <code>123456</code>" +
		"\n<i>Ссылка на профиль:</i> https://telegram.me/c/2069889012/5/999"
	assert.Equal(t, expected, result)
}

func TestFormatProfileManagerView_CoffeeBanAndUnpublished(t *testing.T) {
	cfg := &config.Config{SuperGroupChatID: 1, IntroTopicID: 2}

	result := FormatProfileManagerView(minimalUser(), &repositories.Profile{}, true, cfg)

	expected := "🖐 <b><a href=\"tg://user?id=777\">Anna</a></b> \n" +
		"\n\n<i>Карма:</i> <b>0</b>" +
		"\n<i>Кофейные встречи:</i> ❌ Запрещено" +
		"\n<i>Telegram ID:</i> <code>777</code>"
	assert.Equal(t, expected, result)
	assert.NotContains(t, result, "Ссылка на профиль")
	assert.NotContains(t, result, "О себе")
}

func TestFormatPublicProfileForMessage_WithBio(t *testing.T) {
	profile := &repositories.Profile{Bio: "Hello <world>"}

	result := FormatPublicProfileForMessage(fullUser(), profile, false)

	expected := "🖐 <b><a href=\"tg://user?id=123456\">Ivan Petrov</a></b> (@ivanp)\n" +
		"\n<blockquote>О себе</blockquote>\nHello &lt;world&gt;\n"
	assert.Equal(t, expected, result)
}

func TestFormatPublicProfileForMessage_WithoutBio(t *testing.T) {
	result := FormatPublicProfileForMessage(minimalUser(), &repositories.Profile{}, false)

	assert.Equal(t, "🖐 <b><a href=\"tg://user?id=777\">Anna</a></b> \n", result)
}

func TestFormatPublicProfileForMessage_NeverShowsScore(t *testing.T) {
	user := fullUser()
	user.Score = 1000

	result := FormatPublicProfileForMessage(user, &repositories.Profile{Bio: "bio"}, true)

	assert.NotContains(t, result, "1000")
	assert.NotContains(t, result, "(что это? хм...)")
}
