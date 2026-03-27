# Diff Viewer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a read-only diff viewer to scope mode that shows all changes since the branch diverged from its base, across root repo and submodules, using Monaco's built-in diff editor.

**Architecture:** New Go backend method `GetDiffFiles` computes per-file old/new content by comparing the worktree against its merge base. Frontend renders a scrollable list of Monaco DiffEditor instances in a special "Changes" tab, triggered from a button in the file explorer header. Auto-refreshes every 5 seconds.

**Tech Stack:** Go (git CLI), React, Zustand, Monaco Editor (`@monaco-editor/react` DiffEditor), TypeScript

---

## File Inventory

### New Files
- `session/git/diff_files.go` — `GetDiffFiles` method on `GitWorktree` returning per-file diff data
- `session/git/diff_files_test.go` — Tests for `GetDiffFiles`
- `frontend/src/components/ScopeMode/DiffViewer.tsx` — Main diff viewer component (scrollable list)
- `frontend/src/components/ScopeMode/DiffFileSection.tsx` — Individual file section with Monaco DiffEditor

### Modified Files
- `app/bindings.go` — Add `GetDiffFiles` Wails binding and `DiffFile` struct
- `frontend/src/lib/wails.ts` — Add `DiffFile` type and `GetDiffFiles` API declaration
- `frontend/src/store/sessionStore.ts` — Add diff state/actions, extend `EditorFile` with `type` field
- `frontend/src/components/ScopeMode/FileExplorer.tsx` — Add diff button to header
- `frontend/src/components/ScopeMode/EditorTabBar.tsx` — Handle diff tab rendering
- `frontend/src/components/ScopeMode/EditorPane.tsx` — Render `DiffViewer` when diff tab is active

---

### Task 1: Backend — `GetDiffFiles` on GitWorktree

**Files:**
- Create: `session/git/diff_files.go`
- Create: `session/git/diff_files_test.go`

- [ ] **Step 1: Write the test for GetDiffFiles with a modified file**

In `session/git/diff_files_test.go`:

```go
package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetDiffFiles_ModifiedFile(t *testing.T) {
	repo := createTestRepo(t) // from worktree_git_test.go — creates repo with "initial" commit
	wtPath := worktreeDir(t, "wt-diff")

	// Write a file and commit it as the base
	if err := os.WriteFile(filepath.Join(repo, "hello.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, repo, "add", "hello.go")
	gitCmd(t, repo, "commit", "-m", "add hello.go")
	baseCommit := headSHA(t, repo)

	// Create worktree on a new branch
	gw := &GitWorktree{
		repoPath:      repo,
		worktreePath:  wtPath,
		branchName:    "test-diff",
		baseCommitSHA: baseCommit,
		executor:      defaultExecutor,
	}
	if err := gw.Setup(); err != nil {
		t.Fatal(err)
	}

	// Modify the file in the worktree
	if err := os.WriteFile(filepath.Join(wtPath, "hello.go"), []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := gw.GetDiffFiles()
	if err != nil {
		t.Fatalf("GetDiffFiles() error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	f := files[0]
	if f.Path != "hello.go" {
		t.Errorf("path = %q, want %q", f.Path, "hello.go")
	}
	if f.Status != "modified" {
		t.Errorf("status = %q, want %q", f.Status, "modified")
	}
	if f.OldContent != "package main\n" {
		t.Errorf("oldContent = %q, want %q", f.OldContent, "package main\n")
	}
	if f.NewContent != "package main\n\nfunc main() {}\n" {
		t.Errorf("newContent = %q, want %q", f.NewContent, "package main\n\nfunc main() {}\n")
	}
	if f.Submodule != "" {
		t.Errorf("submodule = %q, want empty", f.Submodule)
	}
}
```

- [ ] **Step 2: Write the test for added and deleted files**

Append to `session/git/diff_files_test.go`:

```go
func TestGetDiffFiles_AddedFile(t *testing.T) {
	repo := createTestRepo(t)
	wtPath := worktreeDir(t, "wt-diff-add")
	baseCommit := headSHA(t, repo)

	gw := &GitWorktree{
		repoPath:      repo,
		worktreePath:  wtPath,
		branchName:    "test-diff-add",
		baseCommitSHA: baseCommit,
		executor:      defaultExecutor,
	}
	if err := gw.Setup(); err != nil {
		t.Fatal(err)
	}

	// Create a new file in the worktree
	if err := os.WriteFile(filepath.Join(wtPath, "new.txt"), []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := gw.GetDiffFiles()
	if err != nil {
		t.Fatalf("GetDiffFiles() error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Status != "added" {
		t.Errorf("status = %q, want %q", files[0].Status, "added")
	}
	if files[0].OldContent != "" {
		t.Errorf("oldContent should be empty for added file, got %q", files[0].OldContent)
	}
	if files[0].NewContent != "hello\n" {
		t.Errorf("newContent = %q, want %q", files[0].NewContent, "hello\n")
	}
}

func TestGetDiffFiles_DeletedFile(t *testing.T) {
	repo := createTestRepo(t)
	wtPath := worktreeDir(t, "wt-diff-del")

	// Create and commit a file
	if err := os.WriteFile(filepath.Join(repo, "remove.txt"), []byte("bye\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, repo, "add", "remove.txt")
	gitCmd(t, repo, "commit", "-m", "add remove.txt")
	baseCommit := headSHA(t, repo)

	gw := &GitWorktree{
		repoPath:      repo,
		worktreePath:  wtPath,
		branchName:    "test-diff-del",
		baseCommitSHA: baseCommit,
		executor:      defaultExecutor,
	}
	if err := gw.Setup(); err != nil {
		t.Fatal(err)
	}

	// Delete the file in the worktree
	if err := os.Remove(filepath.Join(wtPath, "remove.txt")); err != nil {
		t.Fatal(err)
	}

	files, err := gw.GetDiffFiles()
	if err != nil {
		t.Fatalf("GetDiffFiles() error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Status != "deleted" {
		t.Errorf("status = %q, want %q", files[0].Status, "deleted")
	}
	if files[0].OldContent != "bye\n" {
		t.Errorf("oldContent = %q, want %q", files[0].OldContent, "bye\n")
	}
	if files[0].NewContent != "" {
		t.Errorf("newContent should be empty for deleted file, got %q", files[0].NewContent)
	}
}
```

- [ ] **Step 3: Write the test for no changes**

Append to `session/git/diff_files_test.go`:

```go
func TestGetDiffFiles_NoChanges(t *testing.T) {
	repo := createTestRepo(t)
	wtPath := worktreeDir(t, "wt-diff-none")
	baseCommit := headSHA(t, repo)

	gw := &GitWorktree{
		repoPath:      repo,
		worktreePath:  wtPath,
		branchName:    "test-diff-none",
		baseCommitSHA: baseCommit,
		executor:      defaultExecutor,
	}
	if err := gw.Setup(); err != nil {
		t.Fatal(err)
	}

	files, err := gw.GetDiffFiles()
	if err != nil {
		t.Fatalf("GetDiffFiles() error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(files))
	}
}
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `cd /Users/jadams/go/src/bitbucket.org/vervemotion/claude-squad && go test ./session/git/ -run TestGetDiffFiles -v`
Expected: Compilation error — `GetDiffFiles` not defined.

- [ ] **Step 5: Implement GetDiffFiles**

Create `session/git/diff_files.go`:

```go
package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DiffFile represents a single changed file with its old and new content.
type DiffFile struct {
	Path       string `json:"path"`
	OldContent string `json:"oldContent"`
	NewContent string `json:"newContent"`
	Status     string `json:"status"`    // "added", "modified", "deleted"
	Submodule  string `json:"submodule"` // empty for root repo
}

// GetDiffFiles returns all changed files between the base commit and the current
// worktree state (including uncommitted changes).
func (g *GitWorktree) GetDiffFiles() ([]DiffFile, error) {
	if g.baseCommitSHA == "" {
		return nil, fmt.Errorf("base commit SHA not set")
	}

	// Stage untracked files so they appear in the diff
	g.runGitCommand(g.worktreePath, "add", "-N", ".")

	// Get list of changed files: committed + uncommitted vs base
	output, err := g.runGitCommand(g.worktreePath, "--no-pager", "diff", "--name-status", g.baseCommitSHA)
	if err != nil {
		return nil, fmt.Errorf("diff --name-status: %w", err)
	}

	if strings.TrimSpace(output) == "" {
		return nil, nil
	}

	var files []DiffFile
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) < 2 {
			continue
		}
		statusCode := parts[0]
		filePath := parts[1]

		var status string
		switch {
		case statusCode == "A":
			status = "added"
		case statusCode == "D":
			status = "deleted"
		case strings.HasPrefix(statusCode, "R"):
			status = "modified" // treat renames as modified
			// For renames, git outputs "old\tnew" — use the new path
			renameParts := strings.SplitN(filePath, "\t", 2)
			if len(renameParts) == 2 {
				filePath = renameParts[1]
			}
		default:
			status = "modified"
		}

		df := DiffFile{
			Path:   filePath,
			Status: status,
		}

		// Get old content from base commit
		if status != "added" {
			old, err := g.runGitCommand(g.worktreePath, "show", g.baseCommitSHA+":"+filePath)
			if err != nil {
				df.OldContent = "" // file may not exist at base (e.g., rename source)
			} else {
				df.OldContent = old
			}
		}

		// Get new content from working tree
		if status != "deleted" {
			absPath := filepath.Join(g.worktreePath, filePath)
			data, err := os.ReadFile(absPath)
			if err != nil {
				df.NewContent = ""
			} else {
				// Binary detection: check for null bytes in first 512 bytes
				checkLen := len(data)
				if checkLen > 512 {
					checkLen = 512
				}
				isBinary := false
				for i := 0; i < checkLen; i++ {
					if data[i] == 0 {
						isBinary = true
						break
					}
				}
				if isBinary {
					df.OldContent = "Binary file"
					df.NewContent = "Binary file"
				} else {
					df.NewContent = string(data)
				}
			}
		}

		files = append(files, df)
	}

	return files, nil
}

// GetDiffFilesWithSubmodules returns diff files for the root repo and all submodules.
func (g *GitWorktree) GetDiffFilesWithSubmodules() ([]DiffFile, error) {
	// Get root repo diffs
	files, err := g.GetDiffFiles()
	if err != nil {
		return nil, err
	}

	// Discover submodules
	output, err := g.runGitCommand(g.worktreePath, "submodule", "foreach", "--quiet", "echo $sm_path")
	if err != nil || strings.TrimSpace(output) == "" {
		// No submodules or command failed — just return root diffs
		return files, nil
	}

	for _, smPath := range strings.Split(strings.TrimSpace(output), "\n") {
		smPath = strings.TrimSpace(smPath)
		if smPath == "" {
			continue
		}

		smWorktreePath := filepath.Join(g.worktreePath, smPath)

		// Get the base commit for this submodule: what was committed at the base
		smBaseCommit, err := g.runGitCommand(g.worktreePath, "ls-tree", g.baseCommitSHA, smPath)
		if err != nil || strings.TrimSpace(smBaseCommit) == "" {
			// Submodule didn't exist at base — treat all files as added
			continue
		}
		// Parse ls-tree output: "160000 commit <sha>\t<path>"
		lsParts := strings.Fields(smBaseCommit)
		if len(lsParts) < 3 {
			continue
		}
		smBaseSHA := lsParts[2]

		// Create a temporary GitWorktree for the submodule
		smGw := &GitWorktree{
			repoPath:      smWorktreePath,
			worktreePath:  smWorktreePath,
			baseCommitSHA: smBaseSHA,
			executor:      g.executor,
		}

		smFiles, err := smGw.GetDiffFiles()
		if err != nil {
			continue // skip submodules that error
		}

		// Prefix paths and set submodule field
		for i := range smFiles {
			smFiles[i].Path = smPath + "/" + smFiles[i].Path
			smFiles[i].Submodule = smPath
		}
		files = append(files, smFiles...)
	}

	return files, nil
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd /Users/jadams/go/src/bitbucket.org/vervemotion/claude-squad && go test ./session/git/ -run TestGetDiffFiles -v`
Expected: All 4 tests PASS.

- [ ] **Step 7: Commit**

```bash
git add session/git/diff_files.go session/git/diff_files_test.go
git commit -m "feat: add GetDiffFiles and GetDiffFilesWithSubmodules to GitWorktree"
```

---

### Task 2: Backend — Wails binding for GetDiffFiles

**Files:**
- Modify: `app/bindings.go`

- [ ] **Step 1: Add the DiffFile struct and GetDiffFiles method to bindings.go**

Add near the other type definitions (after the `DiffStats` struct around line 62):

```go
type DiffFileResult struct {
	Path       string `json:"path"`
	OldContent string `json:"oldContent"`
	NewContent string `json:"newContent"`
	Status     string `json:"status"`
	Submodule  string `json:"submodule"`
}
```

Add the method (after the existing file-related methods, around line 1050):

```go
// GetDiffFiles returns all changed files for a session, including submodules.
func (api *SessionAPI) GetDiffFiles(sessionID string) ([]DiffFileResult, error) {
	api.mu.RLock()
	inst, ok := api.instances[sessionID]
	api.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	gw, err := inst.GetGitWorktree()
	if err != nil {
		return nil, fmt.Errorf("get git worktree: %w", err)
	}
	if gw == nil {
		return nil, fmt.Errorf("session has no git worktree (in-place session)")
	}

	gitFiles, err := gw.GetDiffFilesWithSubmodules()
	if err != nil {
		return nil, fmt.Errorf("get diff files: %w", err)
	}

	results := make([]DiffFileResult, len(gitFiles))
	for i, f := range gitFiles {
		results[i] = DiffFileResult{
			Path:       f.Path,
			OldContent: f.OldContent,
			NewContent: f.NewContent,
			Status:     f.Status,
			Submodule:  f.Submodule,
		}
	}
	return results, nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/jadams/go/src/bitbucket.org/vervemotion/claude-squad && go build ./app/...`
Expected: Clean compilation.

- [ ] **Step 3: Commit**

```bash
git add app/bindings.go
git commit -m "feat: add GetDiffFiles Wails binding"
```

---

### Task 3: Frontend — Wails types and store additions

**Files:**
- Modify: `frontend/src/lib/wails.ts`
- Modify: `frontend/src/store/sessionStore.ts`
- Modify: `frontend/src/store/sessionStore.test.ts`

- [ ] **Step 1: Add DiffFile type and API declaration to wails.ts**

In `frontend/src/lib/wails.ts`, add the interface before the `declare global` block:

```typescript
export interface DiffFile {
  path: string;
  oldContent: string;
  newContent: string;
  status: "added" | "modified" | "deleted";
  submodule: string;
}
```

Inside the `SessionAPI` declaration (after the `GetAllSymbols` line), add:

```typescript
GetDiffFiles(sessionId: string): Promise<DiffFile[]>;
```

- [ ] **Step 2: Add diff state and actions to sessionStore.ts**

Import `DiffFile` from wails.ts at the top:

```typescript
import type { SessionInfo, SessionStatus, HostInfo, DirectoryEntry, DiffFile } from "../lib/wails";
```

Add a `type` field to the `EditorFile` interface:

```typescript
interface EditorFile {
  path: string;
  contents: string;
  language: string;
  type: "file" | "diff";
}
```

Add to the `SessionState` interface (after `quickOpenVisible`):

```typescript
diffFiles: DiffFile[];
diffLoading: boolean;
fetchDiffFiles: (sessionId: string) => Promise<void>;
clearDiffFiles: () => void;
openDiffTab: () => void;
```

Add initial state values in the `create` call (after `quickOpenVisible: false`):

```typescript
diffFiles: [],
diffLoading: false,
```

Add the action implementations:

```typescript
fetchDiffFiles: async (sessionId) => {
  set({ diffLoading: true });
  try {
    const files = await api().GetDiffFiles(sessionId);
    set({ diffFiles: files ?? [], diffLoading: false });
  } catch (err) {
    console.error("Failed to fetch diff files:", err);
    set({ diffLoading: false });
  }
},

clearDiffFiles: () => set({ diffFiles: [], diffLoading: false }),

openDiffTab: () =>
  set((state) => {
    const existing = state.openEditorFiles.find((f) => f.type === "diff");
    if (existing) return { activeEditorFile: existing.path };
    return {
      openEditorFiles: [
        ...state.openEditorFiles,
        { path: "__diff__", contents: "", language: "plaintext", type: "diff" },
      ],
      activeEditorFile: "__diff__",
    };
  }),
```

Update `openEditorFile` to include the `type` field defaulting to `"file"`:

```typescript
openEditorFile: (path, contents, language) =>
  set((state) => {
    const existing = state.openEditorFiles.find((f) => f.path === path);
    if (existing) return { activeEditorFile: path };
    return {
      openEditorFiles: [...state.openEditorFiles, { path, contents, language, type: "file" }],
      activeEditorFile: path,
    };
  }),
```

Update `exitScopeMode` to also clear diff state:

```typescript
diffFiles: [],
diffLoading: false,
```

Note: The `fetchDiffFiles` action uses the `api()` function. Add this import at the top of `sessionStore.ts`:

```typescript
import { api } from "../lib/wails";
```

- [ ] **Step 3: Write tests for new store actions**

Append to `frontend/src/store/sessionStore.test.ts`:

```typescript
describe("diff tab management", () => {
  it("openDiffTab creates a diff tab with __diff__ path", () => {
    const { openDiffTab } = useSessionStore.getState();
    openDiffTab();
    const state = useSessionStore.getState();
    expect(state.openEditorFiles).toHaveLength(1);
    expect(state.openEditorFiles[0].type).toBe("diff");
    expect(state.openEditorFiles[0].path).toBe("__diff__");
    expect(state.activeEditorFile).toBe("__diff__");
  });

  it("openDiffTab switches to existing diff tab", () => {
    const store = useSessionStore.getState();
    store.openDiffTab();
    store.openEditorFile("test.go", "content", "go");
    expect(useSessionStore.getState().activeEditorFile).toBe("test.go");
    useSessionStore.getState().openDiffTab();
    expect(useSessionStore.getState().activeEditorFile).toBe("__diff__");
    expect(useSessionStore.getState().openEditorFiles).toHaveLength(2);
  });

  it("clearDiffFiles resets diff state", () => {
    useSessionStore.setState({ diffFiles: [{ path: "a", oldContent: "", newContent: "", status: "added", submodule: "" }], diffLoading: true });
    useSessionStore.getState().clearDiffFiles();
    const state = useSessionStore.getState();
    expect(state.diffFiles).toHaveLength(0);
    expect(state.diffLoading).toBe(false);
  });

  it("openEditorFile defaults type to file", () => {
    useSessionStore.getState().openEditorFile("test.go", "content", "go");
    const f = useSessionStore.getState().openEditorFiles[0];
    expect(f.type).toBe("file");
  });
});
```

- [ ] **Step 4: Run frontend tests**

Run: `cd /Users/jadams/go/src/bitbucket.org/vervemotion/claude-squad/frontend && npx vitest run`
Expected: All tests pass including new diff tab tests.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/wails.ts frontend/src/store/sessionStore.ts frontend/src/store/sessionStore.test.ts
git commit -m "feat: add diff state management and DiffFile types"
```

---

### Task 4: Frontend — DiffFileSection component

**Files:**
- Create: `frontend/src/components/ScopeMode/DiffFileSection.tsx`

- [ ] **Step 1: Create the DiffFileSection component**

This component renders a single file's diff with a collapsible header and Monaco DiffEditor.

```tsx
import { useState, useRef, useEffect } from "react";
import { DiffEditor } from "@monaco-editor/react";
import type { DiffFile } from "../../lib/wails";
import { detectLanguage } from "../../store/sessionStore";
import { catppuccinMocha } from "../../lib/monacoTheme";
import type * as Monaco from "monaco-editor";

interface DiffFileSectionProps {
  file: DiffFile;
  onOpenFile: (path: string) => void;
}

const statusColors: Record<string, string> = {
  added: "var(--green)",
  modified: "var(--yellow)",
  deleted: "var(--red)",
};

const statusLabels: Record<string, string> = {
  added: "A",
  modified: "M",
  deleted: "D",
};

export function DiffFileSection({ file, onOpenFile }: DiffFileSectionProps) {
  const [collapsed, setCollapsed] = useState(false);
  const [visible, setVisible] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  // Lazy-render: only mount DiffEditor when near viewport
  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) {
          setVisible(true);
          observer.disconnect();
        }
      },
      { rootMargin: "200px" }
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  const language = detectLanguage(file.path);

  // Count lines to set a reasonable editor height
  const maxLines = Math.max(
    (file.oldContent || "").split("\n").length,
    (file.newContent || "").split("\n").length
  );
  const editorHeight = Math.min(Math.max(maxLines * 19 + 20, 80), 600);

  const handleMount = (_editor: Monaco.editor.IStandaloneDiffEditor, monaco: typeof Monaco) => {
    monaco.editor.defineTheme("catppuccin-mocha", catppuccinMocha);
    monaco.editor.setTheme("catppuccin-mocha");
  };

  return (
    <div ref={containerRef} style={{ marginBottom: 2 }}>
      {/* File header */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 8,
          padding: "6px 12px",
          background: "var(--surface0)",
          cursor: "pointer",
          userSelect: "none",
          fontSize: 12,
        }}
        onClick={() => setCollapsed(!collapsed)}
      >
        <span style={{ color: "var(--overlay0)", fontSize: 10 }}>
          {collapsed ? "\u25B6" : "\u25BC"}
        </span>
        <span
          style={{
            color: statusColors[file.status] ?? "var(--text)",
            fontWeight: 600,
            fontSize: 11,
            minWidth: 14,
          }}
        >
          {statusLabels[file.status] ?? "?"}
        </span>
        <span
          onClick={(e) => {
            e.stopPropagation();
            onOpenFile(file.path);
          }}
          style={{
            color: "var(--blue)",
            cursor: "pointer",
            textDecoration: "underline",
            fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
          }}
          title={`Open ${file.path}`}
        >
          {file.path}
        </span>
        {file.submodule && (
          <span style={{ color: "var(--overlay0)", fontSize: 10 }}>
            ({file.submodule})
          </span>
        )}
      </div>

      {/* Diff editor */}
      {!collapsed && (
        <div style={{ height: editorHeight, borderBottom: "1px solid var(--surface0)" }}>
          {visible ? (
            <DiffEditor
              original={file.oldContent}
              modified={file.newContent}
              language={language}
              theme="catppuccin-mocha"
              onMount={handleMount}
              options={{
                readOnly: true,
                renderSideBySide: false,
                minimap: { enabled: false },
                fontSize: 13,
                fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace",
                scrollBeyondLastLine: false,
                automaticLayout: true,
                lineNumbers: "on",
                renderOverviewRuler: false,
                padding: { top: 4 },
              }}
            />
          ) : (
            <div
              style={{
                height: "100%",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                color: "var(--overlay0)",
                fontSize: 12,
              }}
            >
              Loading...
            </div>
          )}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/ScopeMode/DiffFileSection.tsx
git commit -m "feat: add DiffFileSection component with Monaco DiffEditor"
```

---

### Task 5: Frontend — DiffViewer component

**Files:**
- Create: `frontend/src/components/ScopeMode/DiffViewer.tsx`

- [ ] **Step 1: Create the DiffViewer component**

```tsx
import { useEffect, useRef, useCallback } from "react";
import { useSessionStore, detectLanguage } from "../../store/sessionStore";
import { DiffFileSection } from "./DiffFileSection";
import { api } from "../../lib/wails";

interface DiffViewerProps {
  sessionId: string;
}

export function DiffViewer({ sessionId }: DiffViewerProps) {
  const diffFiles = useSessionStore((s) => s.diffFiles);
  const diffLoading = useSessionStore((s) => s.diffLoading);
  const fetchDiffFiles = useSessionStore((s) => s.fetchDiffFiles);
  const openEditorFile = useSessionStore((s) => s.openEditorFile);
  const intervalRef = useRef<ReturnType<typeof setInterval>>();

  // Initial fetch
  useEffect(() => {
    fetchDiffFiles(sessionId);
  }, [sessionId, fetchDiffFiles]);

  // Auto-refresh every 5 seconds
  useEffect(() => {
    intervalRef.current = setInterval(() => {
      fetchDiffFiles(sessionId);
    }, 5000);
    return () => clearInterval(intervalRef.current);
  }, [sessionId, fetchDiffFiles]);

  const handleRefresh = () => {
    fetchDiffFiles(sessionId);
  };

  const handleOpenFile = useCallback(
    async (filePath: string) => {
      // Strip submodule prefix if needed — the file path is relative to worktree root
      try {
        const contents = await api().ReadFile(sessionId, filePath);
        openEditorFile(filePath, contents, detectLanguage(filePath));
      } catch (err) {
        console.error("Failed to open file from diff:", err);
      }
    },
    [sessionId, openEditorFile]
  );

  // Group files by submodule
  const rootFiles = diffFiles.filter((f) => !f.submodule);
  const submoduleGroups = new Map<string, typeof diffFiles>();
  for (const f of diffFiles) {
    if (f.submodule) {
      const group = submoduleGroups.get(f.submodule) ?? [];
      group.push(f);
      submoduleGroups.set(f.submodule, group);
    }
  }
  const hasSubmodules = submoduleGroups.size > 0;

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%" }}>
      {/* Header */}
      <div
        style={{
          padding: "8px 12px",
          display: "flex",
          alignItems: "center",
          gap: 10,
          borderBottom: "1px solid var(--surface0)",
          background: "var(--base)",
          fontSize: 12,
        }}
      >
        <span style={{ color: "var(--text)", fontWeight: 600 }}>Changes</span>
        <span style={{ color: "var(--overlay0)" }}>
          {diffFiles.length} file{diffFiles.length !== 1 ? "s" : ""}
        </span>
        <span
          onClick={handleRefresh}
          style={{
            cursor: "pointer",
            color: "var(--overlay0)",
            fontSize: 13,
            marginLeft: "auto",
          }}
          title="Refresh diff"
        >
          {"\u21BB"}
        </span>
        {diffLoading && (
          <span style={{ color: "var(--overlay0)", fontSize: 10 }}>
            refreshing...
          </span>
        )}
      </div>

      {/* Diff content */}
      <div style={{ flex: 1, overflowY: "auto", background: "var(--mantle)" }}>
        {diffFiles.length === 0 && !diffLoading && (
          <div
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              height: "100%",
              color: "var(--overlay0)",
              fontSize: 13,
            }}
          >
            No changes
          </div>
        )}

        {/* Root repo files */}
        {hasSubmodules && rootFiles.length > 0 && (
          <div
            style={{
              padding: "6px 12px",
              fontSize: 11,
              color: "var(--subtext0)",
              textTransform: "uppercase",
              background: "var(--base)",
              borderBottom: "1px solid var(--surface0)",
            }}
          >
            Root
          </div>
        )}
        {rootFiles.map((file) => (
          <DiffFileSection
            key={file.path}
            file={file}
            onOpenFile={handleOpenFile}
          />
        ))}

        {/* Submodule groups */}
        {Array.from(submoduleGroups.entries()).map(([smName, smFiles]) => (
          <div key={smName}>
            <div
              style={{
                padding: "6px 12px",
                fontSize: 11,
                color: "var(--subtext0)",
                textTransform: "uppercase",
                background: "var(--base)",
                borderBottom: "1px solid var(--surface0)",
              }}
            >
              {smName}
            </div>
            {smFiles.map((file) => (
              <DiffFileSection
                key={file.path}
                file={file}
                onOpenFile={handleOpenFile}
              />
            ))}
          </div>
        ))}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/ScopeMode/DiffViewer.tsx
git commit -m "feat: add DiffViewer component with auto-refresh and submodule grouping"
```

---

### Task 6: Frontend — Wire up FileExplorer, EditorTabBar, and EditorPane

**Files:**
- Modify: `frontend/src/components/ScopeMode/FileExplorer.tsx`
- Modify: `frontend/src/components/ScopeMode/EditorTabBar.tsx`
- Modify: `frontend/src/components/ScopeMode/EditorPane.tsx`

- [ ] **Step 1: Add diff button to FileExplorer header**

In `frontend/src/components/ScopeMode/FileExplorer.tsx`, add the store action import and the button.

Add to the selector block at the top of the component (after line 16):

```typescript
const openDiffTab = useSessionStore((s) => s.openDiffTab);
```

In the header `<div>` (lines 120-139), replace the single refresh `<span>` with a container holding both buttons. Replace the inner content of the header div (lines 132-138):

```tsx
<span>Explorer</span>
<div style={{ display: "flex", gap: 8, alignItems: "center" }}>
  <span
    onClick={() => openDiffTab()}
    style={{ cursor: "pointer", fontSize: 12 }}
    title="Show changes"
  >
    {"\u0394"}
  </span>
  <span
    onClick={handleRefresh}
    style={{ cursor: "pointer", fontSize: 13 }}
  >
    {"\u21BB"}
  </span>
</div>
```

- [ ] **Step 2: Update EditorTabBar to render diff tab differently**

In `frontend/src/components/ScopeMode/EditorTabBar.tsx`, update the tab label logic. Replace the `name` computation (line 19):

```tsx
const isDiff = file.type === "diff";
const name = isDiff ? "Changes" : (file.path.split("/").pop() ?? file.path);
```

Optionally add a style for the diff tab icon — add before the `{name}` text:

```tsx
{isDiff && <span style={{ fontSize: 10 }}>{"Δ "}</span>}
```

Also need to add the `type` to the `EditorFile` usage. The `EditorTabBar` gets files from the store which now includes `type`, so the existing code will work — we just need to read `file.type` which is already on the object. No type import changes needed since it comes from the store.

- [ ] **Step 3: Update EditorPane to render DiffViewer for diff tabs**

In `frontend/src/components/ScopeMode/EditorPane.tsx`, import the DiffViewer:

```typescript
import { DiffViewer } from "./DiffViewer";
```

Update the `activeFile` check and rendering. After `const activeFile = ...` (line 32-34), add:

```typescript
const isDiffTab = activeFile?.type === "diff";
```

Replace the return statement for when `activeFile` exists (lines 210-240) with:

```tsx
return (
  <div style={{ display: "flex", flexDirection: "column", height: "100%" }}>
    <EditorTabBar />
    <div style={{ flex: 1 }}>
      {isDiffTab ? (
        <DiffViewer sessionId={sessionId} />
      ) : (
        <Editor
          key={activeFile.path}
          defaultValue={activeFile.contents}
          language={activeFile.language}
          theme="catppuccin-mocha"
          onMount={handleMount}
          onChange={handleChange}
          options={{
            minimap: { enabled: false },
            fontSize: 13,
            fontFamily:
              "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace",
            lineNumbers: "on",
            scrollBeyondLastLine: false,
            automaticLayout: true,
            wordWrap: "off",
            bracketPairColorization: { enabled: true },
            padding: { top: 8 },
            gotoLocation: {
              multiple: "peek",
              multipleDefinitions: "peek",
            },
          }}
        />
      )}
    </div>
  </div>
);
```

- [ ] **Step 4: Verify the app compiles**

Run: `cd /Users/jadams/go/src/bitbucket.org/vervemotion/claude-squad && wails build`
Expected: Clean build.

- [ ] **Step 5: Run frontend tests**

Run: `cd /Users/jadams/go/src/bitbucket.org/vervemotion/claude-squad/frontend && npx vitest run`
Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/ScopeMode/FileExplorer.tsx frontend/src/components/ScopeMode/EditorTabBar.tsx frontend/src/components/ScopeMode/EditorPane.tsx
git commit -m "feat: wire up diff viewer button, tab rendering, and editor pane switching"
```

---

### Task 7: Manual Integration Test

- [ ] **Step 1: Build and run the app**

Run: `cd /Users/jadams/go/src/bitbucket.org/vervemotion/claude-squad && wails build && cp -R build/bin/claude-squad.app /Applications/`

- [ ] **Step 2: Manual testing checklist**

1. Open the app, create or open a session in scope mode
2. Verify the "Δ" button appears in the file explorer header, next to the refresh button
3. Click the "Δ" button — a "Changes" tab should appear in the editor tab bar
4. The diff viewer should show all changed files with inline diffs
5. File paths should be clickable blue links — clicking opens the file in a regular editor tab
6. Each file section should be collapsible via the triangle icon
7. The refresh button in the diff header should re-fetch changes
8. Wait 5+ seconds — auto-refresh should update the view
9. Make a change to a file in the editor, switch back to the Changes tab, and verify the diff updates
10. Close the Changes tab via the × button — it should close like any other tab
11. Re-click the "Δ" button — it should reopen the tab (not create a duplicate)
