# Configuration Guide

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
2. [Configuration Architecture](#configuration-architecture)
3. [Configuration Parameters](#configuration-parameters)
4. [Basic Bot Settings](#basic-bot-settings)
5. [Topics Management](#topics-management)
6. [Telegram User Client](#telegram-user-client)
7. [Daily Summarization](#daily-summarization)
8. [Random Coffee System](#random-coffee-system)
9. [Common Issues and Solutions](#common-issues-and-solutions)
10. [Best Practices](#best-practices)

## Introduction

The evocoders-bot-go application uses a comprehensive environment variable-based configuration system to manage all aspects of its functionality. This Configuration Guide provides detailed information about the configuration architecture, parameters, and best practices for setting up the bot. The configuration system is implemented in the `config.go` file and is responsible for loading settings from environment variables, validating their values, and providing default values where appropriate.

The configuration system is designed to be flexible and easy to use, allowing administrators to customize the bot's behavior without modifying code. All configuration is managed through environment variables, which are loaded at application startup and validated for correctness.

**Section sources**
- [config.go](file://internal/config/config.go#L1-L340)
- [main.go](file://main.go#L1-L54)

## Configuration Architecture

The configuration system in evocoders-bot-go follows a structured approach to manage application settings. The `Config` struct in `config.go` organizes settings into logical groups based on functionality, making it easy to understand and manage related configuration options.

```mermaid
classDiagram
class Config {
+string BotToken
+int64 SuperGroupChatID
+string OpenAIAPIKey
+int64 AdminUserID
+[]int ClosedTopicsIDs
+int ForwardingTopicID
+int ToolTopicID
+int ContentTopicID
+int AnnouncementTopicID
+int IntroTopicID
+int TGUserClientAppID
+string TGUserClientAppHash
+string TGUserClientPhoneNumber
+string TGUserClient2FAPass
+string TGUserClientSessionType
+string DBConnection
+[]int MonitoredTopicsIDs
+int SummaryTopicID
+time.Time SummaryTime
+bool SummarizationTaskEnabled
+int RandomCoffeeTopicID
+bool RandomCoffeePollTaskEnabled
+time.Time RandomCoffeePollTime
+time.Weekday RandomCoffeePollDay
+bool RandomCoffeePairsTaskEnabled
+time.Time RandomCoffeePairsTime
+time.Weekday RandomCoffeePairsDay
+LoadConfig() (*Config, error)
}
```

**Diagram sources**
- [config.go](file://internal/config/config.go#L11-L50)

The `LoadConfig()` function is responsible for reading environment variables, parsing their values into appropriate data types, and performing validation. If required variables are missing or have invalid values, the function returns an error, preventing the application from starting with incorrect configuration.

The configuration system implements several key principles:
- **Validation**: All required environment variables are validated for presence and correct format
- **Default values**: Optional parameters have sensible defaults when not specified
- **Type safety**: String values are converted to appropriate types (int, bool, time, etc.)
- **Error reporting**: Clear error messages indicate which configuration parameter failed and why

**Section sources**
- [config.go](file://internal/config/config.go#L57-L340)

## Configuration Parameters

This section documents all configuration parameters used by the evocoders-bot-go application, organized by functional area. Each parameter is listed with its environment variable name, expected format, valid values, and purpose.

### Parameter Format and Types

The configuration system supports several data types, each with specific formatting requirements:

- **String**: Plain text values (e.g., API keys, phone numbers)
- **Integer**: Whole numbers (e.g., topic IDs, chat IDs)
- **Boolean**: `true` or `false` values
- **Time**: 24-hour format in `HH:MM` (e.g., `14:30` for 2:30 PM)
- **Weekday**: Day of the week as lowercase text (e.g., `monday`, `friday`)
- **Comma-separated list**: Multiple integer values separated by commas (e.g., `1,2,3`)

### Required vs. Optional Parameters

Parameters are categorized as required or optional based on whether the application can function without them. Required parameters will cause the application to fail to start if not provided, while optional parameters use default values when not specified.

**Section sources**
- [config.go](file://internal/config/config.go#L1-L340)

## Basic Bot Settings

The basic bot settings configure the core functionality of the Telegram bot, including authentication, API access, and administrative controls.

### Core Authentication

| Parameter | Environment Variable | Type | Required | Default | Description |
|---------|---------------------|------|----------|---------|-------------|
| Bot Token | `TG_EVO_BOT_TOKEN` | String | Yes | None | Telegram bot token obtained from BotFather |
| Supergroup Chat ID | `TG_EVO_BOT_SUPERGROUP_CHAT_ID` | Integer | Yes | None | ID of the Telegram supergroup where the bot operates |
| OpenAI API Key | `TG_EVO_BOT_OPENAI_API_KEY` | String | Yes | None | API key for accessing OpenAI services |
| Admin User ID | `TG_EVO_BOT_ADMIN_USER_ID` | Integer | No | None | Telegram user ID of the administrator (for notifications) |

```mermaid
flowchart TD
A["Application Start"] --> B["Load Environment Variables"]
B --> C{"Required Variables Present?"}
C --> |Yes| D["Parse and Validate Values"]
C --> |No| E["Return Configuration Error"]
D --> F{"Values Valid?"}
F --> |Yes| G["Initialize Configuration"]
F --> |No| H["Return Validation Error"]
G --> I["Start Bot Services"]
```

**Diagram sources**
- [config.go](file://internal/config/config.go#L57-L73)
- [main.go](file://main.go#L10-L15)

The bot token, supergroup chat ID, and OpenAI API key are required for the application to function. Without these values, the bot cannot authenticate with Telegram or access AI services. The admin user ID is optional and is used to send notifications about new topics and other administrative events.

**Section sources**
- [config.go](file://internal/config/config.go#L57-L73)

## Topics Management

The topics management configuration controls how the bot interacts with different topics in the Telegram supergroup, including read-only threads, forwarding rules, and specialized topic IDs.

### Topic Configuration Parameters

| Parameter | Environment Variable | Type | Required | Default | Description |
|---------|---------------------|------|----------|---------|-------------|
| Closed Topics IDs | `TG_EVO_BOT_CLOSED_TOPICS_IDS` | Comma-separated integers | No | None | List of topic IDs that are closed for general chatting |
| Forwarding Topic ID | `TG_EVO_BOT_FORWARDING_TOPIC_ID` | Integer | No | None | Topic ID where replies from closed threads are forwarded |
| Tool Topic ID | `TG_EVO_BOT_TOOL_TOPIC_ID` | Integer | No | None | Topic ID for AI tools database |
| Content Topic ID | `TG_EVO_BOT_CONTENT_TOPIC_ID` | Integer | No | None | Topic ID for content discussions |
| Announcement Topic ID | `TG_EVO_BOT_ANNOUNCEMENT_TOPIC_ID` | Integer | No | None | Topic ID for announcements |
| Intro Topic ID | `TG_EVO_BOT_INTRO_TOPIC_ID` | Integer | No | None | Topic ID for member introductions |

### Topic Management Workflow

```mermaid
flowchart LR
A["Message Received"] --> B{"In Closed Topic?"}
B --> |Yes| C{"Is Admin Message?"}
C --> |No| D["Delete Message"]
C --> |Yes| E["Allow Message"]
B --> |No| F{"Reply in Closed Thread?"}
F --> |Yes| G["Forward to Forwarding Topic"]
F --> |No| H["Process Normally"]
```

**Diagram sources**
- [config.go](file://internal/config/config.go#L79-L136)

The closed topics feature allows administrators to designate certain topics as read-only by setting the `TG_EVO_BOT_CLOSED_TOPICS_IDS` parameter. Messages in these topics from non-administrators are automatically deleted. The `TG_EVO_BOT_FORWARDING_TOPIC_ID` parameter specifies where replies to messages in closed threads should be forwarded, helping to keep discussions organized.

Other topic IDs designate specialized areas for different types of content, enabling the bot to route commands and searches to the appropriate context.

**Section sources**
- [config.go](file://internal/config/config.go#L79-L136)

## Telegram User Client

The Telegram User Client configuration enables the bot to use a separate Telegram account for specific operations that require user-level permissions.

### User Client Configuration Parameters

| Parameter | Environment Variable | Type | Required | Default | Description |
|---------|---------------------|------|----------|---------|-------------|
| App ID | `TG_EVO_BOT_TGUSERCLIENT_APPID` | Integer | No | None | Telegram API App ID for the user client |
| App Hash | `TG_EVO_BOT_TGUSERCLIENT_APPHASH` | String | No | None | Telegram API App Hash for the user client |
| Phone Number | `TG_EVO_BOT_TGUSERCLIENT_PHONENUMBER` | String | No | None | Phone number for the user client account |
| 2FA Password | `TG_EVO_BOT_TGUSERCLIENT_2FAPASS` | String | No | None | Two-factor authentication password if enabled |
| Session Type | `TG_EVO_BOT_TGUSERCLIENT_SESSION_TYPE` | String | No | memory | Session storage method (file, database, memory) |

The Telegram User Client allows the bot to perform actions that require user-level permissions, such as accessing certain chat features or using specific Telegram APIs. The client uses the provided App ID and App Hash to authenticate with Telegram's API.

The session type parameter determines how the client's authentication session is stored:
- **file**: Saves session data to `session.json`, preserving login between restarts
- **database**: Stores session in the PostgreSQL database (requires valid DB connection)
- **memory**: Keeps session in memory only (lost when bot restarts)

After initial setup, a verification code is sent to the Telegram app associated with the phone number, which must be entered in reverse using the `/code` command in the bot.

**Section sources**
- [config.go](file://internal/config/config.go#L146-L158)

## Daily Summarization

The daily summarization feature automatically generates summaries of chat activity and posts them to a designated topic at a scheduled time.

### Summarization Configuration Parameters

| Parameter | Environment Variable | Type | Required | Default | Description |
|---------|---------------------|------|----------|---------|-------------|
| Database Connection | `TG_EVO_BOT_DB_CONNECTION` | String | Yes | None | PostgreSQL connection string for message storage |
| Monitored Topics IDs | `TG_EVO_BOT_MONITORED_TOPICS_IDS` | Comma-separated integers | Yes | None | List of topic IDs to include in daily summaries |
| Summary Topic ID | `TG_EVO_BOT_SUMMARY_TOPIC_ID` | Integer | Yes | None | Topic ID where daily summaries are posted |
| Summary Time | `TG_EVO_BOT_SUMMARY_TIME` | Time (HH:MM) | No | 03:00 | Time of day to generate summaries (UTC) |
| Task Enabled | `TG_EVO_BOT_SUMMARIZATION_TASK_ENABLED` | Boolean | No | true | Whether to enable automatic daily summarization |

### Summarization Process Flow

```mermaid
sequenceDiagram
participant Scheduler
participant Task
participant Service
participant Database
Scheduler->>Task : Check time (every minute)
Task->>Task : Is it summary time?
Task->>Service : RunDailySummarization()
Service->>Database : Query messages from monitored topics
Database-->>Service : Return message data
Service->>Service : Generate AI-powered summary
Service->>Task : Return summary
Task->>Task : Post summary to summary topic
```

**Diagram sources**
- [config.go](file://internal/config/config.go#L161-L208)
- [daily_summarization_task.go](file://internal/tasks/daily_summarization_task.go#L1-L100)

The summarization process begins by collecting messages from the topics specified in `TG_EVO_BOT_MONITORED_TOPICS_IDS`. These messages are processed by the OpenAI service to generate a concise summary, which is then posted to the topic specified by `TG_EVO_BOT_SUMMARY_TOPIC_ID` at the time configured in `TG_EVO_BOT_SUMMARY_TIME`.

The task runs on a schedule determined by the `SummaryTime` parameter, checking every minute to see if it's time to generate a summary. By default, summaries are generated at 3:00 AM UTC, but this can be customized to any time of day.

**Section sources**
- [config.go](file://internal/config/config.go#L161-L208)
- [daily_summarization_task.go](file://internal/tasks/daily_summarization_task.go#L1-L100)

## Random Coffee System

The random coffee system facilitates weekly virtual meetings between community members by automatically creating pairs and managing participation through polls.

### Random Coffee Configuration Parameters

| Parameter | Environment Variable | Type | Required | Default | Description |
|---------|---------------------|------|----------|---------|-------------|
| Random Coffee Topic ID | `TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID` | Integer | Yes | None | Topic ID for random coffee polls and announcements |
| Poll Task Enabled | `TG_EVO_BOT_RANDOM_COFFEE_POLL_TASK_ENABLED` | Boolean | No | true | Whether to enable weekly participation polls |
| Poll Time | `TG_EVO_BOT_RANDOM_COFFEE_POLL_TIME` | Time (HH:MM) | No | 14:00 | Time to send weekly participation poll (UTC) |
| Poll Day | `TG_EVO_BOT_RANDOM_COFFEE_POLL_DAY` | Weekday | No | friday | Day of week to send participation poll |
| Pairs Task Enabled | `TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TASK_ENABLED` | Boolean | No | true | Whether to enable automatic pairs generation |
| Pairs Time | `TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TIME` | Time (HH:MM) | No | 12:00 | Time to generate and announce pairs (UTC) |
| Pairs Day | `TG_EVO_BOT_RANDOM_COFFEE_PAIRS_DAY` | Weekday | No | monday | Day of week to generate pairs |

### Random Coffee Workflow

```mermaid
flowchart TD
A["Friday 2 PM UTC"] --> B["Send Participation Poll"]
B --> C["Members Vote: Participate/Not Participate"]
C --> D["Monday 12 PM UTC"]
D --> E["Generate Random Pairs"]
E --> F["Announce Pairs in Chat"]
F --> G["Members Arrange Meetings"]
```

**Diagram sources**
- [config.go](file://internal/config/config.go#L221-L286)
- [random_coffee_poll_task.go](file://internal/tasks/random_coffee_poll_task.go#L1-L106)
- [random_coffee_pairs_task.go](file://internal/tasks/random_coffee_pairs_task.go#L1-L102)

The random coffee system operates on a weekly cycle with two main components: the participation poll and the pairs generation.

The poll task runs weekly on the day and time specified by `TG_EVO_BOT_RANDOM_COFFEE_POLL_DAY` and `TG_EVO_BOT_RANDOM_COFFEE_POLL_TIME` (defaulting to Friday at 2:00 PM UTC). It sends a poll to the topic specified by `TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID`, asking members if they want to participate in random coffee meetings for the following week.

The pairs task runs on the day and time specified by `TG_EVO_BOT_RANDOM_COFFEE_PAIRS_DAY` and `TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TIME` (defaulting to Monday at 12:00 PM UTC). It collects the list of participants from the most recent poll and randomly pairs them, announcing the pairs in the same topic.

Both tasks can be enabled or disabled independently using their respective enabled flags, allowing administrators to control which aspects of the random coffee system are active.

**Section sources**
- [config.go](file://internal/config/config.go#L221-L286)
- [random_coffee_poll_task.go](file://internal/tasks/random_coffee_poll_task.go#L1-L106)
- [random_coffee_pairs_task.go](file://internal/tasks/random_coffee_pairs_task.go#L1-L102)

## Common Issues and Solutions

This section addresses common configuration issues and their solutions.

### Missing Environment Variables

**Issue**: Application fails to start with error "environment variable is not set"

**Solution**: Ensure all required environment variables are set. Required variables include:
- `TG_EVO_BOT_TOKEN`
- `TG_EVO_BOT_SUPERGROUP_CHAT_ID`
- `TG_EVO_BOT_OPENAI_API_KEY`
- `TG_EVO_BOT_DB_CONNECTION`
- `TG_EVO_BOT_MONITORED_TOPICS_IDS`
- `TG_EVO_BOT_SUMMARY_TOPIC_ID`
- `TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID`

### Invalid Value Formats

**Issue**: Application fails to start with parsing errors

**Common causes and solutions**:
- **Invalid integer**: Ensure topic IDs and chat IDs contain only numeric characters
- **Invalid time format**: Use 24-hour format (HH:MM) without quotes
- **Invalid boolean**: Use `true` or `false` (not `yes`/`no` or `1`/`0`)
- **Invalid weekday**: Use lowercase day names (e.g., `friday`, not `Friday` or `Fri`)

### Database Connection Issues

**Issue**: "Failed to load configuration: TG_EVO_BOT_DB_CONNECTION environment variable is not set"

**Solution**: Verify the PostgreSQL connection string format:
```
postgresql://username:password@hostname:port/database_name
```

Ensure the database is accessible from the bot's environment and that credentials are correct.

### Telegram User Client Authentication

**Issue**: User client fails to authenticate

**Solutions**:
1. Verify App ID and App Hash are correct
2. Ensure phone number is in international format
3. If using 2FA, provide the correct password
4. After first run, check your Telegram app for the verification code
5. Enter the code in reverse using the `/code` command in the bot

**Section sources**
- [config.go](file://internal/config/config.go#L1-L340)
- [README.md](file://README.md#L1-L295)

## Best Practices

Follow these best practices for optimal configuration of the evocoders-bot-go application.

### Security Considerations

- Never commit environment variables to version control
- Use different bot tokens for development and production
- Restrict access to the configuration file and environment variables
- Rotate API keys periodically
- Use strong, unique passwords for 2FA

### Configuration Management

- Use a `.env` file for local development (add to .gitignore)
- Document all configuration parameters for team members
- Test configuration changes in a staging environment first
- Keep backup copies of working configuration
- Use consistent naming conventions for topic IDs

### Monitoring and Maintenance

- Regularly check application logs for configuration-related errors
- Verify scheduled tasks are running as expected
- Monitor database connection stability
- Test the verification code process periodically
- Update configuration when changing Telegram group structure

**Section sources**
- [config.go](file://internal/config/config.go#L1-L340)
- [README.md](file://README.md#L1-L295)