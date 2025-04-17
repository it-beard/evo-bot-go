package constants

// EventType represents the type of event
type EventType string

const (
	EventTypeClubCall    EventType = "club-call"
	EventTypeMeetup      EventType = "meetup"
	EventTypeWorkshop    EventType = "workshop"
	EventTypeReadingClub EventType = "reading-club"
	EventTypeConference  EventType = "conference"
)

// AllEventTypes is a slice containing all possible EventType values
var AllEventTypes = []EventType{
	EventTypeClubCall,
	EventTypeMeetup,
	EventTypeWorkshop,
	EventTypeReadingClub,
	EventTypeConference,
}

// EventStatus represents the status of event
type EventStatus string

const (
	EventStatusFinished EventStatus = "finished"
	EventStatusActual   EventStatus = "actual"
)

// AllEventStatuses is a slice containing all possible EventStatus values
var AllEventStatuses = []EventStatus{
	EventStatusFinished,
	EventStatusActual,
}
