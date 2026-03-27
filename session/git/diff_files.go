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
			status = "modified"
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
				df.OldContent = ""
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
	files, err := g.GetDiffFiles()
	if err != nil {
		return nil, err
	}

	// Discover submodules
	output, err := g.runGitCommand(g.worktreePath, "submodule", "foreach", "--quiet", "echo $sm_path")
	if err != nil || strings.TrimSpace(output) == "" {
		return files, nil
	}

	for _, smPath := range strings.Split(strings.TrimSpace(output), "\n") {
		smPath = strings.TrimSpace(smPath)
		if smPath == "" {
			continue
		}

		smWorktreePath := filepath.Join(g.worktreePath, smPath)

		// Get the base commit for this submodule
		smBaseCommit, err := g.runGitCommand(g.worktreePath, "ls-tree", g.baseCommitSHA, smPath)
		if err != nil || strings.TrimSpace(smBaseCommit) == "" {
			continue
		}
		// Parse ls-tree output: "160000 commit <sha>\t<path>"
		lsParts := strings.Fields(smBaseCommit)
		if len(lsParts) < 3 {
			continue
		}
		smBaseSHA := lsParts[2]

		smGw := &GitWorktree{
			repoPath:      smWorktreePath,
			worktreePath:  smWorktreePath,
			baseCommitSHA: smBaseSHA,
			executor:      g.executor,
		}

		smFiles, err := smGw.GetDiffFiles()
		if err != nil {
			continue
		}

		for i := range smFiles {
			smFiles[i].Path = smPath + "/" + smFiles[i].Path
			smFiles[i].Submodule = smPath
		}
		files = append(files, smFiles...)
	}

	return files, nil
}
