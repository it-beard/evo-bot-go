package tasks

import (
	"context"
	"log"
	"time"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/models"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// WeeklyMeetingPollTask handles scheduling of weekly meeting polls
type WeeklyMeetingPollTask struct {
	config   *config.Config
	bot      *gotgbot.Bot
	pollRepo *repositories.WeeklyMeetingPollRepository
	stop     chan struct{}
}

// NewWeeklyMeetingPollTask creates a new weekly meeting poll task
func NewWeeklyMeetingPollTask(config *config.Config, bot *gotgbot.Bot, pollRepo *repositories.WeeklyMeetingPollRepository) *WeeklyMeetingPollTask {
	return &WeeklyMeetingPollTask{
		config:   config,
		bot:      bot,
		pollRepo: pollRepo,
		stop:     make(chan struct{}),
	}
}

// Start starts the weekly meeting poll task
func (t *WeeklyMeetingPollTask) Start() {
	if !t.config.MeetingPollTaskEnabled {
		log.Println("Weekly Meeting Poll Task: Weekly meeting poll task is disabled")
		return
	}
	log.Printf("Weekly Meeting Poll Task: Starting weekly meeting poll task with time %02d:%02d on %s",
		t.config.MeetingPollTime.Hour(),
		t.config.MeetingPollTime.Minute(),
		t.config.MeetingPollDay.String())
	go t.run()
}

// Stop stops the weekly meeting poll task
func (t *WeeklyMeetingPollTask) Stop() {
	log.Println("Weekly Meeting Poll Task: Stopping weekly meeting poll task")
	close(t.stop)
}

// run runs the weekly meeting poll task
func (t *WeeklyMeetingPollTask) run() {
	// Calculate time until next run
	nextRun := t.calculateNextRun()
	log.Printf("Weekly Meeting Poll Task: Next meeting poll scheduled for: %v", nextRun)

	ticker := time.NewTicker(time.Minute) // Check every minute
	defer ticker.Stop()

	for {
		select {
		case <-t.stop:
			return
		case now := <-ticker.C:
			// Check if it's time to run
			if now.After(nextRun) {
				log.Println("Weekly Meeting Poll Task: Running scheduled weekly meeting poll")

				// Run poll sending in a separate goroutine
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
					defer cancel()

					if err := t.sendWeeklyMeetingPoll(ctx); err != nil {
						log.Printf("Weekly Meeting Poll Task: Error sending weekly meeting poll: %v", err)
					}
				}()

				// Calculate next run time
				nextRun = t.calculateNextRun()
				log.Printf("Weekly Meeting Poll Task: Next meeting poll scheduled for: %v", nextRun)
			}
		}
	}
}

// calculateNextRun calculates the next run time
func (t *WeeklyMeetingPollTask) calculateNextRun() time.Time {
	// Load Moscow location for timezone calculations
	location, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		log.Printf("Weekly Meeting Poll Task: Error loading location 'Europe/Moscow': %v. Using UTC.", err)
		location = time.UTC // Fallback to UTC
	}

	now := time.Now().In(location)

	// Get the configured day and time
	targetWeekday := t.config.MeetingPollDay
	targetHour := t.config.MeetingPollTime.Hour()
	targetMinute := t.config.MeetingPollTime.Minute()

	// Calculate days until the target weekday
	daysUntilTarget := (int(targetWeekday) - int(now.Weekday()) + 7) % 7

	// Create target time for this week
	targetTime := time.Date(now.Year(), now.Month(), now.Day(), targetHour, targetMinute, 0, 0, location)

	if daysUntilTarget == 0 {
		// Today is the target day, check if the target time has already passed
		if now.Before(targetTime) {
			// Target time hasn't passed yet today
			return targetTime
		}
		// Target time has passed, schedule for next week
		daysUntilTarget = 7
	}

	// Add the days to get to the target day
	targetTime = targetTime.Add(time.Duration(daysUntilTarget) * 24 * time.Hour)

	return targetTime
}

// sendWeeklyMeetingPoll sends the weekly meeting poll
func (t *WeeklyMeetingPollTask) sendWeeklyMeetingPoll(ctx context.Context) error {
	chatID := t.config.SuperGroupChatID
	if chatID == 0 {
		log.Printf("Weekly Meeting Poll Task: SuperGroupChatID is not configured. Skipping poll.")
		return nil
	}

	question := "Как это работает? 📝 В конце каждой недели я буду спрашивать здесь, готов ли ты участвовать во встречах на следующей. Are you ready to participate in random coffee meetings next week?"
	options := []gotgbot.InputPollOption{
		{Text: "Yes, I'll participate"},
		{Text: "No, not this week"},
	}
	opts := &gotgbot.SendPollOpts{
		IsAnonymous:           false,
		AllowsMultipleAnswers: false,
	}

	log.Printf("Weekly Meeting Poll Task: Sending poll to chat ID %d", chatID)
	sentPollMsg, err := t.bot.SendPoll(chatID, question, options, opts)
	if err != nil {
		return err
	}
	log.Printf("Weekly Meeting Poll Task: Poll sent successfully. MessageID: %d, ChatID: %d", sentPollMsg.MessageId, sentPollMsg.Chat.Id)

	// Calculate upcoming Monday in Moscow time
	location, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		log.Printf("Weekly Meeting Poll Task: Error loading location 'Europe/Moscow': %v. Using UTC.", err)
		location = time.UTC // Fallback to UTC
	}

	nowInLoc := time.Now().In(location)
	daysUntilMonday := (8 - int(nowInLoc.Weekday())) % 7
	if daysUntilMonday == 0 { // If today is Monday
		daysUntilMonday = 7 // schedule for next Monday
	}
	weekStartDate := nowInLoc.AddDate(0, 0, daysUntilMonday)
	// Normalize to the beginning of the day in the specified location
	weekStartDate = time.Date(weekStartDate.Year(), weekStartDate.Month(), weekStartDate.Day(), 0, 0, 0, 0, location)

	log.Printf("Weekly Meeting Poll Task: Calculated WeekStartDate: %s (Location: %s)",
		weekStartDate.Format("2006-01-02"), location.String())

	if t.pollRepo == nil {
		log.Printf("Weekly Meeting Poll Task: pollRepo is nil, skipping DB interaction.")
		return nil
	}

	newPollEntry := models.WeeklyMeetingPoll{
		MessageID:      sentPollMsg.MessageId,
		ChatID:         sentPollMsg.Chat.Id,
		TelegramPollID: sentPollMsg.Poll.Id,
		WeekStartDate:  weekStartDate,
		// CreatedAt will be set by the repository method or DB default
	}

	pollID, err := t.pollRepo.CreatePoll(newPollEntry)
	if err != nil {
		log.Printf("Weekly Meeting Poll Task: Failed to save weekly meeting poll to DB: %v. Poll Message ID: %d", err, sentPollMsg.MessageId)
		return err
	}
	log.Printf("Weekly Meeting Poll Task: Weekly meeting poll saved to DB with ID: %d, Original MessageID: %d, WeekStartDate: %s",
		pollID, sentPollMsg.MessageId, weekStartDate.Format("2006-01-02"))

	return nil
}
