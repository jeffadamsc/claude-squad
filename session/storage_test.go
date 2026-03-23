package session

import (
	"encoding/json"
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
