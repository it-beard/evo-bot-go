# Evocoders Telegram Bot

![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)

A Telegram bot for Evocoders Club management implemented in Go. Helps moderate discussions, provides AI-powered search, and generates daily chat summaries.

## 🚀 Features

### Moderation
- ✅ **Thread Management**: Deletes non-admin messages in read-only threads
- ✅ **Message Forwarding**: Forwards replies from closed threads to the general topic
- ✅ **Join/Leave Cleanup**: Removes join/leave messages for cleaner conversations
- ✅ **Message Tracking**: Automatically stores group messages for summarization and analysis
- ✅ **Topic Management**: Tracks forum topics and their metadata

### AI-Powered Functionality
- 🔍 **Tools Search** (`/tools`): Finds relevant AI tools based on user queries with fast and deep search options
- 📚 **Content Search** (`/content`): Searches through designated topics for information with fast and deep search options
- 👋 **Club Members Introduction Search** (`/intro`): Provides information about club members with fast and deep search options
- 📋 **Chat Summarization**: Creates daily summaries of conversations
  - Auto-posts at configured times
  - Manual trigger with `/trySummarize` (admin-only)
  - Send a knowledge base link with `/tryLinkToLearn` (admin-only, private)

### 🎲 Weekly Random Coffee Meetings
- **Automated Participation Poll**: Every week (configurable day and time in UTC, defaults to Friday at 2 PM UTC), the bot posts a poll asking members if they want to participate in random coffee meetings for the following week.
- **Opt-in/Opt-out**: Members can easily indicate their availability by responding to the poll. Votes can be changed or retracted before pairs are made.
- **Automated Pairing**: The bot automatically generates and announces pairs on a scheduled basis (configurable day and time in UTC, defaults to Monday at 12 PM UTC) using a smart algorithm that considers pairing history.
- **Smart Pairing Algorithm**: Pairs are generated considering past pairing history to avoid recent duplicates and ensure fair distribution of meetings.
- **Manual Pairing**: An administrator can also manually trigger the pairing process using the `/tryGenerateCoffeePairs` command.
- **Random Pair Announcement**: The bot randomly pairs participating members and announces the pairs in the main chat.
- **Self-Managed Meetings**: Paired members are encouraged to contact each other to arrange the day, time, and format of their meeting.

### User Profile Management
- 👤 **Profile Command** (`/profile`): Manage your personal profile
  - Create and edit personal information (name, bio)
  - Publish your profile to the designated "Intro" topic
  - Search for other club members' profiles

### Event Management
- 📅 **Event Management**: Track and organize community events
  - Support for different event types and statuses
  - Event publishing with start times
  - Topic organization within events

### Events Topic Management
- 📝 **Topic Viewing** (`/topics`): Browse topics and questions from events
- ➕ **Topic Addition** (`/topicAdd`): Add new topics and questions to events (for all club members)
- 🔧 **Admin Topic Management** (`/showTopics`): View and delete topics (admin-only)

### Administrative Controls
- 👥 **Profiles Manager** (`/profilesManager`): Admin tool for managing user profiles
- 🧪 **Test Handlers**: Manual testing tools for coffee pools (`/tryCreateCoffeePool`), pair generation (`/tryGenerateCoffeePairs`), and sending knowledge base link (`/tryLinkToLearn`)
- 📊 **Event Management**: Create, edit, start, and delete events (`/eventSetup`, `/eventEdit`, `/eventStart`, `/eventDelete`)

### Utility
- ❌ **Cancel** (`/cancel`): Cancel any ongoing operation
- 🧩 **Dynamic Templates**: Customizable AI prompts stored in database

For more details on bot usage, use the `/help` command in the bot chat.

## 🛠️ Technology Stack

- **Language**: Go 1.21+
- **Framework**: [gotgbot](https://github.com/PaulSonOfLars/gotgbot) for Telegram Bot API
- **Database**: PostgreSQL with automated migrations
- **AI Integration**: OpenAI API for content analysis and search
- **Architecture**: Clean layered architecture with dependency injection
- **Testing**: Comprehensive unit tests with gotestsum support

## 🏢 Architecture Overview

The bot follows a clean layered architecture:

- **Handlers Layer** (`internal/handlers/`): Processes Telegram updates
  - `adminhandlers/`: Admin-only commands
  - `grouphandlers/`: Group chat moderation
  - `privatehandlers/`: User-facing private commands
- **Services Layer** (`internal/services/`): Business logic
  - Core services: Profile, RandomCoffee, Summarization
  - Group handler services: Message processing, moderation
- **Repository Layer** (`internal/database/repositories/`): Data access
- **Database Layer** (`internal/database/`): Schema migrations and connections
- **Configuration Layer** (`internal/config/`): Environment management
- **Utilities Layer** (`internal/utils/`): Helper functions and utilities

## 🔑 Required Bot Permissions

For the bot to function properly, it must have the following admin permissions in the Telegram supergroup:

- 📌 **Pin messages**: Required for pinning event announcements and important information
- 🗑️ **Delete messages**: Required for clearing service messages and moderating threads

To assign these permissions, add the bot as an administrator in your group and enable these specific rights.

## 💾 Database

The bot uses PostgreSQL with automatically initialized tables:

| Table | Purpose | Key Fields |
|-------|---------|------------|
| **group_messages** | Stores group messages for summarization | `id`, `message_id`, `message_text`, `reply_to_message_id`, `user_tg_id`, `group_topic_id`, `created_at`, `updated_at` |
| **group_topics** | Stores forum topic names and metadata | `id`, `topic_id`, `name`, `created_at`, `updated_at` |
| **prompting_templates** | Stores AI prompting templates | `template_key`, `template_text` |
| **users** | Stores user information | `id`, `tg_id`, `firstname`, `lastname`, `tg_username`, `score`, `has_coffee_ban`, `is_club_member` |
| **profiles** | Stores user profile data | `id`, `user_id`, `bio`, `published_message_id`, `created_at`, `updated_at` |
| **events** | Stores event information | `id`, `name`, `type`, `status`, `started_at`, `created_at`, `updated_at` |
| **topics** | Stores topics related to events | `id`, `topic`, `user_nickname`, `event_id`, `created_at` |
| **random_coffee_polls** | Stores random coffee poll information | `id`, `message_id`, `telegram_poll_id`, `week_start_date`, `created_at` |
| **random_coffee_participants** | Stores poll participants data | `id`, `poll_id`, `user_id`, `participating`, `updated_at` |
| **random_coffee_pairs** | Stores the history of generated random coffee pairs | `id`, `poll_id`, `user1_id`, `user2_id`, `created_at` |
| **migrations** | Tracks database migrations | `id`, `name`, `timestamp`, `created_at` |

## 🔨 Building and Development

### Building the Executable

To build the executable for different platforms:

**For Windows:**
```shell
GOOS=windows GOARCH=amd64 go build -o bot.exe
```

**For Linux:**
```shell
GOOS=linux GOARCH=amd64 go build -o bot
```

**For macOS:**
```shell
GOOS=darwin GOARCH=amd64 go build -o bot
```

### Development Workflow

Run the bot with:

```shell
go run main.go  
```

Build the project:

```shell
go build main.go
```

Command for update dependencies:

```shell
go mod tidy
```

## ⚙️ Configuration

The bot uses environment variables for configuration, make sure to set them all:

### Basic Bot Configuration
- `TG_EVO_BOT_TOKEN`: Your Telegram bot token
- `TG_EVO_BOT_SUPERGROUP_CHAT_ID`: Chat ID of your Supergroup
- `TG_EVO_BOT_ADMIN_USER_ID`: User ID for the administrator account (will get notifications about new topics)
- `TG_EVO_BOT_OPENAI_API_KEY`: OpenAI API key

### Topics Management
- `TG_EVO_BOT_CLOSED_TOPICS_IDS`: Comma-separated list of topic IDs that are closed for chatting
- `TG_EVO_BOT_FORWARDING_TOPIC_ID`: ID of the topic where forwarded replies will be sent (0 for General topic)
- `TG_EVO_BOT_TOOL_TOPIC_ID`: Topic ID for the AI tools database (used by `/tools` command)
- `TG_EVO_BOT_CONTENT_TOPIC_ID`: Topic ID for the content topic (used by `/content` command)
- `TG_EVO_BOT_INTRO_TOPIC_ID`: Topic ID for club introductions and member information (used by `/intro` command)
- `TG_EVO_BOT_ANNOUNCEMENT_TOPIC_ID`: Topic ID for announcements

### Daily Summarization Feature
- `TG_EVO_BOT_DB_CONNECTION`: PostgreSQL connection string (e.g., `postgresql://user:password@localhost:5432/dbname`) - the database will be automatically initialized with required tables
- `TG_EVO_BOT_MONITORED_TOPICS_IDS`: Comma-separated list of topic IDs to monitor for summarization
- `TG_EVO_BOT_SUMMARY_TOPIC_ID`: Topic ID where daily summaries will be posted
- `TG_EVO_BOT_SUMMARY_TIME`: Time to run daily summary in 24-hour format (e.g., `03:00` for 3 AM)
- `TG_EVO_BOT_SUMMARIZATION_TASK_ENABLED`: Enable or disable the daily summarization task (`true` or `false`, defaults to `true` if not specified)

### Random Coffee Feature
- `TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID`: Topic ID where random coffee polls and pairs will be posted
- `TG_EVO_BOT_RANDOM_COFFEE_POLL_TASK_ENABLED`: Enable or disable the weekly coffee poll task (`true` or `false`, defaults to `true` if not specified)
- `TG_EVO_BOT_RANDOM_COFFEE_POLL_TIME`: Time to send the weekly coffee poll in 24-hour format UTC (e.g., `14:00` for 2 PM UTC, defaults to `14:00` if not specified)
- `TG_EVO_BOT_RANDOM_COFFEE_POLL_DAY`: Day of the week to send the poll (e.g., `friday`, `monday`, etc., defaults to `friday` if not specified)
- `TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TASK_ENABLED`: Enable or disable the automatic pairs generation task (`true` or `false`, defaults to `true` if not specified)
- `TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TIME`: Time to generate and announce coffee pairs in 24-hour format UTC (e.g., `12:00` for 12 PM UTC, defaults to `12:00` if not specified)
- `TG_EVO_BOT_RANDOM_COFFEE_PAIRS_DAY`: Day of the week to generate pairs (e.g., `monday`, `tuesday`, etc., defaults to `monday` if not specified)

On Windows, you can set the environment variables using the following commands in Command Prompt:

```shell
# Basic Bot Configuration 
set TG_EVO_BOT_TOKEN=your_bot_token_here
set TG_EVO_BOT_OPENAI_API_KEY=your_openai_api_key_here
set TG_EVO_BOT_SUPERGROUP_CHAT_ID=chat_id
set TG_EVO_BOT_ADMIN_USER_ID=admin_user_id

# Topics Management
set TG_EVO_BOT_CLOSED_TOPICS_IDS=topic_id_1,topic_id_2,topic_id_3
set TG_EVO_BOT_FORWARDING_TOPIC_ID=forwarding_topic_id
set TG_EVO_BOT_TOOL_TOPIC_ID=tool_topic_id
set TG_EVO_BOT_CONTENT_TOPIC_ID=content_topic_id
set TG_EVO_BOT_INTRO_TOPIC_ID=intro_topic_id
set TG_EVO_BOT_ANNOUNCEMENT_TOPIC_ID=announcement_topic_id

# Daily Summarization Feature
set TG_EVO_BOT_DB_CONNECTION=postgresql://user:password@localhost:5432/dbname
set TG_EVO_BOT_MONITORED_TOPICS_IDS=0,2
set TG_EVO_BOT_SUMMARY_TOPIC_ID=3
set TG_EVO_BOT_SUMMARY_TIME=03:00
set TG_EVO_BOT_SUMMARIZATION_TASK_ENABLED=true

# Random Coffee Feature
set TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID=random_coffee_topic_id
set TG_EVO_BOT_RANDOM_COFFEE_POLL_TASK_ENABLED=true
set TG_EVO_BOT_RANDOM_COFFEE_POLL_TIME=14:00
set TG_EVO_BOT_RANDOM_COFFEE_POLL_DAY=friday
set TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TASK_ENABLED=true
set TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TIME=12:00
set TG_EVO_BOT_RANDOM_COFFEE_PAIRS_DAY=monday
```

Then run the executable.

## Running Tests

This project includes unit tests to ensure functionality works as expected. Here are various ways to run the tests:

### Run All Tests

To run all tests in the project:

```shell
go test ./...
```

This command will recursively run all tests in all packages of your project.

### Run Tests in a Specific Package

To run tests in a specific package:

```shell
go test evo-bot-go/internal/handlers/privatehandlers
```

Or navigate to the package directory and run:

```shell
cd internal/handlers/privatehandlers
go test
```

### Run a Specific Test

To run a specific test function:

```shell
go test -run TestHelpHandler_Name evo-bot-go/internal/handlers/privatehandlers
```

The `-run` flag accepts a regular expression that matches test function names.

### Verbose Output

For more detailed test output, add the `-v` flag:

```shell
go test -v ./...
```

### Code Coverage

To see test coverage:

```shell
go test -cover ./...
```

For a detailed coverage report:

```shell
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

This will generate an HTML report showing which lines of code are covered by tests.

### Test with Race Detection

To check for race conditions:

```shell
go test -race ./...
```

### Colored Test Output with gotestsum

For better visibility with colored test output and icons, you can use gotestsum:

```shell
# Install gotestsum
go install gotest.tools/gotestsum@latest

# Run tests with colored output and icons
gotestsum --format pkgname --format-icons hivis

# If gotestsum is not in your PATH, run it directly
go run gotest.tools/gotestsum@latest --format pkgname --format-icons hivis
```

For maximum detail with colors and icons:

```shell
go run gotest.tools/gotestsum@latest --format standard-verbose --format-icons hivis --packages=./... -- -v
```

This provides colored output with clear pass/fail indicators and detailed test information.
