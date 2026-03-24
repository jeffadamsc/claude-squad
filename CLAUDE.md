# Claude Squad IDE

## Building & Installing

The `cs` binary on PATH is at `~/.local/bin/cs`. Use `go build` to target it directly:

```sh
CGO_ENABLED=1 go build -o ~/.local/bin/cs .
```

Do NOT use `go install` — it writes to `~/go/bin/cs` which is not on PATH.
