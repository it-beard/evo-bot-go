# Evocoders Telegram Bot

![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/License-MIT-blue.svg)
![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen.svg)

A Telegram bot for Evocoders Club management implemented in Go. Helps moderate discussions, provides AI-powered search, and generates daily chat summaries.

## 🚀 Features

### Moderation
- ✅ **Thread Management**: Deletes non-admin messages in read-only threads
- ✅ **Message Forwarding**: Forwards replies from closed threads to the general topic
- ✅ **Join/Leave Cleanup**: Removes join/leave messages for cleaner conversations
- 🛡️ **HTML Sanitization**: Automatically sanitizes HTML tags in user content for security

### AI-Powered Functionality
- 🔍 **Tool Search** (`/tool`): Finds relevant AI tools based on user queries
- 📚 **Content Search** (`/content`): Searches through designated topics for information
- 👋 **Club Members Introduction Search** (`/intro`): Provides information about club members
- 📋 **Chat Summarization**: Creates daily summaries of conversations
  - Auto-posts at configured times
  - Manual trigger with `/summarize` (admin-only)

### User Profile Management
- 👤 **Profile Command** (`/profile`): Comprehensive profile management system
  - Create and edit personal information (name, bio with length validation)
  - Publish your profile to the designated "Intro" topic
  - Search for other club members' profiles
  - View profile links and enhanced profile viewing capabilities
  - Duplicate detection for bio input timestamps
  - Profile button text optimization for better user experience

### Event Management
- 📅 **Event Management**: Track and organize community events
  - Support for different event types and statuses
  - Event publishing with start times
  - Topic organization within events

### Utility
- ℹ️ **Help** (`/help`): Provides comprehensive usage information
- 🧩 **Dynamic Templates**: Customizable AI prompts stored in database
- 🔧 **Graceful Shutdown**: Proper resource cleanup on application termination

For more details on bot usage, use the `/help` command in the bot chat.

## 🔑 Required Bot Permissions

For the bot to function properly, it must have the following admin permissions in the Telegram supergroup:

- 📌 **Pin messages**: Required for pinning event announcements and important information
- 🗑️ **Delete messages**: Required for clearing service messages and moderating threads

To assign these permissions, add the bot as an administrator in your group and enable these specific rights.

## 💾 Database

The bot uses PostgreSQL with automatically initialized tables:

| Table | Purpose | Key Fields |
|-------|---------|------------|
| **messages** | Stores chat data for summarization | `id`, `topic_id`, `message_text`, `created_at` |
| **tg_sessions** | Manages Telegram User Client sessions | `id`, `data`, `updated_at` |
| **prompting_templates** | Stores AI prompting templates | `template_key`, `template_text` |
| **users** | Stores user information | `id`, `tg_id`, `firstname`, `lastname`, `tg_username`, `score`, `has_coffee_ban` |
| **profiles** | Stores user profile data | `id`, `user_id`, `bio`, `published_message_id`, `created_at`, `updated_at` |
| **events** | Stores event information | `id`, `name`, `type`, `status`, `started_at`, `created_at`, `updated_at` |
| **topics** | Stores topics related to events | `id`, `topic`, `user_nickname`, `event_id`, `created_at` |
| **migrations** | Tracks database migrations | `id`, `name`, `timestamp`, `created_at` |

## 🏗️ Building and Development

### Building the Executable

To build the executable for different platforms:

**Windows (64-bit):**
```shell
GOOS=windows GOARCH=amd64 go build -o bot.exe
```

**Linux (64-bit):**
```shell
GOOS=linux GOARCH=amd64 go build -o bot
```

**macOS (64-bit):**
```shell
GOOS=darwin GOARCH=amd64 go build -o bot
```

### Development Commands

**Run the bot in development mode:**
```shell
go run main.go  
```

**Build the project:**
```shell
go build main.go
```

**Update dependencies:**
```shell
go mod tidy
```

**Download dependencies:**
```shell
go mod download
```

**Check for potential issues:**
```shell
go vet ./...
```

## 📁 Project Structure

The project follows a clean architecture pattern with the following structure:

```
evo-bot-go/
├── main.go                 # Application entry point
├── go.mod                  # Go module definition
├── go.sum                  # Go module checksums
├── README.md              # Project documentation
├── CLAUDE.md              # AI assistant documentation
└── internal/              # Internal application code
    ├── bot/               # Telegram bot client implementation
    ├── buttons/           # Inline keyboard button definitions
    ├── clients/           # External service clients (OpenAI, etc.)
    ├── config/            # Configuration management
    ├── constants/         # Application constants
    ├── database/          # Database connection and operations
    ├── formatters/        # Message and data formatters
    ├── handlers/          # Message and command handlers
    │   ├── privatehandlers/   # Private chat handlers
    │   └── supergrouphandlers/ # Supergroup chat handlers
    ├── services/          # Business logic services
    ├── tasks/             # Background tasks (summarization, etc.)
    └── utils/             # Utility functions and helpers
```

### Key Components

- **`main.go`**: Application bootstrap with graceful shutdown handling
- **`internal/bot/`**: Core Telegram bot implementation and client management
- **`internal/handlers/`**: Command and message processing logic
- **`internal/services/`**: Business logic for AI search, profiles, events
- **`internal/database/`**: PostgreSQL integration and schema management
- **`internal/tasks/`**: Scheduled tasks like daily summarization
- **`internal/config/`**: Environment-based configuration loading

## ⚙️ Configuration

The bot uses environment variables for configuration, make sure to set them all:

### Basic Bot Configuration
- `TG_EVO_BOT_TOKEN`: Your Telegram bot token
- `TG_EVO_BOT_SUPERGROUP_CHAT_ID`: Chat ID of your Supergroup
- `TG_EVO_BOT_ADMIN_USER_ID`: User ID for the administrator account (will get notifications about new topics)
- `TG_EVO_BOT_OPENAI_API_KEY`: OpenAI API key

### Topics Management
- `TG_EVO_BOT_CLOSED_TOPICS_IDS`: Comma-separated list of topic IDs that closed for chatting
- `TG_EVO_BOT_FORWARDING_TOPIC_ID`: ID of the topic where forwarded replies will be sent (0 for General topic)
- `TG_EVO_BOT_TOOL_TOPIC_ID`: Topic ID for the AI tools database
- `TG_EVO_BOT_CONTENT_TOPIC_ID`: Topic ID for the content topic
- `TG_EVO_BOT_INTRO_TOPIC_ID`: Topic ID for the club introductions and member information
- `TG_EVO_BOT_ANNOUNCEMENT_TOPIC_ID`: Topic ID for announcements

### Telegram User Client
- `TG_EVO_BOT_TGUSERCLIENT_APPID`: Telegram API App ID
- `TG_EVO_BOT_TGUSERCLIENT_APPHASH`: Telegram API App Hash
- `TG_EVO_BOT_TGUSERCLIENT_PHONENUMBER`: Phone number for Telegram user client
- `TG_EVO_BOT_TGUSERCLIENT_2FAPASS`: Two-factor authentication password for Telegram user client (if using 2FA)
- `TG_EVO_BOT_TGUSERCLIENT_SESSION_TYPE`: Session type for Telegram User Client. Available options:
  - `file`: Enables file storage (using `session.json`)
  - `database`: Uses database storage (requires valid `TG_EVO_BOT_DB_CONNECTION`)
  - `memory` or empty: Uses in-memory session storage (session will be lost after restart)

### Daily Summarization Feature
- `TG_EVO_BOT_DB_CONNECTION`: PostgreSQL connection string (e.g., `postgresql://user:password@localhost:5432/dbname`) - the database will be automatically initialized with required tables
- `TG_EVO_BOT_MONITORED_TOPICS_IDS`: Comma-separated list of topic IDs to monitor for summarization
- `TG_EVO_BOT_SUMMARY_TOPIC_ID`: Topic ID where daily summaries will be posted
- `TG_EVO_BOT_SUMMARY_TIME`: Time to run daily summary in 24-hour format (e.g., `03:00` for 3 AM)
- `TG_EVO_BOT_SUMMARIZATION_TASK_ENABLED`: Enable or disable the daily summarization task (`true` or `false`, defaults to `true` if not specified)

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

# Telegram User Client
set TG_EVO_BOT_TGUSERCLIENT_APPID=your_app_id
set TG_EVO_BOT_TGUSERCLIENT_APPHASH=your_app_hash
set TG_EVO_BOT_TGUSERCLIENT_PHONENUMBER=your_phone_number
set TG_EVO_BOT_TGUSERCLIENT_2FAPASS=your_2fa_password
set TG_EVO_BOT_TGUSERCLIENT_SESSION_TYPE=file

# Daily Summarization Feature
set TG_EVO_BOT_DB_CONNECTION=postgresql://user:password@localhost:5432/dbname
set TG_EVO_BOT_MONITORED_TOPICS_IDS=0,2
set TG_EVO_BOT_SUMMARY_TOPIC_ID=3
set TG_EVO_BOT_SUMMARY_TIME=03:00
set TG_EVO_BOT_SUMMARIZATION_TASK_ENABLED=true
```

Then run the executable.

## Obtain Verification Code

To obtain the verification code, you need to run the Telegram User Client. 
After first run you will get this **code in your telegram app**. 

Send this code **REVERTED** by /code command to your bot.

After that your bot will be able to use Telegram User Client and will update session automaticaly once per _30 minutes_.

## 🧪 Running Tests

This project includes comprehensive unit tests to ensure functionality works as expected. Here are various ways to run and analyze the tests:

### Basic Test Commands

**Run all tests in the project:**
```shell
go test ./...
```

**Run tests with verbose output:**
```shell
go test -v ./...
```

**Run tests in a specific package:**
```shell
go test evo-bot-go/internal/handlers/privatehandlers
```

**Run a specific test function:**
```shell
go test -run TestHelpHandler_Name evo-bot-go/internal/handlers/privatehandlers
```

### Advanced Testing Options

**Test with race condition detection:**
```shell
go test -race ./...
```

**Run tests with timeout:**
```shell
go test -timeout 30s ./...
```

**Run tests multiple times to catch flaky tests:**
```shell
go test -count=5 ./...
```

### Code Coverage Analysis

**Basic coverage report:**
```shell
go test -cover ./...
```

**Generate detailed coverage profile:**
```shell
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

**Coverage by function:**
```shell
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

**Set coverage threshold:**
```shell
go test -cover ./... | grep -E "coverage: [0-9]+\.[0-9]+%" | awk '{if($2 < 80.0) exit 1}'
```

### Enhanced Test Output with gotestsum

For better visibility with colored output and icons:

**Install gotestsum:**
```shell
go install gotest.tools/gotestsum@latest
```

**Run tests with colored output:**
```shell
gotestsum --format pkgname --format-icons hivis
```

**Detailed output with colors and icons:**
```shell
gotestsum --format standard-verbose --format-icons hivis --packages=./... -- -v
```

**Run tests with coverage using gotestsum:**
```shell
gotestsum --format pkgname -- -cover ./...
```

### Continuous Integration

For CI/CD pipelines, use these commands:

```shell
# Run all tests with race detection and coverage
go test -race -coverprofile=coverage.out -covermode=atomic ./...

# Generate coverage report
go tool cover -html=coverage.out -o coverage.html

# Check coverage threshold (example: 80%)
go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//' | awk '{if($1 < 80) exit 1}'
```

## 🔧 Troubleshooting

### Common Issues

**Bot not responding to commands:**
- Verify the bot token is correct and the bot is added to the supergroup
- Check that the bot has the required admin permissions (pin messages, delete messages)
- Ensure environment variables are properly set

**Database connection errors:**
- Verify PostgreSQL is running and accessible
- Check the database connection string format
- Ensure the database exists and the user has proper permissions

**Telegram User Client authentication issues:**
- Make sure the phone number format is correct (with country code)
- Verify the App ID and App Hash from Telegram API
- Check if 2FA password is required and properly set

**Summarization not working:**
- Verify monitored topic IDs are correct
- Check if the summarization task is enabled in configuration
- Ensure the summary time format is correct (HH:MM)

**OpenAI API errors:**
- Verify the API key is valid and has sufficient credits
- Check for rate limiting issues
- Ensure the API key has access to the required models

### Debug Mode

To run the bot with more verbose logging, you can modify the log level in the code or add debug environment variables as needed.

### Getting Help

- Check the `/help` command in the bot for usage information
- Review the configuration section for proper environment variable setup
- Examine the logs for specific error messages
- Ensure all dependencies are properly installed with `go mod tidy`

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📞 Support

For support and questions:
- Open an issue in the GitHub repository
- Contact the Evocoders Club administrators
- Check the bot's `/help` command for usage guidance
