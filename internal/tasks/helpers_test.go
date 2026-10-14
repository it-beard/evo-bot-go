package tasks

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Compile-time checks that all tasks implement Task
var (
	_ Task = (*DailySummarizationTask)(nil)
	_ Task = (*RandomCoffeePollTask)(nil)
	_ Task = (*RandomCoffeePairsTask)(nil)
)

// clockTime builds a config time-of-day the same way config.LoadConfig does ("15:04" layout)
func clockTime(t *testing.T, at time.Time) time.Time {
	t.Helper()
	parsed, err := time.Parse("15:04", at.Format("15:04"))
	require.NoError(t, err)
	return parsed
}

// runningGoroutines counts goroutines currently executing the given function frame,
// e.g. "(*DailySummarizationTask).run("
func runningGoroutines(frame string) int {
	buf := make([]byte, 1<<20)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return strings.Count(string(buf[:n]), frame)
		}
		buf = make([]byte, 2*len(buf))
	}
}

// waitFor polls cond until it returns true or the timeout expires
func waitFor(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return cond()
}

func isClosed(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

// assertStartStop starts an enabled task, checks its scheduler goroutine is running and exits after Stop
func assertStartStop(t *testing.T, task Task, stop chan struct{}, frame string) {
	t.Helper()
	require.Zero(t, runningGoroutines(frame), "no scheduler goroutine before Start")

	task.Start()
	require.True(t, waitFor(2*time.Second, func() bool { return runningGoroutines(frame) == 1 }),
		"Start must launch exactly one scheduler goroutine")

	task.Stop()
	require.True(t, isClosed(stop))
	require.True(t, waitFor(2*time.Second, func() bool { return runningGoroutines(frame) == 0 }),
		"the scheduler goroutine must exit after Stop")
}

// assertDisabledStart checks that Start on a disabled task does not launch a scheduler and Stop is still safe
func assertDisabledStart(t *testing.T, task Task, stop chan struct{}, frame string) {
	t.Helper()

	task.Start()
	// Start returns synchronously when disabled, so no goroutine can appear later
	require.Zero(t, runningGoroutines(frame))

	require.NotPanics(t, task.Stop)
	require.True(t, isClosed(stop))
}

type nextRunCase struct {
	name   string
	offset time.Duration
}

// weeklyCases are offsets of the configured weekday+time from now, within one week either way
var weeklyCases = []nextRunCase{
	{"later today", 2 * time.Minute},
	{"in a few hours", 3 * time.Hour},
	{"in three days", 3 * 24 * time.Hour},
	{"almost a week ahead", 6*24*time.Hour + 23*time.Hour},
	{"just passed", -2 * time.Minute},
	{"a few hours ago", -3 * time.Hour},
	{"three days ago", -3 * 24 * time.Hour},
	{"almost a week ago", -(6*24*time.Hour + 23*time.Hour)},
}

// assertWeeklyNextRun checks a weekly scheduler: configure weekday+time at now+offset and expect the next occurrence
func assertWeeklyNextRun(t *testing.T, configure func(day time.Weekday, at time.Time), calculate func() time.Time) {
	t.Helper()
	for _, tc := range weeklyCases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now().UTC()
			target := now.Add(tc.offset).Truncate(time.Minute)
			want := target
			if tc.offset < 0 {
				want = target.AddDate(0, 0, 7)
			}
			configure(target.Weekday(), clockTime(t, target))

			got := calculate()

			require.Equal(t, want, got)
			require.Equal(t, time.UTC, got.Location())
			require.True(t, got.After(now), "next run must be in the future")
			require.LessOrEqual(t, got.Sub(now), 7*24*time.Hour)
		})
	}
}
