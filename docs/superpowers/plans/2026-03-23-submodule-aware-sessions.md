# Submodule-Aware Sessions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend claude-squad so each session can manage a parent repo worktree plus per-submodule worktrees, enabling isolated concurrent work across a monorepo.

**Architecture:** The `GitWorktree` struct gains a `submodules` map and `isSubmoduleAware` flag. A new `SubmoduleWorktree` struct handles per-submodule worktree lifecycle. The UI adds a multi-select submodule picker to the session creation flow, and a new `S` keybinding for adding submodules to running sessions. Diffs are aggregated across submodules.

**Tech Stack:** Go, git CLI, bubbletea TUI framework, tmux

**Spec:** `docs/superpowers/specs/2026-03-23-submodule-aware-sessions-design.md`

---

## File Structure

### New Files

| File | Responsibility |
|------|---------------|
| `session/git/submodule.go` | `SubmoduleWorktree` struct, constructors, git dir discovery, worktree setup/teardown |
| `session/git/submodule_test.go` | Tests for submodule worktree operations |
| `session/git/detect.go` | Submodule detection: parse `git submodule status`, list available submodules |
| `session/git/detect_test.go` | Tests for submodule detection |
| `ui/overlay/submodulePicker.go` | Multi-select checkbox picker for submodules |

### Modified Files

| File | Changes |
|------|---------|
| `session/git/worktree.go` | Add `submodules` map, `isSubmoduleAware` flag to `GitWorktree`; new accessors |
| `session/git/worktree_ops.go` | Extend `Setup()`, `Cleanup()`, `Remove()`, `Prune()` for submodules; update `CleanupWorktrees()` |
| `session/git/diff.go` | Add `AggregatedDiffStats` struct; extend `Diff()` to aggregate across submodules |
| `session/storage.go` | Add `SubmoduleWorktreeData`, extend `GitWorktreeData` and serialization |
| `session/instance.go` | Add `selectedSubmodules` field; extend `Start()`, `Pause()`, `Resume()`, `Kill()` |
| `ui/overlay/textInput.go` | Add submodule picker as a new focus stop |
| `app/app.go` | Wire submodule picker into session creation flow; add `S` keybinding |
| `keys/keys.go` | Add `KeyAddSubmodule` binding for `S` |
| `ui/diff.go` | Render aggregated diffs with section headers |
| `ui/list.go` | Show active submodule indicators per session |

---

## Task 1: Submodule Detection

**Files:**
- Create: `session/git/detect.go`
- Create: `session/git/detect_test.go`
- Reference: `session/git/util.go` (for `findGitRepoRoot`, `IsGitRepo`)

- [ ] **Step 1: Write the failing test for `ListSubmodules()`**

```go
// session/git/detect_test.go
package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func setupTestRepoWithSubmodules(t *testing.T) (parentDir string, submoduleDir string) {
	t.Helper()
	tmpDir := t.TempDir()

	// Create a bare repo to use as a submodule remote
	subRemote := filepath.Join(tmpDir, "sub-remote.git")
	runCmd(t, "", "git", "init", "--bare", subRemote)

	// Create a working copy, add a commit, push to bare
	subWork := filepath.Join(tmpDir, "sub-work")
	runCmd(t, "", "git", "init", subWork)
	runCmd(t, subWork, "git", "config", "user.email", "test@test.com")
	runCmd(t, subWork, "git", "config", "user.name", "Test")
	writeFile(t, filepath.Join(subWork, "file.txt"), "hello")
	runCmd(t, subWork, "git", "add", ".")
	runCmd(t, subWork, "git", "commit", "-m", "init sub")
	runCmd(t, subWork, "git", "remote", "add", "origin", subRemote)
	runCmd(t, subWork, "git", "push", "origin", "HEAD:main")

	// Create parent repo
	parentDir = filepath.Join(tmpDir, "parent")
	runCmd(t, "", "git", "init", parentDir)
	runCmd(t, parentDir, "git", "config", "user.email", "test@test.com")
	runCmd(t, parentDir, "git", "config", "user.name", "Test")

	// Add submodule
	runCmd(t, parentDir, "git", "submodule", "add", subRemote, "my-submodule")
	runCmd(t, parentDir, "git", "commit", "-m", "add submodule")

	submoduleDir = filepath.Join(parentDir, "my-submodule")
	return parentDir, submoduleDir
}

func runCmd(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %s (%v)", name, args, out, err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestListSubmodules(t *testing.T) {
	parentDir, _ := setupTestRepoWithSubmodules(t)

	subs, err := ListSubmodules(parentDir)
	if err != nil {
		t.Fatalf("ListSubmodules failed: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 submodule, got %d", len(subs))
	}
	if subs[0].Path != "my-submodule" {
		t.Errorf("expected path 'my-submodule', got %q", subs[0].Path)
	}
	if subs[0].GitDir == "" {
		t.Error("expected GitDir to be set")
	}
}

func TestListSubmodules_NoSubmodules(t *testing.T) {
	tmpDir := t.TempDir()
	runCmd(t, "", "git", "init", tmpDir)

	subs, err := ListSubmodules(tmpDir)
	if err != nil {
		t.Fatalf("ListSubmodules failed: %v", err)
	}
	if len(subs) != 0 {
		t.Fatalf("expected 0 submodules, got %d", len(subs))
	}
}

func TestHasSubmodules(t *testing.T) {
	parentDir, _ := setupTestRepoWithSubmodules(t)

	if !HasSubmodules(parentDir) {
		t.Error("expected HasSubmodules to return true")
	}

	tmpDir := t.TempDir()
	runCmd(t, "", "git", "init", tmpDir)
	if HasSubmodules(tmpDir) {
		t.Error("expected HasSubmodules to return false for repo without submodules")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/jadams/go/src/bitbucket.org/vervemotion/claude-squad && go test ./session/git/ -run TestListSubmodules -v`
Expected: FAIL — `ListSubmodules` and `HasSubmodules` not defined

- [ ] **Step 3: Implement `ListSubmodules()` and `HasSubmodules()`**

```go
// session/git/detect.go
package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// SubmoduleInfo describes a discovered submodule in a parent repo.
type SubmoduleInfo struct {
	// Path is the relative path within the parent repo (e.g., "verve-backend")
	Path string
	// GitDir is the absolute path to the submodule's git directory,
	// discovered via `git rev-parse --git-dir`.
	GitDir string
}

// ListSubmodules returns all submodules in the given repo path.
// It discovers each submodule's git directory dynamically.
func ListSubmodules(repoPath string) ([]SubmoduleInfo, error) {
	cmd := exec.Command("git", "-C", repoPath, "submodule", "status")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list submodules: %s (%w)", output, err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var submodules []SubmoduleInfo
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: " <sha> <path> (<describe>)" or "+<sha> <path> (<describe>)"
		// Strip leading +/- status char
		line = strings.TrimLeft(line, "+-")
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		subPath := parts[1]

		// Discover the submodule's git directory dynamically
		absSubPath := filepath.Join(repoPath, subPath)
		gitDirCmd := exec.Command("git", "-C", absSubPath, "rev-parse", "--git-dir")
		gitDirOutput, err := gitDirCmd.CombinedOutput()
		if err != nil {
			// Submodule may not be initialized; skip it
			continue
		}
		gitDir := strings.TrimSpace(string(gitDirOutput))
		// Make absolute if relative
		if !filepath.IsAbs(gitDir) {
			gitDir = filepath.Join(absSubPath, gitDir)
		}
		gitDir, _ = filepath.Abs(gitDir)

		submodules = append(submodules, SubmoduleInfo{
			Path:   subPath,
			GitDir: gitDir,
		})
	}

	return submodules, nil
}

// HasSubmodules returns true if the repo at repoPath contains any submodules.
func HasSubmodules(repoPath string) bool {
	cmd := exec.Command("git", "-C", repoPath, "submodule", "status")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) != ""
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/jadams/go/src/bitbucket.org/vervemotion/claude-squad && go test ./session/git/ -run "TestListSubmodules|TestHasSubmodules" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add session/git/detect.go session/git/detect_test.go
git commit -m "feat: add submodule detection utilities"
```

---

## Task 2: SubmoduleWorktree Struct and Lifecycle

**Files:**
- Create: `session/git/submodule.go`
- Create: `session/git/submodule_test.go`
- Reference: `session/git/worktree_git.go` (for `runGitCommand` pattern)

- [ ] **Step 1: Write failing test for `NewSubmoduleWorktree()` and `Setup()`**

```go
// session/git/submodule_test.go
package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSubmoduleWorktreeSetupAndCleanup(t *testing.T) {
	parentDir, _ := setupTestRepoWithSubmodules(t)

	// Discover submodule info
	subs, err := ListSubmodules(parentDir)
	if err != nil {
		t.Fatalf("ListSubmodules: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 submodule, got %d", len(subs))
	}

	// Create a parent worktree first (simulates real flow)
	parentWorktree := filepath.Join(t.TempDir(), "parent-wt")
	runCmd(t, parentDir, "git", "worktree", "add", "-b", "test-session", parentWorktree)

	// Target path for submodule worktree inside parent worktree
	targetPath := filepath.Join(parentWorktree, subs[0].Path)

	sw := NewSubmoduleWorktree(
		subs[0].Path,
		subs[0].GitDir,
		targetPath,
		"test-sub-branch",
	)

	// Setup
	if err := sw.Setup(); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Verify worktree exists
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		t.Fatal("worktree directory was not created")
	}

	// Verify it's a git worktree
	if !IsGitRepo(targetPath) {
		t.Fatal("worktree is not a git repo")
	}

	// Verify branch name
	if sw.GetBranchName() != "test-sub-branch" {
		t.Errorf("expected branch 'test-sub-branch', got %q", sw.GetBranchName())
	}

	// Cleanup
	if err := sw.Remove(); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
}

func TestSubmoduleWorktreeIsDirtyAndCommit(t *testing.T) {
	parentDir, _ := setupTestRepoWithSubmodules(t)
	subs, _ := ListSubmodules(parentDir)

	parentWorktree := filepath.Join(t.TempDir(), "parent-wt")
	runCmd(t, parentDir, "git", "worktree", "add", "-b", "test-dirty", parentWorktree)

	targetPath := filepath.Join(parentWorktree, subs[0].Path)
	sw := NewSubmoduleWorktree(subs[0].Path, subs[0].GitDir, targetPath, "test-dirty-branch")
	if err := sw.Setup(); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	// Should not be dirty initially
	dirty, err := sw.IsDirty()
	if err != nil {
		t.Fatalf("IsDirty: %v", err)
	}
	if dirty {
		t.Error("expected clean worktree")
	}

	// Create a file
	writeFile(t, filepath.Join(targetPath, "new.txt"), "new content")

	// Should be dirty now
	dirty, err = sw.IsDirty()
	if err != nil {
		t.Fatalf("IsDirty: %v", err)
	}
	if !dirty {
		t.Error("expected dirty worktree")
	}

	// Commit changes
	if err := sw.CommitChanges("test commit"); err != nil {
		t.Fatalf("CommitChanges: %v", err)
	}

	// Should be clean again
	dirty, err = sw.IsDirty()
	if err != nil {
		t.Fatalf("IsDirty: %v", err)
	}
	if dirty {
		t.Error("expected clean worktree after commit")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./session/git/ -run "TestSubmoduleWorktree" -v`
Expected: FAIL — `NewSubmoduleWorktree` not defined

- [ ] **Step 3: Implement `SubmoduleWorktree`**

```go
// session/git/submodule.go
package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// SubmoduleWorktree manages a git worktree for a single submodule within a session.
type SubmoduleWorktree struct {
	// submodulePath is the relative path within the parent repo (e.g., "verve-backend")
	submodulePath string
	// gitDir is the absolute path to the submodule's git directory.
	// Discovered via `git -C <path> rev-parse --git-dir`.
	gitDir string
	// worktreePath is where this worktree lives (inside the parent worktree)
	worktreePath string
	// branchName is the branch for this submodule worktree
	branchName string
	// baseCommitSHA is the commit the worktree was created from
	baseCommitSHA string
	// isExistingBranch is true if the branch existed before the session
	isExistingBranch bool
}

// NewSubmoduleWorktree creates a new SubmoduleWorktree.
func NewSubmoduleWorktree(submodulePath, gitDir, worktreePath, branchName string) *SubmoduleWorktree {
	return &SubmoduleWorktree{
		submodulePath: submodulePath,
		gitDir:        gitDir,
		worktreePath:  worktreePath,
		branchName:    branchName,
	}
}

// NewSubmoduleWorktreeFromStorage restores a SubmoduleWorktree from persisted data.
func NewSubmoduleWorktreeFromStorage(submodulePath, gitDir, worktreePath, branchName, baseCommitSHA string, isExistingBranch bool) *SubmoduleWorktree {
	return &SubmoduleWorktree{
		submodulePath:    submodulePath,
		gitDir:           gitDir,
		worktreePath:     worktreePath,
		branchName:       branchName,
		baseCommitSHA:    baseCommitSHA,
		isExistingBranch: isExistingBranch,
	}
}

// Setup creates the submodule worktree.
func (s *SubmoduleWorktree) Setup() error {
	// Remove target directory if it exists (empty submodule placeholder from parent worktree)
	if err := os.RemoveAll(s.worktreePath); err != nil {
		return fmt.Errorf("failed to clean target path: %w", err)
	}

	// Get current HEAD of the submodule to use as base
	headOutput, err := s.runGitDirCommand("rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("failed to get submodule HEAD: %w", err)
	}
	s.baseCommitSHA = strings.TrimSpace(headOutput)

	// Check if branch already exists
	_, err = s.runGitDirCommand("show-ref", "--verify", fmt.Sprintf("refs/heads/%s", s.branchName))
	if err == nil {
		// Branch exists — create worktree from it
		s.isExistingBranch = true
		_, err = s.runGitDirCommand("worktree", "add", s.worktreePath, s.branchName)
	} else {
		// New branch from HEAD
		_, err = s.runGitDirCommand("worktree", "add", "-b", s.branchName, s.worktreePath)
	}
	if err != nil {
		return fmt.Errorf("failed to create submodule worktree: %w", err)
	}

	return nil
}

// Remove removes the worktree but preserves the branch.
func (s *SubmoduleWorktree) Remove() error {
	_, err := s.runGitDirCommand("worktree", "remove", "--force", s.worktreePath)
	if err != nil {
		return fmt.Errorf("failed to remove submodule worktree: %w", err)
	}
	return nil
}

// Cleanup removes the worktree and deletes the branch (unless it was pre-existing).
func (s *SubmoduleWorktree) Cleanup() error {
	// Remove worktree
	if _, err := os.Stat(s.worktreePath); err == nil {
		if err := s.Remove(); err != nil {
			// Force remove directory if git worktree remove fails
			_ = os.RemoveAll(s.worktreePath)
		}
	}

	// Prune stale worktree entries
	_, _ = s.runGitDirCommand("worktree", "prune")

	// Delete branch unless it was pre-existing
	if !s.isExistingBranch {
		_, _ = s.runGitDirCommand("branch", "-D", s.branchName)
	}

	return nil
}

// IsDirty checks if the worktree has uncommitted changes.
func (s *SubmoduleWorktree) IsDirty() (bool, error) {
	output, err := s.runWorktreeCommand("status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("failed to check submodule status: %w", err)
	}
	return strings.TrimSpace(output) != "", nil
}

// CommitChanges stages and commits all changes in the submodule worktree.
func (s *SubmoduleWorktree) CommitChanges(message string) error {
	dirty, err := s.IsDirty()
	if err != nil {
		return err
	}
	if !dirty {
		return nil
	}

	if _, err := s.runWorktreeCommand("add", "."); err != nil {
		return fmt.Errorf("failed to stage submodule changes: %w", err)
	}
	if _, err := s.runWorktreeCommand("commit", "-m", message, "--no-verify"); err != nil {
		return fmt.Errorf("failed to commit submodule changes: %w", err)
	}
	return nil
}

// Diff returns the diff stats for this submodule worktree.
// Uses the same line-counting approach as GitWorktree.Diff() for consistency.
func (s *SubmoduleWorktree) Diff() *DiffStats {
	if s.baseCommitSHA == "" {
		return &DiffStats{Error: fmt.Errorf("base commit SHA not set")}
	}

	stats := &DiffStats{}

	// Stage untracked files so they show in diff
	_, _ = s.runWorktreeCommand("add", "-N", ".")

	content, err := s.runWorktreeCommand("--no-pager", "diff", s.baseCommitSHA)
	if err != nil {
		stats.Error = err
		return stats
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			stats.Added++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			stats.Removed++
		}
	}
	stats.Content = content
	return stats
}

// PushChanges pushes the submodule branch to its remote.
func (s *SubmoduleWorktree) PushChanges() error {
	// Commit any dirty changes first
	dirty, err := s.IsDirty()
	if err != nil {
		return err
	}
	if dirty {
		if err := s.CommitChanges("[claudesquad] pre-push commit"); err != nil {
			return err
		}
	}

	_, err = s.runWorktreeCommand("push", "-u", "origin", s.branchName)
	if err != nil {
		return fmt.Errorf("failed to push submodule %s: %w", s.submodulePath, err)
	}
	return nil
}

// Accessors

func (s *SubmoduleWorktree) GetSubmodulePath() string { return s.submodulePath }
func (s *SubmoduleWorktree) GetGitDir() string        { return s.gitDir }
func (s *SubmoduleWorktree) GetWorktreePath() string   { return s.worktreePath }
func (s *SubmoduleWorktree) GetBranchName() string     { return s.branchName }
func (s *SubmoduleWorktree) GetBaseCommitSHA() string  { return s.baseCommitSHA }
func (s *SubmoduleWorktree) IsExistingBranch() bool    { return s.isExistingBranch }

// runGitDirCommand runs a git command using --git-dir (operates on the bare git dir).
func (s *SubmoduleWorktree) runGitDirCommand(args ...string) (string, error) {
	baseArgs := []string{"--git-dir=" + s.gitDir}
	cmd := exec.Command("git", append(baseArgs, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git command failed: %s (%w)", output, err)
	}
	return string(output), nil
}

// runWorktreeCommand runs a git command in the worktree working directory.
func (s *SubmoduleWorktree) runWorktreeCommand(args ...string) (string, error) {
	baseArgs := []string{"-C", s.worktreePath}
	cmd := exec.Command("git", append(baseArgs, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git command failed: %s (%w)", output, err)
	}
	return string(output), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./session/git/ -run "TestSubmoduleWorktree" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add session/git/submodule.go session/git/submodule_test.go
git commit -m "feat: add SubmoduleWorktree struct and lifecycle"
```

---

## Task 3: Extend GitWorktree for Submodule Awareness

**Files:**
- Modify: `session/git/worktree.go:21-35` (GitWorktree struct)
- Modify: `session/git/worktree_ops.go:13-36` (Setup), `100-134` (Cleanup)
- Modify: `session/git/diff.go` (AggregatedDiffStats)

- [ ] **Step 1: Write failing test for submodule-aware GitWorktree**

```go
// Add to session/git/submodule_test.go

func TestGitWorktreeWithSubmodules(t *testing.T) {
	parentDir, _ := setupTestRepoWithSubmodules(t)

	gw, _, err := NewGitWorktree(parentDir, "test-session")
	if err != nil {
		t.Fatalf("NewGitWorktree: %v", err)
	}

	// Should not be submodule-aware by default
	if gw.IsSubmoduleAware() {
		t.Error("expected non-submodule-aware worktree by default")
	}

	// Enable submodule awareness with submodule list
	subs, _ := ListSubmodules(parentDir)
	subPaths := []string{subs[0].Path}
	if err := gw.InitSubmodules(parentDir, subPaths); err != nil {
		t.Fatalf("InitSubmodules: %v", err)
	}

	if !gw.IsSubmoduleAware() {
		t.Error("expected submodule-aware worktree after InitSubmodules")
	}

	if len(gw.GetSubmodules()) != 1 {
		t.Errorf("expected 1 submodule, got %d", len(gw.GetSubmodules()))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./session/git/ -run TestGitWorktreeWithSubmodules -v`
Expected: FAIL

- [ ] **Step 3: Add submodule fields and methods to GitWorktree**

Add to `session/git/worktree.go` — new fields in `GitWorktree` struct:

```go
// submodules tracks per-submodule worktrees, keyed by relative path.
submodules map[string]*SubmoduleWorktree
// isSubmoduleAware distinguishes submodule-aware sessions from single-repo sessions.
isSubmoduleAware bool
```

Add new methods:

```go
// IsSubmoduleAware returns whether this worktree manages submodules.
func (g *GitWorktree) IsSubmoduleAware() bool {
	return g.isSubmoduleAware
}

// GetSubmodules returns the submodule worktrees map.
func (g *GitWorktree) GetSubmodules() map[string]*SubmoduleWorktree {
	return g.submodules
}

// InitSubmodules sets up submodule worktrees for the given submodule paths.
// originalRepoPath is the original (non-worktree) repo path, used to discover
// submodule git directories.
func (g *GitWorktree) InitSubmodules(originalRepoPath string, submodulePaths []string) error {
	if len(submodulePaths) == 0 {
		return nil
	}

	allSubs, err := ListSubmodules(originalRepoPath)
	if err != nil {
		return fmt.Errorf("failed to list submodules: %w", err)
	}

	// Build lookup map
	subInfoMap := make(map[string]SubmoduleInfo)
	for _, s := range allSubs {
		subInfoMap[s.Path] = s
	}

	if g.submodules == nil {
		g.submodules = make(map[string]*SubmoduleWorktree)
	}

	for _, subPath := range submodulePaths {
		info, ok := subInfoMap[subPath]
		if !ok {
			return fmt.Errorf("submodule %q not found in repo", subPath)
		}

		targetPath := filepath.Join(g.worktreePath, subPath)
		sw := NewSubmoduleWorktree(subPath, info.GitDir, targetPath, g.branchName)

		if err := sw.Setup(); err != nil {
			return fmt.Errorf("failed to setup submodule %s: %w", subPath, err)
		}

		g.submodules[subPath] = sw
	}

	g.isSubmoduleAware = true
	return nil
}

// AddSubmodule adds a single submodule worktree to a running session.
func (g *GitWorktree) AddSubmodule(originalRepoPath string, submodulePath string) error {
	return g.InitSubmodules(originalRepoPath, []string{submodulePath})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./session/git/ -run TestGitWorktreeWithSubmodules -v`
Expected: PASS

- [ ] **Step 5: Extend Cleanup() for submodules**

Modify `session/git/worktree_ops.go` — at the start of `Cleanup()`, add submodule cleanup:

```go
// Clean up submodule worktrees first
for _, sw := range g.submodules {
    if err := sw.Cleanup(); err != nil {
        log.ErrorLog.Printf("failed to cleanup submodule %s: %v", sw.GetSubmodulePath(), err)
    }
}
```

- [ ] **Step 6: Add AggregatedDiffStats to diff.go**

Add to `session/git/diff.go`:

```go
// AggregatedDiffStats holds diff stats across the parent repo and its submodules.
type AggregatedDiffStats struct {
	// Parent holds diff stats for the parent repo (may be nil if no parent-level changes)
	Parent *DiffStats
	// Submodules holds per-submodule diff stats, keyed by relative path
	Submodules map[string]*DiffStats
}

// AggregatedDiff returns diff stats for the parent and all submodule worktrees.
func (g *GitWorktree) AggregatedDiff() *AggregatedDiffStats {
	result := &AggregatedDiffStats{
		Parent:     g.Diff(),
		Submodules: make(map[string]*DiffStats),
	}
	for path, sw := range g.submodules {
		result.Submodules[path] = sw.Diff()
	}
	return result
}

// TotalAdded returns the sum of added lines across parent and all submodules.
func (a *AggregatedDiffStats) TotalAdded() int {
	total := 0
	if a.Parent != nil {
		total += a.Parent.Added
	}
	for _, s := range a.Submodules {
		total += s.Added
	}
	return total
}

// TotalRemoved returns the sum of removed lines across parent and all submodules.
func (a *AggregatedDiffStats) TotalRemoved() int {
	total := 0
	if a.Parent != nil {
		total += a.Parent.Removed
	}
	for _, s := range a.Submodules {
		total += s.Removed
	}
	return total
}
```

- [ ] **Step 7: Commit**

```bash
git add session/git/worktree.go session/git/worktree_ops.go session/git/diff.go session/git/submodule_test.go
git commit -m "feat: extend GitWorktree with submodule awareness"
```

---

## Task 4: Storage Serialization

**Files:**
- Modify: `session/storage.go:28-42` (data structs)
- Modify: `session/instance.go:71-107` (ToInstanceData/FromInstanceData)

- [ ] **Step 1: Write failing test for round-trip serialization**

```go
// Add to a new file session/storage_test.go or existing test file

func TestSubmoduleWorktreeDataSerialization(t *testing.T) {
	data := GitWorktreeData{
		RepoPath:         "/repo",
		WorktreePath:     "/wt",
		SessionName:      "test",
		BranchName:       "test-branch",
		BaseCommitSHA:    "abc123",
		IsExistingBranch: false,
		IsSubmoduleAware: true,
		Submodules: []SubmoduleWorktreeData{
			{
				SubmodulePath:    "verve-backend",
				GitDir:           "/repo/.git/modules/verve-backend",
				WorktreePath:     "/wt/verve-backend",
				BranchName:       "test-branch",
				BaseCommitSHA:    "def456",
				IsExistingBranch: false,
			},
		},
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var restored GitWorktreeData
	if err := json.Unmarshal(jsonBytes, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !restored.IsSubmoduleAware {
		t.Error("expected IsSubmoduleAware to be true")
	}
	if len(restored.Submodules) != 1 {
		t.Fatalf("expected 1 submodule, got %d", len(restored.Submodules))
	}
	if restored.Submodules[0].SubmodulePath != "verve-backend" {
		t.Errorf("unexpected submodule path: %s", restored.Submodules[0].SubmodulePath)
	}
}

func TestBackwardCompatibility_NoSubmodules(t *testing.T) {
	// Old-format JSON without submodule fields
	oldJSON := `{"repo_path":"/repo","worktree_path":"/wt","session_name":"s","branch_name":"b","base_commit_sha":"c","is_existing_branch":false}`

	var data GitWorktreeData
	if err := json.Unmarshal([]byte(oldJSON), &data); err != nil {
		t.Fatalf("unmarshal old format: %v", err)
	}

	if data.IsSubmoduleAware {
		t.Error("old format should not be submodule-aware")
	}
	if len(data.Submodules) != 0 {
		t.Error("old format should have no submodules")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./session/ -run "TestSubmoduleWorktreeData|TestBackwardCompatibility" -v`
Expected: FAIL — `SubmoduleWorktreeData`, `IsSubmoduleAware`, `Submodules` fields don't exist

- [ ] **Step 3: Add serialization structs**

Add to `session/storage.go` after `DiffStatsData`:

```go
// SubmoduleWorktreeData represents the serializable data of a SubmoduleWorktree
type SubmoduleWorktreeData struct {
	SubmodulePath    string `json:"submodule_path"`
	GitDir           string `json:"git_dir"`
	WorktreePath     string `json:"worktree_path"`
	BranchName       string `json:"branch_name"`
	BaseCommitSHA    string `json:"base_commit_sha"`
	IsExistingBranch bool   `json:"is_existing_branch"`
}
```

Add to `GitWorktreeData` struct:

```go
Submodules       []SubmoduleWorktreeData `json:"submodules,omitempty"`
IsSubmoduleAware bool                    `json:"is_submodule_aware,omitempty"`
```

- [ ] **Step 4: Update ToInstanceData and FromInstanceData**

In `session/instance.go` `ToInstanceData()`, after setting `data.Worktree`, add:

```go
if i.gitWorktree != nil && i.gitWorktree.IsSubmoduleAware() {
    data.Worktree.IsSubmoduleAware = true
    for _, sw := range i.gitWorktree.GetSubmodules() {
        data.Worktree.Submodules = append(data.Worktree.Submodules, SubmoduleWorktreeData{
            SubmodulePath:    sw.GetSubmodulePath(),
            GitDir:           sw.GetGitDir(),
            WorktreePath:     sw.GetWorktreePath(),
            BranchName:       sw.GetBranchName(),
            BaseCommitSHA:    sw.GetBaseCommitSHA(),
            IsExistingBranch: sw.IsExistingBranch(),
        })
    }
}
```

In `FromInstanceData()`, after creating the `gitWorktree`, add restoration of submodules:

```go
if data.Worktree.IsSubmoduleAware && len(data.Worktree.Submodules) > 0 {
    subs := make(map[string]*git.SubmoduleWorktree)
    for _, sd := range data.Worktree.Submodules {
        subs[sd.SubmodulePath] = git.NewSubmoduleWorktreeFromStorage(
            sd.SubmodulePath, sd.GitDir, sd.WorktreePath,
            sd.BranchName, sd.BaseCommitSHA, sd.IsExistingBranch,
        )
    }
    instance.gitWorktree.RestoreSubmodules(subs)
}
```

Add `RestoreSubmodules` method to `GitWorktree`:

```go
// RestoreSubmodules sets submodule state from storage (used during deserialization).
func (g *GitWorktree) RestoreSubmodules(subs map[string]*SubmoduleWorktree) {
    g.submodules = subs
    g.isSubmoduleAware = true
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./session/... -run "TestSubmoduleWorktreeData|TestBackwardCompatibility" -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add session/storage.go session/instance.go session/git/worktree.go
git commit -m "feat: add submodule serialization and backward-compatible storage"
```

---

## Task 5: Extend Instance Lifecycle (Pause/Resume/Kill)

**Files:**
- Modify: `session/instance.go:412-469` (Pause), `472-525` (Resume), `277-301` (Kill)

- [ ] **Step 1: Write failing test for submodule-aware Pause**

Create a test that sets up a submodule-aware instance, makes changes in a submodule, pauses, and verifies:
- Submodule changes are committed
- Submodule worktrees are removed
- Parent repo submodule pointer changes are discarded

```go
// Add to session/git/submodule_test.go

func TestPauseResumeWithSubmodules(t *testing.T) {
	parentDir, _ := setupTestRepoWithSubmodules(t)

	gw, _, err := NewGitWorktree(parentDir, "pause-test")
	if err != nil {
		t.Fatalf("NewGitWorktree: %v", err)
	}
	if err := gw.Setup(); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	subs, _ := ListSubmodules(parentDir)
	if err := gw.InitSubmodules(parentDir, []string{subs[0].Path}); err != nil {
		t.Fatalf("InitSubmodules: %v", err)
	}

	// Make a change in the submodule worktree
	subWT := gw.GetSubmodules()[subs[0].Path]
	writeFile(t, filepath.Join(subWT.GetWorktreePath(), "change.txt"), "changed")

	// Pause submodules (commit + remove)
	if err := gw.PauseSubmodules(); err != nil {
		t.Fatalf("PauseSubmodules: %v", err)
	}

	// Submodule worktree should be removed
	if _, err := os.Stat(subWT.GetWorktreePath()); !os.IsNotExist(err) {
		t.Error("expected submodule worktree to be removed after pause")
	}

	// Discard parent pointer changes
	if err := gw.DiscardSubmodulePointers(); err != nil {
		t.Fatalf("DiscardSubmodulePointers: %v", err)
	}

	// Resume submodules
	if err := gw.ResumeSubmodules(); err != nil {
		t.Fatalf("ResumeSubmodules: %v", err)
	}

	// Submodule worktree should exist again
	if _, err := os.Stat(subWT.GetWorktreePath()); os.IsNotExist(err) {
		t.Error("expected submodule worktree to exist after resume")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./session/git/ -run TestPauseResumeWithSubmodules -v`
Expected: FAIL

- [ ] **Step 3: Implement PauseSubmodules, DiscardSubmodulePointers, ResumeSubmodules**

Add to `session/git/worktree_ops.go`:

```go
// PauseSubmodules commits dirty changes in each submodule and removes their worktrees.
func (g *GitWorktree) PauseSubmodules() error {
	for path, sw := range g.submodules {
		dirty, err := sw.IsDirty()
		if err != nil {
			log.ErrorLog.Printf("failed to check if submodule %s is dirty: %v", path, err)
			continue
		}
		if dirty {
			msg := fmt.Sprintf("[claudesquad] paused submodule '%s'", path)
			if err := sw.CommitChanges(msg); err != nil {
				return fmt.Errorf("failed to commit submodule %s: %w", path, err)
			}
		}
		if err := sw.Remove(); err != nil {
			log.ErrorLog.Printf("failed to remove submodule worktree %s: %v", path, err)
		}
	}
	return nil
}

// DiscardSubmodulePointers reverts any submodule pointer changes in the parent worktree.
func (g *GitWorktree) DiscardSubmodulePointers() error {
	if len(g.submodules) == 0 {
		return nil
	}
	paths := make([]string, 0, len(g.submodules))
	for p := range g.submodules {
		paths = append(paths, p)
	}
	args := append([]string{"checkout", "--"}, paths...)
	_, err := g.runGitCommand(g.worktreePath, args...)
	if err != nil {
		// Not fatal — parent may not have pointer changes
		log.ErrorLog.Printf("failed to discard submodule pointers: %v", err)
	}
	return nil
}

// ResumeSubmodules recreates submodule worktrees after a resume.
// Call after the parent worktree has been recreated.
func (g *GitWorktree) ResumeSubmodules() error {
	// Deinit all submodules to ensure clean state
	_, _ = g.runGitCommand(g.worktreePath, "submodule", "deinit", "--all", "-f")

	for path, sw := range g.submodules {
		if err := sw.Setup(); err != nil {
			return fmt.Errorf("failed to resume submodule %s: %w", path, err)
		}
	}
	return nil
}
```

- [ ] **Step 4: Wire into Instance.Pause() and Instance.Resume()**

In `session/instance.go` `Pause()`, before the existing dirty check (line ~423), add:

```go
// Pause submodule worktrees first (commit + remove)
if i.gitWorktree.IsSubmoduleAware() {
    if err := i.gitWorktree.PauseSubmodules(); err != nil {
        errs = append(errs, fmt.Errorf("failed to pause submodules: %w", err))
    }
    // Discard submodule pointer changes in parent
    if err := i.gitWorktree.DiscardSubmodulePointers(); err != nil {
        errs = append(errs, fmt.Errorf("failed to discard submodule pointers: %w", err))
    }
}
```

In `Resume()`, after `i.gitWorktree.Setup()` (line ~489), add:

```go
// Resume submodule worktrees
if i.gitWorktree.IsSubmoduleAware() {
    if err := i.gitWorktree.ResumeSubmodules(); err != nil {
        log.ErrorLog.Printf("failed to resume submodules: %v", err)
        return fmt.Errorf("failed to resume submodules: %w", err)
    }
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./session/git/ -run TestPauseResumeWithSubmodules -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add session/git/worktree_ops.go session/instance.go
git commit -m "feat: submodule-aware pause, resume, and kill"
```

---

## Task 6: Submodule Multi-Select Picker (UI)

**Files:**
- Create: `ui/overlay/submodulePicker.go`
- Reference: `ui/overlay/branchPicker.go` (pattern to follow)

- [ ] **Step 1: Implement SubmodulePicker**

```go
// ui/overlay/submodulePicker.go
package overlay

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// SubmodulePicker provides a multi-select checkbox picker for submodules.
type SubmodulePicker struct {
	items    []string // submodule paths
	selected map[string]bool
	cursor   int
	focused  bool
	width    int
}

// NewSubmodulePicker creates a picker with the given submodule paths.
func NewSubmodulePicker(submodules []string) *SubmodulePicker {
	return &SubmodulePicker{
		items:    submodules,
		selected: make(map[string]bool),
		cursor:   0,
	}
}

// HandleKeyPress handles key events. Returns true if the key was consumed.
func (p *SubmodulePicker) HandleKeyPress(key string) bool {
	if !p.focused || len(p.items) == 0 {
		return false
	}
	switch key {
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		}
		return true
	case "down", "j":
		if p.cursor < len(p.items)-1 {
			p.cursor++
		}
		return true
	case " ":
		// Toggle selection
		item := p.items[p.cursor]
		p.selected[item] = !p.selected[item]
		return true
	case "a":
		// Select all
		for _, item := range p.items {
			p.selected[item] = true
		}
		return true
	}
	return false
}

// GetSelected returns the list of selected submodule paths.
func (p *SubmodulePicker) GetSelected() []string {
	var result []string
	for _, item := range p.items {
		if p.selected[item] {
			result = append(result, item)
		}
	}
	return result
}

// SetFocused sets whether the picker has focus.
func (p *SubmodulePicker) SetFocused(focused bool) {
	p.focused = focused
}

// IsFocused returns whether the picker has focus.
func (p *SubmodulePicker) IsFocused() bool {
	return p.focused
}

// SetWidth sets the render width.
func (p *SubmodulePicker) SetWidth(w int) {
	p.width = w
}

// IsEmpty returns true if there are no items.
func (p *SubmodulePicker) IsEmpty() bool {
	return len(p.items) == 0
}

// View renders the picker.
func (p *SubmodulePicker) View() string {
	if len(p.items) == 0 {
		return ""
	}

	title := "Submodules (space=toggle, a=all):"
	if p.focused {
		title = lipgloss.NewStyle().Bold(true).Render(title)
	}

	var lines []string
	lines = append(lines, title)

	// Show max 8 items with window around cursor
	maxVisible := 8
	start := 0
	if len(p.items) > maxVisible {
		start = p.cursor - maxVisible/2
		if start < 0 {
			start = 0
		}
		if start+maxVisible > len(p.items) {
			start = len(p.items) - maxVisible
		}
	}
	end := start + maxVisible
	if end > len(p.items) {
		end = len(p.items)
	}

	for i := start; i < end; i++ {
		item := p.items[i]
		checkbox := "[ ]"
		if p.selected[item] {
			checkbox = "[x]"
		}
		line := fmt.Sprintf(" %s %s", checkbox, item)
		if i == p.cursor && p.focused {
			line = lipgloss.NewStyle().
				Background(lipgloss.Color("237")).
				Render(line)
		}
		lines = append(lines, line)
	}

	if len(p.items) > maxVisible {
		lines = append(lines, fmt.Sprintf(" ... %d/%d shown", maxVisible, len(p.items)))
	}

	return strings.Join(lines, "\n")
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./ui/overlay/...`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add ui/overlay/submodulePicker.go
git commit -m "feat: add SubmodulePicker multi-select UI component"
```

---

## Task 7: Wire Submodule Picker into Session Creation Flow

**Files:**
- Modify: `ui/overlay/textInput.go:35-84` (TextInputOverlay struct and constructors)
- Modify: `app/app.go:576-604` (KeyPrompt handler), `447-529` (statePrompt handler), `874-876` (newPromptOverlay)
- Modify: `keys/keys.go` (add KeyAddSubmodule)

- [ ] **Step 1: Add submodulePicker to TextInputOverlay**

In `ui/overlay/textInput.go`, add a `submodulePicker *SubmodulePicker` field to `TextInputOverlay` struct (line ~45).

Create a new constructor:

```go
// NewTextInputOverlayWithSubmodules creates an overlay with branch picker,
// profile picker, and submodule picker.
func NewTextInputOverlayWithSubmodules(
	title string,
	initialValue string,
	profiles []config.Profile,
	submodules []string,
) *TextInputOverlay {
	overlay := NewTextInputOverlayWithBranchPicker(title, initialValue, profiles)
	if len(submodules) > 0 {
		overlay.submodulePicker = NewSubmodulePicker(submodules)
		overlay.numStops++ // Add one more focus stop
	}
	return overlay
}
```

Update focus navigation to include the submodule picker as a focus stop between the branch picker and the enter button. The focus order becomes:

1. Profile picker (if present)
2. Textarea
3. Branch picker
4. **Submodule picker** (new)
5. Enter button

In `HandleKeyPress()`, add a case for when the submodule picker is focused — delegate to `submodulePicker.HandleKeyPress()`. Follow the same pattern as the existing branch picker delegation.

In `View()` / `Render()`, add the submodule picker's `View()` output between the branch picker and the enter button. Follow the existing rendering pattern with divider styles between sections.

Add accessor:

```go
// GetSelectedSubmodules returns selected submodules, or nil if no picker.
func (o *TextInputOverlay) GetSelectedSubmodules() []string {
	if o.submodulePicker == nil {
		return nil
	}
	return o.submodulePicker.GetSelected()
}
```

Update `View()` to render the submodule picker when present.

- [ ] **Step 2: Update app.go to detect submodules and create picker**

In `app/app.go`, modify `newPromptOverlay()` (line ~874):

```go
func (m *home) newPromptOverlay() *overlay.TextInputOverlay {
	profiles := m.appConfig.GetProfiles()

	// Check if current directory has submodules
	cwd, _ := os.Getwd()
	submodules, _ := git.ListSubmodules(cwd)
	var subPaths []string
	for _, s := range submodules {
		subPaths = append(subPaths, s.Path)
	}

	if len(subPaths) > 0 {
		return overlay.NewTextInputOverlayWithSubmodules(
			"Enter prompt (optional):",
			"",
			profiles,
			subPaths,
		)
	}
	return overlay.NewTextInputOverlayWithBranchPicker(
		"Enter prompt (optional):",
		"",
		profiles,
	)
}
```

- [ ] **Step 3: Pass selected submodules to instance on submit**

In `app/app.go` statePrompt handler (line ~467-500), after extracting `selectedBranch` and `selectedProgram`, add:

```go
selectedSubmodules := m.textInputOverlay.GetSelectedSubmodules()
```

Then after `instance.Start(true)` succeeds, add submodule initialization. This requires adding a `selectedSubmodules` field to `Instance` and wiring it into `Start()`.

In `session/instance.go`, add:

```go
// selectedSubmodules are submodule paths to initialize on first start
selectedSubmodules []string
```

Add setter:

```go
func (i *Instance) SetSelectedSubmodules(subs []string) {
	i.selectedSubmodules = subs
}
```

In `Start()`, after worktree setup succeeds (line ~258) and before tmux start, add:

```go
// Initialize selected submodules
if len(i.selectedSubmodules) > 0 {
    if err := i.gitWorktree.InitSubmodules(i.Path, i.selectedSubmodules); err != nil {
        setupErr = fmt.Errorf("failed to init submodules: %w", err)
        return setupErr
    }
}
```

- [ ] **Step 4: Add `S` keybinding for adding submodules to existing session**

In `keys/keys.go`, add `KeyAddSubmodule` as the next iota value after `KeyShiftDown`:

```go
KeyAddSubmodule // Key for adding submodules to a session
```

Add to `GlobalKeyStringsMap`:

```go
"S": KeyAddSubmodule,
```

Add to `GlobalkeyBindings`:

```go
KeyAddSubmodule: key.NewBinding(
    key.WithKeys("S"),
    key.WithHelp("S", "add submodule"),
),
```

In `app/app.go`, add a new state `stateAddSubmodule` and handle `KeyAddSubmodule` in the default state key handler. The handler should:

1. Check that the selected instance is `Running` or `Ready` (not `Paused`)
2. Check that the instance is submodule-aware (or in a repo with submodules)
3. Get already-initialized submodule paths from the instance's GitWorktree
4. Get all available submodules via `git.ListSubmodules()`
5. Filter to only uninitialized submodules
6. If none available, show an error "All submodules already initialized"
7. Otherwise, create a standalone `SubmodulePicker` overlay and transition to `stateAddSubmodule`
8. On submit in `stateAddSubmodule`, call `instance.GetGitWorktree().AddSubmodule()` for each selected submodule
9. Transition back to `stateDefault`

```go
// In the stateDefault key handler, add a case for KeyAddSubmodule:
case keys.KeyAddSubmodule:
    instance := m.list.SelectedInstance()
    if instance == nil || instance.Paused() || !instance.Started() {
        return m, nil
    }
    gw, err := instance.GetGitWorktree()
    if err != nil || !gw.IsSubmoduleAware() {
        // Check if repo has submodules at all
        cwd, _ := os.Getwd()
        if !git.HasSubmodules(cwd) {
            return m, nil
        }
    }
    // Get available submodules not yet initialized
    cwd, _ := os.Getwd()
    allSubs, _ := git.ListSubmodules(cwd)
    existingSubs := gw.GetSubmodules()
    var available []string
    for _, s := range allSubs {
        if _, exists := existingSubs[s.Path]; !exists {
            available = append(available, s.Path)
        }
    }
    if len(available) == 0 {
        // Show error: all submodules already initialized
        return m, nil
    }
    // Create submodule picker overlay and transition to stateAddSubmodule
    // (Implementation details follow existing overlay patterns in app.go)
```

Note: the full overlay flow follows the same pattern as `statePrompt` — create an overlay, handle key events in the state handler, and on submit extract selections and act on them. The implementer should follow the `statePrompt` handler at `app.go:447-529` as the pattern.

- [ ] **Step 5: Verify full flow compiles**

Run: `go build ./...`
Expected: Success

- [ ] **Step 6: Commit**

```bash
git add ui/overlay/textInput.go app/app.go session/instance.go keys/keys.go
git commit -m "feat: wire submodule picker into session creation flow"
```

---

## Task 8: Aggregated Diff Display

**Files:**
- Modify: `ui/diff.go:43-95` (SetDiff method)
- Modify: `session/instance.go:528-551` (UpdateDiffStats)
- Modify: `ui/list.go:154-172` (diff stats in session list)

- [ ] **Step 1: Extend Instance.UpdateDiffStats for submodules**

In `session/instance.go`, modify `UpdateDiffStats()` to use `AggregatedDiff()` when submodule-aware:

```go
func (i *Instance) UpdateDiffStats() error {
	if !i.started {
		i.diffStats = nil
		return nil
	}
	if i.Status == Paused {
		return nil
	}

	if i.gitWorktree.IsSubmoduleAware() {
		agg := i.gitWorktree.AggregatedDiff()
		// Combine into a single DiffStats for backward compatibility
		combined := &git.DiffStats{
			Added:   agg.TotalAdded(),
			Removed: agg.TotalRemoved(),
		}
		// Build combined content with section headers
		var contentParts []string
		if agg.Parent != nil && agg.Parent.Content != "" {
			contentParts = append(contentParts, "--- parent ---\n"+agg.Parent.Content)
		}
		for path, stats := range agg.Submodules {
			if stats.Content != "" {
				contentParts = append(contentParts, fmt.Sprintf("--- %s ---\n%s", path, stats.Content))
			}
		}
		combined.Content = strings.Join(contentParts, "\n\n")
		i.diffStats = combined
		return nil
	}

	// Original single-repo behavior
	stats := i.gitWorktree.Diff()
	if stats.Error != nil {
		if strings.Contains(stats.Error.Error(), "base commit SHA not set") {
			i.diffStats = nil
			return nil
		}
		return fmt.Errorf("failed to get diff stats: %w", stats.Error)
	}
	i.diffStats = stats
	return nil
}
```

- [ ] **Step 2: Show submodule indicators in session list**

First, add a safe accessor to `session/instance.go`:

```go
// GetActiveSubmodulePaths returns the relative paths of active submodule worktrees,
// or nil if the session is not submodule-aware or not started.
func (i *Instance) GetActiveSubmodulePaths() []string {
    if !i.started || i.gitWorktree == nil || !i.gitWorktree.IsSubmoduleAware() {
        return nil
    }
    subs := i.gitWorktree.GetSubmodules()
    if len(subs) == 0 {
        return nil
    }
    paths := make([]string, 0, len(subs))
    for p := range subs {
        paths = append(paths, p)
    }
    return paths
}
```

Then in `ui/list.go`, in the `Render()` method of `InstanceRenderer` (line ~117), after rendering the branch name, add submodule indicators:

```go
// Show active submodules if any
subPaths := inst.GetActiveSubmodulePaths()
if len(subPaths) > 0 {
    var names []string
    for _, path := range subPaths {
        // Use last path component for brevity
        parts := strings.Split(path, "/")
        names = append(names, parts[len(parts)-1])
    }
    indicator := "[" + strings.Join(names, ",") + "]"
    // Render in subdued style after branch name
}
```

Note: the exact rendering will need to match the existing style patterns in `list.go`. Read the existing `Render()` method carefully and follow its truncation/styling patterns.

- [ ] **Step 3: Verify it compiles and looks correct**

Run: `go build ./...`
Expected: Success

- [ ] **Step 4: Commit**

```bash
git add session/instance.go ui/diff.go ui/list.go
git commit -m "feat: aggregated diff display and submodule indicators"
```

---

## Task 9: Push Flow for Submodules

**Files:**
- Modify: `session/git/worktree_git.go:69-124` (PushChanges)

- [ ] **Step 1: Extend PushChanges to push submodule branches**

Add a method to `GitWorktree`:

```go
// PushSubmoduleChanges pushes branches in all submodules that have changes.
// Returns a map of submodule path → error (nil for success).
func (g *GitWorktree) PushSubmoduleChanges() map[string]error {
	results := make(map[string]error)
	for path, sw := range g.submodules {
		if err := sw.PushChanges(); err != nil {
			results[path] = err
		} else {
			results[path] = nil
		}
	}
	return results
}
```

Modify the existing `PushChanges()` in `worktree_git.go` to also push submodules when the session is submodule-aware. After the parent push completes, push each submodule.

Add `PushSubmoduleChanges` to `session/git/worktree_git.go` (same file as `PushChanges`):

```go
// PushSubmoduleChanges pushes branches in all submodules that have changes.
// Returns a map of submodule path → error (nil for success).
func (g *GitWorktree) PushSubmoduleChanges() map[string]error {
	results := make(map[string]error)
	for path, sw := range g.submodules {
		if err := sw.PushChanges(); err != nil {
			results[path] = err
		} else {
			results[path] = nil
		}
	}
	return results
}
```

- [ ] **Step 2: Update push confirmation in app.go**

In `app/app.go` where `KeySubmit` is handled (line ~680-702), extend the push flow:

1. After successful parent push, call `gw.PushSubmoduleChanges()` if submodule-aware
2. Collect results and display per-submodule success/failure
3. Show a reminder message: "Remember to update submodule pointers in the parent repo when ready"

The reminder can be shown via the existing `ErrBox` component (used for transient messages) or as part of the confirmation overlay's result text.

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: Success

- [ ] **Step 4: Commit**

```bash
git add session/git/worktree_git.go app/app.go
git commit -m "feat: push flow for submodule branches with reminder"
```

---

## Task 10: Update Global CleanupWorktrees

**Files:**
- Modify: `session/git/worktree_ops.go:155-220` (CleanupWorktrees)

- [ ] **Step 1: Read the current CleanupWorktrees implementation**

Understand how it iterates `~/.claude-squad/worktrees/` and removes directories.

- [ ] **Step 2: Modify to handle submodule worktrees**

Before removing a parent worktree directory, check if it contains submodule worktrees (by looking for `.git` files in subdirectories that reference a `--git-dir`). For each such submodule, run `git worktree remove` from the appropriate git dir to properly unregister the worktree.

This prevents dangling worktree references in submodule git directories.

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: Success

- [ ] **Step 4: Commit**

```bash
git add session/git/worktree_ops.go
git commit -m "fix: clean up submodule worktree references during global cleanup"
```

---

## Task 11: Integration Testing

**Files:**
- Create: `session/git/integration_test.go`

- [ ] **Step 1: Write end-to-end test**

```go
func TestFullSubmoduleSessionLifecycle(t *testing.T) {
	parentDir, _ := setupTestRepoWithSubmodules(t)

	// 1. Detect submodules
	subs, _ := ListSubmodules(parentDir)

	// 2. Create parent worktree
	gw, _, err := NewGitWorktree(parentDir, "integration-test")
	if err != nil {
		t.Fatalf("NewGitWorktree: %v", err)
	}
	if err := gw.Setup(); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	// 3. Init submodules
	subPaths := []string{subs[0].Path}
	if err := gw.InitSubmodules(parentDir, subPaths); err != nil {
		t.Fatalf("InitSubmodules: %v", err)
	}

	// 4. Make changes in submodule
	sw := gw.GetSubmodules()[subs[0].Path]
	writeFile(t, filepath.Join(sw.GetWorktreePath(), "test.txt"), "test")

	// 5. Verify diff
	agg := gw.AggregatedDiff()
	if agg.Submodules[subs[0].Path] == nil {
		t.Fatal("expected submodule diff")
	}
	if agg.Submodules[subs[0].Path].Added == 0 {
		t.Error("expected added lines in submodule diff")
	}

	// 6. Pause
	if err := gw.PauseSubmodules(); err != nil {
		t.Fatalf("PauseSubmodules: %v", err)
	}
	gw.DiscardSubmodulePointers()

	// 7. Remove parent worktree
	if err := gw.Remove(); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// 8. Resume parent
	if err := gw.Setup(); err != nil {
		t.Fatalf("Re-setup: %v", err)
	}
	if err := gw.ResumeSubmodules(); err != nil {
		t.Fatalf("ResumeSubmodules: %v", err)
	}

	// 9. Verify submodule change survived
	content, err := os.ReadFile(filepath.Join(sw.GetWorktreePath(), "test.txt"))
	if err != nil {
		t.Fatalf("reading file after resume: %v", err)
	}
	if string(content) != "test" {
		t.Errorf("expected 'test', got %q", string(content))
	}

	// 10. Cleanup
	if err := gw.Cleanup(); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
}
```

- [ ] **Step 2: Run integration test**

Run: `go test ./session/git/ -run TestFullSubmoduleSessionLifecycle -v`
Expected: PASS

- [ ] **Step 3: Run all existing tests to verify no regressions**

Run: `go test ./...`
Expected: All pass

- [ ] **Step 4: Commit**

```bash
git add session/git/integration_test.go
git commit -m "test: add integration test for full submodule session lifecycle"
```
