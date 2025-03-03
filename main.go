package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"your_module_name/internal/bot"
	"your_module_name/internal/clients"
	"your_module_name/internal/config"
)

func keepTgUserClientSessionAlive() {
	// First time refresh
	if err := clients.KeepSessionAlive(); err != nil {
		log.Printf("Failed to keep session alive: %v", err)
	} else {
		log.Printf("Session refresh successful")
	}
	// Keep session alive every 30 minutes
	ticker := time.NewTicker(30 * time.Minute)
	go func() {
		for range ticker.C {
			if err := clients.KeepSessionAlive(); err != nil {
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
		log.Fatal("TG_EVO_BOT_TOKEN environment variable is empty")
	}

	// Load configuration
	appConfig, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize OpenAI client
	openaiClient, err := clients.NewOpenAiClient()
	if err != nil {
		log.Fatalf("Failed to create OpenAI client: %v", err)
	}

	// Start session keep-alive routine
	keepTgUserClientSessionAlive()

	// Create and start the bot
	botClient, err := bot.NewTgBotClient(token, openaiClient, appConfig)
	if err != nil {
		log.Fatal("Failed to create Telegram Bot Client: " + err.Error())
	}

	// Handle graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("Received shutdown signal, closing resources...")
		if err := botClient.Close(); err != nil {
			log.Printf("Error closing bot client: %v", err)
		}
		os.Exit(0)
	}()

	// Start the bot
	botClient.Start()
}
