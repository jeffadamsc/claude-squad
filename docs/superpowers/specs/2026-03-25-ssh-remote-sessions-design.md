# SSH Remote Sessions

Run claude-squad sessions on remote machines via SSH, with persistent host configs, connection monitoring, and auto-reconnect.

## Requirements

- New host dropdown in session creation dialog, defaulting to "localhost"
- Add Host dialog with: name, host, port (default 22), user, auth method (password or private key with optional passphrase), file browser for key selection (shows hidden files), test button (verifies SSH + program availability), cancel/OK
- Saved hosts persist in `~/.claude-squad/hosts.json`; secrets stored in macOS Keychain
- Remote sessions support all the same options as local (in-place, git worktree, branch selection, prompt, program choice)
- Program must already be installed on the remote machine; error if not found
- SSH connection monitoring with disconnect overlay on terminal, loading indicator in sidebar, and automatic reconnect with resume

## Architecture

### Approach: Pure Go SSH via `golang.org/x/crypto/ssh`

No dependency on the local `ssh` binary. Full programmatic control over connection lifecycle, disconnect detection, and reconnect logic. The library is already an indirect dependency in go.mod.

### New Package: `ssh/`

Located at project root alongside `pty/`, `session/`, etc.

**`ssh/hosts.go`** — Host configuration CRUD:
- Load/save `~/.claude-squad/hosts.json`
- Each host entry: `id` (UUID), `name`, `host`, `port`, `user`, `authMethod` ("password" | "key" | "key+passphrase"), `keyPath`
- No secrets in the JSON file

**`ssh/keychain.go`** — macOS Keychain integration:
- Store/retrieve/delete passwords and key passphrases
- Keyed by host UUID
- Used by the host manager when establishing connections

**`ssh/client.go`** — SSH connection wrapper:
- `Connect(hostConfig)` — establishes connection using password or key auth
- `TestConnection(hostConfig, program)` — connects, runs `which <program>`, returns structured result
- `Connected() bool`, `LastError() error` — connection state
- `RunCommand(cmd string) (string, error)` — execute a command on the remote host, return stdout
- `Close()` — tear down connection
- Background keepalive goroutine: sends `keepalive@openssh.com` every 5 seconds, marks disconnected on failure
- `OnDisconnect(func())` callback for triggering reconnect logic
- Resolves remote `$HOME` on first connect via `echo $HOME`, caches it

**`ssh/process_manager.go`** — implements `session.ProcessManager` over SSH:
- `Spawn(program, args, opts)` — opens SSH session, requests PTY, starts command. Returns session ID. SSH channel stdout feeds into a local `Monitor` (same 64KB ring buffer) and broadcasts to WebSocket subscribers.
- `Kill(id)` — sends SIGTERM via SSH session signal, closes channel
- `Resize(id, rows, cols)` — sends window-change request over SSH channel
- `Write(id, data)` — writes to SSH session stdin
- `GetContent(id)`, `HasUpdated(id)`, `CheckTrustPrompt(id)`, `WaitExit(id, timeout)` — all delegate to the same `Monitor` used by local sessions

### Git Operations: Command Executor Abstraction

New interface in `session/git`:

```go
type CommandExecutor interface {
    Run(dir string, name string, args ...string) ([]byte, error)
}
```

- `LocalExecutor` — wraps `exec.Command` (current behavior, default)
- `RemoteExecutor` — wraps SSH client's `RunCommand`

`GitWorktree` gains an `executor` field (defaults to `LocalExecutor` for backward compatibility). All internal `exec.Command` calls refactored to go through `executor.Run()`.

Remote worktrees use `~/.claude-squad/worktrees/` on the remote machine (same convention as local).

### Instance & API Changes

**Instance additions:**
- `HostID string` — empty = localhost, otherwise references a host UUID
- `remote bool` — derived from HostID
- Serialized in `InstanceData` for persistence

**CreateOptions additions:**
- `HostID string` field

**SessionAPI additions:**
- `hostManager *ssh.HostManager` — loads hosts.json, manages active SSH connections (one per host, shared across sessions)
- `CreateSession()` — if HostID set, looks up host, gets/creates SSH connection, creates `SSHProcessManager`, passes as `ProcessManager`
- `ResumeSession()` — re-establishes SSH if needed before spawning remote process

**New API methods:**
- `GetHosts() []HostInfo`
- `CreateHost(config) (HostInfo, error)`
- `UpdateHost(config) error`
- `DeleteHost(id) error`
- `TestHost(config, program) TestResult` — tests SSH + program availability
- `GetRemoteDirInfo(hostId, dir) DirInfo`
- `SearchRemoteBranches(hostId, dir, filter) []string`
- `SelectFile(startDir) string` — opens native file dialog (Wails `runtime.OpenFileDialog`)

### Frontend Changes

**NewSessionDialog:**
- New "Host" dropdown after Title, before Directory — lists "localhost" + all saved hosts
- "+" button next to dropdown opens AddHostDialog
- When remote host selected, Directory refers to remote path; branch loading uses `GetRemoteDirInfo` / `SearchRemoteBranches`

**AddHostDialog (new component):**
- Fields: Name, Host, Port (default 22), User
- Auth method radio: Password (password field) or Private Key (file path + Browse button + optional passphrase)
- Browse button opens native file dialog starting in `~/.ssh/`, showing hidden files
- Test button calls `TestHost()`, shows inline result (green check or red X with message)
- OK disabled until test passes at least once
- Cancel / OK buttons

**Zustand store:**
- `hosts: HostInfo[]` — loaded on startup via `GetHosts()`
- `addHost(host)`, `removeHost(id)` actions

### Connection Monitoring & Recovery

**Polling:**
- `SessionStatus` gains `SSHConnected *bool` — nil for local, true/false for remote
- `PollAllStatuses()` checks SSH connection state for remote sessions
- Frontend 500ms poller picks this up automatically

**Disconnect overlay (`TerminalPane.tsx`):**
- When `sshConnected === false`, semi-transparent overlay with "Connection lost - reconnecting..." and spinner
- Terminal stays visible but frozen underneath (last buffer contents)

**Sidebar indicator (`Sidebar.tsx`):**
- Remote sessions with `sshConnected === false` show pulsing/loading indicator

**Auto-reconnect (`ssh/client.go`):**
- On disconnect: retry every 3 seconds, exponential backoff capping at 30 seconds
- On reconnect: SSHProcessManager gets new SSH client, Instance re-spawns process with resume logic (check if branch/worktree exists, recreate if needed)
- Overlay clears automatically when next poll shows connected

**Tab behavior during disconnect:**
- User can switch tabs, close session, or pause while disconnected
- Pausing a disconnected session marks it paused locally; remote cleanup happens on next connection

### Edge Cases

**Startup with stale remote sessions:** Loaded as Paused. Opening one establishes SSH, then resumes. SSH failure = error, session stays paused.

**Host deletion with active sessions:** Confirmation warning listing affected sessions. Deleting pauses all sessions on that host first.

**Multiple sessions on same host:** Share one SSH connection via HostManager map. Each process gets its own SSH session/channel.

**Key file permissions:** Warn if private key file permissions are too open (not 0600/0400) in test result.

**Connection timeout:** 10 second SSH dial timeout. Test dialog shows "Connection timed out."

**Remote home directory:** Resolved via `echo $HOME` on first connect, cached for connection lifetime.

**Program not on PATH:** Caught by test button. If program disappears after session creation, spawn failure surfaces as normal start error.
