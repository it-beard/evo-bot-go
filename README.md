# Telegram Bot

This project contains a Telegram bot for Evocoders Club management. Bot implemented in Go.

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

Command for update dependencies:

```shell
go mod tidy
```

## Running the Bot

Before running the bot, make sure to set the following environment variables:

### Basic Bot Configuration
- `TG_EVO_BOT_TOKEN`: Your Telegram bot token
- `TG_EVO_BOT_SUPERGROUP_CHAT_ID`: Chat ID of your Supergroup
- `TG_EVO_BOT_ANONYMOUS_USER_ID`: User ID for the account used on the anonymous group (if applicable)
- `TG_EVO_BOT_OPENAI_API_KEY`: OpenAI API key

### Topics Management
- `TG_EVO_BOT_CLOSED_TOPICS_IDS`: Comma-separated list of topic IDs that closed for chatting
- `TG_EVO_BOT_FORWARDING_TOPIC_ID`: ID of the topic where forwarded replies will be sent (0 for General topic)
- `TG_EVO_BOT_TOOL_TOPIC_ID`: Topic ID for the AI tools database
- `TG_EVO_BOT_CONTENT_TOPIC_ID`: Topic ID for the content topic

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
set TG_EVO_BOT_ANONYMOUS_USER_ID=anonymous_user_id

# Topics Management
set TG_EVO_BOT_CLOSED_TOPICS_IDS=topic_id_1,topic_id_2,topic_id_3
set TG_EVO_BOT_FORWARDING_TOPIC_ID=forwarding_topic_id
set TG_EVO_BOT_TOOL_TOPIC_ID=tool_topic_id
set TG_EVO_BOT_CONTENT_TOPIC_ID=content_topic_id

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

## Features

- Deletes non-admin messages in closed threads (read-only threads)
- Forwards replies from closed threads to the forwarding thread (usualy it is General topic)
- Deletes join/leave messages in all threads
- Forwards messages to direct chat on request (command: `/save`)
- Provides help information (command: `/help`)
- AI-powered tool search functionality (command: `/tool`)
  - Searches through a database of AI tools
  - Provides relevant tool recommendations based on user queries
- AI-powered content search functionality (command: `/content`)
  - Searches through messages in the designated content topic
  - Retrieves relevant information based on user queries
- Automatic daily chat summarization
  - Collects messages from monitored chats
  - Generates a daily summary of chat activities
  - Posts summaries to a designated chat at a configured time
  - Supports manual triggering via `/summarize` command (admin-only, uses Telegram's permission system)
- Dynamic prompting template management
  - Flexible configuration of AI prompting templates stored in the database
  - Allows customization of AI behaviors and responses for summarization, content, and tool features
  - Templates can be updated without code changes

For more details on bot usage, use the `/help` command in the bot chat.

## Database Schema

The bot uses a PostgreSQL database with the following schema that is automatically initialized on startup:

### Tables

1. **messages** - Stores chat messages for summarization
   - `id`: Serial primary key
   - `topic_id`: Topic ID where the message was sent
   - `message_id`: Telegram message ID
   - `reply_to_message_id`: ID of the message being replied to (if applicable)
   - `user_id`: Telegram user ID of the sender
   - `username`: Username of the sender
   - `message_text`: Message content
   - `created_at`: Timestamp when the message was sent

2. **tg_sessions** - Stores Telegram User Client session data
   - `id`: Session ID (primary key)
   - `data`: Session data in binary format
   - `updated_at`: Timestamp when the session was last updated

3. **prompting_templates** - Stores AI prompting templates
   - `template_key`: Template identifier (primary key)
   - `template_text`: The actual template content

The database connection is configured using the `TG_EVO_BOT_DB_CONNECTION` environment variable.

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
go test github.com/it-beard/evo-bot-go/internal/handlers/privatehandlers
```

Or navigate to the package directory and run:

```shell
cd internal/handlers/privatehandlers
go test
```

### Run a Specific Test

To run a specific test function:

```shell
go test -run TestHelpHandler_Name github.com/it-beard/evo-bot-go/internal/handlers/privatehandlers
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
