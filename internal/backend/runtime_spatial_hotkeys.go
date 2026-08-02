package backend

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	runtimeSpatialHotkeyEvent                     = "runtime-spatial-hotkeys"
	runtimeSpatialHotkeyTransientErrorLimit       = 3
	virtualKeyLeft                                = 0x25
	virtualKeyUp                                  = 0x26
	virtualKeyRight                               = 0x27
	virtualKeyDown                                = 0x28
	virtualKeyPageUp                              = 0x21
	virtualKeyPageDown                            = 0x22
	virtualKeyF8                                  = 0x77
	virtualKeyW                                   = 0x57
	virtualKeyA                                   = 0x41
	virtualKeyS                                   = 0x53
	virtualKeyD                                   = 0x44
	runtimeInputKeyboard                          = 1
	runtimeKeyEventUp                             = 0x0002
	runtimeKeyEventScanCode                       = 0x0008
	runtimeSpatialNativeForward             uint8 = 1 << 0
	runtimeSpatialNativeLeft                uint8 = 1 << 1
	runtimeSpatialNativeBack                uint8 = 1 << 2
	runtimeSpatialNativeRight               uint8 = 1 << 3
	runtimeSpatialFlightModeVirtualGround         = "virtual_ground"
	runtimeSpatialFlightModeAerial                = "aerial"
)

var (
	getForegroundWindowProc      = user32DLL.NewProc("GetForegroundWindow")
	getWindowThreadProcessIDProc = user32DLL.NewProc("GetWindowThreadProcessId")
	sendInputProc                = user32DLL.NewProc("SendInput")
	runtimeForegroundProcessID   = foregroundProcessID
	runtimeSpatialSendInput      = sendRuntimeSpatialNativeTransitions
)

type runtimeKeyboardInput struct {
	VirtualKey uint16
	ScanCode   uint16
	Flags      uint32
	Time       uint32
	ExtraInfo  uintptr
}

// Windows INPUT is a DWORD plus an eight-byte aligned 32-byte union on amd64.
// Keeping the union opaque avoids depending on private Wails Win32 structs.
type runtimeInput struct {
	Type uint32
	_    uint32
	Data [32]byte
}

type runtimeNativeKeyTransition struct {
	VirtualKey uint16
	Down       bool
}

type RuntimeSpatialHotkeyStatus struct {
	Enabled           bool                           `json:"enabled"`
	ForegroundOnly    bool                           `json:"foregroundOnly"`
	Speed             float64                        `json:"speed"`
	OwnerLeaseID      string                         `json:"ownerLeaseId"`
	PID               uint32                         `json:"pid"`
	ProcessCreated    uint64                         `json:"processCreated,string"`
	GameVersion       string                         `json:"gameVersion"`
	Source            string                         `json:"source"`
	InputMode         string                         `json:"inputMode"`
	FlightEnabled     bool                           `json:"flightEnabled"`
	FlightMode        string                         `json:"flightMode"`
	VerticalInputMode string                         `json:"verticalInputMode"`
	FlightDiagnostics RuntimeSpatialFlightTickResult `json:"flightDiagnostics"`
	LastError         string                         `json:"lastError"`
	ConsecutiveErrors int                            `json:"consecutiveErrors"`
}

type runtimeSpatialHotkeyConfig struct {
	Enabled             bool
	Speed               float64
	Owner               string
	Process             processInstanceID
	GameVersion         string
	LastError           string
	ConsecutiveErrors   int
	InjectedMask        uint8
	FlightEnabled       bool
	FlightMode          string
	FlightOwnsJump      bool
	FlightToggleDown    bool
	FlightAnchor        runtimeSpatialFlightAnchorState
	FlightLastTick      RuntimeSpatialFlightTickResult
	FlightLastPublished RuntimeSpatialFlightTickResult
}

func normalizeRuntimeSpatialFlightMode(mode string) (string, error) {
	switch strings.TrimSpace(mode) {
	case "", runtimeSpatialFlightModeVirtualGround:
		return runtimeSpatialFlightModeVirtualGround, nil
	case runtimeSpatialFlightModeAerial:
		return runtimeSpatialFlightModeAerial, nil
	default:
		return "", fmt.Errorf("%s", runtimePatchMonitorText("飞行模式无效，请重新选择", "Invalid flight mode; choose a mode again"))
	}
}

func foregroundProcessID() uint32 {
	window, _, _ := getForegroundWindowProc.Call()
	if window == 0 {
		return 0
	}
	var pid uint32
	_, _, _ = getWindowThreadProcessIDProc.Call(window, uintptr(unsafe.Pointer(&pid)))
	return pid
}

func runtimeSpatialNativeMask(pressed func(int) bool) (uint8, error) {
	if pressed == nil {
		return 0, fmt.Errorf("direction-key reader is unavailable")
	}
	var mask uint8
	if pressed(virtualKeyLeft) {
		mask |= runtimeSpatialNativeLeft
	}
	if pressed(virtualKeyRight) {
		mask |= runtimeSpatialNativeRight
	}
	if pressed(virtualKeyDown) {
		mask |= runtimeSpatialNativeBack
	}
	if pressed(virtualKeyUp) {
		mask |= runtimeSpatialNativeForward
	}
	return mask, nil
}

var runtimeSpatialNativeKeyMap = [...]struct {
	Bit uint8
	Key uint16
}{
	{runtimeSpatialNativeForward, virtualKeyW},
	{runtimeSpatialNativeLeft, virtualKeyA},
	{runtimeSpatialNativeBack, virtualKeyS},
	{runtimeSpatialNativeRight, virtualKeyD},
}

func runtimeSpatialNativeTransitions(previous, next uint8) []runtimeNativeKeyTransition {
	result := make([]runtimeNativeKeyTransition, 0, 8)
	for _, mapping := range runtimeSpatialNativeKeyMap {
		if previous&mapping.Bit != 0 && next&mapping.Bit == 0 {
			result = append(result, runtimeNativeKeyTransition{VirtualKey: mapping.Key})
		}
	}
	for _, mapping := range runtimeSpatialNativeKeyMap {
		if previous&mapping.Bit == 0 && next&mapping.Bit != 0 {
			result = append(result, runtimeNativeKeyTransition{VirtualKey: mapping.Key, Down: true})
		}
	}
	return result
}

func sendRuntimeSpatialNativeTransitions(transitions []runtimeNativeKeyTransition) error {
	if len(transitions) == 0 {
		return nil
	}
	inputs := make([]runtimeInput, len(transitions))
	for index, transition := range transitions {
		scanCode, ok := runtimeSpatialNativeScanCode(transition.VirtualKey)
		if !ok {
			return fmt.Errorf("unsupported native movement key: 0x%X", transition.VirtualKey)
		}
		inputs[index].Type = runtimeInputKeyboard
		keyboard := (*runtimeKeyboardInput)(unsafe.Pointer(&inputs[index].Data[0]))
		keyboard.ScanCode = scanCode
		keyboard.Flags = runtimeKeyEventScanCode
		if !transition.Down {
			keyboard.Flags |= runtimeKeyEventUp
		}
	}
	sent, _, callErr := sendInputProc.Call(
		uintptr(len(inputs)),
		uintptr(unsafe.Pointer(&inputs[0])),
		unsafe.Sizeof(inputs[0]),
	)
	if sent != uintptr(len(inputs)) {
		if callErr != nil && callErr.Error() != "The operation completed successfully." {
			return fmt.Errorf("SendInput sent %d/%d events: %w", sent, len(inputs), callErr)
		}
		return fmt.Errorf("SendInput sent %d/%d events", sent, len(inputs))
	}
	return nil
}

func runtimeSpatialNativeScanCode(virtualKey uint16) (uint16, bool) {
	switch virtualKey {
	case virtualKeyW:
		return 0x11, true
	case virtualKeyA:
		return 0x1E, true
	case virtualKeyS:
		return 0x1F, true
	case virtualKeyD:
		return 0x20, true
	default:
		return 0, false
	}
}

func isFiniteFloat64(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func (a *App) runtimeSpatialHotkeyStatusLocked() RuntimeSpatialHotkeyStatus {
	config := a.runtimeSpatialHotkey
	flightMode, err := normalizeRuntimeSpatialFlightMode(config.FlightMode)
	if err != nil {
		flightMode = runtimeSpatialFlightModeVirtualGround
	}
	speed := config.Speed
	if !isFiniteFloat64(speed) || speed < runtimeSpatialFlightMinimumSpeed || speed > runtimeSpatialFlightMaximumSpeed {
		speed = 8
	}
	gameVersion := strings.TrimSpace(config.GameVersion)
	source := ""
	if gameVersion != "" {
		source = "game_runtime_spatial_hotkeys_" + gameVersion
	}
	return RuntimeSpatialHotkeyStatus{
		Enabled:           config.Enabled,
		ForegroundOnly:    true,
		Speed:             speed,
		OwnerLeaseID:      config.Owner,
		PID:               config.Process.PID,
		ProcessCreated:    config.Process.Created,
		GameVersion:       gameVersion,
		Source:            source,
		InputMode:         "native_wasd",
		FlightEnabled:     config.FlightEnabled,
		FlightMode:        flightMode,
		VerticalInputMode: "same_frame_height_hook",
		FlightDiagnostics: config.FlightLastTick,
		LastError:         config.LastError,
		ConsecutiveErrors: config.ConsecutiveErrors,
	}
}

func (a *App) emitRuntimeSpatialHotkeyStatus(status RuntimeSpatialHotkeyStatus) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, runtimeSpatialHotkeyEvent, status)
	}
}

func runtimeSpatialShouldEmitSuccessfulPoll(before, after runtimeSpatialHotkeyConfig, hadErrors, actionAttempted bool) bool {
	if before.FlightEnabled != after.FlightEnabled || (hadErrors && actionAttempted) {
		return true
	}
	if !after.FlightEnabled {
		return false
	}
	last := before.FlightLastPublished
	current := after.FlightLastTick
	if last.ContactTemplateReady != current.ContactTemplateReady ||
		last.ActionID != current.ActionID ||
		last.Grounded != current.Grounded ||
		last.Anchored != current.Anchored ||
		(last.AcceptedContacts == 0 && current.AcceptedContacts > 0) {
		return true
	}
	return current.SnapshotSequence >= last.SnapshotSequence+25
}

func (a *App) RuntimeSpatialHotkeysStatusOwned(owner string) (RuntimeSpatialHotkeyStatus, error) {
	a.runtimeSpatialHotkeyMu.Lock()
	defer a.runtimeSpatialHotkeyMu.Unlock()
	if a.runtimeSpatialHotkey.Owner != "" && !runtimeOwnerTokenMatches(a.runtimeSpatialHotkey.Owner, owner) {
		return a.runtimeSpatialHotkeyStatusLocked(), errRuntimeOwnerLeaseStale
	}
	return a.runtimeSpatialHotkeyStatusLocked(), nil
}

func (a *App) RuntimeSpatialHotkeysSetEnabledOwned(owner string, enabled bool, speed float64) (RuntimeSpatialHotkeyStatus, error) {
	a.runtimeSpatialFlightMu.Lock()
	defer a.runtimeSpatialFlightMu.Unlock()
	a.runtimeSpatialHotkeyMu.Lock()
	if !enabled {
		if a.runtimeSpatialHotkey.Owner != "" && !runtimeOwnerTokenMatches(a.runtimeSpatialHotkey.Owner, owner) {
			status := a.runtimeSpatialHotkeyStatusLocked()
			a.runtimeSpatialHotkeyMu.Unlock()
			return status, errRuntimeOwnerLeaseStale
		}
		injected := a.runtimeSpatialHotkey.InjectedMask
		restoreFlightJump := a.runtimeSpatialHotkey.FlightEnabled && a.runtimeSpatialHotkey.FlightOwnsJump
		// Keep the acquired process identity and detected layout while the shared
		// character lease remains connected.  The frontend validates every status,
		// including a successful disable response; clearing these fields here made
		// the button report an error even though the injected keys were released.
		flightMode, modeErr := normalizeRuntimeSpatialFlightMode(a.runtimeSpatialHotkey.FlightMode)
		if modeErr != nil {
			flightMode = runtimeSpatialFlightModeVirtualGround
		}
		a.runtimeSpatialHotkey.Enabled = false
		a.runtimeSpatialHotkey.Speed = speed
		a.runtimeSpatialHotkey.InjectedMask = 0
		a.runtimeSpatialHotkey.FlightEnabled = false
		a.runtimeSpatialHotkey.FlightMode = flightMode
		a.runtimeSpatialHotkey.FlightOwnsJump = false
		a.runtimeSpatialHotkey.FlightToggleDown = false
		a.runtimeSpatialHotkey.FlightAnchor = runtimeSpatialFlightAnchorState{}
		a.runtimeSpatialHotkey.FlightLastTick = RuntimeSpatialFlightTickResult{}
		a.runtimeSpatialHotkey.FlightLastPublished = RuntimeSpatialFlightTickResult{}
		a.runtimeSpatialHotkey.LastError = ""
		a.runtimeSpatialHotkey.ConsecutiveErrors = 0
		status := a.runtimeSpatialHotkeyStatusLocked()
		a.runtimeSpatialHotkeyMu.Unlock()
		var releaseErr error
		if err := runtimeSpatialSendInput(runtimeSpatialNativeTransitions(injected, 0)); err != nil {
			releaseErr = errors.Join(releaseErr, fmt.Errorf("%s: %w", runtimePatchMonitorText("释放游戏原生移动键失败，请在游戏内轻按一次 W/A/S/D", "Could not release native movement keys; tap W/A/S/D once in game"), err))
		}
		if restoreFlightJump {
			if _, err := a.RuntimeSpatialJumpSetEnabledOwned(owner, false); err != nil {
				releaseErr = errors.Join(releaseErr, fmt.Errorf("%s: %w", runtimePatchMonitorText("连续跳跃停用后未能确认两处入口恢复", "Could not confirm restoration of both continuous-jump sites"), err))
			}
		}
		if err := a.restoreRuntimeSpatialFlightHookOwned(owner, false); err != nil {
			releaseErr = errors.Join(releaseErr, fmt.Errorf("%s: %w", runtimePatchMonitorText("悬空飞行同帧 Hook 未能安全恢复", "Could not safely restore the flight same-frame hook"), err))
		}
		a.emitRuntimeSpatialHotkeyStatus(status)
		return status, releaseErr
	}
	if !isFiniteFloat64(speed) || speed < runtimeSpatialFlightMinimumSpeed || speed > runtimeSpatialFlightMaximumSpeed {
		status := a.runtimeSpatialHotkeyStatusLocked()
		a.runtimeSpatialHotkeyMu.Unlock()
		return status, fmt.Errorf("%s", runtimePatchMonitorText("飞行速度必须在 0.1 到 20 之间", "Flight speed must be between 0.1 and 20"))
	}

	a.procMu.Lock()
	if !runtimeOwnerTokenMatches(a.charaOwnerToken, owner) {
		a.procMu.Unlock()
		status := a.runtimeSpatialHotkeyStatusLocked()
		a.runtimeSpatialHotkeyMu.Unlock()
		return status, errRuntimeOwnerLeaseStale
	}
	if a.hProcess == 0 || a.moduleBase == 0 || a.charaPID == 0 || a.charaCreated == 0 || !processHandleAlive(a.hProcess) {
		a.procMu.Unlock()
		status := a.runtimeSpatialHotkeyStatusLocked()
		a.runtimeSpatialHotkeyMu.Unlock()
		return status, fmt.Errorf("%s", runtimePatchMonitorText("游戏进程连接已失效，请重新连接", "The game-process connection is no longer valid; reconnect"))
	}
	process := a.currentProcessInstance()
	if err := a.verifyRuntimePatchExecutableLocked(process, runtimePatchMonitorText("游戏内方向键移动", "In-game arrow-key movement")); err != nil {
		a.procMu.Unlock()
		status := a.runtimeSpatialHotkeyStatusLocked()
		a.runtimeSpatialHotkeyMu.Unlock()
		return status, err
	}
	layout, err := detectRuntimeGameLayout(remoteRuntimePatchPartyMemory{app: a}, a.moduleBase)
	if err != nil {
		a.procMu.Unlock()
		status := a.runtimeSpatialHotkeyStatusLocked()
		a.runtimeSpatialHotkeyMu.Unlock()
		return status, err
	}
	a.procMu.Unlock()

	a.runtimeSpatialHotkey = runtimeSpatialHotkeyConfig{
		Enabled:     true,
		Speed:       speed,
		Owner:       owner,
		Process:     process,
		GameVersion: layout.Version,
		FlightMode:  runtimeSpatialFlightModeVirtualGround,
	}
	status := a.runtimeSpatialHotkeyStatusLocked()
	a.runtimeSpatialHotkeyMu.Unlock()
	if !enabled {
		if err := a.restoreRuntimeSpatialFlightHookOwned(owner, false); err != nil {
			return status, err
		}
	}
	a.emitRuntimeSpatialHotkeyStatus(status)
	return status, nil
}

func (a *App) disableRuntimeSpatialHotkeysOwned(owner string, force bool) RuntimeSpatialHotkeyStatus {
	a.runtimeSpatialFlightMu.Lock()
	defer a.runtimeSpatialFlightMu.Unlock()
	a.runtimeSpatialHotkeyMu.Lock()
	injected := uint8(0)
	if force || runtimeOwnerTokenMatches(a.runtimeSpatialHotkey.Owner, owner) {
		speed := a.runtimeSpatialHotkey.Speed
		flightMode, modeErr := normalizeRuntimeSpatialFlightMode(a.runtimeSpatialHotkey.FlightMode)
		if modeErr != nil {
			flightMode = runtimeSpatialFlightModeVirtualGround
		}
		injected = a.runtimeSpatialHotkey.InjectedMask
		a.runtimeSpatialHotkey = runtimeSpatialHotkeyConfig{Speed: speed, FlightMode: flightMode}
	}
	status := a.runtimeSpatialHotkeyStatusLocked()
	a.runtimeSpatialHotkeyMu.Unlock()
	if injected != 0 {
		_ = runtimeSpatialSendInput(runtimeSpatialNativeTransitions(injected, 0))
	}
	a.emitRuntimeSpatialHotkeyStatus(status)
	return status
}

func (a *App) pollRuntimeSpatialHotkeys(pressed func(int) bool, foregroundPID func() uint32, interval time.Duration) {
	a.pollRuntimeSpatialHotkeysWithActions(
		pressed,
		foregroundPID,
		interval,
		runtimeSpatialSendInput,
		func(owner string, enabled bool) (bool, error) {
			if !enabled {
				_, err := a.RuntimeSpatialJumpSetEnabledOwned(owner, false)
				return false, err
			}
			status, err := a.RuntimeSpatialJumpStatusOwned(owner)
			if err != nil {
				return false, err
			}
			if status.Enabled {
				return false, nil
			}
			status, err = a.RuntimeSpatialJumpSetEnabledOwned(owner, true)
			return status.Enabled, err
		},
		func(owner string, command RuntimePatchVector3) error {
			_, err := a.RuntimeSpatialFlightAnchorTickOwned(owner, float64(command.Y), interval.Milliseconds())
			return err
		},
	)
}

func (a *App) pollRuntimeSpatialHotkeysWithInput(
	pressed func(int) bool,
	foregroundPID func() uint32,
	interval time.Duration,
	send func([]runtimeNativeKeyTransition) error,
) {
	a.pollRuntimeSpatialHotkeysWithActions(
		pressed, foregroundPID, interval, send,
		func(string, bool) (bool, error) { return false, nil },
		func(string, RuntimePatchVector3) error { return nil },
	)
}

func runtimeSpatialVerticalDelta(pressed func(int) bool, speed float64, interval time.Duration) (RuntimePatchVector3, bool, error) {
	if pressed == nil {
		return RuntimePatchVector3{}, false, fmt.Errorf("vertical-key reader is unavailable")
	}
	if !isFiniteFloat64(speed) || speed < runtimeSpatialFlightMinimumSpeed || speed > runtimeSpatialFlightMaximumSpeed {
		return RuntimePatchVector3{}, false, fmt.Errorf("%s", runtimePatchMonitorText("飞行速度必须在 0.1 到 20 之间", "Flight speed must be between 0.1 and 20"))
	}
	if interval <= 0 {
		return RuntimePatchVector3{}, false, fmt.Errorf("vertical-key interval must be positive")
	}
	up := pressed(virtualKeyPageUp)
	down := pressed(virtualKeyPageDown)
	command := RuntimePatchVector3{}
	switch {
	case up && !down:
		command.Y = float32(speed)
	case down && !up:
		command.Y = -float32(speed)
	}
	return command, true, nil
}

func (a *App) pollRuntimeSpatialHotkeysWithActions(
	pressed func(int) bool,
	foregroundPID func() uint32,
	interval time.Duration,
	send func([]runtimeNativeKeyTransition) error,
	setFlightJump func(string, bool) (bool, error),
	moveVertical func(string, RuntimePatchVector3) error,
) {
	if foregroundPID == nil || send == nil || setFlightJump == nil || moveVertical == nil || interval <= 0 {
		return
	}
	a.runtimeSpatialFlightMu.Lock()
	defer a.runtimeSpatialFlightMu.Unlock()
	a.runtimeSpatialHotkeyMu.Lock()
	config := a.runtimeSpatialHotkey
	a.runtimeSpatialHotkeyMu.Unlock()
	if !config.Enabled || config.Process.PID == 0 {
		return
	}
	foreground := foregroundPID() == config.Process.PID
	desired := uint8(0)
	var err error
	if foreground {
		desired, err = runtimeSpatialNativeMask(pressed)
	}
	transitions := runtimeSpatialNativeTransitions(config.InjectedMask, desired)
	actionAttempted := len(transitions) != 0
	if err == nil && len(transitions) != 0 {
		err = send(transitions)
	}

	flightEnabled := config.FlightEnabled
	flightOwnsJump := config.FlightOwnsJump
	flightMode, modeErr := normalizeRuntimeSpatialFlightMode(config.FlightMode)
	if err == nil && modeErr != nil {
		err = modeErr
	}
	flightToggleDown := foreground && pressed(virtualKeyF8)
	flightTicked := false
	if err == nil && flightToggleDown && !config.FlightToggleDown {
		actionAttempted = true
		if flightEnabled {
			if flightOwnsJump {
				_, err = setFlightJump(config.Owner, false)
			}
			if err == nil {
				flightEnabled = false
				flightOwnsJump = false
			}
		} else {
			if flightMode == runtimeSpatialFlightModeAerial {
				flightOwnsJump, err = setFlightJump(config.Owner, true)
			} else {
				// Ground-action flight must not retain the continuous-jump gates:
				// they keep ExActJump eligible after the virtual floor reports a
				// landing. The user presses their own configured jump key in-game.
				_, err = setFlightJump(config.Owner, false)
				flightOwnsJump = false
			}
			if err == nil {
				flightEnabled = true
			}
		}
	}
	if err == nil && flightEnabled {
		// Height holding is a persistent runtime responsibility and must continue
		// while the user is looking at another window. Only reading PageUp/PageDown
		// is foreground-gated; a background tick always means "hold this height".
		delta := RuntimePatchVector3{}
		if foreground {
			var active bool
			delta, active, err = runtimeSpatialVerticalDelta(pressed, config.Speed, interval)
			if err == nil && !active {
				err = fmt.Errorf("vertical-key reader is unavailable")
			}
		}
		if err == nil {
			actionAttempted = true
			err = moveVertical(config.Owner, delta)
			if err == nil {
				flightTicked = true
			}
		}
	}
	stateChanged := desired != config.InjectedMask || flightEnabled != config.FlightEnabled || flightOwnsJump != config.FlightOwnsJump || flightToggleDown != config.FlightToggleDown || flightTicked
	if err == nil && !stateChanged {
		return
	}
	a.runtimeSpatialHotkeyMu.Lock()
	if !a.runtimeSpatialHotkey.Enabled || a.runtimeSpatialHotkey.Owner != config.Owner || a.runtimeSpatialHotkey.Process != config.Process {
		a.runtimeSpatialHotkeyMu.Unlock()
		return
	}
	if err == nil {
		hadErrors := a.runtimeSpatialHotkey.ConsecutiveErrors != 0 || a.runtimeSpatialHotkey.LastError != ""
		a.runtimeSpatialHotkey.InjectedMask = desired
		a.runtimeSpatialHotkey.FlightEnabled = flightEnabled
		a.runtimeSpatialHotkey.FlightMode = flightMode
		a.runtimeSpatialHotkey.FlightOwnsJump = flightOwnsJump
		a.runtimeSpatialHotkey.FlightToggleDown = flightToggleDown
		if !flightEnabled {
			a.runtimeSpatialHotkey.FlightAnchor = runtimeSpatialFlightAnchorState{}
			a.runtimeSpatialHotkey.FlightLastTick = RuntimeSpatialFlightTickResult{}
			a.runtimeSpatialHotkey.FlightLastPublished = RuntimeSpatialFlightTickResult{}
		}
		if actionAttempted {
			a.runtimeSpatialHotkey.ConsecutiveErrors = 0
			a.runtimeSpatialHotkey.LastError = ""
		}
		shouldEmit := runtimeSpatialShouldEmitSuccessfulPoll(config, a.runtimeSpatialHotkey, hadErrors, actionAttempted)
		if shouldEmit {
			a.runtimeSpatialHotkey.FlightLastPublished = a.runtimeSpatialHotkey.FlightLastTick
		}
		status := a.runtimeSpatialHotkeyStatusLocked()
		a.runtimeSpatialHotkeyMu.Unlock()
		if shouldEmit {
			a.emitRuntimeSpatialHotkeyStatus(status)
		}
		return
	}
	a.runtimeSpatialHotkey.FlightEnabled = flightEnabled
	a.runtimeSpatialHotkey.FlightOwnsJump = flightOwnsJump
	a.runtimeSpatialHotkey.FlightToggleDown = flightToggleDown
	a.runtimeSpatialHotkey.ConsecutiveErrors++
	a.runtimeSpatialHotkey.LastError = strings.TrimSpace(err.Error())
	restoreFlightHook := false
	if runtimeSpatialHotkeyHardError(err) || a.runtimeSpatialHotkey.ConsecutiveErrors >= runtimeSpatialHotkeyTransientErrorLimit {
		a.runtimeSpatialHotkey.Enabled = false
		injected := a.runtimeSpatialHotkey.InjectedMask
		restoreFlightJump := a.runtimeSpatialHotkey.FlightEnabled && a.runtimeSpatialHotkey.FlightOwnsJump
		a.runtimeSpatialHotkey.InjectedMask = 0
		a.runtimeSpatialHotkey.FlightEnabled = false
		a.runtimeSpatialHotkey.FlightOwnsJump = false
		a.runtimeSpatialHotkey.FlightAnchor = runtimeSpatialFlightAnchorState{}
		a.runtimeSpatialHotkey.FlightLastTick = RuntimeSpatialFlightTickResult{}
		a.runtimeSpatialHotkey.FlightLastPublished = RuntimeSpatialFlightTickResult{}
		restoreFlightHook = true
		if injected != 0 {
			_ = send(runtimeSpatialNativeTransitions(injected, 0))
		}
		if restoreFlightJump {
			_, _ = setFlightJump(config.Owner, false)
		}
	}
	status := a.runtimeSpatialHotkeyStatusLocked()
	a.runtimeSpatialHotkeyMu.Unlock()
	if restoreFlightHook {
		_ = a.restoreRuntimeSpatialFlightHookOwned(config.Owner, false)
	}
	a.emitRuntimeSpatialHotkeyStatus(status)
}

func (a *App) setRuntimeSpatialFlightEnabledWithActions(
	owner string,
	enabled bool,
	mode string,
	setFlightJump func(string, bool) (bool, error),
) (RuntimeSpatialHotkeyStatus, error) {
	if setFlightJump == nil {
		return RuntimeSpatialHotkeyStatus{}, fmt.Errorf("flight actions are unavailable")
	}
	flightMode, err := normalizeRuntimeSpatialFlightMode(mode)
	if err != nil {
		return RuntimeSpatialHotkeyStatus{}, err
	}
	a.runtimeSpatialFlightMu.Lock()
	defer a.runtimeSpatialFlightMu.Unlock()

	a.runtimeSpatialHotkeyMu.Lock()
	config := a.runtimeSpatialHotkey
	if !config.Enabled {
		status := a.runtimeSpatialHotkeyStatusLocked()
		a.runtimeSpatialHotkeyMu.Unlock()
		return status, fmt.Errorf("%s", runtimePatchMonitorText("请先开启方向键与飞行常驻监听", "Enable the persistent movement and flight listener first"))
	}
	if !runtimeOwnerTokenMatches(config.Owner, owner) {
		status := a.runtimeSpatialHotkeyStatusLocked()
		a.runtimeSpatialHotkeyMu.Unlock()
		return status, errRuntimeOwnerLeaseStale
	}
	currentMode, modeErr := normalizeRuntimeSpatialFlightMode(config.FlightMode)
	if modeErr != nil {
		currentMode = runtimeSpatialFlightModeVirtualGround
	}
	if config.FlightEnabled == enabled && (!enabled || currentMode == flightMode) {
		status := a.runtimeSpatialHotkeyStatusLocked()
		a.runtimeSpatialHotkeyMu.Unlock()
		return status, nil
	}
	a.runtimeSpatialHotkeyMu.Unlock()

	flightOwnsJump := config.FlightOwnsJump
	if !enabled {
		// Restore the position hook before publishing the disabled state. Merely
		// stopping the watcher leaves the already-installed cave active and holds
		// the character in the last airborne pose indefinitely.
		if err := a.restoreRuntimeSpatialFlightHookOwned(owner, false); err != nil {
			status, _ := a.RuntimeSpatialHotkeysStatusOwned(owner)
			return status, fmt.Errorf("%s: %w", runtimePatchMonitorText("悬空飞行 Hook 未能安全恢复，功能仍保持开启", "The flight hook could not be safely restored; flight remains enabled"), err)
		}
	}
	if enabled {
		if flightMode == runtimeSpatialFlightModeAerial {
			flightOwnsJump, err = setFlightJump(owner, true)
		} else {
			_, err = setFlightJump(owner, false)
			flightOwnsJump = false
		}
		if err != nil {
			status, _ := a.RuntimeSpatialHotkeysStatusOwned(owner)
			return status, err
		}
	} else if flightOwnsJump {
		if _, err := setFlightJump(owner, false); err != nil {
			status, _ := a.RuntimeSpatialHotkeysStatusOwned(owner)
			return status, err
		}
		flightOwnsJump = false
	}

	a.runtimeSpatialHotkeyMu.Lock()
	if !a.runtimeSpatialHotkey.Enabled || a.runtimeSpatialHotkey.Owner != config.Owner || a.runtimeSpatialHotkey.Process != config.Process {
		status := a.runtimeSpatialHotkeyStatusLocked()
		a.runtimeSpatialHotkeyMu.Unlock()
		if enabled && flightOwnsJump {
			_, _ = setFlightJump(owner, false)
		}
		return status, errRuntimeOwnerLeaseStale
	}
	a.runtimeSpatialHotkey.FlightEnabled = enabled
	a.runtimeSpatialHotkey.FlightMode = flightMode
	a.runtimeSpatialHotkey.FlightOwnsJump = enabled && flightOwnsJump
	a.runtimeSpatialHotkey.FlightToggleDown = false
	a.runtimeSpatialHotkey.FlightAnchor = runtimeSpatialFlightAnchorState{}
	a.runtimeSpatialHotkey.FlightLastTick = RuntimeSpatialFlightTickResult{}
	a.runtimeSpatialHotkey.FlightLastPublished = RuntimeSpatialFlightTickResult{}
	a.runtimeSpatialHotkey.LastError = ""
	a.runtimeSpatialHotkey.ConsecutiveErrors = 0
	status := a.runtimeSpatialHotkeyStatusLocked()
	a.runtimeSpatialHotkeyMu.Unlock()
	a.emitRuntimeSpatialHotkeyStatus(status)
	return status, nil
}

func (a *App) RuntimeSpatialHotkeysSetFlightOwned(owner string, enabled bool) (RuntimeSpatialHotkeyStatus, error) {
	return a.RuntimeSpatialHotkeysSetFlightModeOwned(owner, enabled, runtimeSpatialFlightModeVirtualGround)
}

func (a *App) RuntimeSpatialHotkeysSetFlightModeOwned(owner string, enabled bool, mode string) (RuntimeSpatialHotkeyStatus, error) {
	return a.setRuntimeSpatialFlightEnabledWithActions(
		owner,
		enabled,
		mode,
		func(owner string, requested bool) (bool, error) {
			if !requested {
				_, err := a.RuntimeSpatialJumpSetEnabledOwned(owner, false)
				return false, err
			}
			status, err := a.RuntimeSpatialJumpStatusOwned(owner)
			if err != nil {
				return false, err
			}
			if status.Enabled {
				return false, nil
			}
			status, err = a.RuntimeSpatialJumpSetEnabledOwned(owner, true)
			return status.Enabled, err
		},
	)
}

func runtimeSpatialHotkeyHardError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errRuntimeOwnerLeaseStale) {
		return true
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	for _, fragment := range []string{
		"owner", "lease", "process", "pid", "signature", "executable", "original byte",
		"所有权", "进程", "签名", "原字节", "连接已失效",
	} {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	return false
}
