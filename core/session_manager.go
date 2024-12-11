package core

import (
	"sync"
	"time"

	"github.com/kgretzky/evilginx2/log"
)

const (
	defaultCleanupInterval = 5 * time.Minute
	defaultSessionTimeout  = 24 * time.Hour
)

type SessionManager struct {
	sessions    map[string]*Session
	lock        sync.RWMutex
	stopCleanup chan struct{}
}

func NewSessionManager() *SessionManager {
	sm := &SessionManager{
		sessions:    make(map[string]*Session),
		stopCleanup: make(chan struct{}),
	}
	go sm.cleanupLoop()
	return sm
}

func (sm *SessionManager) Add(s *Session) {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	sm.sessions[s.Id] = s
}

func (sm *SessionManager) Get(id string) *Session {
	sm.lock.RLock()
	defer sm.lock.RUnlock()
	if s, ok := sm.sessions[id]; ok {
		s.lock.Lock()
		s.LastAccessed = time.Now().UTC()
		s.lock.Unlock()
		return s
	}
	return nil
}

func (sm *SessionManager) Remove(id string) {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	delete(sm.sessions, id)
}

func (sm *SessionManager) cleanupLoop() {
	ticker := time.NewTicker(defaultCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.cleanup()
		case <-sm.stopCleanup:
			return
		}
	}
}

func (sm *SessionManager) cleanup() {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	now := time.Now().UTC()
	for id, s := range sm.sessions {
		s.lock.RLock()
		if now.After(s.ExpiresAt) || now.Sub(s.LastAccessed) > defaultSessionTimeout {
			log.Info("Session expired: %s", id)
			delete(sm.sessions, id)
		}
		s.lock.RUnlock()
	}
}

func (sm *SessionManager) Stop() {
	close(sm.stopCleanup)
}
