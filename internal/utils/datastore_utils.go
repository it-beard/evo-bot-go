package utils

import (
	"sync"
)

// UserDataStore provides thread-safe storage for user conversation data
type UserDataStore struct {
	rwMux    sync.RWMutex
	userData map[int64]map[string]any
}

// NewUserDataStore creates a new UserDataStore instance
func NewUserDataStore() *UserDataStore {
	return &UserDataStore{
		userData: make(map[int64]map[string]any),
	}
}

// Get retrieves a value for a user by key
func (s *UserDataStore) Get(userID int64, key string) (any, bool) {
	s.rwMux.RLock()
	defer s.rwMux.RUnlock()

	userData, ok := s.userData[userID]
	if !ok {
		return nil, false
	}

	v, ok := userData[key]
	return v, ok
}

// Set stores a value for a user by key
func (s *UserDataStore) Set(userID int64, key string, val any) {
	s.rwMux.Lock()
	defer s.rwMux.Unlock()

	userData, ok := s.userData[userID]
	if !ok {
		userData = make(map[string]any)
		s.userData[userID] = userData
	}

	userData[key] = val
}

// Clear removes all data for a user
func (s *UserDataStore) Clear(userID int64) {
	s.rwMux.Lock()
	defer s.rwMux.Unlock()

	delete(s.userData, userID)
}

// SetPreviousMessageInfo stores message ID and chat ID for a user
func (s *UserDataStore) SetPreviousMessageInfo(userID int64, messageID int64, chatID int64, messageIDKey string, chatIDKey string) {
	s.Set(userID, messageIDKey, messageID)
	s.Set(userID, chatIDKey, chatID)
}

// GetPreviousMessageInfo retrieves the stored message ID and chat ID for a user
func (s *UserDataStore) GetPreviousMessageInfo(userID int64, messageIDKey string, chatIDKey string) (messageID int64, chatID int64) {
	if val, ok := s.Get(userID, messageIDKey); ok {
		messageID = val.(int64)
	}
	if val, ok := s.Get(userID, chatIDKey); ok {
		chatID = val.(int64)
	}
	return
}
