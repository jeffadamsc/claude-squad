package ssh

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHostStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewHostStore(filepath.Join(dir, "hosts.json"))

	host := HostConfig{
		ID:         "test-id-1",
		Name:       "dev-server",
		Host:       "192.168.1.50",
		Port:       22,
		User:       "deploy",
		AuthMethod: AuthMethodKey,
		KeyPath:    "/home/user/.ssh/id_ed25519",
	}

	err := store.Save(host)
	require.NoError(t, err)

	hosts, err := store.LoadAll()
	require.NoError(t, err)
	require.Len(t, hosts, 1)
	assert.Equal(t, host, hosts[0])
}

func TestHostStore_Delete(t *testing.T) {
	dir := t.TempDir()
	store := NewHostStore(filepath.Join(dir, "hosts.json"))

	host := HostConfig{ID: "del-1", Name: "server1", Host: "10.0.0.1", Port: 22, User: "root", AuthMethod: AuthMethodPassword}
	require.NoError(t, store.Save(host))

	err := store.Delete("del-1")
	require.NoError(t, err)

	hosts, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, hosts, 0)
}

func TestHostStore_Update(t *testing.T) {
	dir := t.TempDir()
	store := NewHostStore(filepath.Join(dir, "hosts.json"))

	host := HostConfig{ID: "upd-1", Name: "old-name", Host: "10.0.0.1", Port: 22, User: "root", AuthMethod: AuthMethodPassword}
	require.NoError(t, store.Save(host))

	host.Name = "new-name"
	host.Port = 2222
	err := store.Update(host)
	require.NoError(t, err)

	hosts, err := store.LoadAll()
	require.NoError(t, err)
	require.Len(t, hosts, 1)
	assert.Equal(t, "new-name", hosts[0].Name)
	assert.Equal(t, 2222, hosts[0].Port)
}

func TestHostStore_LoadEmpty(t *testing.T) {
	dir := t.TempDir()
	store := NewHostStore(filepath.Join(dir, "hosts.json"))

	hosts, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, hosts, 0)
}

func TestHostStore_GetByID(t *testing.T) {
	dir := t.TempDir()
	store := NewHostStore(filepath.Join(dir, "hosts.json"))

	h1 := HostConfig{ID: "a", Name: "server-a", Host: "1.1.1.1", Port: 22, User: "u", AuthMethod: AuthMethodPassword}
	h2 := HostConfig{ID: "b", Name: "server-b", Host: "2.2.2.2", Port: 22, User: "u", AuthMethod: AuthMethodKey}
	require.NoError(t, store.Save(h1))
	require.NoError(t, store.Save(h2))

	found, err := store.GetByID("b")
	require.NoError(t, err)
	assert.Equal(t, "server-b", found.Name)

	_, err = store.GetByID("nonexistent")
	assert.Error(t, err)
}
