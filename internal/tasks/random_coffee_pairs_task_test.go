package tasks

import (
	"testing"
	"time"

	"evo-bot-go/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const pairsRunFrame = "(*RandomCoffeePairsTask).run("

func TestRandomCoffeePairsTask_calculateNextRun(t *testing.T) {
	cfg := &config.Config{}
	task := NewRandomCoffeePairsTask(cfg, nil)

	assertWeeklyNextRun(t,
		func(day time.Weekday, at time.Time) {
			cfg.RandomCoffeePairsDay = day
			cfg.RandomCoffeePairsTime = at
		},
		task.calculateNextRun,
	)
}

func TestRandomCoffeePairsTask_calculateNextRun_UsesConfiguredWeekdayAndTime(t *testing.T) {
	for day := time.Sunday; day <= time.Saturday; day++ {
		t.Run(day.String(), func(t *testing.T) {
			cfg := &config.Config{
				RandomCoffeePairsDay:  day,
				RandomCoffeePairsTime: time.Date(0, 1, 1, 12, 5, 0, 0, time.UTC),
			}
			now := time.Now().UTC()

			got := NewRandomCoffeePairsTask(cfg, nil).calculateNextRun()

			assert.Equal(t, day, got.Weekday())
			assert.Equal(t, 12, got.Hour())
			assert.Equal(t, 5, got.Minute())
			assert.Zero(t, got.Second())
			assert.True(t, got.After(now))
			// one extra minute tolerates the clock ticking past the configured time between now and the call
			assert.LessOrEqual(t, got.Sub(now), 7*24*time.Hour+time.Minute)
		})
	}
}

func TestRandomCoffeePairsTask_Start_Disabled(t *testing.T) {
	task := NewRandomCoffeePairsTask(&config.Config{RandomCoffeePairsTaskEnabled: false}, nil)

	assertDisabledStart(t, task, task.stop, pairsRunFrame)
}

func TestRandomCoffeePairsTask_StartStop_Enabled(t *testing.T) {
	cfg := &config.Config{
		RandomCoffeePairsTaskEnabled: true,
		RandomCoffeePairsDay:         time.Monday,
		RandomCoffeePairsTime:        time.Date(0, 1, 1, 12, 0, 0, 0, time.UTC),
	}
	task := NewRandomCoffeePairsTask(cfg, nil)

	assertStartStop(t, task, task.stop, pairsRunFrame)
}

func TestNewRandomCoffeePairsTask(t *testing.T) {
	cfg := &config.Config{}
	task := NewRandomCoffeePairsTask(cfg, nil)

	require.NotNil(t, task)
	assert.Same(t, cfg, task.config)
	assert.False(t, isClosed(task.stop))
}
