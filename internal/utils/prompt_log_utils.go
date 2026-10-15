package utils

import (
	"fmt"
	"os"
)

// PromptLogFile keeps the last prompt sent to OpenAI, for debugging
const PromptLogFile = "last-prompt-log.txt"

// WritePromptLog saves the prompt into PromptLogFile. Prompts contain members' names, bios and chat
// messages, so the file is readable by the bot's own OS user only.
func WritePromptLog(prompt string) error {
	// Replace the file instead of rewriting it: an old log may be world-readable, and O_EXCL never
	// follows a symlink planted under the log's name
	if err := os.Remove(PromptLogFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to replace prompt log: %w", err)
	}

	file, err := os.OpenFile(PromptLogFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("failed to create prompt log: %w", err)
	}

	if _, err := file.WriteString(prompt); err != nil {
		_ = file.Close()
		return fmt.Errorf("failed to write prompt log: %w", err)
	}
	return file.Close()
}
