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
- `Spawn(program, args, opts)` — opens SSH session, requests PTY, starts command. Returns session ID. Creates a `pty.Session`-compatible wrapper (see WebSocket integration below) with its own `Monitor` and subscriber support. SSH channel stdout feeds into the Monitor and broadcasts to subscribers.
- `Kill(id)` — sends SIGTERM via SSH session signal, closes channel
- `Resize(id, rows, cols)` — sends window-change request over SSH channel
- `Write(id, data)` — writes to SSH session stdin
- `GetContent(id)`, `HasUpdated(id)`, `CheckTrustPrompt(id)`, `WaitExit(id, timeout)` — all delegate to the Monitor

### WebSocket Integration: Session Registry

The current `WebSocketServer` takes a concrete `*pty.Manager` and calls `manager.Get(sessionID)` to find sessions. SSH sessions are invisible to it. To fix this, introduce a `SessionRegistry` interface:

```go
// In pty/registry.go
type StreamableSession interface {
    Write(p []byte) (int, error)
    Subscribe() *subscriber
    Unsubscribe(sub *subscriber)
    Closed() bool
    GetSnapshot() []byte  // current monitor buffer for replay on connect
}

type SessionRegistry interface {
    Get(id string) StreamableSession
}
```

- `pty.Manager` already satisfies this (its `Session` struct has all these methods; add `GetSnapshot()` delegating to `monitor.Content()`)
- `SSHProcessManager` creates wrapper sessions that satisfy `StreamableSession` — wrapping the SSH channel's stdin for Write, and using a local Monitor + subscriber list for Subscribe/broadcast
- A `CompositeRegistry` combines both: tries `pty.Manager` first, then `SSHProcessManager`. The WebSocket server takes a `SessionRegistry` instead of `*Manager`.

This is the core data path for terminal rendering — without it, remote terminals display nothing.

### Git Operations: Command Executor Abstraction

New interface in `session/git`:

```go
type CommandExecutor interface {
    // Run executes a command in the given directory and returns combined output.
    // Returns the output and an error that preserves exit code semantics
    // (wraps exec.ExitError for local, equivalent for remote).
    Run(dir string, name string, args ...string) ([]byte, error)
}
```

- `LocalExecutor` — wraps `exec.Command` with `cmd.Dir` set (current behavior, default)
- `RemoteExecutor` — wraps SSH client. Constructs `cd <dir> && <name> <args...>` with proper shell escaping (using `shellescape` or manual quoting for paths with spaces/special chars). Preserves exit code by inspecting `ssh.ExitError`. Returns combined stdout+stderr to match `CombinedOutput()` semantics.

`GitWorktree` gains an `executor` field (defaults to `LocalExecutor` for backward compatibility). All internal `exec.Command` calls in `worktree_ops.go`, `worktree_git.go`, and `util.go` refactored to go through `executor.Run()`.

**Standalone git functions** (`GetCurrentBranch`, `GetDefaultBranch`, `FetchBranches`, `SearchBranches`) are called outside of `GitWorktree` (from `Instance.Start()` and `SessionAPI.GetDirInfo()`). For remote sessions, these are handled by the separate API methods `GetRemoteDirInfo` and `SearchRemoteBranches`, which use the SSH client's `RunCommand` directly rather than the executor abstraction. The `Instance.Start()` code path branches: if `HostID` is set, it uses the SSH client to run the equivalent git commands remotely rather than calling the standalone functions.

Remote worktrees use `~/.claude-squad/worktrees/` on the remote machine (same convention as local).

### Instance & API Changes

**Instance additions:**
- `HostID string` — empty = localhost, otherwise references a host UUID. Serialized as `host_id` in `InstanceData` JSON.
- `remote bool` — derived from HostID being non-empty

**InstanceData additions:**
- `HostID string` field, persisted in `state.json`

**CreateOptions additions:**
- `HostID string` field

**SessionAPI additions:**
- `hostManager *ssh.HostManager` — loads hosts.json, manages active SSH connections (one per host, shared across sessions)
- `CreateSession()` — if HostID set, looks up host, gets/creates SSH connection, creates `SSHProcessManager`, passes as `ProcessManager`
- `ResumeSession()` — re-establishes SSH if needed before spawning remote process

**Startup/restore flow for remote sessions:**
In `NewSessionAPI`, when loading persisted sessions from `state.json`:
- All sessions (local and remote) are loaded as Paused, same as today
- Remote sessions (`HostID != ""`) are restored with a nil ProcessManager initially — they are paused so no process is needed yet
- When the user opens/resumes a remote session, `SessionAPI` checks `HostID`, establishes the SSH connection via `hostManager`, creates an `SSHProcessManager`, calls `inst.SetProcessManager(pm)`, then proceeds with normal resume flow
- If SSH connection fails at resume time, return error to user, session stays paused

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
- Reconnect is coordinated by `HostManager`, not individual sessions. When a shared connection drops, `HostManager` runs a single reconnect loop. On success, it notifies all active sessions on that host.
- For each notified session: `SessionAPI` spawns a goroutine that acquires `api.mu`, calls `inst.SetProcessManager(newPM)`, then calls `inst.spawnProcess(workDir, true)` to resume. The remote process is assumed dead (SSH session teardown sends SIGHUP). If the worktree is gone, recreate it. If `--resume` fails (conversation not found), start fresh — same logic as local resume.
- Overlay clears automatically when next poll shows connected
- Thread safety: the shared `*ssh.Client` is safe for concurrent use (opening multiple channels). The `HostManager` serializes reconnect attempts with a mutex. Individual `SSHProcessManager` instances hold their own session/channel state.

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

**Host key verification:** Use `golang.org/x/crypto/ssh/knownhosts` to verify against `~/.ssh/known_hosts`. If the host key is unknown, the Test button shows the fingerprint and asks the user to confirm. On confirmation, the key is appended to `known_hosts`. Reject connections with changed host keys (show error with explanation).

**Platform:** macOS only for now. Keychain integration uses macOS Keychain. If cross-platform support is added later, the secrets storage should be extracted behind an interface with platform-specific implementations.

**Shared connection lifecycle:** When all sessions on a host are paused or killed, the SSH connection is closed after a 30-second idle timeout. If a new session is created or resumed on that host, a fresh connection is established.

**Directory validation for remote paths:** `GetRemoteDirInfo` and `SearchRemoteBranches` gracefully return empty results if the remote directory does not exist or the SSH connection drops mid-request — same behavior as the local `GetDirInfo` error handling.
