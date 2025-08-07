# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Commands

### Development
- **Run the bot**: `go run main.go`
- **Build executable**: `go build main.go`
- **Update dependencies**: `go mod tidy`
- **Build for Windows**: `GOOS=windows GOARCH=amd64 go build -o bot.exe`

### Testing
- **Run all tests**: `go test ./...`
- **Run tests with verbose output**: `go test -v ./...`
- **Run tests in specific package**: `go test evo-bot-go/internal/handlers/privatehandlers`
- **Run specific test**: `go test -run TestHelpHandler_Name evo-bot-go/internal/handlers/privatehandlers`
- **Test coverage**: `go test -cover ./...`
- **Coverage report**: `go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out`
- **Race detection**: `go test -race ./...`
- **Enhanced test output**: `go run gotest.tools/gotestsum@latest --format pkgname --format-icons hivis`

## High-Level Architecture

This is a Telegram bot for the Evocoders Club built in Go with a layered architecture:

### Core Components

**main.go** → Entry point that initializes configuration, OpenAI client, and bot client with graceful shutdown

**internal/bot/bot.go** → Central bot orchestration with dependency injection pattern. Contains `TgBotClient` struct that manages:
- Bot instance and dispatcher setup
- Database connection and repository initialization  
- Service layer initialization (message sending, permissions, profiles, random coffee, summarization)
- Handler registration for all chat types (admin, group, private)
- Scheduled task management (daily summaries, random coffee polls/pairing)

### Layer Structure

**Configuration** (`internal/config/`) → Environment variable loading and validation for all bot features

**Database** (`internal/database/`) → PostgreSQL connection with migration system. Auto-initializes schema on startup

**Repositories** (`internal/database/repositories/`) → Data access layer for users, profiles, events, random coffee polls/pairs, prompting templates

**Services** (`internal/services/`) → Business logic layer including:
- `RandomCoffeeService` → Weekly poll creation and smart pairing with historical consideration
- `ProfileService` → User profile management and publishing
- `SummarizationService` → AI-powered daily chat summarization
- `PermissionsService` → Admin/user authorization
- `MessageSenderService` → Centralized message sending

**Handlers** (`internal/handlers/`) → Telegram update processing organized by context:
- `adminhandlers/` → Admin-only commands (event management, test handlers)
- `grouphandlers/` → Group chat moderation (thread management, join/leave cleanup)
- `privatehandlers/` → Private commands (AI tools, content search, profile management)

**Tasks** (`internal/tasks/`) → Scheduled background jobs for daily summaries and weekly random coffee automation

### Key Features Architecture

**Random Coffee System**: Weekly automated polls → participant tracking → smart pairing algorithm considering past pairings → automated announcements

**AI Integration**: OpenAI client for multiple use cases - content search (`/content`), tool search (`/tool`), member intro search (`/intro`), and daily chat summarization

**Database Schema**: PostgreSQL with migrations supporting users, profiles, events, topics, random coffee polls/participants/pairs, and message storage for summarization

### Handler Dependencies
All handlers receive a `HandlerDependencies` struct containing necessary services and repositories, enabling clean separation of concerns and easy testing.