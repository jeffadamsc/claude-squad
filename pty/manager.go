package pty

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

type SpawnOptions struct {
	Dir  string
	Env  []string
	Rows uint16
	Cols uint16
}

type Session struct {
	id      string
	ptmx    *os.File
	cmd     *exec.Cmd
	mu      sync.Mutex
	closed  bool
	monitor *Monitor
	pipeR   *io.PipeReader
	pipeW   *io.PipeWriter
}

func (s *Session) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, io.ErrClosedPipe
	}
	return s.ptmx.Write(p)
}

func (s *Session) Read(p []byte) (int, error) {
	return s.pipeR.Read(p)
}

func (s *Session) Closed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *Session) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	s.pipeW.Close()
	s.ptmx.Close()
	s.cmd.Process.Kill()
	s.cmd.Wait()
}

type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	counter  int
}

func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
	}
}

func (m *Manager) Spawn(program string, args []string, opts SpawnOptions) (string, error) {
	cmd := exec.Command(program, args...)
	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	if opts.Env != nil {
		cmd.Env = append(cmd.Env, opts.Env...)
	}

	rows, cols := opts.Rows, opts.Cols
	if rows == 0 {
		rows = 24
	}
	if cols == 0 {
		cols = 80
	}

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
	if err != nil {
		return "", fmt.Errorf("pty start: %w", err)
	}

	pipeR, pipeW := io.Pipe()
	monitor := NewMonitor(64 * 1024)

	m.mu.Lock()
	m.counter++
	id := fmt.Sprintf("session-%d", m.counter)
	sess := &Session{
		id:      id,
		ptmx:    ptmx,
		cmd:     cmd,
		monitor: monitor,
		pipeR:   pipeR,
		pipeW:   pipeW,
	}
	m.sessions[id] = sess
	m.mu.Unlock()

	// Tee PTY output to both monitor and pipe (for WebSocket consumers).
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				sess.monitor.Write(buf[:n])
				sess.pipeW.Write(buf[:n])
			}
			if err != nil {
				sess.pipeW.CloseWithError(err)
				return
			}
		}
	}()

	go func() {
		cmd.Wait()
		sess.mu.Lock()
		sess.closed = true
		sess.mu.Unlock()
	}()

	return id, nil
}

func (m *Manager) Get(id string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[id]
}

func (m *Manager) Kill(id string) error {
	m.mu.Lock()
	sess, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("session %s not found", id)
	}
	delete(m.sessions, id)
	m.mu.Unlock()

	sess.close()
	return nil
}

func (m *Manager) Resize(id string, rows, cols uint16) error {
	m.mu.RLock()
	sess, ok := m.sessions[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("session %s not found", id)
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()
	if sess.closed {
		return fmt.Errorf("session %s is closed", id)
	}

	return pty.Setsize(sess.ptmx, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
}

func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, sess := range m.sessions {
		sess.close()
		delete(m.sessions, id)
	}
}

func (m *Manager) GetContent(id string) string {
	m.mu.RLock()
	sess, ok := m.sessions[id]
	m.mu.RUnlock()
	if !ok {
		return ""
	}
	return sess.monitor.Content()
}

func (m *Manager) HasUpdated(id string) (bool, bool) {
	m.mu.RLock()
	sess, ok := m.sessions[id]
	m.mu.RUnlock()
	if !ok {
		return false, false
	}
	return sess.monitor.HasUpdated()
}

func (m *Manager) CheckTrustPrompt(id string) bool {
	m.mu.RLock()
	sess, ok := m.sessions[id]
	m.mu.RUnlock()
	if !ok {
		return false
	}
	return sess.monitor.CheckTrustPrompt()
}

func (m *Manager) Write(id string, data []byte) error {
	m.mu.RLock()
	sess, ok := m.sessions[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("session %s not found", id)
	}
	_, err := sess.Write(data)
	return err
}
