package utils

import (
	"evo-bot-go/internal/config"
	"fmt"
)

func GetIntroMessageLink(config *config.Config, introMessageID int64) string {
	return fmt.Sprintf("https://t.me/c/%d/%d/%d", config.SuperGroupChatID, config.IntroTopicID, introMessageID)
}

func GetIntroTopicLink(config *config.Config) string {
	return fmt.Sprintf("https://t.me/c/%d/%d", config.SuperGroupChatID, config.IntroTopicID)
}
