package tasks

import (
	"fmt"
	"log"
	"time"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/models"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/robfig/cron/v3"
)

// WeeklyMeetingPollTask handles scheduling of weekly meeting polls
type WeeklyMeetingPollTask struct {
	config   *config.Config
	bot      *gotgbot.Bot
	pollRepo *repositories.WeeklyMeetingPollRepository
	cron     *cron.Cron
	cronID   cron.EntryID
	location *time.Location // For time zone specific calculations
}

// NewWeeklyMeetingPollTask creates a new WeeklyMeetingPollTask
func NewWeeklyMeetingPollTask(cfg *config.Config, b *gotgbot.Bot, pr *repositories.WeeklyMeetingPollRepository) *WeeklyMeetingPollTask {
	return &WeeklyMeetingPollTask{
		config:   cfg,
		bot:      b,
		pollRepo: pr,
	}
}

// Name returns the name of the task
func (t *WeeklyMeetingPollTask) Name() string {
	return "Weekly Meeting Poll Task"
}

// Start sets up the cron schedule for the task
func (t *WeeklyMeetingPollTask) Start() {
	schedule := t.config.MeetingPollSchedule
	if schedule == "" {
		schedule = "CRON_TZ=Europe/Moscow 0 17 * * FRI" // Default schedule
		log.Printf("%s: Schedule not found in config, using default: %s", t.Name(), schedule)
	}

	log.Printf("%s: Starting with schedule: %s", t.Name(), schedule)

	// Load Moscow location
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		log.Printf("%s: Error loading location 'Europe/Moscow': %v. Using UTC.", t.Name(), err)
		loc = time.UTC // Fallback to UTC
	}
	t.location = loc

	t.cron = cron.New(cron.WithLocation(t.location)) // Use location for cron
	t.cronID, err = t.cron.AddFunc(schedule, t.sendPoll)
	if err != nil {
		log.Printf("%s: Error scheduling task: %v", t.Name(), err)
		return
	}
	t.cron.Start()
	log.Printf("%s: Scheduled with ID %v in location %s", t.Name(), t.cronID, t.location.String())
}

// Stop stops the scheduled task
func (t *WeeklyMeetingPollTask) Stop() {
	if t.cron != nil {
		log.Printf("%s: Stopping", t.Name())
		t.cron.Remove(t.cronID)
		t.cron.Stop() // Stop the cron scheduler
	}
}

// sendPoll is the function called by the cron scheduler
func (t *WeeklyMeetingPollTask) sendPoll() {
	chatID := t.config.SupergroupChatID
	if chatID == 0 {
		log.Printf("%s: SupergroupChatID is not configured. Skipping poll.", t.Name())
		return
	}

	question := "Как это работает? 📝 В конце каждой недели я буду спрашивать здесь, готов ли ты участвовать во встречах на следующей. Are you ready to participate in random coffee meetings next week?"
	options := []gotgbot.InputPollOption{
		{Text: "Yes, I'll participate"},
		{Text: "No, not this week"},
	}
	opts := &gotgbot.SendPollOpts{
		IsAnonymous:         false,
		AllowsMultipleAnswers: false,
	}

	log.Printf("%s: Sending poll to chat ID %d", t.Name(), chatID)
	sentPollMsg, err := t.bot.SendPoll(chatID, question, options, opts)
	if err != nil {
		log.Printf("%s: Error sending poll: %v", t.Name(), err)
		return
	}
	log.Printf("%s: Poll sent successfully. MessageID: %d, ChatID: %d", t.Name(), sentPollMsg.MessageId, sentPollMsg.Chat.Id)

	// Calculate upcoming Monday in the task's location (e.g., Europe/Moscow)
	nowInLoc := time.Now().In(t.location)
	daysUntilMonday := (8 - int(nowInLoc.Weekday())) % 7
	if daysUntilMonday == 0 { // If today is Monday (e.g., cron ran late or was misconfigured)
		daysUntilMonday = 7 // schedule for next Monday
	}
	weekStartDate := nowInLoc.AddDate(0, 0, daysUntilMonday)
	// Normalize to the beginning of the day in the task's location
	weekStartDate = time.Date(weekStartDate.Year(), weekStartDate.Month(), weekStartDate.Day(), 0, 0, 0, 0, t.location)
	// Convert to UTC for storage, if preferred, though TIMESTAMPTZ handles this.
	// weekStartDate = weekStartDate.UTC() 

	log.Printf("%s: Calculated WeekStartDate: %s (Location: %s)",
		t.Name(), weekStartDate.Format("2006-01-02"), t.location.String())

	if t.pollRepo == nil {
		log.Printf("%s: pollRepo is nil, skipping DB interaction.", t.Name())
		return
	}

	newPollEntry := models.WeeklyMeetingPoll{
		MessageID:      sentPollMsg.MessageId,
		ChatID:         sentPollMsg.Chat.Id,
		TelegramPollID: sentPollMsg.Poll.Id, // Add this line
		WeekStartDate:  weekStartDate,
		// CreatedAt will be set by the repository method or DB default
	}

	pollID, err := t.pollRepo.CreatePoll(newPollEntry)
	if err != nil {
		log.Printf("%s: Failed to save weekly meeting poll to DB: %v. Poll Message ID: %d", t.Name(), err, sentPollMsg.MessageId)
		// Consider how to handle this failure, e.g., retry or alert admin
		return
	}
	log.Printf("%s: Weekly meeting poll saved to DB with ID: %d, Original MessageID: %d, WeekStartDate: %s",
		t.Name(), pollID, sentPollMsg.MessageId, weekStartDate.Format("2006-01-02"))
}
