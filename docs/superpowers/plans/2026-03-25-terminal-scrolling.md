# Terminal Pane Scrolling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add mouse wheel and keyboard scrolling to terminal panes via tmux copy-mode.

**Architecture:** Intercept Fyne scroll events on the pane overlay and keyboard hotkeys, translate them to SGR mouse wheel escape sequences or PageUp/PageDown sequences written to the tmux PTY, and let tmux handle scrollback via its copy-mode. Also add SGR mouse report parsing to the terminal widget's escape handler so incoming mouse reports from tmux don't produce garbage output.

**Tech Stack:** Go, Fyne v2, fyne-terminal-fork, tmux

---

### Task 1: Add SGR Mouse Report Parsing to Escape Handler

Prevent garbage output when tmux sends SGR mouse report sequences back through the PTY. The escape parser currently doesn't recognize the `<` prefix used in SGR encoding.

**Files:**
- Modify: `fyne-terminal-fork/output.go:226-236` (parseEscape — add `<` to continue chars)
- Modify: `fyne-terminal-fork/escape.go:37-49` (handleEscape — consume SGR mouse reports)

- [ ] **Step 1: Add `<` to the escape parser continue set**

In `fyne-terminal-fork/output.go`, modify `parseEscape` so that `<` is accumulated rather than treated as a terminator:

```go
func (t *Terminal) parseEscape(r rune) {
	t.state.code += string(r)
	if (r < '0' || r > '9') && r != ';' && r != '=' && r != '?' && r != '>' && r != '<' {
		code := t.state.code
		fyne.Do(func() {
			t.handleEscape(code)
		})
		t.state.code = ""
		t.state.esc = noEscape
	}
}
```

- [ ] **Step 2: Silently consume SGR mouse reports in handleEscape**

In `fyne-terminal-fork/escape.go`, add a check at the top of `handleEscape` to silently discard SGR mouse report sequences (codes starting with `<`):

```go
func (t *Terminal) handleEscape(code string) {
	code = trimLeftZeros(code)
	if code == "" {
		return
	}

	// Silently consume SGR mouse reports (CSI < Params M/m).
	// These are sent by tmux when mouse mode is on.
	if len(code) > 1 && code[0] == '<' {
		return
	}

	runes := []rune(code)
	if esc, ok := escapes[runes[len(code)-1]]; ok {
		esc(t, code[:len(code)-1])
	} else if t.debug {
		log.Println("Unrecognised Escape:", code)
	}
}
```

- [ ] **Step 3: Build and verify**

Run: `CGO_ENABLED=1 go build -o ~/.local/bin/cs .`
Expected: Builds successfully with no errors.

- [ ] **Step 4: Commit**

```bash
git add fyne-terminal-fork/output.go fyne-terminal-fork/escape.go
git commit -m "feat: add SGR mouse report parsing to terminal escape handler"
```

---

### Task 2: Add ScrollUp/ScrollDown Methods to Terminal Widget

Expose methods on the Terminal type that write SGR mouse wheel escape sequences to the PTY.

**Files:**
- Modify: `fyne-terminal-fork/input.go` (add ScrollUp/ScrollDown methods)

- [ ] **Step 1: Add scroll methods to input.go**

Add the following methods at the end of `fyne-terminal-fork/input.go`:

```go
// ScrollUp writes SGR mouse wheel up escape sequences to the PTY.
// Each call sends `lines` individual scroll-up events.
// Tmux interprets these to enter/navigate copy-mode.
func (t *Terminal) ScrollUp(lines int) {
	for i := 0; i < lines; i++ {
		// SGR encoding: button 64 = wheel up, col 1, row 1
		_, _ = t.in.Write([]byte("\x1b[<64;1;1M"))
	}
}

// ScrollDown writes SGR mouse wheel down escape sequences to the PTY.
// Each call sends `lines` individual scroll-down events.
func (t *Terminal) ScrollDown(lines int) {
	for i := 0; i < lines; i++ {
		// SGR encoding: button 65 = wheel down, col 1, row 1
		_, _ = t.in.Write([]byte("\x1b[<65;1;1M"))
	}
}
```

- [ ] **Step 2: Build and verify**

Run: `CGO_ENABLED=1 go build -o ~/.local/bin/cs .`
Expected: Builds successfully.

- [ ] **Step 3: Commit**

```bash
git add fyne-terminal-fork/input.go
git commit -m "feat: add ScrollUp/ScrollDown methods to terminal widget"
```

---

### Task 3: Implement Hover-Based Mouse Wheel Scrolling on Pane Overlay

Make `tapOverlay` implement the `Scrollable` interface and wire it to the terminal's scroll methods via a callback.

**Files:**
- Modify: `gui/panes/pane.go:63-94` (tapOverlay — add onScroll callback and Scrollable interface)
- Modify: `gui/panes/pane.go:128-139` (NewPane — wire onScroll callback)

- [ ] **Step 1: Add onScroll callback and Scrollable interface to tapOverlay**

In `gui/panes/pane.go`, add the `onScroll` field to `tapOverlay` and implement the `Scrolled` method:

```go
type tapOverlay struct {
	widget.BaseWidget
	onTap          func()
	onSecondaryTap func(*fyne.PointEvent)
	onScroll       func(dy float32)
	focusable      fyne.Focusable
	canvas         fyne.Canvas
}

// Scrolled implements fyne.Scrollable for hover-based mouse wheel scrolling.
func (t *tapOverlay) Scrolled(ev *fyne.ScrollEvent) {
	if t.onScroll != nil {
		t.onScroll(ev.Scrolled.DY)
	}
}
```

- [ ] **Step 2: Wire the onScroll callback in NewPane**

In `gui/panes/pane.go`, after the `p.overlay.onSecondaryTap = ...` block (around line 139), add:

```go
const scrollLines = 3

p.overlay.onScroll = func(dy float32) {
	term := p.conn.Terminal()
	if term == nil {
		return
	}
	if dy > 0 {
		term.ScrollUp(scrollLines)
	} else if dy < 0 {
		term.ScrollDown(scrollLines)
	}
}
```

- [ ] **Step 3: Build and verify**

Run: `CGO_ENABLED=1 go build -o ~/.local/bin/cs .`
Expected: Builds successfully.

- [ ] **Step 4: Manual test**

1. Run `cs gui`
2. Open a session in a pane
3. Generate some output (e.g., run `seq 1 200` in the terminal)
4. Hover over the pane and scroll up with the mouse wheel/trackpad
5. Verify tmux enters copy-mode and you can see earlier output
6. Scroll down to return to the bottom
7. Verify that hovering over a different pane and scrolling only affects that pane

- [ ] **Step 5: Commit**

```bash
git add gui/panes/pane.go
git commit -m "feat: add hover-based mouse wheel scrolling to terminal panes"
```

---

### Task 4: Add Keyboard Scroll Hotkeys

Add Ctrl+Shift+PageUp/PageDown hotkeys that send PageUp/PageDown escape sequences to the focused pane's terminal. Note: bare PageUp/PageDown already works when the terminal has keyboard focus (handled by `TypedKey` in `input.go:73-76`). These hotkeys ensure scrolling works even when focus is elsewhere.

**Files:**
- Modify: `gui/panes/pane.go` (add `SendPageUp`/`SendPageDown` methods on `*Pane`)
- Modify: `gui/hotkeys.go:20-37` (Handlers struct — add scroll callbacks)
- Modify: `gui/hotkeys.go:51-69` (buildShortcuts — add PageUp/PageDown entries)
- Modify: `gui/app.go` (wire scroll handlers to pane manager)

- [ ] **Step 1: Add PageUp/PageDown methods on Pane**

In `gui/panes/pane.go`, add methods that write PageUp/PageDown escape sequences directly to the terminal PTY. This is cleaner than sending N mouse wheel events — a single sequence triggers tmux copy-mode page scroll.

```go
// SendPageUp writes a PageUp escape sequence to the terminal PTY.
// Tmux interprets this to enter/scroll copy-mode one page up.
func (p *Pane) SendPageUp() {
	if term := p.conn.Terminal(); term != nil {
		term.Write([]byte("\x1b[5~"))
	}
}

// SendPageDown writes a PageDown escape sequence to the terminal PTY.
// Tmux interprets this to scroll copy-mode one page down.
func (p *Pane) SendPageDown() {
	if term := p.conn.Terminal(); term != nil {
		term.Write([]byte("\x1b[6~"))
	}
}
```

- [ ] **Step 2: Add scroll handlers to Handlers struct**

In `gui/hotkeys.go`, add to the `Handlers` struct:

```go
type Handlers struct {
	// ... existing fields ...
	ScrollPageUp   func()
	ScrollPageDown func()
}
```

- [ ] **Step 3: Add shortcuts to buildShortcuts**

In `gui/hotkeys.go`, add to the `buildShortcuts` return slice:

```go
{fyne.KeyPageUp, h.ScrollPageUp},
{fyne.KeyPageDown, h.ScrollPageDown},
```

- [ ] **Step 4: Wire scroll handlers in app.go**

In `gui/app.go`, add to the `Handlers` struct literal (around line 148):

```go
ScrollPageUp: func() {
	if p := paneManager.FocusedPane(); p != nil {
		p.SendPageUp()
	}
},
ScrollPageDown: func() {
	if p := paneManager.FocusedPane(); p != nil {
		p.SendPageDown()
	}
},
```

- [ ] **Step 5: Build and verify**

Run: `CGO_ENABLED=1 go build -o ~/.local/bin/cs .`
Expected: Builds successfully.

- [ ] **Step 6: Manual test**

1. Run `cs gui`
2. Open a session, generate output
3. Press Ctrl+Shift+PageUp (or Cmd+Shift+PageUp on macOS)
4. Verify tmux enters copy-mode and scrolls up a full page
5. Press Ctrl+Shift+PageDown to scroll back down
6. Verify pressing `q` or `Escape` exits copy-mode

- [ ] **Step 7: Commit**

```bash
git add gui/panes/pane.go gui/hotkeys.go gui/app.go
git commit -m "feat: add keyboard scroll hotkeys (Ctrl+Shift+PageUp/Down)"
```

---

### Task 5: Update Hint Text and Final Verification

Update the pane header hint text to mention scroll capability, and do a final integration test.

**Files:**
- Modify: `gui/panes/pane.go:315-321` (hintText — add scroll hint)

- [ ] **Step 1: Update hint text**

In `gui/panes/pane.go`, update `hintText()` to mention scrolling:

```go
func hintText() string {
	mod := "Ctrl+Shift"
	if runtime.GOOS == "darwin" {
		mod = "⌘⇧"
	}
	return fmt.Sprintf("Split: %s+\\  %s+-  |  Close: %s+W  |  Nav: %s+←→↑↓  |  Scroll: mousewheel or %s+PgUp/Dn", mod, mod, mod, mod, mod)
}
```

- [ ] **Step 2: Final integration test**

1. Run `cs gui`
2. Open multiple sessions in split panes
3. Test mouse wheel scrolling on each pane (hover, don't click first)
4. Test keyboard page up/down on the focused pane
5. Verify no garbage characters appear in any terminal
6. Verify new output arrives correctly while scrolled (stays at scroll position)
7. Verify exiting copy-mode with `q` or `Escape` works

- [ ] **Step 3: Commit**

```bash
git add gui/panes/pane.go
git commit -m "feat: update pane hint text with scroll instructions"
```
