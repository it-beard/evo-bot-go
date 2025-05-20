package utils

// utf16CodeUnitCount returns the number of UTF-16 code units in a string
func Utf16CodeUnitCount(s string) int {
	count := 0
	for _, r := range s {
		if r <= 0xFFFF {
			count += 1
		} else {
			count += 2
		}
	}
	return count
}

// CutStringByUTF16Units cuts the string s so that its length in UTF-16 code units is at most limit.
// It returns the prefix of s that satisfies this condition.
func CutStringByUTF16Units(s string, limit int) string {
	var cuCount int   // Cumulative UTF-16 code units
	var byteIndex int // Byte index in the string
	for i, r := range s {
		// Determine the number of UTF-16 code units for this rune
		cuLen := 0
		if r <= 0xFFFF {
			cuLen = 1
		} else {
			cuLen = 2
		}

		// Check if adding this rune exceeds the limit
		if cuCount+cuLen > limit {
			break
		}

		cuCount += cuLen
		byteIndex = i + len(string(r))
	}

	return s[:byteIndex]
}
