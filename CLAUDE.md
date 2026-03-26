# Claude Squad IDE

## Building & Installing

This is a **Wails** application. Use `wails build` (not plain `go build`) to compile
the frontend, embed assets, and produce a macOS `.app` bundle:

```sh
wails build
```

The output is `build/bin/claude-squad.app`. Install it and create a CLI symlink:

```sh
cp -R build/bin/claude-squad.app /Applications/
ln -sf /Applications/claude-squad.app/Contents/MacOS/cs ~/.local/bin/cs
```

The `cs` binary is then available on PATH at `~/.local/bin/cs`.

### DMG Installer

To create a `.dmg` for distribution:

```sh
./scripts/build-dmg.sh
```

Output: `build/bin/Claude Squad.dmg`

### Development

For live-reload during development:

```sh
wails dev
```

Do NOT use `go build` or `go install` directly — they skip frontend compilation
and macOS app bundling.
