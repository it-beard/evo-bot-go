package grouphandlersservices

import (
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"
	"log"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

type RandomCoffeePollAnswersService struct {
	messageSenderService *services.MessageSenderService
	config               *config.Config
	pollRepo             *repositories.RandomCoffeePollRepository
	participantRepo      *repositories.RandomCoffeeParticipantRepository
	userRepo             *repositories.UserRepository
}

func NewRandomCoffeePollAnswersService(
	messageSenderService *services.MessageSenderService,
	config *config.Config,
	pollRepo *repositories.RandomCoffeePollRepository,
	participantRepo *repositories.RandomCoffeeParticipantRepository,
	userRepo *repositories.UserRepository,
) *RandomCoffeePollAnswersService {
	return &RandomCoffeePollAnswersService{
		messageSenderService: messageSenderService,
		config:               config,
		pollRepo:             pollRepo,
		participantRepo:      participantRepo,
		userRepo:             userRepo,
	}
}

func (s *RandomCoffeePollAnswersService) ProcessAnswer(pollAnswer *gotgbot.PollAnswer, internalUser *repositories.User) error {
	// Get poll from the database using Telegram's Poll ID
	retrievedPoll, err := s.pollRepo.GetPollByTelegramPollID(pollAnswer.PollId)
	if err != nil {
		log.Printf("%s: Error fetching poll by telegram_poll_id %s: %v", utils.GetCurrentTypeName(), pollAnswer.PollId, err)
		return nil
	}
	if retrievedPoll == nil {
		// This poll answer is not for a poll we are tracking (e.g., some other poll in the chat, or an old poll).
		log.Printf("%s: Poll with telegram_poll_id %s not found in our DB. Ignoring.", utils.GetCurrentTypeName(), pollAnswer.PollId)
		return nil
	}

	if len(pollAnswer.OptionIds) == 0 { // Vote retracted
		err = s.participantRepo.RemoveParticipant(retrievedPoll.ID, int64(internalUser.ID))
		if err != nil {
			log.Printf("%s: Error removing participant (PollID: %d, UserID: %d): %v", utils.GetCurrentTypeName(), retrievedPoll.ID, internalUser.ID, err)
		} else {
			log.Printf("%s: Participant (PollID: %d, UserID: %d) removed due to vote retraction.", utils.GetCurrentTypeName(), retrievedPoll.ID, internalUser.ID)
		}
	} else { // New vote or changed vote
		// Assuming "Yes, I'll participate" is the first option (index 0) and "No" is the second (index 1)
		// The poll options are: {Text: "Yes, I'll participate"}, {Text: "No, not this week"}
		// So, OptionIds[0] being 0 means "Yes", OptionIds[0] being 1 means "No".
		isParticipating := pollAnswer.OptionIds[0] == 0

		participant := repositories.RandomCoffeeParticipant{
			PollID:          retrievedPoll.ID,
			UserID:          int64(internalUser.ID),
			IsParticipating: isParticipating,
		}
		err = s.participantRepo.UpsertParticipant(participant)
		if err != nil {
			log.Printf("%s: Error upserting participant (PollID: %d, UserID: %d, Participating: %t): %v", utils.GetCurrentTypeName(), retrievedPoll.ID, internalUser.ID, isParticipating, err)
		} else {
			log.Printf("%s: Participant (PollID: %d, UserID: %d, Participating: %t) upserted.", utils.GetCurrentTypeName(), retrievedPoll.ID, internalUser.ID, isParticipating)
		}
	}
	return nil
}

func (s *RandomCoffeePollAnswersService) IsAnswerShouldBeProcessed(pollAnswer *gotgbot.PollAnswer, internalUser *repositories.User) bool {
	// Do nothing if pollAnswer is nil
	if pollAnswer == nil {
		log.Printf("%s: Received nil PollAnswer", utils.GetCurrentTypeName())
		return false
	}

	// Anonymous answers (voter_chat) have no user, and the user may be missing after a database error
	if pollAnswer.User == nil || internalUser == nil {
		log.Printf("%s: Poll answer without a known user. Ignoring.", utils.GetCurrentTypeName())
		return false
	}

	// Do nothing if user is bot and send message to admin
	if pollAnswer.User.IsBot {
		if len(pollAnswer.OptionIds) > 0 && pollAnswer.OptionIds[0] == 0 {
			s.messageSenderService.SendHtml(
				s.config.AdminUserID,
				"🚫 К сожалению, участие в опросе Random Coffee для ботов недоступно. Пожалуйста, отзови свой голос.",
				nil,
			)
		}
		return false
	}
	// Do nothing if user is banned from coffee and send message to user
	if internalUser.HasCoffeeBan {
		log.Printf("%s: User %d is banned. Ignoring.", utils.GetCurrentTypeName(), pollAnswer.User.Id)
		if len(pollAnswer.OptionIds) > 0 && pollAnswer.OptionIds[0] == 0 {
			s.messageSenderService.SendHtml(
				internalUser.TgID,
				"🚫 К сожалению, участие в опросе Random Coffee для тебя недоступно, так как ты находишься в бане. "+
					"Пожалуйста, отзови свой голос, и обратись к администратору для разблокировки.",
				nil,
			)
		}
		return false
	}

	return true
}

// GetInternalUser returns the voter's database user, or nil when the answer has no user or it cannot be loaded
func (s *RandomCoffeePollAnswersService) GetInternalUser(pollAnswer *gotgbot.PollAnswer) *repositories.User {
	if pollAnswer == nil || pollAnswer.User == nil {
		return nil
	}

	internalUser, err := s.userRepo.GetOrCreate(pollAnswer.User)
	if err != nil {
		log.Printf("%s: Error getting user %d for poll answer: %v", utils.GetCurrentTypeName(), pollAnswer.User.Id, err)
		return nil
	}
	return internalUser
}
