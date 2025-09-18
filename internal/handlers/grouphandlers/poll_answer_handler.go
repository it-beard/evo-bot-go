package grouphandlers

import (
	"evo-bot-go/internal/services/grouphandlersservices"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/pollanswer"
)

type PollAnswerHandler struct {
	randomCoffeePollAnswersService *grouphandlersservices.RandomCoffeePollAnswersService
}

func NewPollAnswerHandler(
	randomCoffeePollAnswersService *grouphandlersservices.RandomCoffeePollAnswersService,
) ext.Handler {
	h := &PollAnswerHandler{
		randomCoffeePollAnswersService: randomCoffeePollAnswersService,
	}
	return handlers.NewPollAnswer(pollanswer.All, h.handleUpdate)
}

func (h *PollAnswerHandler) handleUpdate(b *gotgbot.Bot, ctx *ext.Context) error {
	internalUser := h.randomCoffeePollAnswersService.GetInternalUser(ctx.PollAnswer)

	if h.randomCoffeePollAnswersService.IsAnswerShouldBeProcessed(ctx.PollAnswer, internalUser) {
		return h.randomCoffeePollAnswersService.ProcessAnswer(ctx.PollAnswer, internalUser)
	}

	return nil
}
