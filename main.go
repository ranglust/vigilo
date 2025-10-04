package main

import (
	_ "embed"
	"os"
	"os/exec"
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

	//go:embed icon/on.png
	enabledIcon []byte

	//go:embed icon/off.png
	disabledIcon []byte
)

const (
	kCFStringEncodingUTF8 = 0x08000100
	kIOPMAssertionLevelOn = 255
)

func cfstr(s string) CFStringRef {
	cs := append([]byte(s), 0)
	return CFStringCreateWithCString(0, &cs[0], kCFStringEncodingUTF8)
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

func onReady() {
	initIOKit()

	systray.SetIcon(enabledIcon)
	systray.SetTitle("ON")
	systray.SetTooltip("")

	mToggle := systray.AddMenuItem("Disable", "")
	mQuit := systray.AddMenuItem("Quit", "")

	enableAssertion()

	go func() {
		for {
			select {
			case <-mToggle.ClickedCh:
				if isEnabled {
					disableAssertion()
					systray.SetIcon(disabledIcon)
					systray.SetTitle("OFF")
					mToggle.SetTitle("Enable")
				} else {
					enableAssertion()
					systray.SetIcon(enabledIcon)
					systray.SetTitle("ON")
					mToggle.SetTitle("Disable")
				}
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func onExit() {
	if isEnabled {
		disableAssertion()
	}
}

func main() {
	if syscall.Getppid() == 1 {
		systray.Run(onReady, onExit)
	} else {
		cmd := exec.Command(os.Args[0])
		cmd.Start()
	}
}
