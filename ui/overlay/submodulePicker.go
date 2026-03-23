package overlay

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SubmodulePicker is an embeddable component for selecting multiple submodules.
// It displays a vertical list with checkboxes for multi-select.
type SubmodulePicker struct {
	items    []string
	selected map[string]bool
	cursor   int
	focused  bool
	width    int
}

// NewSubmodulePicker creates a new submodule picker with the given submodule list.
func NewSubmodulePicker(submodules []string) *SubmodulePicker {
	return &SubmodulePicker{
		items:    submodules,
		selected: make(map[string]bool),
		cursor:   0,
	}
}

// Focus gives the submodule picker focus.
func (sp *SubmodulePicker) Focus() {
	sp.focused = true
}

// Blur removes focus from the submodule picker.
func (sp *SubmodulePicker) Blur() {
	sp.focused = false
}

// IsFocused returns whether the submodule picker is focused.
func (sp *SubmodulePicker) IsFocused() bool {
	return sp.focused
}

// SetWidth sets the rendering width.
func (sp *SubmodulePicker) SetWidth(w int) {
	sp.width = w
}

// IsEmpty returns true if there are no submodules to display.
func (sp *SubmodulePicker) IsEmpty() bool {
	return len(sp.items) == 0
}

// HandleKeyPress processes a key event. Returns true if the key was consumed.
func (sp *SubmodulePicker) HandleKeyPress(msg tea.KeyMsg) bool {
	if !sp.focused || len(sp.items) == 0 {
		return false
	}
	switch msg.Type {
	case tea.KeyUp:
		if sp.cursor > 0 {
			sp.cursor--
		}
		return true
	case tea.KeyDown:
		if sp.cursor < len(sp.items)-1 {
			sp.cursor++
		}
		return true
	case tea.KeySpace:
		item := sp.items[sp.cursor]
		sp.selected[item] = !sp.selected[item]
		return true
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "k":
			if sp.cursor > 0 {
				sp.cursor--
			}
			return true
		case "j":
			if sp.cursor < len(sp.items)-1 {
				sp.cursor++
			}
			return true
		case "a":
			for _, item := range sp.items {
				sp.selected[item] = true
			}
			return true
		}
	}
	return false
}

// GetSelected returns the list of selected submodule names, preserving original order.
func (sp *SubmodulePicker) GetSelected() []string {
	var result []string
	for _, item := range sp.items {
		if sp.selected[item] {
			result = append(result, item)
		}
	}
	return result
}

var (
	spLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("62")).
			Bold(true)

	spSelectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("0"))

	spDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	spCheckedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("2"))
)

// Render renders the submodule picker.
func (sp *SubmodulePicker) Render() string {
	if len(sp.items) == 0 {
		return ""
	}

	var s strings.Builder
	s.WriteString(spLabelStyle.Render("Submodules"))
	if sp.focused {
		s.WriteString(spDimStyle.Render("  space=toggle, a=select all"))
	}
	s.WriteString("\n\n")

	maxVisible := 8
	start := 0
	if len(sp.items) > maxVisible {
		start = sp.cursor - maxVisible/2
		if start < 0 {
			start = 0
		}
		if start+maxVisible > len(sp.items) {
			start = len(sp.items) - maxVisible
		}
	}
	end := start + maxVisible
	if end > len(sp.items) {
		end = len(sp.items)
	}

	for i := start; i < end; i++ {
		item := sp.items[i]
		checkbox := "[ ]"
		if sp.selected[item] {
			checkbox = spCheckedStyle.Render("[x]")
		}
		line := fmt.Sprintf("  %s %s", checkbox, item)
		if i == sp.cursor && sp.focused {
			s.WriteString(spSelectedStyle.Render(line))
		} else if i == sp.cursor {
			s.WriteString(line)
		} else {
			s.WriteString(spDimStyle.Render(line))
		}
		if i < end-1 {
			s.WriteString("\n")
		}
	}

	if len(sp.items) > maxVisible {
		s.WriteString("\n")
		s.WriteString(spDimStyle.Render(fmt.Sprintf("  ... %d/%d shown", maxVisible, len(sp.items))))
	}

	return s.String()
}
