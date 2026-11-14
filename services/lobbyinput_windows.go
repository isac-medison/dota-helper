//go:build windows

package services

import (
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

// Mouse, keyboard, hotkeys and clipboard through plain Win32 calls: the same input a person makes,
// sent with SendInput. Nothing here touches the game's process.

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procRegisterHotKey       = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey     = user32.NewProc("UnregisterHotKey")
	procPeekMessage          = user32.NewProc("PeekMessageW")
	procGetCursorPos         = user32.NewProc("GetCursorPos")
	procSetCursorPos         = user32.NewProc("SetCursorPos")
	procGetSystemMetrics     = user32.NewProc("GetSystemMetrics")
	procGetForegroundWindow  = user32.NewProc("GetForegroundWindow")
	procGetWindowRect        = user32.NewProc("GetWindowRect")
	procSendInput            = user32.NewProc("SendInput")
	procMapVirtualKey        = user32.NewProc("MapVirtualKeyW")
	procOpenClipboard        = user32.NewProc("OpenClipboard")
	procCloseClipboard       = user32.NewProc("CloseClipboard")
	procGetClipboardData     = user32.NewProc("GetClipboardData")
	procGetClipboardSequence = user32.NewProc("GetClipboardSequenceNumber")
	procSetThreadDpiContext  = user32.NewProc("SetThreadDpiAwarenessContext")
	procGlobalLock           = kernel32.NewProc("GlobalLock")
	procGlobalUnlock         = kernel32.NewProc("GlobalUnlock")
	procGlobalSize           = kernel32.NewProc("GlobalSize")
	procBeep                 = kernel32.NewProc("Beep")
)

const (
	wmHotkey     = 0x0312
	pmRemove     = 0x0001
	modNoRepeat  = 0x4000
	hotkeyIdBase = 0x5D0A

	inputMouse    = 0
	inputKeyboard = 1

	mouseMove        = 0x0001
	mouseVirtualDesk = 0x4000
	mouseAbsolute    = 0x8000

	// GetSystemMetrics: the rectangle of all monitors together.
	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCxVirtualScreen = 78
	smCyVirtualScreen = 79

	mouseLeftDown  = 0x0002
	mouseLeftUp    = 0x0004
	mouseRightDown = 0x0008
	mouseRightUp   = 0x0010

	keyExtended = 0x0001
	keyUp       = 0x0002

	cfUnicodeText     = 13
	maxClipboardChars = 4096
)

// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 is (HANDLE)-4.
const dpiPerMonitorV2 = ^uintptr(3)

type winPoint struct{ x, y int32 }

// MSG, as PeekMessage fills it on 64-bit Windows.
type winMsg struct {
	hwnd     uintptr
	message  uint32
	wParam   uintptr
	lParam   uintptr
	time     uint32
	pt       winPoint
	lPrivate uint32
}

// INPUT with its MOUSEINPUT member (40 bytes on 64-bit Windows).
type mouseInput struct {
	typ       uint32
	_         uint32
	dx, dy    int32
	mouseData uint32
	flags     uint32
	time      uint32
	_         uint32
	extra     uintptr
}

// INPUT with its KEYBDINPUT member, padded to the size of the union.
type keyboardInput struct {
	typ   uint32
	_     uint32
	vk    uint16
	scan  uint16
	flags uint32
	time  uint32
	_     uint32
	extra uintptr
	_     [8]byte
}

// withHotkeys registers the keys system-wide for as long as fn runs and gives it next(): wait up to
// that long and report which key was pressed (its index), or -1. Hotkeys belong to the thread that
// registered them, so everything stays on one OS thread.
func withHotkeys(keys []hotkey, fn func(next func(time.Duration) int) error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Real pixels whatever scaling the display has, so a calibration means the same in every build.
	if procSetThreadDpiContext.Find() == nil {
		if old, _, _ := procSetThreadDpiContext.Call(dpiPerMonitorV2); old != 0 {
			defer procSetThreadDpiContext.Call(old)
		}
	}

	for i, key := range keys {
		r, _, err := procRegisterHotKey.Call(0, uintptr(hotkeyIdBase+i), modNoRepeat, uintptr(key.vk))
		if r == 0 {
			for j := 0; j < i; j++ {
				procUnregisterHotKey.Call(0, uintptr(hotkeyIdBase+j))
			}
			return fmt.Errorf("клавіша %s зайнята іншою програмою: %v", key.name, err)
		}
	}
	defer func() {
		for i := range keys {
			procUnregisterHotKey.Call(0, uintptr(hotkeyIdBase+i))
		}
	}()

	next := func(wait time.Duration) int {
		deadline := time.Now().Add(wait)
		for {
			var m winMsg
			for {
				r, _, _ := procPeekMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0, pmRemove)
				if r == 0 {
					break
				}
				if m.message == wmHotkey {
					if i := int(m.wParam) - hotkeyIdBase; i >= 0 && i < len(keys) {
						return i
					}
				}
			}

			left := time.Until(deadline)
			if left <= 0 {
				return -1
			}
			time.Sleep(min(left, 10*time.Millisecond))
		}
	}
	return fn(next)
}

func cursorPos() (int, int, error) {
	var p winPoint
	if r, _, err := procGetCursorPos.Call(uintptr(unsafe.Pointer(&p))); r == 0 {
		return 0, 0, fmt.Errorf("GetCursorPos: %w", err)
	}
	return int(p.x), int(p.y), nil
}

// activeWindowCorner is the top-left corner of the window that has the focus. Only its place on the
// screen is asked from Windows; nothing of the window itself is read.
func activeWindowCorner() (int, int, bool) {
	window, _, _ := procGetForegroundWindow.Call()
	if window == 0 {
		return 0, 0, false
	}
	var rect struct{ left, top, right, bottom int32 }
	if r, _, _ := procGetWindowRect.Call(window, uintptr(unsafe.Pointer(&rect))); r == 0 {
		return 0, 0, false
	}
	return int(rect.left), int(rect.top), true
}

func moveCursor(x, y int) error {
	if r, _, err := procSetCursorPos.Call(uintptr(int32(x)), uintptr(int32(y))); r == 0 {
		return fmt.Errorf("SetCursorPos: %w", err)
	}
	return nil
}

// glide moves the cursor the way a mouse does, as input the game receives, not only as a new
// position: a game that follows mouse input keeps its hover and its click target in step with it.
// Absolute input is in 1/65536ths of the whole desktop.
func glide(x, y int) error {
	metric := func(index uintptr) int {
		r, _, _ := procGetSystemMetrics.Call(index)
		return int(int32(r))
	}
	left, top := metric(smXVirtualScreen), metric(smYVirtualScreen)
	width, height := metric(smCxVirtualScreen), metric(smCyVirtualScreen)
	if width <= 1 || height <= 1 {
		return moveCursor(x, y)
	}
	dx := ((x-left)*65536 + width/2) / width
	dy := ((y-top)*65536 + height/2) / height
	return sendMouseAt(int32(dx), int32(dy), mouseMove|mouseAbsolute|mouseVirtualDesk)
}

func sendMouse(flags uint32) error {
	return sendMouseAt(0, 0, flags)
}

func sendMouseAt(dx, dy int32, flags uint32) error {
	in := mouseInput{typ: inputMouse, dx: dx, dy: dy, flags: flags}
	if r, _, err := procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in)); r != 1 {
		return fmt.Errorf("SendInput (mouse): %w", err)
	}
	return nil
}

// click arrives next to the point first and then on it, and rests there for the hover time before
// pressing: a UI that only notices a moving cursor (a hover highlight, a menu) then has the element
// under it by the time the click lands.
func click(x, y int, right bool, hover time.Duration) error {
	if err := glide(x+3, y+2); err != nil {
		return err
	}
	time.Sleep(20 * time.Millisecond)
	if err := glide(x, y); err != nil {
		return err
	}
	// Input coordinates round to the nearest pixel at best; this puts the cursor on the exact one.
	if err := moveCursor(x, y); err != nil {
		return err
	}
	time.Sleep(hover)

	down, up := uint32(mouseLeftDown), uint32(mouseLeftUp)
	if right {
		down, up = mouseRightDown, mouseRightUp
	}
	if err := sendMouse(down); err != nil {
		return err
	}
	time.Sleep(60 * time.Millisecond)
	return sendMouse(up)
}

func sendKey(key keyStroke, up bool) error {
	scan, _, _ := procMapVirtualKey.Call(uintptr(key.vk), 0)
	in := keyboardInput{typ: inputKeyboard, vk: key.vk, scan: uint16(scan)}
	if key.extended {
		in.flags |= keyExtended
	}
	if up {
		in.flags |= keyUp
	}
	if r, _, err := procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in)); r != 1 {
		return fmt.Errorf("SendInput (keyboard): %w", err)
	}
	return nil
}

// pressKeys types combinations like "ctrl+a, ctrl+c": each one held down in order and released in reverse.
func pressKeys(text string) error {
	combos, err := parseKeys(text)
	if err != nil {
		return err
	}
	for i, combo := range combos {
		if i > 0 {
			time.Sleep(80 * time.Millisecond)
		}
		for _, key := range combo {
			if err := sendKey(key, false); err != nil {
				return err
			}
			time.Sleep(20 * time.Millisecond)
		}
		for j := len(combo) - 1; j >= 0; j-- {
			if err := sendKey(combo[j], true); err != nil {
				return err
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	return nil
}

// clipboardSequence changes every time anything is copied.
func clipboardSequence() uint32 {
	r, _, _ := procGetClipboardSequence.Call()
	return uint32(r)
}

func clipboardText() string {
	// Whoever copied may still hold the clipboard for a moment.
	opened := false
	for try := 0; try < 20 && !opened; try++ {
		if r, _, _ := procOpenClipboard.Call(0); r != 0 {
			opened = true
		} else {
			time.Sleep(15 * time.Millisecond)
		}
	}
	if !opened {
		return ""
	}
	defer procCloseClipboard.Call()

	handle, _, _ := procGetClipboardData.Call(cfUnicodeText)
	if handle == 0 {
		return ""
	}
	address, _, _ := procGlobalLock.Call(handle)
	if address == 0 {
		return ""
	}
	defer procGlobalUnlock.Call(handle)

	size, _, _ := procGlobalSize.Call(handle)
	chars := min(int(size/2), maxClipboardChars)
	if chars == 0 {
		return ""
	}
	// The block is the OS's memory, not Go's: read through the address without turning it into a
	// Go-managed pointer on the way.
	text := unsafe.Slice((*uint16)(*(*unsafe.Pointer)(unsafe.Pointer(&address))), chars)
	return syscall.UTF16ToString(text)
}

// beep is the only feedback while the game is on screen; it doesn't hold up what comes next.
func beep(frequency, milliseconds int) {
	go procBeep.Call(uintptr(frequency), uintptr(milliseconds))
}
