package utils

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/stretchr/testify/assert"
)

func TestConvertToHTML(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		entities []gotgbot.MessageEntity
		expected string
	}{
		{
			name:     "Empty text with no entities",
			text:     "",
			entities: []gotgbot.MessageEntity{},
			expected: "",
		},
		{
			name:     "Plain text with no entities",
			text:     "Hello world",
			entities: []gotgbot.MessageEntity{},
			expected: "Hello world",
		},
		{
			name:     "Text with newlines",
			text:     "Hello\nworld",
			entities: []gotgbot.MessageEntity{},
			expected: "Hello<br>\nworld",
		},
		{
			name: "Single bold entity",
			text: "Hello world",
			entities: []gotgbot.MessageEntity{
				{Type: "bold", Offset: 0, Length: 5},
			},
			expected: "<strong>Hello</strong> world",
		},
		{
			name: "Multiple non-overlapping entities",
			text: "Hello world",
			entities: []gotgbot.MessageEntity{
				{Type: "bold", Offset: 0, Length: 5},
				{Type: "italic", Offset: 6, Length: 5},
			},
			expected: "<strong>Hello</strong> <em>world</em>",
		},
		{
			name: "Code entity",
			text: "Some code here",
			entities: []gotgbot.MessageEntity{
				{Type: "code", Offset: 5, Length: 4},
			},
			expected: "Some <code>code</code> here",
		},
		{
			name: "Text link entity",
			text: "Visit Google",
			entities: []gotgbot.MessageEntity{
				{Type: "text_link", Offset: 6, Length: 6, Url: "https://google.com"},
			},
			expected: "Visit <a href=\"https://google.com\">Google</a>",
		},
		{
			name: "Blockquote entity",
			text: "This is a quote",
			entities: []gotgbot.MessageEntity{
				{Type: "blockquote", Offset: 0, Length: 15},
			},
			expected: "<blockquote>This is a quote</blockquote>",
		},
		{
			name:     "HTML special characters escaped",
			text:     "Test <script>alert('xss')</script>",
			entities: []gotgbot.MessageEntity{},
			expected: "Test &lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToHTML(tt.text, tt.entities)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFilterOverlappingEntities(t *testing.T) {
	tests := []struct {
		name     string
		entities []gotgbot.MessageEntity
		expected []gotgbot.MessageEntity
	}{
		{
			name:     "Empty entities",
			entities: []gotgbot.MessageEntity{},
			expected: []gotgbot.MessageEntity{},
		},
		{
			name: "Single entity",
			entities: []gotgbot.MessageEntity{
				{Type: "bold", Offset: 0, Length: 5},
			},
			expected: []gotgbot.MessageEntity{
				{Type: "bold", Offset: 0, Length: 5},
			},
		},
		{
			name: "Overlapping entities - first one kept",
			entities: []gotgbot.MessageEntity{
				{Type: "bold", Offset: 0, Length: 8},
				{Type: "italic", Offset: 3, Length: 5},
			},
			expected: []gotgbot.MessageEntity{
				{Type: "bold", Offset: 0, Length: 8},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterOverlappingEntities(tt.entities)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUtf16OffsetToByteOffset(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		utf16Offset  int
		expectedByte int
	}{
		{
			name:         "Empty string",
			text:         "",
			utf16Offset:  0,
			expectedByte: 0,
		},
		{
			name:         "ASCII text",
			text:         "hello",
			utf16Offset:  3,
			expectedByte: 3,
		},
		{
			name:         "Unicode text - Cyrillic",
			text:         "привет",
			utf16Offset:  3,
			expectedByte: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := utf16OffsetToByteOffset(tt.text, tt.utf16Offset)
			assert.Equal(t, tt.expectedByte, result)
		})
	}
}

func TestConvertEntityToHTML(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		entity   gotgbot.MessageEntity
		expected string
	}{
		{
			name:     "Bold entity",
			text:     "bold",
			entity:   gotgbot.MessageEntity{Type: "bold"},
			expected: "<strong>bold</strong>",
		},
		{
			name:     "Code entity",
			text:     "code",
			entity:   gotgbot.MessageEntity{Type: "code"},
			expected: "<code>code</code>",
		},
		{
			name:     "URL entity",
			text:     "https://example.com",
			entity:   gotgbot.MessageEntity{Type: "url"},
			expected: "<a href=\"https://example.com\">https://example.com</a>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertEntityToHTML(tt.text, tt.entity)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEscapeAndConvertNewlines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Plain text",
			input:    "Hello world",
			expected: "Hello world",
		},
		{
			name:     "Text with HTML characters",
			input:    "<script>alert('xss')</script>",
			expected: "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;",
		},
		{
			name:     "Text with newline",
			input:    "Line 1\nLine 2",
			expected: "Line 1<br>\nLine 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeAndConvertNewlines(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestApplyBlockquoteHTML(t *testing.T) {
	tests := []struct {
		name      string
		innerHTML string
		expected  string
	}{
		{
			name:      "Empty content",
			innerHTML: "",
			expected:  "<blockquote></blockquote>",
		},
		{
			name:      "Plain text",
			innerHTML: "This is a quote",
			expected:  "<blockquote>This is a quote</blockquote>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyBlockquoteHTML(tt.innerHTML)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractNestedEntities(t *testing.T) {
	tests := []struct {
		name      string
		all       []gotgbot.MessageEntity
		container gotgbot.MessageEntity
		expected  []gotgbot.MessageEntity
	}{
		{
			name:      "Empty entities list",
			all:       []gotgbot.MessageEntity{},
			container: gotgbot.MessageEntity{Type: "blockquote", Offset: 0, Length: 10},
			expected:  nil,
		},
		{
			name: "Single nested entity",
			all: []gotgbot.MessageEntity{
				{Type: "blockquote", Offset: 0, Length: 10},
				{Type: "bold", Offset: 2, Length: 5},
			},
			container: gotgbot.MessageEntity{Type: "blockquote", Offset: 0, Length: 10},
			expected: []gotgbot.MessageEntity{
				{Type: "bold", Offset: 2, Length: 5},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractNestedEntities(tt.all, tt.container)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertEntityToHTML_AllEntityTypes(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		entity   gotgbot.MessageEntity
		expected string
	}{
		{
			name:     "Italic with newline",
			text:     "a\nb",
			entity:   gotgbot.MessageEntity{Type: "italic"},
			expected: "<em>a<br>\nb</em>",
		},
		{
			name:     "Underline",
			text:     "under",
			entity:   gotgbot.MessageEntity{Type: "underline"},
			expected: "<u>under</u>",
		},
		{
			name:     "Strikethrough",
			text:     "gone",
			entity:   gotgbot.MessageEntity{Type: "strikethrough"},
			expected: "<s>gone</s>",
		},
		{
			name:     "Code escapes HTML",
			text:     "a<b>&c",
			entity:   gotgbot.MessageEntity{Type: "code"},
			expected: "<code>a&lt;b&gt;&amp;c</code>",
		},
		{
			name:     "Pre with language keeps raw newlines",
			text:     "if a < b {\n}",
			entity:   gotgbot.MessageEntity{Type: "pre", Language: "go"},
			expected: "<pre><code class=\"language-go\">if a &lt; b {\n}</code></pre>",
		},
		{
			name:     "Pre language is escaped",
			text:     "x",
			entity:   gotgbot.MessageEntity{Type: "pre", Language: `"><script>`},
			expected: "<pre><code class=\"language-&#34;&gt;&lt;script&gt;\">x</code></pre>",
		},
		{
			name:     "Pre without language",
			text:     "line1\nline2",
			entity:   gotgbot.MessageEntity{Type: "pre"},
			expected: "<pre><code>line1\nline2</code></pre>",
		},
		{
			name:     "Text link escapes URL",
			text:     "click",
			entity:   gotgbot.MessageEntity{Type: "text_link", Url: `https://e.com/?a=1&b="2"`},
			expected: "<a href=\"https://e.com/?a=1&amp;b=&#34;2&#34;\">click</a>",
		},
		{
			name:     "Text mention with user",
			text:     "Bob",
			entity:   gotgbot.MessageEntity{Type: "text_mention", User: &gotgbot.User{Id: 42}},
			expected: "<a href=\"tg://user?id=42\">Bob</a>",
		},
		{
			name:     "Text mention without user",
			text:     "Bob <x>",
			entity:   gotgbot.MessageEntity{Type: "text_mention"},
			expected: "Bob &lt;x&gt;",
		},
		{
			name:     "Mention",
			text:     "@bob",
			entity:   gotgbot.MessageEntity{Type: "mention"},
			expected: "@bob",
		},
		{
			name:     "Hashtag",
			text:     "#go",
			entity:   gotgbot.MessageEntity{Type: "hashtag"},
			expected: "#go",
		},
		{
			name:     "Cashtag",
			text:     "$USD",
			entity:   gotgbot.MessageEntity{Type: "cashtag"},
			expected: "$USD",
		},
		{
			name:     "Bot command",
			text:     "/start",
			entity:   gotgbot.MessageEntity{Type: "bot_command"},
			expected: "/start",
		},
		{
			name:     "URL with query is escaped",
			text:     "https://x.io/?a=1&b=2",
			entity:   gotgbot.MessageEntity{Type: "url"},
			expected: "<a href=\"https://x.io/?a=1&amp;b=2\">https://x.io/?a=1&amp;b=2</a>",
		},
		{
			name:     "Email",
			text:     "me@example.com",
			entity:   gotgbot.MessageEntity{Type: "email"},
			expected: "<a href=\"mailto:me@example.com\">me@example.com</a>",
		},
		{
			name:     "Phone number",
			text:     "+1234567",
			entity:   gotgbot.MessageEntity{Type: "phone_number"},
			expected: "<a href=\"tel:+1234567\">+1234567</a>",
		},
		{
			name:     "Spoiler",
			text:     "secret",
			entity:   gotgbot.MessageEntity{Type: "spoiler"},
			expected: "<span class=\"spoiler\">secret</span>",
		},
		{
			name:     "Blockquote",
			text:     "quote\nline",
			entity:   gotgbot.MessageEntity{Type: "blockquote"},
			expected: "<blockquote>quote<br>\nline</blockquote>",
		},
		{
			name:     "Unknown type is escaped plain text",
			text:     "<custom>",
			entity:   gotgbot.MessageEntity{Type: "custom_emoji"},
			expected: "&lt;custom&gt;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, convertEntityToHTML(tt.text, tt.entity))
		})
	}
}

func TestConvertToHTML_SameOffsetPrefersLongerEntity(t *testing.T) {
	entities := []gotgbot.MessageEntity{
		{Type: "bold", Offset: 0, Length: 5},
		{Type: "italic", Offset: 0, Length: 11},
	}

	result := ConvertToHTML("Hello world", entities)

	assert.Equal(t, "<em>Hello world</em>", result)
}

func TestConvertToHTML_DoesNotMutateInputEntities(t *testing.T) {
	entities := []gotgbot.MessageEntity{
		{Type: "italic", Offset: 6, Length: 5},
		{Type: "bold", Offset: 0, Length: 5},
	}
	original := append([]gotgbot.MessageEntity(nil), entities...)

	result := ConvertToHTML("Hello world", entities)

	assert.Equal(t, "<strong>Hello</strong> <em>world</em>", result)
	assert.Equal(t, original, entities)
}

func TestConvertToHTML_UTF16OffsetsWithEmoji(t *testing.T) {
	// "👍" occupies two UTF-16 code units, so "bold" starts at UTF-16 offset 3
	result := ConvertToHTML("👍 bold!", []gotgbot.MessageEntity{{Type: "bold", Offset: 3, Length: 4}})

	assert.Equal(t, "👍 <strong>bold</strong>!", result)
}

func TestConvertToHTML_MixedEntitiesAndEscaping(t *testing.T) {
	text := "Hi @bob, see https://x.io & <tag>"
	entities := []gotgbot.MessageEntity{
		{Type: "mention", Offset: 3, Length: 4},
		{Type: "url", Offset: 13, Length: 12},
	}

	result := ConvertToHTML(text, entities)

	assert.Equal(t, "Hi @bob, see <a href=\"https://x.io\">https://x.io</a> &amp; &lt;tag&gt;", result)
}

func TestEscapeAndConvertNewlines_TrailingAndConsecutiveNewlines(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{input: "a\n", expected: "a<br>\n"},
		{input: "a\n\nb", expected: "a<br>\n<br>\nb"},
		{input: "\n", expected: "<br>\n"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, escapeAndConvertNewlines(tt.input), "input %q", tt.input)
	}
}
