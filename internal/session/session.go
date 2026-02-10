package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/biodoia/golem/pkg/zhipu"
)

const (
	sessionDir         = ".golem/sessions"
	DefaultAutoSaveN   = 5 // Auto-save every N messages
)

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
	mu           sync.RWMutex
	sessions     map[string]*Session
	current      *Session
	autoSaveN    int
	messageCount int
}

// NewSessionManager creates a new session manager
func NewSessionManager() (*SessionManager, error) {
	sm := &SessionManager{
		sessions:  make(map[string]*Session),
		autoSaveN: DefaultAutoSaveN,
	}

	// Load existing sessions
	path := sm.sessionDir()
	if err := os.MkdirAll(path, 0755); err != nil {
		return sm, err
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return sm, err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			id := strings.TrimSuffix(entry.Name(), ".json")
			session, err := sm.loadSession(id)
			if err == nil {
				sm.sessions[session.ID] = session
			}
		}
	}

	return sm, nil
}

// SetAutoSaveN sets how many messages trigger an auto-save
func (sm *SessionManager) SetAutoSaveN(n int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.autoSaveN = n
}

// sessionDir returns the session storage directory
func (sm *SessionManager) sessionDir() string {
	return filepath.Join(os.Getenv("HOME"), sessionDir)
}

// CreateSession creates a new session
func (sm *SessionManager) CreateSession(name string, model string) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

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
	sm.messageCount = 0
	return session
}

// GetSession returns a session by ID
func (sm *SessionManager) GetSession(id string) (*Session, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	s, ok := sm.sessions[id]
	return s, ok
}

// Current returns the current session
func (sm *SessionManager) Current() *Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.current
}

// SetCurrent sets the current session
func (sm *SessionManager) SetCurrent(session *Session) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.current = session
	sm.messageCount = len(session.Messages)
}

// AddMessage adds a message to the current session and triggers auto-save if needed
func (sm *SessionManager) AddMessage(role string, content string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.current == nil {
		sm.createSessionLocked("New Session", "glm-4-32b-0414")
	}
	sm.current.Messages = append(sm.current.Messages, zhipu.Message{
		Role:    role,
		Content: content,
	})
	sm.current.UpdatedAt = time.Now()
	sm.messageCount++

	// Return true if auto-save should trigger
	return sm.autoSaveN > 0 && sm.messageCount >= sm.autoSaveN
}

// createSessionLocked creates a session (caller must hold lock)
func (sm *SessionManager) createSessionLocked(name string, model string) *Session {
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
	sm.messageCount = 0
	return session
}

// ResetAutoSaveCounter resets the message counter after saving
func (sm *SessionManager) ResetAutoSaveCounter() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.messageCount = 0
}

// Save saves a session to disk
func (sm *SessionManager) Save(session *Session) error {
	if session == nil {
		return nil
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()

	path := filepath.Join(sm.sessionDir(), session.ID+".json")
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	sm.messageCount = 0 // Reset counter on save
	return os.WriteFile(path, data, 0644)
}

// Load loads a session by ID and sets it as current
func (sm *SessionManager) Load(id string) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check if already loaded
	if s, ok := sm.sessions[id]; ok {
		sm.current = s
		sm.messageCount = len(s.Messages)
		return s, nil
	}

	// Try to load from disk
	session, err := sm.loadSession(id)
	if err != nil {
		return nil, err
	}
	sm.sessions[session.ID] = session
	sm.current = session
	sm.messageCount = len(session.Messages)
	return session, nil
}

// loadSession loads a session from disk
func (sm *SessionManager) loadSession(id string) (*Session, error) {
	path := filepath.Join(sm.sessionDir(), id+".json")
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

// LoadSession loads a session from disk (for backward compatibility)
func LoadSession(id string) (*Session, error) {
	path := filepath.Join(os.Getenv("HOME"), sessionDir, id+".json")
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

// ListSessions lists all sessions sorted by UpdatedAt (newest first)
func (sm *SessionManager) ListSessions() []*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := make([]*Session, 0, len(sm.sessions))
	for _, s := range sm.sessions {
		sessions = append(sessions, s)
	}

	// Sort by UpdatedAt descending
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions
}

// SessionInfo returns a formatted info string for a session
type SessionInfo struct {
	ID        string
	Name      string
	Model     string
	Messages  int
	CreatedAt time.Time
	UpdatedAt time.Time
	IsCurrent bool
}

// ListSessionsInfo returns detailed info about all sessions
func (sm *SessionManager) ListSessionsInfo() []SessionInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	infos := make([]SessionInfo, 0, len(sm.sessions))
	for _, s := range sm.sessions {
		infos = append(infos, SessionInfo{
			ID:        s.ID,
			Name:      s.Name,
			Model:     s.Model,
			Messages:  len(s.Messages),
			CreatedAt: s.CreatedAt,
			UpdatedAt: s.UpdatedAt,
			IsCurrent: sm.current != nil && sm.current.ID == s.ID,
		})
	}

	// Sort by UpdatedAt descending
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].UpdatedAt.After(infos[j].UpdatedAt)
	})

	return infos
}

// DeleteSession deletes a session
func (sm *SessionManager) DeleteSession(id string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.current != nil && sm.current.ID == id {
		sm.current = nil
	}
	delete(sm.sessions, id)
	path := filepath.Join(sm.sessionDir(), id+".json")
	return os.Remove(path)
}

// RenameSession renames a session
func (sm *SessionManager) RenameSession(id string, newName string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	s, ok := sm.sessions[id]
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}
	s.Name = newName
	s.UpdatedAt = time.Now()
	
	// Save to disk
	path := filepath.Join(sm.sessionDir(), s.ID+".json")
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// FindSessionByPartialID finds a session by partial ID match
func (sm *SessionManager) FindSessionByPartialID(partial string) (*Session, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var matches []*Session
	for id, s := range sm.sessions {
		if strings.HasPrefix(id, partial) {
			matches = append(matches, s)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no session found matching: %s", partial)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("ambiguous session ID: %s (matches %d sessions)", partial, len(matches))
	}
	return matches[0], nil
}

// ExportSession exports a session to a specified path
func (sm *SessionManager) ExportSession(id string, path string) error {
	sm.mu.RLock()
	s, ok := sm.sessions[id]
	sm.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ImportSession imports a session from a file
func (sm *SessionManager) ImportSession(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	// Generate new ID to avoid conflicts
	session.ID = generateID()
	session.UpdatedAt = time.Now()

	sm.mu.Lock()
	sm.sessions[session.ID] = &session
	sm.mu.Unlock()

	// Save to our session dir
	return &session, sm.Save(&session)
}

func generateID() string {
	return time.Now().Format("20060102150405")
}
