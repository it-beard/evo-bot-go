package utils

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
)

func TestConvertToMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		entities []gotgbot.MessageEntity
		expected string
	}{
		{
			name:     "Empty text and entities",
			text:     "",
			entities: []gotgbot.MessageEntity{},
			expected: "",
		},
		{
			name:     "Plain text without entities",
			text:     "Hello world",
			entities: []gotgbot.MessageEntity{},
			expected: "Hello world",
		},
		{
			name: "Bold text",
			text: "Hello world",
			entities: []gotgbot.MessageEntity{
				{Type: "bold", Offset: 0, Length: 5},
			},
			expected: "**Hello** world",
		},
		{
			name: "Italic text",
			text: "Hello world",
			entities: []gotgbot.MessageEntity{
				{Type: "italic", Offset: 6, Length: 5},
			},
			expected: "Hello *world*",
		},
		{
			name: "Multiple entities",
			text: "Hello world test",
			entities: []gotgbot.MessageEntity{
				{Type: "bold", Offset: 0, Length: 5},
				{Type: "italic", Offset: 6, Length: 5},
				{Type: "code", Offset: 12, Length: 4},
			},
			expected: "**Hello** *world* `test`",
		},
		{
			name: "Code block with language",
			text: "func main() {\n    fmt.Println(\"Hello\")\n}",
			entities: []gotgbot.MessageEntity{
				{Type: "pre", Offset: 0, Length: 41, Language: "go"},
			},
			expected: "```go\nfunc main() {\n    fmt.Println(\"Hello\")\n}\n```",
		},
		{
			name: "Text link",
			text: "Visit GitHub",
			entities: []gotgbot.MessageEntity{
				{Type: "text_link", Offset: 6, Length: 6, Url: "https://github.com"},
			},
			expected: "Visit [GitHub](https://github.com)",
		},
		{
			name: "Mention and hashtag (no formatting)",
			text: "@username #hashtag",
			entities: []gotgbot.MessageEntity{
				{Type: "mention", Offset: 0, Length: 9},
				{Type: "hashtag", Offset: 10, Length: 8},
			},
			expected: "@username #hashtag",
		},
		{
			name: "Strikethrough and underline",
			text: "Hello world",
			entities: []gotgbot.MessageEntity{
				{Type: "strikethrough", Offset: 0, Length: 5},
				{Type: "underline", Offset: 6, Length: 5},
			},
			expected: "~~Hello~~ __world__",
		},
		{
			name: "Spoiler text",
			text: "This is a spoiler",
			entities: []gotgbot.MessageEntity{
				{Type: "spoiler", Offset: 10, Length: 7},
			},
			expected: "This is a ||spoiler||",
		},
		{
			name: "Blockquote",
			text: "This is quoted text\nSecond line",
			entities: []gotgbot.MessageEntity{
				{Type: "blockquote", Offset: 0, Length: 31},
			},
			expected: "> This is quoted text\n> Second line",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToMarkdown(tt.text, tt.entities)
			assert.Equal(t, tt.expected, result, "Markdown conversion should match expected value")
		})
	}
}

func TestConvertEntityToMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		entity   gotgbot.MessageEntity
		expected string
	}{
		{
			name:     "Unknown entity type",
			text:     "test",
			entity:   gotgbot.MessageEntity{Type: "unknown"},
			expected: "test",
		},
		{
			name:     "URL entity (no formatting)",
			text:     "https://example.com",
			entity:   gotgbot.MessageEntity{Type: "url"},
			expected: "https://example.com",
		},
		{
			name:     "Email entity (no formatting)",
			text:     "test@example.com",
			entity:   gotgbot.MessageEntity{Type: "email"},
			expected: "test@example.com",
		},
		{
			name:     "Bot command (no formatting)",
			text:     "/start",
			entity:   gotgbot.MessageEntity{Type: "bot_command"},
			expected: "/start",
		},
		{
			name:     "Text mention with user",
			text:     "John Doe",
			entity:   gotgbot.MessageEntity{Type: "text_mention", User: &gotgbot.User{Id: 123456}},
			expected: "[John Doe](tg://user?id=123456)",
		},
		{
			name:     "Text mention without user",
			text:     "John Doe",
			entity:   gotgbot.MessageEntity{Type: "text_mention"},
			expected: "John Doe",
		},
		{
			name:     "Code block without language",
			text:     "console.log('test')",
			entity:   gotgbot.MessageEntity{Type: "pre"},
			expected: "```\nconsole.log('test')\n```",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertEntityToMarkdown(tt.text, tt.entity)
			assert.Equal(t, tt.expected, result, "Entity conversion should match expected value")
		})
	}
}
