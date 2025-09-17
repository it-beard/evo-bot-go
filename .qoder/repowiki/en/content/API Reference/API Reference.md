# API Reference

<cite>
**Referenced Files in This Document**   
- [config.go](file://internal/config/config.go)
- [help_handler.go](file://internal/handlers/privatehandlers/help_handler.go)
- [profile_handler.go](file://internal/handlers/privatehandlers/profile_handler.go)
- [content_handler.go](file://internal/handlers/privatehandlers/content_handler.go)
- [tools_handler.go](file://internal/handlers/privatehandlers/tools_handler.go)
- [intro_handler.go](file://internal/handlers/privatehandlers/intro_handler.go)
- [handlers_admin_constants.go](file://internal/constants/handlers_admin_constants.go)
</cite>

## Table of Contents
1. [Introduction](#introduction)
2. [Telegram Bot Commands](#telegram-bot-commands)
3. [Configuration API](#configuration-api)
4. [Authentication and Authorization](#authentication-and-authorization)
5. [Rate Limiting and Performance](#rate-limiting-and-performance)
6. [Versioning and Migration](#versioning-and-migration)
7. [Client Implementation Guidelines](#client-implementation-guidelines)
8. [Troubleshooting Guide](#troubleshooting-guide)

## Introduction
The evocoders-bot-go application provides a Telegram bot interface for managing the Evocoders Club community. The primary API consists of Telegram bot commands that enable users to search for information, manage profiles, and access AI-powered features. This documentation details the available commands, configuration options, authentication mechanisms, and best practices for using the bot effectively.

## Telegram Bot Commands
The bot provides several commands accessible through direct messages. These commands follow the standard Telegram bot command pattern (/command) and are designed to provide information retrieval and profile management capabilities.

### Profile Command
The `/profile` command allows users to manage their personal profile information.

**Invocation**: `/profile`  
**Parameters**: None  
**Response Format**: Interactive menu with options to view, edit, and publish profile information. The response includes HTML-formatted text with inline keyboard buttons for navigation.

**Functionality**:
- View and edit personal information (name, bio)
- Publish profile to the designated "Intro" topic
- Search for other club members' profiles
- Validate profile completeness before publication

**Section sources**
- [profile_handler.go](file://internal/handlers/privatehandlers/profile_handler.go#L151-L175)

### Content Search Command
The `/content` command searches through designated topics for relevant information using AI-powered analysis.

**Invocation**: `/content`  
**Parameters**: Search query (text input after command invocation)  
**Response Format**: Markdown-formatted response with relevant information, sources, and context from the content topic.

**Functionality**:
- Accepts a search query from the user
- Retrieves messages from the configured content topic
- Uses OpenAI to analyze and summarize relevant information
- Returns AI-generated response with contextual information

**Section sources**
- [content_handler.go](file://internal/handlers/privatehandlers/content_handler.go#L78-L92)

### Tool Search Command
The `/tool` command finds relevant AI tools based on user queries.

**Invocation**: `/tool`  
**Parameters**: Search query (text input after command invocation)  
**Response Format**: Markdown-formatted response listing relevant tools with descriptions, use cases, and links.

**Functionality**:
- Accepts a tool-related search query
- Analyzes the tool database topic using OpenAI
- Generates a comprehensive response with recommended tools
- Provides context and usage information for each tool

**Section sources**
- [tools_handler.go](file://internal/handlers/privatehandlers/tools_handler.go#L78-L92)

### Introduction Search Command
The `/intro` command provides information about club members and their introductions.

**Invocation**: `/intro`  
**Parameters**: Optional search query (text input after command invocation)  
**Response Format**: Markdown-formatted response with member information, bios, and relevant details.

**Functionality**:
- Searches through member introductions and profiles
- Returns information about club members based on the query
- Can provide general club information when no query is specified
- Uses AI to synthesize relevant information from multiple profiles

**Section sources**
- [intro_handler.go](file://internal/handlers/privatehandlers/intro_handler.go#L78-L92)

### Help Command
The `/help` command provides usage information and available commands.

**Invocation**: `/help`  
**Parameters**: None  
**Response Format**: HTML-formatted help message with command descriptions and usage instructions.

**Functionality**:
- Displays available commands and their purposes
- Shows different options based on user permissions (admin vs. regular member)
- Provides guidance on how to use the bot's features

**Section sources**
- [help_handler.go](file://internal/handlers/privatehandlers/help_handler.go#L28-L45)

## Configuration API
The bot's behavior is configured through environment variables that control various aspects of its functionality.

### Basic Bot Configuration
These variables define the core bot settings and credentials.

**Environment Variables**:
- `TG_EVO_BOT_TOKEN`: Bot authentication token (required)
- `TG_EVO_BOT_SUPERGROUP_CHAT_ID`: Supergroup chat ID (required)
- `TG_EVO_BOT_OPENAI_API_KEY`: OpenAI API key (required)
- `TG_EVO_BOT_ADMIN_USER_ID`: Administrator user ID for admin commands

**Section sources**
- [config.go](file://internal/config/config.go#L15-L24)

### Topics Management
These variables configure the bot's interaction with different topics in the Telegram group.

**Environment Variables**:
- `TG_EVO_BOT_CLOSED_TOPICS_IDS`: Comma-separated list of topic IDs closed for chatting
- `TG_EVO_BOT_FORWARDING_TOPIC_ID`: Topic ID for forwarding replies from closed threads
- `TG_EVO_BOT_TOOL_TOPIC_ID`: Topic ID for the AI tools database
- `TG_EVO_BOT_CONTENT_TOPIC_ID`: Topic ID for the content topic
- `TG_EVO_BOT_INTRO_TOPIC_ID`: Topic ID for club introductions and member information
- `TG_EVO_BOT_ANNOUNCEMENT_TOPIC_ID`: Topic ID for announcements

**Section sources**
- [config.go](file://internal/config/config.go#L26-L37)

### Daily Summarization Feature
These variables control the daily chat summarization functionality.

**Environment Variables**:
- `TG_EVO_BOT_DB_CONNECTION`: PostgreSQL connection string (required)
- `TG_EVO_BOT_MONITORED_TOPICS_IDS`: Comma-separated list of topic IDs to monitor for summarization (required)
- `TG_EVO_BOT_SUMMARY_TOPIC_ID`: Topic ID where daily summaries will be posted (required)
- `TG_EVO_BOT_SUMMARY_TIME`: Time to run daily summary in 24-hour format (default: "03:00")
- `TG_EVO_BOT_SUMMARIZATION_TASK_ENABLED`: Enable/disable daily summarization task (default: true)

**Section sources**
- [config.go](file://internal/config/config.go#L47-L57)

### Random Coffee Feature
These variables configure the weekly random coffee meeting automation.

**Environment Variables**:
- `TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID`: Topic ID for random coffee polls and pairs (required)
- `TG_EVO_BOT_RANDOM_COFFEE_POLL_TASK_ENABLED`: Enable/disable weekly coffee poll task (default: true)
- `TG_EVO_BOT_RANDOM_COFFEE_POLL_TIME`: Time to send weekly coffee poll in 24-hour format UTC (default: "14:00")
- `TG_EVO_BOT_RANDOM_COFFEE_POLL_DAY`: Day of week to send poll (default: "friday")
- `TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TASK_ENABLED`: Enable/disable automatic pairs generation (default: true)
- `TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TIME`: Time to generate coffee pairs in 24-hour format UTC (default: "12:00")
- `TG_EVO_BOT_RANDOM_COFFEE_PAIRS_DAY`: Day of week to generate pairs (default: "monday")

**Section sources**
- [config.go](file://internal/config/config.go#L59-L73)

## Authentication and Authorization
The bot implements role-based access control to protect administrative functionality.

### Admin Command Authentication
Admin commands require user ID verification to ensure only authorized users can execute them.

**Implementation**:
- Admin privileges are determined by comparing the user's Telegram ID with the `TG_EVO_BOT_ADMIN_USER_ID` environment variable
- Commands like `/profilesManager` and `/showTopics` are restricted to users with admin privileges
- The `IsUserAdminOrCreator` utility function performs the verification

**Admin Commands**:
- `/profilesManager`: Manage user profiles (admin only)
- `/showTopics`: Show and manage topics (admin only)
- `/trySummarize`: Test summarization functionality (admin only)

**Section sources**
- [handlers_admin_constants.go](file://internal/constants/handlers_admin_constants.go#L25-L35)
- [config.go](file://internal/config/config.go#L19-L20)

### Club Member Verification
Certain commands are restricted to club members only, ensuring that only community members can access specific features.

**Verification Process**:
- The bot checks if the user is a member of the Evocoders Club
- Non-members receive limited functionality and are encouraged to join
- Membership status is determined through Telegram group membership

**Section sources**
- [profile_handler.go](file://internal/handlers/privatehandlers/profile_handler.go#L160-L163)

## Rate Limiting and Performance
The bot implements several mechanisms to manage AI-powered features and ensure optimal performance.

### AI Feature Rate Limiting
To prevent abuse and manage API costs, AI-powered features include rate limiting considerations.

**Rate Limiting Strategies**:
- Sequential request blocking: Prevents multiple concurrent requests from the same user
- Context cancellation: Allows users to cancel ongoing AI operations with the `/cancel` command
- Typing indicators: Provides feedback during long-running operations to improve user experience

**Implementation Details**:
- User-specific processing flags prevent concurrent requests
- Cancellable contexts allow graceful termination of AI operations
- Periodic typing actions maintain user engagement during processing

**Section sources**
- [content_handler.go](file://internal/handlers/privatehandlers/content_handler.go#L118-L125)
- [tools_handler.go](file://internal/handlers/privatehandlers/tools_handler.go#L118-L125)
- [intro_handler.go](file://internal/handlers/privatehandlers/intro_handler.go#L118-L125)

### Performance Optimization
The bot includes several performance optimizations for frequent queries.

**Optimization Techniques**:
- Efficient data preparation: Messages and profiles are formatted efficiently before AI processing
- Connection management: Database and API connections are managed to minimize latency
- Error handling: Comprehensive error handling prevents cascading failures
- Logging: Selective logging helps diagnose performance issues without impacting speed

**Best Practices for Users**:
- Use specific search queries to reduce processing time
- Avoid submitting multiple requests simultaneously
- Cancel long-running operations if no longer needed
- Use the `/cancel` command to terminate ongoing processes

**Section sources**
- [content_handler.go](file://internal/handlers/privatehandlers/content_handler.go#L180-L250)
- [tools_handler.go](file://internal/handlers/privatehandlers/tools_handler.go#L180-L250)

## Versioning and Migration
The bot's command interface evolves over time, with mechanisms in place for managing changes.

### Command Versioning
Commands are versioned through the codebase structure and migration system.

**Versioning Approach**:
- Database migrations track schema changes over time
- Command constants are defined in dedicated files for easy reference
- New features are added without breaking existing functionality
- Backwards compatibility is maintained for existing commands

**Migration Tracking**:
- Database migrations are stored in `/internal/database/migrations/implementations/`
- Each migration includes a timestamp and descriptive name
- Migration history is tracked in the `migrations` database table

**Section sources**
- [config.go](file://internal/config/config.go)
- [handlers_admin_constants.go](file://internal/constants/handlers_admin_constants.go)

### Backwards Compatibility
The bot maintains backwards compatibility for existing commands and configurations.

**Compatibility Measures**:
- Default values for optional configuration variables
- Graceful handling of missing or invalid configuration
- Non-breaking updates to existing commands
- Clear error messages for deprecated or invalid usage

**Deprecation Policy**:
- Commands are not removed without notice
- Deprecated commands continue to function with warnings
- Migration guides are provided for configuration changes
- Users are notified of upcoming changes through announcements

## Client Implementation Guidelines
This section provides guidance for users and developers on how to effectively use the bot's API.

### User Guidelines
For end users interacting with the bot through Telegram.

**Best Practices**:
- Use clear and specific search queries for better results
- Keep profile bios within the character limit (defined in constants)
- Publish profiles only when all required information is complete
- Use the `/cancel` command to stop long-running operations
- Check the `/help` command for updated usage instructions

**Command Flow**:
1. Start with `/help` to understand available commands
2. Use `/profile` to set up and manage your profile
3. Use `/content`, `/tool`, or `/intro` for information retrieval
4. Use `/cancel` if a command takes too long to respond

### Developer Guidelines
For developers integrating with or extending the bot's functionality.

**Integration Tips**:
- Use environment variables for configuration to enable easy deployment
- Follow the existing handler pattern when adding new commands
- Use the provided service classes for common operations
- Implement proper error handling in new features
- Add comprehensive logging for debugging purposes

**Extension Points**:
- Add new commands by creating handler classes in the appropriate directory
- Modify AI prompts by updating the database templates
- Extend functionality by adding new service methods
- Customize behavior through configuration variables

## Troubleshooting Guide
This section addresses common issues and their solutions.

### Configuration Issues
**Problem**: Bot fails to start with configuration errors  
**Solution**: Verify all required environment variables are set, particularly `TG_EVO_BOT_TOKEN`, `TG_EVO_BOT_SUPERGROUP_CHAT_ID`, `TG_EVO_BOT_OPENAI_API_KEY`, and `TG_EVO_BOT_DB_CONNECTION`

**Problem**: Features not working as expected  
**Solution**: Check that topic IDs in configuration match the actual Telegram topic IDs and that the bot has necessary permissions in the group

**Section sources**
- [config.go](file://internal/config/config.go)

### AI Feature Issues
**Problem**: AI commands return errors or timeout  
**Solution**: Verify the OpenAI API key is valid and has sufficient quota; check network connectivity

**Problem**: AI responses are irrelevant or incomplete  
**Solution**: Refine the search query with more specific terms; check that the relevant topic contains sufficient information

**Section sources**
- [content_handler.go](file://internal/handlers/privatehandlers/content_handler.go)
- [tools_handler.go](file://internal/handlers/privatehandlers/tools_handler.go)

### Permission Issues
**Problem**: Admin commands not accessible  
**Solution**: Verify the `TG_EVO_BOT_ADMIN_USER_ID` matches the user's Telegram ID and that the user is interacting with the bot in private chat

**Problem**: Club member features inaccessible  
**Solution**: Ensure the user is a member of the Evocoders Club Telegram group

**Section sources**
- [profile_handler.go](file://internal/handlers/privatehandlers/profile_handler.go#L160-L163)