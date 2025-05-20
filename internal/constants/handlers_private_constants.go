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
	ProfilePrefix                   = "profile_"
	ProfileViewMyProfileCallback    = ProfilePrefix + "view_my"
	ProfileEditMyProfileCallback    = ProfilePrefix + "edit_my"
	ProfileViewOtherProfileCallback = ProfilePrefix + "view_other"

	ProfileEditBioCallback      = ProfilePrefix + "edit_bio"
	ProfileEditLinkedinCallback = ProfilePrefix + "edit_linkedin"
	ProfileEditGithubCallback   = ProfilePrefix + "edit_github"
	ProfileEditWebsiteCallback  = ProfilePrefix + "edit_website"

	ProfileStartCallback = ProfilePrefix + "start"
	ProfileSaveCallback  = ProfilePrefix + "save"
	ProfileFullCancel    = "full_cancel" + ProfilePrefix
)
