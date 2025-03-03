# Telegram Bot

This project contains a Telegram bot implemented in Go. Tested on Windows 11.

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
- `TG_EVO_BOT_MAIN_CHAT_ID`: Chat ID of your Supergroup
- `TG_EVO_BOT_OPENAI_API_KEY`: OpenAI API key

### Chat Management
- `TG_EVO_BOT_CLOSED_THREADS_IDS`: Comma-separated list of thread IDs that closed for chatting
- `TG_EVO_BOT_ANONYMOUS_USER_ID`: User ID for the account used on the anonymous group (if applicable)
- `TG_EVO_BOT_FORWARDING_THREAD_ID`: ID of the thread where forwarded replies will be sent (0 for General topic)
- `TG_EVO_BOT_TOOL_TOPIC_ID`: Topic ID for the AI tools database
- `TG_EVO_BOT_CONTENT_TOPIC_ID`: Topic ID for the content topic

### Telegram User Client
- `TG_EVO_BOT_TGUSERCLIENT_APPID`: Telegram API App ID
- `TG_EVO_BOT_TGUSERCLIENT_APPHASH`: Telegram API App Hash
- `TG_EVO_BOT_TGUSERCLIENT_PHONENUMBER`: Phone number for Telegram user client
- `TG_EVO_BOT_TGUSERCLIENT_2FAPASS`: Two-factor authentication password for Telegram user client (if using 2FA)
- `TG_EVO_BOT_TGUSERCLIENT_SESSION_TYPE`: Session type for Telegram User Client. Set it to `file` to enable file storage (using `session.json`), otherwise it defaults to in-memory session storage.

### Daily Summarization Feature
- `TG_EVO_BOT_DB_CONNECTION`: PostgreSQL connection string (e.g., `postgresql://user:password@localhost:5432/dbname`)
- `TG_EVO_BOT_MONITORED_CHAT_IDS`: Comma-separated list of chat IDs to monitor for summarization
- `TG_EVO_BOT_SUMMARY_CHAT_ID`: Chat ID where daily summaries will be posted
- `TG_EVO_BOT_SUMMARY_TIME`: Time to run daily summary in 24-hour format (e.g., `03:00` for 3 AM)

On Windows, you can set the environment variables using the following commands in Command Prompt:

```shell
# Telegram Bot API settings
set TG_EVO_BOT_TOKEN=your_bot_token_here

# OpenAI API settings
set TG_EVO_BOT_OPENAI_API_KEY=your_openai_api_key_here

# Telegram group settings
set TG_EVO_BOT_MAIN_CHAT_ID=tool_chat_id
set TG_EVO_BOT_CLOSED_THREADS_IDS=thread_id_1,thread_id_2,thread_id_3
set TG_EVO_BOT_ANONYMOUS_USER_ID=anonymous_user_id
set TG_EVO_BOT_FORWARDING_THREAD_ID=forwarding_thread_id
set TG_EVO_BOT_TOOL_TOPIC_ID=tool_topic_id

# Telegram User Client API settings
set TG_EVO_BOT_TGUSERCLIENT_APPID=your_app_id
set TG_EVO_BOT_TGUSERCLIENT_APPHASH=your_app_hash
set TG_EVO_BOT_TGUSERCLIENT_PHONENUMBER=your_phone_number
set TG_EVO_BOT_TGUSERCLIENT_2FAPASS=your_2fa_password
set TG_EVO_BOT_TGUSERCLIENT_SESSION_TYPE=file

# Daily Summarization settings
set TG_EVO_BOT_DB_CONNECTION=postgresql://user:password@localhost:5432/dbname
set TG_EVO_BOT_MONITORED_CHAT_IDS=-1001234567890,-1001987654321
set TG_EVO_BOT_SUMMARY_CHAT_ID=-1001234567890
set TG_EVO_BOT_SUMMARY_TIME=03:00
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
  - Supports Russian language queries and responses
- Daily chat summarization
  - Collects messages from monitored chats
  - Uses RAG (Retrieval-Augmented Generation) to find the most relevant messages
  - Generates a daily summary of chat activities
  - Posts summaries to a designated chat at a configured time
  - Supports manual triggering via `/summarize` command (admin-only, uses Telegram's permission system)

For more details on bot usage, use the `/help` command in the bot chat.
