package dialogs

import (
	"claude-squad/config"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

// SessionOptions holds the result of the new session dialog.
type SessionOptions struct {
	Name    string
	Prompt  string
	Program string
	Branch  string // empty = new branch from default
	InPlace bool
	WorkDir string // working directory for the session
}

// DirChangeResult holds the branch data returned when the working directory changes.
type DirChangeResult struct {
	DefaultBranch string
	Branches      []string
}

// ShowNewSession shows a dialog for creating a new session.
func ShowNewSession(
	profiles []config.Profile,
	defaultBranch string,
	branches []string,
	initialWorkDir string,
	parent fyne.Window,
	onBranchSearch func(dir string, filter string) []string,
	onDirChanged func(dir string) DirChangeResult,
	onSubmit func(SessionOptions),
) {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Session name")

	promptEntry := widget.NewMultiLineEntry()
	promptEntry.SetPlaceHolder("Initial prompt (optional)")
	promptEntry.SetMinRowsVisible(3)

	// In-place toggle
	inPlaceCheck := widget.NewCheck("Run in-place (no git isolation)", nil)

	// Mutable state shared across closures
	selectedDir := initialWorkDir
	newBranchLabel := fmt.Sprintf("New branch (from %s)", defaultBranch)
	originDefault := findOriginDefault(defaultBranch, branches)

	// Branch picker (declared early so directory picker closure can update it)
	branchOptions := buildBranchOptions(originDefault, newBranchLabel, branches)
	branchSelect := widget.NewSelect(branchOptions, nil)
	if originDefault != "" {
		branchSelect.SetSelected(originDefault)
	} else {
		branchSelect.SetSelected(newBranchLabel)
	}

	// Working directory picker
	dirLabel := widget.NewLabel(displayPath(initialWorkDir))
	dirLabel.Truncation = fyne.TextTruncateClip

	dirBrowseBtn := widget.NewButton("Browse...", func() {
		folderDialog := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			selectedDir = uri.Path()
			dirLabel.SetText(displayPath(selectedDir))

			// Re-fetch branches for the new directory
			if onDirChanged != nil {
				result := onDirChanged(selectedDir)
				newBranchLabel = fmt.Sprintf("New branch (from %s)", result.DefaultBranch)
				originDefault = findOriginDefault(result.DefaultBranch, result.Branches)
				branchSelect.Options = buildBranchOptions(originDefault, newBranchLabel, result.Branches)
				if originDefault != "" {
					branchSelect.SetSelected(originDefault)
				} else {
					branchSelect.SetSelected(newBranchLabel)
				}
				branchSelect.Refresh()
			}
		}, parent)

		// Set initial location for the folder dialog
		if selectedDir != "" {
			uri := storage.NewFileURI(selectedDir)
			lister, err := storage.ListerForURI(uri)
			if err == nil {
				folderDialog.SetLocation(lister)
			}
		}
		folderDialog.Show()
	})

	dirContainer := container.NewBorder(nil, nil, nil, dirBrowseBtn, dirLabel)

	// Search entry for filtering branches
	branchSearch := widget.NewEntry()
	branchSearch.SetPlaceHolder("Search branches...")
	branchSearch.OnChanged = func(filter string) {
		if onBranchSearch == nil {
			return
		}
		filtered := onBranchSearch(selectedDir, filter)
		branchSelect.Options = buildBranchOptions(originDefault, newBranchLabel, filtered)
		branchSelect.Refresh()
	}

	branchContainer := container.NewVBox(branchSearch, branchSelect)
	branchFormItem := widget.NewFormItem("Branch", branchContainer)

	// Toggle branch picker visibility based on in-place checkbox
	inPlaceCheck.OnChanged = func(checked bool) {
		if checked {
			branchFormItem.Widget = widget.NewLabel("(disabled for in-place sessions)")
		} else {
			branchFormItem.Widget = branchContainer
		}
		branchFormItem.Widget.Refresh()
	}

	// Program/profile selector
	profileNames := make([]string, len(profiles))
	for i, p := range profiles {
		profileNames[i] = p.Name
	}
	programSelect := widget.NewSelect(profileNames, nil)
	if len(profileNames) > 0 {
		programSelect.SetSelected(profileNames[0])
	}

	items := []*widget.FormItem{
		widget.NewFormItem("Name", nameEntry),
		widget.NewFormItem("Directory", dirContainer),
		widget.NewFormItem("In-place", inPlaceCheck),
		branchFormItem,
		widget.NewFormItem("Prompt", promptEntry),
	}
	if len(profiles) > 1 {
		items = append(items, widget.NewFormItem("Program", programSelect))
	}

	d := dialog.NewForm("New Session", "Create", "Cancel", items, func(confirmed bool) {
		if !confirmed {
			return
		}
		opts := SessionOptions{
			Name:    nameEntry.Text,
			Prompt:  promptEntry.Text,
			InPlace: inPlaceCheck.Checked,
			WorkDir: selectedDir,
		}

		// Resolve branch selection
		if !inPlaceCheck.Checked && branchSelect.Selected != newBranchLabel {
			opts.Branch = branchSelect.Selected
		}

		// Resolve program from profile
		selected := programSelect.Selected
		for _, p := range profiles {
			if p.Name == selected {
				opts.Program = p.Program
				break
			}
		}
		if onSubmit != nil {
			onSubmit(opts)
		}
	}, parent)
	d.Resize(fyne.NewSize(500, 550))
	d.Show()
}

// findOriginDefault finds the "origin/<defaultBranch>" entry in the branch list.
// Returns the matching branch name, or empty string if not found.
func findOriginDefault(defaultBranch string, branches []string) string {
	target := fmt.Sprintf("origin/%s", defaultBranch)
	for _, b := range branches {
		if b == target {
			return target
		}
	}
	return ""
}

// buildBranchOptions builds the ordered branch list for the dropdown.
// If originDefault is non-empty, it comes first, then "New branch (from ...)",
// then remaining branches with originDefault excluded to avoid duplication.
func buildBranchOptions(originDefault, newBranchLabel string, branches []string) []string {
	var opts []string
	if originDefault != "" {
		opts = append(opts, originDefault)
	}
	opts = append(opts, newBranchLabel)
	for _, b := range branches {
		if b != originDefault {
			opts = append(opts, b)
		}
	}
	return opts
}

// displayPath returns a user-friendly display string for a directory path.
// Replaces home directory prefix with "~" and returns "(current directory)" for empty.
func displayPath(dir string) string {
	if dir == "" {
		return "(current directory)"
	}
	home, err := os.UserHomeDir()
	if err == nil {
		if rel, err := filepath.Rel(home, dir); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.Join("~", rel)
		}
	}
	return dir
}
