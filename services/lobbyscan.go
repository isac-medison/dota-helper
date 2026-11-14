package services

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"dota-helper/models"
)

// LobbyScan clicks through the players of a match the way a person would: a click on the player,
// then whatever opens their profile and copies their ID, and so on for the next one. Where to click
// is recorded once by hand (calibration), because it depends on the screen and on the game's UI.
//
// It only moves the mouse and presses keys; it never reads the game's memory or files.
type LobbyScan struct {
	Http *http.Client

	// One capture or run at a time: both own the mouse and the hotkeys.
	busy   sync.Mutex
	cancel atomic.Bool
}

const (
	lobbyHotkeyTimeout = 3 * time.Minute
	lobbyCountdown     = 3 * time.Second
	// A step never waits longer than this, whatever the calibration says.
	lobbyMaxDelay = 10 * time.Second
	// The cursor rests on a point at least this long before the click.
	lobbyMinHover = 50 * time.Millisecond
	// How long the cursor rests on each point of a dry run: enough to see what is under it.
	lobbyDryRunPause = 900 * time.Millisecond
)

var errLobbyBusy = errors.New("скан лобі або калібровка вже виконується")

// A key registered system-wide while a capture or a run is on (Windows virtual-key code).
type hotkey struct {
	vk   uint32
	name string
}

var (
	// Records a point when calibrating; starts and stops a run.
	hotkeyPoint = hotkey{vk: 0x77, name: "F8"}
	// Ends a recording.
	hotkeyDone = hotkey{vk: 0x78, name: "F9"}
	// Records a point that takes a right click.
	hotkeyRight = hotkey{vk: 0x76, name: "F7"}
)

// One key of a combination (Windows virtual-key code; extended = arrows and the block above them).
type keyStroke struct {
	vk       uint16
	extended bool
}

var namedKeys = map[string]keyStroke{
	"ctrl": {vk: 0x11}, "control": {vk: 0x11}, "shift": {vk: 0x10}, "alt": {vk: 0x12},
	"esc": {vk: 0x1B}, "escape": {vk: 0x1B}, "enter": {vk: 0x0D}, "return": {vk: 0x0D},
	"tab": {vk: 0x09}, "space": {vk: 0x20}, "backspace": {vk: 0x08},
	"delete": {vk: 0x2E, extended: true}, "del": {vk: 0x2E, extended: true},
	"insert": {vk: 0x2D, extended: true}, "home": {vk: 0x24, extended: true}, "end": {vk: 0x23, extended: true},
	"pageup": {vk: 0x21, extended: true}, "pagedown": {vk: 0x22, extended: true},
	"left": {vk: 0x25, extended: true}, "up": {vk: 0x26, extended: true},
	"right": {vk: 0x27, extended: true}, "down": {vk: 0x28, extended: true},
}

// parseKeys reads "ctrl+a, ctrl+c" into combinations of keys: names above, letters, digits and F1–F12.
func parseKeys(text string) ([][]keyStroke, error) {
	var combos [][]keyStroke
	for _, combo := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return r == ',' || r == ';' || r == ' ' }) {
		var keys []keyStroke
		for _, name := range strings.Split(combo, "+") {
			key, ok := namedKeys[name]
			switch {
			case ok:
			case len(name) == 1 && name[0] >= 'a' && name[0] <= 'z':
				key = keyStroke{vk: uint16(name[0] - 'a' + 'A')}
			case len(name) == 1 && name[0] >= '0' && name[0] <= '9':
				key = keyStroke{vk: uint16(name[0])}
			case len(name) >= 2 && name[0] == 'f':
				var n int
				if _, err := fmt.Sscanf(name[1:], "%d", &n); err != nil || n < 1 || n > 12 {
					return nil, fmt.Errorf("невідома клавіша %q", name)
				}
				key = keyStroke{vk: uint16(0x70 + n - 1)}
			default:
				return nil, fmt.Errorf("невідома клавіша %q", name)
			}
			keys = append(keys, key)
		}
		combos = append(combos, keys)
	}
	if len(combos) == 0 {
		return nil, errors.New("крок «клавіші» порожній")
	}
	return combos, nil
}

// checkSteps finds a broken calibration before the mouse starts moving.
func checkSteps(steps []models.LobbyStep) error {
	for i, step := range steps {
		switch step.Kind {
		case "click", "rightClick", "doubleClick", "wait":
		case "keys":
			if _, err := parseKeys(step.Keys); err != nil {
				return fmt.Errorf("крок %d: %w", i+1, err)
			}
		default:
			return fmt.Errorf("крок %d: невідомий тип %q", i+1, step.Kind)
		}
	}
	return nil
}

// Capture waits for a calibration hotkey pressed anywhere (the game keeps the focus): F8 records
// where the cursor is, F7 does the same for a right click, F9 ends the recording.
func (s *LobbyScan) Capture() (*models.LobbyCapture, error) {
	if !s.busy.TryLock() {
		return nil, errLobbyBusy
	}
	defer s.busy.Unlock()
	s.cancel.Store(false)

	var capture *models.LobbyCapture
	err := withHotkeys([]hotkey{hotkeyPoint, hotkeyDone, hotkeyRight}, func(next func(time.Duration) int) error {
		deadline := time.Now().Add(lobbyHotkeyTimeout)
		for time.Now().Before(deadline) {
			if s.cancel.Load() {
				return errors.New("скасовано")
			}
			switch key := next(100 * time.Millisecond); key {
			case 0, 2:
				x, y, err := cursorPos()
				if err != nil {
					return err
				}
				capture = &models.LobbyCapture{Key: "point", X: x, Y: y}
				if wx, wy, ok := activeWindowCorner(); ok {
					capture.Window = &models.LobbyPoint{X: wx, Y: wy}
				}
				if key == 2 {
					capture.Key = "right"
					beep(1000, 60)
				} else {
					beep(1200, 60)
				}
				return nil
			case 1:
				capture = &models.LobbyCapture{Key: "done"}
				beep(700, 120)
				return nil
			}
		}
		return errors.New("не дочекався клавіші")
	})
	return capture, err
}

// Run goes through the players. It starts when F8 is pressed in the game (or after a countdown),
// and F8 again stops it. The result has one entry per player: what the steps left in the clipboard.
func (s *LobbyScan) Run(req models.LobbyScanRequest) (*models.LobbyScanResult, error) {
	if len(req.Players) == 0 || len(req.Steps) == 0 {
		return nil, errors.New("спершу зроби калібровку: немає точок гравців або кроків")
	}
	if err := checkSteps(req.Steps); err != nil {
		return nil, err
	}
	if !s.busy.TryLock() {
		return nil, errLobbyBusy
	}
	defer s.busy.Unlock()
	s.cancel.Store(false)

	result := &models.LobbyScanResult{Raw: make([]string, len(req.Players))}
	err := withHotkeys([]hotkey{hotkeyPoint}, func(next func(time.Duration) int) error {
		// pause waits and reports whether the run should stop (F8 or Cancel).
		pause := func(d time.Duration) bool {
			return next(d) == 0 || s.cancel.Load()
		}

		if req.NoHotkey {
			if pause(lobbyCountdown) {
				result.Aborted = true
				return nil
			}
		} else {
			deadline := time.Now().Add(lobbyHotkeyTimeout)
			for started := false; !started; {
				if s.cancel.Load() {
					result.Aborted = true
					return nil
				}
				if time.Now().After(deadline) {
					return errors.New("не дочекався F8 у грі")
				}
				started = next(100*time.Millisecond) == 0
			}
		}
		beep(900, 80)

		hover := min(max(time.Duration(req.HoverMs)*time.Millisecond, lobbyMinHover), lobbyMaxDelay)

		for i, player := range req.Players {
			before := clipboardSequence()

			for _, step := range req.Steps {
				x, y := step.X, step.Y
				if step.Relative {
					x, y = player.X+step.X, player.Y+step.Y
				} else if step.Window {
					// Wherever the window is now, not where it was when the step was recorded.
					if wx, wy, ok := activeWindowCorner(); ok {
						x, y = wx+step.X, wy+step.Y
					}
				}

				if req.DryRun {
					if step.Kind == "keys" || step.Kind == "wait" {
						continue
					}
					if err := moveCursor(x, y); err != nil {
						return err
					}
					beep(1000, 30)
					if pause(lobbyDryRunPause) {
						result.Aborted = true
						return nil
					}
					continue
				}

				var err error
				switch step.Kind {
				case "click":
					err = click(x, y, false, hover)
				case "rightClick":
					err = click(x, y, true, hover)
				case "doubleClick":
					if err = click(x, y, false, hover); err == nil {
						time.Sleep(90 * time.Millisecond)
						err = click(x, y, false, 0)
					}
				case "keys":
					err = pressKeys(step.Keys)
				}
				if err != nil {
					return err
				}

				delay := time.Duration(step.DelayMs) * time.Millisecond
				if delay > lobbyMaxDelay {
					delay = lobbyMaxDelay
				}
				if pause(delay) {
					result.Aborted = true
					beep(400, 200)
					return nil
				}
			}

			if req.DryRun {
				continue
			}

			// Only a copy made during this player's steps counts, not what was there before.
			if clipboardSequence() != before {
				result.Raw[i] = clipboardText()
			}
			if result.Raw[i] != "" {
				beep(1200, 40)
			} else {
				beep(400, 40)
			}
		}

		beep(1500, 120)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Cancel stops a capture or a run that is waiting or in progress.
func (s *LobbyScan) Cancel() {
	s.cancel.Store(true)
}

var (
	digitsOnly     = regexp.MustCompile(`^\d{1,20}$`)
	steamProfileId = regexp.MustCompile(`steamcommunity\.com/profiles/(\d+)`)
	steamVanity    = regexp.MustCompile(`steamcommunity\.com/id/([^/?#\s]+)`)
	steamId64Xml   = regexp.MustCompile(`<steamID64>(\d+)</steamID64>`)
	longNumber     = regexp.MustCompile(`\d{5,20}`)
)

// ResolveIds turns what was copied for each player into a Steam ID (64-bit) or a friend ID:
// the number itself, a profile link, or a link with a custom name (asked from Steam). "" = no ID in it.
func (s *LobbyScan) ResolveIds(raw []string) []string {
	ids := make([]string, len(raw))
	for i, text := range raw {
		ids[i] = s.resolveId(strings.TrimSpace(text))
	}
	return ids
}

func (s *LobbyScan) resolveId(text string) string {
	if text == "" {
		return ""
	}
	if digitsOnly.MatchString(text) {
		return text
	}
	if m := steamProfileId.FindStringSubmatch(text); m != nil {
		return m[1]
	}
	if m := steamVanity.FindStringSubmatch(text); m != nil {
		if id, err := s.vanityId(m[1]); err == nil {
			return id
		}
		return ""
	}
	// "Friend ID: 123456789" and the like.
	return longNumber.FindString(text)
}

// vanityId asks Steam which account a custom profile name belongs to; the profile page says it in its XML form.
func (s *LobbyScan) vanityId(name string) (string, error) {
	client := s.Http
	if client == nil {
		client = http.DefaultClient
	}
	// The name comes out of a link, where it may be percent-encoded already.
	if plain, err := url.PathUnescape(name); err == nil {
		name = plain
	}
	resp, err := client.Get("https://steamcommunity.com/id/" + url.PathEscape(name) + "/?xml=1")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return "", err
	}
	if m := steamId64Xml.FindSubmatch(body); m != nil {
		return string(m[1]), nil
	}
	return "", errors.New("steam profile has no steamID64")
}
