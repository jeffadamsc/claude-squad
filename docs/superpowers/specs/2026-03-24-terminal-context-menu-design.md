# Terminal Pane Context Menu

## Summary

Add a right-click context menu to terminal panes in the claude-squad GUI. Replace the current right-click-to-paste behavior with a `PopUpMenu` offering copy, paste, split, pause, and kill actions.

## Context Menu Items

| Item | Condition | Action |
|------|-----------|--------|
| Copy | Enabled only when text is selected | Copy selected text to clipboard |
| Paste | Always enabled | Paste clipboard content into terminal |
| *(separator)* | | |
| Split Horizontal | Always enabled | Split pane top/bottom (`SplitHorizontal()`) |
| Split Vertical | Always enabled | Split pane left/right (`SplitVertical()`) |
| *(separator)* | | |
| Pause Session | Always enabled (only reachable on active sessions) | Pause the session |
| Kill Session | Always enabled | Kill the session, no confirmation (matches existing hotkey behavior) |

Resume Session is intentionally excluded — paused sessions are not displayed in terminal panes, so resume would never be reachable.

## Approach

Use Fyne's built-in `widget.PopUpMenu` for native look, automatic dismiss-on-click-outside, and keyboard navigation.

## Implementation Points

### 1. Terminal widget: export clipboard methods and add callback (`fyne-terminal-fork`)

**mouse.go:**
- Remove existing right-click-to-paste behavior from `MouseDown`
- On secondary button press, do NOT auto-copy-and-clear the selection (so the selection remains visible for the context menu's Copy action)
- Add exported callback field on Terminal: `OnSecondaryMouseDown func(fyne.Position)`
- Invoke the callback with the absolute canvas position (use `fyne.CurrentApp().Driver().AbsolutePositionForObject(t)` plus the relative event position)

**select.go:**
- Export `copySelectedText` → `CopySelectedText(clipboard fyne.Clipboard)`
- Export `pasteText` → `PasteText(clipboard fyne.Clipboard)`
- Add new `HasSelectedText() bool` — returns true if there is an active text selection

**term.go:**
- Add `OnSecondaryMouseDown func(fyne.Position)` field to Terminal struct

### 2. Handle right-click at the overlay level (`gui/panes/pane.go`)

The pane's `tapOverlay` sits on top of the terminal and intercepts pointer events. Right-click events hit the overlay before the terminal. To handle this:

- Add `SecondaryTapped(e *fyne.PointEvent)` to `tapOverlay` (implements `fyne.SecondaryTappable`)
- In `SecondaryTapped`, construct the context menu and show it at the tap position
- This means the `OnSecondaryMouseDown` callback on the terminal is not strictly needed for the overlay path, but we keep it for completeness in case the terminal is used without the overlay

### 3. Context menu construction (`gui/panes/pane.go`)

- Build a `fyne.NewMenu` with the items listed above
- Query `terminal.HasSelectedText()` to set the `Disabled` field on the Copy menu item
- Show via `widget.NewPopUpMenu(menu, canvas)` at the tap position
- Menu item handlers:
  - Copy: `terminal.CopySelectedText(clipboard)`
  - Paste: `terminal.PasteText(clipboard)`
  - Split Horizontal: pane manager's `SplitHorizontal()`
  - Split Vertical: pane manager's `SplitVertical()`
  - Pause: instance's pause function
  - Kill: instance's kill function

### Files Changed

- `fyne-terminal-fork/mouse.go` — remove right-click paste, suppress auto-copy-on-right-click
- `fyne-terminal-fork/term.go` — add `OnSecondaryMouseDown` callback field
- `fyne-terminal-fork/select.go` — export `CopySelectedText`, `PasteText`, add `HasSelectedText()`
- `gui/panes/pane.go` — add `SecondaryTapped` to `tapOverlay`, construct and show context menu
