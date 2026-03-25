package pty

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_SpawnAndRead(t *testing.T) {
	m := NewManager()
	defer m.Close()

	id, err := m.Spawn("echo", []string{"hello"}, SpawnOptions{})
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	time.Sleep(200 * time.Millisecond)

	sess := m.Get(id)
	require.NotNil(t, sess)
	assert.True(t, sess.Closed())
}

func TestManager_SpawnShellAndWrite(t *testing.T) {
	m := NewManager()
	defer m.Close()

	id, err := m.Spawn("/bin/sh", []string{}, SpawnOptions{
		Dir: "/tmp",
	})
	require.NoError(t, err)

	sess := m.Get(id)
	require.NotNil(t, sess)

	_, err = sess.Write([]byte("echo test123\n"))
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)
	assert.False(t, sess.Closed())

	err = m.Kill(id)
	require.NoError(t, err)
	assert.Nil(t, m.Get(id))
}

func TestManager_Resize(t *testing.T) {
	m := NewManager()
	defer m.Close()

	id, err := m.Spawn("/bin/sh", []string{}, SpawnOptions{
		Rows: 24,
		Cols: 80,
	})
	require.NoError(t, err)

	err = m.Resize(id, 48, 120)
	require.NoError(t, err)

	m.Kill(id)
}

func TestManager_MonitorIntegration(t *testing.T) {
	m := NewManager()
	defer m.Close()

	id, err := m.Spawn("/bin/sh", []string{"-c", "echo monitored-output; sleep 2"}, SpawnOptions{})
	require.NoError(t, err)

	time.Sleep(300 * time.Millisecond)

	content := m.GetContent(id)
	assert.Contains(t, content, "monitored-output")

	updated, _ := m.HasUpdated(id)
	assert.True(t, updated)

	updated, _ = m.HasUpdated(id)
	assert.False(t, updated)
}

func TestManager_KillNonexistent(t *testing.T) {
	m := NewManager()
	defer m.Close()

	err := m.Kill("nonexistent")
	assert.Error(t, err)
}
