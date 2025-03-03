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
	// Database configuration
	DBConnection string

	// Chat IDs to monitor (comma-separated list)
	MonitoredChatIDs []int64

	// Target chat for summaries
	SummaryChatID int64

	// Time to run daily summary (24h format)
	SummaryTime time.Time
}

// LoadConfig loads the configuration from environment variables
func LoadConfig() (*Config, error) {
	config := &Config{}

	// Database connection
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
