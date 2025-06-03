package services

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

type RandomCoffeeService struct {
	config          *config.Config
	pollSender      *PollSenderService
	messageSender   *MessageSenderService
	pollRepo        *repositories.RandomCoffeePollRepository
	participantRepo *repositories.RandomCoffeeParticipantRepository
	profileRepo     *repositories.ProfileRepository
}

// NewRandomCoffeeService creates a new random coffee poll service
func NewRandomCoffeeService(
	config *config.Config,
	pollSender *PollSenderService,
	messageSender *MessageSenderService,
	pollRepo *repositories.RandomCoffeePollRepository,
	participantRepo *repositories.RandomCoffeeParticipantRepository,
	profileRepo *repositories.ProfileRepository,
) *RandomCoffeeService {
	return &RandomCoffeeService{
		config:          config,
		pollSender:      pollSender,
		messageSender:   messageSender,
		pollRepo:        pollRepo,
		participantRepo: participantRepo,
		profileRepo:     profileRepo,
	}
}

func (s *RandomCoffeeService) SendPoll(ctx context.Context) error {
	chatID := utils.ChatIdToFullChatId(s.config.SuperGroupChatID)
	if chatID == 0 {
		log.Println("Random Coffee Poll Service: SuperGroupChatID is not configured. Skipping poll.")
		return nil
	}

	if s.config.RandomCoffeeTopicID == 0 {
		return fmt.Errorf("Random Coffee Poll Service: RandomCoffeeTopicID is not configured")
	}

	// Send the poll
	question := "📝 Готов/а ли ты участвовать в рандомных кофе-встречах на следующей неделе?" +
		"\n\nКак это работает: в конце каждой недели я буду спрашивать здесь, хочешь ли ты участвовать во встречах. " +
		"Если ответишь «да», то в понедельник тебя могут объединить в пару с другим участником/цей для неформального общения!"
	answers := []gotgbot.InputPollOption{
		{Text: "Да, участвую! ☕️"},
		{Text: "Нет, пропускаю эту неделю"},
	}
	options := &gotgbot.SendPollOpts{
		IsAnonymous:           false,
		AllowsMultipleAnswers: false,
		MessageThreadId:       int64(s.config.RandomCoffeeTopicID),
	}
	sentPollMsg, err := s.pollSender.SendPoll(chatID, question, answers, options)
	if err != nil {
		return err
	}

	// Save to database
	return s.savePollToDB(sentPollMsg)
}

// savePollToDB saves the poll information to the database
func (s *RandomCoffeeService) savePollToDB(sentPollMsg *gotgbot.Message) error {
	if s.pollRepo == nil {
		log.Println("Random Coffee Poll Service: pollRepo is nil, skipping DB interaction.")
		return nil
	}

	// Calculate next Monday (week start date)
	now := time.Now().UTC()
	daysUntilMonday := (8 - int(now.Weekday())) % 7
	if daysUntilMonday == 0 {
		daysUntilMonday = 7 // Next Monday if today is Monday
	}

	weekStartDate := now.AddDate(0, 0, daysUntilMonday)
	weekStartDate =
		time.Date(
			weekStartDate.Year(),
			weekStartDate.Month(),
			weekStartDate.Day(),
			0, 0, 0, 0, time.UTC,
		)

	log.Printf(
		"Random Coffee Poll Service: Calculated WeekStartDate: %s (UTC)",
		weekStartDate.Format("2006-01-02"),
	)

	newPollEntry := repositories.RandomCoffeePoll{
		MessageID:      sentPollMsg.MessageId,
		TelegramPollID: sentPollMsg.Poll.Id,
		WeekStartDate:  weekStartDate,
	}

	pollID, err := s.pollRepo.CreatePoll(newPollEntry)
	if err != nil {
		log.Printf("Random Coffee Poll Service: Failed to save random coffee poll to DB: %v. Poll Message ID: %d", err, sentPollMsg.MessageId)
		return err
	}

	log.Printf("Random Coffee Poll Service: Random coffee poll saved to DB with ID: %d, Original MessageID: %d, WeekStartDate: %s",
		pollID, sentPollMsg.MessageId, weekStartDate.Format("2006-01-02"))

	return nil
}

func (s *RandomCoffeeService) GenerateAndSendPairs() error {
	latestPoll, err := s.pollRepo.GetLatestPoll()
	if err != nil {
		return fmt.Errorf("error getting latest poll: %w", err)
	}
	if latestPoll == nil {
		return fmt.Errorf("опрос для рандом кофе не найден")
	}

	// Stop the poll first before generating pairs
	chatID := utils.ChatIdToFullChatId(s.config.SuperGroupChatID)
	_, err = s.pollSender.StopPoll(chatID, latestPoll.MessageID, nil)
	if err != nil {
		log.Printf("RandomCoffeeService: Warning - failed to stop poll (message ID %d): %v", latestPoll.MessageID, err)
		// Continue anyway - we might still be able to generate pairs
	} else {
		log.Printf("RandomCoffeeService: Successfully stopped poll (message ID %d)", latestPoll.MessageID)
	}

	participants, err := s.participantRepo.GetParticipatingUsers(latestPoll.ID)
	if err != nil {
		return fmt.Errorf("error getting participants for poll ID %d: %w", latestPoll.ID, err)
	}

	if len(participants) < 2 {
		return fmt.Errorf("недостаточно участников для создания пар (нужно минимум 2, зарегистрировалось %d)", len(participants))
	}

	// Random Pairing Logic
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(participants), func(i, j int) {
		participants[i], participants[j] = participants[j], participants[i]
	})

	var pairsText []string
	var unpairedUserText string

	for i := 0; i < len(participants); i += 2 {
		user1 := participants[i]
		user1Display := s.formatUserDisplay(&user1)

		if i+1 < len(participants) {
			user2 := participants[i+1]
			user2Display := s.formatUserDisplay(&user2)
			pairsText = append(pairsText, fmt.Sprintf("%s x %s", user1Display, user2Display))
		} else {
			unpairedUserText = user1Display
		}
	}

	var messageBuilder strings.Builder
	messageBuilder.WriteString(fmt.Sprintf("☕️ Пары для рандом кофе (неделя %s):\n\n", latestPoll.WeekStartDate.Format("Mon, Jan 2")))
	for _, pair := range pairsText {
		messageBuilder.WriteString(fmt.Sprintf("➪ %s\n", pair))
	}
	if unpairedUserText != "" {
		messageBuilder.WriteString(fmt.Sprintf("\n😔 %s ищет кофе-компаньона на эту неделю!\n", unpairedUserText))
	}
	messageBuilder.WriteString("\n🗓 День, время и формат встречи вы выбираете сами. Просто напиши партнеру в личку, когда и в каком формате тебе удобно встретиться.")

	// Send the pairing message
	opts := &gotgbot.SendMessageOpts{
		MessageThreadId: int64(s.config.RandomCoffeeTopicID),
	}

	err = s.messageSender.SendHtml(chatID, messageBuilder.String(), opts)
	if err != nil {
		return fmt.Errorf("error sending pairing message to chat %d: %w", chatID, err)
	}

	log.Printf("RandomCoffeeService: Successfully sent pairings for poll ID %d to chat %d.", latestPoll.ID, s.config.SuperGroupChatID)
	return nil
}

func (s *RandomCoffeeService) formatUserDisplay(user *repositories.User) string {
	profile, err := s.profileRepo.GetOrCreate(user.ID)
	if err != nil {
		log.Printf("RandomCoffeeService: Error getting profile for user %d: %v", user.ID, err)
		if user.TgUsername != "" {
			return fmt.Sprintf("@%s", user.TgUsername)
		}
		return user.Firstname
	}

	hasPublishedProfile := profile.PublishedMessageID.Valid && profile.PublishedMessageID.Int64 > 0
	if hasPublishedProfile {
		fullName := user.Firstname
		if user.Lastname != "" {
			fullName += " " + user.Lastname
		}

		profileLink := utils.GetIntroMessageLink(s.config, profile.PublishedMessageID.Int64)
		linkedName := fmt.Sprintf("<a href=\"%s\">%s</a>", profileLink, fullName)

		if user.TgUsername != "" {
			return fmt.Sprintf("%s (@%s)", linkedName, user.TgUsername)
		}
		return linkedName
	} else {
		if user.TgUsername != "" {
			return fmt.Sprintf("@%s", user.TgUsername)
		}
		return user.Firstname
	}
}
