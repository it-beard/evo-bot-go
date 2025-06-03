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
const AdminProfilesCommand = "profilesManager"

// Callback data constants for admin "/profilesManager" handler
const (
	AdminProfilesPrefix                           = "admin_profiles_"
	AdminProfilesSearchByUsernameCallback         = AdminProfilesPrefix + "search_by_username"
	AdminProfilesSearchByTelegramIDCallback       = AdminProfilesPrefix + "search_by_telegram_id"
	AdminProfilesSearchByFullNameCallback         = AdminProfilesPrefix + "search_by_full_name"
	AdminProfilesCreateByForwardedMessageCallback = AdminProfilesPrefix + "create_by_forwarded_message"
	AdminProfilesCreateByTelegramIDCallback       = AdminProfilesPrefix + "create_by_telegram_id"
	AdminProfilesEditMenuCallback                 = AdminProfilesPrefix + "edit_menu"

	AdminProfilesEditBioCallback          = AdminProfilesPrefix + "edit_bio"
	AdminProfilesEditFirstnameCallback    = AdminProfilesPrefix + "edit_firstname"
	AdminProfilesEditLastnameCallback     = AdminProfilesPrefix + "edit_lastname"
	AdminProfilesEditUsernameCallback     = AdminProfilesPrefix + "edit_username"
	AdminProfilesEditCoffeeBanCallback    = AdminProfilesPrefix + "edit_coffee_ban"
	AdminProfilesToggleCoffeeBanCallback  = AdminProfilesPrefix + "toggle_coffee_ban"
	AdminProfilesPublishCallback          = AdminProfilesPrefix + "publish"
	AdminProfilesPublishNoPreviewCallback = AdminProfilesPrefix + "publish_without_preview"

	AdminProfilesStartCallback  = AdminProfilesPrefix + "start"
	AdminProfilesCancelCallback = AdminProfilesPrefix + "cancel"
)

const CoffeeGeneratePairsCommand = "coffeeGeneratePairs"
const CoffeeRestartCommand = "coofeeRestart"

// Coffee Start Handler callback constants
const (
	CoffeeStartPrefix          = "coffee_start_"
	CoffeeStartConfirmCallback = CoffeeStartPrefix + "confirm"
	CoffeeStartCancelCallback  = CoffeeStartPrefix + "cancel"
)

// Coffee Generate Pairs Handler callback constants
const (
	CoffeeGeneratePairsPrefix          = "coffee_generate_pairs_"
	CoffeeGeneratePairsConfirmCallback = CoffeeGeneratePairsPrefix + "confirm"
	CoffeeGeneratePairsBackCallback    = CoffeeGeneratePairsPrefix + "back"
	CoffeeGeneratePairsCancelCallback  = CoffeeGeneratePairsPrefix + "cancel"
)
