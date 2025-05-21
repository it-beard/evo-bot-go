package constants

const CodeCommand = "code"
const TrySummarizeCommand = "trySummarize"
const SummarizeDmFlag = "-dm"

// Event Handlers
const EventEditCommand = "eventEdit"
const EventEditGetLastLimit = 10
const EventSetupCommand = "eventSetup"
const EventDeleteCommand = "eventDelete"
const EventStartCommand = "eventStart"

// Topics Handlers
const ShowTopicsCommand = "showTopics"

// Profiles Handler
const AdminProfilesCommand = "profiles"

// Callback data constants for admin "/profiles" handler
const (
	AdminProfilesPrefix         = "admin_profiles_"
	AdminProfilesEditCallback   = AdminProfilesPrefix + "edit"
	AdminProfilesCreateCallback = AdminProfilesPrefix + "create"

	AdminProfilesEditBioCallback          = AdminProfilesPrefix + "edit_bio"
	AdminProfilesEditFirstnameCallback    = AdminProfilesPrefix + "edit_firstname"
	AdminProfilesEditLastnameCallback     = AdminProfilesPrefix + "edit_lastname"
	AdminProfilesEditCoffeeBanCallback    = AdminProfilesPrefix + "edit_coffee_ban"
	AdminProfilesPublishCallback          = AdminProfilesPrefix + "publish"
	AdminProfilesPublishNoPreviewCallback = AdminProfilesPrefix + "publish_without_preview"

	AdminProfilesStartCallback  = AdminProfilesPrefix + "start"
	AdminProfilesCancelCallback = AdminProfilesPrefix + "cancel"
)
