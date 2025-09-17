package utils

import (
	"fmt"
	"sort"
	"strings"

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
		return sortedEntities[i].Offset < sortedEntities[j].Offset
	})

	// Convert text to runes for proper unicode handling
	runes := []rune(text)
	result := strings.Builder{}
	lastOffset := int64(0)

	for _, entity := range sortedEntities {
		// Add text before this entity
		if entity.Offset > lastOffset {
			result.WriteString(string(runes[lastOffset:entity.Offset]))
		}

		// Get the entity text
		entityEnd := entity.Offset + entity.Length
		if entityEnd > int64(len(runes)) {
			entityEnd = int64(len(runes))
		}
		entityText := string(runes[entity.Offset:entityEnd])

		// Convert entity to markdown
		markdownText := convertEntityToMarkdown(entityText, entity)
		result.WriteString(markdownText)

		lastOffset = entityEnd
	}

	// Add remaining text
	if lastOffset < int64(len(runes)) {
		result.WriteString(string(runes[lastOffset:]))
	}

	return result.String()
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
		// Convert blockquote to markdown blockquote
		lines := strings.Split(text, "\n")
		for i, line := range lines {
			lines[i] = "> " + line
		}
		return strings.Join(lines, "\n")
	default:
		// Unknown entity type, return text as-is
		return text
	}
}
