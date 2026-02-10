package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/biodoia/golem/pkg/zhipu"
)

const sessionDir = ".golem/sessions"

// Session represents a chat session
type Session struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Model     string          `json:"model"`
	Messages  []zhipu.Message `json:"messages"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// SessionManager manages chat sessions
type SessionManager struct {
	sessions map[string]*Session
	current  *Session
}

// NewSessionManager creates a new session manager
func NewSessionManager() (*SessionManager, error) {
	sm := &SessionManager{
		sessions: make(map[string]*Session),
	}

	// Load existing sessions
	path := filepath.Join(os.Getenv("HOME"), sessionDir)
	if err := os.MkdirAll(path, 0755); err != nil {
		return sm, err
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return sm, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			session, err := LoadSession(entry.Name())
			if err == nil {
				sm.sessions[session.ID] = session
			}
		}
	}

	return sm, nil
}

// CreateSession creates a new session
func (sm *SessionManager) CreateSession(name string, model string) *Session {
	session := &Session{
		ID:        generateID(),
		Name:      name,
		Model:     model,
		Messages:  []zhipu.Message{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	sm.sessions[session.ID] = session
	sm.current = session
	return session
}

// GetSession returns a session by ID
func (sm *SessionManager) GetSession(id string) (*Session, bool) {
	s, ok := sm.sessions[id]
	return s, ok
}

// Current returns the current session
func (sm *SessionManager) Current() *Session {
	return sm.current
}

// SetCurrent sets the current session
func (sm *SessionManager) SetCurrent(session *Session) {
	sm.current = session
}

// AddMessage adds a message to the current session
func (sm *SessionManager) AddMessage(role string, content string) {
	if sm.current == nil {
		sm.CreateSession("New Session", "glm-4-32b-0414")
	}
	sm.current.Messages = append(sm.current.Messages, zhipu.Message{
		Role:    role,
		Content: content,
	})
	sm.current.UpdatedAt = time.Now()
}

// Save saves a session to disk
func (sm *SessionManager) Save(session *Session) error {
	if session == nil {
		return nil
	}
	path := filepath.Join(os.Getenv("HOME"), sessionDir, session.ID)
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadSession loads a session from disk
func LoadSession(id string) (*Session, error) {
	path := filepath.Join(os.Getenv("HOME"), sessionDir, id)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// ListSessions lists all sessions
func (sm *SessionManager) ListSessions() []*Session {
	sessions := make([]*Session, 0, len(sm.sessions))
	for _, s := range sm.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// DeleteSession deletes a session
func (sm *SessionManager) DeleteSession(id string) error {
	delete(sm.sessions, id)
	path := filepath.Join(os.Getenv("HOME"), sessionDir, id)
	return os.Remove(path)
}

func generateID() string {
	return time.Now().Format("20060102150405")
}
