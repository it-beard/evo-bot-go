package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUtf16CodeUnitCount(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: 0,
		},
		{
			name:     "ASCII characters only",
			input:    "hello",
			expected: 5,
		},
		{
			name:     "Single ASCII character",
			input:    "a",
			expected: 1,
		},
		{
			name:     "ASCII with spaces",
			input:    "hello world",
			expected: 11,
		},
		{
			name:     "Basic Latin Extended characters",
			input:    "café",
			expected: 4, // c, a, f, é (1 code unit each)
		},
		{
			name:     "Cyrillic characters",
			input:    "привет",
			expected: 6, // Each Cyrillic character is 1 UTF-16 code unit
		},
		{
			name:     "Mixed ASCII and Unicode",
			input:    "hello мир",
			expected: 9, // h,e,l,l,o, ,м,и,р = 9 units
		},
		{
			name:     "Chinese characters",
			input:    "你好",
			expected: 2, // Each Chinese character is 1 UTF-16 code unit
		},
		{
			name:     "Emoji (surrogate pair)",
			input:    "😀",
			expected: 2, // Emoji requires 2 UTF-16 code units
		},
		{
			name:     "Multiple emojis",
			input:    "😀😃",
			expected: 4, // 2 emojis × 2 code units each = 4
		},
		{
			name:     "Mixed text with emoji",
			input:    "Hello 😀",
			expected: 8, // H,e,l,l,o, ,😀(2 units) = 8
		},
		{
			name:     "High Unicode character (beyond BMP)",
			input:    "𝔘𝔫𝔦𝔠𝔬𝔡𝔢", // Mathematical script characters
			expected: 14, // Each character is 2 UTF-16 code units, 7 chars × 2 = 14
		},
		{
			name:     "Numbers",
			input:    "12345",
			expected: 5,
		},
		{
			name:     "Special symbols",
			input:    "!@#$%",
			expected: 5,
		},
		{
			name:     "Complex emoji with modifiers",
			input:    "👨‍💻", // Man technologist (complex emoji sequence)
			expected: 5, // This is a ZWJ sequence: 👨 + ZWJ + 💻 = multiple code units
		},
		{
			name:     "Musical symbols (high Unicode)",
			input:    "𝄞", // Treble clef
			expected: 2, // Beyond BMP, requires 2 code units
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Utf16CodeUnitCount(tt.input)
			assert.Equal(t, tt.expected, result, "UTF-16 code unit count should match expected value")
		})
	}
}

func TestCutStringByUTF16Units(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		limit    int
		expected string
	}{
		{
			name:     "Empty string with any limit",
			input:    "",
			limit:    10,
			expected: "",
		},
		{
			name:     "Limit zero",
			input:    "hello",
			limit:    0,
			expected: "",
		},
		{
			name:     "Negative limit",
			input:    "hello",
			limit:    -5,
			expected: "",
		},
		{
			name:     "ASCII string within limit",
			input:    "hello",
			limit:    10,
			expected: "hello",
		},
		{
			name:     "ASCII string exactly at limit",
			input:    "hello",
			limit:    5,
			expected: "hello",
		},
		{
			name:     "ASCII string cut at limit",
			input:    "hello world",
			limit:    5,
			expected: "hello",
		},
		{
			name:     "Cut at single character",
			input:    "hello",
			limit:    1,
			expected: "h",
		},
		{
			name:     "Unicode characters within limit",
			input:    "привет",
			limit:    10,
			expected: "привет",
		},
		{
			name:     "Unicode characters cut at limit",
			input:    "привет мир",
			limit:    6,
			expected: "привет",
		},
		{
			name:     "Cut at Unicode boundary",
			input:    "hello мир",
			limit:    7,
			expected: "hello м",
		},
		{
			name:     "Emoji cutting - don't break surrogate pair",
			input:    "😀😃😄",
			limit:    3,
			expected: "😀", // Should stop at 2 units, not break the second emoji
		},
		{
			name:     "Emoji exactly at limit",
			input:    "😀😃",
			limit:    4,
			expected: "😀😃",
		},
		{
			name:     "Mixed ASCII and emoji cutting",
			input:    "Hi😀test",
			limit:    4,
			expected: "Hi😀", // H(1) + i(1) + 😀(2) = 4 units
		},
		{
			name:     "High Unicode characters cutting",
			input:    "𝔘𝔫𝔦𝔠𝔬",
			limit:    5,
			expected: "𝔘𝔫", // First char(2) + second char(2) = 4 units, can't fit third
		},
		{
			name:     "Complex case with mixed characters",
			input:    "a😀b𝔘c",
			limit:    6,
			expected: "a😀b𝔘", // a(1) + 😀(2) + b(1) + 𝔘(2) = 6 units
		},
		{
			name:     "Limit exceeds string length",
			input:    "test",
			limit:    100,
			expected: "test",
		},
		{
			name:     "Chinese characters cutting",
			input:    "你好世界",
			limit:    3,
			expected: "你好世",
		},
		{
			name:     "Cut at exact emoji boundary",
			input:    "text😀more",
			limit:    6,
			expected: "text😀", // t,e,x,t,😀(2) = 6 units exactly
		},
		{
			name:     "Single high Unicode character",
			input:    "𝄞",
			limit:    1,
			expected: "", // Can't fit 2-unit character in 1-unit limit
		},
		{
			name:     "Single high Unicode character with exact limit",
			input:    "𝄞",
			limit:    2,
			expected: "𝄞",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CutStringByUTF16Units(tt.input, tt.limit)
			assert.Equal(t, tt.expected, result, "Cut string should match expected value")
			
			// Verify that result doesn't exceed the limit
			if tt.limit >= 0 {
				resultUnits := Utf16CodeUnitCount(result)
				assert.LessOrEqual(t, resultUnits, tt.limit, "Result should not exceed UTF-16 unit limit")
			}
		})
	}
}