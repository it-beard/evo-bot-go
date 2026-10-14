package services

import (
	"testing"

	"evo-bot-go/internal/database/repositories"
	"evo-bot-go/internal/testutils"

	"github.com/stretchr/testify/assert"
)

func TestProfileService_IsProfileComplete(t *testing.T) {
	complete := func() (*repositories.User, *repositories.Profile) {
		return &repositories.User{Firstname: "Ivan", Lastname: "Petrov"}, &repositories.Profile{Bio: "Go developer"}
	}

	tests := []struct {
		name   string
		mutate func(u **repositories.User, p **repositories.Profile)
		want   bool
	}{
		{name: "complete profile", mutate: func(**repositories.User, **repositories.Profile) {}, want: true},
		{name: "nil user", mutate: func(u **repositories.User, _ **repositories.Profile) { *u = nil }, want: false},
		{name: "nil profile", mutate: func(_ **repositories.User, p **repositories.Profile) { *p = nil }, want: false},
		{name: "missing first name", mutate: func(u **repositories.User, _ **repositories.Profile) { (*u).Firstname = "" }, want: false},
		{name: "missing last name", mutate: func(u **repositories.User, _ **repositories.Profile) { (*u).Lastname = "" }, want: false},
		{name: "missing bio", mutate: func(_ **repositories.User, p **repositories.Profile) { (*p).Bio = "" }, want: false},
	}

	bot, client := testutils.NewTestBot()
	s := NewProfileService(bot)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, profile := complete()
			tt.mutate(&user, &profile)

			assert.Equal(t, tt.want, s.IsProfileComplete(user, profile))
		})
	}
	assert.Empty(t, client.Calls(), "the check is purely local")
}
