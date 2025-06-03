package tasks

import (
	"context"
	"log"
	"time"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/services"
)

// RandomCoffeePollTask handles scheduling of random coffee polls
type RandomCoffeePollTask struct {
	config              *config.Config
	randomCoffeeService *services.RandomCoffeeService
	stop                chan struct{}
}

// NewRandomCoffeePollTask creates a new random coffee poll task
func NewRandomCoffeePollTask(config *config.Config, randomCoffeeService *services.RandomCoffeeService) *RandomCoffeePollTask {
	return &RandomCoffeePollTask{
		config:              config,
		randomCoffeeService: randomCoffeeService,
		stop:                make(chan struct{}),
	}
}

// Start starts the random coffee poll task
func (t *RandomCoffeePollTask) Start() {
	if !t.config.RandomCoffeePollTaskEnabled {
		log.Println("Random Coffee Poll Task: Random coffee poll task is disabled")
		return
	}
	log.Printf("Random Coffee Poll Task: Starting random coffee poll task with time %02d:%02d UTC on %s",
		t.config.RandomCoffeePollTime.Hour(),
		t.config.RandomCoffeePollTime.Minute(),
		t.config.RandomCoffeePollDay.String())
	go t.run()
}

// Stop stops the random coffee poll task
func (t *RandomCoffeePollTask) Stop() {
	log.Println("Random Coffee Poll Task: Stopping random coffee poll task")
	close(t.stop)
}

// run runs the random coffee poll task
func (t *RandomCoffeePollTask) run() {
	nextRun := t.calculateNextRun()
	log.Printf("Random Coffee Poll Task: Next random coffee poll scheduled for: %v", nextRun)

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-t.stop:
			return
		case now := <-ticker.C:
			if now.After(nextRun) {
				log.Println("Random Coffee Poll Task: Running scheduled random coffee poll")

				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
					defer cancel()

					if err := t.randomCoffeeService.SendPoll(ctx); err != nil {
						log.Printf("Random Coffee Poll Task: Error sending random coffee poll: %v", err)
					}
				}()

				nextRun = t.calculateNextRun()
				log.Printf("Random Coffee Poll Task: Next random coffee poll scheduled for: %v", nextRun)
			}
		}
	}
}

// calculateNextRun calculates the next run time
func (t *RandomCoffeePollTask) calculateNextRun() time.Time {
	now := time.Now().UTC()
	targetHour := t.config.RandomCoffeePollTime.Hour()
	targetMinute := t.config.RandomCoffeePollTime.Minute()
	targetWeekday := t.config.RandomCoffeePollDay

	// Calculate days until target weekday
	daysUntilTarget := (int(targetWeekday) - int(now.Weekday()) + 7) % 7

	// Create target time for today
	targetTime := time.Date(now.Year(), now.Month(), now.Day(), targetHour, targetMinute, 0, 0, time.UTC)

	if daysUntilTarget == 0 && now.Before(targetTime) {
		// Today is target day and time hasn't passed yet
		return targetTime
	}

	// Either not target day or time has passed - schedule for next occurrence
	if daysUntilTarget == 0 {
		daysUntilTarget = 7 // Next week
	}

	return targetTime.AddDate(0, 0, daysUntilTarget)
}
