# Telegram Bot

This project contains a Telegram bot implemented in Go. The bot is designed to be a simple and efficient bot that can be used for a variety of purposes.

## Building the Executable

To build the executable for Windows, use the following command:

```bash
GOOS=windows GOARCH=amd64 go build -o bot.exe
```

This command will create a Windows executable named `bot.exe` that can run on 64-bit Windows systems.

## Running the Bot

Before running the bot, make sure to set the `TG_EVO_BOT_TOKEN` environment variable with your Telegram bot token.

On Windows, you can set the environment variable using the following command in Command Prompt:

```
set TG_EVO_BOT_TOKEN=your_bot_token_here
```

Then run the executable