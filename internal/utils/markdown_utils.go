package utils

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf16"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// ConvertToMarkdown converts Telegram message text and entities to markdown format
func ConvertToMarkdown(text string, entities []gotgbot.MessageEntity) string {
	if len(entities) == 0 {
		return text
	}

	// Sort entities by offset to process them in order
	sortedEntities := make([]gotgbot.MessageEntity, len(entities))
	copy(sortedEntities, entities)
	sort.Slice(sortedEntities, func(i, j int) bool {
		if sortedEntities[i].Offset == sortedEntities[j].Offset {
			// For same offset, process longer entities first
			return sortedEntities[i].Length > sortedEntities[j].Length
		}
		return sortedEntities[i].Offset < sortedEntities[j].Offset
	})

	// Remove overlapping entities to prevent duplication
	nonOverlappingEntities := filterOverlappingEntities(sortedEntities)

	// Build result by processing entities in order
	result := strings.Builder{}
	lastOffset := 0

	for _, entity := range nonOverlappingEntities {
		// Convert offsets to byte positions in UTF-8 string
		byteOffset := utf16OffsetToByteOffset(text, int(entity.Offset))
		byteEnd := utf16OffsetToByteOffset(text, int(entity.Offset+entity.Length))

		// Add text before this entity
		if byteOffset > lastOffset {
			result.WriteString(text[lastOffset:byteOffset])
		}

		// Get the entity text
		entityText := text[byteOffset:byteEnd]

		// Special handling for blockquote to keep nested entities (e.g., links)
		if entity.Type == "blockquote" {
			// Collect entities fully inside this blockquote and shift offsets
			nested := extractNestedEntities(entities, entity)
			inner := ConvertToMarkdown(entityText, nested)
			result.WriteString(applyBlockquote(inner))
		} else {
			// Convert entity to markdown
			markdownText := convertEntityToMarkdown(entityText, entity)
			result.WriteString(markdownText)
		}

		lastOffset = byteEnd
	}

	// Add remaining text
	if lastOffset < len(text) {
		result.WriteString(text[lastOffset:])
	}

	// Ensure single newlines are preserved as hard line breaks in Markdown
	return ensureHardLineBreaks(result.String())
}

// filterOverlappingEntities removes overlapping entities to prevent content duplication
func filterOverlappingEntities(entities []gotgbot.MessageEntity) []gotgbot.MessageEntity {
	if len(entities) <= 1 {
		return entities
	}

	var result []gotgbot.MessageEntity
	lastEnd := int64(-1)

	for _, entity := range entities {
		// Skip entities that overlap with the previous one
		if entity.Offset >= lastEnd {
			result = append(result, entity)
			lastEnd = entity.Offset + entity.Length
		}
	}

	return result
}

// utf16OffsetToByteOffset converts UTF-16 offset to byte offset in UTF-8 string
func utf16OffsetToByteOffset(text string, utf16Offset int) int {
	if utf16Offset <= 0 {
		return 0
	}

	runes := []rune(text)
	utf16Text := utf16.Encode(runes)

	if utf16Offset >= len(utf16Text) {
		return len(text)
	}

	// Convert the portion up to the offset to runes
	partialUtf16 := utf16Text[:utf16Offset]
	partialRunes := utf16.Decode(partialUtf16)

	// Convert runes back to UTF-8 bytes
	partialText := string(partialRunes)
	return len(partialText)
}

// convertEntityToMarkdown converts a single entity to its markdown representation
func convertEntityToMarkdown(text string, entity gotgbot.MessageEntity) string {
	switch entity.Type {
	case "bold":
		return fmt.Sprintf("**%s**", text)
	case "italic":
		return fmt.Sprintf("*%s*", text)
	case "underline":
		return fmt.Sprintf("__%s__", text)
	case "strikethrough":
		return fmt.Sprintf("~~%s~~", text)
	case "code":
		return fmt.Sprintf("`%s`", text)
	case "pre":
		if entity.Language != "" {
			return fmt.Sprintf("```%s\n%s\n```", entity.Language, text)
		}
		return fmt.Sprintf("```\n%s\n```", text)
	case "text_link":
		return fmt.Sprintf("[%s](%s)", text, entity.Url)
	case "text_mention":
		if entity.User != nil {
			return fmt.Sprintf("[%s](tg://user?id=%d)", text, entity.User.Id)
		}
		return text
	case "mention":
		return text // @username is already formatted
	case "hashtag":
		return text // #hashtag is already formatted
	case "cashtag":
		return text // $CASHTAG is already formatted
	case "bot_command":
		return text // /command is already formatted
	case "url":
		return text // URLs don't need special formatting in markdown
	case "email":
		return text // Emails don't need special formatting in markdown
	case "phone_number":
		return text // Phone numbers don't need special formatting
	case "spoiler":
		return fmt.Sprintf("||%s||", text)
	case "blockquote":
		return applyBlockquote(text)
	default:
		// Unknown entity type, return text as-is
		return text
	}
}

// extractNestedEntities returns entities fully contained in container, with offsets shifted
func extractNestedEntities(all []gotgbot.MessageEntity, container gotgbot.MessageEntity) []gotgbot.MessageEntity {
	if len(all) == 0 {
		return nil
	}

	var nested []gotgbot.MessageEntity
	cStart := container.Offset
	cEnd := container.Offset + container.Length
	for _, e := range all {
		if e == container {
			continue
		}
		eStart := e.Offset
		eEnd := e.Offset + e.Length
		if eStart >= cStart && eEnd <= cEnd {
			shifted := e
			shifted.Offset = e.Offset - cStart
			nested = append(nested, shifted)
		}
	}
	return nested
}

// applyBlockquote prefixes every line with "> "
func applyBlockquote(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = "> " + line
	}
	return strings.Join(lines, "\n")
}

// ensureHardLineBreaks converts single newlines to Markdown hard line breaks (two spaces + \n)
// while keeping code blocks (``` ... ```) intact.
func ensureHardLineBreaks(s string) string {
	if s == "" {
		return s
	}

	var b strings.Builder
	inCode := false

	// Process text line-by-line while preserving whether a newline was present
	segments := strings.SplitAfter(s, "\n")
	for _, seg := range segments {
		if seg == "" {
			continue
		}

		hasNL := strings.HasSuffix(seg, "\n")
		line := strings.TrimSuffix(seg, "\n")

		// Toggle code block state on fence lines and keep them unmodified
		if strings.HasPrefix(line, "```") {
			b.WriteString(seg)
			inCode = !inCode
			continue
		}

		if inCode || !hasNL {
			b.WriteString(seg)
			continue
		}

		if line == "" { // blank line -> paragraph break
			b.WriteString(seg)
			continue
		}

		// Add two spaces before newline to force a hard line break
		b.WriteString(line)
		b.WriteString("  \n")
	}

	return b.String()
}
