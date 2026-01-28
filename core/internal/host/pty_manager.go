package host

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/google/uuid"
)

// SimpleBuffer is a thread-safe byte buffer
type SimpleBuffer struct {
	buffer []byte
	mu     sync.RWMutex
}

func (b *SimpleBuffer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buffer = append(b.buffer, p...)
	// Truncate if too large to prevent memory leak (e.g. 1MB)
	if len(b.buffer) > 1024*1024 {
		cut := len(b.buffer) - 1024*1024
		b.buffer = b.buffer[cut:]
	}
	return len(p), nil
}

func (b *SimpleBuffer) String() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return string(b.buffer)
}

// PTYSession represents an active pseudo-terminal
type PTYSession struct {
	ID        string
	Cmd       *exec.Cmd
	PTY       *os.File // The pseudo-terminal file
	Output    *SimpleBuffer
	CreatedAt time.Time
	Running   bool
	mu        sync.Mutex
}

// PTYManager manages multiple PTY sessions
type PTYManager struct {
	sessions map[string]*PTYSession
	mu       sync.RWMutex
}

func NewPTYManager() *PTYManager {
	return &PTYManager{
		sessions: make(map[string]*PTYSession),
	}
}

func (m *PTYManager) Start(command string, args []string, cwd string, env []string) (*PTYSession, error) {
	// Create command
	c := exec.Command(command, args...)
	c.Dir = cwd
	if len(env) > 0 {
		c.Env = append(os.Environ(), env...)
	}

	// Start PTY
	ptmx, err := pty.Start(c)
	if err != nil {
		return nil, fmt.Errorf("failed to start pty: %w", err)
	}

	id := uuid.New().String()
	outputBuf := &SimpleBuffer{}

	session := &PTYSession{
		ID:        id,
		Cmd:       c,
		PTY:       ptmx,
		Output:    outputBuf,
		CreatedAt: time.Now(),
		Running:   true,
	}

	// Copy PTY output to buffer
	go func() {
		defer func() {
			session.mu.Lock()
			session.Running = false
			session.mu.Unlock()
			// Notify exit?
		}()
		_, _ = io.Copy(outputBuf, ptmx)
	}()

	m.mu.Lock()
	m.sessions[id] = session
	m.mu.Unlock()

	return session, nil
}

// GetSession retrieves a running session
func (m *PTYManager) GetSession(id string) *PTYSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[id]
}

// WriteInput sends data to the PTY
func (m *PTYManager) WriteInput(id string, data string) error {
	session := m.GetSession(id)
	if session == nil {
		return fmt.Errorf("session not found: %s", id)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if !session.Running {
		return fmt.Errorf("session is not running")
	}

	_, err := session.PTY.WriteString(data)
	return err
}

// ReadOutput retrieves buffer content (full buffer for now)
func (m *PTYManager) ReadOutput(id string) (string, error) {
	session := m.GetSession(id)
	if session == nil {
		return "", fmt.Errorf("session not found: %s", id)
	}
	return session.Output.String(), nil
}

// Close terminates the PTY session
func (m *PTYManager) Close(id string) error {
	session := m.GetSession(id)
	if session == nil {
		return nil
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.Running {
		// Kill process
		if session.Cmd.Process != nil {
			_ = session.Cmd.Process.Kill()
		}
		// Close PTY
		_ = session.PTY.Close()
		session.Running = false
	}

	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()

	return nil
}
