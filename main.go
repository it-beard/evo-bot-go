package main

import (
	"log"
	"os"

	"your_module_name/internal/bot"
)

func main() {
	token := os.Getenv("TG_EVO_BOT_TOKEN")
	if token == "" {
		log.Fatal("TOKEN environment variable is empty")
	}

	bot, err := bot.NewBot(token)
	if err != nil {
		log.Fatal("failed to create new bot: " + err.Error())
	}

	bot.Start()
}
