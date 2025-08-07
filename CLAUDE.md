# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Telegram bot for Evocoders Club management implemented in Go. The bot helps moderate discussions, provides AI-powered search functionality, and generates daily chat summaries.

### Key Features

- **Thread Management**: Deletes non-admin messages in read-only threads
- **Message Forwarding**: Forwards replies from closed threads to the general topic
- **Join/Leave Cleanup**: Removes join/leave messages for cleaner conversations
- **AI-Powered Search**: Tool search (`/tools`), content search (`/content`), and club member introduction search (`/intro`)
- **Chat Summarization**: Creates daily summaries automatically or on-demand with `/trySummarize`
- **Event Management**: Create and manage club events like meetups and club calls
- **User Profiles**: Personal profile management with `/profile` command
- **Random Coffee**: Weekly automated polls and pairing for random coffee meetings

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
   - **Admin Handlers**: Commands only available to admins like event management (`/eventSetup`, `/eventStart`, `/eventEdit`, `/eventDelete`), profile management (`/profilesManager`), and testing commands (`/trySummarize`, `/tryCreateCoffeePool`, `/tryGenerateCoffeePairs`)
   - **Group Handlers**: Handle group chat functionality like thread moderation, join/leave message cleanup, and Random Coffee poll responses
   - **Private Handlers**: User commands like help (`/help`), search commands (`/tools`, `/content`, `/intro`), event viewing (`/events`, `/topics`), topic suggestions (`/topicAdd`), and profile management (`/profile`)

4. **Database**:
   - PostgreSQL database with migration system
   - Stores events, topics, and AI prompting templates
   - Manages Telegram User Client sessions

5. **Services**:
   - **SummarizationService**: Generates daily chat summaries 
   - **MessageSenderService**: Centralized message sending with formatting
   - **PermissionsService**: Handles user permission checks
   - **ProfileService**: Manages user profile operations
   - **RandomCoffeeService**: Handles Random Coffee polls and pairing
   - **PollSenderService**: Sends and manages Telegram polls

6. **Tasks**:
   - **DailySummarizationTask**: Runs chat summarization at configured time
   - **SessionKeepAliveTask**: Keeps Telegram User Client session alive
   - **RandomCoffeePollTask**: Creates weekly Random Coffee participation polls
   - **RandomCoffeePairsTask**: Generates and announces Random Coffee pairs

### Database Schema

The database has several key tables:
- **events**: Stores club events like meetups and club calls
- **topics**: Stores discussion topics linked to events
- **users**: Stores user information including club membership status
- **profiles**: Stores user profile data with bio and published message references
- **tg_sessions**: Manages Telegram User Client sessions
- **prompting_templates**: Stores AI prompting templates
- **random_coffee_polls**: Stores weekly Random Coffee polls
- **random_coffee_participants**: Tracks poll participation
- **random_coffee_pairs**: Stores generated coffee meeting pairs
- **migrations**: Database migration tracking

### Handler Workflow

1. User sends a command to the bot
2. Bot routes the command to the appropriate handler
3. Handler validates permissions and input
4. Handler processes the command and sends a response

## Configuration

The bot uses environment variables for configuration. These must be set before running:

### Critical Environment Variables

- `TG_EVO_BOT_TOKEN`: Telegram bot token
- `TG_EVO_BOT_SUPERGROUP_CHAT_ID`: Chat ID of your Supergroup
- `TG_EVO_BOT_ADMIN_USER_ID`: User ID for admin account
- `TG_EVO_BOT_OPENAI_API_KEY`: OpenAI API key
- `TG_EVO_BOT_DB_CONNECTION`: PostgreSQL connection string

See the README.md for the complete list of environment variables.