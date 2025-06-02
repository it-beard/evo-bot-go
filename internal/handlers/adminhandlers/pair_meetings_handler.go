package adminhandlers

import (
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/database/repositories" // User model is also in here
	"evo-bot-go/internal/services"
	"evo-bot-go/internal/utils"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

type PairMeetingsHandler struct {
	config          *config.Config
	permissions     *services.PermissionsService
	sender          *services.MessageSenderService
	pollRepo        *repositories.WeeklyMeetingPollRepository
	participantRepo *repositories.WeeklyMeetingParticipantRepository
}

func NewPairMeetingsHandler(
	config *config.Config,
	permissions *services.PermissionsService,
	sender *services.MessageSenderService,
	pollRepo *repositories.WeeklyMeetingPollRepository,
	participantRepo *repositories.WeeklyMeetingParticipantRepository,
) ext.Handler {
	h := &PairMeetingsHandler{
		config:          config,
		permissions:     permissions,
		sender:          sender,
		pollRepo:        pollRepo,
		participantRepo: participantRepo,
	}
	return handlers.NewCommand(constants.PairMeetingsCommand, h.handleCommand)
}

func (h *PairMeetingsHandler) handleCommand(b *gotgbot.Bot, ctx *ext.Context) error {
	if !utils.IsUserAdminOrCreator(b, ctx.EffectiveUser.Id, h.config) { // Using IsAdmin for permission check
		log.Printf("PairMeetingsHandler: User %d (%s) tried to use /%s without admin permissions.",
			ctx.EffectiveUser.Id, ctx.EffectiveUser.Username, constants.PairMeetingsCommand)
		// Optionally send a message, or just ignore
		// h.sender.Reply(ctx.EffectiveMessage, "You do not have permission to use this command.", nil)
		return nil
	}

	chatID := h.config.SuperGroupChatID // Assuming polls are always in the supergroup
	if chatID == 0 {
		log.Println("PairMeetingsHandler: SupergroupChatID is not configured.")
		h.sender.Reply(ctx.EffectiveMessage, "Supergroup chat ID is not configured.", nil)
		return nil
	}

	latestPoll, err := h.pollRepo.GetLatestPollForChat(chatID)
	if err != nil {
		log.Printf("PairMeetingsHandler: Error getting latest poll for chat %d: %v", chatID, err)
		h.sender.Reply(ctx.EffectiveMessage, "Error fetching poll information.", nil)
		return nil
	}
	if latestPoll == nil {
		h.sender.Reply(ctx.EffectiveMessage, fmt.Sprintf("No weekly meeting poll found for chat ID %d.", chatID), nil)
		return nil
	}

	// Optional: Check if this poll is for the current/upcoming week
	// Example: if latestPoll.WeekStartDate is too old, maybe warn admin or find a more recent one.
	// For simplicity, we'll use the latest one found.

	participants, err := h.participantRepo.GetParticipatingUsers(latestPoll.ID)
	if err != nil {
		log.Printf("PairMeetingsHandler: Error getting participants for poll ID %d: %v", latestPoll.ID, err)
		h.sender.Reply(ctx.EffectiveMessage, "Error fetching participants.", nil)
		return nil
	}

	if len(participants) < 2 {
		msg := fmt.Sprintf("Not enough participants for pairing from poll (ID: %d, Week: %s). Need at least 2, got %d.",
			latestPoll.ID, latestPoll.WeekStartDate.Format("2006-01-02"), len(participants))
		h.sender.Send(chatID, msg, nil) // Send to supergroup
		log.Printf("PairMeetingsHandler: %s", msg)
		return nil
	}

	// Random Pairing Logic
	r := rand.New(rand.NewSource(time.Now().UnixNano())) // Create a new Rand instance
	r.Shuffle(len(participants), func(i, j int) {
		participants[i], participants[j] = participants[j], participants[i]
	})

	var pairsText []string
	var unpairedUserText string // Changed to string for direct use

	for i := 0; i < len(participants); i += 2 {
		user1 := participants[i]
		user1Display := user1.Firstname
		if user1.TgUsername != "" {
			user1Display = fmt.Sprintf("%s (@%s)", user1.Firstname, user1.TgUsername)
		}

		if i+1 < len(participants) {
			user2 := participants[i+1]
			user2Display := user2.Firstname
			if user2.TgUsername != "" {
				user2Display = fmt.Sprintf("%s (@%s)", user2.Firstname, user2.TgUsername)
			}
			pairsText = append(pairsText, fmt.Sprintf("%s - %s", user1Display, user2Display))
		} else {
			unpairedUserText = user1Display // Last user is unpaired
		}
	}

	var messageBuilder strings.Builder
	messageBuilder.WriteString(fmt.Sprintf("☕️ Pairs for Random Coffee (Week of %s):\n\n", latestPoll.WeekStartDate.Format("Mon, Jan 2")))
	for _, pair := range pairsText {
		messageBuilder.WriteString(fmt.Sprintf("• %s\n", pair))
	}
	if unpairedUserText != "" {
		messageBuilder.WriteString(fmt.Sprintf("\n😔 %s is looking for a coffee buddy this week!\n", unpairedUserText))
	}
	messageBuilder.WriteString("\n🗓 День, время и формат встречи вы выбираете сами. Просто напиши партнеру в личку, когда и в каком формате тебе удобно встретиться.")

	// Send the pairing message to the SupergroupChatID, not as a reply to the admin command.
	err = h.sender.Send(chatID, messageBuilder.String(), nil)
	if err != nil {
		log.Printf("PairMeetingsHandler: Error sending pairing message to chat %d: %v", chatID, err)
		// Notify admin who invoked the command about the failure
		h.sender.Reply(ctx.EffectiveMessage, "Error sending pairing message to the group. Please check logs.", nil)
	} else {
		log.Printf("PairMeetingsHandler: Successfully sent pairings for poll ID %d to chat %d.", latestPoll.ID, chatID)
		// Optionally, confirm to admin
		h.sender.Reply(ctx.EffectiveMessage, "Pairings announced in the supergroup.", nil)
	}
	return nil
}

// Name method for the handler interface (optional, but good practice)
func (h *PairMeetingsHandler) Name() string {
	return "PairMeetingsHandler"
}
