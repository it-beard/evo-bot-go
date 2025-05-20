# Evocoders Telegram Bot

![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)

A Telegram bot for Evocoders Club management implemented in Go. Helps moderate discussions, provides AI-powered search, and generates daily chat summaries.

## 🚀 Features

### Moderation
- ✅ **Thread Management**: Deletes non-admin messages in read-only threads
- ✅ **Message Forwarding**: Forwards replies from closed threads to the general topic
- ✅ **Join/Leave Cleanup**: Removes join/leave messages for cleaner conversations

### AI-Powered Functionality
- 🔍 **Tool Search** (`/tool`): Finds relevant AI tools based on user queries
- 📚 **Content Search** (`/content`): Searches through designated topics for information
- 👋 **Club Members Introduction Search** (`/intro`): Provides information about clubs members
- 📋 **Chat Summarization**: Creates daily summaries of conversations
  - Auto-posts at configured times
  - Manual trigger with `/summarize` (admin-only)

### User Profile Management
- 👤 **Profile Command** (`/profile`): Manage your personal profile
  - Create and edit personal information (name, bio)
  - Add social links (LinkedIn, GitHub, and custom links)
  - Publish your profile to the designated "Intro" topic
  - Search for other club members' profiles

### Event Management
- 📅 **Event Management**: Track and organize community events
  - Support for different event types and statuses
  - Event publishing with start times
  - Topic organization within events

### Utility
- ℹ️ **Help** (`/help`): Provides usage information
- 🧩 **Dynamic Templates**: Customizable AI prompts stored in database

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
| **profiles** | Stores user profile data | `id`, `user_id`, `bio`, `linkedin`, `github`, `freelink`, `published_message_id`, `created_at`, `updated_at` |
| **events** | Stores event information | `id`, `name`, `type`, `status`, `started_at`, `created_at`, `updated_at` |
| **topics** | Stores topics related to events | `id`, `topic`, `user_nickname`, `event_id`, `created_at` |
| **migrations** | Tracks database migrations | `id`, `name`, `timestamp`, `created_at` |

## Building the Executable

To build the executable for Windows, use the following command:

```shell
GOOS=windows GOARCH=amd64 go build -o bot.exe
```

This command will create a Windows executable named `bot.exe` that can run on 64-bit Windows systems.

## Develop and run the bot

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
