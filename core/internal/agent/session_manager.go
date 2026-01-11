package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	context_manager "github.com/igoryan-dao/ricochet/internal/context"
	"github.com/igoryan-dao/ricochet/internal/protocol"
)

// SessionData is the persistable part of a session
type SessionData struct {
	ID        string             `json:"id"`
	Messages  []protocol.Message `json:"messages"`
	Todos     []protocol.Todo    `json:"todos"`
	CreatedAt time.Time          `json:"created_at"`
}

// SessionManager handles concurrent agents and their persistence
type SessionManager struct {
	mu         sync.RWMutex
	sessions   map[string]*Session
	storageDir string
}

func NewSessionManager(storageDir string) *SessionManager {
	if storageDir != "" {
		if err := os.MkdirAll(storageDir, 0755); err != nil {
			log.Printf("Warning: failed to create storage dir: %v", err)
		}
	}

	manager := &SessionManager{
		sessions:   make(map[string]*Session),
		storageDir: storageDir,
	}

	manager.LoadAll()
	return manager
}

func (m *SessionManager) CreateSession() *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("s_%d", time.Now().Unix())
	session := &Session{
		ID:           id,
		StateHandler: NewMessageStateHandler(id),
		FileTracker:  context_manager.NewFileTracker(),
		CreatedAt:    time.Now(),
	}

	m.sessions[id] = session
	m.saveLocked(session)
	return session
}

func (m *SessionManager) GetSession(id string) *Session {
	m.mu.RLock()
	session, ok := m.sessions[id]
	m.mu.RUnlock()

	if ok {
		return session
	}

	// Default session
	if id == "default" {
		return m.CreateSession()
	}

	return nil
}

func (m *SessionManager) ListSessions() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []*Session
	for _, s := range m.sessions {
		list = append(list, s)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})

	return list
}

func (m *SessionManager) Save(id string) error {
	if m.storageDir == "" {
		return nil
	}

	m.mu.RLock()
	session, ok := m.sessions[id]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	return m.saveLocked(session)
}

// saveLocked saves a session to disk. It assumes the caller holds the lock (read or write).
func (m *SessionManager) saveLocked(session *Session) error {
	if m.storageDir == "" {
		return nil
	}

	data := SessionData{
		ID:        session.ID,
		Messages:  session.StateHandler.GetMessages(),
		Todos:     session.Todos,
		CreatedAt: session.CreatedAt,
	}

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(m.storageDir, session.ID+".json")
	return os.WriteFile(path, bytes, 0644)
}

func (m *SessionManager) LoadAll() {
	if m.storageDir == "" {
		return
	}

	files, err := os.ReadDir(m.storageDir)
	if err != nil {
		return
	}

	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			data, err := os.ReadFile(filepath.Join(m.storageDir, f.Name()))
			if err != nil {
				continue
			}

			var sd SessionData
			if err := json.Unmarshal(data, &sd); err != nil {
				continue
			}

			session := &Session{
				ID:           sd.ID,
				StateHandler: NewMessageStateHandler(sd.ID),
				FileTracker:  context_manager.NewFileTracker(),
				Todos:        sd.Todos,
				CreatedAt:    sd.CreatedAt,
			}
			session.StateHandler.SetMessages(sd.Messages)

			m.mu.Lock()
			m.sessions[sd.ID] = session
			m.mu.Unlock()
		}
	}
}

func (m *SessionManager) DeleteSession(id string) error {
	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()

	if m.storageDir != "" {
		path := filepath.Join(m.storageDir, id+".json")
		os.Remove(path)
	}
	return nil
}
