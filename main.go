package main

import (
	"bufio"
	_ "embed"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ebitengine/purego"
	"github.com/getlantern/systray"
)

type (
	CFStringRef        uintptr
	IOPMAssertionID    uint32
	IOPMAssertionLevel uint32
)

var (
	CFStringCreateWithCString   func(alloc uintptr, cstr *byte, encoding uint32) CFStringRef
	CFRelease                   func(cf uintptr)
	IOPMAssertionCreateWithName func(assertType CFStringRef, level IOPMAssertionLevel, name CFStringRef, id *IOPMAssertionID) uint32
	IOPMAssertionRelease        func(id IOPMAssertionID) uint32

	currentAssertionID   IOPMAssertionID
	currentOSAssertionID IOPMAssertionID
	isEnabled            bool

	//go:embed resources/on.png
	enabledIcon []byte

	//go:embed resources/off.png
	disabledIcon []byte

	//go:embed resources/vigilo.plist
	plist []byte
)

const (
	kCFStringEncodingUTF8 = 0x08000100
	kIOPMAssertionLevelOn = 255
	socketPath            = "/tmp/vigilo.sock"
)

func cfstr(s string) CFStringRef {
	cs := append([]byte(s), 0)
	return CFStringCreateWithCString(0, &cs[0], kCFStringEncodingUTF8)
}

func hideFromDock() {
	objc, _ := purego.Dlopen("/usr/lib/libobjc.A.dylib", purego.RTLD_NOW)
	purego.Dlopen("/System/Library/Frameworks/AppKit.framework/AppKit", purego.RTLD_NOW)

	getClassSym, _ := purego.Dlsym(objc, "objc_getClass")
	selRegSym, _ := purego.Dlsym(objc, "sel_registerName")
	msgSendSym, _ := purego.Dlsym(objc, "objc_msgSend")

	var objcGetClass func(name *byte) uintptr
	purego.RegisterFunc(&objcGetClass, getClassSym)

	var selRegisterName func(name *byte) uintptr
	purego.RegisterFunc(&selRegisterName, selRegSym)

	var msgSend func(id uintptr, sel uintptr) uintptr
	purego.RegisterFunc(&msgSend, msgSendSym)

	var msgSendInt func(id uintptr, sel uintptr, arg uintptr) uintptr
	purego.RegisterFunc(&msgSendInt, msgSendSym)

	cls := append([]byte("NSApplication"), 0)
	sel1 := append([]byte("sharedApplication"), 0)
	sel2 := append([]byte("setActivationPolicy:"), 0)

	nsApp := msgSend(objcGetClass(&cls[0]), selRegisterName(&sel1[0]))
	msgSendInt(nsApp, selRegisterName(&sel2[0]), 1) // NSApplicationActivationPolicyAccessory
}

func initIOKit() {
	iokit, _ := purego.Dlopen("/System/Library/Frameworks/IOKit.framework/IOKit", purego.RTLD_NOW)
	cf, _ := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_NOW)

	cfStringCreate, _ := purego.Dlsym(cf, "CFStringCreateWithCString")
	cfRelease, _ := purego.Dlsym(cf, "CFRelease")
	assertCreate, _ := purego.Dlsym(iokit, "IOPMAssertionCreateWithName")
	assertRelease, _ := purego.Dlsym(iokit, "IOPMAssertionRelease")

	purego.RegisterFunc(&CFStringCreateWithCString, cfStringCreate)
	purego.RegisterFunc(&CFRelease, cfRelease)
	purego.RegisterFunc(&IOPMAssertionCreateWithName, assertCreate)
	purego.RegisterFunc(&IOPMAssertionRelease, assertRelease)
}

func enableAssertion() {
	if isEnabled {
		return
	}

	typ := cfstr("PreventUserIdleDisplaySleep")
	name := cfstr("Vigilo - Preventing Display Sleep")
	defer CFRelease(uintptr(typ))
	defer CFRelease(uintptr(name))
	IOPMAssertionCreateWithName(typ, kIOPMAssertionLevelOn, name, &currentAssertionID)

	typ2 := cfstr("PreventUserIdleSystemSleep")
	name2 := cfstr("Vigilo - Preventing System Sleep")
	defer CFRelease(uintptr(typ2))
	defer CFRelease(uintptr(name2))
	IOPMAssertionCreateWithName(typ2, kIOPMAssertionLevelOn, name2, &currentOSAssertionID)

	isEnabled = true
}

func disableAssertion() {
	if !isEnabled {
		return
	}

	if currentAssertionID != 0 {
		IOPMAssertionRelease(currentAssertionID)
		currentAssertionID = 0
	}

	if currentOSAssertionID != 0 {
		IOPMAssertionRelease(currentOSAssertionID)
		currentOSAssertionID = 0
	}

	isEnabled = false
}

func setSleepPrevention(on bool, mToggle *systray.MenuItem) {
	if on {
		enableAssertion()
		systray.SetIcon(enabledIcon)
		systray.SetTitle("ON")
		mToggle.SetTitle("Disable")
	} else {
		disableAssertion()
		systray.SetIcon(disabledIcon)
		systray.SetTitle("OFF")
		mToggle.SetTitle("Enable")
	}
}

func setStartOnStartup(on bool, mStartOnStartup *systray.MenuItem) {
	homeDir, _ := os.UserHomeDir()
	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", "com.angluster.vigilo.plist")

	if on {
		execPath, err := os.Executable()
		if err != nil {
			return
		}
		plistContent := strings.Replace(string(plist), "%EXEC_LOCATION%", execPath, -1)
		os.MkdirAll(filepath.Dir(plistPath), 0755)
		os.WriteFile(plistPath, []byte(plistContent), 0644)
		mStartOnStartup.SetTitle("✓ Start on Startup")
	} else {
		os.Remove(plistPath)
		mStartOnStartup.SetTitle("Start on Startup")
	}
}

func isStartOnStartupEnabled() bool {
	homeDir, _ := os.UserHomeDir()
	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", "com.angluster.vigilo.plist")
	_, err := os.Stat(plistPath)
	return err == nil
}

func startCommandListener(mToggle, mStartOnStartup *systray.MenuItem) {
	os.Remove(socketPath)

	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
	if err != nil {
		return
	}

	go func() {
		defer listener.Close()
		for {
			conn, err := listener.AcceptUnix()
			if err != nil {
				return
			}
			go handleConnection(conn, mToggle, mStartOnStartup)
		}
	}()
}

func handleConnection(conn *net.UnixConn, mToggle, mStartOnStartup *systray.MenuItem) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return
	}
	cmd := strings.TrimSpace(scanner.Text())

	var response string
	switch cmd {
	case "on":
		setSleepPrevention(true, mToggle)
		response = "sleep prevention enabled"
	case "off":
		setSleepPrevention(false, mToggle)
		response = "sleep prevention disabled"
	case "enable":
		setStartOnStartup(true, mStartOnStartup)
		response = "start on startup enabled"
	case "disable":
		setStartOnStartup(false, mStartOnStartup)
		response = "start on startup disabled"
	case "status":
		sleepStatus := "off"
		if isEnabled {
			sleepStatus = "on"
		}
		startupStatus := "disabled"
		if isStartOnStartupEnabled() {
			startupStatus = "enabled"
		}
		response = fmt.Sprintf("sleep prevention: %s\nstart on startup: %s", sleepStatus, startupStatus)
	case "stop":
		fmt.Fprintln(conn, "daemon stopped")
		conn.Close()
		systray.Quit()
		return
	default:
		response = "unknown command: " + cmd
	}

	fmt.Fprintln(conn, response)
}

func onReady() {
	initIOKit()

	systray.SetIcon(enabledIcon)
	systray.SetTitle("ON")
	systray.SetTooltip("")

	mToggle := systray.AddMenuItem("Disable", "")

	var startOnStartupTitle string
	if isStartOnStartupEnabled() {
		startOnStartupTitle = "✓ Start on Startup"
	} else {
		startOnStartupTitle = "Start on Startup"
	}

	mStartOnStartup := systray.AddMenuItem(startOnStartupTitle, "")
	mQuit := systray.AddMenuItem("Quit", "")

	enableAssertion()
	startCommandListener(mToggle, mStartOnStartup)

	go func() {
		for {
			select {
			case <-mToggle.ClickedCh:
				setSleepPrevention(!isEnabled, mToggle)
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			case <-mStartOnStartup.ClickedCh:
				setStartOnStartup(!isStartOnStartupEnabled(), mStartOnStartup)
			}
		}
	}()
}

func onExit() {
	os.Remove(socketPath)
	if isEnabled {
		disableAssertion()
	}
}

func sendCommand(cmd string) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "vigilo is not running")
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Fprintln(conn, cmd)

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
}

func acquireLock() (*os.File, bool) {
	lockPath := filepath.Join(os.TempDir(), "vigilo.lock")
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, false
	}

	err = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		lockFile.Close()
		return nil, false
	}

	return lockFile, true
}

func printUsage() {
	fmt.Println(`Usage: vigilo <command>

Commands:
  serve     Start the daemon (menu bar + socket listener)
  on        Enable sleep prevention
  off       Disable sleep prevention
  enable    Enable start on startup
  disable   Disable start on startup
  status    Show current status
  stop      Stop the daemon`)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	switch os.Args[1] {
	case "serve":
		if syscall.Getppid() != 1 {
			exe, err := os.Executable()
			if err != nil {
				fmt.Fprintln(os.Stderr, "failed to find executable:", err)
				os.Exit(1)
			}
			cmd := exec.Command(exe, "serve")
			if err := cmd.Start(); err != nil {
				fmt.Fprintln(os.Stderr, "failed to start daemon:", err)
				os.Exit(1)
			}
			return
		}
		lockFile, acquired := acquireLock()
		if !acquired {
			fmt.Fprintln(os.Stderr, "another instance is already running")
			os.Exit(1)
		}
		defer lockFile.Close()
		hideFromDock()
		systray.Run(onReady, onExit)
	case "on", "off", "enable", "disable", "status", "stop":
		sendCommand(os.Args[1])
	default:
		printUsage()
		os.Exit(1)
	}
}
