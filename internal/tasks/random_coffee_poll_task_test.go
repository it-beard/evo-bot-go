package tasks

import (
	"testing"
	"time"

	"evo-bot-go/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const pollRunFrame = "(*RandomCoffeePollTask).run("

func TestRandomCoffeePollTask_calculateNextRun(t *testing.T) {
	cfg := &config.Config{}
	task := NewRandomCoffeePollTask(cfg, nil)

	assertWeeklyNextRun(t,
		func(day time.Weekday, at time.Time) {
			cfg.RandomCoffeePollDay = day
			cfg.RandomCoffeePollTime = at
		},
		task.calculateNextRun,
	)
}

func TestRandomCoffeePollTask_calculateNextRun_UsesConfiguredWeekdayAndTime(t *testing.T) {
	for day := time.Sunday; day <= time.Saturday; day++ {
		t.Run(day.String(), func(t *testing.T) {
			cfg := &config.Config{
				RandomCoffeePollDay:  day,
				RandomCoffeePollTime: time.Date(0, 1, 1, 14, 30, 0, 0, time.UTC),
			}
			now := time.Now().UTC()

			got := NewRandomCoffeePollTask(cfg, nil).calculateNextRun()

			assert.Equal(t, day, got.Weekday())
			assert.Equal(t, 14, got.Hour())
			assert.Equal(t, 30, got.Minute())
			assert.Zero(t, got.Second())
			assert.True(t, got.After(now))
			// one extra minute tolerates the clock ticking past the configured time between now and the call
			assert.LessOrEqual(t, got.Sub(now), 7*24*time.Hour+time.Minute)
		})
	}
}

func TestRandomCoffeePollTask_Start_Disabled(t *testing.T) {
	task := NewRandomCoffeePollTask(&config.Config{RandomCoffeePollTaskEnabled: false}, nil)

	assertDisabledStart(t, task, task.stop, pollRunFrame)
}

func TestRandomCoffeePollTask_StartStop_Enabled(t *testing.T) {
	cfg := &config.Config{
		RandomCoffeePollTaskEnabled: true,
		RandomCoffeePollDay:         time.Friday,
		RandomCoffeePollTime:        time.Date(0, 1, 1, 14, 0, 0, 0, time.UTC),
	}
	task := NewRandomCoffeePollTask(cfg, nil)

	assertStartStop(t, task, task.stop, pollRunFrame)
}

func TestNewRandomCoffeePollTask(t *testing.T) {
	cfg := &config.Config{}
	task := NewRandomCoffeePollTask(cfg, nil)

	require.NotNil(t, task)
	assert.Same(t, cfg, task.config)
	assert.False(t, isClosed(task.stop))
}
