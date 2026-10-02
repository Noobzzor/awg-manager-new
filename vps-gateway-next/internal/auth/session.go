package auth

import (
	"crypto/rand"
	"encoding/hex"
	"github.com/hoaxisr/awg-manager/internal/logging"
	"sync"
	"time"
)

const (
	// defaultSessionTTL is used when no TTL getter is wired (nil) or the
	// getter returns a non-positive duration.
	defaultSessionTTL = 24 * time.Hour
	SessionCookie     = "awg_session"
	tokenLength       = 32
	cleanupInterval   = 5 * time.Minute
)

// Session represents an authenticated user session.
type Session struct {
	Token     string
	Login     string
	CreatedAt time.Time
	LastSeen  time.Time
}

// SessionStore manages user sessions in memory.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	stopCh   chan struct{}
	// log — журнал приложения (system/auth); nil-safe. Истечение сессии —
	// причина внезапного «разлогинило», без строки в журнале она выглядит
	// как сбой.
	log *logging.ScopedLogger
	// ttl returns the configured session lifetime. Read LIVE on every
	// expiry check, so a shortened TTL takes effect immediately for
	// already-issued sessions (the server-side check is authoritative;
	// the browser cookie Max-Age catches up on the next login).
	ttl func() time.Duration
}

// NewSessionStore creates a new session store and starts cleanup goroutine.
// ttl supplies the session lifetime (wired to the settings store); nil or
// a non-positive result falls back to the 24h default.
func NewSessionStore(ttl func() time.Duration) *SessionStore {
	s := &SessionStore{
		sessions: make(map[string]*Session),
		stopCh:   make(chan struct{}),
		ttl:      ttl,
	}
	go s.cleanupLoop()
	return s
}

// TTL returns the current session lifetime — used for the expiry checks,
// the login cookie Max-Age and the /auth/status expiresIn field.
func (s *SessionStore) TTL() time.Duration {
	if s.ttl != nil {
		if d := s.ttl(); d > 0 {
			return d
		}
	}
	return defaultSessionTTL
}

// Create creates a new session for the given login and returns the token.
func (s *SessionStore) Create(login string) (string, error) {
	token, err := generateToken()
	if err != nil {
		return "", err
	}

	now := time.Now()
	session := &Session{
		Token:     token,
		Login:     login,
		CreatedAt: now,
		LastSeen:  now,
	}

	s.mu.Lock()
	s.sessions[token] = session
	s.mu.Unlock()

	return token, nil
}

// Get retrieves a session by token and updates LastSeen.
// Returns nil if session doesn't exist or is expired.
func (s *SessionStore) Get(token string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[token]
	if !ok {
		return nil
	}

	// Check if expired (TTL read live — settings changes apply immediately)
	if time.Since(session.LastSeen) > s.TTL() {
		delete(s.sessions, token)
		if s.log != nil {
			s.log.Info("session-expired", session.Login, "session expired (inactive longer than TTL)")
		}
		return nil
	}

	// Update last seen (session activity extends TTL)
	session.LastSeen = time.Now()
	return session
}

// Delete removes a session by token.
func (s *SessionStore) Delete(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

// Stop stops the cleanup goroutine.
func (s *SessionStore) Stop() {
	close(s.stopCh)
}

// cleanupLoop periodically removes expired sessions.
func (s *SessionStore) cleanupLoop() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanup()
		case <-s.stopCh:
			return
		}
	}
}

// cleanup removes all expired sessions.
func (s *SessionStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	ttl := s.TTL()
	for token, session := range s.sessions {
		if now.Sub(session.LastSeen) > ttl {
			delete(s.sessions, token)
			if s.log != nil {
				s.log.Info("session-expired", session.Login, "session expired (inactive longer than TTL)")
			}
		}
	}
}

// SetLogger подключает журнал приложения (после создания logging.Service).
func (s *SessionStore) SetLogger(log *logging.ScopedLogger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log = log
}

// generateToken creates a cryptographically secure random token.
func generateToken() (string, error) {
	bytes := make([]byte, tokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
