package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the application configuration
type Config struct {
	// Basic Bot Configuration
	BotToken     string
	MainChatID   int64
	OpenAIAPIKey string

	// Chat Management
	ClosedThreadsIDs   []int64
	AnonymousUserID    int64
	ForwardingThreadID int64
	ToolTopicID        int
	ContentTopicID     int

	// Telegram User Client
	TGUserClientAppID       int
	TGUserClientAppHash     string
	TGUserClientPhoneNumber string
	TGUserClient2FAPass     string
	TGUserClientSessionType string

	// Daily Summarization Feature
	DBConnection     string
	MonitoredChatIDs []int64
	SummaryChatID    int64
	SummaryTime      time.Time
}

// LoadConfig loads the configuration from environment variables
func LoadConfig() (*Config, error) {
	config := &Config{}

	// Basic Bot Configuration
	config.BotToken = os.Getenv("TG_EVO_BOT_TOKEN")
	if config.BotToken == "" {
		return nil, fmt.Errorf("TG_EVO_BOT_TOKEN environment variable is not set")
	}

	mainChatIDStr := os.Getenv("TG_EVO_BOT_MAIN_CHAT_ID")
	if mainChatIDStr == "" {
		return nil, fmt.Errorf("TG_EVO_BOT_MAIN_CHAT_ID environment variable is not set")
	}

	mainChatID, err := strconv.ParseInt(mainChatIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid main chat ID: %s", mainChatIDStr)
	}
	config.MainChatID = mainChatID

	config.OpenAIAPIKey = os.Getenv("TG_EVO_BOT_OPENAI_API_KEY")
	if config.OpenAIAPIKey == "" {
		return nil, fmt.Errorf("TG_EVO_BOT_OPENAI_API_KEY environment variable is not set")
	}

	// Chat Management
	closedThreadsIDsStr := os.Getenv("TG_EVO_BOT_CLOSED_THREADS_IDS")
	if closedThreadsIDsStr != "" {
		threadIDs := strings.Split(closedThreadsIDsStr, ",")
		for _, threadIDStr := range threadIDs {
			threadID, err := strconv.ParseInt(strings.TrimSpace(threadIDStr), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid thread ID in TG_EVO_BOT_CLOSED_THREADS_IDS: %s", threadIDStr)
			}
			config.ClosedThreadsIDs = append(config.ClosedThreadsIDs, threadID)
		}
	}

	anonymousUserIDStr := os.Getenv("TG_EVO_BOT_ANONYMOUS_USER_ID")
	if anonymousUserIDStr != "" {
		anonymousUserID, err := strconv.ParseInt(anonymousUserIDStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid anonymous user ID: %s", anonymousUserIDStr)
		}
		config.AnonymousUserID = anonymousUserID
	}

	forwardingThreadIDStr := os.Getenv("TG_EVO_BOT_FORWARDING_THREAD_ID")
	if forwardingThreadIDStr != "" {
		forwardingThreadID, err := strconv.ParseInt(forwardingThreadIDStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid forwarding thread ID: %s", forwardingThreadIDStr)
		}
		config.ForwardingThreadID = forwardingThreadID
	}

	toolTopicIDStr := os.Getenv("TG_EVO_BOT_TOOL_TOPIC_ID")
	if toolTopicIDStr != "" {
		toolTopicID, err := strconv.Atoi(toolTopicIDStr)
		if err != nil {
			return nil, fmt.Errorf("invalid tool topic ID: %s", toolTopicIDStr)
		}
		config.ToolTopicID = toolTopicID
	}

	contentTopicIDStr := os.Getenv("TG_EVO_BOT_CONTENT_TOPIC_ID")
	if contentTopicIDStr != "" {
		contentTopicID, err := strconv.Atoi(contentTopicIDStr)
		if err != nil {
			return nil, fmt.Errorf("invalid content topic ID: %s", contentTopicIDStr)
		}
		config.ContentTopicID = contentTopicID
	}

	// Telegram User Client
	tgUserClientAppIDStr := os.Getenv("TG_EVO_BOT_TGUSERCLIENT_APPID")
	if tgUserClientAppIDStr != "" {
		tgUserClientAppID, err := strconv.Atoi(tgUserClientAppIDStr)
		if err != nil {
			return nil, fmt.Errorf("invalid Telegram User Client App ID: %s", tgUserClientAppIDStr)
		}
		config.TGUserClientAppID = tgUserClientAppID
	}

	config.TGUserClientAppHash = os.Getenv("TG_EVO_BOT_TGUSERCLIENT_APPHASH")
	config.TGUserClientPhoneNumber = os.Getenv("TG_EVO_BOT_TGUSERCLIENT_PHONENUMBER")
	config.TGUserClient2FAPass = os.Getenv("TG_EVO_BOT_TGUSERCLIENT_2FAPASS")
	config.TGUserClientSessionType = os.Getenv("TG_EVO_BOT_TGUSERCLIENT_SESSION_TYPE")

	// Daily Summarization Feature
	config.DBConnection = os.Getenv("TG_EVO_BOT_DB_CONNECTION")
	if config.DBConnection == "" {
		return nil, fmt.Errorf("TG_EVO_BOT_DB_CONNECTION environment variable is not set")
	}

	// Monitored chat IDs
	monitoredChatIDsStr := os.Getenv("TG_EVO_BOT_MONITORED_CHAT_IDS")
	if monitoredChatIDsStr == "" {
		return nil, fmt.Errorf("TG_EVO_BOT_MONITORED_CHAT_IDS environment variable is not set")
	}

	chatIDs := strings.Split(monitoredChatIDsStr, ",")
	for _, chatIDStr := range chatIDs {
		chatID, err := strconv.ParseInt(strings.TrimSpace(chatIDStr), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid chat ID in TG_EVO_BOT_MONITORED_CHAT_IDS: %s", chatIDStr)
		}
		config.MonitoredChatIDs = append(config.MonitoredChatIDs, chatID)
	}

	// Summary chat ID
	summaryChatIDStr := os.Getenv("TG_EVO_BOT_SUMMARY_CHAT_ID")
	if summaryChatIDStr == "" {
		return nil, fmt.Errorf("TG_EVO_BOT_SUMMARY_CHAT_ID environment variable is not set")
	}

	summaryChatID, err := strconv.ParseInt(summaryChatIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid summary chat ID: %s", summaryChatIDStr)
	}
	config.SummaryChatID = summaryChatID

	// Summary time
	summaryTimeStr := os.Getenv("TG_EVO_BOT_SUMMARY_TIME")
	if summaryTimeStr == "" {
		// Default to 3:00 AM if not specified
		summaryTimeStr = "03:00"
	}

	// Parse the time in 24-hour format
	summaryTime, err := time.Parse("15:04", summaryTimeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid summary time format: %s", summaryTimeStr)
	}
	config.SummaryTime = summaryTime

	return config, nil
}

// IsMonitoredChat checks if a chat ID is in the monitored list
func (c *Config) IsMonitoredChat(chatID int64) bool {
	for _, id := range c.MonitoredChatIDs {
		if id == chatID {
			return true
		}
	}
	return false
}
