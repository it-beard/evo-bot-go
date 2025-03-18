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
	BotToken         string
	SuperGroupChatID int64
	OpenAIAPIKey     string
	AnonymousUserID  int64

	// Topics Management
	ClosedTopicsIDs   []int
	ForwardingTopicID int
	ToolTopicID       int
	ContentTopicID    int

	// Telegram User Client
	TGUserClientAppID       int
	TGUserClientAppHash     string
	TGUserClientPhoneNumber string
	TGUserClient2FAPass     string
	TGUserClientSessionType string

	// Daily Summarization Feature
	DBConnection             string
	MonitoredTopicsIDs       []int
	SummaryTopicID           int
	SummaryTime              time.Time
	SummarizationTaskEnabled bool
}

// LoadConfig loads the configuration from environment variables
func LoadConfig() (*Config, error) {
	config := &Config{}

	// Basic Bot Configuration
	config.BotToken = os.Getenv("TG_EVO_BOT_TOKEN")
	if config.BotToken == "" {
		return nil, fmt.Errorf("TG_EVO_BOT_TOKEN environment variable is not set")
	}

	supergroupChatIDStr := os.Getenv("TG_EVO_BOT_SUPERGROUP_CHAT_ID")
	if supergroupChatIDStr == "" {
		return nil, fmt.Errorf("TG_EVO_BOT_SUPERGROUP_CHAT_ID environment variable is not set")
	}

	supergroupChatID, err := strconv.ParseInt(supergroupChatIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid supergroup chat ID: %s", supergroupChatIDStr)
	}
	config.SuperGroupChatID = supergroupChatID

	config.OpenAIAPIKey = os.Getenv("TG_EVO_BOT_OPENAI_API_KEY")
	if config.OpenAIAPIKey == "" {
		return nil, fmt.Errorf("TG_EVO_BOT_OPENAI_API_KEY environment variable is not set")
	}

	// Topics Management
	closedTopicsIDsStr := os.Getenv("TG_EVO_BOT_CLOSED_TOPICS_IDS")
	if closedTopicsIDsStr != "" {
		topicIDs := strings.Split(closedTopicsIDsStr, ",")
		for _, topicIDStr := range topicIDs {
			topicID, err := strconv.Atoi(strings.TrimSpace(topicIDStr))
			if err != nil {
				return nil, fmt.Errorf("invalid topic ID in TG_EVO_BOT_CLOSED_TOPICS_IDS: %s", topicIDStr)
			}
			config.ClosedTopicsIDs = append(config.ClosedTopicsIDs, topicID)
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

	forwardingTopicIDStr := os.Getenv("TG_EVO_BOT_FORWARDING_TOPIC_ID")
	if forwardingTopicIDStr != "" {
		forwardingTopicID, err := strconv.Atoi(forwardingTopicIDStr)
		if err != nil {
			return nil, fmt.Errorf("invalid forwarding topic ID: %s", forwardingTopicIDStr)
		}
		config.ForwardingTopicID = forwardingTopicID
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

	// Monitored topic IDs
	monitoredTopicsIDsStr := os.Getenv("TG_EVO_BOT_MONITORED_TOPICS_IDS")
	if monitoredTopicsIDsStr == "" {
		return nil, fmt.Errorf("TG_EVO_BOT_MONITORED_TOPICS_IDS environment variable is not set")
	}

	topicID := strings.Split(monitoredTopicsIDsStr, ",")
	for _, topicIDStr := range topicID {
		topicID, err := strconv.Atoi(strings.TrimSpace(topicIDStr))
		if err != nil {
			return nil, fmt.Errorf("invalid topic ID in TG_EVO_BOT_MONITORED_TOPICS_IDS: %s", topicIDStr)
		}
		config.MonitoredTopicsIDs = append(config.MonitoredTopicsIDs, topicID)
	}

	// Summary topic ID
	summaryTopicIDStr := os.Getenv("TG_EVO_BOT_SUMMARY_TOPIC_ID")
	if summaryTopicIDStr == "" {
		return nil, fmt.Errorf("TG_EVO_BOT_SUMMARY_TOPIC_ID environment variable is not set")
	}

	summaryTopicID, err := strconv.Atoi(summaryTopicIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid summary topic ID: %s", summaryTopicIDStr)
	}
	config.SummaryTopicID = summaryTopicID

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

	// Summarization task enabled/disabled
	summarizationTaskEnabledStr := os.Getenv("TG_EVO_BOT_SUMMARIZATION_TASK_ENABLED")
	if summarizationTaskEnabledStr == "" {
		// Default to enabled if not specified
		config.SummarizationTaskEnabled = true
	} else {
		summarizationTaskEnabled, err := strconv.ParseBool(summarizationTaskEnabledStr)
		if err != nil {
			return nil, fmt.Errorf("invalid summarization task enabled value: %s", summarizationTaskEnabledStr)
		}
		config.SummarizationTaskEnabled = summarizationTaskEnabled
	}

	return config, nil
}
