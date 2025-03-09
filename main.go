package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/it-beard/evo-bot-go/internal/bot"
	"github.com/it-beard/evo-bot-go/internal/clients"
	"github.com/it-beard/evo-bot-go/internal/config"
)



func main() {
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


	// Create and start the bot
	botClient, err := bot.NewTgBotClient(openaiClient, appConfig)
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
