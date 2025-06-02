package grouphandlers

import (
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/models"
	"log"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/pollanswer"
)

type PollAnswerHandler struct {
	config          *config.Config
	userRepo        *repositories.UserRepository // Assuming this repository is available and provides GetUserByTgID
	pollRepo        *repositories.WeeklyMeetingPollRepository
	participantRepo *repositories.WeeklyMeetingParticipantRepository
}

func NewPollAnswerHandler(
	config *config.Config,
	userRepo *repositories.UserRepository,
	pollRepo *repositories.WeeklyMeetingPollRepository,
	participantRepo *repositories.WeeklyMeetingParticipantRepository,
) ext.Handler {
	h := &PollAnswerHandler{
		config:          config,
		userRepo:        userRepo,
		pollRepo:        pollRepo,
		participantRepo: participantRepo,
	}
	return handlers.NewPollAnswer(pollanswer.All, h.handleUpdate)
}

func (h *PollAnswerHandler) handleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	pollAnswer := ctx.PollAnswer
	if pollAnswer == nil {
		// This case should ideally not be reached if pollanswer.All filter is working as expected.
		log.Println("PollAnswerHandler: Received nil PollAnswer")
		return nil
	}

	// 1. Get internal user ID from database
	// Assuming userRepo.GetUserByTgID exists and is correctly implemented.
	internalUser, err := h.userRepo.GetByTelegramID(pollAnswer.User.Id)
	if err != nil {
		log.Printf("PollAnswerHandler: Error getting user by tg_id %d: %v", pollAnswer.User.Id, err)
		return nil // Returning nil to avoid stopping the bot for one failed handler
	}
	if internalUser == nil {
		log.Printf("PollAnswerHandler: User with tg_id %d not found in DB. Ignoring poll answer.", pollAnswer.User.Id)
		return nil
	}

	// 2. Get our poll from the database using Telegram's Poll ID
	retrievedPoll, err := h.pollRepo.GetPollByTelegramPollID(pollAnswer.PollId)
	if err != nil {
		log.Printf("PollAnswerHandler: Error fetching poll by telegram_poll_id %s: %v", pollAnswer.PollId, err)
		return nil
	}
	if retrievedPoll == nil {
		// This poll answer is not for a poll we are tracking (e.g., some other poll in the chat, or an old poll).
		// log.Printf("PollAnswerHandler: Poll with telegram_poll_id %s not found in our DB. Ignoring.", pollAnswer.PollId)
		return nil
	}

	// Optional: Check if the poll originated from the expected chat, if necessary.
	// However, Telegram Poll IDs are globally unique, so this might be redundant if polls are only sent to one chat.
	// if retrievedPoll.ChatID != h.config.SupergroupChatID {
	//  log.Printf("PollAnswerHandler: Poll %s was answered in chat %d, but expected chat %d. Ignoring.",
	//      pollAnswer.PollId, internalUser.TgID, h.config.SupergroupChatID)
	//  return nil
	// }

	if len(pollAnswer.OptionIds) == 0 { // Vote retracted
		err = h.participantRepo.RemoveParticipant(retrievedPoll.ID, int64(internalUser.ID))
		if err != nil {
			log.Printf("PollAnswerHandler: Error removing participant (PollID: %d, UserID: %d): %v", retrievedPoll.ID, internalUser.ID, err)
		} else {
			log.Printf("PollAnswerHandler: Participant (PollID: %d, UserID: %d) removed due to vote retraction.", retrievedPoll.ID, internalUser.ID)
		}
	} else { // New vote or changed vote
		// Assuming "Yes, I'll participate" is the first option (index 0) and "No" is the second (index 1)
		// The poll options are: {Text: "Yes, I'll participate"}, {Text: "No, not this week"}
		// So, OptionIds[0] being 0 means "Yes", OptionIds[0] being 1 means "No".
		isParticipating := pollAnswer.OptionIds[0] == 0

		participant := models.WeeklyMeetingParticipant{
			PollID:          retrievedPoll.ID,
			UserID:          int64(internalUser.ID),
			IsParticipating: isParticipating,
		}
		err = h.participantRepo.UpsertParticipant(participant)
		if err != nil {
			log.Printf("PollAnswerHandler: Error upserting participant (PollID: %d, UserID: %d, Participating: %t): %v", retrievedPoll.ID, internalUser.ID, isParticipating, err)
		} else {
			log.Printf("PollAnswerHandler: Participant (PollID: %d, UserID: %d, Participating: %t) upserted.", retrievedPoll.ID, internalUser.ID, isParticipating)
		}
	}
	return nil // Errors are logged, and we don't want to stop other handlers.
}

// Name method for the handler interface (optional, but good practice)
func (h *PollAnswerHandler) Name() string {
	return "PollAnswerHandler"
}
