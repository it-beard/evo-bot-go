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

- `TG_EVO_BOT_TOKEN`: Your Telegram bot token
- `TG_EVO_BOT_MAIN_CHAT_ID`: Chat ID of your Supergroup
- `TG_EVO_BOT_OPENAI_API_KEY`: OpenAI API key
- `TG_EVO_BOT_CLOSED_THREADS_IDS`: Comma-separated list of thread IDs that closed for chatting
- `TG_EVO_BOT_ANONYMOUS_USER_ID`: User ID for the that sitting on the anonymous group account (if you are using anonymous group account)
- `TG_EVO_BOT_FORWARDING_THREAD_ID`: ID of the thread where forwarded from closed threads replies will be sent (0 for General topic)
- `TG_EVO_BOT_TOOL_TOPIC_ID`: Topic ID for the AI tools database
- `TG_EVO_BOT_TGUSERCLIENT_APPID`: Telegram API App ID
- `TG_EVO_BOT_TGUSERCLIENT_APPHASH`: Telegram API App Hash
- `TG_EVO_BOT_TGUSERCLIENT_PHONENUMBER`: Phone number for Telegram user client
- `TG_EVO_BOT_TGUSERCLIENT_2FAPASS`: Two-factor authentication password for Telegram user client (if you are using 2FA)

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

For more details on bot usage, use the `/help` command in the bot chat.
