package grouphandlers

import (
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"
	"log"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/pollanswer"
)

type RandomCoffeePollAnswerHandler struct {
	config               *config.Config
	userRepo             *repositories.UserRepository
	pollRepo             *repositories.RandomCoffeePollRepository
	participantRepo      *repositories.RandomCoffeeParticipantRepository
	messageSenderService *services.MessageSenderService
}

func NewRandomCoffeePollAnswerHandler(
	config *config.Config,
	userRepo *repositories.UserRepository,
	pollRepo *repositories.RandomCoffeePollRepository,
	participantRepo *repositories.RandomCoffeeParticipantRepository,
	messageSenderService *services.MessageSenderService,
) ext.Handler {
	h := &RandomCoffeePollAnswerHandler{
		config:               config,
		userRepo:             userRepo,
		pollRepo:             pollRepo,
		participantRepo:      participantRepo,
		messageSenderService: messageSenderService,
	}
	return handlers.NewPollAnswer(pollanswer.All, h.handleUpdate)
}

func (h *RandomCoffeePollAnswerHandler) handleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	pollAnswer := ctx.PollAnswer
	if pollAnswer == nil {
		// This case should ideally not be reached if pollanswer.All filter is working as expected.
		log.Printf("%s: Received nil PollAnswer", utils.GetCurrentTypeName())
		return nil
	}

	// 0. Check if user is bot
	if pollAnswer.User.IsBot {
		log.Printf("%s: Bot tried to vote. Ignoring.", utils.GetCurrentTypeName())
		if len(pollAnswer.OptionIds) > 0 && pollAnswer.OptionIds[0] == 0 {
			err := h.messageSenderService.SendHtml(
				h.config.AdminUserID,
				"🚫 К сожалению, участие в опросе Random Coffee для ботов недоступно. Пожалуйста, отзови свой голос.",
				nil,
			)
			if err != nil {
				log.Printf("%s: Error sending message to admin: %v", utils.GetCurrentTypeName(), err)
			}
		}
		return nil
	}

	// 1. Get internal user ID from database
	internalUser, err := h.userRepo.GetOrCreate(pollAnswer.User)
	if err != nil {
		log.Printf("%s: Error getting user by tg_id %d: %v", utils.GetCurrentTypeName(), pollAnswer.User.Id, err)
		return nil // Returning nil to avoid stopping the bot for one failed handler
	}

	// 2. Check if user is banned from coffee
	if internalUser.HasCoffeeBan {
		log.Printf("%s: User %d is banned. Ignoring.", utils.GetCurrentTypeName(), pollAnswer.User.Id)
		if len(pollAnswer.OptionIds) > 0 && pollAnswer.OptionIds[0] == 0 {
			err := h.messageSenderService.SendHtml(
				internalUser.TgID,
				"🚫 К сожалению, участие в опросе Random Coffee для тебя недоступно, так как ты находишься в бане. "+
					"Пожалуйста, отзови свой голос, и обратись к администратору для разблокировки.",
				nil,
			)
			if err != nil {
				log.Printf("%s: Error sending message to user %d: %v", utils.GetCurrentTypeName(), pollAnswer.User.Id, err)
			}
		}
		return nil
	}

	// 3. Get our poll from the database using Telegram's Poll ID
	retrievedPoll, err := h.pollRepo.GetPollByTelegramPollID(pollAnswer.PollId)
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
		err = h.participantRepo.RemoveParticipant(retrievedPoll.ID, int64(internalUser.ID))
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
		err = h.participantRepo.UpsertParticipant(participant)
		if err != nil {
			log.Printf("%s: Error upserting participant (PollID: %d, UserID: %d, Participating: %t): %v", utils.GetCurrentTypeName(), retrievedPoll.ID, internalUser.ID, isParticipating, err)
		} else {
			log.Printf("%s: Participant (PollID: %d, UserID: %d, Participating: %t) upserted.", utils.GetCurrentTypeName(), retrievedPoll.ID, internalUser.ID, isParticipating)
		}
	}
	return nil
}
