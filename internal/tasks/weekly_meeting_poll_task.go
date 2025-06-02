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
	log.Printf("Weekly Meeting Poll Task: Starting weekly meeting poll task with time %02d:%02d UTC on %s",
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
	nextRun := t.calculateNextRun()
	log.Printf("Weekly Meeting Poll Task: Next meeting poll scheduled for: %v", nextRun)

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-t.stop:
			return
		case now := <-ticker.C:
			if now.After(nextRun) {
				log.Println("Weekly Meeting Poll Task: Running scheduled weekly meeting poll")

				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
					defer cancel()

					if err := t.sendWeeklyMeetingPoll(ctx); err != nil {
						log.Printf("Weekly Meeting Poll Task: Error sending weekly meeting poll: %v", err)
					}
				}()

				nextRun = t.calculateNextRun()
				log.Printf("Weekly Meeting Poll Task: Next meeting poll scheduled for: %v", nextRun)
			}
		}
	}
}

// calculateNextRun calculates the next run time
func (t *WeeklyMeetingPollTask) calculateNextRun() time.Time {
	now := time.Now().UTC()
	targetHour := t.config.MeetingPollTime.Hour()
	targetMinute := t.config.MeetingPollTime.Minute()
	targetWeekday := t.config.MeetingPollDay

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

// sendWeeklyMeetingPoll sends the weekly meeting poll
func (t *WeeklyMeetingPollTask) sendWeeklyMeetingPoll(ctx context.Context) error {
	chatID := t.config.SuperGroupChatID
	if chatID == 0 {
		log.Println("Weekly Meeting Poll Task: SuperGroupChatID is not configured. Skipping poll.")
		return nil
	}

	// Send the poll
	sentPollMsg, err := t.sendPoll(chatID)
	if err != nil {
		return err
	}

	// Save to database
	return t.savePollToDB(sentPollMsg)
}

// sendPoll sends the actual poll message
func (t *WeeklyMeetingPollTask) sendPoll(chatID int64) (*gotgbot.Message, error) {
	question := "📝 Готов ли ты участвовать в рандомных кофе-встречах на следующей неделе?\n\nКак это работает: в конце каждой недели я буду спрашивать здесь, хочешь ли ты участвовать во встречах. Если ответишь «да», то в понедельник тебя могут объединить в пару с другим участником для неформального общения!"
	options := []gotgbot.InputPollOption{
		{Text: "Да, участвую! ☕️"},
		{Text: "Нет, пропускаю эту неделю"},
	}
	opts := &gotgbot.SendPollOpts{
		IsAnonymous:           false,
		AllowsMultipleAnswers: false,
	}

	log.Printf("Weekly Meeting Poll Task: Sending poll to chat ID %d", chatID)
	sentPollMsg, err := t.bot.SendPoll(chatID, question, options, opts)
	if err != nil {
		return nil, err
	}

	log.Printf("Weekly Meeting Poll Task: Poll sent successfully. MessageID: %d, ChatID: %d", sentPollMsg.MessageId, sentPollMsg.Chat.Id)
	return sentPollMsg, nil
}

// savePollToDB saves the poll information to the database
func (t *WeeklyMeetingPollTask) savePollToDB(sentPollMsg *gotgbot.Message) error {
	if t.pollRepo == nil {
		log.Println("Weekly Meeting Poll Task: pollRepo is nil, skipping DB interaction.")
		return nil
	}

	// Calculate next Monday (week start date)
	now := time.Now().UTC()
	daysUntilMonday := (8 - int(now.Weekday())) % 7
	if daysUntilMonday == 0 {
		daysUntilMonday = 7 // Next Monday if today is Monday
	}

	weekStartDate := now.AddDate(0, 0, daysUntilMonday)
	weekStartDate = time.Date(weekStartDate.Year(), weekStartDate.Month(), weekStartDate.Day(), 0, 0, 0, 0, time.UTC)

	log.Printf("Weekly Meeting Poll Task: Calculated WeekStartDate: %s (UTC)", weekStartDate.Format("2006-01-02"))

	newPollEntry := models.WeeklyMeetingPoll{
		MessageID:      sentPollMsg.MessageId,
		ChatID:         sentPollMsg.Chat.Id,
		TelegramPollID: sentPollMsg.Poll.Id,
		WeekStartDate:  weekStartDate,
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
