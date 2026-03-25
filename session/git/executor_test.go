package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalExecutor_Run(t *testing.T) {
	exec := &LocalExecutor{}
	out, err := exec.Run("", "echo", "hello")
	require.NoError(t, err)
	assert.Contains(t, string(out), "hello")
}

func TestLocalExecutor_RunWithDir(t *testing.T) {
	dir := t.TempDir()
	exec := &LocalExecutor{}
	out, err := exec.Run(dir, "pwd")
	require.NoError(t, err)
	assert.Contains(t, string(out), dir)
}

func TestLocalExecutor_RunFailure(t *testing.T) {
	exec := &LocalExecutor{}
	_, err := exec.Run("", "false")
	assert.Error(t, err)
}

func TestRemoteExecutor_BuildCommand(t *testing.T) {
	cmd := buildRemoteCommand("/some/path", "git", "status", "--short")
	assert.Equal(t, "cd '/some/path' && git status --short", cmd)
}

func TestRemoteExecutor_BuildCommandPathEscaping(t *testing.T) {
	cmd := buildRemoteCommand("/path with spaces/repo", "git", "log")
	assert.Equal(t, "cd '/path with spaces/repo' && git log", cmd)
}
