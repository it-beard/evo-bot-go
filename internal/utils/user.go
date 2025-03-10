package utils

import (
	"log"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"evo-bot-go/internal/config"
)

func IsUserClubMember(b *gotgbot.Bot, msg *gotgbot.Message, config *config.Config) bool {
	chatId := ChatIdToFullChatId(config.SuperGroupChatID)
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
