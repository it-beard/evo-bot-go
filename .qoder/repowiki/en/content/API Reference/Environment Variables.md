# Environment Variables

<cite>
**Referenced Files in This Document**   
- [config.go](file://internal/config/config.go)
- [main.go](file://main.go)
- [README.md](file://README.md)
</cite>

## Table of Contents
1. [Introduction](#introduction)
2. [Environment Variables by Domain](#environment-variables-by-domain)
3. [Configuration Initialization Process](#configuration-initialization-process)
4. [Sample .env Files](#sample-env-files)
5. [Security Implications and Protection Measures](#security-implications-and-protection-measures)
6. [Troubleshooting Common Configuration Issues](#troubleshooting-common-configuration-issues)
7. [Version Compatibility and Migration](#version-compatibility-and-migration)

## Introduction

This document provides comprehensive documentation for all environment variables used in the evocoders-bot-go application. The configuration system is implemented in the `config.go` file and follows a structured approach to manage application settings across different domains including Telegram integration, database configuration, OpenAI settings, application behavior, and feature flags.

The application uses environment variables prefixed with `TG_EVO_BOT_` to ensure namespace isolation and prevent conflicts with other applications. All configuration values are loaded at application startup through the `LoadConfig()` function, which performs validation and type conversion. Required variables trigger application failure if not set, while optional variables have sensible defaults.

**Section sources**
- [config.go](file://internal/config/config.go#L1-L319)
- [README.md](file://README.md#L1-L274)

## Environment Variables by Domain

### Telegram Integration

| Variable Name | Data Type | Required/Optional | Default Value | Functional Significance |
|---------------|---------|-------------------|---------------|-------------------------|
| `TG_EVO_BOT_TOKEN` | string | Required | None | Authentication token for the Telegram Bot API. This token is obtained from BotFather and grants the bot access to send and receive messages in the specified supergroup. Without this token, the bot cannot connect to Telegram's servers. |
| `TG_EVO_BOT_SUPERGROUP_CHAT_ID` | int64 | Required | None | Identifier of the Telegram supergroup where the bot operates. This numeric ID is used to target messages, polls, and other interactions to the correct chat. The bot uses this ID to post summaries, random coffee polls, and other automated content. |
| `TG_EVO_BOT_ADMIN_USER_ID` | int64 | Optional | None | Telegram user ID of the administrator who receives notifications about new topics and can execute admin-only commands. When set, the bot grants special privileges to this user, such as the ability to manually trigger summarization or manage events. |
| `TG_EVO_BOT_CLOSED_TOPICS_IDS` | comma-separated integers | Optional | Empty | List of topic IDs in the supergroup that are considered "closed" for general chatting. The bot automatically deletes messages in these topics unless posted by administrators, helping maintain focused discussions. |
| `TG_EVO_BOT_FORWARDING_TOPIC_ID` | integer | Optional | 0 (General) | Topic ID where replies from closed threads are forwarded. Setting this to 0 directs messages to the General topic. This feature helps preserve valuable contributions while maintaining thread cleanliness. |
| `TG_EVO_BOT_TOOL_TOPIC_ID` | integer | Optional | None | Topic ID dedicated to AI tools database. The `/tool` command searches content within this topic to provide relevant tool recommendations based on user queries. |
| `TG_EVO_BOT_CONTENT_TOPIC_ID` | integer | Optional | None | Topic ID for general content discussions. The `/content` command searches through messages in this topic to find information requested by users. |
| `TG_EVO_BOT_INTRO_TOPIC_ID` | integer | Optional | None | Topic ID where club member introductions are published. The `/intro` command searches this topic to provide information about members, and user profiles are published here when created. |
| `TG_EVO_BOT_ANNOUNCEMENT_TOPIC_ID` | integer | Optional | None | Topic ID designated for official announcements. The bot may use this topic for important system messages or event notifications. |

**Section sources**
- [config.go](file://internal/config/config.go#L14-L34)
- [README.md](file://README.md#L1-L274)

### Database Configuration

| Variable Name | Data Type | Required/Optional | Default Value | Functional Significance |
|---------------|---------|-------------------|---------------|-------------------------|
| `TG_EVO_BOT_DB_CONNECTION` | string | Required | None | PostgreSQL connection string in the format `postgresql://user:password@host:port/database`. This connection is used for all database operations including storing group messages, topics, user profiles, and random coffee history. The database schema is automatically initialized with required tables. |

**Section sources**
- [config.go](file://internal/config/config.go#L36-L37)
- [README.md](file://README.md#L1-L274)

### OpenAI Settings

| Variable Name | Data Type | Required/Optional | Default Value | Functional Significance |
|---------------|---------|-------------------|---------------|-------------------------|
| `TG_EVO_BOT_OPENAI_API_KEY` | string | Required | None | API key for authenticating with OpenAI services. This key enables the bot's AI-powered features including tool search, content search, member introduction search, and daily chat summarization. Without this key, all AI functionality will be disabled. |

**Section sources**
- [config.go](file://internal/config/config.go#L16-L17)
- [README.md](file://README.md#L1-L274)

### Application Behavior

| Variable Name | Data Type | Required/Optional | Default Value | Functional Significance |
|---------------|---------|-------------------|---------------|-------------------------|
| `TG_EVO_BOT_SUMMARY_TIME` | string (HH:MM) | Optional | "03:00" | Time in 24-hour format (UTC) when the daily summarization task runs. The bot generates and posts a summary of monitored topics at this time. The default of 3:00 AM UTC ensures summaries are ready for morning readers across time zones. |
| `TG_EVO_BOT_RANDOM_COFFEE_POLL_TIME` | string (HH:MM) | Optional | "14:00" | Time in 24-hour format (UTC) when the weekly random coffee poll is posted. The default of 2:00 PM UTC targets a time when most members are likely to be active. |
| `TG_EVO_BOT_RANDOM_COFFEE_POLL_DAY` | string | Optional | "friday" | Day of the week when the random coffee poll is posted. Valid values are: sunday, monday, tuesday, wednesday, thursday, friday, saturday. The default of "friday" allows members to indicate availability for meetings in the following week. |
| `TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TIME` | string (HH:MM) | Optional | "12:00" | Time in 24-hour format (UTC) when random coffee pairs are generated and announced. The default of 12:00 PM UTC targets midday when members can plan afternoon or evening meetings. |
| `TG_EVO_BOT_RANDOM_COFFEE_PAIRS_DAY` | string | Optional | "monday" | Day of the week when random coffee pairs are generated. Valid values are: sunday, monday, tuesday, wednesday, thursday, friday, saturday. The default of "monday" kickstarts new connections at the beginning of the work week. |

**Section sources**
- [config.go](file://internal/config/config.go#L39-L43)
- [README.md](file://README.md#L1-L274)

### Feature Flags

| Variable Name | Data Type | Required/Optional | Default Value | Functional Significance |
|---------------|---------|-------------------|---------------|-------------------------|
| `TG_EVO_BOT_SUMMARIZATION_TASK_ENABLED` | boolean | Optional | true | Controls whether the daily summarization task is active. When set to `true`, the bot automatically generates and posts daily summaries at the configured time. When `false`, the task is disabled but can still be triggered manually by administrators using the `/summarize` command. |
| `TG_EVO_BOT_RANDOM_COFFEE_POLL_TASK_ENABLED` | boolean | Optional | true | Determines if the weekly random coffee poll is automatically posted. When `true`, the bot creates a poll on the configured day and time asking members if they want to participate in random coffee meetings. When `false`, the poll feature is disabled. |
| `TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TASK_ENABLED` | boolean | Optional | true | Controls the automatic generation of random coffee pairs. When `true`, the bot automatically pairs participating members on the configured day and time. When `false`, pairing must be done manually by an administrator using the `/pair_meetings` command. |
| `TG_EVO_BOT_MONITORED_TOPICS_IDS` | comma-separated integers | Required | None | List of topic IDs that are monitored for the daily summarization feature. The bot analyzes conversations in these topics to generate comprehensive summaries. At least one topic ID must be specified for summarization to work. |
| `TG_EVO_BOT_SUMMARY_TOPIC_ID` | integer | Required | None | Topic ID where daily summaries are posted. This should be a visible topic where all members can access the generated summaries. |
| `TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID` | integer | Required | None | Topic ID where random coffee polls and pair announcements are posted. This dedicated topic keeps coffee-related discussions organized and visible to all members. |

**Section sources**
- [config.go](file://internal/config/config.go#L38-L43)
- [README.md](file://README.md#L1-L274)

## Configuration Initialization Process

The configuration initialization process is implemented in the `LoadConfig()` function within `config.go`. This function follows a systematic approach to read, validate, and convert environment variables into a structured configuration object.

```mermaid
flowchart TD
Start([Application Start]) --> LoadConfig["Call LoadConfig()"]
LoadConfig --> BotToken["Read TG_EVO_BOT_TOKEN"]
BotToken --> TokenValid{"Token Set?"}
TokenValid --> |No| Error1["Return Error: Token Not Set"]
TokenValid --> |Yes| SuperGroupID["Read TG_EVO_BOT_SUPERGROUP_CHAT_ID"]
SuperGroupID --> SGValid{"ID Set?"}
SGValid --> |No| Error2["Return Error: Chat ID Not Set"]
SGValid --> |Yes| ParseSG["Parse as int64"]
ParseSG --> SGParseValid{"Valid Number?"}
SGParseValid --> |No| Error3["Return Error: Invalid Chat ID"]
SGParseValid --> |Yes| OpenAIAPIKey["Read TG_EVO_BOT_OPENAI_API_KEY"]
OpenAIAPIKey --> APIKeyValid{"API Key Set?"}
APIKeyValid --> |No| Error4["Return Error: API Key Not Set"]
APIKeyValid --> |Yes| DBConnection["Read TG_EVO_BOT_DB_CONNECTION"]
DBConnection --> DBValid{"Connection String Set?"}
DBValid --> |No| Error5["Return Error: DB Connection Not Set"]
DBValid --> |Yes| MonitoredTopics["Read TG_EVO_BOT_MONITORED_TOPICS_IDS"]
MonitoredTopics --> TopicsValid{"Topics IDs Set?"}
TopicsValid --> |No| Error6["Return Error: Monitored Topics Not Set"]
TopicsValid --> |Yes| ParseTopics["Parse as comma-separated integers"]
ParseTopics --> TopicsParseValid{"Valid IDs?"}
TopicsParseValid --> |No| Error7["Return Error: Invalid Topic IDs"]
TopicsParseValid --> |Yes| SummaryTopic["Read TG_EVO_BOT_SUMMARY_TOPIC_ID"]
SummaryTopic --> SummaryValid{"Summary Topic Set?"}
SummaryValid --> |No| Error8["Return Error: Summary Topic Not Set"]
SummaryValid --> |Yes| RandomCoffeeTopic["Read TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID"]
RandomCoffeeTopic --> CoffeeValid{"Coffee Topic Set?"}
CoffeeValid --> |No| Error9["Return Error: Coffee Topic Not Set"]
CoffeeValid --> |Yes| Optional["Process Optional Variables"]
Optional --> ClosedTopics["Read TG_EVO_BOT_CLOSED_TOPICS_IDS"]
Optional --> AdminUserID["Read TG_EVO_BOT_ADMIN_USER_ID"]
Optional --> ForwardingTopic["Read TG_EVO_BOT_FORWARDING_TOPIC_ID"]
Optional --> ToolTopic["Read TG_EVO_BOT_TOOL_TOPIC_ID"]
Optional --> ContentTopic["Read TG_EVO_BOT_CONTENT_TOPIC_ID"]
Optional --> IntroTopic["Read TG_EVO_BOT_INTRO_TOPIC_ID"]
Optional --> AnnouncementTopic["Read TG_EVO_BOT_ANNOUNCEMENT_TOPIC_ID"]
Optional --> SummaryTime["Read TG_EVO_BOT_SUMMARY_TIME"]
Optional --> SummarizationEnabled["Read TG_EVO_BOT_SUMMARIZATION_TASK_ENABLED"]
Optional --> CoffeePollTime["Read TG_EVO_BOT_RANDOM_COFFEE_POLL_TIME"]
Optional --> CoffeePollDay["Read TG_EVO_BOT_RANDOM_COFFEE_POLL_DAY"]
Optional --> CoffeePairsTime["Read TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TIME"]
Optional --> CoffeePairsDay["Read TG_EVO_BOT_RANDOM_COFFEE_PAIRS_DAY"]
Optional --> CoffeePollEnabled["Read TG_EVO_BOT_RANDOM_COFFEE_POLL_TASK_ENABLED"]
Optional --> CoffeePairsEnabled["Read TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TASK_ENABLED"]
Optional --> Success["Return Config Object"]
Error1 --> End([Application Fails])
Error2 --> End
Error3 --> End
Error4 --> End
Error5 --> End
Error6 --> End
Error7 --> End
Error8 --> End
Error9 --> End
Success --> End
```

**Diagram sources**
- [config.go](file://internal/config/config.go#L46-L317)

**Section sources**
- [config.go](file://internal/config/config.go#L46-L317)
- [main.go](file://main.go#L13-L16)

## Sample .env Files

### Development Environment
```env
# Basic Bot Configuration 
TG_EVO_BOT_TOKEN=dev_bot_token_12345
TG_EVO_BOT_OPENAI_API_KEY=dev_openai_key_67890
TG_EVO_BOT_SUPERGROUP_CHAT_ID=123456789
TG_EVO_BOT_ADMIN_USER_ID=987654321

# Topics Management
TG_EVO_BOT_CLOSED_TOPICS_IDS=101,102,103
TG_EVO_BOT_FORWARDING_TOPIC_ID=104
TG_EVO_BOT_TOOL_TOPIC_ID=105
TG_EVO_BOT_CONTENT_TOPIC_ID=106
TG_EVO_BOT_INTRO_TOPIC_ID=107
TG_EVO_BOT_ANNOUNCEMENT_TOPIC_ID=108

# Daily Summarization Feature
TG_EVO_BOT_DB_CONNECTION=postgresql://dev_user:dev_password@localhost:5432/dev_db
TG_EVO_BOT_MONITORED_TOPICS_IDS=101,102
TG_EVO_BOT_SUMMARY_TOPIC_ID=109
TG_EVO_BOT_SUMMARY_TIME=03:00
TG_EVO_BOT_SUMMARIZATION_TASK_ENABLED=true

# Random Coffee Feature
TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID=110
TG_EVO_BOT_RANDOM_COFFEE_POLL_TASK_ENABLED=true
TG_EVO_BOT_RANDOM_COFFEE_POLL_TIME=14:00
TG_EVO_BOT_RANDOM_COFFEE_POLL_DAY=friday
TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TASK_ENABLED=true
TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TIME=12:00
TG_EVO_BOT_RANDOM_COFFEE_PAIRS_DAY=monday
```

### Production Environment
```env
# Basic Bot Configuration 
TG_EVO_BOT_TOKEN=prod_bot_token_abcde
TG_EVO_BOT_OPENAI_API_KEY=prod_openai_key_fghij
TG_EVO_BOT_SUPERGROUP_CHAT_ID=9876543210
TG_EVO_BOT_ADMIN_USER_ID=1234567890

# Topics Management
TG_EVO_BOT_CLOSED_TOPICS_IDS=201,202,203
TG_EVO_BOT_FORWARDING_TOPIC_ID=204
TG_EVO_BOT_TOOL_TOPIC_ID=205
TG_EVO_BOT_CONTENT_TOPIC_ID=206
TG_EVO_BOT_INTRO_TOPIC_ID=207
TG_EVO_BOT_ANNOUNCEMENT_TOPIC_ID=208

# Daily Summarization Feature
TG_EVO_BOT_DB_CONNECTION=postgresql://prod_user:prod_password@db.prod.example.com:5432/prod_db
TG_EVO_BOT_MONITORED_TOPICS_IDS=201,202
TG_EVO_BOT_SUMMARY_TOPIC_ID=209
TG_EVO_BOT_SUMMARY_TIME=03:00
TG_EVO_BOT_SUMMARIZATION_TASK_ENABLED=true

# Random Coffee Feature
TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID=210
TG_EVO_BOT_RANDOM_COFFEE_POLL_TASK_ENABLED=true
TG_EVO_BOT_RANDOM_COFFEE_POLL_TIME=14:00
TG_EVO_BOT_RANDOM_COFFEE_POLL_DAY=friday
TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TASK_ENABLED=true
TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TIME=12:00
TG_EVO_BOT_RANDOM_COFFEE_PAIRS_DAY=monday
```

### Staging Environment with Features Disabled
```env
# Basic Bot Configuration 
TG_EVO_BOT_TOKEN=staging_bot_token_xyz
TG_EVO_BOT_OPENAI_API_KEY=staging_openai_key_123
TG_EVO_BOT_SUPERGROUP_CHAT_ID=555555555
TG_EVO_BOT_ADMIN_USER_ID=444444444

# Topics Management
TG_EVO_BOT_CLOSED_TOPICS_IDS=301,302
TG_EVO_BOT_FORWARDING_TOPIC_ID=303
TG_EVO_BOT_TOOL_TOPIC_ID=304
TG_EVO_BOT_CONTENT_TOPIC_ID=305
TG_EVO_BOT_INTRO_TOPIC_ID=306
TG_EVO_BOT_ANNOUNCEMENT_TOPIC_ID=307

# Daily Summarization Feature
TG_EVO_BOT_DB_CONNECTION=postgresql://staging_user:staging_password@localhost:5432/staging_db
TG_EVO_BOT_MONITORED_TOPICS_IDS=301,302
TG_EVO_BOT_SUMMARY_TOPIC_ID=308
TG_EVO_BOT_SUMMARY_TIME=03:00
TG_EVO_BOT_SUMMARIZATION_TASK_ENABLED=false

# Random Coffee Feature
TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID=309
TG_EVO_BOT_RANDOM_COFFEE_POLL_TASK_ENABLED=false
TG_EVO_BOT_RANDOM_COFFEE_POLL_TIME=14:00
TG_EVO_BOT_RANDOM_COFFEE_POLL_DAY=friday
TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TASK_ENABLED=false
TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TIME=12:00
TG_EVO_BOT_RANDOM_COFFEE_PAIRS_DAY=monday
```

**Section sources**
- [README.md](file://README.md#L1-L274)

## Security Implications and Protection Measures

### Sensitive Variables and Security Implications

The following environment variables contain sensitive information that requires special protection:

- **`TG_EVO_BOT_TOKEN`**: This is the primary authentication credential for the Telegram bot. If compromised, an attacker could:
  - Send unauthorized messages to your community
  - Read all messages in the supergroup
  - Delete messages and moderate the chat
  - Potentially access user data through the bot's permissions

- **`TG_EVO_BOT_OPENAI_API_KEY`**: This key provides access to OpenAI's API with associated billing. If compromised:
  - An attacker could use your API quota, incurring charges
  - They could access AI models for unauthorized purposes
  - They might extract sensitive information through prompt engineering

- **`TG_EVO_BOT_DB_CONNECTION`**: This connection string contains database credentials. If compromised:
  - An attacker could read, modify, or delete all data in your database
  - They could access user profiles, chat history, and other sensitive information
  - They might execute malicious queries or exfiltrate data

### Recommended Protection Measures

1. **Environment Variable Management**:
   - Never commit environment variables to version control
   - Use `.env` files with proper `.gitignore` entries
   - For production, use environment-specific configuration management tools

2. **Access Control**:
   - Restrict access to environment variables to only essential personnel
   - Use role-based access control for configuration management systems
   - Regularly audit who has access to configuration secrets

3. **Secret Rotation**:
   - Regularly rotate API keys and tokens
   - Have a documented procedure for rotating secrets in case of compromise
   - For Telegram bot tokens, use BotFather to generate new tokens when needed

4. **Monitoring and Alerting**:
   - Monitor for unusual API usage patterns that might indicate compromise
   - Set up alerts for high OpenAI API usage
   - Monitor database access logs for suspicious activity

5. **Infrastructure Security**:
   - Use secure deployment platforms that support encrypted secrets
   - Ensure servers are properly secured with firewalls and regular updates
   - Use TLS/SSL for all database connections

6. **Principle of Least Privilege**:
   - Configure the Telegram bot with only the permissions it needs
   - Use database users with minimal required privileges
   - Avoid using admin accounts for application connections

**Section sources**
- [config.go](file://internal/config/config.go#L14-L17)
- [README.md](file://README.md#L1-L274)

## Troubleshooting Common Configuration Issues

### Connection Failures

**Issue**: Application fails to start with "Failed to load configuration" error
- **Cause**: Required environment variables are missing or incorrectly formatted
- **Solution**: Verify all required variables are set:
  - `TG_EVO_BOT_TOKEN`
  - `TG_EVO_BOT_SUPERGROUP_CHAT_ID`
  - `TG_EVO_BOT_OPENAI_API_KEY`
  - `TG_EVO_BOT_DB_CONNECTION`
  - `TG_EVO_BOT_MONITORED_TOPICS_IDS`
  - `TG_EVO_BOT_SUMMARY_TOPIC_ID`
  - `TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID`

**Issue**: Database connection fails
- **Cause**: Incorrect connection string format or network issues
- **Solution**: 
  - Verify the connection string follows `postgresql://user:password@host:port/database` format
  - Check that the database server is accessible from the application server
  - Ensure firewall rules allow the connection
  - Test the connection independently using a PostgreSQL client

### Authentication Errors

**Issue**: Telegram API authentication fails
- **Cause**: Invalid bot token
- **Solution**:
  - Verify the token format (should be numeric ID:token)
  - Regenerate the token using BotFather if compromised
  - Ensure no spaces or special characters are included

**Issue**: OpenAI API authentication fails
- **Cause**: Invalid or revoked API key
- **Solution**:
  - Verify the API key format (starts with "sk-")
  - Generate a new key from the OpenAI dashboard
  - Check that the OpenAI account has sufficient quota

### Feature-Specific Issues

**Issue**: Daily summaries are not being generated
- **Cause**: Summarization task disabled or misconfigured
- **Solution**:
  - Verify `TG_EVO_BOT_SUMMARIZATION_TASK_ENABLED` is set to `true`
  - Check that `TG_EVO_BOT_MONITORED_TOPICS_IDS` includes the correct topic IDs
  - Verify `TG_EVO_BOT_SUMMARY_TOPIC_ID` points to an existing topic
  - Confirm the bot has permission to post in the summary topic

**Issue**: Random coffee polls are not being posted
- **Cause**: Poll task disabled or timing issues
- **Solution**:
  - Verify `TG_EVO_BOT_RANDOM_COFFEE_POLL_TASK_ENABLED` is set to `true`
  - Check that `TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID` is correctly configured
  - Ensure the bot has permission to create polls in the designated topic
  - Verify the timezone configuration (all times are in UTC)

**Issue**: Bot cannot post messages in topics
- **Cause**: Insufficient Telegram bot permissions
- **Solution**:
  - Add the bot as an administrator in the supergroup
  - Enable "Pin messages" and "Delete messages" permissions
  - Ensure the bot has permission to post in the specific topics

**Section sources**
- [config.go](file://internal/config/config.go#L46-L317)
- [README.md](file://README.md#L1-L274)

## Version Compatibility and Migration

The current configuration system is compatible with Go 1.21+ as specified in the project documentation. There are no deprecated environment variables in the current version, as the configuration system appears to be relatively new and stable.

When upgrading from previous versions, ensure that:
1. All environment variable names use the `TG_EVO_BOT_` prefix consistently
2. Topic ID variables are integers, not strings
3. Boolean values are expressed as `true`/`false` rather than `1`/`0`
4. Time values use 24-hour format (HH:MM) without seconds

For future migrations, the configuration system is designed to be backward compatible by providing default values for optional parameters. When new features are added, they should follow the same pattern of using descriptive, prefixed environment variable names with appropriate defaults.

**Section sources**
- [config.go](file://internal/config/config.go#L1-L319)
- [README.md](file://README.md#L1-L274)