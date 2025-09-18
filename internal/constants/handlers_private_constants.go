package constants

const ToolsCommand = "tools"
const ContentCommand = "content"
const EventsCommand = "events"
const TopicsCommand = "topics"
const TopicAddCommand = "topicAdd"
const HelpCommand = "help"
const StartCommand = "start"
const IntroCommand = "intro"
const ProfileCommand = "profile"

// Callback data constants for profile handler
const (
	ProfilePrefix                = "profile_"
	ProfileEditMyProfileCallback = ProfilePrefix + "edit_my"
	ProfileSearchProfileCallback = ProfilePrefix + "search_profile"

	ProfileEditBioCallback       = ProfilePrefix + "edit_bio"
	ProfileEditFirstnameCallback = ProfilePrefix + "edit_firstname"
	ProfileEditLastnameCallback  = ProfilePrefix + "edit_lastname"

	ProfileStartCallback = ProfilePrefix + "start"
	ProfileFullCancel    = "full_cancel" + ProfilePrefix
)
