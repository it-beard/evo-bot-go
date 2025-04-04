package tasks

import (
	"log"
	"time"

	"evo-bot-go/internal/clients"
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
	log.Printf("Session Keep-Alive Task: Starting session keep-alive task with interval %s", s.interval)

	// First time refresh
	if err := clients.TgKeepSessionAlive(); err != nil {
		log.Printf("Session Keep-Alive Task: Failed to keep session alive: %v", err)
	} else {
		log.Printf("Session Keep-Alive Task: Session refresh successful")
	}

	go s.run()
}

// Stop stops the session keep-alive task
func (s *SessionKeepAliveTask) Stop() {
	log.Println("Session Keep-Alive Task: Stopping session keep-alive task")
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
			log.Println("Session Keep-Alive Task: Running scheduled session keep-alive")

			// Run keep-alive in a separate goroutine
			go func() {
				if err := clients.TgKeepSessionAlive(); err != nil {
					log.Printf("Session Keep-Alive Task: Failed to keep session alive: %v", err)
				} else {
					log.Printf("Session Keep-Alive Task: Session refresh successful")
				}
			}()
		}
	}
}
