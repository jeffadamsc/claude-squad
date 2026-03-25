# Wails + xterm.js Port Design

Port the Claude Squad IDE from Fyne (native Go GUI) to Wails (Go backend + React frontend) with xterm.js for terminal rendering, replacing the fyne-terminal-fork and tmux dependency.

## Decisions

- **GUI Framework:** Wails v2 with React (Vite SPA)
- **Terminal:** xterm.js (replaces fyne-terminal-fork)
- **PTY management:** Direct via `creack/pty` (replaces tmux)
- **Terminal I/O:** WebSocket bridge (one per session)
- **Layout:** Tab-based with optional splits (replaces binary tree of splits)
- **Scope:** Full feature parity with current Fyne app
- **Theme:** Catppuccin Mocha (same palette, ported to CSS)

## Architecture

Two communication channels between frontend and backend:

### Wails Bindings (JSON-RPC)

Request/response calls for session management operations:

- Session CRUD (create, load, delete)
- Lifecycle control (start, pause, resume, kill)
- Actions (push changes, send prompt)
- Queries (status polling, config, branches, diff stats)

### WebSocket (binary, per session)

High-throughput terminal I/O:

- Endpoint: `ws://localhost:{port}/ws/{sessionID}`
- PTY stdout → Go read loop → WebSocket → xterm.js
- xterm.js keystrokes → WebSocket → Go → PTY stdin write
- Resize events from xterm.js → Go → `pty.Setsize()`
- Uses xterm.js `AttachAddon` for native WebSocket binding

## PTY Manager

New component replacing the tmux dependency.

### Responsibilities

- Spawn processes (claude, aider, shell) attached to a PTY via `creack/pty`
- Run a local WebSocket server on a random port (localhost only)
- Bridge PTY I/O to/from WebSocket connections
- Handle PTY resize when xterm.js reports size changes
- Buffer recent output and scan for prompt patterns (replaces `tmux capture-pane`)
- Clean up PTY on session kill

### Lifecycle

```
CreateSession → PTY Manager spawns process with pty.Start()
              → Stores PTY file descriptor keyed by sessionID
              → Returns sessionID to frontend

OpenTab       → Frontend connects WebSocket to /ws/{sessionID}
              → xterm.js AttachAddon binds to WebSocket
              → Bidirectional I/O flows

CloseTab      → Frontend disconnects WebSocket
              → PTY stays alive (process still running)

KillSession   → Go closes PTY fd → process receives SIGHUP → cleanup
```

### Pause/Resume

Since tmux is removed, pause/resume works as follows:

- **Pause:** Commit changes, kill PTY (process exits), remove worktree. This matches current behavior where pause already commits + removes worktree.
- **Resume:** Recreate worktree from saved branch, spawn new PTY with fresh process.

### Status Monitoring

The Go read loop buffers recent output and scans for prompt patterns (e.g., "Claude >") directly, replacing `tmux capture-pane` polling. Same detection logic, different data source.

## Wails Bindings API

### SessionAPI

```go
// CRUD
CreateSession(opts SessionOptions) (*SessionInfo, error)
LoadSessions() ([]*SessionInfo, error)
DeleteSession(id string) error

// Lifecycle
StartSession(id string) error
PauseSession(id string) error
ResumeSession(id string) error
KillSession(id string) error

// Actions
PushSession(id string, createPR bool) error
SendPrompt(id string, prompt string) error

// Queries
GetSessionStatus(id string) (*SessionStatus, error)
GetWebSocketPort() int
PollAllStatuses() ([]*SessionStatus, error)

// Config
GetConfig() (*Config, error)
GetProfiles() ([]Profile, error)
ListBranches(repoPath string) ([]string, error)
```

### Data Types

```typescript
SessionInfo {
  id: string
  title: string
  path: string
  branch: string
  program: string
  status: "running" | "ready" | "loading" | "paused"
}

SessionStatus {
  id: string
  status: "running" | "ready" | "loading" | "paused"
  branch: string
  diffStats: { added: number, removed: number }
  hasPrompt: boolean
}
```

The frontend calls `PollAllStatuses()` on a 500ms interval to update sidebar indicators and detect prompts.

## UI Layout

### Window Zones

1. **Sidebar** (left, collapsible via Ctrl+Shift+B) — Session list with status indicators (green dot = running, yellow = ready, pause icon = paused), branch names, diff stats. "New Session" button at bottom. Right-click context menu.

2. **Tab Bar** (top of main area) — One tab per open session with status dot and close button. Split button on right edge.

3. **Terminal Area** (center) — xterm.js instance. When split, multiple xterm.js instances with draggable dividers. Focused pane gets blue border highlight.

4. **Status Bar** (bottom) — Active session name, branch, status on left. Keyboard shortcut hints on right.

### Tab + Split Model

Sessions open in tabs by default. Within a tab, users can split horizontally or vertically to view multiple terminals. This replaces the current pure binary-tree-of-splits model.

## Frontend Component Structure

```
src/
├── App.tsx                    # Root: Wails runtime init, global hotkeys
├── components/
│   ├── Sidebar/
│   │   ├── Sidebar.tsx        # Collapsible sidebar container
│   │   ├── SessionItem.tsx    # Session row (status, title, branch, diff)
│   │   └── ContextMenu.tsx    # Right-click menu (open, pause, delete)
│   ├── TabBar/
│   │   ├── TabBar.tsx         # Tab strip with split button
│   │   └── Tab.tsx            # Single tab (title, status dot, close)
│   ├── Terminal/
│   │   ├── TerminalPane.tsx   # xterm.js instance + WebSocket attach
│   │   ├── SplitContainer.tsx # Draggable split (allotment lib)
│   │   └── PaneManager.tsx    # Manages tabs → pane trees
│   ├── Dialogs/
│   │   ├── NewSessionDialog.tsx
│   │   └── ConfirmDialog.tsx
│   └── StatusBar.tsx          # Bottom bar: session info + hotkey hints
├── hooks/
│   ├── useSessionPoller.ts    # 500ms PollAllStatuses() loop
│   ├── useHotkeys.ts          # Global keyboard shortcuts
│   └── useTerminal.ts         # xterm.js lifecycle + WebSocket management
├── store/
│   └── sessionStore.ts        # Session state (zustand or context)
├── lib/
│   ├── wails.ts               # Typed wrappers around Wails bindings
│   └── theme.ts               # Catppuccin Mocha color tokens
└── main.tsx                   # Entry point
```

### Key Libraries

- **xterm.js** + `@xterm/addon-attach` + `@xterm/addon-fit` — terminal rendering, WebSocket attach, auto-resize
- **allotment** — draggable split panes
- **zustand** — lightweight state management

### Hotkeys

Same Ctrl+Shift modifier pattern as current app, registered on `document`:

| Shortcut | Action |
|----------|--------|
| Ctrl+Shift+N | New session |
| Ctrl+Shift+\ | Split vertical |
| Ctrl+Shift+- | Split horizontal |
| Ctrl+Shift+W | Close pane |
| Ctrl+Shift+Arrow | Navigate panes |
| Ctrl+Shift+J/K | Move sidebar selection |
| Ctrl+Shift+Enter | Open in tab |
| Ctrl+Shift+D | Delete session |
| Ctrl+Shift+P | Push changes |
| Ctrl+Shift+R | Pause/resume |
| Ctrl+Shift+B | Toggle sidebar |
| Ctrl+Shift+Q | Quit |

## Migration Plan

### Kept As-Is

- `session/instance.go` — refactor to use PTY manager instead of tmux
- `session/storage.go` — no changes
- `session/git/` — no changes
- `config/` — no changes
- `daemon/` — refactor to work with PTY manager
- `cmd/` — no changes
- `log/` — no changes

### New Go Code

- `pty/manager.go` — PTY lifecycle management (spawn, resize, kill)
- `pty/websocket.go` — WebSocket server, per-session I/O bridge
- `pty/monitor.go` — Output buffering + prompt detection
- `wails/bindings.go` — SessionAPI struct exposed to frontend
- `main.go` — updated to initialize Wails app

### Removed

- `gui/` — entire directory (replaced by React frontend)
- `fyne-terminal-fork/` — replaced by xterm.js
- `session/tmux/` — replaced by PTY manager
- Fyne dependencies from `go.mod`

### New Frontend (`web/`)

- Replace Next.js scaffold with Vite + React + TypeScript
- All components listed above
- `package.json` with xterm.js, allotment, zustand

### Build

- `wails build` produces a single binary with embedded frontend assets
- Update CLAUDE.md build instructions
- `cs` binary at `~/.local/bin/cs` remains the install target
