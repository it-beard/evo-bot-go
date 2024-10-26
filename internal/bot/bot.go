package bot

import (
	"log"
	"time"

	"your_module_name/internal/clients"
	"your_module_name/internal/handlers/privatehandlers"
	"your_module_name/internal/handlers/publichandlers"
	"your_module_name/internal/services"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type TgBotClient struct {
	bot        *gotgbot.Bot
	dispatcher *ext.Dispatcher
	updater    *ext.Updater
}

func NewTgBotClient(token string, openaiClient *clients.OpenAiClient) (*TgBotClient, error) {
	b, err := gotgbot.NewBot(token, nil)
	if err != nil {
		return nil, err
	}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
			log.Println(err.Error())
			return ext.DispatcherActionNoop
		},
		MaxRoutines: ext.DefaultMaxRoutines,
	})

	updater := ext.NewUpdater(dispatcher, nil)

	bot := &TgBotClient{
		bot:        b,
		dispatcher: dispatcher,
		updater:    updater,
	}

	bot.registerHandlers(openaiClient)

	return bot, nil
}

func (b *TgBotClient) registerHandlers(openaiClient *clients.OpenAiClient) {
	messageSender := services.NewMessageSender(b.bot)

	// Private handlers
	b.dispatcher.AddHandler(privatehandlers.NewStartHandler())
	b.dispatcher.AddHandler(privatehandlers.NewHelpHandler())
	b.dispatcher.AddHandler(privatehandlers.NewToolHandler(openaiClient))
	b.dispatcher.AddHandler(privatehandlers.NewCodeHandler())

	// Public handlers
	b.dispatcher.AddHandler(publichandlers.NewDeleteJoinLeftMessagesHandler())
	b.dispatcher.AddHandler(publichandlers.NewSaveHandler(messageSender))
	b.dispatcher.AddHandler(publichandlers.NewRepliesFromClosedThreadsHandler(messageSender))
	b.dispatcher.AddHandler(publichandlers.NewCleanClosedThreadsHandler(messageSender))
}

func (b *TgBotClient) Start() {
	err := b.updater.StartPolling(b.bot, &ext.PollingOpts{
		DropPendingUpdates: true,
		GetUpdatesOpts: &gotgbot.GetUpdatesOpts{
			Timeout: 9,
			RequestOpts: &gotgbot.RequestOpts{
				Timeout: time.Second * 10,
			},
		},
	})
	if err != nil {
		log.Fatal("failed to start polling: " + err.Error())
	}
	log.Printf("%s has been started...\n", b.bot.User.Username)

	b.updater.Idle()
}
