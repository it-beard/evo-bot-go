package tasks

import (
	"log"
	"time"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"
)

// RandomCoffeePairsTask handles scheduling of random coffee pairs generation
type RandomCoffeePairsTask struct {
	config              *config.Config
	randomCoffeeService *services.RandomCoffeeService
	stop                chan struct{}
}

// NewRandomCoffeePairsTask creates a new random coffee pairs task
func NewRandomCoffeePairsTask(config *config.Config, randomCoffeeService *services.RandomCoffeeService) *RandomCoffeePairsTask {
	return &RandomCoffeePairsTask{
		config:              config,
		randomCoffeeService: randomCoffeeService,
		stop:                make(chan struct{}),
	}
}

// Start starts the random coffee pairs task
func (t *RandomCoffeePairsTask) Start() {
	if !t.config.RandomCoffeePairsTaskEnabled {
		log.Printf("%s: Random coffee pairs task is disabled", utils.GetCurrentTypeName())
		return
	}
	log.Printf("%s: Starting random coffee pairs task with time %02d:%02d UTC on %s",
		utils.GetCurrentTypeName(),
		t.config.RandomCoffeePairsTime.Hour(),
		t.config.RandomCoffeePairsTime.Minute(),
		t.config.RandomCoffeePairsDay.String())
	go t.run()
}

// Stop stops the random coffee pairs task
func (t *RandomCoffeePairsTask) Stop() {
	log.Printf("%s: Stopping random coffee pairs task", utils.GetCurrentTypeName())
	close(t.stop)
}

// run runs the random coffee pairs task
func (t *RandomCoffeePairsTask) run() {
	nextRun := t.calculateNextRun()
	log.Printf("%s: Next random coffee pairs generation scheduled for: %v", utils.GetCurrentTypeName(), nextRun)

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-t.stop:
			return
		case now := <-ticker.C:
			if now.After(nextRun) {
				log.Printf("%s: Running scheduled random coffee pairs generation", utils.GetCurrentTypeName())

				go func() {
					if err := t.randomCoffeeService.GenerateAndSendPairs(); err != nil {
						log.Printf("%s: Error generating random coffee pairs: %v", utils.GetCurrentTypeName(), err)
					}
				}()

				nextRun = t.calculateNextRun()
				log.Printf("%s: Next random coffee pairs generation scheduled for: %v", utils.GetCurrentTypeName(), nextRun)
			}
		}
	}
}

// calculateNextRun calculates the next run time
func (t *RandomCoffeePairsTask) calculateNextRun() time.Time {
	now := time.Now().UTC()
	targetHour := t.config.RandomCoffeePairsTime.Hour()
	targetMinute := t.config.RandomCoffeePairsTime.Minute()
	targetWeekday := t.config.RandomCoffeePairsDay

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
