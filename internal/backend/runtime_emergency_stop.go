package backend

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

const (
	runtimeEmergencyStopEvent = "runtime-emergency-stop"
	virtualKeyF12             = 0x7B
)

var (
	user32DLL            = windows.NewLazySystemDLL("user32.dll")
	getAsyncKeyStateProc = user32DLL.NewProc("GetAsyncKeyState")
	runtimeKeyPressed    = func(key int) bool {
		state, _, _ := getAsyncKeyStateProc.Call(uintptr(key))
		return uint16(state)&0x8000 != 0
	}
)

type RuntimeEmergencyStopResult struct {
	TriggeredAt string `json:"triggeredAt"`
	Restored    bool   `json:"restored"`
	Detail      string `json:"detail"`
}

func (a *App) startRuntimeEmergencyWatcher() {
	a.emergencyWatcherMu.Lock()
	defer a.emergencyWatcherMu.Unlock()
	if a.emergencyWatcher != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.emergencyWatcher = cancel
	a.emergencyWatcherWG.Add(1)
	go func() {
		defer a.emergencyWatcherWG.Done()
		a.runRuntimeEmergencyWatcher(ctx, runtimeKeyPressed, 40*time.Millisecond, func() {
			result, err := a.runtimeEmergencyStop("F12")
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, runtimeEmergencyStopEvent, result)
			}
			if err != nil && a.ctx != nil {
				runtime.LogErrorf(a.ctx, "F12 emergency restoration failed: %v", err)
			}
		})
	}()
}

func (a *App) stopRuntimeEmergencyWatcher() {
	a.emergencyWatcherMu.Lock()
	cancel := a.emergencyWatcher
	a.emergencyWatcher = nil
	a.emergencyWatcherMu.Unlock()
	if cancel != nil {
		cancel()
		a.emergencyWatcherWG.Wait()
	}
}

func (a *App) runRuntimeEmergencyWatcher(ctx context.Context, pressed func(int) bool, interval time.Duration, trigger func()) {
	if pressed == nil || interval <= 0 || trigger == nil {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	wasPressed := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			isPressed := pressed(virtualKeyF12)
			if isPressed && !wasPressed {
				trigger()
			}
			wasPressed = isPressed
		}
	}
}

// RuntimeEmergencyStop restores every app-owned runtime hook and direct patch.
// It is safe to call repeatedly; unresolved recovery leases stay attached so a
// later attempt can retry instead of discarding the only restoration evidence.
func (a *App) RuntimeEmergencyStop() (RuntimeEmergencyStopResult, error) {
	return a.runtimeEmergencyStop("UI")
}

func (a *App) runtimeEmergencyStop(source string) (RuntimeEmergencyStopResult, error) {
	a.emergencyStopMu.Lock()
	defer a.emergencyStopMu.Unlock()

	triggeredAt := time.Now().UTC().Format(time.RFC3339Nano)
	restoreErr := runRuntimeEmergencyRestoration(
		a.closeFormulaSampler,
		func() error { return a.closeRuntimeLoadoutDetector(false) },
		a.closeRuntimeCompanions,
		a.CharaDetach,
	)
	result := RuntimeEmergencyStopResult{
		TriggeredAt: triggeredAt,
		Restored:    restoreErr == nil,
		Detail:      fmt.Sprintf("%s emergency stop restored all app-owned runtime features", source),
	}
	if restoreErr != nil {
		result.Detail = fmt.Sprintf("%s emergency stop could not prove complete restoration: %v", source, restoreErr)
		appendDiagnosticError("runtime emergency stop", restoreErr)
	}
	return result, restoreErr
}

func runRuntimeEmergencyRestoration(steps ...func() error) error {
	var restoreErr error
	for _, step := range steps {
		if step != nil {
			restoreErr = errors.Join(restoreErr, step())
		}
	}
	return restoreErr
}
