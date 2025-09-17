package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIndexAny(t *testing.T) {
	tests := []struct {
		name       string
		s          string
		substr     string
		expectedIdx int
	}{
		{
			name:       "Substring found at beginning",
			s:          "hello world",
			substr:     "hello",
			expectedIdx: 0,
		},
		{
			name:       "Substring found in middle",
			s:          "hello world",
			substr:     "wo",
			expectedIdx: 6,
		},
		{
			name:       "Substring found at end",
			s:          "hello world",
			substr:     "world",
			expectedIdx: 6,
		},
		{
			name:       "Substring not found",
			s:          "hello world",
			substr:     "xyz",
			expectedIdx: -1,
		},
		{
			name:       "Empty string with non-empty substring",
			s:          "",
			substr:     "hello",
			expectedIdx: -1,
		},
		{
			name:       "Empty substring",
			s:          "hello world",
			substr:     "",
			expectedIdx: 0, // strings.Index returns 0 for empty substring
		},
		{
			name:       "Both strings empty",
			s:          "",
			substr:     "",
			expectedIdx: 0,
		},
		{
			name:       "Single character match",
			s:          "hello",
			substr:     "e",
			expectedIdx: 1,
		},
		{
			name:       "Single character no match",
			s:          "hello",
			substr:     "x",
			expectedIdx: -1,
		},
		{
			name:       "Case sensitive - no match",
			s:          "Hello World",
			substr:     "hello",
			expectedIdx: -1,
		},
		{
			name:       "Case sensitive - match",
			s:          "Hello World",
			substr:     "Hello",
			expectedIdx: 0,
		},
		{
			name:       "Multiple occurrences - returns first",
			s:          "hello hello world",
			substr:     "hello",
			expectedIdx: 0,
		},
		{
			name:       "Unicode characters",
			s:          "привет мир",
			substr:     "мир",
			expectedIdx: 13, // Position in bytes for Cyrillic
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IndexAny(tt.s, tt.substr)
			assert.Equal(t, tt.expectedIdx, result, "Index should match expected value")
		})
	}
}

func TestExtractNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Valid number in parentheses",
			input:    "FLOOD_WAIT (8)",
			expected: "8",
		},
		{
			name:     "Large number in parentheses",
			input:    "ERROR_CODE (12345)",
			expected: "12345",
		},
		{
			name:     "Zero in parentheses",
			input:    "COUNT (0)",
			expected: "0",
		},
		{
			name:     "Single digit",
			input:    "STATUS (5)",
			expected: "5",
		},
		{
			name:     "Multiple numbers - returns first match",
			input:    "FIRST (123) SECOND (456)",
			expected: "123",
		},
		{
			name:     "No parentheses",
			input:    "FLOOD_WAIT 8",
			expected: "",
		},
		{
			name:     "Empty parentheses",
			input:    "ERROR ()",
			expected: "",
		},
		{
			name:     "Non-numeric content in parentheses",
			input:    "MESSAGE (abc)",
			expected: "",
		},
		{
			name:     "Mixed content in parentheses",
			input:    "DATA (123abc)",
			expected: "",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Only parentheses",
			input:    "()",
			expected: "",
		},
		{
			name:     "Parentheses with spaces",
			input:    "ERROR ( 42 )",
			expected: "",
		},
		{
			name:     "Number without parentheses",
			input:    "ERROR 123",
			expected: "",
		},
		{
			name:     "Multiple digits",
			input:    "TIMEOUT (999999)",
			expected: "999999",
		},
		{
			name:     "Nested parentheses - outer match",
			input:    "NESTED ((42))",
			expected: "42", // The regex will find the first number in parentheses
		},
		{
			name:     "Leading zeros",
			input:    "CODE (007)",
			expected: "007",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractNumber(tt.input)
			assert.Equal(t, tt.expected, result, "Extracted number should match expected value")
		})
	}
}

func TestEscapeMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Text with underscores",
			input:    "hello_world_test",
			expected: "hello-world-test",
		},
		{
			name:     "Text without special characters",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Single underscore",
			input:    "_",
			expected: "-",
		},
		{
			name:     "Multiple underscores",
			input:    "___",
			expected: "---",
		},
		{
			name:     "Mixed text with underscores",
			input:    "This_is_a_test_string",
			expected: "This-is-a-test-string",
		},
		{
			name:     "Underscores at beginning",
			input:    "_start",
			expected: "-start",
		},
		{
			name:     "Underscores at end",
			input:    "end_",
			expected: "end-",
		},
		{
			name:     "Underscores at both ends",
			input:    "_middle_",
			expected: "-middle-",
		},
		{
			name:     "Text with other special characters (should remain unchanged)",
			input:    "hello*world&test#symbol",
			expected: "hello*world&test#symbol",
		},
		{
			name:     "Mixed underscores and other characters",
			input:    "hello_world*test_case",
			expected: "hello-world*test-case",
		},
		{
			name:     "Numbers and underscores",
			input:    "test_123_case",
			expected: "test-123-case",
		},
		{
			name:     "Unicode with underscores",
			input:    "привет_мир",
			expected: "привет-мир",
		},
		{
			name:     "Consecutive underscores",
			input:    "hello__world___test",
			expected: "hello--world---test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EscapeMarkdown(tt.input)
			assert.Equal(t, tt.expected, result, "Escaped string should match expected value")
		})
	}
}