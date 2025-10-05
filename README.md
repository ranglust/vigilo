# Vigilo

A lightweight macOS menu bar application that prevents your Mac from sleeping.

## Features

- Prevents display sleep
- Prevents system sleep
- Simple toggle interface
- Visual status indicator in menu bar
- Start on startup option
- Minimal resource usage

## Installation

```bash
go install github.com/ranglust/vigilo@latest
```

### Build from source

```bash
go build -o vigilo main.go
```

### Run

```bash
./vigilo
```

The application will appear in your menu bar.

## Usage

1. Click the menu bar icon to open the menu
2. Click "Disable" to turn off sleep prevention
3. Click "Enable" to turn it back on
4. The icon shows "ON" or "OFF" to indicate current state
5. Click "Start on Startup" to automatically launch Vigilo when you log in

The application starts enabled by default.

## Requirements

- macOS
- Go 1.25.1 or later

## Dependencies

- github.com/ebitengine/purego - Pure Go system calls
- github.com/getlantern/systray - System tray functionality

## How it works

Vigilo uses macOS IOKit Power Management APIs to create power assertions that prevent both display and system sleep. When enabled, it maintains two assertions:

- `PreventUserIdleDisplaySleep` - Keeps the display on
- `PreventUserIdleSystemSleep` - Keeps the system awake

## License

See LICENSE file for details.

