package tasks

import (
	"testing"
	"time"

	"evo-bot-go/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const dailyRunFrame = "(*DailySummarizationTask).run("

func TestDailySummarizationTask_calculateNextRun(t *testing.T) {
	tests := []nextRunCase{
		{"later today", 2 * time.Minute},
		{"in a few hours", 5 * time.Hour},
		{"almost a day ahead", 23 * time.Hour},
		{"just passed", -2 * time.Minute},
		{"a few hours ago", -5 * time.Hour},
		{"almost a day ago", -23 * time.Hour},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now().UTC()
			target := now.Add(tc.offset).Truncate(time.Minute)
			want := target
			if tc.offset < 0 {
				want = target.Add(24 * time.Hour)
			}
			cfg := &config.Config{SummaryTime: clockTime(t, target)}
			task := NewDailySummarizationTask(cfg, nil)

			got := task.calculateNextRun()

			assert.Equal(t, want, got)
			assert.Equal(t, time.UTC, got.Location())
			assert.True(t, got.After(now), "next run must be in the future")
			assert.LessOrEqual(t, got.Sub(now), 24*time.Hour)
			assert.Zero(t, got.Second())
		})
	}
}

func TestDailySummarizationTask_Start_Disabled(t *testing.T) {
	task := NewDailySummarizationTask(&config.Config{SummarizationTaskEnabled: false}, nil)

	assertDisabledStart(t, task, task.stop, dailyRunFrame)
}

func TestDailySummarizationTask_StartStop_Enabled(t *testing.T) {
	cfg := &config.Config{
		SummarizationTaskEnabled: true,
		SummaryTime:              time.Date(0, 1, 1, 3, 0, 0, 0, time.UTC),
	}
	task := NewDailySummarizationTask(cfg, nil)

	assertStartStop(t, task, task.stop, dailyRunFrame)
}

func TestNewDailySummarizationTask(t *testing.T) {
	cfg := &config.Config{}
	task := NewDailySummarizationTask(cfg, nil)

	require.NotNil(t, task)
	assert.Same(t, cfg, task.config)
	assert.NotNil(t, task.stop)
	assert.False(t, isClosed(task.stop))
}
