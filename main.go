package main

import (
	"log"
	"os"

	"your_module_name/internal/bot"
	"your_module_name/internal/clients"
)

func main() {
	token := os.Getenv("TG_EVO_BOT_TOKEN")
	if token == "" {
		log.Fatal("TOKEN environment variable is empty")
	}

	// Initialize OpenAI client
	openaiClient, err := clients.NewOpenAiClient()
	if err != nil {
		log.Fatalf("Failed to create OpenAI client: %v", err)
	}

	bot, err := bot.NewTgBotClient(token, openaiClient)
	if err != nil {
		log.Fatal("Failed to create Telegram Bot Client: " + err.Error())
	}

	bot.Start()
}
