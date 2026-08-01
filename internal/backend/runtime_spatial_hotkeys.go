package backend

import (
	"fmt"
	"math"
	"strings"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	runtimeSpatialHotkeyEvent = "runtime-spatial-hotkeys"
	virtualKeyLeft            = 0x25
	virtualKeyUp              = 0x26
	virtualKeyRight           = 0x27
	virtualKeyDown            = 0x28
)

var (
	getForegroundWindowProc      = user32DLL.NewProc("GetForegroundWindow")
	getWindowThreadProcessIDProc = user32DLL.NewProc("GetWindowThreadProcessId")
	runtimeForegroundProcessID   = foregroundProcessID
)

type RuntimeSpatialHotkeyStatus struct {
	Enabled        bool    `json:"enabled"`
	ForegroundOnly bool    `json:"foregroundOnly"`
	Speed          float64 `json:"speed"`
	OwnerLeaseID   string  `json:"ownerLeaseId"`
	PID            uint32  `json:"pid"`
	ProcessCreated uint64  `json:"processCreated"`
	GameVersion    string  `json:"gameVersion"`
	Source         string  `json:"source"`
	LastError      string  `json:"lastError"`
}

type runtimeSpatialHotkeyConfig struct {
	Enabled     bool
	Speed       float64
	Owner       string
	Process     processInstanceID
	GameVersion string
	LastError   string
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

func runtimeSpatialHotkeyDelta(pressed func(int) bool, speed float64, interval time.Duration) (RuntimePatchVector3, bool, error) {
	if pressed == nil {
		return RuntimePatchVector3{}, false, fmt.Errorf("direction-key reader is unavailable")
	}
	if !isFiniteFloat64(speed) || speed < 0.1 || speed > 1000 {
		return RuntimePatchVector3{}, false, fmt.Errorf("%s", runtimePatchMonitorText("移动速度必须在 0.1 到 1000 单位/秒之间", "Movement speed must be between 0.1 and 1000 units/s"))
	}
	if interval <= 0 {
		return RuntimePatchVector3{}, false, fmt.Errorf("direction-key interval must be positive")
	}
	x := 0.0
	z := 0.0
	if pressed(virtualKeyLeft) {
		x--
	}
	if pressed(virtualKeyRight) {
		x++
	}
	if pressed(virtualKeyDown) {
		z--
	}
	if pressed(virtualKeyUp) {
		z++
	}
	if x == 0 && z == 0 {
		return RuntimePatchVector3{}, false, nil
	}
	length := math.Hypot(x, z)
	distance := speed * interval.Seconds()
	delta := RuntimePatchVector3{
		X: float32(x / length * distance),
		Z: float32(z / length * distance),
	}
	if err := validateRuntimeSpatialDelta(delta); err != nil {
		return RuntimePatchVector3{}, false, err
	}
	return delta, true, nil
}

func isFiniteFloat64(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func (a *App) runtimeSpatialHotkeyStatusLocked() RuntimeSpatialHotkeyStatus {
	config := a.runtimeSpatialHotkey
	speed := config.Speed
	if !isFiniteFloat64(speed) || speed < 0.1 || speed > 1000 {
		speed = 8
	}
	gameVersion := strings.TrimSpace(config.GameVersion)
	source := ""
	if gameVersion != "" {
		source = "game_runtime_spatial_hotkeys_" + gameVersion
	}
	return RuntimeSpatialHotkeyStatus{
		Enabled:        config.Enabled,
		ForegroundOnly: true,
		Speed:          speed,
		OwnerLeaseID:   config.Owner,
		PID:            config.Process.PID,
		ProcessCreated: config.Process.Created,
		GameVersion:    gameVersion,
		Source:         source,
		LastError:      config.LastError,
	}
}

func (a *App) emitRuntimeSpatialHotkeyStatus(status RuntimeSpatialHotkeyStatus) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, runtimeSpatialHotkeyEvent, status)
	}
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
	a.runtimeSpatialHotkeyMu.Lock()
	if !enabled {
		if a.runtimeSpatialHotkey.Owner != "" && !runtimeOwnerTokenMatches(a.runtimeSpatialHotkey.Owner, owner) {
			status := a.runtimeSpatialHotkeyStatusLocked()
			a.runtimeSpatialHotkeyMu.Unlock()
			return status, errRuntimeOwnerLeaseStale
		}
		a.runtimeSpatialHotkey = runtimeSpatialHotkeyConfig{Speed: speed}
		status := a.runtimeSpatialHotkeyStatusLocked()
		a.runtimeSpatialHotkeyMu.Unlock()
		a.emitRuntimeSpatialHotkeyStatus(status)
		return status, nil
	}
	if !isFiniteFloat64(speed) || speed < 0.1 || speed > 1000 {
		status := a.runtimeSpatialHotkeyStatusLocked()
		a.runtimeSpatialHotkeyMu.Unlock()
		return status, fmt.Errorf("%s", runtimePatchMonitorText("移动速度必须在 0.1 到 1000 单位/秒之间", "Movement speed must be between 0.1 and 1000 units/s"))
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
	}
	status := a.runtimeSpatialHotkeyStatusLocked()
	a.runtimeSpatialHotkeyMu.Unlock()
	a.emitRuntimeSpatialHotkeyStatus(status)
	return status, nil
}

func (a *App) disableRuntimeSpatialHotkeysOwned(owner string, force bool) RuntimeSpatialHotkeyStatus {
	a.runtimeSpatialHotkeyMu.Lock()
	if force || runtimeOwnerTokenMatches(a.runtimeSpatialHotkey.Owner, owner) {
		speed := a.runtimeSpatialHotkey.Speed
		a.runtimeSpatialHotkey = runtimeSpatialHotkeyConfig{Speed: speed}
	}
	status := a.runtimeSpatialHotkeyStatusLocked()
	a.runtimeSpatialHotkeyMu.Unlock()
	a.emitRuntimeSpatialHotkeyStatus(status)
	return status
}

func (a *App) pollRuntimeSpatialHotkeys(pressed func(int) bool, foregroundPID func() uint32, interval time.Duration) {
	a.pollRuntimeSpatialHotkeysWithMove(pressed, foregroundPID, interval, func(owner string, delta RuntimePatchVector3) error {
		_, err := a.RuntimeSpatialMoveOwned(owner, delta)
		return err
	})
}

func (a *App) pollRuntimeSpatialHotkeysWithMove(
	pressed func(int) bool,
	foregroundPID func() uint32,
	interval time.Duration,
	move func(string, RuntimePatchVector3) error,
) {
	if foregroundPID == nil || move == nil {
		return
	}
	a.runtimeSpatialHotkeyMu.Lock()
	if !a.runtimeSpatialHotkey.Enabled || a.runtimeSpatialHotkey.Process.PID == 0 {
		a.runtimeSpatialHotkeyMu.Unlock()
		return
	}
	if foregroundPID() != a.runtimeSpatialHotkey.Process.PID {
		a.runtimeSpatialHotkeyMu.Unlock()
		return
	}
	delta, active, err := runtimeSpatialHotkeyDelta(pressed, a.runtimeSpatialHotkey.Speed, interval)
	if err == nil && active {
		err = move(a.runtimeSpatialHotkey.Owner, delta)
	}
	if err == nil {
		a.runtimeSpatialHotkeyMu.Unlock()
		return
	}
	a.runtimeSpatialHotkey.Enabled = false
	a.runtimeSpatialHotkey.LastError = strings.TrimSpace(err.Error())
	status := a.runtimeSpatialHotkeyStatusLocked()
	a.runtimeSpatialHotkeyMu.Unlock()
	a.emitRuntimeSpatialHotkeyStatus(status)
}
