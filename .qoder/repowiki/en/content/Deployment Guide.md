# Deployment Guide

<cite>
**Referenced Files in This Document**   
- [main.go](file://main.go)
- [config.go](file://internal/config/config.go)
- [db.go](file://internal/database/db.go)
- [migrator.go](file://internal/database/migrations/migrator.go)
- [README.md](file://README.md)
</cite>

## Table of Contents
1. [Introduction](#introduction)
2. [Deployment Architecture](#deployment-architecture)
3. [Production Configuration](#production-configuration)
4. [Database Setup and Management](#database-setup-and-management)
5. [Monitoring and Logging Practices](#monitoring-and-logging-practices)
6. [Backup and Recovery Procedures](#backup-and-recovery-procedures)
7. [Cross-Platform Deployment](#cross-platform-deployment)
8. [Scaling Considerations](#scaling-considerations)
9. [Common Production Issues and Solutions](#common-production-issues-and-solutions)
10. [Conclusion](#conclusion)

## Introduction

The evocoders-bot-go is a Telegram bot designed for community management, featuring AI-powered search, daily chat summarization, random coffee meetings, and user profile management. This deployment guide provides comprehensive instructions for setting up the bot in a production environment, covering configuration, database management, monitoring, and scaling considerations. The guide is designed to serve both beginners seeking conceptual understanding and experienced developers requiring technical implementation details.

## Deployment Architecture

The evocoders-bot-go follows a modular architecture with clear separation of concerns. The deployment components work together to provide a robust community management solution.

```mermaid
graph TD
A[Telegram Bot] --> B[Configuration]
A --> C[OpenAI Client]
A --> D[Database]
A --> E[Telegram User Client]
B --> F[Environment Variables]
D --> G[PostgreSQL]
D --> H[Database Migrations]
A --> I[Task Scheduler]
I --> J[Daily Summarization]
I --> K[Random Coffee Polls]
I --> L[Random Coffee Pairs]
A --> M[Message Handlers]
M --> N[Private Handlers]
M --> O[Admin Handlers]
M --> P[Group Handlers]
style A fill:#4CAF50,stroke:#388E3C
style D fill:#2196F3,stroke:#1976D2
style I fill:#FF9800,stroke:#F57C00
style M fill:#9C27B0,stroke:#7B1FA2
```

**Diagram sources**
- [main.go](file://main.go#L1-L54)
- [config.go](file://internal/config/config.go#L1-L341)
- [db.go](file://internal/database/db.go#L1-L45)

**Section sources**
- [main.go](file://main.go#L1-L54)
- [config.go](file://internal/config/config.go#L1-L341)

## Production Configuration

The evocoders-bot-go uses environment variables for configuration, allowing for flexible deployment across different environments. The configuration system is implemented in the `config` package and loaded at application startup.

### Configuration Loading Process

The configuration loading process begins in the `main.go` file, where the `LoadConfig()` function is called to initialize the application configuration from environment variables. This approach enables secure configuration management without hardcoding sensitive information in the source code.

```mermaid
flowchart TD
Start([Application Start]) --> LoadConfig["LoadConfig()"]
LoadConfig --> ValidateBotToken{"Bot Token Set?"}
ValidateBotToken --> |No| Error1["Error: TG_EVO_BOT_TOKEN not set"]
ValidateBotToken --> |Yes| ValidateChatID{"Supergroup Chat ID Set?"}
ValidateChatID --> |No| Error2["Error: TG_EVO_BOT_SUPERGROUP_CHAT_ID not set"]
ValidateChatID --> |Yes| ValidateOpenAI{"OpenAI API Key Set?"}
ValidateOpenAI --> |No| Error3["Error: TG_EVO_BOT_OPENAI_API_KEY not set"]
ValidateOpenAI --> |Yes| ValidateDB{"Database Connection Set?"}
ValidateDB --> |No| Error4["Error: TG_EVO_BOT_DB_CONNECTION not set"]
ValidateDB --> |Yes| ValidateTopics{"Monitored Topics IDs Set?"}
ValidateTopics --> |No| Error5["Error: TG_EVO_BOT_MONITORED_TOPICS_IDS not set"]
ValidateTopics --> |Yes| ValidateSummary{"Summary Topic ID Set?"}
ValidateSummary --> |No| Error6["Error: TG_EVO_BOT_SUMMARY_TOPIC_ID not set"]
ValidateSummary --> |Yes| ValidateCoffee{"Random Coffee Topic ID Set?"}
ValidateCoffee --> |No| Error7["Error: TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID not set"]
ValidateCoffee --> |Yes| Success["Configuration Loaded Successfully"]
Error1 --> End([Application Exit])
Error2 --> End
Error3 --> End
Error4 --> End
Error5 --> End
Error6 --> End
Error7 --> End
Success --> End
style Error1 fill:#f44336,stroke:#d32f2f
style Error2 fill:#f44336,stroke:#d32f2f
style Error3 fill:#f44336,stroke:#d32f2f
style Error4 fill:#f44336,stroke:#d32f2f
style Error5 fill:#f44336,stroke:#d32f2f
style Error6 fill:#f44336,stroke:#d32f2f
style Error7 fill:#f44336,stroke:#d32f2f
style Success fill:#4CAF50,stroke:#388E3C
```

**Diagram sources**
- [main.go](file://main.go#L1-L54)
- [config.go](file://internal/config/config.go#L1-L341)

**Section sources**
- [config.go](file://internal/config/config.go#L1-L341)

### Essential Environment Variables

The following environment variables are required for production deployment:

**Basic Bot Configuration**
- `TG_EVO_BOT_TOKEN`: Your Telegram bot token (required)
- `TG_EVO_BOT_SUPERGROUP_CHAT_ID`: Chat ID of your Supergroup (required)
- `TG_EVO_BOT_OPENAI_API_KEY`: OpenAI API key (required)
- `TG_EVO_BOT_ADMIN_USER_ID`: User ID for the administrator account (optional)

**Topics Management**
- `TG_EVO_BOT_CLOSED_TOPICS_IDS`: Comma-separated list of topic IDs that are closed for chatting
- `TG_EVO_BOT_FORWARDING_TOPIC_ID`: ID of the topic where forwarded replies will be sent (0 for General topic)
- `TG_EVO_BOT_TOOL_TOPIC_ID`: Topic ID for the AI tools database
- `TG_EVO_BOT_CONTENT_TOPIC_ID`: Topic ID for the content topic
- `TG_EVO_BOT_INTRO_TOPIC_ID`: Topic ID for the club introductions and member information
- `TG_EVO_BOT_ANNOUNCEMENT_TOPIC_ID`: Topic ID for announcements

**Telegram User Client**
- `TG_EVO_BOT_TGUSERCLIENT_APPID`: Telegram API App ID
- `TG_EVO_BOT_TGUSERCLIENT_APPHASH`: Telegram API App Hash
- `TG_EVO_BOT_TGUSERCLIENT_PHONENUMBER`: Phone number for Telegram user client
- `TG_EVO_BOT_TGUSERCLIENT_2FAPASS`: Two-factor authentication password for Telegram user client (if using 2FA)
- `TG_EVO_BOT_TGUSERCLIENT_SESSION_TYPE`: Session type for Telegram User Client (file, database, or memory)

**Daily Summarization Feature**
- `TG_EVO_BOT_DB_CONNECTION`: PostgreSQL connection string (required)
- `TG_EVO_BOT_MONITORED_TOPICS_IDS`: Comma-separated list of topic IDs to monitor for summarization (required)
- `TG_EVO_BOT_SUMMARY_TOPIC_ID`: Topic ID where daily summaries will be posted (required)
- `TG_EVO_BOT_SUMMARY_TIME`: Time to run daily summary in 24-hour format (e.g., `03:00` for 3 AM)
- `TG_EVO_BOT_SUMMARIZATION_TASK_ENABLED`: Enable or disable the daily summarization task (`true` or `false`, defaults to `true`)

**Random Coffee Feature**
- `TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID`: Topic ID where random coffee polls and pairs will be posted (required)
- `TG_EVO_BOT_RANDOM_COFFEE_POLL_TASK_ENABLED`: Enable or disable the weekly coffee poll task (`true` or `false`, defaults to `true`)
- `TG_EVO_BOT_RANDOM_COFFEE_POLL_TIME`: Time to send the weekly coffee poll in 24-hour format UTC (e.g., `14:00` for 2 PM UTC, defaults to `14:00`)
- `TG_EVO_BOT_RANDOM_COFFEE_POLL_DAY`: Day of the week to send the poll (e.g., `friday`, `monday`, etc., defaults to `friday`)
- `TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TASK_ENABLED`: Enable or disable the automatic pairs generation task (`true` or `false`, defaults to `true`)
- `TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TIME`: Time to generate and announce coffee pairs in 24-hour format UTC (e.g., `12:00` for 12 PM UTC, defaults to `12:00`)
- `TG_EVO_BOT_RANDOM_COFFEE_PAIRS_DAY`: Day of the week to generate pairs (e.g., `monday`, `tuesday`, etc., defaults to `monday`)

## Database Setup and Management

The evocoders-bot-go uses PostgreSQL as its primary database with an automated migration system to ensure database schema consistency across deployments.

### Database Initialization

The database is initialized and configured in the `internal/database/db.go` file. The `NewDB` function creates a new database connection using the PostgreSQL driver, and the `InitWithMigrations` method ensures that all pending migrations are applied when the application starts.

```mermaid
sequenceDiagram
participant App as Application
participant DB as Database
participant Migrations as Migrations System
App->>DB : Open connection with connection string
DB-->>App : Connection established
App->>DB : Ping database
DB-->>App : Ping successful
App->>Migrations : RunMigrations()
Migrations->>DB : Check if migrations table exists
DB-->>Migrations : Table status
alt Migrations table doesn't exist
Migrations->>DB : Create migrations table
DB-->>Migrations : Table created
end
Migrations->>DB : Query applied migrations
DB-->>Migrations : List of applied migrations
Migrations->>Migrations : Get all registered migrations
loop For each pending migration
Migrations->>Migrations : Apply migration
alt Migration succeeds
Migrations->>DB : Record migration in database
DB-->>Migrations : Record successful
else Migration fails
Migrations->>Migrations : Rollback migration
Migrations->>DB : Remove migration record
Migrations->>App : Return error
break
end
end
Migrations-->>App : All migrations applied
```

**Diagram sources**
- [db.go](file://internal/database/db.go#L1-L45)
- [migrator.go](file://internal/database/migrations/migrator.go#L1-L150)

**Section sources**
- [db.go](file://internal/database/db.go#L1-L45)
- [migrator.go](file://internal/database/migrations/migrator.go#L1-L150)

### Migration System

The migration system is implemented in the `internal/database/migrations` package and provides several key features:

- **Automatic tracking**: Applied migrations are tracked in a `migrations` table
- **Idempotent execution**: Migrations can be safely run multiple times
- **Automatic rollback**: Failed migrations are automatically rolled back
- **Ordered execution**: Migrations are applied in order based on their timestamp
- **Failure handling**: If a migration fails, the system stops processing further migrations

The migration system follows a registry pattern where all migrations are registered in the `Registry()` function in `migrator.go`. Each migration implements the `Migration` interface with `Apply` and `Rollback` methods, ensuring that every change can be reversed if necessary.

When the application starts, the migration system:
1. Initializes the migrations table schema if it doesn't exist
2. Retrieves the list of already applied migrations
3. Compares this list with the registry of available migrations
4. Applies each pending migration in order
5. Records successful migrations in the database
6. Stops and rolls back if any migration fails

This approach ensures database schema consistency and provides a reliable mechanism for evolving the database schema over time.

## Monitoring and Logging Practices

The evocoders-bot-go implements comprehensive monitoring and logging practices to ensure reliable operation in production environments.

### Logging Implementation

The application uses Go's standard `log` package for logging, with strategic placement of log statements throughout the codebase. Key logging practices include:

- **Startup logging**: The application logs when it starts and when it receives shutdown signals
- **Error logging**: All errors are logged with context to aid in troubleshooting
- **Migration logging**: Database migrations are logged with their status (applying, success, failure)
- **Graceful shutdown**: The application logs when it begins shutting down and closing resources

The logging system is designed to provide sufficient information for monitoring the application's health without overwhelming log files with excessive detail. Error logs include enough context to identify the source of issues, while informational logs track key application events.

### Task Monitoring

The application includes several scheduled tasks that are critical to its functionality:

- **Daily summarization**: Generates daily summaries of chat conversations
- **Random coffee polls**: Posts weekly polls to gauge member interest in random coffee meetings
- **Random coffee pairs**: Generates and announces pairs for random coffee meetings

These tasks are configured through environment variables that control their schedule and enable/disable state. The default configuration runs the coffee poll task on Fridays at 2 PM UTC and the pairs generation task on Mondays at 12 PM UTC.

Monitoring these tasks is essential for ensuring the bot's community engagement features function correctly. Operators should verify that:
- Summaries are posted at the configured time
- Coffee polls are created weekly
- Pairs are generated and announced as expected
- No errors occur during task execution

## Backup and Recovery Procedures

While the evocoders-bot-go documentation doesn't explicitly detail backup procedures, the architecture suggests several best practices for ensuring data durability and recovery.

### Data Persistence Strategy

The application stores critical data in PostgreSQL, including:
- Chat messages for summarization
- User profiles and information
- Event details and topics
- Random coffee poll and pairing history
- Migration tracking

This centralized data storage simplifies backup strategies, as all critical data resides in a single PostgreSQL database.

### Recommended Backup Procedures

For production deployments, implement the following backup procedures:

1. **Regular database backups**: Schedule automated backups of the PostgreSQL database using `pg_dump` or similar tools
2. **Backup retention**: Maintain multiple backup generations (daily, weekly, monthly) to allow recovery from different points in time
3. **Offsite storage**: Store backups in a separate location from the production database to protect against site-wide failures
4. **Backup verification**: Regularly test backup restoration to ensure backups are valid and complete
5. **Session data backup**: If using file-based session storage for the Telegram User Client, include these files in the backup strategy

### Recovery Process

In the event of data loss or corruption, the recovery process should:

1. Stop the evocoders-bot-go application
2. Restore the database from the most recent valid backup
3. Verify data integrity
4. Restart the application
5. Monitor for any issues during startup

The automated migration system provides an additional layer of protection, as it will ensure the database schema is up-to-date when the application restarts, even if restoring to a point where schema changes had not yet been applied.

## Cross-Platform Deployment

The evocoders-bot-go can be deployed on various platforms, with specific considerations for different operating systems.

### Windows Deployment

To deploy the bot on Windows systems, compile the executable using the following command:

```shell
GOOS=windows GOARCH=amd64 go build -o bot.exe
```

This command creates a Windows executable named `bot.exe` that can run on 64-bit Windows systems. When deploying on Windows:

1. Set environment variables using the Command Prompt:
```shell
set TG_EVO_BOT_TOKEN=your_bot_token_here
set TG_EVO_BOT_OPENAI_API_KEY=your_openai_api_key_here
# ... set other required variables
```

2. Run the executable:
```shell
bot.exe
```

### Unix-like Systems Deployment

For Linux, macOS, and other Unix-like systems, compile and run the bot as follows:

```shell
# Build the executable
go build -o bot main.go

# Set environment variables
export TG_EVO_BOT_TOKEN="your_bot_token_here"
export TG_EVO_BOT_OPENAI_API_KEY="your_openai_api_key_here"
# ... set other required variables

# Run the bot
./bot
```

### Containerized Deployment

For containerized deployments using Docker, create a Dockerfile that:

1. Uses a Go base image for building
2. Copies the source code
3. Builds the executable
4. Uses a minimal base image (like Alpine Linux) for the final image
5. Copies the executable to the final image
6. Sets environment variables as needed
7. Specifies the command to run the bot

This approach provides consistency across deployment environments and simplifies dependency management.

## Scaling Considerations

The evocoders-bot-go architecture has several implications for scaling to handle large communities and message volumes.

### Current Architecture Limitations

The current implementation is designed as a single-instance application with the following characteristics:

- **Single-threaded event processing**: Telegram updates are processed sequentially
- **In-memory state**: Some state may be maintained in memory during operation
- **Database-dependent**: Performance is closely tied to database performance
- **No horizontal scaling**: The application is not designed to run multiple instances simultaneously

### Scaling Strategies

To handle larger communities and message volumes, consider the following strategies:

1. **Vertical scaling**: Increase the resources (CPU, memory) of the host machine to handle higher loads
2. **Database optimization**: Optimize PostgreSQL configuration, add indexes to frequently queried columns, and consider connection pooling
3. **Caching**: Implement caching for frequently accessed data to reduce database load
4. **Asynchronous processing**: Refactor long-running operations (like summarization) to run asynchronously
5. **Rate limiting**: Implement rate limiting to prevent abuse and ensure fair resource usage

### Performance Monitoring

Monitor the following metrics to identify scaling needs:

- **Message processing latency**: Time between receiving a message and processing it
- **Database query performance**: Response times for database operations
- **Memory usage**: Application memory consumption over time
- **CPU utilization**: Processing load on the host machine
- **Task execution time**: Duration of scheduled tasks like summarization

When any of these metrics consistently approach system limits, it may be time to implement scaling measures.

## Common Production Issues and Solutions

This section addresses common issues encountered during production deployment of the evocoders-bot-go and provides solutions.

### Configuration Issues

**Issue**: Missing required environment variables
**Symptoms**: Application fails to start with error messages indicating missing configuration
**Solution**: Ensure all required environment variables are set, particularly:
- `TG_EVO_BOT_TOKEN`
- `TG_EVO_BOT_SUPERGROUP_CHAT_ID`
- `TG_EVO_BOT_OPENAI_API_KEY`
- `TG_EVO_BOT_DB_CONNECTION`
- `TG_EVO_BOT_MONITORED_TOPICS_IDS`
- `TG_EVO_BOT_SUMMARY_TOPIC_ID`
- `TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID`

### Database Connection Problems

**Issue**: Unable to connect to PostgreSQL database
**Symptoms**: Application fails to start with database connection errors
**Solution**: 
1. Verify the connection string format: `postgresql://user:password@host:port/database`
2. Ensure the PostgreSQL server is running and accessible
3. Check firewall settings to ensure the port is open
4. Verify database credentials are correct
5. Confirm the database exists and the user has appropriate permissions

### Migration Failures

**Issue**: Database migration fails during startup
**Symptoms**: Application fails to start, with error messages about migration failures
**Solution**:
1. Check the error message to identify the specific migration that failed
2. Review the migration code in `internal/database/migrations/implementations/`
3. Verify the database state before the migration
4. If the migration cannot be fixed, consider restoring from backup
5. The migration system will automatically rollback failed migrations when possible

### Telegram User Client Authentication

**Issue**: Telegram User Client fails to authenticate
**Symptoms**: Bot cannot perform actions requiring the User Client, such as sending typing indicators
**Solution**:
1. Ensure the Telegram API App ID and App Hash are correct
2. Verify the phone number is entered in the correct format
3. If using 2FA, ensure the password is correct
4. After first run, obtain the verification code from your Telegram app and send it reverted with the `/code` command to your bot
5. The session will be updated automatically once per 30 minutes

### Task Scheduling Issues

**Issue**: Scheduled tasks (summarization, coffee polls) don't run as expected
**Symptoms**: Summaries not posted, polls not created, or pairs not generated
**Solution**:
1. Verify the task is enabled in configuration (`TG_EVO_BOT_SUMMARIZATION_TASK_ENABLED=true`, etc.)
2. Check the configured times and days are correct
3. Ensure the application has been running continuously since the last scheduled time
4. Review logs for any errors during task execution
5. Verify the bot has the necessary permissions in the Telegram group

## Conclusion

The evocoders-bot-go provides a comprehensive solution for Telegram community management with features like AI-powered search, daily summarization, and random coffee meetings. Successful production deployment requires careful attention to configuration, database management, and monitoring. By following the guidelines in this document, operators can ensure reliable operation of the bot in production environments. For optimal performance with large communities, consider implementing the scaling strategies outlined in this guide and monitoring key performance metrics to identify when additional resources are needed.