# Configuration Parameters

<cite>
**Referenced Files in This Document**   
- [config.go](file://internal/config/config.go)
- [main.go](file://main.go)
- [README.md](file://README.md)
- [daily_summarization_task.go](file://internal/tasks/daily_summarization_task.go)
- [random_coffee_poll_task.go](file://internal/tasks/random_coffee_poll_task.go)
- [random_coffee_pairs_task.go](file://internal/tasks/random_coffee_pairs_task.go)
</cite>

## Table of Contents
1. [Introduction](#introduction)
2. [Configuration Loading Mechanism](#configuration-loading-mechanism)
3. [Basic Bot Configuration](#basic-bot-configuration)
4. [Topics Management](#topics-management)
5. [Database and Summarization Configuration](#database-and-summarization-configuration)
6. [Random Coffee Feature Configuration](#random-coffee-feature-configuration)
7. [Configuration Validation and Error Handling](#configuration-validation-and-error-handling)
8. [Scheduled Tasks Configuration](#scheduled-tasks-configuration)
9. [Environment-Specific Configuration Examples](#environment-specific-configuration-examples)
10. [Security Best Practices](#security-best-practices)

## Introduction

The evocoders-bot-go application utilizes a comprehensive configuration system based on environment variables to control various aspects of its functionality. This document provides detailed reference material for all configuration parameters used in the application, including their purpose, format, default values, and impact on system behavior.

The configuration system is centered around the Config struct defined in config.go, which is populated from environment variables during application startup. The bot leverages these configuration parameters to manage Telegram bot operations, database connections, AI integrations, and feature-specific behaviors such as daily summarization and random coffee meetings.

**Section sources**
- [config.go](file://internal/config/config.go#L11-L43)
- [main.go](file://main.go#L15-L20)

## Configuration Loading Mechanism

The configuration loading mechanism in evocoders-bot-go follows a structured approach to initialize the application settings from environment variables. The LoadConfig function in config.go is responsible for reading environment variables and populating the Config struct with appropriate values.

During application startup in main.go, the LoadConfig function is called before initializing other components. This ensures that all services and handlers have access to the configuration data they need to function properly. The configuration is passed as a dependency to various components, promoting loose coupling and testability.

The loading process includes type conversion for non-string values (such as integers, booleans, and time values), with appropriate error handling for invalid formats. Required configuration parameters trigger application startup failure if not provided, while optional parameters have sensible defaults.

```mermaid
flowchart TD
Start([Application Start]) --> LoadConfig["LoadConfig()"]
LoadConfig --> ParseBasic["Parse Basic Bot Configuration"]
ParseBasic --> ParseTopics["Parse Topics Management"]
ParseTopics --> ParseSummarization["Parse Summarization Settings"]
ParseSummarization --> ParseRandomCoffee["Parse Random Coffee Settings"]
ParseRandomCoffee --> Validate["Validate Configuration"]
Validate --> |Success| ReturnConfig["Return Config Struct"]
Validate --> |Failure| Error["Return Error"]
ReturnConfig --> InitializeComponents["Initialize Application Components"]
Error --> Exit["Exit with Error"]
```

**Diagram sources**
- [config.go](file://internal/config/config.go#L45-L318)
- [main.go](file://main.go#L15-L20)

**Section sources**
- [config.go](file://internal/config/config.go#L45-L318)
- [main.go](file://main.go#L15-L20)

## Basic Bot Configuration

The basic bot configuration parameters control fundamental aspects of the Telegram bot's operation and integration with external services.

### Telegram Bot Credentials
- **TG_EVO_BOT_TOKEN**: Authentication token for the Telegram bot (required)
  - Format: String
  - Purpose: Authenticates the bot with Telegram's API
  - Impact: Without this token, the bot cannot connect to Telegram

- **TG_EVO_BOT_SUPERGROUP_CHAT_ID**: Chat ID of the Telegram supergroup (required)
  - Format: Integer (64-bit)
  - Purpose: Identifies the main group where the bot operates
  - Impact: Determines which chat the bot will monitor and interact with

- **TG_EVO_BOT_ADMIN_USER_ID**: User ID of the administrator account (optional)
  - Format: Integer (64-bit)
  - Default: Not set (no admin notifications)
  - Purpose: Enables admin-specific notifications and commands
  - Impact: Admin receives notifications about new topics and can use admin commands

### OpenAI Integration
- **TG_EVO_BOT_OPENAI_API_KEY**: API key for OpenAI services (required)
  - Format: String
  - Purpose: Authenticates requests to OpenAI's API for AI-powered features
  - Impact: Without this key, AI features like search and summarization will not work

**Section sources**
- [config.go](file://internal/config/config.go#L13-L16)

## Topics Management

The topics management configuration parameters control how the bot interacts with different topics in the Telegram supergroup.

### Closed Topics
- **TG_EVO_BOT_CLOSED_TOPICS_IDS**: Comma-separated list of topic IDs closed for chatting
  - Format: Comma-separated integers (e.g., "1,2,3")
  - Default: Empty (no closed topics)
  - Purpose: Identifies topics where non-admin messages should be deleted
  - Impact: Enables moderation in specific topics

### Topic Routing
- **TG_EVO_BOT_FORWARDING_TOPIC_ID**: Topic ID for forwarding replies from closed threads
  - Format: Integer
  - Default: Not set
  - Purpose: Specifies where replies from closed topics should be forwarded
  - Impact: Maintains conversation flow while enforcing topic rules

- **TG_EVO_BOT_TOOL_TOPIC_ID**: Topic ID for AI tools database
  - Format: Integer
  - Purpose: Identifies the topic containing AI tool information
  - Impact: Enables /tool command to search in the correct topic

- **TG_EVO_BOT_CONTENT_TOPIC_ID**: Topic ID for general content
  - Format: Integer
  - Purpose: Identifies the topic for general content searches
  - Impact: Enables /content command to search in the correct topic

- **TG_EVO_BOT_INTRO_TOPIC_ID**: Topic ID for club introductions
  - Format: Integer
  - Purpose: Identifies the topic for member introductions
  - Impact: Enables /intro command to search member information

- **TG_EVO_BOT_ANNOUNCEMENT_TOPIC_ID**: Topic ID for announcements
  - Format: Integer
  - Purpose: Identifies the topic for official announcements
  - Impact: May be used for targeted messaging or monitoring

**Section sources**
- [config.go](file://internal/config/config.go#L18-L24)

## Database and Summarization Configuration

These parameters control the database connection and daily summarization features.

### Database Connection
- **TG_EVO_BOT_DB_CONNECTION**: PostgreSQL connection string (required)
  - Format: Standard PostgreSQL connection string (e.g., "postgresql://user:password@localhost:5432/dbname")
  - Purpose: Connects to the PostgreSQL database for data persistence
  - Impact: Without this, the bot cannot store or retrieve data

### Daily Summarization
- **TG_EVO_BOT_MONITORED_TOPICS_IDS**: Comma-separated list of topic IDs to monitor for summarization (required)
  - Format: Comma-separated integers
  - Purpose: Specifies which topics' content should be included in daily summaries
  - Impact: Determines the scope of the summarization feature

- **TG_EVO_BOT_SUMMARY_TOPIC_ID**: Topic ID where daily summaries will be posted (required)
  - Format: Integer
  - Purpose: Specifies the destination for daily summary messages
  - Impact: Without this, summaries cannot be posted

- **TG_EVO_BOT_SUMMARY_TIME**: Time to run daily summary in 24-hour format
  - Format: "HH:MM" (24-hour format, UTC)
  - Default: "03:00" (3:00 AM UTC)
  - Purpose: Schedules when the daily summary should be generated
  - Impact: Controls the timing of automated summary generation

- **TG_EVO_BOT_SUMMARIZATION_TASK_ENABLED**: Enables or disables the daily summarization task
  - Format: Boolean ("true" or "false")
  - Default: true
  - Purpose: Allows temporary disabling of the summarization feature
  - Impact: When disabled, no automated summaries are generated

```mermaid
classDiagram
class DailySummarizationTask {
+config *Config
+summarizationService *SummarizationService
+stop chan struct{}
+Start()
+Stop()
+run()
+calculateNextRun() time.Time
}
class Config {
+DBConnection string
+MonitoredTopicsIDs []int
+SummaryTopicID int
+SummaryTime time.Time
+SummarizationTaskEnabled bool
}
class SummarizationService {
+RunDailySummarization(ctx context.Context, sendToDM bool) error
}
DailySummarizationTask --> Config : "uses"
DailySummarizationTask --> SummarizationService : "delegates"
```

**Diagram sources**
- [config.go](file://internal/config/config.go#L26-L32)
- [daily_summarization_task.go](file://internal/tasks/daily_summarization_task.go#L11-L17)

**Section sources**
- [config.go](file://internal/config/config.go#L26-L32)
- [daily_summarization_task.go](file://internal/tasks/daily_summarization_task.go#L11-L100)

## Random Coffee Feature Configuration

These parameters control the random coffee meeting automation features.

### General Settings
- **TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID**: Topic ID for random coffee polls and pairs (required)
  - Format: Integer
  - Purpose: Specifies where random coffee polls and pair announcements are posted
  - Impact: Centralizes random coffee interactions in one topic

### Poll Scheduling
- **TG_EVO_BOT_RANDOM_COFFEE_POLL_TASK_ENABLED**: Enables or disables the weekly coffee poll task
  - Format: Boolean
  - Default: true
  - Purpose: Allows temporary disabling of poll automation
  - Impact: When disabled, no automated polls are sent

- **TG_EVO_BOT_RANDOM_COFFEE_POLL_TIME**: Time to send the weekly coffee poll (UTC)
  - Format: "HH:MM" (24-hour format, UTC)
  - Default: "14:00" (2:00 PM UTC)
  - Purpose: Schedules when the participation poll is sent
  - Impact: Determines when members are asked about availability

- **TG_EVO_BOT_RANDOM_COFFEE_POLL_DAY**: Day of the week to send the poll
  - Format: Day name in lowercase (e.g., "friday", "monday")
  - Default: "friday"
  - Purpose: Specifies which day of the week the poll is sent
  - Impact: Weekly timing of the participation poll

### Pairs Generation
- **TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TASK_ENABLED**: Enables or disables automatic pairs generation
  - Format: Boolean
  - Default: true
  - Purpose: Allows temporary disabling of pair generation
  - Impact: When disabled, no automated pair announcements are made

- **TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TIME**: Time to generate and announce coffee pairs (UTC)
  - Format: "HH:MM" (24-hour format, UTC)
  - Default: "12:00" (12:00 PM UTC)
  - Purpose: Schedules when pairs are generated and announced
  - Impact: Timing of pair announcements

- **TG_EVO_BOT_RANDOM_COFFEE_PAIRS_DAY**: Day of the week to generate pairs
  - Format: Day name in lowercase
  - Default: "monday"
  - Purpose: Specifies which day of the week pairs are generated
  - Impact: Weekly timing of pair generation

```mermaid
sequenceDiagram
participant Task as RandomCoffeePollTask
participant Service as RandomCoffeeService
participant Bot as Telegram Bot
participant Members as Club Members
Task->>Task : calculateNextRun()
loop Every minute
Task->>Task : Check current time
alt Time to run
Task->>Service : SendPoll()
Service->>Bot : Create poll message
Bot->>Members : Send poll to random coffee topic
Members->>Bot : Vote on poll
Bot->>Service : Collect votes
end
end
```

**Diagram sources**
- [config.go](file://internal/config/config.go#L34-L43)
- [random_coffee_poll_task.go](file://internal/tasks/random_coffee_poll_task.go#L11-L17)

**Section sources**
- [config.go](file://internal/config/config.go#L34-L43)
- [random_coffee_poll_task.go](file://internal/tasks/random_coffee_poll_task.go#L11-L106)
- [random_coffee_pairs_task.go](file://internal/tasks/random_coffee_pairs_task.go#L11-L102)

## Configuration Validation and Error Handling

The configuration system implements comprehensive validation to ensure the application starts with valid settings.

### Required Parameters
The following parameters are required and will cause the application to fail startup if not provided:
- TG_EVO_BOT_TOKEN
- TG_EVO_BOT_SUPERGROUP_CHAT_ID
- TG_EVO_BOT_OPENAI_API_KEY
- TG_EVO_BOT_DB_CONNECTION
- TG_EVO_BOT_MONITORED_TOPICS_IDS
- TG_EVO_BOT_SUMMARY_TOPIC_ID
- TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID

### Type Validation
The system validates the type of each configuration parameter:
- Integer parameters are parsed using strconv.Atoi or strconv.ParseInt
- Boolean parameters are parsed using strconv.ParseBool
- Time parameters are parsed using time.Parse("15:04", value)
- Day of week parameters are validated against valid day names

### Error Handling
When invalid configuration is detected, the LoadConfig function returns a descriptive error message that includes:
- The name of the problematic environment variable
- The invalid value provided
- The expected format or valid values
- Specific guidance for correction

The main function handles these errors by logging the error and terminating the application, preventing operation with invalid configuration.

**Section sources**
- [config.go](file://internal/config/config.go#L45-L318)

## Scheduled Tasks Configuration

The configuration parameters directly control the scheduling of automated tasks in the application.

### Task Architecture
The application uses a task scheduling system where each automated feature is managed by a dedicated task struct that:
- Reads configuration values during initialization
- Calculates next execution time based on configuration
- Runs in a separate goroutine with periodic checking
- Can be gracefully stopped

### Scheduling Logic
The scheduling logic for all tasks follows a similar pattern:
1. Check if the task is enabled in configuration
2. Calculate the next run time based on configured day and time
3. Use a ticker to check the current time periodically
4. Execute the task when the current time exceeds the next run time
5. Calculate the subsequent run time and continue monitoring

This approach ensures reliable execution while allowing for configuration-driven scheduling without requiring external cron systems.

```mermaid
flowchart TD
TaskStart([Task Start]) --> IsEnabled{"Task Enabled?"}
IsEnabled --> |No| LogDisabled["Log: Task Disabled"]
IsEnabled --> |Yes| CalculateNext["Calculate Next Run Time"]
CalculateNext --> LogNext["Log Next Run Time"]
LogNext --> CreateTicker["Create Minute Ticker"]
CreateTicker --> CheckTime["Check Current Time"]
CheckTime --> |Time to Run| ExecuteTask["Execute Task"]
ExecuteTask --> Recalculate["Recalculate Next Run"]
Recalculate --> CheckTime
CheckTime --> |Not Time| Wait["Wait for Next Check"]
Wait --> CheckTime
```

**Diagram sources**
- [daily_summarization_task.go](file://internal/tasks/daily_summarization_task.go#L50-L100)
- [random_coffee_poll_task.go](file://internal/tasks/random_coffee_poll_task.go#L50-L106)
- [random_coffee_pairs_task.go](file://internal/tasks/random_coffee_pairs_task.go#L50-L102)

**Section sources**
- [daily_summarization_task.go](file://internal/tasks/daily_summarization_task.go#L50-L100)
- [random_coffee_poll_task.go](file://internal/tasks/random_coffee_poll_task.go#L50-L106)
- [random_coffee_pairs_task.go](file://internal/tasks/random_coffee_pairs_task.go#L50-L102)

## Environment-Specific Configuration Examples

### Development Environment
```bash
# Basic Bot Configuration 
export TG_EVO_BOT_TOKEN=dev_bot_token_123
export TG_EVO_BOT_OPENAI_API_KEY=dev_openai_key_456
export TG_EVO_BOT_SUPERGROUP_CHAT_ID=-1001234567890
export TG_EVO_BOT_ADMIN_USER_ID=123456789

# Topics Management
export TG_EVO_BOT_CLOSED_TOPICS_IDS=1,2,3
export TG_EVO_BOT_FORWARDING_TOPIC_ID=0
export TG_EVO_BOT_TOOL_TOPIC_ID=4
export TG_EVO_BOT_CONTENT_TOPIC_ID=5
export TG_EVO_BOT_INTRO_TOPIC_ID=6
export TG_EVO_BOT_ANNOUNCEMENT_TOPIC_ID=7

# Daily Summarization Feature
export TG_EVO_BOT_DB_CONNECTION="postgresql://dev_user:dev_password@localhost:5432/evocoders_dev"
export TG_EVO_BOT_MONITORED_TOPICS_IDS=0,2
export TG_EVO_BOT_SUMMARY_TOPIC_ID=3
export TG_EVO_BOT_SUMMARY_TIME=03:00
export TG_EVO_BOT_SUMMARIZATION_TASK_ENABLED=true

# Random Coffee Feature
export TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID=8
export TG_EVO_BOT_RANDOM_COFFEE_POLL_TASK_ENABLED=true
export TG_EVO_BOT_RANDOM_COFFEE_POLL_TIME=14:00
export TG_EVO_BOT_RANDOM_COFFEE_POLL_DAY=friday
export TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TASK_ENABLED=true
export TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TIME=12:00
export TG_EVO_BOT_RANDOM_COFFEE_PAIRS_DAY=monday
```

### Production Environment
```bash
# Basic Bot Configuration 
export TG_EVO_BOT_TOKEN=prod_bot_token_987
export TG_EVO_BOT_OPENAI_API_KEY=prod_openai_key_654
export TG_EVO_BOT_SUPERGROUP_CHAT_ID=-1009876543210
export TG_EVO_BOT_ADMIN_USER_ID=987654321

# Topics Management
export TG_EVO_BOT_CLOSED_TOPICS_IDS=10,20,30
export TG_EVO_BOT_FORWARDING_TOPIC_ID=0
export TG_EVO_BOT_TOOL_TOPIC_ID=40
export TG_EVO_BOT_CONTENT_TOPIC_ID=50
export TG_EVO_BOT_INTRO_TOPIC_ID=60
export TG_EVO_BOT_ANNOUNCEMENT_TOPIC_ID=70

# Daily Summarization Feature
export TG_EVO_BOT_DB_CONNECTION="postgresql://prod_user:prod_password@db-server:5432/evocoders_prod"
export TG_EVO_BOT_MONITORED_TOPICS_IDS=0,20
export TG_EVO_BOT_SUMMARY_TOPIC_ID=30
export TG_EVO_BOT_SUMMARY_TIME=03:00
export TG_EVO_BOT_SUMMARIZATION_TASK_ENABLED=true

# Random Coffee Feature
export TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID=80
export TG_EVO_BOT_RANDOM_COFFEE_POLL_TASK_ENABLED=true
export TG_EVO_BOT_RANDOM_COFFEE_POLL_TIME=14:00
export TG_EVO_BOT_RANDOM_COFFEE_POLL_DAY=friday
export TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TASK_ENABLED=true
export TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TIME=12:00
export TG_EVO_BOT_RANDOM_COFFEE_PAIRS_DAY=monday
```

**Section sources**
- [README.md](file://README.md#L200-L273)

## Security Best Practices

### Sensitive Data Management
- Never commit environment variables to version control
- Use .env files for development (add to .gitignore)
- Use environment-specific configuration management
- Rotate API keys periodically
- Use least-privilege principles for database credentials

### Deployment Recommendations
- Store configuration in environment variables or secure configuration management systems
- Use different API keys for different environments
- Implement monitoring for configuration changes
- Regularly audit configuration settings
- Use encrypted storage for sensitive configuration data

### Access Control
- Restrict access to configuration management systems
- Implement role-based access control for configuration changes
- Maintain audit logs of configuration changes
- Use separate credentials for different environments
- Implement multi-factor authentication for configuration management access

**Section sources**
- [README.md](file://README.md#L200-L273)
- [config.go](file://internal/config/config.go#L45-L318)