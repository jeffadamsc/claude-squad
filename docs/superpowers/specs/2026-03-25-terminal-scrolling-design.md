# Terminal Pane Scrolling via Tmux Copy-Mode

## Problem

Terminal panes in the GUI display claude instance output but have no scrollback capability. When there's a lot of text, users cannot scroll up to read earlier content. The forked fyne-terminal widget only stores visible rows with no history buffer.

## Solution

Leverage tmux's built-in copy-mode and scrollback buffer. Intercept mouse wheel and keyboard events in the GUI, forward them as escape sequences to the tmux PTY, and let tmux handle scrollback rendering natively.

## Design

### Mouse Wheel Scrolling (Hover-Based)

The `tapOverlay` on each pane implements Fyne's `Scrollable` interface (`Scrolled(*fyne.ScrollEvent)`). When the user scrolls over any pane — regardless of focus state — the overlay routes the event to that pane's terminal connection via an `onScroll func(deltaY float32)` callback, consistent with the existing `onTap` and `onSecondaryTap` pattern.

The `Pane` sets this callback to call through to the terminal's scroll methods. The terminal widget exposes `ScrollUp(lines int)` and `ScrollDown(lines int)` methods that write SGR-encoded mouse wheel escape sequences **to the tmux PTY input** (not parsed by the fyne-terminal widget's own escape handler):

- Scroll up: `\x1b[<64;Col;RowM`
- Scroll down: `\x1b[<65;Col;RowM`

These are standard xterm SGR mouse encoding (mode 1006). Tmux with `mouse on` interprets these sequences on its input side. Scrolling up enters copy-mode automatically and scrolls through the scrollback buffer. Scrolling down moves forward, and copy-mode exits when reaching the bottom.

Each mouse wheel tick sends 3 lines of scroll. This is a constant in the `ScrollUp`/`ScrollDown` call site in `pane.go` and can be tuned easily.

### Incoming SGR Mouse Reports

When tmux has `mouse on`, it may send SGR mouse report escape sequences *back* through the PTY to the terminal widget. The fyne-terminal fork currently only handles X10 (mode 9) and VT200 (mode 1000) mouse encoding in its escape parser. Unrecognized SGR sequences from tmux could produce garbage characters.

To handle this, add SGR mouse mode 1006 parsing support to the escape handler in `escape.go`. The parser should recognize `\x1b[<...M` and `\x1b[<...m` sequences and silently consume them (the GUI doesn't need to act on mouse reports from tmux — it already knows where the mouse is).

### Keyboard Scrolling

New hotkeys registered in the hotkey system:

| Hotkey | Action |
|--------|--------|
| Ctrl+Shift+PageUp | Scroll up one page in focused pane |
| Ctrl+Shift+PageDown | Scroll down one page in focused pane |

The hotkey handler resolves the focused pane via the pane manager's focus tracking and writes PageUp/PageDown escape sequences (`ESC[5~` / `ESC[6~`) to that pane's terminal PTY. Tmux interprets these to enter/navigate copy-mode. This is more natural than sending mouse wheel sequences for keyboard-triggered scrolling.

### Tmux Configuration

Tmux sessions already have mouse mode enabled via session-scoped `set-option -t <session> mouse on` in `session/tmux/tmux.go:145`. No change needed here.

### Copy-Mode UX Note

When the user scrolls up and tmux enters copy-mode, typing does not go to the shell — tmux intercepts it for copy-mode navigation/search. The user exits copy-mode by pressing `q` or `Escape`, or by scrolling back to the bottom. This is standard tmux behavior and does not need special handling, but it's worth noting as a potential point of confusion.

## Files Changed

| File | Change |
|------|--------|
| `gui/panes/pane.go` | Add `onScroll` callback to `tapOverlay`; implement `Scrollable` interface; `Pane` wires callback to terminal scroll methods |
| `fyne-terminal-fork/input.go` | Add `ScrollUp(lines int)` / `ScrollDown(lines int)` methods on `*Terminal` that write SGR mouse wheel escape sequences to PTY |
| `fyne-terminal-fork/escape.go` | Add SGR mouse mode 1006 parsing to silently consume incoming `\x1b[<...M` / `\x1b[<...m` sequences |
| `gui/hotkeys.go` | Add Ctrl+Shift+PageUp/PageDown hotkeys; handler resolves focused pane and writes PageUp/PageDown escape sequences to its PTY |

## Scroll Event Flow

```
Mouse wheel on pane
  -> tapOverlay.Scrolled(event)
  -> onScroll callback (set by Pane)
  -> terminal.ScrollUp(lines) or terminal.ScrollDown(lines)
  -> write SGR mouse wheel escape sequence to tmux PTY input
  -> tmux receives sequence, enters/manages copy-mode
  -> tmux re-renders terminal content with scrolled view
  -> fyne-terminal widget displays updated content
```

## Edge Cases

- **New output while scrolled**: Tmux copy-mode stays at the scrolled position. New content appends below. The user remains at their scroll position until they scroll back to the bottom or press `q`/`Escape` to exit copy-mode.
- **Multiple panes**: Hover-based routing ensures only the pane under the cursor receives scroll events. No cross-talk between panes.
- **Pane resize while in copy-mode**: Tmux handles re-rendering the scrollback at the new size.
- **Typing while in copy-mode**: Tmux intercepts keystrokes for copy-mode navigation. User must exit copy-mode (`q`/`Escape`) to resume typing to the shell.

## Non-Goals

- Custom scrollback buffer in the terminal widget (future enhancement)
- Scrollbar UI element
- Search within scrollback (tmux copy-mode supports `/` search natively)
