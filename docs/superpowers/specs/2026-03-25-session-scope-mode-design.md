# Session Scope Mode Design

## Overview

Session scope mode is a focused editing view that transforms the Claude Squad IDE layout into a VS Code-like experience. It lets users browse and edit files in a session's worktree while keeping the Claude Code terminal visible as a sidebar — bridging the gap between the Claude-first workflow and manual file editing.

## Architecture

**Approach: Single-Component Mode Switch**

When scope mode is active, `App.tsx` renders a completely separate `ScopeLayout` component tree instead of the normal sidebar + terminal layout. This keeps scope mode fully isolated from the existing layout, avoids regressions, and is easy to build incrementally.

## Layout

Four panes in a horizontal Allotment split:

1. **Session sidebar** (~110px, half of normal 220px) — shows only the scoped session with an exit button at the bottom. Session names truncated with ellipsis.
2. **File explorer** (~200px, resizable) — tree view of the session's worktree directory. Lazy-loaded, respects .gitignore.
3. **Monaco editor** (fills remaining space) — tabbed file editor with full VS Code capabilities (autocomplete, find/replace, minimap). Catppuccin Mocha theme.
4. **Claude Code terminal** (~300px, resizable) — same xterm.js terminal as normal mode, styled as a right sidebar with a header.

All panes are resizable via Allotment drag handles.

## Entry & Exit

### Entering scope mode

Three ways to enter:
- **Context menu:** Right-click session in sidebar → "Scope Mode"
- **Top bar button:** Microscope icon in the tab bar → scopes the active tab's session
- **Keyboard shortcut:** `Ctrl+Shift+S`

On enter:
1. Zustand store snapshots current state (open tabs, active tab, sidebar visibility)
2. All sessions except the target are hidden
3. Layout switches to `ScopeLayout` component
4. File explorer loads the session's worktree root
5. If the session is paused, it auto-resumes (worktree must exist)

### Exiting scope mode

Three ways to exit:
- Exit (X) button in the shrunk sidebar
- Same keyboard shortcut (`Ctrl+Shift+S`)
- Context menu → "Exit Scope Mode"

On exit:
1. Monaco editor and file explorer unmount (frees memory)
2. Layout switches back to normal component tree
3. Previously open tabs, active tab, and sidebar state restored from snapshot
4. Sidebar re-expands to 220px

## File Explorer

- **Data source:** New `ListDirectory(sessionId, path)` Go backend method. Returns entries with `name`, `path`, `isDir`, `size`.
- **Lazy loading:** Only loads children when a folder is expanded. No upfront full-tree scan.
- **Interactions:** Click file → opens in editor tab (or focuses if already open). Click folder → expand/collapse.
- **Visual styling:** 16px indentation per depth. Folder icons (▶/▼). File type coloring matching Catppuccin palette — folders in mauve, Go in blue, config in yellow.
- **Filtering:** Backend filters `.git/` and gitignored paths.
- **Remote sessions:** `ListDirectory` runs `ls` via existing SSH executor.

## Monaco Editor

- **Package:** `@monaco-editor/react` — lazy-loads the ~5MB Monaco bundle.
- **Tabs:** Each open file gets a tab with filename and close button (X).
- **File loading:** New `ReadFile(sessionId, path)` backend method returns contents as string.
- **Auto-save:** Debounced writes (500ms after last keystroke) via new `WriteFile(sessionId, path, contents)` backend method. No manual save needed.
- **Language detection:** Monaco auto-detects from file extension (built-in).
- **Theme:** Custom Catppuccin Mocha theme registered via `defineTheme`.
- **Config:** Minimap on, word wrap off, line numbers on, bracket matching.

## Backend API

Three new methods on `SessionAPI`, exposed via Wails:

### `ListDirectory(sessionId string, dirPath string) ([]DirectoryEntry, error)`
- Returns `[]DirectoryEntry` where each entry has `Name`, `Path`, `IsDir`, `Size`
- Filters `.git/` and gitignored paths
- Local: `os.ReadDir` + gitignore parsing
- Remote: `ls` via SSH executor with same filtering
- `dirPath` is relative to worktree root

### `ReadFile(sessionId string, filePath string) (string, error)`
- Returns file contents as string
- Local: `os.ReadFile`
- Remote: `cat` via SSH executor
- Path traversal validation (must stay within worktree)
- `filePath` is relative to worktree root

### `WriteFile(sessionId string, filePath string, contents string) error`
- Writes contents to file
- Local: `os.WriteFile`
- Remote: writes via SSH
- Same path traversal validation
- `filePath` is relative to worktree root

All methods resolve the session's worktree path from the session ID. The frontend only works with relative paths.

## State Management

New state in the existing Zustand `sessionStore`:

```typescript
// State
scopeMode: {
  active: boolean;
  sessionId: string | null;
  snapshot: { tabs: Tab[]; activeTabId: string | null; sidebarVisible: boolean } | null;
}
explorerTree: Map<string, DirectoryEntry[]>  // keyed by parent path
openEditorFiles: { path: string; contents: string; language: string }[]
activeEditorFile: string | null

// Actions
enterScopeMode(sessionId: string): void
exitScopeMode(): void
expandDirectory(path: string): void
openFile(path: string): void
closeEditorFile(path: string): void
saveFile(path: string, contents: string): void  // debounced
```

## Component Structure

New files:

```
frontend/src/components/ScopeMode/
  ScopeLayout.tsx        — Top-level: Allotment with 4 panes
  ScopeSidebar.tsx       — Shrunk sidebar with exit button
  FileExplorer.tsx       — Tree view with lazy-loaded directories
  FileTreeItem.tsx       — Single row (file or folder)
  EditorPane.tsx         — Monaco editor with tabbed interface
  EditorTabBar.tsx       — Tab bar for open files
  ClaudeTerminal.tsx     — Wrapper around TerminalPane for right sidebar
```

**App.tsx wiring:**
```tsx
function App() {
  const scopeMode = useSessionStore(s => s.scopeMode);
  if (scopeMode.active) return <ScopeLayout />;
  return <NormalLayout />;
}
```

Existing components (`Sidebar`, `TabBar`, `PaneManager`, `TerminalPane`) remain untouched. `ClaudeTerminal` reuses the `useTerminal` hook internally.

## Remote Session Support

All operations work identically for local and remote sessions:
- File explorer calls `ListDirectory` which routes through the backend's session resolution — local sessions use `os.ReadDir`, remote use SSH `ls`
- Editor reads/writes call `ReadFile`/`WriteFile` which similarly route through local fs or SSH
- The Claude Code terminal already handles remote sessions via the WebSocket + CompositeRegistry

The frontend never needs to know whether a session is local or remote.
