package git

import (
	"path/filepath"
	"testing"
)

func setupBareRemote(t *testing.T, defaultBranch string) string {
	t.Helper()
	remote := filepath.Join(t.TempDir(), "remote.git")
	runCmd(t, "", "git", "init", "--bare", "--initial-branch="+defaultBranch, remote)
	return remote
}

func setupRepoWithRemote(t *testing.T, remoteBranch string) string {
	t.Helper()
	remote := setupBareRemote(t, remoteBranch)
	repo := filepath.Join(t.TempDir(), "repo")
	runCmd(t, "", "git", "clone", remote, repo)
	writeFile(t, filepath.Join(repo, "README.md"), "init")
	runCmd(t, repo, "git", "add", ".")
	runCmd(t, repo, "git", "commit", "-m", "init")
	runCmd(t, repo, "git", "push", "origin", remoteBranch)
	return repo
}

func TestDetectDefaultRemoteBranch_Main(t *testing.T) {
	repo := setupRepoWithRemote(t, "main")
	branches := DetectDefaultRemoteBranches(repo)
	if len(branches) != 1 || branches[0] != "origin/main" {
		t.Errorf("expected [origin/main], got %v", branches)
	}
}

func TestDetectDefaultRemoteBranch_Master(t *testing.T) {
	repo := setupRepoWithRemote(t, "master")
	branches := DetectDefaultRemoteBranches(repo)
	if len(branches) != 1 || branches[0] != "origin/master" {
		t.Errorf("expected [origin/master], got %v", branches)
	}
}

func TestDetectDefaultRemoteBranch_Neither(t *testing.T) {
	repo := setupRepoWithRemote(t, "develop")
	branches := DetectDefaultRemoteBranches(repo)
	if len(branches) != 0 {
		t.Errorf("expected empty, got %v", branches)
	}
}

func TestDetectDefaultRemoteBranch_Both(t *testing.T) {
	repo := setupRepoWithRemote(t, "main")
	runCmd(t, repo, "git", "checkout", "-b", "master")
	runCmd(t, repo, "git", "push", "origin", "master")
	runCmd(t, repo, "git", "checkout", "main")
	branches := DetectDefaultRemoteBranches(repo)
	if len(branches) != 2 || branches[0] != "origin/main" || branches[1] != "origin/master" {
		t.Errorf("expected [origin/main, origin/master], got %v", branches)
	}
}
