package constants

const PrivateChatType = "private"
const SuperGroupChatType = "supergroup"
const CancelCommand = "cancel"

// Profile fields
const (
	ProfileBioLengthLimit = 3900 //max Telegram message length
)

// TopicTextLengthLimit caps a topic or question added via /topicAdd, so a single topic cannot make
// the topic list of an event too long to be sent
const TopicTextLengthLimit = 1000

// Search types
const (
	SearchTypeFast = "fast"
	SearchTypeDeep = "deep"
)
