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
	ProfileBioSearchCallback        = ProfilePrefix + "bio_search"

	ProfileEditBioCallback               = ProfilePrefix + "edit_bio"
	ProfileEditFirstnameCallback         = ProfilePrefix + "edit_firstname"
	ProfileEditLastnameCallback          = ProfilePrefix + "edit_lastname"
	ProfilePublishCallback               = ProfilePrefix + "publish"
	ProfilePublishWithoutPreviewCallback = ProfilePrefix + "publish_without_preview"

	ProfileStartCallback = ProfilePrefix + "start"
	ProfileFullCancel    = "full_cancel" + ProfilePrefix
)
