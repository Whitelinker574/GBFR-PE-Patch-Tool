package backend

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRuntimeSpatialHotkeyStatusSerializesExactProcessCreationIdentity(t *testing.T) {
	app := NewApp()
	app.runtimeSpatialHotkey = runtimeSpatialHotkeyConfig{
		Enabled: true,
		Speed:   8,
		Owner:   "owner",
		Process: processInstanceID{PID: 25660, Created: 134147703140663100},
	}
	payload, err := json.Marshal(app.runtimeSpatialHotkeyStatusLocked())
	if err != nil {
		t.Fatal(err)
	}
	if got := string(payload); !strings.Contains(got, `"processCreated":"134147703140663100"`) {
		t.Fatalf("process creation identity lost JSON precision: %s", got)
	}
}

func pressedKeys(keys ...int) func(int) bool {
	active := make(map[int]bool, len(keys))
	for _, key := range keys {
		active[key] = true
	}
	return func(key int) bool { return active[key] }
}

func TestRuntimeSpatialHotkeysMapArrowsToNativeMovementKeys(t *testing.T) {
	tests := []struct {
		name string
		keys []int
		mask uint8
	}{
		{name: "left", keys: []int{virtualKeyLeft}, mask: runtimeSpatialNativeLeft},
		{name: "right", keys: []int{virtualKeyRight}, mask: runtimeSpatialNativeRight},
		{name: "up", keys: []int{virtualKeyUp}, mask: runtimeSpatialNativeForward},
		{name: "down", keys: []int{virtualKeyDown}, mask: runtimeSpatialNativeBack},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mask, err := runtimeSpatialNativeMask(pressedKeys(tc.keys...))
			if err != nil {
				t.Fatal(err)
			}
			if mask != tc.mask {
				t.Fatalf("mask = %04b, want %04b", mask, tc.mask)
			}
		})
	}
}

func TestRuntimeSpatialHotkeyTransitionsPressAndReleaseNativeWASD(t *testing.T) {
	mask, err := runtimeSpatialNativeMask(pressedKeys(virtualKeyUp, virtualKeyRight))
	if err != nil {
		t.Fatal(err)
	}
	pressed := runtimeSpatialNativeTransitions(0, mask)
	if len(pressed) != 2 || pressed[0] != (runtimeNativeKeyTransition{VirtualKey: virtualKeyW, Down: true}) || pressed[1] != (runtimeNativeKeyTransition{VirtualKey: virtualKeyD, Down: true}) {
		t.Fatalf("press transitions = %+v", pressed)
	}
	released := runtimeSpatialNativeTransitions(mask, 0)
	if len(released) != 2 || released[0] != (runtimeNativeKeyTransition{VirtualKey: virtualKeyW}) || released[1] != (runtimeNativeKeyTransition{VirtualKey: virtualKeyD}) {
		t.Fatalf("release transitions = %+v", released)
	}
}

func TestRuntimeSpatialNativeMovementUsesGameScanCodes(t *testing.T) {
	for virtualKey, want := range map[uint16]uint16{
		virtualKeyW: 0x11,
		virtualKeyA: 0x1E,
		virtualKeyS: 0x1F,
		virtualKeyD: 0x20,
	} {
		if got, ok := runtimeSpatialNativeScanCode(virtualKey); !ok || got != want {
			t.Fatalf("virtual key 0x%X scan code=0x%X ok=%v, want 0x%X", virtualKey, got, ok, want)
		}
	}
	if _, ok := runtimeSpatialNativeScanCode(0xFF); ok {
		t.Fatal("unknown virtual key unexpectedly mapped to a game scan code")
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

func TestRuntimeSpatialFlightHotkeysDefaultOffAndReportHeightAnchorMode(t *testing.T) {
	app := NewApp()
	status := app.runtimeSpatialHotkeyStatusLocked()
	if status.FlightEnabled {
		t.Fatal("experimental flight hotkeys must default to off")
	}
	if status.VerticalInputMode != "same_frame_height_hook" {
		t.Fatalf("vertical mode = %q", status.VerticalInputMode)
	}
	if status.FlightMode != runtimeSpatialFlightModeVirtualGround {
		t.Fatalf("default flight mode = %q", status.FlightMode)
	}
}

func TestRuntimeSpatialVirtualGroundModeDoesNotKeepContinuousJumpPatch(t *testing.T) {
	app := NewApp()
	app.runtimeSpatialHotkey = runtimeSpatialHotkeyConfig{
		Enabled: true, Speed: 8, Owner: "owner", Process: processInstanceID{PID: 25660, Created: 99}, FlightMode: runtimeSpatialFlightModeVirtualGround,
	}
	var jump []bool
	setFlight := func(owner string, enabled bool) (bool, error) {
		if owner != "owner" {
			t.Fatalf("owner = %q", owner)
		}
		jump = append(jump, enabled)
		return enabled, nil
	}
	move := func(string, RuntimePatchVector3) error { return nil }
	send := func([]runtimeNativeKeyTransition) error { return nil }

	app.pollRuntimeSpatialHotkeysWithActions(pressedKeys(virtualKeyF8), func() uint32 { return 25660 }, 40*time.Millisecond, send, setFlight, move)
	if len(jump) != 1 || jump[0] || !app.runtimeSpatialHotkey.FlightEnabled || app.runtimeSpatialHotkey.FlightOwnsJump {
		t.Fatalf("virtual-ground F8 retained continuous jump: calls=%v config=%+v", jump, app.runtimeSpatialHotkey)
	}
	app.pollRuntimeSpatialHotkeysWithActions(pressedKeys(virtualKeyF8), func() uint32 { return 25660 }, 40*time.Millisecond, send, setFlight, move)
	if len(jump) != 1 {
		t.Fatalf("held F8 toggled repeatedly: %v", jump)
	}
	app.pollRuntimeSpatialHotkeysWithActions(pressedKeys(), func() uint32 { return 25660 }, 40*time.Millisecond, send, setFlight, move)
	app.pollRuntimeSpatialHotkeysWithActions(pressedKeys(virtualKeyF8), func() uint32 { return 25660 }, 40*time.Millisecond, send, setFlight, move)
	if len(jump) != 1 || app.runtimeSpatialHotkey.FlightEnabled || app.runtimeSpatialHotkey.FlightOwnsJump {
		t.Fatalf("second F8 edge changed an unowned jump patch: calls=%v config=%+v", jump, app.runtimeSpatialHotkey)
	}
}

func TestRuntimeSpatialFlightButtonsKeepVirtualGroundAndAerialModesSeparate(t *testing.T) {
	app := NewApp()
	app.runtimeSpatialHotkey = runtimeSpatialHotkeyConfig{
		Enabled: true, Speed: 8, Owner: "owner", Process: processInstanceID{PID: 25660, Created: 99},
	}
	var jump []bool
	setJump := func(owner string, enabled bool) (bool, error) {
		if owner != "owner" {
			t.Fatalf("owner=%q", owner)
		}
		jump = append(jump, enabled)
		return enabled, nil
	}

	status, err := app.setRuntimeSpatialFlightEnabledWithActions("owner", true, runtimeSpatialFlightModeVirtualGround, setJump)
	if err != nil {
		t.Fatal(err)
	}
	if !status.FlightEnabled || status.FlightMode != runtimeSpatialFlightModeVirtualGround || len(jump) != 1 || jump[0] || app.runtimeSpatialHotkey.FlightOwnsJump {
		t.Fatalf("takeoff status=%+v jump=%v", status, jump)
	}
	status, err = app.setRuntimeSpatialFlightEnabledWithActions("owner", false, runtimeSpatialFlightModeVirtualGround, setJump)
	if err != nil {
		t.Fatal(err)
	}
	if status.FlightEnabled || len(jump) != 1 {
		t.Fatalf("landing status=%+v jump=%v", status, jump)
	}

	status, err = app.setRuntimeSpatialFlightEnabledWithActions("owner", true, runtimeSpatialFlightModeAerial, setJump)
	if err != nil {
		t.Fatal(err)
	}
	if !status.FlightEnabled || status.FlightMode != runtimeSpatialFlightModeAerial || len(jump) != 2 || !jump[1] || !app.runtimeSpatialHotkey.FlightOwnsJump {
		t.Fatalf("aerial takeoff status=%+v jump=%v", status, jump)
	}
	status, err = app.setRuntimeSpatialFlightEnabledWithActions("owner", false, runtimeSpatialFlightModeAerial, setJump)
	if err != nil {
		t.Fatal(err)
	}
	if status.FlightEnabled || len(jump) != 3 || jump[2] {
		t.Fatalf("aerial landing status=%+v jump=%v", status, jump)
	}
}

func TestRuntimeSpatialFlightHotkeysAlwaysHoldHeightButReadVerticalKeysOnlyInForeground(t *testing.T) {
	app := NewApp()
	app.runtimeSpatialHotkey = runtimeSpatialHotkeyConfig{
		Enabled: true, FlightEnabled: true, Speed: 8, Owner: "owner", Process: processInstanceID{PID: 25660, Created: 99},
	}
	var moves []RuntimePatchVector3
	move := func(owner string, delta RuntimePatchVector3) error {
		if owner != "owner" {
			t.Fatalf("owner = %q", owner)
		}
		moves = append(moves, delta)
		return nil
	}
	setFlight := func(string, bool) (bool, error) { return false, nil }
	send := func([]runtimeNativeKeyTransition) error { return nil }

	app.pollRuntimeSpatialHotkeysWithActions(pressedKeys(virtualKeyPageUp), func() uint32 { return 123 }, 40*time.Millisecond, send, setFlight, move)
	if len(moves) != 1 || moves[0].Y != 0 {
		t.Fatalf("background tick must hold height without reading PageUp: %+v", moves)
	}
	app.pollRuntimeSpatialHotkeysWithActions(pressedKeys(virtualKeyPageUp), func() uint32 { return 25660 }, 40*time.Millisecond, send, setFlight, move)
	app.pollRuntimeSpatialHotkeysWithActions(pressedKeys(), func() uint32 { return 25660 }, 40*time.Millisecond, send, setFlight, move)
	app.pollRuntimeSpatialHotkeysWithActions(pressedKeys(virtualKeyPageDown), func() uint32 { return 25660 }, 40*time.Millisecond, send, setFlight, move)
	app.pollRuntimeSpatialHotkeysWithActions(pressedKeys(virtualKeyPageUp, virtualKeyPageDown), func() uint32 { return 25660 }, 40*time.Millisecond, send, setFlight, move)
	if len(moves) != 5 || moves[1].Y != 8 || moves[2].Y != 0 || moves[3].Y != -8 || moves[4].Y != 0 {
		t.Fatalf("vertical moves = %+v", moves)
	}
}

func TestRuntimeSpatialFlightDiagnosticTransitionMustBePublished(t *testing.T) {
	before := runtimeSpatialHotkeyConfig{
		Enabled: true, FlightEnabled: true, Owner: "owner", Process: processInstanceID{PID: 25660, Created: 99},
		FlightLastTick: RuntimeSpatialFlightTickResult{ContactTemplateReady: false, FloorQueries: 4},
	}
	after := before
	after.FlightLastTick = RuntimeSpatialFlightTickResult{ContactTemplateReady: true, FloorQueries: 8, SnapshotSequence: 4}
	if !runtimeSpatialShouldEmitSuccessfulPoll(before, after, false, true) {
		t.Fatal("a successful background tick captured the ground template but would leave the UI on its stale waiting state")
	}
}

func TestRuntimeSpatialFlightCounterOnlyDiagnosticsAreThrottled(t *testing.T) {
	baseline := RuntimeSpatialFlightTickResult{ContactTemplateReady: true, Grounded: true, ActionID: 0, SnapshotSequence: 100, FloorQueries: 200}
	before := runtimeSpatialHotkeyConfig{Enabled: true, FlightEnabled: true, FlightLastPublished: baseline, FlightLastTick: baseline}
	after := before
	after.FlightLastTick.SnapshotSequence = 124
	after.FlightLastTick.FloorQueries = 224
	if runtimeSpatialShouldEmitSuccessfulPoll(before, after, false, true) {
		t.Fatal("counter-only flight diagnostics were published faster than the bounded interval")
	}
	after.FlightLastTick.SnapshotSequence = 125
	if !runtimeSpatialShouldEmitSuccessfulPoll(before, after, false, true) {
		t.Fatal("counter-only flight diagnostics were not published at the bounded interval")
	}
}

func TestRuntimeSpatialFlightToggleFailurePersistsAcrossKeyReleaseAndFailsClosed(t *testing.T) {
	app := NewApp()
	app.runtimeSpatialHotkey = runtimeSpatialHotkeyConfig{
		Enabled: true, Speed: 8, Owner: "owner", Process: processInstanceID{PID: 25660, Created: 99},
	}
	gravityErr := errors.New("gravity temporarily unavailable")
	setFlight := func(string, bool) (bool, error) { return false, gravityErr }
	move := func(string, RuntimePatchVector3) error { return nil }
	send := func([]runtimeNativeKeyTransition) error { return nil }
	for attempt := 1; attempt <= runtimeSpatialHotkeyTransientErrorLimit; attempt++ {
		app.pollRuntimeSpatialHotkeysWithActions(pressedKeys(virtualKeyF8), func() uint32 { return 25660 }, 40*time.Millisecond, send, setFlight, move)
		app.pollRuntimeSpatialHotkeysWithActions(pressedKeys(), func() uint32 { return 25660 }, 40*time.Millisecond, send, setFlight, move)
		if attempt < runtimeSpatialHotkeyTransientErrorLimit && app.runtimeSpatialHotkey.LastError != gravityErr.Error() {
			t.Fatalf("attempt %d lost the gravity failure after key release: %+v", attempt, app.runtimeSpatialHotkey)
		}
	}
	if app.runtimeSpatialHotkey.Enabled || app.runtimeSpatialHotkey.ConsecutiveErrors != runtimeSpatialHotkeyTransientErrorLimit {
		t.Fatalf("repeated gravity failures did not stop the listener: %+v", app.runtimeSpatialHotkey)
	}
}

func TestRuntimeSpatialHotkeysInjectOnlyWhenGameIsForegroundAndReleaseOnBlur(t *testing.T) {
	app := NewApp()
	app.runtimeSpatialHotkey = runtimeSpatialHotkeyConfig{
		Enabled: true,
		Speed:   8,
		Owner:   "owner",
		Process: processInstanceID{PID: 25660, Created: 99},
	}
	var batches [][]runtimeNativeKeyTransition
	send := func(transitions []runtimeNativeKeyTransition) error {
		batches = append(batches, append([]runtimeNativeKeyTransition(nil), transitions...))
		return nil
	}
	app.pollRuntimeSpatialHotkeysWithInput(pressedKeys(virtualKeyRight), func() uint32 { return 123 }, 40*time.Millisecond, send)
	if len(batches) != 0 {
		t.Fatalf("background injections = %+v", batches)
	}
	app.pollRuntimeSpatialHotkeysWithInput(pressedKeys(virtualKeyRight), func() uint32 { return 25660 }, 40*time.Millisecond, send)
	if len(batches) != 1 || batches[0][0] != (runtimeNativeKeyTransition{VirtualKey: virtualKeyD, Down: true}) {
		t.Fatalf("foreground injections = %+v", batches)
	}
	app.pollRuntimeSpatialHotkeysWithInput(pressedKeys(), func() uint32 { return 123 }, 40*time.Millisecond, send)
	if len(batches) != 2 || batches[1][0] != (runtimeNativeKeyTransition{VirtualKey: virtualKeyD}) {
		t.Fatalf("blur must release injected key: %+v", batches)
	}
}

func TestRuntimeSpatialHotkeysTolerateTransientMovementCompetition(t *testing.T) {
	app := NewApp()
	app.runtimeSpatialHotkey = runtimeSpatialHotkeyConfig{
		Enabled: true,
		Speed:   8,
		Owner:   "owner",
		Process: processInstanceID{PID: 25660, Created: 99},
	}
	moveErr := errors.New("SendInput temporarily unavailable")
	app.pollRuntimeSpatialHotkeysWithInput(
		pressedKeys(virtualKeyUp),
		func() uint32 { return 25660 },
		40*time.Millisecond,
		func([]runtimeNativeKeyTransition) error { return moveErr },
	)
	status, err := app.RuntimeSpatialHotkeysStatusOwned("owner")
	if err != nil {
		t.Fatal(err)
	}
	if !status.Enabled || status.LastError != moveErr.Error() || status.ConsecutiveErrors != 1 {
		t.Fatalf("status = %+v", status)
	}
	app.pollRuntimeSpatialHotkeysWithInput(pressedKeys(virtualKeyUp), func() uint32 { return 25660 }, 40*time.Millisecond, func([]runtimeNativeKeyTransition) error { return moveErr })
	app.pollRuntimeSpatialHotkeysWithInput(pressedKeys(virtualKeyUp), func() uint32 { return 25660 }, 40*time.Millisecond, func([]runtimeNativeKeyTransition) error { return moveErr })
	status, err = app.RuntimeSpatialHotkeysStatusOwned("owner")
	if err != nil {
		t.Fatal(err)
	}
	if status.Enabled || status.ConsecutiveErrors != 3 {
		t.Fatalf("three consecutive movement failures must stop the session: %+v", status)
	}
}

func TestRuntimeSpatialHotkeysStopImmediatelyOnOwnerFailure(t *testing.T) {
	app := NewApp()
	app.runtimeSpatialHotkey = runtimeSpatialHotkeyConfig{
		Enabled: true, Speed: 8, Owner: "owner", Process: processInstanceID{PID: 25660, Created: 99},
	}
	app.pollRuntimeSpatialHotkeysWithInput(
		pressedKeys(virtualKeyUp), func() uint32 { return 25660 }, 40*time.Millisecond,
		func([]runtimeNativeKeyTransition) error { return errRuntimeOwnerLeaseStale },
	)
	status, err := app.RuntimeSpatialHotkeysStatusOwned("owner")
	if err != nil {
		t.Fatal(err)
	}
	if status.Enabled || status.ConsecutiveErrors != 1 {
		t.Fatalf("owner failure must stop immediately: %+v", status)
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

func TestRuntimeSpatialHotkeyOwnedDisableRetainsConnectedIdentity(t *testing.T) {
	app := NewApp()
	app.runtimeSpatialHotkey = runtimeSpatialHotkeyConfig{
		Enabled:      true,
		Speed:        8,
		Owner:        "current-owner",
		Process:      processInstanceID{PID: 25660, Created: 99},
		GameVersion:  "2.0.3",
		InjectedMask: runtimeSpatialNativeForward,
	}
	status, err := app.RuntimeSpatialHotkeysSetEnabledOwned("current-owner", false, 8)
	if err != nil {
		t.Fatal(err)
	}
	if status.Enabled || status.OwnerLeaseID != "current-owner" || status.PID != 25660 || status.ProcessCreated != 99 {
		t.Fatalf("disabled status lost the connected owner/process identity: %+v", status)
	}
	if status.GameVersion != "2.0.3" || status.Source != "game_runtime_spatial_hotkeys_2.0.3" {
		t.Fatalf("disabled status lost the detected game layout: %+v", status)
	}
	if status.LastError != "" || status.ConsecutiveErrors != 0 {
		t.Fatalf("disabled status retained a stale runtime error: %+v", status)
	}
}

func TestCharaAcquireDoesNotSilentlyDisableActiveSpatialHotkeys(t *testing.T) {
	app := NewApp()
	app.runtimeSpatialHotkey = runtimeSpatialHotkeyConfig{
		Enabled: true,
		Speed:   8,
		Owner:   "current-owner",
		Process: processInstanceID{PID: 25660, Created: 99},
	}
	if _, err := app.CharaAcquire(1); err == nil || !strings.Contains(err.Error(), "方向键") {
		t.Fatalf("acquire should report the active hotkey owner, got %v", err)
	}
	if !app.runtimeSpatialHotkey.Enabled || app.runtimeSpatialHotkey.Owner != "current-owner" {
		t.Fatalf("failed acquire silently disabled the active hotkeys: %+v", app.runtimeSpatialHotkey)
	}
}
