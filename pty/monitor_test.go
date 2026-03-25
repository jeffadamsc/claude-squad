package pty

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMonitor_WriteAndRead(t *testing.T) {
	m := NewMonitor(1024)

	m.Write([]byte("hello world\n"))
	content := m.Content()
	assert.Contains(t, content, "hello world")
}

func TestMonitor_RingBufferOverflow(t *testing.T) {
	m := NewMonitor(16)

	m.Write([]byte("abcdefghijklmnop")) // fills buffer
	m.Write([]byte("QRST"))             // overwrites start

	content := m.Content()
	assert.NotContains(t, content, "abcd")
	assert.Contains(t, content, "QRST")
}

func TestMonitor_HasUpdated(t *testing.T) {
	m := NewMonitor(1024)

	updated, hasPrompt := m.HasUpdated()
	assert.False(t, updated)
	assert.False(t, hasPrompt)

	m.Write([]byte("some output\n"))
	updated, hasPrompt = m.HasUpdated()
	assert.True(t, updated)
	assert.False(t, hasPrompt)

	updated, hasPrompt = m.HasUpdated()
	assert.False(t, updated)
}

func TestMonitor_DetectsClaudePrompt(t *testing.T) {
	m := NewMonitor(1024)

	m.Write([]byte("Working on it...\n"))
	m.HasUpdated() // clear

	m.Write([]byte("Do you want to proceed? (y/n)\n"))
	updated, hasPrompt := m.HasUpdated()
	assert.True(t, updated)
	assert.True(t, hasPrompt)
}

func TestMonitor_DetectsTrustPrompt(t *testing.T) {
	m := NewMonitor(1024)

	m.Write([]byte("Do you trust the files in this folder?\n"))
	assert.True(t, m.CheckTrustPrompt())
}
