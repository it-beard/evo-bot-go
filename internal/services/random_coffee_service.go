package services

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/utils"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

type RandomCoffeeService struct {
	bot             *gotgbot.Bot
	config          *config.Config
	pollSender      *PollSenderService
	messageSender   *MessageSenderService
	pollRepo        *repositories.RandomCoffeePollRepository
	participantRepo *repositories.RandomCoffeeParticipantRepository
	profileRepo     *repositories.ProfileRepository
	pairRepo        *repositories.RandomCoffeePairRepository
	userRepo        *repositories.UserRepository
	pointsLogRepo   *repositories.UserPointsLogRepository
}

// NewRandomCoffeeService creates a new random coffee poll service
func NewRandomCoffeeService(
	bot *gotgbot.Bot,
	config *config.Config,
	pollSender *PollSenderService,
	messageSender *MessageSenderService,
	pollRepo *repositories.RandomCoffeePollRepository,
	participantRepo *repositories.RandomCoffeeParticipantRepository,
	profileRepo *repositories.ProfileRepository,
	pairRepo *repositories.RandomCoffeePairRepository,
	userRepo *repositories.UserRepository,
	pointsLogRepo *repositories.UserPointsLogRepository,
) *RandomCoffeeService {
	return &RandomCoffeeService{
		bot:             bot,
		config:          config,
		pollSender:      pollSender,
		messageSender:   messageSender,
		pollRepo:        pollRepo,
		participantRepo: participantRepo,
		profileRepo:     profileRepo,
		pairRepo:        pairRepo,
		userRepo:        userRepo,
		pointsLogRepo:   pointsLogRepo,
	}
}

func (s *RandomCoffeeService) SendPoll(ctx context.Context) error {
	chatID := utils.ChatIdToFullChatId(s.config.SuperGroupChatID)
	if chatID == 0 {
		log.Printf("%s: SuperGroupChatID is not configured. Skipping poll.", utils.GetCurrentTypeName())
		return nil
	}

	if s.config.RandomCoffeeTopicID == 0 {
		return fmt.Errorf("%s: RandomCoffeeTopicID is not configured", utils.GetCurrentTypeName())
	}

	// Send reqular message with link to rules and new random coffee poll
	message :=
		fmt.Sprintf("Привет! Открываю запись на новый <b>Random Coffee</b> <i>(<a href=\"https://t.me/c/%d/%d/%d\">правила участия</a>)</i>.",
			s.config.SuperGroupChatID,
			s.config.RandomCoffeeTopicID,
			s.config.RandomCoffeeTopicID+1, // next message id (small hack)
		) + " Голосуй в опросе ниже, если хочешь участвовать ⬇️"

	opts := &gotgbot.SendMessageOpts{
		MessageThreadId: int64(s.config.RandomCoffeeTopicID),
	}
	err := s.messageSender.SendHtml(chatID, message, opts)
	if err != nil {
		return fmt.Errorf("%s: Failed to send regular message: %v", utils.GetCurrentTypeName(), err)
	}

	// Send the poll
	question := "Будешь участвовать в Random Coffee на следующей неделе? ☕️"
	answers := []gotgbot.InputPollOption{
		{Text: "Да! 🤗"},
		{Text: "Не в этот раз 💁🏽"},
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

	// Pin the poll with notification
	err = s.messageSender.PinMessage(
		sentPollMsg.Chat.Id,
		sentPollMsg.MessageId,
		true,
	)
	if err != nil {
		return fmt.Errorf("%s: Failed to pin poll: %v", utils.GetCurrentTypeName(), err)
	}

	// Save to database
	return s.savePollToDB(sentPollMsg)
}

// savePollToDB saves the poll information to the database
func (s *RandomCoffeeService) savePollToDB(sentPollMsg *gotgbot.Message) error {
	if s.pollRepo == nil {
		log.Printf("%s: pollRepo is nil, skipping DB interaction.", utils.GetCurrentTypeName())
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
		"%s: Calculated WeekStartDate: %s (UTC)",
		utils.GetCurrentTypeName(),
		weekStartDate.Format("2006-01-02"),
	)

	newPollEntry := repositories.RandomCoffeePoll{
		MessageID:      sentPollMsg.MessageId,
		TelegramPollID: sentPollMsg.Poll.Id,
		WeekStartDate:  weekStartDate,
	}

	pollID, err := s.pollRepo.CreatePoll(newPollEntry)
	if err != nil {
		log.Printf("%s: Failed to save random coffee poll to DB: %v. Poll Message ID: %d",
			utils.GetCurrentTypeName(),
			err,
			sentPollMsg.MessageId)
		return err
	}

	log.Printf("%s: Random coffee poll saved to DB with ID: %d, Original MessageID: %d, WeekStartDate: %s",
		utils.GetCurrentTypeName(),
		pollID,
		sentPollMsg.MessageId,
		weekStartDate.Format("2006-01-02"),
	)

	return nil
}

func (s *RandomCoffeeService) GenerateAndSendPairs() error {
	latestPoll, err := s.pollRepo.GetLatestPoll()
	if err != nil {
		return fmt.Errorf("%s: error getting latest poll: %w", utils.GetCurrentTypeName(), err)
	}
	if latestPoll == nil {
		return fmt.Errorf("%s: опрос для рандом кофе не найден", utils.GetCurrentTypeName())
	}

	// Stop the poll first before generating pairs
	chatID := utils.ChatIdToFullChatId(s.config.SuperGroupChatID)
	_, err = s.pollSender.StopPoll(chatID, latestPoll.MessageID, nil)
	if err != nil {
		log.Printf("%s: Warning - failed to stop poll (message ID %d): %v", utils.GetCurrentTypeName(), latestPoll.MessageID, err)
		// Continue anyway - we might still be able to generate pairs
	} else {
		log.Printf("%s: Successfully stopped poll (message ID %d)", utils.GetCurrentTypeName(), latestPoll.MessageID)
	}

	participants, err := s.participantRepo.GetParticipatingUsers(latestPoll.ID)
	if err != nil {
		return fmt.Errorf("%s: error getting participants for poll ID %d: %w", utils.GetCurrentTypeName(), latestPoll.ID, err)
	}

	if len(participants) < 2 {
		return fmt.Errorf("недостаточно участников для создания пар (нужно минимум 2, зарегистрировалось %d)", len(participants))
	}

	// Update participant info using Telegram Bot API if any field has changed
	for i := range participants {
		participant := &participants[i]
		user, err := s.userRepo.GetByTelegramID(participant.TgID)
		if err != nil {
			log.Printf("%s: error getting user by telegram ID %d: %v", utils.GetCurrentTypeName(), participant.TgID, err)
			continue
		}

		// Get current user info from Telegram
		chatMember, err := s.bot.GetChatMember(chatID, participant.TgID, nil)
		if err != nil {
			log.Printf("%s: error getting chat member for user ID %d: %v", utils.GetCurrentTypeName(), participant.TgID, err)
			continue
		}

		currentUser := chatMember.GetUser()

		// Check and update username
		if user.TgUsername != currentUser.Username {
			err = s.userRepo.UpdateTelegramUsername(user.ID, currentUser.Username)
			if err != nil {
				log.Printf("%s: error updating username for user ID %d: %v", utils.GetCurrentTypeName(), user.ID, err)
			} else {
				log.Printf("%s: Updated username for user ID %d from '%s' to '%s'", utils.GetCurrentTypeName(), user.ID, user.TgUsername, currentUser.Username)
				participant.TgUsername = currentUser.Username
			}
		}

		// Check and update firstname
		if user.Firstname != currentUser.FirstName {
			err = s.userRepo.UpdateFirstname(user.ID, currentUser.FirstName)
			if err != nil {
				log.Printf("%s: error updating firstname for user ID %d: %v", utils.GetCurrentTypeName(), user.ID, err)
			} else {
				log.Printf("%s: Updated firstname for user ID %d from '%s' to '%s'", utils.GetCurrentTypeName(), user.ID, user.Firstname, currentUser.FirstName)
				participant.Firstname = currentUser.FirstName
			}
		}

		// Check and update lastname
		if user.Lastname != currentUser.LastName {
			err = s.userRepo.UpdateLastname(user.ID, currentUser.LastName)
			if err != nil {
				log.Printf("%s: error updating lastname for user ID %d: %v", utils.GetCurrentTypeName(), user.ID, err)
			} else {
				log.Printf("%s: Updated lastname for user ID %d from '%s' to '%s'", utils.GetCurrentTypeName(), user.ID, user.Lastname, currentUser.LastName)
				participant.Lastname = currentUser.LastName
			}
		}
	}

	// Smart Pairing Logic with History Consideration
	pairs, unpaired, err := s.generateSmartPairs(participants, int(latestPoll.ID))
	if err != nil {
		log.Printf("%s: Smart pairing failed, falling back to random: %v", utils.GetCurrentTypeName(), err)
		// Fallback to old random logic
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		r.Shuffle(len(participants), func(i, j int) {
			participants[i], participants[j] = participants[j], participants[i]
		})
		pairs, unpaired = s.createPairsFromShuffled(participants, int(latestPoll.ID))
	}

	// Format pairs display text
	var pairsText []string
	var unpairedUserText string

	for _, pair := range pairs {
		user1Display := s.formatUserDisplay(&pair.User1)
		user2Display := s.formatUserDisplay(&pair.User2)
		pairsText = append(pairsText, fmt.Sprintf("%s x %s", user1Display, user2Display))
	}

	if unpaired != nil {
		unpairedUserText = s.formatUserDisplay(unpaired)
	}

	var messageBuilder strings.Builder
	messageBuilder.WriteString(fmt.Sprintf("☕️ Пары для рандом кофе ➪ <b><i>неделя %s</i></b>:\n\n", latestPoll.WeekStartDate.Format("Mon, Jan 2")))
	for _, pair := range pairsText {
		messageBuilder.WriteString(fmt.Sprintf("➪ %s\n", pair))
	}
	if unpairedUserText != "" {
		messageBuilder.WriteString(fmt.Sprintf("\n😔 %s без пары и ищет компанию на эту неделю!\n", unpairedUserText))
	}
	messageBuilder.WriteString("\n🗓 День, время и формат встречи вы выбираете сами. Просто напиши своей паре в личку, когда и в каком формате тебе удобно встретиться.")

	// Send the pairing message
	opts := &gotgbot.SendMessageOpts{
		MessageThreadId: int64(s.config.RandomCoffeeTopicID),
	}

	message, err := s.messageSender.SendHtmlWithReturnMessage(chatID, messageBuilder.String(), opts)
	if err != nil {
		return fmt.Errorf("%s: error sending pairing message to chat %d: %w", utils.GetCurrentTypeName(), chatID, err)
	}

	// Pin the message without notification
	err = s.messageSender.PinMessage(message.Chat.Id, message.MessageId, false)
	if err != nil {
		log.Printf("%s: Failed to pin message: %v", utils.GetCurrentTypeName(), err)
	}

	// Award points to participants
	if s.pointsLogRepo != nil {
		err = s.awardPointsToParticipants(pairs, unpaired, latestPoll.ID)
		if err != nil {
			log.Printf("%s: Error awarding points to participants: %v", utils.GetCurrentTypeName(), err)
		}
	}

	log.Printf("%s: Successfully sent pairings for poll ID %d to chat %d.", utils.GetCurrentTypeName(), latestPoll.ID, s.config.SuperGroupChatID)
	return nil
}

func (s *RandomCoffeeService) formatUserDisplay(user *repositories.User) string {
	userDisplay := user.Firstname

	if user.TgUsername != "" {
		userDisplay = fmt.Sprintf("@%s", user.TgUsername)
	}

	profile, err := s.profileRepo.GetOrCreate(user.ID)
	if err != nil {
		log.Printf("%s: Error getting profile for user %d: %v", utils.GetCurrentTypeName(), user.ID, err)
		return userDisplay
	}

	if profile.PublishedMessageID.Valid &&
		profile.PublishedMessageID.Int64 > 0 {
		profileLink := utils.GetIntroMessageLink(s.config, profile.PublishedMessageID.Int64)
		linkedName := fmt.Sprintf(" <i>(<a href=\"%s\">профиль</a>)</i>", profileLink)

		userDisplay += linkedName
	}

	return userDisplay
}

// CoffeePair represents a pair of users for coffee meetings
type CoffeePair struct {
	User1 repositories.User
	User2 repositories.User
}

// generateSmartPairs creates pairs considering history to avoid recent repeats
func (s *RandomCoffeeService) generateSmartPairs(participants []repositories.User, pollID int) ([]CoffeePair, *repositories.User, error) {
	if len(participants) < 2 {
		return nil, nil, fmt.Errorf("not enough participants for pairing")
	}

	// Get user IDs for history lookup
	userIDs := make([]int, len(participants))
	for i, user := range participants {
		userIDs[i] = user.ID
	}

	// Get history of pairs from last 4 polls
	pairHistory, err := s.pairRepo.GetPairsHistoryForUsers(userIDs, 4)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get pair history: %w", err)
	}

	log.Printf("%s: Smart pairing for %d participants, found %d historical pair combinations",
		utils.GetCurrentTypeName(), len(participants), len(pairHistory))

	// Create a copy of participants for pairing
	availableUsers := make([]repositories.User, len(participants))
	copy(availableUsers, participants)

	// Randomly shuffle to maintain fairness
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(availableUsers), func(i, j int) {
		availableUsers[i], availableUsers[j] = availableUsers[j], availableUsers[i]
	})

	var pairs []CoffeePair
	used := make(map[int]bool)

	// Try to pair users avoiding recent history
	for i := 0; i < len(availableUsers); i++ {
		if used[availableUsers[i].ID] {
			continue
		}

		user1 := availableUsers[i]
		bestPartner := -1
		oldestPoll := 999999 // Very high number to represent "never paired"

		// Find best partner (never paired or oldest pairing)
		for j := i + 1; j < len(availableUsers); j++ {
			if used[availableUsers[j].ID] {
				continue
			}

			user2 := availableUsers[j]
			pairKey := fmt.Sprintf("%d-%d", user1.ID, user2.ID)

			// Check if they were paired in recent history
			if pollIDs, exists := pairHistory[pairKey]; exists && len(pollIDs) > 0 {
				// Find the oldest poll ID (minimum value) where they were paired
				oldestPairPoll := pollIDs[0]
				for _, pollID := range pollIDs {
					if pollID < oldestPairPoll {
						oldestPairPoll = pollID
					}
				}

				// If this is older than our current best, update
				if oldestPairPoll < oldestPoll {
					oldestPoll = oldestPairPoll
					bestPartner = j
				}
			} else {
				// Never paired - perfect match
				bestPartner = j
				oldestPoll = 0 // Reset to indicate "never paired"
				break
			}
		}

		// If we found a partner, create the pair
		if bestPartner >= 0 {
			user2 := availableUsers[bestPartner]
			pairs = append(pairs, CoffeePair{User1: user1, User2: user2})

			// Log pairing decision
			if oldestPoll == 0 {
				log.Printf("%s: Created NEW pair: %s x %s (never paired before)",
					utils.GetCurrentTypeName(), user1.Firstname, user2.Firstname)
			} else {
				log.Printf("%s: Created REPEAT pair: %s x %s (last paired in poll %d)",
					utils.GetCurrentTypeName(), user1.Firstname, user2.Firstname, oldestPoll)
			}

			// Save to database
			if s.pairRepo != nil {
				u1ID, u2ID := user1.ID, user2.ID
				if u1ID > u2ID {
					u1ID, u2ID = u2ID, u1ID
				}
				err := s.pairRepo.CreatePair(pollID, u1ID, u2ID)
				if err != nil {
					log.Printf("%s: failed to save smart pair to DB: %v", utils.GetCurrentTypeName(), err)
				}
			}

			used[user1.ID] = true
			used[user2.ID] = true
		}
	}

	// Find any unpaired user
	var unpaired *repositories.User
	for _, user := range availableUsers {
		if !used[user.ID] {
			unpaired = &user
			break
		}
	}

	return pairs, unpaired, nil
}

// createPairsFromShuffled creates pairs from already shuffled participants (fallback method)
func (s *RandomCoffeeService) createPairsFromShuffled(participants []repositories.User, pollID int) ([]CoffeePair, *repositories.User) {
	var pairs []CoffeePair
	var unpaired *repositories.User

	for i := 0; i < len(participants); i += 2 {
		user1 := participants[i]

		if i+1 < len(participants) {
			user2 := participants[i+1]
			pairs = append(pairs, CoffeePair{User1: user1, User2: user2})

			if s.pairRepo != nil {
				u1ID, u2ID := user1.ID, user2.ID
				if u1ID > u2ID {
					u1ID, u2ID = u2ID, u1ID
				}
				err := s.pairRepo.CreatePair(pollID, u1ID, u2ID)
				if err != nil {
					log.Printf("%s: failed to save fallback pair to DB: %v", utils.GetCurrentTypeName(), err)
				}
			}
		} else {
			unpaired = &user1
		}
	}

	return pairs, unpaired
}

// awardPointsToParticipants awards points to all participants who actually participated in coffee pairing
func (s *RandomCoffeeService) awardPointsToParticipants(pairs []CoffeePair, unpaired *repositories.User, pollID int64) error {
	pointsPerParticipation := constants.PointsPerRandomCoffeeParticipation
	participationReason := constants.RandomCoffeeParticipationReason
	
	// Award points to paired users
	for _, pair := range pairs {
		// Check if user1 already received points for this poll
		existingLog, err := s.pointsLogRepo.GetPointsForPoll(pair.User1.ID, pollID)
		if err != nil {
			log.Printf("%s: Error checking existing points for user %d poll %d: %v", utils.GetCurrentTypeName(), pair.User1.ID, pollID, err)
		} else if existingLog == nil {
			err = s.pointsLogRepo.AddPoints(pair.User1.ID, pointsPerParticipation, participationReason, &pollID)
			if err != nil {
				log.Printf("%s: Error awarding points to user %d: %v", utils.GetCurrentTypeName(), pair.User1.ID, err)
			} else {
				log.Printf("%s: Awarded %d points to user %s (ID: %d) for Random Coffee participation", 
					utils.GetCurrentTypeName(), pointsPerParticipation, pair.User1.Firstname, pair.User1.ID)
			}
		}

		// Check if user2 already received points for this poll
		existingLog, err = s.pointsLogRepo.GetPointsForPoll(pair.User2.ID, pollID)
		if err != nil {
			log.Printf("%s: Error checking existing points for user %d poll %d: %v", utils.GetCurrentTypeName(), pair.User2.ID, pollID, err)
		} else if existingLog == nil {
			err = s.pointsLogRepo.AddPoints(pair.User2.ID, pointsPerParticipation, participationReason, &pollID)
			if err != nil {
				log.Printf("%s: Error awarding points to user %d: %v", utils.GetCurrentTypeName(), pair.User2.ID, err)
			} else {
				log.Printf("%s: Awarded %d points to user %s (ID: %d) for Random Coffee participation", 
					utils.GetCurrentTypeName(), pointsPerParticipation, pair.User2.Firstname, pair.User2.ID)
			}
		}
	}

	// Award points to unpaired user if exists
	if unpaired != nil {
		existingLog, err := s.pointsLogRepo.GetPointsForPoll(unpaired.ID, pollID)
		if err != nil {
			log.Printf("%s: Error checking existing points for unpaired user %d poll %d: %v", utils.GetCurrentTypeName(), unpaired.ID, pollID, err)
		} else if existingLog == nil {
			err = s.pointsLogRepo.AddPoints(unpaired.ID, pointsPerParticipation, participationReason, &pollID)
			if err != nil {
				log.Printf("%s: Error awarding points to unpaired user %d: %v", utils.GetCurrentTypeName(), unpaired.ID, err)
			} else {
				log.Printf("%s: Awarded %d points to unpaired user %s (ID: %d) for Random Coffee participation", 
					utils.GetCurrentTypeName(), pointsPerParticipation, unpaired.Firstname, unpaired.ID)
			}
		}
	}

	return nil
}
