# Getting Started

<cite>
**Referenced Files in This Document**   
- [README.md](file://README.md)
- [config.go](file://internal/config/config.go)
- [main.go](file://main.go)
- [bot.go](file://internal/bot/bot.go)
- [daily_summarization_task.go](file://internal/tasks/daily_summarization_task.go)
- [random_coffee_service.go](file://internal/services/random_coffee_service.go)
</cite>

## Table of Contents
1. [Prerequisites](#prerequisites)
2. [Environment Configuration](#environment-configuration)
3. [Building and Running the Application](#building-and-running-the-application)
4. [Telegram User Client Setup](#telegram-user-client-setup)

## Prerequisites

Before setting up the evocoders-bot-go application, ensure you have the following prerequisites installed and configured:

- **Go 1.21+**: The application is built using Go version 1.21 or higher. Verify your Go installation with `go version`.
- **PostgreSQL**: The bot uses PostgreSQL for data persistence. Ensure PostgreSQL is installed and running, and that you have a database created for the application.
- **Telegram Bot Token**: Obtain a bot token from [@BotFather](https://t.me/BotFather) on Telegram.
- **Telegram API App ID and Hash**: Required for the Telegram User Client. These can be obtained by creating a new application at [my.telegram.org](https://my.telegram.org).
- **OpenAI API Key**: Required for AI-powered features such as search and summarization.

Ensure that the Telegram bot has the following admin permissions in your supergroup:
- **Pin messages**: Required for pinning event announcements and important information.
- **Delete messages**: Required for clearing service messages and moderating threads.

**Section sources**
- [README.md](file://README.md#L1-L295)

## Environment Configuration

The evocoders-bot-go application uses environment variables for configuration. All required environment variables are validated at startup in the `LoadConfig` function.

### Basic Bot Configuration
The following environment variables are required for basic bot operation:
- `TG_EVO_BOT_TOKEN`: Your Telegram bot token
- `TG_EVO_BOT_SUPERGROUP_CHAT_ID`: Chat ID of your Telegram supergroup
- `TG_EVO_BOT_OPENAI_API_KEY`: OpenAI API key for AI features
- `TG_EVO_BOT_ADMIN_USER_ID`: Telegram user ID of the administrator (optional)

### Topics Management
Configure topic IDs for various chat functionalities:
- `TG_EVO_BOT_CLOSED_TOPICS_IDS`: Comma-separated list of topic IDs closed for chatting
- `TG_EVO_BOT_FORWARDING_TOPIC_ID`: Topic ID where replies from closed threads will be forwarded (use 0 for General topic)
- `TG_EVO_BOT_TOOL_TOPIC_ID`: Topic ID for AI tools database
- `TG_EVO_BOT_CONTENT_TOPIC_ID`: Topic ID for content topics
- `TG_EVO_BOT_INTRO_TOPIC_ID`: Topic ID for club introductions and member information
- `TG_EVO_BOT_ANNOUNCEMENT_TOPIC_ID`: Topic ID for announcements

### Telegram User Client
Configure the Telegram User Client for extended functionality:
- `TG_EVO_BOT_TGUSERCLIENT_APPID`: Telegram API App ID
- `TG_EVO_BOT_TGUSERCLIENT_APPHASH`: Telegram API App Hash
- `TG_EVO_BOT_TGUSERCLIENT_PHONENUMBER`: Phone number for the Telegram account
- `TG_EVO_BOT_TGUSERCLIENT_2FAPASS`: Two-factor authentication password (if enabled)
- `TG_EVO_BOT_TGUSERCLIENT_SESSION_TYPE`: Session storage type (`file`, `database`, or `memory`)

### Daily Summarization Feature
Configure settings for the daily chat summarization:
- `TG_EVO_BOT_DB_CONNECTION`: PostgreSQL connection string (e.g., `postgresql://user:password@localhost:5432/dbname`)
- `TG_EVO_BOT_MONITORED_TOPICS_IDS`: Comma-separated list of topic IDs to monitor for summarization
- `TG_EVO_BOT_SUMMARY_TOPIC_ID`: Topic ID where daily summaries will be posted
- `TG_EVO_BOT_SUMMARY_TIME`: Time to run daily summary in 24-hour format (e.g., `03:00`)
- `TG_EVO_BOT_SUMMARIZATION_TASK_ENABLED`: Enable/disable daily summarization (`true`/`false`, defaults to `true`)

### Random Coffee Feature
Configure settings for the weekly random coffee meetings:
- `TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID`: Topic ID where random coffee polls and pairs will be posted
- `TG_EVO_BOT_RANDOM_COFFEE_POLL_TASK_ENABLED`: Enable/disable weekly coffee poll (`true`/`false`, defaults to `true`)
- `TG_EVO_BOT_RANDOM_COFFEE_POLL_TIME`: Time to send poll in 24-hour UTC format (e.g., `14:00`, defaults to `14:00`)
- `TG_EVO_BOT_RANDOM_COFFEE_POLL_DAY`: Day to send poll (e.g., `friday`, defaults to `friday`)
- `TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TASK_ENABLED`: Enable/disable automatic pairs generation (`true`/`false`, defaults to `true`)
- `TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TIME`: Time to generate pairs in 24-hour UTC format (e.g., `12:00`, defaults to `12:00`)
- `TG_EVO_BOT_RANDOM_COFFEE_PAIRS_DAY`: Day to generate pairs (e.g., `monday`, defaults to `monday`)

**Section sources**
- [config.go](file://internal/config/config.go#L0-L341)
- [README.md](file://README.md#L1-L295)

## Building and Running the Application

To build and run the evocoders-bot-go application, follow these steps:

### Running the Application
Execute the bot using the following command:
```shell
go run main.go
```

### Building the Application
Compile the application with:
```shell
go build main.go
```

### Building for Windows
To create a Windows executable, use:
```shell
GOOS=windows GOARCH=amd64 go build -o bot.exe
```
This generates a 64-bit Windows executable named `bot.exe`.

### Updating Dependencies
Keep dependencies up to date with:
```shell
go mod tidy
```

The application will validate all required environment variables at startup. If any required variable is missing, the application will exit with an error message specifying the missing configuration.

**Section sources**
- [main.go](file://main.go#L0-L54)
- [README.md](file://README.md#L1-L295)

## Telegram User Client Setup

The Telegram User Client enables additional functionality by connecting to Telegram with a user account.

### Configuration
Set the following environment variables:
- `TG_EVO_BOT_TGUSERCLIENT_APPID`: Your Telegram API App ID
- `TG_EVO_BOT_TGUSERCLIENT_APPHASH`: Your Telegram API App Hash
- `TG_EVO_BOT_TGUSERCLIENT_PHONENUMBER`: Your phone number in international format
- `TG_EVO_BOT_TGUSERCLIENT_2FAPASS`: Your 2FA password if enabled
- `TG_EVO_BOT_TGUSERCLIENT_SESSION_TYPE`: Set to `file` to save session data to `session.json`, `database` to store in PostgreSQL, or `memory` for in-memory storage

### Obtaining Verification Code
1. Run the application for the first time with the Telegram User Client configured
2. You will receive a verification code in your Telegram app
3. Send the code in **reversed** order to your bot using the `/code` command
4. The bot will authenticate and automatically update the session every 30 minutes

The session type determines where session data is stored:
- **File**: Creates `session.json` in the working directory
- **Database**: Stores session data in the `tg_sessions` table
- **Memory**: Session is lost when the application restarts

**Section sources**
- [bot.go](file://internal/bot/bot.go#L0-L385)
- [README.md](file://README.md#L1-L295)