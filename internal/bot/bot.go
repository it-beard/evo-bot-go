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
	OpenAiClient                      *clients.OpenAiClient
	AppConfig                         *config.Config
	SummarizationService              *services.SummarizationService
	MessageSenderService              *services.MessageSenderService
	PermissionsService                *services.PermissionsService
	EventRepository                   *repositories.EventRepository
	TopicRepository                   *repositories.TopicRepository
	PromptingTemplateRepository       *repositories.PromptingTemplateRepository
	UserRepository                    *repositories.UserRepository
	ProfileRepository                 *repositories.ProfileRepository
	RandomCoffeePollRepository        *repositories.RandomCoffeePollRepository
	RandomCoffeeParticipantRepository *repositories.RandomCoffeeParticipantRepository
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
	userRepository := repositories.NewUserRepository(db.DB)
	profileRepository := repositories.NewProfileRepository(db.DB)
	randomCoffeePollRepository := repositories.NewRandomCoffeePollRepository(db.DB)
	randomCoffeeParticipantRepository := repositories.NewRandomCoffeeParticipantRepository(db.DB)

	// Initialize services
	messageSenderService := services.NewMessageSenderService(bot)
	pollSenderService := services.NewPollSenderService(bot)
	permissionsService := services.NewPermissionsService(appConfig, bot, messageSenderService)
	summarizationService := services.NewSummarizationService(
		appConfig, openaiClient, messageSenderService, promptingTemplateRepository,
	)
	randomCoffeePollService := services.NewRandomCoffeePollService(
		appConfig, pollSenderService, randomCoffeePollRepository)

	// Initialize scheduled tasks
	scheduledTasks := []tasks.Task{
		tasks.NewSessionKeepAliveTask(30 * time.Minute),
		tasks.NewDailySummarizationTask(appConfig, summarizationService),
		tasks.NewRandomCoffeePollTask(appConfig, randomCoffeePollService),
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
		OpenAiClient:                      openaiClient,
		AppConfig:                         appConfig,
		SummarizationService:              summarizationService,
		MessageSenderService:              messageSenderService,
		PermissionsService:                permissionsService,
		EventRepository:                   eventRepository,
		TopicRepository:                   topicRepository,
		PromptingTemplateRepository:       promptingTemplateRepository,
		UserRepository:                    userRepository,
		ProfileRepository:                 profileRepository,
		RandomCoffeePollRepository:        randomCoffeePollRepository,
		RandomCoffeeParticipantRepository: randomCoffeeParticipantRepository,
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
	b.dispatcher.AddHandler(handlers.NewStartHandler(deps.AppConfig, deps.MessageSenderService, deps.PermissionsService))

	// Register admin chat handlers
	adminHandlers := []ext.Handler{
		adminhandlers.NewCodeHandler(deps.AppConfig, deps.MessageSenderService, deps.PermissionsService),
		adminhandlers.NewTrySummarizeHandler(deps.AppConfig, deps.SummarizationService, deps.MessageSenderService, deps.PermissionsService),
		adminhandlers.NewShowTopicsHandler(deps.AppConfig, deps.TopicRepository, deps.EventRepository, deps.MessageSenderService, deps.PermissionsService),
		adminhandlers.NewAdminProfilesHandler(deps.AppConfig, deps.MessageSenderService, deps.PermissionsService, deps.UserRepository, deps.ProfileRepository),
		eventhandlers.NewEventEditHandler(deps.AppConfig, deps.EventRepository, deps.MessageSenderService, deps.PermissionsService),
		eventhandlers.NewEventSetupHandler(deps.AppConfig, deps.EventRepository, deps.MessageSenderService, deps.PermissionsService),
		eventhandlers.NewEventDeleteHandler(deps.AppConfig, deps.EventRepository, deps.MessageSenderService, deps.PermissionsService),
		eventhandlers.NewEventStartHandler(deps.AppConfig, deps.EventRepository, deps.MessageSenderService, deps.PermissionsService),
		adminhandlers.NewPairRandomCoffeeHandler(deps.AppConfig, deps.PermissionsService, deps.MessageSenderService, deps.RandomCoffeePollRepository, deps.RandomCoffeeParticipantRepository),
	}

	// Register private chat handlers
	privateHandlers := []ext.Handler{
		privatehandlers.NewHelpHandler(deps.AppConfig, deps.MessageSenderService, deps.PermissionsService),
		privatehandlers.NewToolsHandler(deps.AppConfig, deps.OpenAiClient, deps.MessageSenderService, deps.PromptingTemplateRepository, deps.PermissionsService),
		privatehandlers.NewContentHandler(deps.AppConfig, deps.OpenAiClient, deps.MessageSenderService, deps.PromptingTemplateRepository, deps.PermissionsService),
		privatehandlers.NewIntroHandler(deps.AppConfig, deps.OpenAiClient, deps.MessageSenderService, deps.PromptingTemplateRepository, deps.PermissionsService),
		privatehandlers.NewEventsHandler(deps.AppConfig, deps.EventRepository, deps.MessageSenderService, deps.PermissionsService),
		privatehandlers.NewProfileHandler(deps.AppConfig, deps.MessageSenderService, deps.PermissionsService, deps.UserRepository, deps.ProfileRepository),
		topicshandlers.NewTopicsHandler(deps.AppConfig, deps.TopicRepository, deps.EventRepository, deps.MessageSenderService, deps.PermissionsService),
		topicshandlers.NewTopicAddHandler(deps.AppConfig, deps.TopicRepository, deps.EventRepository, deps.MessageSenderService, deps.PermissionsService),
	}

	// Register group chat handlers
	groupHandlers := []ext.Handler{
		grouphandlers.NewDeleteJoinLeftMessagesHandler(),
		grouphandlers.NewRepliesFromClosedThreadsHandler(deps.AppConfig, deps.MessageSenderService),
		grouphandlers.NewCleanClosedThreadsHandler(deps.AppConfig, deps.MessageSenderService),
		grouphandlers.NewRandomCoffeePollAnswerHandler(deps.AppConfig, deps.UserRepository, deps.RandomCoffeePollRepository, deps.RandomCoffeeParticipantRepository),
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
	log.Printf("Bot Runner: Current server time is %s (UTC: %s)", time.Now(), time.Now().UTC())
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
