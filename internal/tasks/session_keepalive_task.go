package tasks

import (
	"log"
	"time"

	"evo-bot-go/internal/clients"
	"evo-bot-go/internal/utils"
)

// SessionKeepAliveTask handles scheduling of session keep-alive tasks for tg_user client session.
type SessionKeepAliveTask struct {
	interval time.Duration
	stop     chan struct{}
}

// NewSessionKeepAliveTask creates a new session keep-alive task
func NewSessionKeepAliveTask(interval time.Duration) *SessionKeepAliveTask {
	return &SessionKeepAliveTask{
		interval: interval,
		stop:     make(chan struct{}),
	}
}

// Start starts the session keep-alive task
func (s *SessionKeepAliveTask) Start() {
	log.Printf("%s: Starting session keep-alive task with interval %s", utils.GetCurrentTypeName(), s.interval)

	// First time refresh
	if err := clients.TgKeepSessionAlive(); err != nil {
		log.Printf("%s: Failed to keep session alive: %v", utils.GetCurrentTypeName(), err)
	} else {
		log.Printf("%s: Session refresh successful", utils.GetCurrentTypeName())
	}

	go s.run()
}

// Stop stops the session keep-alive task
func (s *SessionKeepAliveTask) Stop() {
	log.Printf("%s: Stopping session keep-alive task", utils.GetCurrentTypeName())
	close(s.stop)
}

// run runs the session keep-alive task
func (s *SessionKeepAliveTask) run() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			log.Printf("%s: Running scheduled session keep-alive", utils.GetCurrentTypeName())

			// Run keep-alive in a separate goroutine
			go func() {
				if err := clients.TgKeepSessionAlive(); err != nil {
					log.Printf("%s: Failed to keep session alive: %v", utils.GetCurrentTypeName(), err)
				} else {
					log.Printf("%s: Session refresh successful", utils.GetCurrentTypeName())
				}
			}()
		}
	}
}
