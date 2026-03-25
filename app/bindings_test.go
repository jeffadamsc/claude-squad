package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionAPI_CreateAndLoad(t *testing.T) {
	api := newTestAPI(t)

	info, err := api.CreateSession(CreateOptions{
		Title:   "test-session",
		Path:    "/tmp",
		Program: "echo hello",
	})
	require.NoError(t, err)
	assert.Equal(t, "test-session", info.Title)
	assert.NotEmpty(t, info.ID)

	sessions, err := api.LoadSessions()
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
	assert.Equal(t, "test-session", sessions[0].Title)
}

func TestSessionAPI_GetWebSocketPort(t *testing.T) {
	api := newTestAPI(t)
	port := api.GetWebSocketPort()
	assert.Greater(t, port, 0)
}

func newTestAPI(t *testing.T) *SessionAPI {
	t.Helper()
	api, err := NewSessionAPI(SessionAPIOptions{
		DataDir: t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { api.Close() })
	return api
}
