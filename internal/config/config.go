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
	AdminUserID      int64

	// Topics Management
	ClosedTopicsIDs     []int
	ForwardingTopicID   int
	ToolTopicID         int
	ContentTopicID      int
	AnnouncementTopicID int
	IntroTopicID        int

	// Daily Summarization Feature
	DBConnection             string
	MonitoredTopicsIDs       []int
	SummaryTopicID           int
	SummaryTime              time.Time
	SummarizationTaskEnabled bool

	// Random Coffee Feature
	RandomCoffeeTopicID int

	RandomCoffeePollTaskEnabled bool
	RandomCoffeePollTime        time.Time
	RandomCoffeePollDay         time.Weekday

	RandomCoffeePairsTaskEnabled bool
	RandomCoffeePairsTime        time.Time
	RandomCoffeePairsDay         time.Weekday
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

	adminUserIDStr := os.Getenv("TG_EVO_BOT_ADMIN_USER_ID")
	if adminUserIDStr != "" {
		adminUserID, err := strconv.ParseInt(adminUserIDStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid admin user ID: %s", adminUserIDStr)
		}
		config.AdminUserID = adminUserID
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

	announcementTopicIDStr := os.Getenv("TG_EVO_BOT_ANNOUNCEMENT_TOPIC_ID")
	if announcementTopicIDStr != "" {
		announcementTopicID, err := strconv.Atoi(announcementTopicIDStr)
		if err != nil {
			return nil, fmt.Errorf("invalid announcement topic ID: %s", announcementTopicIDStr)
		}
		config.AnnouncementTopicID = announcementTopicID
	}

	introTopicIDStr := os.Getenv("TG_EVO_BOT_INTRO_TOPIC_ID")
	if introTopicIDStr != "" {
		introTopicID, err := strconv.Atoi(introTopicIDStr)
		if err != nil {
			return nil, fmt.Errorf("invalid intro topic ID: %s", introTopicIDStr)
		}
		config.IntroTopicID = introTopicID
	}

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

	// Random coffee topic ID
	randomCoffeeTopicIDStr := os.Getenv("TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID")
	if randomCoffeeTopicIDStr == "" {
		return nil, fmt.Errorf("TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID environment variable is not set")
	}

	randomCoffeeTopicID, err := strconv.Atoi(randomCoffeeTopicIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid random coffee topic ID: %s", randomCoffeeTopicIDStr)
	}
	config.RandomCoffeeTopicID = randomCoffeeTopicID

	// Random Coffee Poll Feature
	randomCoffeePollTaskEnabledStr := os.Getenv("TG_EVO_BOT_RANDOM_COFFEE_POLL_TASK_ENABLED")
	if randomCoffeePollTaskEnabledStr == "" {
		// Default to enabled if not specified
		config.RandomCoffeePollTaskEnabled = true
	} else {
		randomCoffeePollTaskEnabled, err := strconv.ParseBool(randomCoffeePollTaskEnabledStr)
		if err != nil {
			return nil, fmt.Errorf("invalid random coffee poll task enabled value: %s", randomCoffeePollTaskEnabledStr)
		}
		config.RandomCoffeePollTaskEnabled = randomCoffeePollTaskEnabled
	}

	// Meeting poll time
	randomCoffeePollTimeStr := os.Getenv("TG_EVO_BOT_RANDOM_COFFEE_POLL_TIME")
	if randomCoffeePollTimeStr == "" {
		// Default to 2:00 PM if not specified
		randomCoffeePollTimeStr = "14:00"
	}

	// Parse the time in 24-hour format
	randomCoffeePollTime, err := time.Parse("15:04", randomCoffeePollTimeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid random coffee poll time format: %s", randomCoffeePollTimeStr)
	}
	config.RandomCoffeePollTime = randomCoffeePollTime

	// Meeting poll day
	randomCoffeePollDayStr := os.Getenv("TG_EVO_BOT_RANDOM_COFFEE_POLL_DAY")
	if randomCoffeePollDayStr == "" {
		// Default to Friday if not specified
		config.RandomCoffeePollDay = time.Friday
	} else {
		switch strings.ToLower(randomCoffeePollDayStr) {
		case "sunday":
			config.RandomCoffeePollDay = time.Sunday
		case "monday":
			config.RandomCoffeePollDay = time.Monday
		case "tuesday":
			config.RandomCoffeePollDay = time.Tuesday
		case "wednesday":
			config.RandomCoffeePollDay = time.Wednesday
		case "thursday":
			config.RandomCoffeePollDay = time.Thursday
		case "friday":
			config.RandomCoffeePollDay = time.Friday
		case "saturday":
			config.RandomCoffeePollDay = time.Saturday
		default:
			return nil, fmt.Errorf("invalid random coffee poll day: %s (valid values: sunday, monday, tuesday, wednesday, thursday, friday, saturday)", randomCoffeePollDayStr)
		}
	}

	// Random Coffee Pairs Feature
	randomCoffeePairsTaskEnabledStr := os.Getenv("TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TASK_ENABLED")
	if randomCoffeePairsTaskEnabledStr == "" {
		// Default to enabled if not specified
		config.RandomCoffeePairsTaskEnabled = true
	} else {
		randomCoffeePairsTaskEnabled, err := strconv.ParseBool(randomCoffeePairsTaskEnabledStr)
		if err != nil {
			return nil, fmt.Errorf("invalid random coffee pairs task enabled value: %s", randomCoffeePairsTaskEnabledStr)
		}
		config.RandomCoffeePairsTaskEnabled = randomCoffeePairsTaskEnabled
	}

	// Pairs generation time
	randomCoffeePairsTimeStr := os.Getenv("TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TIME")
	if randomCoffeePairsTimeStr == "" {
		// Default to 12:00 PM if not specified
		randomCoffeePairsTimeStr = "12:00"
	}

	// Parse the time in 24-hour format
	randomCoffeePairsTime, err := time.Parse("15:04", randomCoffeePairsTimeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid random coffee pairs time format: %s", randomCoffeePairsTimeStr)
	}
	config.RandomCoffeePairsTime = randomCoffeePairsTime

	// Pairs generation day
	randomCoffeePairsDayStr := os.Getenv("TG_EVO_BOT_RANDOM_COFFEE_PAIRS_DAY")
	if randomCoffeePairsDayStr == "" {
		// Default to Monday if not specified
		config.RandomCoffeePairsDay = time.Monday
	} else {
		switch strings.ToLower(randomCoffeePairsDayStr) {
		case "sunday":
			config.RandomCoffeePairsDay = time.Sunday
		case "monday":
			config.RandomCoffeePairsDay = time.Monday
		case "tuesday":
			config.RandomCoffeePairsDay = time.Tuesday
		case "wednesday":
			config.RandomCoffeePairsDay = time.Wednesday
		case "thursday":
			config.RandomCoffeePairsDay = time.Thursday
		case "friday":
			config.RandomCoffeePairsDay = time.Friday
		case "saturday":
			config.RandomCoffeePairsDay = time.Saturday
		default:
			return nil, fmt.Errorf("invalid random coffee pairs day: %s (valid values: sunday, monday, tuesday, wednesday, thursday, friday, saturday)", randomCoffeePairsDayStr)
		}
	}

	return config, nil
}
