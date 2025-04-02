package utils

import (
	"fmt"
	"strings"

	"evo-bot-go/internal/database/repositories"
)

// FormatContentList formats a slice of contents for display to users
// It returns a markdown-formatted string with content information
func FormatContentList(contents []repositories.Content, title string, cancelCommand string, actionDescription string) string {
	var response strings.Builder
	response.WriteString(fmt.Sprintf("*%s*\n", title))

	for _, content := range contents {
		// Handle optional started_at field
		startedAtStr := "не указано"
		if content.StartedAt != nil && !content.StartedAt.IsZero() {
			startedAtStr = content.StartedAt.Format("02.01.2006 в 15:04")
		}

		// Emoji based on content status
		statusEmoji := "🔄"
		if content.Status == "finished" {
			statusEmoji = "✅"
		} else if content.Status == "actual" {
			statusEmoji = "🔄"
		}

		response.WriteString(fmt.Sprintf("\nID `%d`: *%s*\n", content.ID, content.Name))
		response.WriteString(fmt.Sprintf("└ %s, тип: _%s_, старт: _%s_\n",
			statusEmoji, content.Type, startedAtStr))
	}

	response.WriteString(fmt.Sprintf("\nПожалуйста, отправь ID контента, %s, или /%s для отмены.",
		actionDescription, cancelCommand))

	return response.String()
}
