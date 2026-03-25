package session

import (
	"encoding/json"
	"testing"
)

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

func TestHostIDSerialization(t *testing.T) {
	data := InstanceData{
		Title:   "remote-session",
		Path:    "/remote/path",
		Program: "claude",
		HostID:  "host-uuid-123",
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var restored InstanceData
	if err := json.Unmarshal(jsonBytes, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if restored.HostID != "host-uuid-123" {
		t.Errorf("expected HostID 'host-uuid-123', got %q", restored.HostID)
	}
}

func TestHostIDBackwardCompatibility(t *testing.T) {
	// Old JSON without host_id field
	oldJSON := `{"title":"old","path":"/old","status":0,"program":"claude"}`

	var data InstanceData
	if err := json.Unmarshal([]byte(oldJSON), &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if data.HostID != "" {
		t.Error("old sessions should have empty HostID")
	}
}

func TestInPlaceBackwardCompatibility(t *testing.T) {
	// Old JSON without in_place field
	oldJSON := `{"title":"old","path":"/old","status":0,"program":"claude","worktree":{"repo_path":"/r","worktree_path":"/w","session_name":"s","branch_name":"b","base_commit_sha":"c"}}`

	var data InstanceData
	if err := json.Unmarshal([]byte(oldJSON), &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if data.InPlace {
		t.Error("old sessions should not be in-place")
	}
}
