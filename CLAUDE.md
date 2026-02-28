# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Vigilo is a macOS-only menu bar application that prevents display and system sleep using IOKit power management assertions. Single-file Go app (`main.go`), package `main`, no sub-packages. Module path: `github.com/ranglust/vigilo`. No cgo — uses `purego` for runtime dlopen/dlsym of macOS frameworks.

## Build & Run

```bash
make build       # go build -o vigilo main.go
make run         # build + run (note: daemonizes immediately, terminal returns)
make clean       # go clean + rm binary
make install     # copy to ~/.local/bin/
make uninstall   # remove from ~/.local/bin/
```

No tests exist. Go version: `go 1.25.1`.

## Dependencies

```
github.com/ebitengine/purego v0.9.0    # Pure Go dlopen/dlsym (no cgo)
github.com/getlantern/systray v1.2.2   # macOS system tray / menu bar
```

`vendor/` is gitignored. After cloning, run `go mod vendor` before building.

## Architecture

### IOKit bindings (`initIOKit()`)

Loads IOKit and CoreFoundation frameworks at runtime via `purego.Dlopen`/`Dlsym`, binding four C functions to Go function variables: `CFStringCreateWithCString`, `CFRelease`, `IOPMAssertionCreateWithName`, `IOPMAssertionRelease`. The `cfstr()` helper converts Go strings to CFStringRef (caller must `CFRelease`).

### Sleep prevention

Two IOPMAssertions (`PreventUserIdleDisplaySleep` + `PreventUserIdleSystemSleep`) are created/released together, tracked by `currentAssertionID`, `currentOSAssertionID`, and `isEnabled`. Assertions are enabled immediately on startup.

### System tray UI (`onReady()`)

Menu items: Toggle (enable/disable sleep prevention), Start on Startup (toggle launchd plist), Quit. Event loop runs in a goroutine with `select` on `ClickedCh` channels.

### Start on startup (`toggleStartOnStartup()`)

Writes/removes `~/Library/LaunchAgents/com.angluster.vigilo.plist` (template embedded from `resources/vigilo.plist` with `%EXEC_LOCATION%` placeholder). The plist has `KeepAlive: true` and `RunAtLoad: true`.

### Self-daemonization and single-instance lock (`main()`)

- **ppid == 1** (launched by launchd): acquires exclusive flock on `/tmp/vigilo.lock` via `acquireLock()`. If lock fails (another instance), exits silently. Otherwise runs systray.
- **ppid != 1** (terminal launch): re-execs itself via `exec.Command`, parent exits immediately. Child is adopted by launchd (ppid becomes 1) and takes the lock path.

### Embedded resources (`//go:embed`)

`resources/on.png`, `resources/off.png` (menu bar icons), `resources/vigilo.plist` (launchd template).

## Gotchas

- **Running from terminal daemonizes**: `./vigilo` or `make run` forks immediately. The parent exits and the child runs under launchd. Logs go to the daemon, not your terminal.
- **`launchctl unload` kills the running process**: Because the plist has `KeepAlive: true`, unloading the launchd job stops the daemon. When toggling "Start on Startup" off, only remove the plist file — do NOT call `launchctl unload`.
- **Vendor not committed**: `vendor/` is in `.gitignore`. Run `go mod vendor` after cloning.
