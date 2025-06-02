package grouphandlers

import (
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/database/repositories"
	"log"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/pollanswer"
)

type RandomCoffeePollAnswerHandler struct {
	config          *config.Config
	userRepo        *repositories.UserRepository
	pollRepo        *repositories.RandomCoffeePollRepository
	participantRepo *repositories.RandomCoffeeParticipantRepository
}

func NewRandomCoffeePollAnswerHandler(
	config *config.Config,
	userRepo *repositories.UserRepository,
	pollRepo *repositories.RandomCoffeePollRepository,
	participantRepo *repositories.RandomCoffeeParticipantRepository,
) ext.Handler {
	h := &RandomCoffeePollAnswerHandler{
		config:          config,
		userRepo:        userRepo,
		pollRepo:        pollRepo,
		participantRepo: participantRepo,
	}
	return handlers.NewPollAnswer(pollanswer.All, h.handleUpdate)
}

func (h *RandomCoffeePollAnswerHandler) handleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	pollAnswer := ctx.PollAnswer
	if pollAnswer == nil {
		// This case should ideally not be reached if pollanswer.All filter is working as expected.
		log.Println("RandomCoffeePollAnswerHandler: Received nil PollAnswer")
		return nil
	}

	// 1. Get internal user ID from database
	// Assuming userRepo.GetUserByTgID exists and is correctly implemented.
	internalUser, err := h.userRepo.GetByTelegramID(pollAnswer.User.Id)
	if err != nil {
		log.Printf("RandomCoffeePollAnswerHandler: Error getting user by tg_id %d: %v", pollAnswer.User.Id, err)
		return nil // Returning nil to avoid stopping the bot for one failed handler
	}
	if internalUser == nil {
		log.Printf("RandomCoffeePollAnswerHandler: User with tg_id %d not found in DB. Ignoring poll answer.", pollAnswer.User.Id)
		return nil
	}

	// 2. Get our poll from the database using Telegram's Poll ID
	retrievedPoll, err := h.pollRepo.GetPollByTelegramPollID(pollAnswer.PollId)
	if err != nil {
		log.Printf("RandomCoffeePollAnswerHandler: Error fetching poll by telegram_poll_id %s: %v", pollAnswer.PollId, err)
		return nil
	}
	if retrievedPoll == nil {
		// This poll answer is not for a poll we are tracking (e.g., some other poll in the chat, or an old poll).
		log.Printf("RandomCoffeePollAnswerHandler: Poll with telegram_poll_id %s not found in our DB. Ignoring.", pollAnswer.PollId)
		return nil
	}

	if len(pollAnswer.OptionIds) == 0 { // Vote retracted
		err = h.participantRepo.RemoveParticipant(retrievedPoll.ID, int64(internalUser.ID))
		if err != nil {
			log.Printf("RandomCoffeePollAnswerHandler: Error removing participant (PollID: %d, UserID: %d): %v", retrievedPoll.ID, internalUser.ID, err)
		} else {
			log.Printf("RandomCoffeePollAnswerHandler: Participant (PollID: %d, UserID: %d) removed due to vote retraction.", retrievedPoll.ID, internalUser.ID)
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
		err = h.participantRepo.UpsertParticipant(participant)
		if err != nil {
			log.Printf("RandomCoffeePollAnswerHandler: Error upserting participant (PollID: %d, UserID: %d, Participating: %t): %v", retrievedPoll.ID, internalUser.ID, isParticipating, err)
		} else {
			log.Printf("RandomCoffeePollAnswerHandler: Participant (PollID: %d, UserID: %d, Participating: %t) upserted.", retrievedPoll.ID, internalUser.ID, isParticipating)
		}
	}
	return nil // Errors are logged, and we don't want to stop other handlers.
}

// Name method for the handler interface (optional, but good practice)
func (h *RandomCoffeePollAnswerHandler) Name() string {
	return "RandomCoffeePollAnswerHandler"
}
