package pty

import (
	"crypto/sha256"
	"strings"
	"sync"
)

var promptPatterns = []string{
	"(y/n)",
	"(Y/N)",
	"[Y/n]",
	"[y/N]",
	"? (y)",
	"? (n)",
	"Do you want to proceed",
	"Would you like to",
	"Press Enter",
	"press enter",
}

var trustPatterns = []string{
	"Do you trust the files in this folder",
	"Trust this project",
	"trust the authors",
}

type Monitor struct {
	mu       sync.Mutex
	buf      []byte
	size     int
	writePos int
	used     int
	lastHash [sha256.Size]byte
}

func NewMonitor(size int) *Monitor {
	return &Monitor{
		buf:  make([]byte, size),
		size: size,
	}
}

func (m *Monitor) Write(p []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, b := range p {
		m.buf[m.writePos] = b
		m.writePos = (m.writePos + 1) % m.size
		if m.used < m.size {
			m.used++
		}
	}
}

func (m *Monitor) Content() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.contentLocked()
}

func (m *Monitor) contentLocked() string {
	if m.used == 0 {
		return ""
	}
	if m.used < m.size {
		return string(m.buf[:m.used])
	}
	result := make([]byte, m.size)
	start := m.writePos
	copy(result, m.buf[start:])
	copy(result[m.size-start:], m.buf[:start])
	return string(result)
}

func (m *Monitor) HasUpdated() (updated bool, hasPrompt bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.used == 0 {
		return false, false
	}

	hash := sha256.Sum256(m.buf[:m.used])
	if hash == m.lastHash {
		return false, false
	}
	m.lastHash = hash

	content := m.contentLocked()
	tail := content
	if len(tail) > 500 {
		tail = tail[len(tail)-500:]
	}

	for _, p := range promptPatterns {
		if strings.Contains(tail, p) {
			return true, true
		}
	}

	return true, false
}

func (m *Monitor) CheckTrustPrompt() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	content := m.contentLocked()
	tail := content
	if len(tail) > 500 {
		tail = tail[len(tail)-500:]
	}

	for _, p := range trustPatterns {
		if strings.Contains(tail, p) {
			return true
		}
	}
	return false
}
