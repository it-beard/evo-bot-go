package grouphandlers

import (
	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/utils"
	"fmt"
	"log"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/chatmember"
)

type JoinLeftHandler struct {
	userRepo *repositories.UserRepository
}

func NewJoinLeftHandler(userRepo *repositories.UserRepository) ext.Handler {
	h := &JoinLeftHandler{userRepo: userRepo}
	return handlers.NewChatMember(chatmember.All, h.handle)
}

func (h *JoinLeftHandler) handle(b *gotgbot.Bot, ctx *ext.Context) error {
	chatMember := ctx.ChatMember
	user := chatMember.NewChatMember.GetUser()

	newStatus := chatMember.NewChatMember.GetStatus()

	isNowMember := newStatus == "member" || newStatus == "administrator" || newStatus == "creator"
	isNowLeftOrBanned := newStatus == "left" || newStatus == "kicked"

	if isNowMember {
		dbUser, _, err := h.userRepo.GetOrFullCreate(&user)
		if err != nil {
			return fmt.Errorf("%s: failed to get or create user: %w", utils.GetCurrentTypeName(), err)
		}
		log.Printf("%s: User %s (%d) is now a member, setting IsClubMember to true", utils.GetCurrentTypeName(), user.Username, user.Id)
		err = h.userRepo.SetClubMemberStatus(dbUser.ID, true)
		if err != nil {
			return fmt.Errorf("%s: failed to set club member status to true for user %d: %w", utils.GetCurrentTypeName(), dbUser.ID, err)
		}

	} else if isNowLeftOrBanned {
		dbUser, _, err := h.userRepo.GetOrFullCreate(&user)
		if err != nil {
			return fmt.Errorf("%s: failed to get or create user: %w", utils.GetCurrentTypeName(), err)

		}

		if dbUser.IsClubMember {
			log.Printf("%s: User %s (%d) is now left/banned, setting IsClubMember to false", utils.GetCurrentTypeName(), user.Username, user.Id)
			err := h.userRepo.SetClubMemberStatus(dbUser.ID, false)
			if err != nil {
				return fmt.Errorf("%s: failed to set club member status to false for user %d: %w", utils.GetCurrentTypeName(), dbUser.ID, err)
			}
		}
	}

	return nil
}
