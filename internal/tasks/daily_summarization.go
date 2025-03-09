package tasks

import (
	"context"
	"log"
	"time"

	"github.com/it-beard/evo-bot-go/internal/config"
	"github.com/it-beard/evo-bot-go/internal/services"
)

// DailySummarizationTask handles scheduling of daily summarization tasks
type DailySummarizationTask struct {
	config               *config.Config
	summarizationService *services.SummarizationService
	stop                 chan struct{}
}

// NewDailySummarizationTask creates a new daily summarization task
func NewDailySummarizationTask(config *config.Config, summarizationService *services.SummarizationService) *DailySummarizationTask {
	return &DailySummarizationTask{
		config:               config,
		summarizationService: summarizationService,
		stop:                 make(chan struct{}),
	}
}

// Start starts the daily summarization task
func (s *DailySummarizationTask) Start() {
	log.Println("Starting daily summarization task")
	go s.run()
}

// Stop stops the daily summarization task
func (s *DailySummarizationTask) Stop() {
	log.Println("Stopping daily summarization task")
	close(s.stop)
}

// run runs the daily summarization task
func (s *DailySummarizationTask) run() {
	// Calculate time until next run
	nextRun := s.calculateNextRun()
	log.Printf("Next summarization scheduled for: %v", nextRun)

	ticker := time.NewTicker(time.Minute) // Check every minute
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case now := <-ticker.C:
			// Check if it's time to run
			if now.After(nextRun) {
				log.Println("Running scheduled summarization")

				// Run summarization in a separate goroutine
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
					defer cancel()

					// For scheduled tasks, always send to the chat (not to DM)
					if err := s.summarizationService.RunDailySummarization(ctx, false); err != nil {
						log.Printf("Error running daily summarization: %v", err)
					}
				}()

				// Calculate next run time
				nextRun = s.calculateNextRun()
				log.Printf("Next summarization scheduled for: %v", nextRun)
			}
		}
	}
}

// calculateNextRun calculates the next run time
func (s *DailySummarizationTask) calculateNextRun() time.Time {
	now := time.Now()

	// Get the configured hour and minute
	targetHour := s.config.SummaryTime.Hour()
	targetMinute := s.config.SummaryTime.Minute()

	// Create a time for today at the target hour and minute
	targetToday := time.Date(now.Year(), now.Month(), now.Day(), targetHour, targetMinute, 0, 0, now.Location())

	// If the target time has already passed today, schedule for tomorrow
	if now.After(targetToday) {
		targetToday = targetToday.Add(24 * time.Hour)
	}

	return targetToday
}
