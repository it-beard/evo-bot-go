# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Telegram bot for Evocoders Club management implemented in Go. The bot helps moderate discussions, provides AI-powered search functionality, and generates daily chat summaries.

### Key Features

- **Thread Management**: Deletes non-admin messages in read-only threads
- **Message Forwarding**: Forwards replies from closed threads to the general topic
- **Join/Leave Cleanup**: Removes join/leave messages for cleaner conversations
- **AI-Powered Search**: Tool search, content search, and club member introduction search
- **Chat Summarization**: Creates daily summaries automatically or on-demand
- **Event Management**: Create and manage club events like meetups and club calls

## Development Commands

### Running the Bot

```bash
# Run the bot
go run main.go
```

### Managing Dependencies

```bash
# Update dependencies
go mod tidy
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with colored output (if gotestsum is installed)
go run gotest.tools/gotestsum@latest --format pkgname --format-icons hivis

# Run tests with coverage
go test -cover ./...

# Run specific test
go test -run TestFunctionName ./path/to/package

# Run tests with race detection
go test -race ./...
```

## Project Architecture

### Main Components

1. **Bot Client (`internal/bot/bot.go`)**: 
   - Core component that initializes and manages the bot instance
   - Registers all handlers and starts scheduled tasks

2. **Config (`internal/config/config.go`)**: 
   - Loads configuration from environment variables
   - Contains settings for bot token, chat IDs, OpenAI API key, and more

3. **Handlers**: 
   - **Admin Handlers**: Commands only available to admins like event management
   - **Group Handlers**: Handle group chat functionality like thread moderation
   - **Private Handlers**: User commands like help, content search, and tool search

4. **Database**:
   - PostgreSQL database with migration system
   - Stores events, topics, and AI prompting templates
   - Manages Telegram User Client sessions

5. **Services**:
   - **SummarizationService**: Generates daily chat summaries 
   - **MessageSenderService**: Centralized message sending with formatting
   - **PermissionsService**: Handles user permission checks

6. **Tasks**:
   - **DailySummarizationTask**: Runs chat summarization at configured time
   - **SessionKeepAliveTask**: Keeps Telegram User Client session alive

### Database Schema

The database has several key tables:
- **events**: Stores club events like meetups and club calls
- **topics**: Stores discussion topics linked to events
- **tg_sessions**: Manages Telegram User Client sessions
- **prompting_templates**: Stores AI prompting templates
- **users**: Stores user information with Telegram details
- **profiles**: User profile data and publishing information
- **random_coffee_polls**: Weekly random coffee participation polls
- **random_coffee_participants**: Poll participant responses
- **random_coffee_pairs**: Historical pairing data
- **migrations**: Database migration tracking (automatically managed)

### Handler Workflow

1. User sends a command to the bot
2. Bot routes the command to the appropriate handler
3. Handler validates permissions and input
4. Handler processes the command and sends a response

### Important Architectural Patterns

#### Dependency Injection Pattern
The bot uses a centralized `HandlerDependencies` struct (`internal/bot/bot.go:26`) that contains all services, repositories, and clients needed by handlers. This promotes clean separation of concerns and makes testing easier.

#### Repository Pattern
All database interactions go through repository interfaces located in `internal/database/repositories/`. Each entity (User, Profile, Event, etc.) has its own repository with standardized CRUD operations.

#### Service Layer Architecture
Business logic is separated into service layers (`internal/services/`):
- **MessageSenderService**: Centralized message sending with formatting
- **PermissionsService**: User permission validation
- **ProfileService**: User profile management
- **SummarizationService**: Chat summarization logic
- **RandomCoffeeService**: Random coffee pairing logic

#### Migration System
Database migrations are automatically managed in `internal/database/migrations/`. New migrations go in `implementations/` directory and must be added to the registry in `migrator.go`. Migrations run automatically on app startup.

#### Handler Registration
All handlers are registered in `internal/bot/bot.go:174` in the `registerHandlers` method, organized by type:
- **Admin handlers**: Commands only for administrators
- **Group handlers**: Handle group chat events and moderation
- **Private handlers**: User commands in private chats

### Application Startup Sequence

1. **Configuration Loading** (`config.LoadConfig()`): Loads all environment variables and validates required settings
2. **Client Initialization** (`clients.NewOpenAiClient()`): Creates OpenAI client for AI features
3. **Bot Creation** (`bot.NewTgBotClient()`): Initializes bot with all dependencies:
   - Database connection and migration execution
   - Repository and service initialization
   - Handler registration
   - Scheduled task setup
4. **Graceful Shutdown Setup**: Configures signal handling for clean termination
5. **Bot Start** (`botClient.Start()`): Begins polling and starts scheduled tasks

### Development Guidelines

#### Adding New Commands
1. Create handler in appropriate subdirectory (`adminhandlers/`, `grouphandlers/`, `privatehandlers/`)
2. Implement the `Handler` interface with `Name()` and `CheckUpdate()` methods
3. Add dependencies to `HandlerDependencies` struct if needed
4. Register handler in `registerHandlers()` method
5. Test with both unit tests and integration testing

#### Adding New Database Tables
1. Create migration in `internal/database/migrations/implementations/`
2. Add migration to registry in `migrator.go`
3. Create repository in `internal/database/repositories/`
4. Add repository to dependency injection in `bot.go`

#### Testing Patterns
The codebase uses table-driven tests. See `internal/utils/*_test.go` for examples of the testing patterns used.

## Configuration

The bot uses environment variables for configuration. These must be set before running:

### Critical Environment Variables

- `TG_EVO_BOT_TOKEN`: Telegram bot token
- `TG_EVO_BOT_SUPERGROUP_CHAT_ID`: Chat ID of your Supergroup
- `TG_EVO_BOT_ADMIN_USER_ID`: User ID for admin account
- `TG_EVO_BOT_OPENAI_API_KEY`: OpenAI API key
- `TG_EVO_BOT_DB_CONNECTION`: PostgreSQL connection string

See the README.md for the complete list of environment variables.