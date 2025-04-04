package utils

import (
	"evo-bot-go/internal/clients"
	"evo-bot-go/internal/config"
	"fmt"
	"log"
)

func ChatIdToFullChatId(chatId int64) int64 {
	return -1000000000000 - chatId
}

// GetTopicName retrieves the topic name from the topic ID using the Telegram API
func GetTopicName(topicId int) (string, error) {
	// hack for 0 topic (1 in links)
	if topicId == 0 {
		return "Оффтопчик", nil
	}
	// Convert topicId to int since GetChatMessageById expects an int
	topicIdInt := int(topicId)

	// Load configuration
	appConfig, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	chatId := appConfig.SuperGroupChatID

	// Get the topic message by ID
	message, err := clients.TgGetChatMessageById(chatId, topicIdInt)
	if err != nil {
		return "Topic", fmt.Errorf("failed to get thread message: %w", err)
	}

	// Extract and truncate the topic name if needed
	topicName := message.Message
	if topicName == "" {
		topicName = "Topic"
	}

	return topicName, nil
}
