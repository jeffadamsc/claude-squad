# Terminal Pane Context Menu Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a right-click context menu to terminal panes with Copy, Paste, Split, Pause, and Kill actions.

**Architecture:** The `tapOverlay` in each pane intercepts right-clicks via `SecondaryTapped` and builds a `widget.PopUpMenu`. Terminal clipboard methods are exported so the pane package can call them. Action callbacks (split, pause, kill) are passed from app.go through the Manager to each Pane.

**Tech Stack:** Go, Fyne v2 (`widget.PopUpMenu`, `fyne.NewMenu`)

---

### Task 1: Export terminal clipboard methods

**Files:**
- Modify: `fyne-terminal-fork/select.go:65-90`

- [ ] **Step 1: Rename `copySelectedText` to `CopySelectedText`**

In `select.go`, rename the method and update all call sites within the package:

```go
func (t *Terminal) CopySelectedText(clipboard fyne.Clipboard) {
	text := t.SelectedText()
	clipboard.SetContent(text)
	t.clearSelectedText()
}
```

- [ ] **Step 2: Rename `pasteText` to `PasteText`**

```go
func (t *Terminal) PasteText(clipboard fyne.Clipboard) {
	content := clipboard.Content()
	if t.bracketedPasteMode {
		_, _ = t.in.Write(append(
			append(
				[]byte{asciiEscape, '[', '2', '0', '0', '~'},
				[]byte(content)...),
			[]byte{asciiEscape, '[', '2', '0', '1', '~'}...),
		)
		return
	}
	_, _ = t.in.Write([]byte(content))
}
```

- [ ] **Step 3: Rename `hasSelectedText` to `HasSelectedText`**

```go
func (t *Terminal) HasSelectedText() bool {
	return t.selStart != nil && t.selEnd != nil
}
```

- [ ] **Step 4: Update all in-package call sites**

In `term.go:137-142` (MouseDown), update to use the new exported names:
- `t.hasSelectedText()` → `t.HasSelectedText()`
- `t.copySelectedText(...)` → `t.CopySelectedText(...)`
- `t.pasteText(...)` → `t.PasteText(...)`

In `term.go:448-467` (setupShortcuts), update:
- `t.pasteText(...)` → `t.PasteText(...)`
- `t.copySelectedText(...)` → `t.CopySelectedText(...)`

- [ ] **Step 5: Verify it compiles**

Run: `cd /Users/jadams/go/src/bitbucket.org/vervemotion/claude-squad && CGO_ENABLED=1 go build ./...`
Expected: compiles with no errors

- [ ] **Step 6: Commit**

```bash
git add fyne-terminal-fork/select.go fyne-terminal-fork/term.go
git commit -m "refactor: export terminal clipboard methods for use by pane package"
```

---

### Task 2: Remove right-click paste from terminal MouseDown

**Files:**
- Modify: `fyne-terminal-fork/term.go:136-154`

- [ ] **Step 1: Modify MouseDown to skip auto-copy and paste on right-click**

Replace the current `MouseDown` method at `term.go:136-154` with:

```go
func (t *Terminal) MouseDown(ev *desktop.MouseEvent) {
	if ev.Button == desktop.MouseButtonSecondary {
		// Don't auto-copy/clear selection or paste on right-click.
		// The pane's context menu handles copy/paste actions.
		// Still forward to onMouseDown so mouse-aware terminal apps
		// (vim, htop, etc.) receive right-click events via X10/SGR.
		if t.onMouseDown != nil {
			t.onMouseDown(2, ev.Modifier, ev.Position)
		}
		return
	}

	if t.HasSelectedText() {
		t.CopySelectedText(fyne.CurrentApp().Clipboard())
		t.clearSelectedText()
	}

	if t.onMouseDown == nil {
		return
	}

	if ev.Button == desktop.MouseButtonPrimary {
		t.onMouseDown(1, ev.Modifier, ev.Position)
	}
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/jadams/go/src/bitbucket.org/vervemotion/claude-squad && CGO_ENABLED=1 go build ./...`
Expected: compiles with no errors

- [ ] **Step 3: Commit**

```bash
git add fyne-terminal-fork/term.go
git commit -m "fix: remove right-click paste from terminal, defer to context menu"
```

---

### Task 3: Add context menu callbacks to Pane and Manager

**Files:**
- Modify: `gui/panes/pane.go:32-48` (Pane struct), `gui/panes/pane.go:80-124` (NewPane)
- Modify: `gui/panes/manager.go:26-44` (Manager struct, NewManager)

- [ ] **Step 1: Add PaneActions struct and plumb into Pane**

In `pane.go`, add a `PaneActions` struct after the imports/colors and add the field to `Pane`:

```go
// PaneActions provides callbacks for context menu actions that require
// access to the pane manager or application state.
type PaneActions struct {
	SplitHorizontal func()
	SplitVertical   func()
	PauseSession    func(*session.Instance)
	KillSession     func(*session.Instance)
}
```

Add `actions PaneActions` field to the `Pane` struct, and add it as a parameter to `NewPane`:

```go
func NewPane(onFocus func(*Pane), registerKeys ShortcutRegistrar, actions PaneActions, c fyne.Canvas) *Pane {
```

Store it: `actions: actions,` in the struct literal.

- [ ] **Step 2: Update Manager to accept and forward PaneActions**

In `manager.go`, add `actions PaneActions` field to `Manager` struct and update `NewManager`:

```go
func NewManager(onFocus func(*Pane), registerKeys ShortcutRegistrar, actions PaneActions, c fyne.Canvas) *Manager {
	m := &Manager{onFocus: onFocus, registerKeys: registerKeys, actions: actions, canvas: c}
	pane := NewPane(func(p *Pane) {
		m.FocusPane(p)
	}, registerKeys, actions, c)
```

Also update `split()` at `manager.go:96` where new panes are created:

```go
	newPane := NewPane(func(p *Pane) {
		m.FocusPane(p)
	}, m.registerKeys, m.actions, m.canvas)
```

- [ ] **Step 3: Verify it compiles (will fail — app.go not updated yet)**

This step will have a compile error because `app.go` calls `NewManager` with the old signature. That's expected — we fix it in the next step.

- [ ] **Step 4: Update app.go to pass PaneActions**

In `app.go:83-87`, update the `NewManager` call. The `PaneActions` callbacks need to reference `paneManager` and `rootSplit` which are defined later, so we use a closure pattern. Define `paneActions` before `paneManager`:

```go
	var paneManager *panes.Manager
	var rootSplit *container.Split

	paneActions := panes.PaneActions{
		SplitHorizontal: func() {
			paneManager.SplitHorizontal()
			rootSplit.Trailing = paneManager.Widget()
			rootSplit.Refresh()
		},
		SplitVertical: func() {
			paneManager.SplitVertical()
			rootSplit.Trailing = paneManager.Widget()
			rootSplit.Refresh()
		},
		PauseSession: func(inst *session.Instance) {
			togglePauseResume(inst, state, sidebarWidget)
		},
		KillSession: func(inst *session.Instance) {
			killSession(inst, state, sidebarWidget, paneManager)
		},
	}

	paneManager = panes.NewManager(func(p *panes.Pane) {
		// Focus callback
	}, func(target panes.ShortcutAdder) {
		RegisterTerminalShortcuts(target, hotkeyDefs)
	}, paneActions, w.Canvas())
```

The `var rootSplit *container.Split` declaration above means the existing `rootSplit := container.NewHSplit(...)` on line 123 must change to an assignment (remove the `:`):

```go
	rootSplit = container.NewHSplit(sidebarObj, paneManager.Widget())
```

- [ ] **Step 5: Verify it compiles**

Run: `cd /Users/jadams/go/src/bitbucket.org/vervemotion/claude-squad && CGO_ENABLED=1 go build ./...`
Expected: compiles with no errors

- [ ] **Step 6: Commit**

```bash
git add gui/panes/pane.go gui/panes/manager.go gui/app.go
git commit -m "feat: add PaneActions callback struct for context menu wiring"
```

---

### Task 4: Implement context menu on right-click

**Files:**
- Modify: `gui/panes/pane.go`

- [ ] **Step 1: Add SecondaryTapped to tapOverlay**

Add a callback field and the `SecondaryTapped` method to `tapOverlay`:

```go
type tapOverlay struct {
	widget.BaseWidget
	onTap          func()
	onSecondaryTap func(*fyne.PointEvent)
	focusable      fyne.Focusable
	canvas         fyne.Canvas
}
```

```go
func (t *tapOverlay) SecondaryTapped(ev *fyne.PointEvent) {
	if t.onSecondaryTap != nil {
		t.onSecondaryTap(ev)
	}
}
```

- [ ] **Step 2: Add showContextMenu method to Pane**

Add this method to `pane.go`:

```go
func (p *Pane) showContextMenu(ev *fyne.PointEvent) {
	term := p.conn.Terminal()
	inst := p.conn.Instance()

	copyItem := fyne.NewMenuItem("Copy", func() {
		if term != nil {
			term.CopySelectedText(fyne.CurrentApp().Clipboard())
		}
	})
	if term == nil || !term.HasSelectedText() {
		copyItem.Disabled = true
	}

	pasteItem := fyne.NewMenuItem("Paste", func() {
		if term != nil {
			term.PasteText(fyne.CurrentApp().Clipboard())
		}
	})

	splitHItem := fyne.NewMenuItem("Split Horizontal", func() {
		if p.actions.SplitHorizontal != nil {
			p.actions.SplitHorizontal()
		}
	})

	splitVItem := fyne.NewMenuItem("Split Vertical", func() {
		if p.actions.SplitVertical != nil {
			p.actions.SplitVertical()
		}
	})

	pauseItem := fyne.NewMenuItem("Pause Session", func() {
		if inst != nil && p.actions.PauseSession != nil {
			p.actions.PauseSession(inst)
		}
	})
	if inst == nil {
		pauseItem.Disabled = true
	}

	killItem := fyne.NewMenuItem("Kill Session", func() {
		if inst != nil && p.actions.KillSession != nil {
			p.actions.KillSession(inst)
		}
	})
	if inst == nil {
		killItem.Disabled = true
	}

	menu := fyne.NewMenu("",
		copyItem,
		pasteItem,
		fyne.NewMenuItemSeparator(),
		splitHItem,
		splitVItem,
		fyne.NewMenuItemSeparator(),
		pauseItem,
		killItem,
	)

	popUp := widget.NewPopUpMenu(menu, p.canvas)
	popUp.ShowAtPosition(ev.AbsolutePosition)
}
```

This requires importing the `terminal` package. Add to imports:

```go
terminal "github.com/fyne-io/terminal"
```

Note: The `conn.Terminal()` method returns `*terminal.Terminal`. Check the return type matches — if it returns `fyne.CanvasObject` or similar, a type assertion may be needed.

- [ ] **Step 3: Wire up the overlay's onSecondaryTap in NewPane**

In `NewPane`, update the overlay creation at line 110:

```go
	p.overlay = newTapOverlay(c, func() {
		if p.onFocus != nil {
			p.onFocus(p)
		}
	})
	p.overlay.onSecondaryTap = func(ev *fyne.PointEvent) {
		// Focus this pane first so split/pause/kill act on the correct pane
		if p.onFocus != nil {
			p.onFocus(p)
		}
		p.showContextMenu(ev)
	}
```

- [ ] **Step 4: Verify it compiles**

Run: `cd /Users/jadams/go/src/bitbucket.org/vervemotion/claude-squad && CGO_ENABLED=1 go build ./...`
Expected: compiles with no errors

- [ ] **Step 5: Manual test**

Run: `~/.local/bin/cs gui`
1. Open a session in a pane
2. Right-click on the terminal — context menu should appear
3. Select text by dragging, then right-click — "Copy" should be enabled
4. Click "Copy" — text should be copied to clipboard
5. Right-click and click "Paste" — clipboard content should be typed into terminal
6. Click "Split Horizontal" — pane should split top/bottom
7. Click "Split Vertical" — pane should split left/right
8. Click "Pause Session" — session should pause
9. Click "Kill Session" — session should be killed
10. Right-click on empty pane (no session) — Pause and Kill should be grayed out

- [ ] **Step 6: Commit**

```bash
git add gui/panes/pane.go
git commit -m "feat: add right-click context menu to terminal panes

Context menu provides Copy, Paste, Split Horizontal, Split Vertical,
Pause Session, and Kill Session. Copy is disabled when no text is
selected. Pause/Kill are disabled when no session is connected."
```
