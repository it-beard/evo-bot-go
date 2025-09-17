package utils

import (
	"fmt"
	"html"
	"sort"
	"strings"
	"unicode/utf16"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// ConvertToHTML converts Telegram message text and entities to HTML
func ConvertToHTML(text string, entities []gotgbot.MessageEntity) string {
	if len(entities) == 0 {
		return escapeAndConvertNewlines(text)
	}

	sortedEntities := make([]gotgbot.MessageEntity, len(entities))
	copy(sortedEntities, entities)
	sort.Slice(sortedEntities, func(i, j int) bool {
		if sortedEntities[i].Offset == sortedEntities[j].Offset {
			return sortedEntities[i].Length > sortedEntities[j].Length
		}
		return sortedEntities[i].Offset < sortedEntities[j].Offset
	})

	nonOverlappingEntities := filterOverlappingEntities(sortedEntities)

	result := strings.Builder{}
	lastOffset := 0

	for _, entity := range nonOverlappingEntities {
		byteOffset := utf16OffsetToByteOffset(text, int(entity.Offset))
		byteEnd := utf16OffsetToByteOffset(text, int(entity.Offset+entity.Length))

		if byteOffset > lastOffset {
			result.WriteString(escapeAndConvertNewlines(text[lastOffset:byteOffset]))
		}

		entityText := text[byteOffset:byteEnd]

		if entity.Type == "blockquote" {
			nested := extractNestedEntities(entities, entity)
			inner := ConvertToHTML(entityText, nested)
			result.WriteString(applyBlockquoteHTML(inner))
		} else {
			htmlText := convertEntityToHTML(entityText, entity)
			result.WriteString(htmlText)
		}

		lastOffset = byteEnd
	}

	if lastOffset < len(text) {
		result.WriteString(escapeAndConvertNewlines(text[lastOffset:]))
	}

	return result.String()
}

// filterOverlappingEntities removes overlapping entities to prevent content duplication
func filterOverlappingEntities(entities []gotgbot.MessageEntity) []gotgbot.MessageEntity {
	if len(entities) <= 1 {
		return entities
	}
	var result []gotgbot.MessageEntity
	lastEnd := int64(-1)
	for _, entity := range entities {
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
	partialUtf16 := utf16Text[:utf16Offset]
	partialRunes := utf16.Decode(partialUtf16)
	partialText := string(partialRunes)
	return len(partialText)
}

// convertEntityToHTML converts a single entity to its HTML representation
func convertEntityToHTML(text string, entity gotgbot.MessageEntity) string {
	switch entity.Type {
	case "bold":
		return "<strong>" + escapeAndConvertNewlines(text) + "</strong>"
	case "italic":
		return "<em>" + escapeAndConvertNewlines(text) + "</em>"
	case "underline":
		return "<u>" + escapeAndConvertNewlines(text) + "</u>"
	case "strikethrough":
		return "<s>" + escapeAndConvertNewlines(text) + "</s>"
	case "code":
		return "<code>" + html.EscapeString(text) + "</code>"
	case "pre":
		if entity.Language != "" {
			return "<pre><code class=\"language-" + html.EscapeString(entity.Language) + "\">" + html.EscapeString(text) + "</code></pre>"
		}
		return "<pre><code>" + html.EscapeString(text) + "</code></pre>"
	case "text_link":
		return "<a href=\"" + html.EscapeString(entity.Url) + "\">" + escapeAndConvertNewlines(text) + "</a>"
	case "text_mention":
		if entity.User != nil {
			return fmt.Sprintf("<a href=\"tg://user?id=%d\">%s</a>", entity.User.Id, escapeAndConvertNewlines(text))
		}
		return escapeAndConvertNewlines(text)
	case "mention", "hashtag", "cashtag", "bot_command":
		return escapeAndConvertNewlines(text)
	case "url":
		escaped := html.EscapeString(text)
		return "<a href=\"" + escaped + "\">" + escaped + "</a>"
	case "email":
		escaped := html.EscapeString(text)
		return "<a href=\"mailto:" + escaped + "\">" + escaped + "</a>"
	case "phone_number":
		escaped := html.EscapeString(text)
		return "<a href=\"tel:" + escaped + "\">" + escaped + "</a>"
	case "spoiler":
		return "<span class=\"spoiler\">" + escapeAndConvertNewlines(text) + "</span>"
	case "blockquote":
		return applyBlockquoteHTML(escapeAndConvertNewlines(text))
	default:
		return escapeAndConvertNewlines(text)
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

// applyBlockquoteHTML wraps the given content into a blockquote preserving inner HTML
func applyBlockquoteHTML(innerHTML string) string {
	return "<blockquote>" + innerHTML + "</blockquote>"
}

// escapeAndConvertNewlines escapes HTML special chars and converts "\n" to <br>
// This should not be used inside <pre> or other contexts where raw newlines are desired.
func escapeAndConvertNewlines(s string) string {
	if s == "" {
		return ""
	}
	escaped := html.EscapeString(s)
	parts := strings.SplitAfter(escaped, "\n")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		if strings.HasSuffix(p, "\n") {
			b.WriteString(strings.TrimSuffix(p, "\n"))
			b.WriteString("<br>\n")
		} else {
			b.WriteString(p)
		}
	}
	return b.String()
}
