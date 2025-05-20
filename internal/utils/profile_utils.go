package utils

import "evo-bot-go/internal/database/repositories"

// IsProfileComplete checks if a profile has the minimum required fields for publishing
func IsProfileComplete(user *repositories.User, profile *repositories.Profile) bool {
	if user == nil || profile == nil {
		return false
	}

	if user.Firstname == "" {
		return false
	}

	if user.Lastname == "" {
		return false
	}

	if profile.Bio == "" {
		return false
	}

	// Check if at least one link is set
	hasLink := profile.LinkedIn != "" || profile.GitHub != "" || profile.FreeLink != ""
	return hasLink
}
