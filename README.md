# Telegram Bot

This project contains a Telegram bot implemented in Go. Tested on Windows 11.

## Building the Executable

To build the executable for Windows, use the following command:

```shell
GOOS=windows GOARCH=amd64 go build -o bot.exe
```

This command will create a Windows executable named `bot.exe` that can run on 64-bit Windows systems.

## Running the Bot

Before running the bot, make sure to set the following environment variables:

1. `TG_EVO_BOT_TOKEN`: Your Telegram bot token
2. `TG_EVO_BOT_CLOSED_THREADS_IDS`: Comma-separated list of thread IDs that closed for chatting
3. `TG_EVO_BOT_ANONYMOUS_USER_ID`: User ID for the that sitting on the anonymous group account (if you are using anonymous group account)
4. `TG_EVO_BOT_FORWARDING_THREAD_ID`: ID of the thread where forwarded from closed threads replies will be sent (0 for General topic)


On Windows, you can set the environment variables using the following commands in Command Prompt:

```shell
set TG_EVO_BOT_TOKEN=your_bot_token_here
set TG_EVO_BOT_CLOSED_THREADS_IDS=thread_id_1,thread_id_2,thread_id_3
set TG_EVO_BOT_ANONYMOUS_USER_ID=anonymous_user_id
set TG_EVO_BOT_FORWARDING_THREAD_ID=forwarding_thread_id
```

Then run the executable.

## Features

- Deletes non-admin messages in closed threads (read-only threads)
- Forwards replies from closed threads to the forwarding thread (usualy it is General topic)
- Deletes join/leave messages in all threads
- Forwards messages to direct chat on request (command: `/save`)
- Provides help information (command: `/help`)

For more details on bot usage, use the `/help` command in the bot chat.