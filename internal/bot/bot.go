package bot

import (
	"log"
	"time"

	"github.com/it-beard/evo-bot-go/internal/clients"
	"github.com/it-beard/evo-bot-go/internal/config"
	"github.com/it-beard/evo-bot-go/internal/database"
	"github.com/it-beard/evo-bot-go/internal/database/storages"
	"github.com/it-beard/evo-bot-go/internal/handlers/privatehandlers"
	"github.com/it-beard/evo-bot-go/internal/handlers/publichandlers"
	"github.com/it-beard/evo-bot-go/internal/scheduler"
	"github.com/it-beard/evo-bot-go/internal/services"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type TgBotClient struct {
	bot        *gotgbot.Bot
	dispatcher *ext.Dispatcher
	updater    *ext.Updater
	db         *database.DB
	scheduler  *scheduler.DailyScheduler
}

func NewTgBotClient(openaiClient *clients.OpenAiClient, appConfig *config.Config) (*TgBotClient, error) {
	b, err := gotgbot.NewBot(appConfig.BotToken, nil)
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

	// Initialize database
	db, err := database.NewDB(appConfig.DBConnection)
	if err != nil {
		return nil, err
	}

	// Initialize schema
	if err := db.InitSchema(); err != nil {
		return nil, err
	}

	// Create message store
	messageStore := storages.NewMessageStore(db)

	// Create message sender
	messageSender := services.NewMessageSender(b)

	// Create summarization service
	summarizationService := services.NewSummarizationService(
		appConfig,
		messageStore,
		openaiClient,
		messageSender,
	)

	// Create daily scheduler
	dailyScheduler := scheduler.NewDailyScheduler(appConfig, summarizationService)

	bot := &TgBotClient{
		bot:        b,
		dispatcher: dispatcher,
		updater:    updater,
		db:         db,
		scheduler:  dailyScheduler,
	}

	bot.registerHandlers(openaiClient, appConfig, messageStore, summarizationService)

	return bot, nil
}

func (b *TgBotClient) registerHandlers(openaiClient *clients.OpenAiClient, appConfig *config.Config, messageStore *storages.MessageStore, summarizationService *services.SummarizationService) {
	messageSender := services.NewMessageSender(b.bot)

	// Private handlers
	b.dispatcher.AddHandler(privatehandlers.NewStartHandler())
	b.dispatcher.AddHandler(privatehandlers.NewHelpHandler())
	b.dispatcher.AddHandler(privatehandlers.NewToolHandler(openaiClient, appConfig))
	b.dispatcher.AddHandler(privatehandlers.NewContentHandler(openaiClient, appConfig))
	b.dispatcher.AddHandler(privatehandlers.NewCodeHandler(appConfig))
	b.dispatcher.AddHandler(privatehandlers.NewSummarizeHandler(summarizationService, appConfig))

	// Public handlers
	b.dispatcher.AddHandler(publichandlers.NewDeleteJoinLeftMessagesHandler())
	b.dispatcher.AddHandler(publichandlers.NewSaveHandler(messageSender, appConfig))
	b.dispatcher.AddHandler(publichandlers.NewRepliesFromClosedThreadsHandler(messageSender, appConfig))
	b.dispatcher.AddHandler(publichandlers.NewCleanClosedThreadsHandler(messageSender, appConfig))
	b.dispatcher.AddHandler(publichandlers.NewMessageCollectorHandler(messageStore, appConfig))
}

func (b *TgBotClient) Start() {
	// Start the daily scheduler
	b.scheduler.Start()
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

// Close closes the bot client
func (b *TgBotClient) Close() error {
	// Stop the scheduler
	b.scheduler.Stop()

	// Close the database connection
	if err := b.db.Close(); err != nil {
		return err
	}

	return nil
}
