package backend

import (
	"errors"
	"math"
	"testing"
	"time"
)

func pressedKeys(keys ...int) func(int) bool {
	active := make(map[int]bool, len(keys))
	for _, key := range keys {
		active[key] = true
	}
	return func(key int) bool { return active[key] }
}

func TestRuntimeSpatialHotkeyDeltaMapsArrowKeysToWorldPlane(t *testing.T) {
	tests := []struct {
		name string
		keys []int
		x    float32
		z    float32
	}{
		{name: "left", keys: []int{virtualKeyLeft}, x: -0.32},
		{name: "right", keys: []int{virtualKeyRight}, x: 0.32},
		{name: "up", keys: []int{virtualKeyUp}, z: 0.32},
		{name: "down", keys: []int{virtualKeyDown}, z: -0.32},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			delta, active, err := runtimeSpatialHotkeyDelta(pressedKeys(tc.keys...), 8, 40*time.Millisecond)
			if err != nil {
				t.Fatal(err)
			}
			if !active || math.Abs(float64(delta.X-tc.x)) > 0.0001 || math.Abs(float64(delta.Z-tc.z)) > 0.0001 || delta.Y != 0 {
				t.Fatalf("delta = %+v, active = %v", delta, active)
			}
		})
	}
}

func TestRuntimeSpatialHotkeyDeltaNormalizesDiagonalSpeed(t *testing.T) {
	delta, active, err := runtimeSpatialHotkeyDelta(pressedKeys(virtualKeyUp, virtualKeyRight), 10, 40*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !active {
		t.Fatal("diagonal input should be active")
	}
	if got := math.Hypot(float64(delta.X), float64(delta.Z)); math.Abs(got-0.4) > 0.0001 {
		t.Fatalf("diagonal distance = %f, want 0.4", got)
	}
}

func TestRuntimeSpatialHotkeyStatusUsesDetectedGameVersion(t *testing.T) {
	app := NewApp()
	app.runtimeSpatialHotkey = runtimeSpatialHotkeyConfig{
		Enabled:     true,
		Speed:       8,
		Owner:       "owner",
		Process:     processInstanceID{PID: 25660, Created: 99},
		GameVersion: "2.0.3",
	}
	status := app.runtimeSpatialHotkeyStatusLocked()
	if status.GameVersion != "2.0.3" || status.Source != "game_runtime_spatial_hotkeys_2.0.3" {
		t.Fatalf("hotkey status retained a legacy version gate: %+v", status)
	}
}

func TestRuntimeSpatialHotkeysMoveOnlyWhenGameIsForeground(t *testing.T) {
	app := NewApp()
	app.runtimeSpatialHotkey = runtimeSpatialHotkeyConfig{
		Enabled: true,
		Speed:   8,
		Owner:   "owner",
		Process: processInstanceID{PID: 25660, Created: 99},
	}
	moves := 0
	move := func(owner string, delta RuntimePatchVector3) error {
		moves++
		if owner != "owner" || delta.X <= 0 {
			t.Fatalf("owner = %q, delta = %+v", owner, delta)
		}
		return nil
	}
	app.pollRuntimeSpatialHotkeysWithMove(pressedKeys(virtualKeyRight), func() uint32 { return 123 }, 40*time.Millisecond, move)
	if moves != 0 {
		t.Fatalf("background moves = %d, want 0", moves)
	}
	app.pollRuntimeSpatialHotkeysWithMove(pressedKeys(virtualKeyRight), func() uint32 { return 25660 }, 40*time.Millisecond, move)
	if moves != 1 {
		t.Fatalf("foreground moves = %d, want 1", moves)
	}
}

func TestRuntimeSpatialHotkeysFailClosedAfterMoveError(t *testing.T) {
	app := NewApp()
	app.runtimeSpatialHotkey = runtimeSpatialHotkeyConfig{
		Enabled: true,
		Speed:   8,
		Owner:   "owner",
		Process: processInstanceID{PID: 25660, Created: 99},
	}
	moveErr := errors.New("player topology changed")
	app.pollRuntimeSpatialHotkeysWithMove(
		pressedKeys(virtualKeyUp),
		func() uint32 { return 25660 },
		40*time.Millisecond,
		func(string, RuntimePatchVector3) error { return moveErr },
	)
	status, err := app.RuntimeSpatialHotkeysStatusOwned("owner")
	if err != nil {
		t.Fatal(err)
	}
	if status.Enabled || status.LastError != moveErr.Error() {
		t.Fatalf("status = %+v", status)
	}
}

func TestRuntimeSpatialHotkeyOwnedDisableCannotStopAnotherOwner(t *testing.T) {
	app := NewApp()
	app.runtimeSpatialHotkey = runtimeSpatialHotkeyConfig{
		Enabled: true,
		Speed:   8,
		Owner:   "current-owner",
		Process: processInstanceID{PID: 25660, Created: 99},
	}
	app.disableRuntimeSpatialHotkeysOwned("stale-owner", false)
	if !app.runtimeSpatialHotkey.Enabled {
		t.Fatal("stale owner disabled the active direction-key session")
	}
	app.disableRuntimeSpatialHotkeysOwned("current-owner", false)
	if app.runtimeSpatialHotkey.Enabled {
		t.Fatal("current owner did not disable its direction-key session")
	}
}
