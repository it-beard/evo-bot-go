package constants

// ContentType represents the type of content
type ContentType string

const (
	ContentTypeClubCall ContentType = "club-call"
	ContentTypeMeetup   ContentType = "meetup"
)

// AllContentTypes is a slice containing all possible ContentType values
var AllContentTypes = []ContentType{
	ContentTypeClubCall,
	ContentTypeMeetup,
}
