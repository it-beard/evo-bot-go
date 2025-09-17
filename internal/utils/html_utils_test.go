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
