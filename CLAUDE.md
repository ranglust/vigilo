# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Vigilo is a macOS-only menu bar application that prevents display and system sleep using IOKit power management assertions. Single-file Go app (`main.go`), package `main`, no sub-packages. Module path: `github.com/ranglust/vigilo`. Dependencies are vendored in `vendor/`.

## Build & Run

```bash
make build       # go build -o vigilo main.go
make run         # build + run
make clean       # go clean + rm binary
make install     # copy to ~/.local/bin/
make uninstall   # remove from ~/.local/bin/
```

No tests exist.

## Dependencies (exact versions)

```
github.com/ebitengine/purego v0.9.0    # Pure Go dlopen/dlsym (no cgo)
github.com/getlantern/systray v1.2.2   # macOS system tray / menu bar
```

Go version: `go 1.25.1`

## Complete Architecture

### Type definitions

```go
type CFStringRef        uintptr
type IOPMAssertionID    uint32
type IOPMAssertionLevel uint32
```

### Constants

```go
kCFStringEncodingUTF8 = 0x08000100
kIOPMAssertionLevelOn = 255
```

### IOKit binding (`initIOKit()`)

Uses `purego.Dlopen` to load two macOS frameworks at runtime:
- `/System/Library/Frameworks/IOKit.framework/IOKit` — symbols: `IOPMAssertionCreateWithName`, `IOPMAssertionRelease`
- `/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation` — symbols: `CFStringCreateWithCString`, `CFRelease`

These are loaded via `purego.Dlsym` and bound to package-level function variables via `purego.RegisterFunc`:

```go
CFStringCreateWithCString   func(alloc uintptr, cstr *byte, encoding uint32) CFStringRef
CFRelease                   func(cf uintptr)
IOPMAssertionCreateWithName func(assertType CFStringRef, level IOPMAssertionLevel, name CFStringRef, id *IOPMAssertionID) uint32
IOPMAssertionRelease        func(id IOPMAssertionID) uint32
```

### CFString helper (`cfstr()`)

Converts Go string to CFStringRef: appends null byte to `[]byte(s)`, calls `CFStringCreateWithCString(0, &cs[0], kCFStringEncodingUTF8)`. Caller must `CFRelease` the result.

### Sleep prevention

Two IOPMAssertions are created/released together, tracked by separate IDs (`currentAssertionID`, `currentOSAssertionID`) and a single `isEnabled` bool:

| Assertion Type | Human-Readable Name |
|---|---|
| `PreventUserIdleDisplaySleep` | `Vigilo - Preventing Display Sleep` |
| `PreventUserIdleSystemSleep` | `Vigilo - Preventing System Sleep` |

`enableAssertion()`: guards on `isEnabled`, creates both assertions, sets `isEnabled = true`. CFStringRefs are deferred-released.
`disableAssertion()`: guards on `!isEnabled`, releases both assertions (if ID != 0), zeros IDs, sets `isEnabled = false`.

### Embedded resources (`//go:embed`)

Requires blank import `_ "embed"`. Three resources in `resources/`:
- `resources/on.png` → `enabledIcon []byte` — menu bar icon when active
- `resources/off.png` → `disabledIcon []byte` — menu bar icon when inactive
- `resources/vigilo.plist` → `plist []byte` — launchd plist template

### System tray UI (`onReady()`)

Calls `initIOKit()` first. Sets icon to `enabledIcon`, title to `"ON"`, tooltip to `""`.

Menu items in order:
1. **Toggle** — initially `"Disable"`. Clicking toggles between enable/disable, swaps icon, title (`"ON"`/`"OFF"`), and button text (`"Disable"`/`"Enable"`).
2. **Start on Startup** — checks if `~/Library/LaunchAgents/com.angluster.vigilo.plist` exists at startup. Shows `"✓ Start on Startup"` if exists, `"Start on Startup"` if not. Clicking calls `toggleStartOnStartup()` and updates title accordingly.
3. **Quit** — calls `systray.Quit()` and returns from the goroutine.

Assertions are enabled immediately after menu setup. Event loop runs in a goroutine with `select` on `ClickedCh` channels.

`onExit()`: releases assertions if enabled.

### Start on startup (`toggleStartOnStartup()`)

Plist path: `~/Library/LaunchAgents/com.angluster.vigilo.plist`

**Enable**: Gets `os.Executable()` path, replaces `%EXEC_LOCATION%` placeholder in embedded plist template, creates `LaunchAgents` dir (0755), writes plist (0644), runs `launchctl load <path>`. Returns `true`.

**Disable**: Runs `launchctl unload <path>`, removes the plist file. Returns `false`.

Plist template content (label: `com.angluster.vigilo`, RunAtLoad: true, KeepAlive: true, stdout/stderr to `/tmp/vigilo.out` and `/tmp/vigilo.err`):

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>Label</key>
    <string>com.angluster.vigilo</string>
    <key>ProgramArguments</key>
    <array>
      <string>%EXEC_LOCATION%</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/vigilo.out</string>
    <key>StandardErrorPath</key>
    <string>/tmp/vigilo.err</string>
  </dict>
</plist>
```

### Self-daemonization and single-instance lock (`main()`)

**If `syscall.Getppid() == 1`** (launched by launchd/init): acquires an exclusive non-blocking file lock on `/tmp/vigilo.lock` via `syscall.Flock(fd, LOCK_EX|LOCK_NB)`. If lock fails (another instance running), exits silently. Otherwise runs `systray.Run(onReady, onExit)`. Lock file is deferred-closed.

**If ppid != 1** (launched from terminal): re-execs itself as detached process via `exec.Command(os.Args[0]).Start()` and the parent returns immediately. The child will be adopted by launchd (ppid becomes 1) and take the lock path.

### File layout

```
main.go                  # All application code
go.mod / go.sum          # Module definition
Makefile                 # Build targets
resources/on.png         # Enabled icon (embedded)
resources/off.png        # Disabled icon (embedded)
resources/vigilo.plist   # Launchd template (embedded)
vendor/                  # Vendored dependencies
```

### .gitignore

```
vendor
.DS_Store
vigilo
.vscode
```
