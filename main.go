package main

import (
	"log"
	"os"
	"time"

	"your_module_name/internal/bot"
	"your_module_name/internal/clients"
)

func keepTgUserClientSessionAlive() {
	// First time refresh
	if err := clients.TgUserClientKeepSessionAlive(); err != nil {
		log.Printf("Failed to keep session alive: %v", err)
	} else {
		log.Printf("Session refresh successful")
	}
	// Keep session alive every 30 minutes
	ticker := time.NewTicker(30 * time.Minute)
	go func() {
		for range ticker.C {
			if err := clients.TgUserClientKeepSessionAlive(); err != nil {
				log.Printf("Failed to keep session alive: %v", err)
			} else {
				log.Printf("Session refresh successful")
			}
		}
	}()
}

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

	// Start session keep-alive routine
	keepTgUserClientSessionAlive()

	bot, err := bot.NewTgBotClient(token, openaiClient)
	if err != nil {
		log.Fatal("Failed to create Telegram Bot Client: " + err.Error())
	}

	bot.Start()
}
