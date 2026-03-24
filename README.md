# Claude Squad

A native desktop app that manages multiple [Claude Code](https://github.com/anthropics/claude-code), [Codex](https://github.com/openai/codex), [Gemini](https://github.com/google-gemini/gemini-cli) (and other local agents including [Aider](https://github.com/Aider-AI/aider)) in separate workspaces, allowing you to work on multiple tasks simultaneously.

### Highlights
- Run multiple AI agent sessions side by side in a native GUI
- Split panes to view several sessions at once
- Each task gets its own isolated git workspace (worktree), so no conflicts
- Auto-accept mode for hands-off background work
- Create sessions with a prompt, branch, and profile in one dialog

### Installation

Build from source:

```bash
git clone https://github.com/jeffadamsc/claude-squad.git
cd claude-squad
go build -o cs .
```

Place the `cs` binary somewhere on your `$PATH` (e.g. `~/.local/bin`).

### Prerequisites

- [Go](https://go.dev/dl/) 1.21+ (to build)
- [tmux](https://github.com/tmux/tmux/wiki/Installing) (used under the hood for agent terminal sessions)

### Usage

```
Usage:
  cs [flags]
  cs [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  debug       Print debug information like config paths
  help        Help about any command
  reset       Reset all stored instances
  version     Print the version number of claude-squad

Flags:
  -y, --autoyes          [experimental] If enabled, all instances will automatically accept prompts
  -h, --help             help for claude-squad
  -p, --program string   Program to run in new instances (e.g. 'aider --model ollama_chat/gemma3:1b')
```

Launch the app from inside a git repository:

```bash
cs
```

The default program is `claude`. You can override it with `-p` or by configuring profiles (see below).

**Using other AI assistants:**
- Codex: `cs -p "codex"` (set `OPENAI_API_KEY` first)
- Aider: `cs -p "aider ..."`
- Gemini: `cs -p "gemini"`

### Keyboard Shortcuts

All shortcuts use **Cmd+Shift** (macOS) or **Ctrl+Shift** (Linux/Windows) as the modifier.

| Shortcut | Action |
|----------|--------|
| `N` | New session |
| `\` | Split pane vertically |
| `-` | Split pane horizontally |
| `W` | Close focused pane |
| `Arrow keys` | Navigate between panes |
| `J` / `K` | Move down / up in the sidebar session list |
| `Enter` | Open selected session in the focused pane |
| `D` | Kill (delete) session |
| `P` | Push changes |
| `R` | Pause / resume session |
| `B` | Toggle sidebar visibility |
| `Q` | Quit |

### Configuration

Claude Squad stores its configuration in `~/.claude-squad/config.json`. You can find the exact path by running `cs debug`.

#### Profiles

Profiles let you define multiple named program configurations and switch between them when creating a new session. The new session dialog shows a profile dropdown when more than one profile is defined.

```json
{
  "default_program": "claude",
  "profiles": [
    { "name": "claude", "program": "claude" },
    { "name": "codex", "program": "codex" },
    { "name": "aider", "program": "aider --model ollama_chat/gemma3:1b" }
  ]
}
```

| Field     | Description                                              |
|-----------|----------------------------------------------------------|
| `name`    | Display name shown in the profile picker                 |
| `program` | Shell command used to launch the agent for that profile  |

If no profiles are defined, Claude Squad uses `default_program` directly as the launch command (the default is `claude`).

### How It Works

1. **tmux** to create isolated terminal sessions for each agent
2. **git worktrees** to isolate codebases so each session works on its own branch
3. A native GUI (Fyne) with embedded terminal panes for viewing and interacting with sessions

### License

[AGPL-3.0](LICENSE.md)
