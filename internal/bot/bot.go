package bot

import (
	"log"
	"time"

	"evo-bot-go/internal/clients"
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/database"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/handlers"
	"evo-bot-go/internal/handlers/adminhandlers"
	"evo-bot-go/internal/handlers/adminhandlers/eventhandlers"
	"evo-bot-go/internal/handlers/grouphandlers"
	"evo-bot-go/internal/handlers/privatehandlers"
	"evo-bot-go/internal/handlers/privatehandlers/topicshandlers"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/tasks"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// HandlerDependencies contains all dependencies needed by handlers
type HandlerDependencies struct {
	OpenAiClient                *clients.OpenAiClient
	AppConfig                   *config.Config
	SummarizationService        *services.SummarizationService
	MessageSenderService        services.MessageSenderService
	EventRepository             *repositories.EventRepository
	TopicRepository             *repositories.TopicRepository
	PromptingTemplateRepository *repositories.PromptingTemplateRepository
}

// TgBotClient represents a Telegram bot client with all required dependencies
type TgBotClient struct {
	bot        *gotgbot.Bot
	dispatcher *ext.Dispatcher
	updater    *ext.Updater
	db         *database.DB
	tasks      []tasks.Task
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

	// Initialize repositories
	eventRepository := repositories.NewEventRepository(db.DB)
	topicRepository := repositories.NewTopicRepository(db.DB)
	promptingTemplateRepository := repositories.NewPromptingTemplateRepository(db.DB)
	// Initialize services
	messageSenderService := services.NewMessageSenderService(bot)
	summarizationService := services.NewSummarizationService(
		appConfig, openaiClient, messageSenderService, promptingTemplateRepository,
	)

	// Initialize scheduled tasks
	scheduledTasks := []tasks.Task{
		tasks.NewSessionKeepAliveTask(30 * time.Minute),
		tasks.NewDailySummarizationTask(appConfig, summarizationService),
	}

	// Create bot client
	client := &TgBotClient{
		bot:        bot,
		dispatcher: dispatcher,
		updater:    updater,
		db:         db,
		tasks:      scheduledTasks,
	}

	// Create dependencies container
	deps := &HandlerDependencies{
		OpenAiClient:                openaiClient,
		AppConfig:                   appConfig,
		SummarizationService:        summarizationService,
		MessageSenderService:        messageSenderService,
		EventRepository:             eventRepository,
		TopicRepository:             topicRepository,
		PromptingTemplateRepository: promptingTemplateRepository,
	}

	// Register all handlers
	client.registerHandlers(deps)

	return client, nil
}

// setupDatabase initializes the database connection and schema
func setupDatabase(connectionString string) (*database.DB, error) {
	db, err := database.NewDB(connectionString)
	if err != nil {
		return nil, err
	}

	if err := db.InitWithMigrations(); err != nil {
		return nil, err
	}

	return db, nil
}

// registerHandlers registers all bot handlers
func (b *TgBotClient) registerHandlers(deps *HandlerDependencies) {
	// Register start handler, that avaliable for all users
	b.dispatcher.AddHandler(handlers.NewStartHandler(deps.AppConfig))

	// Register admin chat handlers
	adminHandlers := []ext.Handler{
		adminhandlers.NewCodeHandler(deps.AppConfig),
		adminhandlers.NewTrySummarizeHandler(deps.SummarizationService, deps.MessageSenderService, deps.AppConfig),
		adminhandlers.NewShowTopicsHandler(deps.TopicRepository, deps.EventRepository, deps.MessageSenderService, deps.AppConfig),
		eventhandlers.NewEventEditHandler(deps.EventRepository, deps.AppConfig),
		eventhandlers.NewEventSetupHandler(deps.EventRepository, deps.AppConfig),
		eventhandlers.NewEventDeleteHandler(deps.EventRepository, deps.AppConfig),
		eventhandlers.NewEventFinishHandler(deps.EventRepository, deps.AppConfig),
	}

	// Register private chat handlers
	privateHandlers := []ext.Handler{
		privatehandlers.NewHelpHandler(deps.AppConfig),
		privatehandlers.NewToolsHandler(deps.OpenAiClient, deps.MessageSenderService, deps.PromptingTemplateRepository, deps.AppConfig),
		privatehandlers.NewContentHandler(deps.OpenAiClient, deps.MessageSenderService, deps.PromptingTemplateRepository, deps.AppConfig),
		privatehandlers.NewEventsHandler(deps.EventRepository, deps.AppConfig),
		topicshandlers.NewTopicsHandler(deps.TopicRepository, deps.EventRepository, deps.MessageSenderService, deps.AppConfig),
		topicshandlers.NewTopicAddHandler(deps.TopicRepository, deps.EventRepository, deps.MessageSenderService, deps.AppConfig),
	}

	// Register group chat handlers
	groupHandlers := []ext.Handler{
		grouphandlers.NewDeleteJoinLeftMessagesHandler(),
		grouphandlers.NewRepliesFromClosedThreadsHandler(deps.MessageSenderService, deps.AppConfig),
		grouphandlers.NewCleanClosedThreadsHandler(deps.MessageSenderService, deps.AppConfig),
	}

	// Combine all handlers
	allHandlers := append(append(adminHandlers, privateHandlers...), groupHandlers...)
	for _, handler := range allHandlers {
		b.dispatcher.AddHandler(handler)
	}
}

// Start begins the bot polling and starts scheduled tasks
func (b *TgBotClient) Start() {
	// Start scheduled tasks
	for _, task := range b.tasks {
		task.Start()
	}

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
		log.Fatal("Bot Runner: Failed to start polling: " + err.Error())
	}

	log.Printf("Bot Runner: Bot @%s has been started successfully\n", b.bot.User.Username)
	b.updater.Idle()
}

// Close gracefully shuts down the bot and all its resources
func (b *TgBotClient) Close() error {
	// Stop scheduled tasks
	for _, task := range b.tasks {
		task.Stop()
	}

	// Close database connection
	return b.db.Close()
}
