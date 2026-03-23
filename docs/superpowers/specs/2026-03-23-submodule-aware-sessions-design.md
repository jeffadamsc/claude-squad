# Submodule-Aware Sessions for Claude Squad

**Date:** 2026-03-23
**Status:** Draft

## Problem

Claude Squad creates one git worktree per session for a single repository. Users working in
monorepos with git submodules need to work on multiple features simultaneously, where each
feature may span changes across several submodules. Today this requires serializing work,
duplicating entire repo trees, or using separate machines.

## Solution

Extend claude-squad so that a single session can manage a **parent repo worktree** plus
**per-submodule worktrees** within it. Each session gets full isolation — two sessions
touching the same submodule each get their own worktree with their own branch.

## Core Concept

1. At session creation, detect if the repo contains submodules
2. Create a git worktree of the parent repo (lightweight — shares `.git` objects)
3. Submodule directories in the parent worktree start empty/uninitialized
4. Submodules are initialized on demand: user selects which ones at session creation, and
   can add more later
5. Each submodule worktree gets its own branch with a consistent naming convention

## Data Model

### New: `SubmoduleWorktree`

Tracks one submodule's worktree within a session:

```go
type SubmoduleWorktree struct {
    // Relative path within parent repo (e.g., "verve-backend")
    submodulePath string
    // Absolute path to the submodule's git directory.
    // Discovered via `git -C <submodule_path> rev-parse --git-dir`, NOT assumed.
    // Typically resolves to parent/.git/modules/<submodule> but may differ
    // for older clones, relocated repos, or nested submodule paths.
    gitDir string
    // Where this worktree lives — inside the parent worktree
    worktreePath string
    // Branch name for this submodule's worktree
    branchName string
    // Base commit SHA
    baseCommitSHA string
    // Whether the branch existed before the session
    isExistingBranch bool
}
```

### Modified: `GitWorktree`

The existing struct becomes the "parent worktree" and gains:

```go
// New fields on GitWorktree:
submodules       map[string]*SubmoduleWorktree  // keyed by relative path
isSubmoduleAware bool                            // distinguishes old vs new sessions
```

When `isSubmoduleAware` is false, all behavior is identical to today. Existing single-repo
sessions are unaffected.

### Modified: Storage

`GitWorktreeData` gains:

```go
type SubmoduleWorktreeData struct {
    SubmodulePath    string `json:"submodule_path"`
    GitDir           string `json:"git_dir"`
    WorktreePath     string `json:"worktree_path"`
    BranchName       string `json:"branch_name"`
    BaseCommitSHA    string `json:"base_commit_sha"`
    IsExistingBranch bool   `json:"is_existing_branch"`
}

// New fields on GitWorktreeData:
Submodules       []SubmoduleWorktreeData `json:"submodules,omitempty"`
IsSubmoduleAware bool                    `json:"is_submodule_aware,omitempty"`
```

### Diff Aggregation

The existing `DiffStats` (single repo) is extended with a per-submodule aggregate:

```go
type AggregatedDiffStats struct {
    // Parent repo diff (may be empty if no parent-level changes)
    Parent *DiffStats
    // Per-submodule diffs, keyed by submodule relative path
    Submodules map[string]*DiffStats
}
```

For non-submodule-aware sessions, `AggregatedDiffStats` is nil and behavior is unchanged.

## Session Lifecycle

### Creation

1. Run `git submodule status` to detect submodules
2. If submodules exist, create a parent repo worktree (same mechanism as today)
3. Show a multi-select picker: "Which submodules do you want to initialize?"
4. For each selected submodule, create a submodule worktree (see below)
5. Launch tmux session in the parent worktree directory

### Submodule Worktree Setup

When a submodule is initialized for a session:

1. Parse `.gitmodules` to find the submodule's path
2. Discover the submodule's git directory dynamically:
   ```
   git -C <original_submodule_path> rev-parse --git-dir
   ```
   Do NOT assume a path convention like `.git/modules/<name>`.
3. Create a git worktree using the discovered git directory:
   ```
   git --git-dir=<submodule_git_dir> worktree add <path_in_parent_worktree> -b <branch>
   ```
   The target path is inside the parent worktree
   (e.g., `~/.claude-squad/worktrees/cs-feature-a_abc123/verve-backend`)
4. Branch naming: `<prefix><session_name>` (same as parent), consistent across all
   submodules in a session. This means the same branch name appears in multiple repos,
   which is intentional — it makes it easy to identify which branches belong to which session.
5. Record the submodule worktree in session state

### Pause

Order matters to avoid committing submodule pointer changes in the parent:

1. **Submodules first:** For each submodule worktree:
   - Check if dirty, commit changes if so
   - Remove the worktree but preserve the branch
2. **Parent last:** After all submodule worktrees are removed:
   - Run `git checkout -- <submodule_paths>` in the parent worktree to discard any
     submodule pointer changes (we don't want to commit pointer updates)
   - Check if parent has other dirty changes, commit if so
   - Remove the parent worktree but preserve the branch

### Resume

1. Recreate the parent worktree from its existing branch
2. Deinitialize all submodules in the parent worktree (`git submodule deinit --all`)
   to ensure clean state — no stale `.git` files in submodule directories
3. Recreate each recorded submodule worktree within the parent worktree

### Kill / Cleanup

1. For each submodule worktree: run `git --git-dir=<submodule_git_dir> worktree remove`
   to properly unregister the worktree before removing files
2. Remove the parent worktree
3. Delete branches in each submodule (unless `isExistingBranch`)

Note: the global `CleanupWorktrees()` function (which iterates `~/.claude-squad/worktrees/`)
must be updated to run submodule worktree removal before removing parent directories, to
avoid leaving dangling worktree references in submodule git dirs.

### Push

1. Iterate each submodule that has changes
2. Push each submodule's branch independently using `git push` (not `gh repo sync`,
   since submodule repos may use different remotes or hosting providers)
3. Report success/failure per submodule
4. Parent repo submodule pointer updates are left to the user
5. After push, show a reminder: "Remember to update submodule pointers in the parent repo
   when ready"

### Adding Submodules to Existing Sessions

Users can add submodules to a running session via a new keybinding (`S`). This shows a
picker filtered to submodules not yet initialized in the session, sets up the worktree,
and records it in state.

The session must be in `Running` or `Ready` state to add submodules. Adding submodules
to a paused session is not supported (resume first).

## Implementation Notes

### Git Command Execution

`SubmoduleWorktree` needs its own git command runner. Rather than embedding `GitWorktree`,
it should have a standalone `runGitCommand` helper that operates on `gitDir` or
`worktreePath` as appropriate. Key operations needed:

- `IsDirty()` — run against `worktreePath`
- `CommitChanges()` — run against `worktreePath`
- `Diff()` — run against `worktreePath`, comparing to `baseCommitSHA`
- `Remove()` — run against `gitDir` (`git --git-dir=<gitDir> worktree remove <path>`)
- `Setup()` — run against `gitDir` (`git --git-dir=<gitDir> worktree add ...`)

This avoids confusion between `repoPath` semantics in `GitWorktree` (a working directory)
and `gitDir` in `SubmoduleWorktree` (a bare git directory).

## UX Changes

### Session Creation

When claude-squad detects submodules in the current repo:
1. Normal session creation flow (title, prompt, etc.)
2. Additional step: multi-select picker for submodules
3. User can select zero or more, and add more later

### Diff View

Combined diff with section headers per submodule:

```
--- verve-backend ---
+3 -1  src/handler.go

--- verve-backend-infra ---
+15 -0  terraform/lambda.tf
```

### Session List

Show active submodules per session, e.g., `[backend, infra]` as a compact indicator.

### Push Flow

Show which submodules have changes, push each independently, report per-submodule results.
Show reminder about parent repo submodule pointer updates.

## Scope

### In Scope

- Submodule detection at session creation
- Parent repo worktree creation
- Lazy submodule worktree initialization (select at creation + add later)
- Per-submodule pause/resume/cleanup/push
- Combined diff view across submodules
- Session list showing active submodules
- Full backward compatibility with single-repo sessions

### Non-Goals

- Automatic detection when claude code enters an uninitialized submodule (future)
- Parent repo submodule pointer management (user handles separately)
- Cross-session warnings (e.g., "session B also touches verve-backend")
- Nested submodules (submodules within submodules)
- Full clone fallback (Approach B — see below)

## Risks and Mitigations

**Git worktrees within worktrees:** Uncommon pattern. Mitigated by the fact that submodule
worktrees are created from the submodule's git directory (discovered dynamically), not from
the parent worktree. They are independent worktrees placed inside the parent worktree's
directory structure.

**Scale (10 sessions x 5 submodules = 50 worktrees):** Mitigated by lazy initialization —
only submodules that are actually needed get worktrees. Cleanup is thorough on kill.

**Submodule pointer drift during pause:** Mitigated by explicitly discarding submodule
pointer changes in the parent worktree before committing parent changes during pause.

**Dangling worktree references on cleanup:** Mitigated by running `git worktree remove`
on each submodule worktree before removing the parent worktree directory.

## Appendix: Full Clone Approach (Approach B)

If the worktree-per-submodule approach proves fragile, the fallback is a full clone:

Each session gets `git clone --recurse-submodules --reference <parent> <dest>`.
This gives complete isolation with zero orchestration but higher disk usage.

Key insertion points for implementing Approach B:
- `NewGitWorktree()` — clone instead of worktree add
- `Setup()` — recurse submodules in the clone
- `Cleanup()` — rm -rf the clone directory
- Storage serialization — track clone path instead of worktree paths
- Diff/Push — operate on the clone's submodules directly

The data model would simplify: no `SubmoduleWorktree` struct needed, just a single
clone path per session. The tradeoff is ~24G per session (though `--reference`
significantly reduces actual disk usage by sharing objects).
