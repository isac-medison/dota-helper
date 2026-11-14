//go:build !windows

package services

import (
	"errors"
	"time"
)

// The lobby scan drives the mouse through Win32; elsewhere it only reports that it can't.

var errLobbyWindowsOnly = errors.New("скан лобі працює лише на Windows")

func withHotkeys(keys []hotkey, fn func(next func(time.Duration) int) error) error {
	return errLobbyWindowsOnly
}

func cursorPos() (int, int, error) { return 0, 0, errLobbyWindowsOnly }

func moveCursor(x, y int) error { return errLobbyWindowsOnly }

func activeWindowCorner() (int, int, bool) { return 0, 0, false }

func click(x, y int, right bool, hover time.Duration) error { return errLobbyWindowsOnly }

func pressKeys(text string) error { return errLobbyWindowsOnly }

func clipboardSequence() uint32 { return 0 }

func clipboardText() string { return "" }

func beep(frequency, milliseconds int) {}
