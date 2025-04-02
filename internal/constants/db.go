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

// ContentStatus represents the status of content
type ContentStatus string

const (
	ContentStatusFinished ContentStatus = "finished"
	ContentStatusActual   ContentStatus = "actual"
)

// AllContentStatuses is a slice containing all possible ContentStatus values
var AllContentStatuses = []ContentStatus{
	ContentStatusFinished,
	ContentStatusActual,
}
