package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// allEnvVars lists every environment variable read by LoadConfig.
var allEnvVars = []string{
	"TG_EVO_BOT_TOKEN",
	"TG_EVO_BOT_SUPERGROUP_CHAT_ID",
	"TG_EVO_BOT_OPENAI_API_KEY",
	"TG_EVO_BOT_CLOSED_TOPICS_IDS",
	"TG_EVO_BOT_ADMIN_USER_ID",
	"TG_EVO_BOT_FORWARDING_TOPIC_ID",
	"TG_EVO_BOT_TOOL_TOPIC_ID",
	"TG_EVO_BOT_CONTENT_TOPIC_ID",
	"TG_EVO_BOT_ANNOUNCEMENT_TOPIC_ID",
	"TG_EVO_BOT_INTRO_TOPIC_ID",
	"TG_EVO_BOT_DB_CONNECTION",
	"TG_EVO_BOT_MONITORED_TOPICS_IDS",
	"TG_EVO_BOT_SUMMARY_TOPIC_ID",
	"TG_EVO_BOT_SUMMARY_TIME",
	"TG_EVO_BOT_SUMMARIZATION_TASK_ENABLED",
	"TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID",
	"TG_EVO_BOT_RANDOM_COFFEE_INTERVAL_WEEKS",
	"TG_EVO_BOT_RANDOM_COFFEE_POLL_TASK_ENABLED",
	"TG_EVO_BOT_RANDOM_COFFEE_POLL_TIME",
	"TG_EVO_BOT_RANDOM_COFFEE_POLL_DAY",
	"TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TASK_ENABLED",
	"TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TIME",
	"TG_EVO_BOT_RANDOM_COFFEE_PAIRS_DAY",
}

// requiredEnv holds valid values for every mandatory environment variable.
var requiredEnv = map[string]string{
	"TG_EVO_BOT_TOKEN":                  "test-token",
	"TG_EVO_BOT_SUPERGROUP_CHAT_ID":     "1234567890",
	"TG_EVO_BOT_OPENAI_API_KEY":         "sk-test",
	"TG_EVO_BOT_DB_CONNECTION":          "postgres://user:pass@localhost/db",
	"TG_EVO_BOT_MONITORED_TOPICS_IDS":   "10,20",
	"TG_EVO_BOT_SUMMARY_TOPIC_ID":       "30",
	"TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID": "40",
}

// setupEnv clears every config env var (so the host environment cannot leak in),
// sets the required ones to valid values, and then applies overrides.
// An override with an empty value unsets that variable.
func setupEnv(t *testing.T, overrides map[string]string) {
	t.Helper()
	for _, key := range allEnvVars {
		t.Setenv(key, "")
	}
	for key, value := range requiredEnv {
		t.Setenv(key, value)
	}
	for key, value := range overrides {
		t.Setenv(key, value)
	}
}

func mustParseClock(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("15:04", value)
	require.NoError(t, err)
	return parsed
}

func TestLoadConfig_RequiredOnly_AppliesDefaults(t *testing.T) {
	setupEnv(t, nil)

	cfg, err := LoadConfig()

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "test-token", cfg.BotToken)
	assert.Equal(t, int64(1234567890), cfg.SuperGroupChatID)
	assert.Equal(t, "sk-test", cfg.OpenAIAPIKey)
	assert.Equal(t, "postgres://user:pass@localhost/db", cfg.DBConnection)
	assert.Equal(t, []int{10, 20}, cfg.MonitoredTopicsIDs)
	assert.Equal(t, 30, cfg.SummaryTopicID)
	assert.Equal(t, 40, cfg.RandomCoffeeTopicID)

	// Optional values stay at zero values
	assert.Nil(t, cfg.ClosedTopicsIDs)
	assert.Zero(t, cfg.AdminUserID)
	assert.Zero(t, cfg.ForwardingTopicID)
	assert.Zero(t, cfg.ToolTopicID)
	assert.Zero(t, cfg.ContentTopicID)
	assert.Zero(t, cfg.AnnouncementTopicID)
	assert.Zero(t, cfg.IntroTopicID)

	// Defaults
	assert.Equal(t, mustParseClock(t, "03:00"), cfg.SummaryTime)
	assert.True(t, cfg.SummarizationTaskEnabled)
	assert.Equal(t, 1, cfg.RandomCoffeeIntervalWeeks)
	assert.True(t, cfg.RandomCoffeePollTaskEnabled)
	assert.Equal(t, mustParseClock(t, "14:00"), cfg.RandomCoffeePollTime)
	assert.Equal(t, time.Friday, cfg.RandomCoffeePollDay)
	assert.True(t, cfg.RandomCoffeePairsTaskEnabled)
	assert.Equal(t, mustParseClock(t, "12:00"), cfg.RandomCoffeePairsTime)
	assert.Equal(t, time.Monday, cfg.RandomCoffeePairsDay)
}

func TestLoadConfig_AllVariablesSet(t *testing.T) {
	setupEnv(t, map[string]string{
		"TG_EVO_BOT_SUPERGROUP_CHAT_ID":               "-1001234567890",
		"TG_EVO_BOT_CLOSED_TOPICS_IDS":                "1, 2 ,3",
		"TG_EVO_BOT_ADMIN_USER_ID":                    "987654321",
		"TG_EVO_BOT_FORWARDING_TOPIC_ID":              "11",
		"TG_EVO_BOT_TOOL_TOPIC_ID":                    "12",
		"TG_EVO_BOT_CONTENT_TOPIC_ID":                 "13",
		"TG_EVO_BOT_ANNOUNCEMENT_TOPIC_ID":            "14",
		"TG_EVO_BOT_INTRO_TOPIC_ID":                   "15",
		"TG_EVO_BOT_MONITORED_TOPICS_IDS":             " 5 , 6,7 ",
		"TG_EVO_BOT_SUMMARY_TOPIC_ID":                 "16",
		"TG_EVO_BOT_SUMMARY_TIME":                     "22:15",
		"TG_EVO_BOT_SUMMARIZATION_TASK_ENABLED":       "false",
		"TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID":           "17",
		"TG_EVO_BOT_RANDOM_COFFEE_INTERVAL_WEEKS":     "3",
		"TG_EVO_BOT_RANDOM_COFFEE_POLL_TASK_ENABLED":  "0",
		"TG_EVO_BOT_RANDOM_COFFEE_POLL_TIME":          "09:05",
		"TG_EVO_BOT_RANDOM_COFFEE_POLL_DAY":           "Wednesday",
		"TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TASK_ENABLED": "FALSE",
		"TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TIME":         "18:45",
		"TG_EVO_BOT_RANDOM_COFFEE_PAIRS_DAY":          "SATURDAY",
	})

	cfg, err := LoadConfig()

	require.NoError(t, err)
	assert.Equal(t, int64(-1001234567890), cfg.SuperGroupChatID)
	assert.Equal(t, []int{1, 2, 3}, cfg.ClosedTopicsIDs)
	assert.Equal(t, int64(987654321), cfg.AdminUserID)
	assert.Equal(t, 11, cfg.ForwardingTopicID)
	assert.Equal(t, 12, cfg.ToolTopicID)
	assert.Equal(t, 13, cfg.ContentTopicID)
	assert.Equal(t, 14, cfg.AnnouncementTopicID)
	assert.Equal(t, 15, cfg.IntroTopicID)
	assert.Equal(t, []int{5, 6, 7}, cfg.MonitoredTopicsIDs)
	assert.Equal(t, 16, cfg.SummaryTopicID)
	assert.Equal(t, mustParseClock(t, "22:15"), cfg.SummaryTime)
	assert.False(t, cfg.SummarizationTaskEnabled)
	assert.Equal(t, 17, cfg.RandomCoffeeTopicID)
	assert.Equal(t, 3, cfg.RandomCoffeeIntervalWeeks)
	assert.False(t, cfg.RandomCoffeePollTaskEnabled)
	assert.Equal(t, mustParseClock(t, "09:05"), cfg.RandomCoffeePollTime)
	assert.Equal(t, time.Wednesday, cfg.RandomCoffeePollDay)
	assert.False(t, cfg.RandomCoffeePairsTaskEnabled)
	assert.Equal(t, mustParseClock(t, "18:45"), cfg.RandomCoffeePairsTime)
	assert.Equal(t, time.Saturday, cfg.RandomCoffeePairsDay)
}

func TestLoadConfig_TaskFlagsExplicitlyEnabled(t *testing.T) {
	setupEnv(t, map[string]string{
		"TG_EVO_BOT_SUMMARIZATION_TASK_ENABLED":       "true",
		"TG_EVO_BOT_RANDOM_COFFEE_POLL_TASK_ENABLED":  "1",
		"TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TASK_ENABLED": "T",
	})

	cfg, err := LoadConfig()

	require.NoError(t, err)
	assert.True(t, cfg.SummarizationTaskEnabled)
	assert.True(t, cfg.RandomCoffeePollTaskEnabled)
	assert.True(t, cfg.RandomCoffeePairsTaskEnabled)
}

func TestLoadConfig_SingleMonitoredTopic(t *testing.T) {
	setupEnv(t, map[string]string{"TG_EVO_BOT_MONITORED_TOPICS_IDS": "42"})

	cfg, err := LoadConfig()

	require.NoError(t, err)
	assert.Equal(t, []int{42}, cfg.MonitoredTopicsIDs)
}

func TestLoadConfig_Weekdays_ParsedCaseInsensitive(t *testing.T) {
	days := map[string]time.Weekday{
		"sunday":    time.Sunday,
		"Monday":    time.Monday,
		"TUESDAY":   time.Tuesday,
		"wednesday": time.Wednesday,
		"Thursday":  time.Thursday,
		"friday":    time.Friday,
		"saturday":  time.Saturday,
	}

	for input, expected := range days {
		t.Run(input, func(t *testing.T) {
			setupEnv(t, map[string]string{
				"TG_EVO_BOT_RANDOM_COFFEE_POLL_DAY":  input,
				"TG_EVO_BOT_RANDOM_COFFEE_PAIRS_DAY": input,
			})

			cfg, err := LoadConfig()

			require.NoError(t, err)
			assert.Equal(t, expected, cfg.RandomCoffeePollDay)
			assert.Equal(t, expected, cfg.RandomCoffeePairsDay)
		})
	}
}

func TestLoadConfig_Errors(t *testing.T) {
	tests := []struct {
		name        string
		overrides   map[string]string
		expectedErr string
	}{
		{
			name:        "missing bot token",
			overrides:   map[string]string{"TG_EVO_BOT_TOKEN": ""},
			expectedErr: "TG_EVO_BOT_TOKEN environment variable is not set",
		},
		{
			name:        "missing supergroup chat id",
			overrides:   map[string]string{"TG_EVO_BOT_SUPERGROUP_CHAT_ID": ""},
			expectedErr: "TG_EVO_BOT_SUPERGROUP_CHAT_ID environment variable is not set",
		},
		{
			name:        "invalid supergroup chat id",
			overrides:   map[string]string{"TG_EVO_BOT_SUPERGROUP_CHAT_ID": "abc"},
			expectedErr: "invalid supergroup chat ID: abc",
		},
		{
			name:        "missing openai api key",
			overrides:   map[string]string{"TG_EVO_BOT_OPENAI_API_KEY": ""},
			expectedErr: "TG_EVO_BOT_OPENAI_API_KEY environment variable is not set",
		},
		{
			name:        "invalid closed topic id",
			overrides:   map[string]string{"TG_EVO_BOT_CLOSED_TOPICS_IDS": "1,x,3"},
			expectedErr: "invalid topic ID in TG_EVO_BOT_CLOSED_TOPICS_IDS: x",
		},
		{
			name:        "invalid admin user id",
			overrides:   map[string]string{"TG_EVO_BOT_ADMIN_USER_ID": "admin"},
			expectedErr: "invalid admin user ID: admin",
		},
		{
			name:        "invalid forwarding topic id",
			overrides:   map[string]string{"TG_EVO_BOT_FORWARDING_TOPIC_ID": "f"},
			expectedErr: "invalid forwarding topic ID: f",
		},
		{
			name:        "invalid tool topic id",
			overrides:   map[string]string{"TG_EVO_BOT_TOOL_TOPIC_ID": "t"},
			expectedErr: "invalid tool topic ID: t",
		},
		{
			name:        "invalid content topic id",
			overrides:   map[string]string{"TG_EVO_BOT_CONTENT_TOPIC_ID": "c"},
			expectedErr: "invalid content topic ID: c",
		},
		{
			name:        "invalid announcement topic id",
			overrides:   map[string]string{"TG_EVO_BOT_ANNOUNCEMENT_TOPIC_ID": "a"},
			expectedErr: "invalid announcement topic ID: a",
		},
		{
			name:        "invalid intro topic id",
			overrides:   map[string]string{"TG_EVO_BOT_INTRO_TOPIC_ID": "i"},
			expectedErr: "invalid intro topic ID: i",
		},
		{
			name:        "missing db connection",
			overrides:   map[string]string{"TG_EVO_BOT_DB_CONNECTION": ""},
			expectedErr: "TG_EVO_BOT_DB_CONNECTION environment variable is not set",
		},
		{
			name:        "missing monitored topics",
			overrides:   map[string]string{"TG_EVO_BOT_MONITORED_TOPICS_IDS": ""},
			expectedErr: "TG_EVO_BOT_MONITORED_TOPICS_IDS environment variable is not set",
		},
		{
			name:        "invalid monitored topic id",
			overrides:   map[string]string{"TG_EVO_BOT_MONITORED_TOPICS_IDS": "1,two"},
			expectedErr: "invalid topic ID in TG_EVO_BOT_MONITORED_TOPICS_IDS: two",
		},
		{
			name:        "missing summary topic id",
			overrides:   map[string]string{"TG_EVO_BOT_SUMMARY_TOPIC_ID": ""},
			expectedErr: "TG_EVO_BOT_SUMMARY_TOPIC_ID environment variable is not set",
		},
		{
			name:        "invalid summary topic id",
			overrides:   map[string]string{"TG_EVO_BOT_SUMMARY_TOPIC_ID": "s"},
			expectedErr: "invalid summary topic ID: s",
		},
		{
			name:        "invalid summary time",
			overrides:   map[string]string{"TG_EVO_BOT_SUMMARY_TIME": "25:00"},
			expectedErr: "invalid summary time format: 25:00",
		},
		{
			name:        "invalid summarization task enabled",
			overrides:   map[string]string{"TG_EVO_BOT_SUMMARIZATION_TASK_ENABLED": "yes"},
			expectedErr: "invalid summarization task enabled value: yes",
		},
		{
			name:        "missing random coffee topic id",
			overrides:   map[string]string{"TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID": ""},
			expectedErr: "TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID environment variable is not set",
		},
		{
			name:        "invalid random coffee topic id",
			overrides:   map[string]string{"TG_EVO_BOT_RANDOM_COFFEE_TOPIC_ID": "rc"},
			expectedErr: "invalid random coffee topic ID: rc",
		},
		{
			name:        "non-numeric random coffee interval weeks",
			overrides:   map[string]string{"TG_EVO_BOT_RANDOM_COFFEE_INTERVAL_WEEKS": "two"},
			expectedErr: "invalid random coffee interval weeks: two (must be a positive integer)",
		},
		{
			name:        "zero random coffee interval weeks",
			overrides:   map[string]string{"TG_EVO_BOT_RANDOM_COFFEE_INTERVAL_WEEKS": "0"},
			expectedErr: "invalid random coffee interval weeks: 0 (must be a positive integer)",
		},
		{
			name:        "negative random coffee interval weeks",
			overrides:   map[string]string{"TG_EVO_BOT_RANDOM_COFFEE_INTERVAL_WEEKS": "-2"},
			expectedErr: "invalid random coffee interval weeks: -2 (must be a positive integer)",
		},
		{
			name:        "invalid random coffee poll task enabled",
			overrides:   map[string]string{"TG_EVO_BOT_RANDOM_COFFEE_POLL_TASK_ENABLED": "on"},
			expectedErr: "invalid random coffee poll task enabled value: on",
		},
		{
			name:        "invalid random coffee poll time",
			overrides:   map[string]string{"TG_EVO_BOT_RANDOM_COFFEE_POLL_TIME": "2pm"},
			expectedErr: "invalid random coffee poll time format: 2pm",
		},
		{
			name:        "invalid random coffee poll day",
			overrides:   map[string]string{"TG_EVO_BOT_RANDOM_COFFEE_POLL_DAY": "funday"},
			expectedErr: "invalid random coffee poll day: funday",
		},
		{
			name:        "invalid random coffee pairs task enabled",
			overrides:   map[string]string{"TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TASK_ENABLED": "nope"},
			expectedErr: "invalid random coffee pairs task enabled value: nope",
		},
		{
			name:        "invalid random coffee pairs time",
			overrides:   map[string]string{"TG_EVO_BOT_RANDOM_COFFEE_PAIRS_TIME": "12:60"},
			expectedErr: "invalid random coffee pairs time format: 12:60",
		},
		{
			name:        "invalid random coffee pairs day",
			overrides:   map[string]string{"TG_EVO_BOT_RANDOM_COFFEE_PAIRS_DAY": "mon"},
			expectedErr: "invalid random coffee pairs day: mon",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupEnv(t, tt.overrides)

			cfg, err := LoadConfig()

			require.Error(t, err)
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}
