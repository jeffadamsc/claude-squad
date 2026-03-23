package session

import (
	"encoding/json"
	"strings"
	"testing"
)

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

func TestSubmoduleSerializationRoundTrip_MultipleSubmodules(t *testing.T) {
	original := GitWorktreeData{
		RepoPath:         "/repo",
		WorktreePath:     "/wt",
		SessionName:      "multi",
		BranchName:       "multi-branch",
		BaseCommitSHA:    "aaa111",
		IsExistingBranch: false,
		IsSubmoduleAware: true,
		Submodules: []SubmoduleWorktreeData{
			{
				SubmodulePath:    "sub-a",
				GitDir:           "/repo/.git/modules/sub-a",
				WorktreePath:     "/wt/sub-a",
				BranchName:       "multi-branch",
				BaseCommitSHA:    "bbb222",
				IsExistingBranch: false,
			},
			{
				SubmodulePath:    "sub-b",
				GitDir:           "/repo/.git/modules/sub-b",
				WorktreePath:     "/wt/sub-b",
				BranchName:       "multi-branch",
				BaseCommitSHA:    "ccc333",
				IsExistingBranch: true,
			},
		},
	}

	jsonBytes, err := json.Marshal(original)
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
	if len(restored.Submodules) != 2 {
		t.Fatalf("expected 2 submodules, got %d", len(restored.Submodules))
	}

	// Build a map for order-independent comparison
	byPath := make(map[string]SubmoduleWorktreeData)
	for _, s := range restored.Submodules {
		byPath[s.SubmodulePath] = s
	}

	for _, orig := range original.Submodules {
		got, ok := byPath[orig.SubmodulePath]
		if !ok {
			t.Errorf("submodule %q missing after round-trip", orig.SubmodulePath)
			continue
		}
		if got.GitDir != orig.GitDir {
			t.Errorf("%s: GitDir = %q, want %q", orig.SubmodulePath, got.GitDir, orig.GitDir)
		}
		if got.WorktreePath != orig.WorktreePath {
			t.Errorf("%s: WorktreePath = %q, want %q", orig.SubmodulePath, got.WorktreePath, orig.WorktreePath)
		}
		if got.BranchName != orig.BranchName {
			t.Errorf("%s: BranchName = %q, want %q", orig.SubmodulePath, got.BranchName, orig.BranchName)
		}
		if got.BaseCommitSHA != orig.BaseCommitSHA {
			t.Errorf("%s: BaseCommitSHA = %q, want %q", orig.SubmodulePath, got.BaseCommitSHA, orig.BaseCommitSHA)
		}
		if got.IsExistingBranch != orig.IsExistingBranch {
			t.Errorf("%s: IsExistingBranch = %v, want %v", orig.SubmodulePath, got.IsExistingBranch, orig.IsExistingBranch)
		}
	}
}

func TestInPlaceSessionSerialization(t *testing.T) {
	data := InstanceData{
		Title:   "test-inplace",
		Path:    "/some/path",
		InPlace: true,
		Program: "claude",
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var restored InstanceData
	if err := json.Unmarshal(jsonBytes, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !restored.InPlace {
		t.Error("expected InPlace to be true")
	}
	if restored.Worktree.RepoPath != "" {
		t.Error("expected empty worktree for in-place session")
	}
}

func TestInPlaceBackwardCompatibility(t *testing.T) {
	oldJSON := `{"title":"old","path":"/old","status":0,"program":"claude","worktree":{"repo_path":"/r","worktree_path":"/w","session_name":"s","branch_name":"b","base_commit_sha":"c"}}`

	var data InstanceData
	if err := json.Unmarshal([]byte(oldJSON), &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if data.InPlace {
		t.Error("old sessions should not be in-place")
	}
}

func TestSubmoduleSerializationRoundTrip_EmptySubmodules(t *testing.T) {
	original := GitWorktreeData{
		RepoPath:         "/repo",
		WorktreePath:     "/wt",
		SessionName:      "empty-subs",
		BranchName:       "empty-branch",
		BaseCommitSHA:    "abc123",
		IsExistingBranch: false,
		IsSubmoduleAware: true,
		Submodules:       []SubmoduleWorktreeData{},
	}

	jsonBytes, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var restored GitWorktreeData
	if err := json.Unmarshal(jsonBytes, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !restored.IsSubmoduleAware {
		t.Error("expected IsSubmoduleAware to be preserved as true")
	}
	if len(restored.Submodules) != 0 {
		t.Errorf("expected 0 submodules, got %d", len(restored.Submodules))
	}
	if restored.RepoPath != original.RepoPath {
		t.Errorf("RepoPath = %q, want %q", restored.RepoPath, original.RepoPath)
	}
	if restored.BaseCommitSHA != original.BaseCommitSHA {
		t.Errorf("BaseCommitSHA = %q, want %q", restored.BaseCommitSHA, original.BaseCommitSHA)
	}
}

func TestInPlaceSessionSerialization_AllFields(t *testing.T) {
	data := InstanceData{
		Title:   "in-place-test",
		Path:    "/home/user/project",
		Branch:  "main",
		Status:  0, // Running
		InPlace: true,
		Program: "claude",
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Verify in_place is present in JSON
	jsonStr := string(jsonBytes)
	if !strings.Contains(jsonStr, `"in_place":true`) {
		t.Errorf("expected in_place:true in JSON, got: %s", jsonStr)
	}

	// Verify worktree is zero (empty fields not serialized with omitempty would still be present)
	if strings.Contains(jsonStr, `"repo_path":"/"`) {
		t.Error("in-place session should not have repo_path set")
	}

	var restored InstanceData
	if err := json.Unmarshal(jsonBytes, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !restored.InPlace {
		t.Error("expected InPlace to be true after round-trip")
	}
	if restored.Path != "/home/user/project" {
		t.Errorf("expected path preserved, got %q", restored.Path)
	}
	if restored.Branch != "main" {
		t.Errorf("expected branch preserved, got %q", restored.Branch)
	}
}

func TestInPlaceFromInstanceData_SkipsWorktreeConstruction(t *testing.T) {
	// Verify that FromInstanceData with InPlace=true does NOT construct a GitWorktree.
	// We can't call FromInstanceData directly (it starts tmux), but we can verify
	// the serialization round-trip preserves InPlace and has zero-value worktree.
	data := InstanceData{
		Title:   "in-place-from",
		Path:    "/tmp/test-project",
		Branch:  "feature",
		Status:  1, // Paused — avoids calling Start()
		InPlace: true,
		Program: "claude",
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var restored InstanceData
	if err := json.Unmarshal(jsonBytes, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !restored.InPlace {
		t.Error("expected InPlace to be true")
	}
	// Worktree fields should be zero-value (not populated)
	if restored.Worktree.RepoPath != "" {
		t.Error("expected empty worktree RepoPath for in-place session")
	}
	if restored.Worktree.WorktreePath != "" {
		t.Error("expected empty worktree WorktreePath for in-place session")
	}
	if restored.Worktree.BranchName != "" {
		t.Error("expected empty worktree BranchName for in-place session")
	}
}
