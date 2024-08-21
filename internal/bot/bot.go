package bot

import (
	"log"
	"time"

	"your_module_name/internal/handlers"
	"your_module_name/internal/services"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type Bot struct {
	bot        *gotgbot.Bot
	dispatcher *ext.Dispatcher
	updater    *ext.Updater
}

func NewBot(token string) (*Bot, error) {
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

	bot := &Bot{
		bot:        b,
		dispatcher: dispatcher,
		updater:    updater,
	}

	bot.registerHandlers()

	return bot, nil
}

func (b *Bot) registerHandlers() {
	messageSender := services.NewMessageSender(b.bot)

	b.dispatcher.AddHandler(handlers.NewStartHandler())
	b.dispatcher.AddHandler(handlers.NewHelpHandler())
	b.dispatcher.AddHandler(handlers.NewForwardHandler(messageSender))
	b.dispatcher.AddHandler(handlers.NewDeleteMessagesHandler())
}

func (b *Bot) Start() {
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
