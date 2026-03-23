package overlay

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSubmodulePickerSelection(t *testing.T) {
	items := []string{"sub-a", "sub-b", "sub-c"}
	sp := NewSubmodulePicker(items)
	sp.Focus()

	// Select the first item (cursor starts at 0)
	sp.HandleKeyPress(tea.KeyMsg{Type: tea.KeySpace})

	// Move down and select the second item
	sp.HandleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	sp.HandleKeyPress(tea.KeyMsg{Type: tea.KeySpace})

	selected := sp.GetSelected()
	if len(selected) != 2 {
		t.Fatalf("expected 2 selected items, got %d: %v", len(selected), selected)
	}
	if selected[0] != "sub-a" {
		t.Errorf("expected first selected to be 'sub-a', got %q", selected[0])
	}
	if selected[1] != "sub-b" {
		t.Errorf("expected second selected to be 'sub-b', got %q", selected[1])
	}
}

func TestSubmodulePickerSelectAll(t *testing.T) {
	items := []string{"sub-a", "sub-b", "sub-c"}
	sp := NewSubmodulePicker(items)
	sp.Focus()

	// Press 'a' to select all
	sp.HandleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})

	selected := sp.GetSelected()
	if len(selected) != len(items) {
		t.Fatalf("expected all %d items selected, got %d", len(items), len(selected))
	}
	for i, item := range items {
		if selected[i] != item {
			t.Errorf("selected[%d] = %q, want %q", i, selected[i], item)
		}
	}
}

func TestSubmodulePickerNavigation(t *testing.T) {
	items := []string{"sub-a", "sub-b", "sub-c"}
	sp := NewSubmodulePicker(items)
	sp.Focus()

	// Cursor starts at 0; pressing up should not go below 0
	consumed := sp.HandleKeyPress(tea.KeyMsg{Type: tea.KeyUp})
	if !consumed {
		t.Error("expected KeyUp to be consumed when focused")
	}
	// Internal cursor is unexported; verify indirectly via selection order
	// Select at cursor (should still be index 0 = "sub-a")
	sp.HandleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	selected := sp.GetSelected()
	if len(selected) != 1 || selected[0] != "sub-a" {
		t.Errorf("after clamped-up navigation, expected 'sub-a' selected, got %v", selected)
	}
	// Deselect
	sp.HandleKeyPress(tea.KeyMsg{Type: tea.KeySpace})

	// Move down twice (to index 2)
	sp.HandleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	sp.HandleKeyPress(tea.KeyMsg{Type: tea.KeyDown})

	// Moving down again should clamp at last index
	sp.HandleKeyPress(tea.KeyMsg{Type: tea.KeyDown})

	// Select at cursor (should be index 2 = "sub-c")
	sp.HandleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	selected = sp.GetSelected()
	if len(selected) != 1 || selected[0] != "sub-c" {
		t.Errorf("after clamped-down navigation, expected 'sub-c' selected, got %v", selected)
	}

	// Test j/k navigation (vim keys)
	sp2 := NewSubmodulePicker(items)
	sp2.Focus()

	// j moves cursor down
	sp2.HandleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	sp2.HandleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	sel2 := sp2.GetSelected()
	if len(sel2) != 1 || sel2[0] != "sub-b" {
		t.Errorf("expected 'sub-b' selected after 'j', got %v", sel2)
	}

	// k moves cursor up
	sp2.HandleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	sp2.HandleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	sel2 = sp2.GetSelected()
	if len(sel2) != 2 || sel2[0] != "sub-a" || sel2[1] != "sub-b" {
		t.Errorf("expected 'sub-a' and 'sub-b' selected after 'k'+space, got %v", sel2)
	}
}

func TestSubmodulePickerEmpty(t *testing.T) {
	sp := NewSubmodulePicker([]string{})

	if !sp.IsEmpty() {
		t.Error("expected IsEmpty() to return true for picker with no items")
	}

	rendered := sp.Render()
	if rendered != "" {
		t.Errorf("expected Render() to return empty string for empty picker, got %q", rendered)
	}

	consumed := sp.HandleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	if consumed {
		t.Error("expected HandleKeyPress to return false for empty picker")
	}
}

func TestSubmodulePickerUnfocused(t *testing.T) {
	items := []string{"sub-a", "sub-b"}
	sp := NewSubmodulePicker(items)
	// Do NOT call Focus()

	consumed := sp.HandleKeyPress(tea.KeyMsg{Type: tea.KeySpace})
	if consumed {
		t.Error("expected HandleKeyPress to return false when picker is not focused")
	}

	consumed = sp.HandleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	if consumed {
		t.Error("expected KeyDown to be ignored when not focused")
	}

	consumed = sp.HandleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if consumed {
		t.Error("expected 'a' key to be ignored when not focused")
	}

	selected := sp.GetSelected()
	if len(selected) != 0 {
		t.Errorf("expected no items selected when unfocused, got %v", selected)
	}
}
