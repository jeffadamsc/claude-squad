# Diff Viewer for Scope Mode

## Overview

A read-only diff viewer in scope mode that shows all changes across the worktree and its submodules since the branch diverged from its base. Accessed via a button in the file explorer header, rendered as a special tab in the editor pane using Monaco's built-in diff editor.

## Backend

### New API Method

```go
func (s *SessionAPI) GetDiffFiles(sessionId string) ([]DiffFile, error)
```

### DiffFile Struct

```go
type DiffFile struct {
    Path       string `json:"path"`
    OldContent string `json:"oldContent"`
    NewContent string `json:"newContent"`
    Status     string `json:"status"`     // "added", "modified", "deleted", "renamed"
    Submodule  string `json:"submodule"`  // empty string for root repo
}
```

### Implementation

1. Resolve the session's worktree directory and base commit SHA (already stored on the session).
2. Find the merge base: `git merge-base <base-commit> HEAD`.
3. Get the list of changed files: combine `git diff --name-status <merge-base>...HEAD` (committed changes) with `git diff --name-status` and `git diff --name-status --staged` (uncommitted changes). Deduplicate by path, preferring the most recent state.
4. For each changed file:
   - Old content: `git show <merge-base>:<path>` (empty string for added files).
   - New content: read from working tree (empty string for deleted files).
5. Repeat steps 2-4 for each submodule via `git submodule foreach --recursive`, prefixing paths with the submodule directory and populating the `submodule` field.
6. Support both local and remote sessions by routing through the existing local/remote executor pattern.

### Edge Cases

- **Added files:** `oldContent` is empty, status is `"added"`.
- **Deleted files:** `newContent` is empty, status is `"deleted"`.
- **Binary files:** Detect binary content (null bytes in first 512 bytes, same as `ReadFile`). Return status `"modified"` with both contents set to `"Binary file"` placeholder text.
- **Merge base not found:** Fall back to diffing against the base commit directly.
- **Submodule not initialized:** Skip it, don't error.

## Frontend

### File Explorer Button

Add a button to the `FileExplorer` header bar, next to the existing refresh button. Icon or label indicating "Diff" or a git-diff icon. On click, it opens the diff tab.

### Tab System Changes

Extend the editor tab model to support a special diff tab:

- Add a `type` field to `EditorFile`: `'file' | 'diff'`. Default to `'file'` for backward compatibility.
- When `type` is `'diff'`, the `EditorTabBar` renders it with a distinct label (e.g., "Changes") and the `EditorPane` renders the `DiffViewer` component instead of the Monaco editor.
- Only one diff tab can exist at a time. Clicking the button when a diff tab is already open switches to it.

### DiffViewer Component

A scrollable container rendering all changed files:

```
[Refresh button] [File count summary] [Auto-refresh indicator]
---
[Submodule: verve-backend]  (header, only shown if multiple repos)
  [verve-backend/api/handler.go - modified] (clickable -> opens file tab)
  [Monaco DiffEditor: inline/unified mode]

  [verve-backend/api/routes.go - added]
  [Monaco DiffEditor: inline/unified mode]
---
[Root repo]
  [main.go - modified]
  [Monaco DiffEditor: inline/unified mode]
```

Each file section has:
- **File header:** Path (clickable link that opens the file in a regular editor tab), change status badge, submodule label.
- **Monaco DiffEditor:** Rendered in inline (unified) mode. Read-only. Uses the same Catppuccin Mocha theme as the main editor. Language detection based on file extension (reuse existing `getLanguageFromPath`).
- **Collapsible:** Each file section can be collapsed/expanded. Default expanded.

### Lazy Rendering

Use an IntersectionObserver to only mount Monaco DiffEditor instances when their container is near the viewport. Unmount when scrolled far away. This prevents performance issues when many files are changed.

### Refresh Behavior

- **Manual:** Refresh button in the DiffViewer header re-fetches `GetDiffFiles`.
- **Auto-poll:** Re-fetch every 5 seconds while the diff tab is active. Preserve scroll position across refreshes. Show a subtle indicator when auto-refreshing.
- **Smart update:** Compare new diff data with cached data. Only re-render file sections whose content actually changed to avoid unnecessary Monaco remounts.

### Zustand Store Additions

```typescript
// New state
diffFiles: DiffFile[]
diffLoading: boolean

// New actions
fetchDiffFiles(sessionId: string): Promise<void>
clearDiffFiles(): void
```

### Wails Type Addition

```typescript
interface DiffFile {
    path: string
    oldContent: string
    newContent: string
    status: 'added' | 'modified' | 'deleted' | 'renamed'
    submodule: string
}
```

## File Inventory

### New Files
- `frontend/src/components/ScopeMode/DiffViewer.tsx` — Main diff viewer component
- `frontend/src/components/ScopeMode/DiffFileSection.tsx` — Individual file diff section with Monaco DiffEditor

### Modified Files
- `app/bindings.go` — Add `GetDiffFiles` method and `DiffFile` struct
- `frontend/src/lib/wails.ts` — Add `DiffFile` type and `GetDiffFiles` API binding
- `frontend/src/store/sessionStore.ts` — Add diff state and actions, extend `EditorFile` type
- `frontend/src/components/ScopeMode/FileExplorer.tsx` — Add diff button to header
- `frontend/src/components/ScopeMode/EditorTabBar.tsx` — Handle diff tab type rendering
- `frontend/src/components/ScopeMode/EditorPane.tsx` — Render DiffViewer when diff tab is active
