package services

import (
	"evo-bot-go/internal/config"
	"evo-bot-go/internal/constants"
	"evo-bot-go/internal/utils"
	"log"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

type PermissionsService struct {
	config               *config.Config
	bot                  *gotgbot.Bot
	messageSenderService *MessageSenderService
}

func NewPermissionsService(
	config *config.Config,
	bot *gotgbot.Bot,
	messageSenderService *MessageSenderService,
) *PermissionsService {
	return &PermissionsService{
		config:               config,
		bot:                  bot,
		messageSenderService: messageSenderService,
	}
}

// CheckAdminPermissions checks if the user has admin permissions and returns an appropriate error response
// Returns true if user has permission, false otherwise
func (s *PermissionsService) CheckAdminPermissions(msg *gotgbot.Message, commandName string) bool {
	if !utils.IsUserAdminOrCreator(s.bot, msg.From.Id, s.config) {
		if err := s.messageSenderService.Reply(
			msg,
			"Эта команда доступна только администраторам.",
			nil,
		); err != nil {
			log.Printf("%s: Failed to send admin-only message: %v", utils.GetCurrentTypeName(), err)
		}
		log.Printf("%s: User %d tried to use %s without admin rights", utils.GetCurrentTypeName(), msg.From.Id, commandName)
		return false
	}

	return true
}

// CheckPrivateChatType checks if the command is used in a private chat and returns an appropriate error response
// Returns true if used in private chat, false otherwise
func (s *PermissionsService) CheckPrivateChatType(msg *gotgbot.Message) bool {
	if msg.Chat.Type != constants.PrivateChatType {
		if err := s.messageSenderService.ReplyWithCleanupAfterDelayWithPing(
			msg,
			"*Прошу прощения* 🧐\n\nЭта команда работает только в _личной беседе_ со мной. "+
				"Напишите мне в ЛС, и я с удовольствием помогу (я тебя там пинганул, если мы общались ранее)."+
				"\n\nДанное сообщение и твоя команда будут автоматически удалены через 10 секунд.",
			10, &gotgbot.SendMessageOpts{
				ParseMode: "Markdown",
			}); err != nil {
			log.Printf("%s: Failed to send private-only message: %v", utils.GetCurrentTypeName(), err)
		}
		return false
	}

	return true
}

func (s *PermissionsService) CheckClubMemberPermissions(msg *gotgbot.Message, commandName string) bool {
	if !utils.IsUserClubMember(s.bot, msg.From.Id, s.config) {
		if err := s.messageSenderService.Reply(
			msg,
			"Эта команда доступна только участникам клуба.",
			nil,
		); err != nil {
			log.Printf("%s: Failed to send club-only message: %v", utils.GetCurrentTypeName(), err)
		}
		log.Printf("%s: User %d tried to use %s without club member rights", utils.GetCurrentTypeName(), msg.From.Id, commandName)
		return false
	}

	return true
}

// CheckAdminAndPrivateChat combines permission and chat type checking for admin-only private commands
// Returns true if all checks pass, false otherwise
func (s *PermissionsService) CheckAdminAndPrivateChat(msg *gotgbot.Message, commandName string) bool {
	if !s.CheckAdminPermissions(msg, commandName) {
		return false
	}

	if !s.CheckPrivateChatType(msg) {
		return false
	}

	return true
}
