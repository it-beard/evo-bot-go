# Core Architecture

<cite>
**Referenced Files in This Document**   
- [main.go](file://main.go)
- [internal/bot/bot.go](file://internal/bot/bot.go)
- [internal/config/config.go](file://internal/config/config.go)
- [internal/clients/openai_client.go](file://internal/clients/openai_client.go)
- [internal/database/db.go](file://internal/database/db.go)
- [internal/services/summarization_service.go](file://internal/services/summarization_service.go)
- [internal/services/random_coffee_service.go](file://internal/services/random_coffee_service.go)
- [internal/services/message_sender_service.go](file://internal/services/message_sender_service.go)
- [internal/services/permissions_service.go](file://internal/services/permissions_service.go)
- [internal/database/repositories/user_repository.go](file://internal/database/repositories/user_repository.go)
- [internal/database/repositories/group_message_repository.go](file://internal/database/repositories/group_message_repository.go)
- [internal/database/repositories/random_coffee_poll_repository.go](file://internal/database/repositories/random_coffee_poll_repository.go)
- [internal/handlers/privatehandlers/profile_handler.go](file://internal/handlers/privatehandlers/profile_handler.go)
- [internal/tasks/daily_summarization_task.go](file://internal/tasks/daily_summarization_task.go)
- [internal/tasks/random_coffee_poll_task.go](file://internal/tasks/random_coffee_poll_task.go)
- [internal/tasks/random_coffee_pairs_task.go](file://internal/tasks/random_coffee_pairs_task.go)
- [internal/tasks/task.go](file://internal/tasks/task.go)
</cite>

## Table of Contents
1. [Introduction](#introduction)
2. [Project Structure](#project-structure)
3. [Core Components](#core-components)
4. [Architecture Overview](#architecture-overview)
5. [Detailed Component Analysis](#detailed-component-analysis)
6. [Dependency Analysis](#dependency-analysis)
7. [Performance Considerations](#performance-considerations)
8. [Troubleshooting Guide](#troubleshooting-guide)
9. [Conclusion](#conclusion)

## Introduction
The evocoders-bot-go application is a Telegram bot designed to facilitate community engagement through automated summarization, profile management, and social interaction features such as random coffee pairings. The system follows a clean, layered architecture with clear separation of concerns between handlers, services, and repositories. This documentation provides a comprehensive overview of the core architectural design, focusing on component interactions, dependency injection patterns, external integrations, and operational workflows.

## Project Structure

```mermaid
graph TD
A[main.go] --> B[internal/bot/bot.go]
B --> C[internal/config/config.go]
B --> D[internal/clients/openai_client.go]
B --> E[internal/database/db.go]
B --> F[internal/handlers]
B --> G[internal/services]
B --> H[internal/tasks]
E --> I[internal/database/migrations]
F --> J[privatehandlers]
F --> K[adminhandlers]
F --> L[grouphandlers]
G --> M[summarization_service.go]
G --> N[random_coffee_service.go]
H --> O[daily_summarization_task.go]
H --> P[random_coffee_poll_task.go]
H --> Q[random_coffee_pairs_task.go]
```

**Diagram sources**
- [main.go](file://main.go#L1-L53)
- [internal/bot/bot.go](file://internal/bot/bot.go#L1-L385)

**Section sources**
- [main.go](file://main.go#L1-L53)
- [internal/bot/bot.go](file://internal/bot/bot.go#L1-L385)

## Core Components

The application is structured around a layered architecture where each layer has a distinct responsibility:

- **Handlers**: Process incoming Telegram updates and route them to appropriate services.
- **Services**: Contain business logic and coordinate operations between repositories and external clients.
- **Repositories**: Handle data persistence and retrieval from the database.
- **Clients**: Interface with external APIs (e.g., OpenAI).
- **Tasks**: Orchestrate scheduled background operations.

This separation ensures modularity, testability, and maintainability.

**Section sources**
- [internal/bot/bot.go](file://internal/bot/bot.go#L17-L385)
- [internal/services/summarization_service.go](file://internal/services/summarization_service.go#L1-L150)
- [internal/services/random_coffee_service.go](file://internal/services/random_coffee_service.go#L1-L200)

## Architecture Overview

```mermaid
graph TD
subgraph "External Systems"
OpenAI[(OpenAI API)]
Telegram[(Telegram API)]
PostgreSQL[(PostgreSQL DB)]
end
subgraph "Application Layers"
Handlers[Handlers Layer]
Services[Services Layer]
Repositories[Repositories Layer]
end
subgraph "Orchestration"
Tasks[Tasks]
Main[main.go]
end
Telegram --> |Updates| Handlers
Handlers --> |Business Logic| Services
Services --> |Data Access| Repositories
Repositories --> |SQL| PostgreSQL
Services --> |API Calls| OpenAI
Tasks --> |Scheduled Execution| Services
Main --> |Bootstrapping| Handlers
Main --> |Bootstrapping| Services
Main --> |Bootstrapping| Repositories
Main --> |Signal Handling| Tasks
```

**Diagram sources**
- [main.go](file://main.go#L1-L53)
- [internal/bot/bot.go](file://internal/bot/bot.go#L1-L385)
- [internal/clients/openai_client.go](file://internal/clients/openai_client.go#L1-L98)
- [internal/database/db.go](file://internal/database/db.go#L1-L45)

## Detailed Component Analysis

### Handler Dependencies and Dependency Injection

The `HandlerDependencies` struct in `bot.go` serves as a dependency injection container that provides all necessary services and repositories to handlers. This pattern enables loose coupling, facilitates testing, and centralizes dependency management.

```mermaid
classDiagram
class HandlerDependencies {
+OpenAiClient *OpenAiClient
+AppConfig *Config
+ProfileService *ProfileService
+SummarizationService *SummarizationService
+RandomCoffeeService *RandomCoffeeService
+MessageSenderService *MessageSenderService
+PermissionsService *PermissionsService
+EventRepository *EventRepository
+UserRepository *UserRepository
+ProfileRepository *ProfileRepository
+RandomCoffeePollRepository *RandomCoffeePollRepository
+GroupMessageRepository *GroupMessageRepository
}
class TgBotClient {
-bot *gotgbot.Bot
-dispatcher *ext.Dispatcher
-updater *ext.Updater
-db *DB
-tasks []Task
+Start()
+Close()
}
TgBotClient --> HandlerDependencies : "creates"
HandlerDependencies --> OpenAiClient : "uses"
HandlerDependencies --> Config : "uses"
HandlerDependencies --> ProfileService : "uses"
HandlerDependencies --> SummarizationService : "uses"
HandlerDependencies --> RandomCoffeeService : "uses"
HandlerDependencies --> MessageSenderService : "uses"
HandlerDependencies --> PermissionsService : "uses"
HandlerDependencies --> EventRepository : "uses"
HandlerDependencies --> UserRepository : "uses"
HandlerDependencies --> ProfileRepository : "uses"
HandlerDependencies --> RandomCoffeePollRepository : "uses"
HandlerDependencies --> GroupMessageRepository : "uses"
```

**Diagram sources**
- [internal/bot/bot.go](file://internal/bot/bot.go#L17-L385)

**Section sources**
- [internal/bot/bot.go](file://internal/bot/bot.go#L17-L385)

### Data Flow from Telegram Update to Repository

The data flow begins with a Telegram update and proceeds through handlers to services and repositories:

```mermaid
sequenceDiagram
participant Telegram as Telegram API
participant Handler as Handler
participant Service as Service
participant Repository as Repository
participant DB as PostgreSQL
Telegram->>Handler : Receive Update
Handler->>Service : Call Business Logic
Service->>Repository : Request Data Operation
Repository->>DB : Execute Query
DB-->>Repository : Return Data
Repository-->>Service : Return Result
Service-->>Handler : Return Response
Handler->>Telegram : Send Message
```

**Diagram sources**
- [internal/bot/bot.go](file://internal/bot/bot.go#L170-L207)
- [internal/handlers/privatehandlers/profile_handler.go](file://internal/handlers/privatehandlers/profile_handler.go#L1-L100)
- [internal/services/profile_service.go](file://internal/services/profile_service.go#L1-L150)
- [internal/database/repositories/user_repository.go](file://internal/database/repositories/user_repository.go#L1-L80)

### Integration with External APIs

The bot integrates with OpenAI for content generation and summarization:

```mermaid
sequenceDiagram
participant Handler as ProfileHandler
participant Service as ProfileService
participant Client as OpenAiClient
participant OpenAI as OpenAI API
Handler->>Service : Request Profile Generation
Service->>Client : GetCompletion(prompt)
Client->>OpenAI : HTTP POST /chat/completions
OpenAI-->>Client : Return Response
Client-->>Service : Return Text
Service-->>Handler : Return Profile
```

**Diagram sources**
- [internal/clients/openai_client.go](file://internal/clients/openai_client.go#L1-L98)
- [internal/services/profile_service.go](file://internal/services/profile_service.go#L1-L150)

## Dependency Analysis

```mermaid
graph TD
main[main.go] --> bot[internal/bot/bot.go]
bot --> config[internal/config/config.go]
bot --> openai[internal/clients/openai_client.go]
bot --> db[internal/database/db.go]
bot --> handlers[internal/handlers]
bot --> services[internal/services]
bot --> tasks[internal/tasks]
services --> repositories[internal/database/repositories]
repositories --> db
tasks --> services
handlers --> services
services --> openai
```

**Diagram sources**
- [main.go](file://main.go#L1-L53)
- [internal/bot/bot.go](file://internal/bot/bot.go#L1-L385)
- [internal/config/config.go](file://internal/config/config.go#L1-L341)
- [internal/clients/openai_client.go](file://internal/clients/openai_client.go#L1-L98)
- [internal/database/db.go](file://internal/database/db.go#L1-L45)

**Section sources**
- [main.go](file://main.go#L1-L53)
- [internal/bot/bot.go](file://internal/bot/bot.go#L1-L385)

## Performance Considerations

The architecture prioritizes responsiveness and scalability through:

- **Goroutine-based concurrency**: Scheduled tasks run in separate goroutines to avoid blocking the main event loop.
- **Connection pooling**: Database connections are managed efficiently via sql.DB.
- **Caching strategies**: While not explicitly implemented, the separation of services allows for easy addition of caching layers.
- **Rate limiting awareness**: External API calls (e.g., OpenAI) are wrapped with context timeouts to prevent hanging requests.

The system is designed to handle multiple concurrent Telegram updates while maintaining scheduled task execution without interference.

## Troubleshooting Guide

Common issues and their resolutions:

**Section sources**
- [main.go](file://main.go#L40-L52)
- [internal/bot/bot.go](file://internal/bot/bot.go#L370-L385)
- [internal/tasks/daily_summarization_task.go](file://internal/tasks/daily_summarization_task.go#L44-L87)
- [internal/tasks/random_coffee_poll_task.go](file://internal/tasks/random_coffee_poll_task.go#L48-L91)
- [internal/tasks/random_coffee_pairs_task.go](file://internal/tasks/random_coffee_pairs_task.go#L47-L87)

## Conclusion

The evocoders-bot-go application demonstrates a well-structured Go application with clean architectural boundaries. The layered design (handlers → services → repositories), combined with dependency injection through the `HandlerDependencies` struct, enables maintainable and testable code. External integrations with Telegram and OpenAI are properly abstracted, while scheduled tasks provide automated functionality without disrupting real-time message processing. The graceful shutdown mechanism ensures data integrity during termination, and the configuration system allows for flexible deployment across environments.