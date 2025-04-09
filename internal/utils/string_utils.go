package utils

import (
	"regexp"
	"strings"
)

// IndexAny finds the first index of the specified substring in a string
func IndexAny(s, substr string) int {
	return strings.Index(s, substr)
}

// ExtractNumber extracts the first number from a string
// Example: from "FLOOD_WAIT (8)" it extracts "8"
func ExtractNumber(s string) string {
	re := regexp.MustCompile(`\((\d+)\)`)
	matches := re.FindStringSubmatch(s)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}
