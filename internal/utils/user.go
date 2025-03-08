package utils

import (
	"log"
	"strconv"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/it-beard/evo-bot-go/internal/config"
)

func IsUserClubMember(b *gotgbot.Bot, msg *gotgbot.Message, config *config.Config) bool {
	chatId, err := strconv.ParseInt("-100"+strconv.FormatInt(config.SuperGroupChatID, 10), 10, 64)
	if err != nil {
		log.Printf("Failed to parse chat ID: %v", err)
		return false
	}
	// Check if user is member of target group
	chatMember, err := b.GetChatMember(chatId, msg.From.Id, nil)
	if err != nil {
		log.Printf("Failed to get chat member: %v", err)
		return false
	}

	status := chatMember.GetStatus()
	if status == "left" || status == "kicked" {
		return false
	}

	return true
}
