package bot

import (
	"log"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/it-beard/evo-bot-go/internal/clients"
	"github.com/it-beard/evo-bot-go/internal/config"
	"github.com/it-beard/evo-bot-go/internal/database"
	"github.com/it-beard/evo-bot-go/internal/database/repositories"
	"github.com/it-beard/evo-bot-go/internal/handlers/privatehandlers"
	"github.com/it-beard/evo-bot-go/internal/handlers/publichandlers"
	"github.com/it-beard/evo-bot-go/internal/services"
	"github.com/it-beard/evo-bot-go/internal/tasks"
)

// TgBotClient represents a Telegram bot client with all required dependencies
type TgBotClient struct {
	bot                    *gotgbot.Bot
	dispatcher             *ext.Dispatcher
	updater                *ext.Updater
	db                     *database.DB
	dailySummarizationTask *tasks.DailySummarizationTask
	sessionKeepAliveTask   *tasks.SessionKeepAliveTask
}

// NewTgBotClient creates and initializes a new Telegram bot client
func NewTgBotClient(openaiClient *clients.OpenAiClient, appConfig *config.Config) (*TgBotClient, error) {

	// Initialize bot
	bot, err := gotgbot.NewBot(appConfig.BotToken, nil)
	if err != nil {
		return nil, err
	}

	// Setup dispatcher with error handling
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
			log.Println(err.Error())
			return ext.DispatcherActionNoop
		},
		MaxRoutines: ext.DefaultMaxRoutines,
	})

	// Initialize updater
	updater := ext.NewUpdater(dispatcher, nil)

	// Setup database
	db, err := setupDatabase(appConfig.DBConnection)
	if err != nil {
		return nil, err
	}

	// Initialize services and repositories
	messageRepo := repositories.NewMessageRepository(db)
	messageSenderService := services.NewMessageSenderService(bot)
	summarizationService := services.NewSummarizationService(
		appConfig, messageRepo, openaiClient, messageSenderService,
	)

	// Initialize scheduled tasks
	dailySummarization := tasks.NewDailySummarizationTask(appConfig, summarizationService)
	sessionKeepAlive := tasks.NewSessionKeepAliveTask(30 * time.Minute)

	// Create bot client
	client := &TgBotClient{
		bot:                    bot,
		dispatcher:             dispatcher,
		updater:                updater,
		db:                     db,
		dailySummarizationTask: dailySummarization,
		sessionKeepAliveTask:   sessionKeepAlive,
	}

	// Register all handlers
	client.registerHandlers(openaiClient, appConfig, messageRepo, summarizationService, messageSenderService)

	return client, nil
}

// setupDatabase initializes the database connection and schema
func setupDatabase(connectionString string) (*database.DB, error) {
	db, err := database.NewDB(connectionString)
	if err != nil {
		return nil, err
	}

	if err := db.InitSchema(); err != nil {
		return nil, err
	}

	return db, nil
}

// registerHandlers registers all bot handlers
func (b *TgBotClient) registerHandlers(
	openaiClient *clients.OpenAiClient,
	appConfig *config.Config,
	messageRepository *repositories.MessageRepository,
	summarizationService *services.SummarizationService,
	messageSenderService services.MessageSenderService,
) {
	// Register private chat handlers
	privateHandlers := []ext.Handler{
		privatehandlers.NewStartHandler(),
		privatehandlers.NewHelpHandler(),
		privatehandlers.NewToolHandler(openaiClient, messageSenderService, appConfig),
		privatehandlers.NewContentHandler(openaiClient, messageSenderService, appConfig),
		privatehandlers.NewCodeHandler(appConfig),
		privatehandlers.NewSummarizeHandler(summarizationService, messageSenderService, appConfig),
	}
	for _, handler := range privateHandlers {
		b.dispatcher.AddHandler(handler)
	}

	// Register public chat handlers
	publicHandlers := []ext.Handler{
		publichandlers.NewDeleteJoinLeftMessagesHandler(),
		publichandlers.NewSaveHandler(messageSenderService, appConfig),
		publichandlers.NewRepliesFromClosedThreadsHandler(messageSenderService, appConfig),
		publichandlers.NewCleanClosedThreadsHandler(messageSenderService, appConfig),
		publichandlers.NewMessageCollectorHandler(messageRepository, appConfig),
	}
	for _, handler := range publicHandlers {
		b.dispatcher.AddHandler(handler)
	}
}

// Start begins the bot polling and starts scheduled tasks
func (b *TgBotClient) Start() {
	// Start scheduled tasks
	b.dailySummarizationTask.Start()
	b.sessionKeepAliveTask.Start()

	// Configure and start polling
	pollingOpts := &ext.PollingOpts{
		DropPendingUpdates: true,
		GetUpdatesOpts: &gotgbot.GetUpdatesOpts{
			Timeout: 9,
			RequestOpts: &gotgbot.RequestOpts{
				Timeout: time.Second * 10,
			},
		},
	}

	if err := b.updater.StartPolling(b.bot, pollingOpts); err != nil {
		log.Fatal("Failed to start polling: " + err.Error())
	}

	log.Printf("Bot @%s has been started successfully\n", b.bot.User.Username)
	b.updater.Idle()
}

// Close gracefully shuts down the bot and all its resources
func (b *TgBotClient) Close() error {
	// Stop scheduled tasks
	b.dailySummarizationTask.Stop()
	b.sessionKeepAliveTask.Stop()

	// Close database connection
	return b.db.Close()
}
