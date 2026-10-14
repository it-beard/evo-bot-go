package utils

// JoinBlocksIntoChunks joins consecutive blocks into chunks of at most limit UTF-16 code units, so a long
// message can be sent as several Telegram messages without cutting a block (and its markup) in half.
// A single block longer than limit becomes a chunk of its own.
func JoinBlocksIntoChunks(blocks []string, limit int) []string {
	var chunks []string
	current := ""
	currentLen := 0

	for _, block := range blocks {
		blockLen := Utf16CodeUnitCount(block)
		if current != "" && currentLen+blockLen > limit {
			chunks = append(chunks, current)
			current, currentLen = "", 0
		}
		current += block
		currentLen += blockLen
	}

	if current != "" {
		chunks = append(chunks, current)
	}
	return chunks
}
