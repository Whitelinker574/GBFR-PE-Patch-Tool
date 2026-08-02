package backend

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"testing"
	"time"
	"unsafe"
)

// This read-only probe is the tight feedback loop for the reported symptom:
// virtual-ground is installed for the current player, yet the active
// action remains Jump/Landing instead of returning to Wait/Run. It never owns
// or mutates the existing tool session.
func TestRuntimeSpatialFlightLiveVirtualGroundReturnsToGroundAction203(t *testing.T) {
	if os.Getenv("GBFR_LIVE_SPATIAL_FLIGHT_OBSERVE") != "1" {
		t.Skip("set GBFR_LIVE_SPATIAL_FLIGHT_OBSERVE=1 while virtual-ground hover is active")
	}
	process, err := openReadOnlyGameProcessForLayouts(windowsReadOnlyProcessBackend{}, charaProcessName, runtimeCharacterPanelRuntimeLayouts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = process.Close() })
	layout, err := detectRuntimeGameLayout(process, process.moduleBase)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := resolveRuntimeSpatialFlightBinding(process, process.moduleBase, layout)
	if err != nil {
		t.Fatal(err)
	}
	entry := make([]byte, runtimeSpatialFlightHookSize)
	if err := process.ReadAt(process.moduleBase+runtimeSpatialFlightHookRVA, entry); err != nil {
		t.Fatal(err)
	}
	if entry[0] != 0xE9 {
		t.Fatalf("floor-query virtual-contact hook is not installed: entry=% X", entry)
	}
	cave := relJumpTarget(process.moduleBase+runtimeSpatialFlightHookRVA, entry)
	marker := make([]byte, len(runtimeSpatialFlightMarker))
	if err := process.ReadAt(cave+runtimeSpatialFlightMarkerOffset, marker); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(marker, runtimeSpatialFlightMarker[:]) {
		t.Fatalf("floor-query cave marker=% X", marker)
	}
	controllerRaw := make([]byte, 8)
	if err := process.ReadAt(cave+runtimeSpatialFlightControllerDataOffset, controllerRaw); err != nil {
		t.Fatal(err)
	}
	if got := uintptr(binary.LittleEndian.Uint64(controllerRaw)); got != binding.Controller {
		t.Fatalf("virtual-ground owns controller=0x%X current=0x%X", got, binding.Controller)
	}

	var lastAction uint32
	var lastGrounded byte
	var diagnostics runtimeSpatialFlightHookDiagnostics
	for attempt := 0; attempt < 60; attempt++ {
		actionRaw := make([]byte, 4)
		groundedRaw := make([]byte, 1)
		if err := process.ReadAt(binding.Owner+0x40, actionRaw); err != nil {
			t.Fatal(err)
		}
		if err := process.ReadAt(binding.Controller+runtimeSpatialControllerGroundedOffset, groundedRaw); err != nil {
			t.Fatal(err)
		}
		lastAction = binary.LittleEndian.Uint32(actionRaw)
		lastGrounded = groundedRaw[0]
		diagnostics, err = readRuntimeSpatialFlightHookDiagnostics(process, cave)
		if err != nil {
			t.Fatal(err)
		}
		if diagnostics.ContactTemplateReady && diagnostics.AcceptedContacts > 0 && lastGrounded != 0 && (lastAction == 0 || lastAction == 1) {
			t.Logf("virtual contact reached a ground action: action=%d grounded=%d diagnostics=%+v", lastAction, lastGrounded, diagnostics)
			return
		}
		time.Sleep(16 * time.Millisecond)
	}
	t.Fatalf("virtual contact remained outside Wait/Run: action=%d grounded=%d controller=0x%X diagnostics=%+v", lastAction, lastGrounded, binding.Controller, diagnostics)
}

// This opt-in probe is strictly read-only and observes the already-running
// tool session. It is the red-capable loop for the aerial-mode regression:
// after Jump/Avoid/AirAttack owns the character, the tool must replay contact
// long enough for the native action sequence to reach Landing then Wait/Run.
func TestRuntimeSpatialFlightLiveAerialActionRecovery203(t *testing.T) {
	if os.Getenv("GBFR_LIVE_SPATIAL_FLIGHT_AERIAL_RECOVERY") != "1" {
		t.Skip("set GBFR_LIVE_SPATIAL_FLIGHT_AERIAL_RECOVERY=1, then perform one aerial attack in the active tool session")
	}
	process, err := openReadOnlyGameProcessForLayouts(windowsReadOnlyProcessBackend{}, charaProcessName, runtimeCharacterPanelRuntimeLayouts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = process.Close() })
	layout, err := detectRuntimeGameLayout(process, process.moduleBase)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := resolveRuntimeSpatialFlightBinding(process, process.moduleBase, layout)
	if err != nil {
		t.Fatal(err)
	}
	entry := make([]byte, runtimeSpatialFlightHookSize)
	if err := process.ReadAt(process.moduleBase+runtimeSpatialFlightHookRVA, entry); err != nil {
		t.Fatal(err)
	}
	if entry[0] != 0xE9 {
		t.Fatalf("aerial floor-query hook is not installed: entry=% X", entry)
	}
	cave := relJumpTarget(process.moduleBase+runtimeSpatialFlightHookRVA, entry)
	marker := make([]byte, len(runtimeSpatialFlightMarker))
	if err := process.ReadAt(cave+runtimeSpatialFlightMarkerOffset, marker); err != nil || !bytes.Equal(marker, runtimeSpatialFlightMarker[:]) {
		t.Fatalf("aerial floor-query cave marker=% X error=%v", marker, err)
	}

	deadline := time.Now().Add(30 * time.Second)
	var sawTrigger, sawLanding, sawWait bool
	var lastAction uint32 = math.MaxUint32
	var trace []string
	for time.Now().Before(deadline) {
		actionRaw := make([]byte, 4)
		groundedRaw := []byte{0}
		control := make([]byte, 2)
		anchorControl := make([]byte, 2)
		if err := process.ReadAt(binding.Owner+0x40, actionRaw); err != nil {
			t.Fatal(err)
		}
		if err := process.ReadAt(binding.Controller+runtimeSpatialControllerGroundedOffset, groundedRaw); err != nil {
			t.Fatal(err)
		}
		if err := process.ReadAt(cave+runtimeSpatialFlightActiveDataOffset, control); err != nil {
			t.Fatal(err)
		}
		if err := process.ReadAt(cave+runtimeSpatialFlightHeightAnchorDataOffset, anchorControl); err != nil {
			t.Fatal(err)
		}
		velocity, err := readRuntimeSpatialFlightFloat(process, binding.Controller+runtimeSpatialControllerVelocityOffset, "aerial recovery velocity")
		if err != nil {
			t.Fatal(err)
		}
		action := binary.LittleEndian.Uint32(actionRaw)
		if action != lastAction || control[1] != 0 {
			diagnostics, readErr := readRuntimeSpatialFlightHookDiagnostics(process, cave)
			if readErr != nil {
				t.Fatal(readErr)
			}
			frame := fmt.Sprintf("action=%d grounded=%d velocity=%.3f active=%d contact=%d heightAnchor=%d clearVelocity=%d accepted=%d", action, groundedRaw[0], velocity, control[0], control[1], anchorControl[0], anchorControl[1], diagnostics.AcceptedContacts)
			if len(trace) == 0 || trace[len(trace)-1] != frame {
				trace = append(trace, frame)
			}
		}
		lastAction = action
		if action == 2 || action == 4 || action == 9 {
			sawTrigger = true
		}
		if sawTrigger && action == 3 {
			sawLanding = true
		}
		if sawLanding && (action == 0 || action == 1) {
			sawWait = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !sawTrigger {
		t.Fatalf("no Jump/Avoid/AirAttack action was observed; trace=%v", trace)
	}
	if !sawLanding || !sawWait {
		t.Fatalf("aerial action did not recover through Landing -> Wait/Run: sawLanding=%t sawWait=%t trace=%v", sawLanding, sawWait, trace)
	}
	t.Logf("aerial action recovered: trace=%v", trace)
}

func TestRuntimeSpatialFlightRecoverOrphanedProbe203(t *testing.T) {
	if os.Getenv("GBFR_LIVE_SPATIAL_FLIGHT_RECOVER") != "1" {
		t.Skip("set GBFR_LIVE_SPATIAL_FLIGHT_RECOVER=1 to restore a marker-verified orphaned probe")
	}
	app := NewApp()
	info, err := app.CharaAcquire(1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = app.CharaRelease(info.OwnerToken) }()
	readOwnedEntry := func(rva uintptr, size int, original []byte, markerOffset uintptr, marker []byte) ([]byte, uintptr) {
		entryAddr := app.moduleBase + rva
		entry := make([]byte, size)
		if err := readProcessMemory(app.hProcess, entryAddr, unsafe.Pointer(&entry[0]), uintptr(len(entry))); err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(entry, original) {
			return entry, 0
		}
		if entry[0] != 0xE9 {
			t.Fatalf("probe entry RVA 0x%X is not an installed rel32 hook: % X", rva, entry)
		}
		cave := relJumpTarget(entryAddr, entry)
		actualMarker := make([]byte, len(marker))
		if err := readProcessMemory(app.hProcess, cave+markerOffset, unsafe.Pointer(&actualMarker[0]), uintptr(len(actualMarker))); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(actualMarker, marker) {
			t.Fatalf("probe cave marker RVA 0x%X=% X", rva, actualMarker)
		}
		return entry, cave
	}
	mainEntry, mainCave := readOwnedEntry(runtimeSpatialFlightHookRVA, runtimeSpatialFlightHookSize, runtimeSpatialFlightHookOriginal, runtimeSpatialFlightMarkerOffset, runtimeSpatialFlightMarker[:])
	_, groundCave := readOwnedEntry(runtimeSpatialVirtualGroundHookRVA, runtimeSpatialVirtualGroundHookSize, runtimeSpatialVirtualGroundHookOriginal, runtimeSpatialVirtualGroundMarkerOffset, runtimeSpatialVirtualGroundMarker[:])
	if mainCave == 0 && groundCave == 0 {
		t.Log("both flight probe entries are already restored")
		return
	}
	if groundCave != 0 {
		if err := writeAndVerifyRuntimeHookEntry(app.hProcess, app.moduleBase+runtimeSpatialVirtualGroundHookRVA, runtimeSpatialVirtualGroundHookOriginal); err != nil {
			t.Fatal(err)
		}
		app.runtimePatchMu.Lock()
		app.retireRuntimeCaveLocked(groundCave, "legacy virtual-ground probe recovery")
		app.runtimePatchMu.Unlock()
		t.Logf("restored legacy action-forcing probe at 0x%X", groundCave)
	}
	if mainCave == 0 {
		return
	}
	controllerRaw := make([]byte, 8)
	heightRaw := make([]byte, 8)
	if err := readProcessMemory(app.hProcess, mainCave+runtimeSpatialFlightControllerDataOffset, unsafe.Pointer(&controllerRaw[0]), 8); err != nil {
		t.Fatal(err)
	}
	if err := readProcessMemory(app.hProcess, mainCave+runtimeSpatialFlightHeightDataOffset, unsafe.Pointer(&heightRaw[0]), 8); err != nil {
		t.Fatal(err)
	}
	app.runtimePatchMu.Lock()
	app.runtimeSpatialFlightHookLease = &runtimeSpatialFlightHookLease{
		OwnerToken: info.OwnerToken, Process: app.currentProcessInstance(),
		EntryAddr: app.moduleBase + runtimeSpatialFlightHookRVA, Original: append([]byte(nil), runtimeSpatialFlightHookOriginal...), Installed: mainEntry, CaveAddr: mainCave,
		Controller: uintptr(binary.LittleEndian.Uint64(controllerRaw)), HeightAddr: uintptr(binary.LittleEndian.Uint64(heightRaw)),
		ContactReplay: true,
	}
	err = app.restoreRuntimeSpatialFlightHookOwnedLocked(info.OwnerToken, false)
	app.runtimePatchMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("restored orphaned flight floor-query probe at 0x%X", mainCave)
}

func sendRuntimeSpatialLiveTestScanCode(scanCode uint16, down bool) error {
	input := runtimeInput{Type: runtimeInputKeyboard}
	keyboard := (*runtimeKeyboardInput)(unsafe.Pointer(&input.Data[0]))
	keyboard.ScanCode = scanCode
	keyboard.Flags = runtimeKeyEventScanCode
	if !down {
		keyboard.Flags |= runtimeKeyEventUp
	}
	sent, _, callErr := sendInputProc.Call(1, uintptr(unsafe.Pointer(&input)), unsafe.Sizeof(input))
	if sent != 1 {
		return fmt.Errorf("SendInput scan=0x%X down=%t sent=%d error=%v", scanCode, down, sent, callErr)
	}
	return nil
}

// This opt-in loop performs one synthetic press of the configured F jump
// binding, then samples the production flight tick, both hooks, grounded state,
// and active action every 16 ms. It catches the exact "hover works but remains
// in jump attacks" failure without asking the user to time a manual probe.
func TestRuntimeSpatialFlightLiveSyntheticJumpReturnsToGroundAction203(t *testing.T) {
	inputMode := os.Getenv("GBFR_LIVE_SPATIAL_FLIGHT_SYNTHETIC_JUMP")
	if inputMode != "1" && inputMode != "external" {
		t.Skip("set GBFR_LIVE_SPATIAL_FLIGHT_SYNTHETIC_JUMP=1 with GAME 2.0.3 foreground and jump bound to F")
	}
	app := NewApp()
	info, err := app.CharaAcquire(1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = app.RuntimeSpatialHotkeysSetEnabledOwned(info.OwnerToken, false, 8)
		_ = app.CharaRelease(info.OwnerToken)
	})
	if runtimeForegroundProcessID() != info.PID {
		t.Fatalf("game must be foreground before the synthetic jump: foreground=%d game=%d", runtimeForegroundProcessID(), info.PID)
	}
	if _, err := app.RuntimeSpatialHotkeysSetEnabledOwned(info.OwnerToken, true, 8); err != nil {
		t.Fatal(err)
	}
	if _, err := app.RuntimeSpatialHotkeysSetFlightModeOwned(info.OwnerToken, true, runtimeSpatialFlightModeVirtualGround); err != nil {
		t.Fatal(err)
	}
	initial, err := app.RuntimeSpatialFlightAnchorTickOwned(info.OwnerToken, 0, 16)
	if err != nil {
		t.Fatal(err)
	}
	if !initial.Grounded {
		t.Fatal("synthetic jump probe must start on the ground")
	}
	app.runtimeSpatialHotkeyMu.Lock()
	binding := app.runtimeSpatialHotkey.FlightAnchor.Binding
	app.runtimeSpatialHotkeyMu.Unlock()
	if binding.Entity == 0 || binding.Controller == 0 {
		t.Fatal("flight binding was not retained after the initial ground sample")
	}
	memory := remoteRuntimeSpatialMemory{app: app}
	if inputMode == "1" {
		if err := sendRuntimeSpatialLiveTestScanCode(0x21, true); err != nil { // physical F key
			t.Fatal(err)
		}
		time.Sleep(60 * time.Millisecond)
		if err := sendRuntimeSpatialLiveTestScanCode(0x21, false); err != nil {
			t.Fatal(err)
		}
	} else {
		t.Log("waiting for one external F jump input")
	}

	var sawJump, sawAnchor bool
	var lastAction uint32
	var lastGrounded byte
	var lastHeight float32
	var acceptedContacts uint64
	attemptLimit := 240
	if inputMode == "external" {
		attemptLimit = 750
	}
	for attempt := 0; attempt < attemptLimit; attempt++ {
		result, tickErr := app.RuntimeSpatialFlightAnchorTickOwned(info.OwnerToken, 0, 16)
		if tickErr != nil {
			t.Fatal(tickErr)
		}
		actionRaw := make([]byte, 4)
		groundedRaw := make([]byte, 1)
		if err := memory.ReadAt(binding.Owner+0x40, actionRaw); err != nil {
			t.Fatal(err)
		}
		if err := memory.ReadAt(binding.Controller+runtimeSpatialControllerGroundedOffset, groundedRaw); err != nil {
			t.Fatal(err)
		}
		lastAction = binary.LittleEndian.Uint32(actionRaw)
		lastGrounded = groundedRaw[0]
		lastHeight = result.CurrentHeight
		if lastAction == 2 {
			sawJump = true
		}
		app.runtimePatchMu.Lock()
		lease := app.runtimeSpatialFlightHookLease
		if lease != nil && lease.ContactReplay {
			sawAnchor = true
			countRaw := make([]byte, 8)
			if err := memory.ReadAt(lease.CaveAddr+runtimeSpatialFlightAcceptCountDataOffset, countRaw); err == nil {
				acceptedContacts = binary.LittleEndian.Uint64(countRaw)
			}
		}
		app.runtimePatchMu.Unlock()
		if sawJump && sawAnchor && acceptedContacts > 0 && lastGrounded != 0 && (lastAction == 0 || lastAction == 1) && lastHeight > initial.GroundHeight+0.02 {
			t.Logf("virtual-ground recovered ground action in air: action=%d grounded=%d height=%v acceptedContacts=%d", lastAction, lastGrounded, lastHeight, acceptedContacts)
			return
		}
		time.Sleep(16 * time.Millisecond)
	}
	t.Fatalf("virtual-ground did not recover Wait/Run in air: sawJump=%t sawAnchor=%t action=%d grounded=%d height=%v ground=%v acceptedContacts=%d", sawJump, sawAnchor, lastAction, lastGrounded, lastHeight, initial.GroundHeight, acceptedContacts)
}

// This opt-in acceptance loop owns a short aerial session, injects the
// configured physical F jump binding, and waits for one externally supplied normal attack
// (a mouse click from the test driver). It exercises the production watcher and
// fails on the reported one-shot action lifecycle.
func TestRuntimeSpatialFlightLiveSyntheticAerialAttackRecovery203(t *testing.T) {
	if os.Getenv("GBFR_LIVE_SPATIAL_FLIGHT_SYNTHETIC_AERIAL") != "1" {
		t.Skip("set GBFR_LIVE_SPATIAL_FLIGHT_SYNTHETIC_AERIAL=1 with GAME 2.0.3 foreground and F bound to jump")
	}
	app := NewApp()
	info, err := app.CharaAcquire(1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = app.RuntimeSpatialHotkeysSetEnabledOwned(info.OwnerToken, false, 8)
		_ = app.CharaRelease(info.OwnerToken)
	})
	if runtimeForegroundProcessID() != info.PID {
		t.Fatalf("game must be foreground before the synthetic aerial test: foreground=%d game=%d", runtimeForegroundProcessID(), info.PID)
	}
	if _, err := app.RuntimeSpatialHotkeysSetEnabledOwned(info.OwnerToken, true, 8); err != nil {
		t.Fatal(err)
	}
	if _, err := app.RuntimeSpatialHotkeysSetFlightModeOwned(info.OwnerToken, true, runtimeSpatialFlightModeAerial); err != nil {
		t.Fatal(err)
	}
	initial, err := app.RuntimeSpatialFlightAnchorTickOwned(info.OwnerToken, 0, 16)
	if err != nil {
		t.Fatal(err)
	}
	if !initial.Grounded || initial.ActionID != 0 {
		t.Fatalf("synthetic aerial test must start in grounded Wait: %+v", initial)
	}
	if err := sendRuntimeSpatialLiveTestScanCode(0x21, true); err != nil {
		t.Fatal(err)
	}
	time.Sleep(60 * time.Millisecond)
	if err := sendRuntimeSpatialLiveTestScanCode(0x21, false); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(8 * time.Second)
	var sawJump, sawActive, sawAttack, sawLanding, sawWait bool
	var lastAction uint32 = math.MaxUint32
	var trace []string
	for time.Now().Before(deadline) {
		result, tickErr := app.RuntimeSpatialFlightAnchorTickOwned(info.OwnerToken, 0, 16)
		if tickErr != nil {
			t.Fatal(tickErr)
		}
		if result.ActionID != lastAction || result.RecoveryActive {
			frame := fmt.Sprintf("action=%d grounded=%t velocity=%.3f anchored=%t recovery=%t accepted=%d", result.ActionID, result.Grounded, result.VerticalVelocity, result.Anchored, result.RecoveryActive, result.AcceptedContacts)
			if len(trace) == 0 || trace[len(trace)-1] != frame {
				trace = append(trace, frame)
			}
		}
		lastAction = result.ActionID
		if result.ActionID == 2 {
			sawJump = true
		}
		if result.Anchored {
			sawActive = true
		}
		if sawActive && result.ActionID != 0 && result.ActionID != 1 && result.ActionID != 2 && result.ActionID != 3 {
			sawAttack = true
		}
		if sawAttack && result.ActionID == 3 {
			sawLanding = true
		}
		if sawLanding && (result.ActionID == 0 || result.ActionID == 1) {
			sawWait = true
			break
		}
		time.Sleep(16 * time.Millisecond)
	}
	if !sawJump || !sawActive {
		t.Fatalf("synthetic F jump did not reach aerial hover: sawJump=%t sawActive=%t trace=%v", sawJump, sawActive, trace)
	}
	if !sawAttack {
		t.Fatalf("no external aerial attack was observed; click normal attack once during the loop: trace=%v", trace)
	}
	if !sawLanding || !sawWait {
		t.Fatalf("aerial attack remained one-shot instead of Landing -> Wait/Run: sawLanding=%t sawWait=%t trace=%v", sawLanding, sawWait, trace)
	}
	t.Logf("aerial attack recovered: trace=%v", trace)
}

// This opt-in acceptance test performs one bounded 0.1-unit height-anchor
// tick against a running GAME 2.0.3 process. It proves the production hook is
// reached on airborne-capable frames, owns the current controller, and applies
// its target. Cleanup restores the hook entry even when an assertion fails.
func TestRuntimeSpatialFlightLiveSameFrameHook203(t *testing.T) {
	if os.Getenv("GBFR_LIVE_SPATIAL_FLIGHT_WRITE") != "1" {
		t.Skip("set GBFR_LIVE_SPATIAL_FLIGHT_WRITE=1 while GAME 2.0.3 is idle")
	}

	app := NewApp()
	info, err := app.CharaAcquire(1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = app.RuntimeSpatialHotkeysSetEnabledOwned(info.OwnerToken, false, 8)
		_ = app.CharaRelease(info.OwnerToken)
	})
	if _, err := app.RuntimeSpatialHotkeysSetEnabledOwned(info.OwnerToken, true, 8); err != nil {
		t.Fatal(err)
	}
	if _, err := app.setRuntimeSpatialFlightEnabledWithActions(
		info.OwnerToken,
		true,
		runtimeSpatialFlightModeVirtualGround,
		func(string, bool) (bool, error) { return false, nil },
	); err != nil {
		t.Fatal(err)
	}

	memory := remoteRuntimeSpatialMemory{app: app}
	result, err := app.RuntimeSpatialFlightAnchorTickOwned(info.OwnerToken, 4, 250)
	if err != nil {
		t.Fatal(err)
	}
	var lease *runtimeSpatialFlightHookLease
	var hits, accepted uint64
	for attempt := 0; attempt < 180; attempt++ {
		result, err = app.RuntimeSpatialFlightAnchorTickOwned(info.OwnerToken, 0, 16)
		if err != nil {
			t.Fatal(err)
		}
		app.runtimePatchMu.Lock()
		lease = app.runtimeSpatialFlightHookLease
		app.runtimePatchMu.Unlock()
		if lease != nil {
			counts := make([]byte, 16)
			if err := memory.ReadAt(lease.CaveAddr+runtimeSpatialFlightHitCountDataOffset, counts); err != nil {
				t.Fatal(err)
			}
			hits = binary.LittleEndian.Uint64(counts[:8])
			accepted = binary.LittleEndian.Uint64(counts[8:])
			if hits > 0 && accepted > 0 {
				break
			}
		}
		time.Sleep(16 * time.Millisecond)
	}
	if lease == nil || hits == 0 || accepted == 0 || accepted > hits {
		t.Fatalf("same-frame cave did not execute for the owned ExFall after takeoff: result=%+v hits=%d accepted=%d", result, hits, accepted)
	}
	currentY, err := readRuntimeSpatialFlightFloat(memory, lease.HeightAddr, "live character height")
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(float64(currentY-lease.TargetHeight)) > 0.02 {
		t.Fatalf("same-frame cave ran but height was not held: current=%v target=%v hits=%d accepted=%d", currentY, lease.TargetHeight, hits, accepted)
	}

	// Exercise the exact PageUp integration path after hover is active.
	up, err := app.RuntimeSpatialFlightAnchorTickOwned(info.OwnerToken, 4, 100)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	currentY, err = readRuntimeSpatialFlightFloat(memory, lease.HeightAddr, "live character height after PageUp")
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(float64(currentY-up.TargetHeight)) > 0.02 {
		t.Fatalf("PageUp target was not applied: current=%v target=%v", currentY, up.TargetHeight)
	}
	down, err := app.RuntimeSpatialFlightAnchorTickOwned(info.OwnerToken, -4, 100)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	currentY, err = readRuntimeSpatialFlightFloat(memory, lease.HeightAddr, "live character height after PageDown")
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(float64(currentY-down.TargetHeight)) > 0.02 {
		t.Fatalf("PageDown target was not applied: current=%v target=%v", currentY, down.TargetHeight)
	}
	t.Logf("pid=%d currentY=%v hits=%d accepted=%d pageUpY=%v pageDownY=%v", info.PID, currentY, hits, accepted, up.TargetHeight, down.TargetHeight)
}

// This opt-in probe installs only the inactive floor-query wrapper. It never
// moves the character or changes an action. A grounded 2.0.3 scene must feed
// at least one complete native floor contact into the pre-takeoff template.
func TestRuntimeSpatialFlightLiveCapturesGroundTemplate203(t *testing.T) {
	if os.Getenv("GBFR_LIVE_SPATIAL_FLIGHT_CAPTURE") != "1" {
		t.Skip("set GBFR_LIVE_SPATIAL_FLIGHT_CAPTURE=1 while GAME 2.0.3 is standing in a stable scene")
	}
	app := NewApp()
	info, err := app.CharaAcquire(1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = app.restoreRuntimeSpatialFlightHookOwned(info.OwnerToken, true)
		_ = app.CharaRelease(info.OwnerToken)
	})

	liveMemoryWriteMu.Lock()
	app.procMu.Lock()
	if err := app.ensureLiveMemoryWritesSafe(); err != nil {
		app.procMu.Unlock()
		liveMemoryWriteMu.Unlock()
		t.Fatal(err)
	}
	memory := remoteRuntimeSpatialMemory{app: app}
	layout, err := detectRuntimeGameLayout(remoteRuntimePatchPartyMemory{app: app}, app.moduleBase)
	if err != nil {
		app.procMu.Unlock()
		liveMemoryWriteMu.Unlock()
		t.Fatal(err)
	}
	binding, err := resolveRuntimeSpatialFlightBinding(memory, app.moduleBase, layout)
	if err != nil {
		app.procMu.Unlock()
		liveMemoryWriteMu.Unlock()
		t.Fatal(err)
	}
	groundY, err := readRuntimeSpatialFlightFloat(memory, binding.TransformNode+runtimePatchPartyPositionYOffset, "live ground-template height")
	if err != nil {
		app.procMu.Unlock()
		liveMemoryWriteMu.Unlock()
		t.Fatal(err)
	}
	state := runtimeSpatialFlightAnchorState{GroundY: groundY, PeakY: groundY, PreviousY: groundY, TargetY: groundY, Binding: binding}
	app.runtimePatchMu.Lock()
	err = app.syncRuntimeSpatialFlightHookLocked(info.OwnerToken, binding, state, runtimeSpatialFlightModeVirtualGround)
	lease := app.runtimeSpatialFlightHookLease
	app.runtimePatchMu.Unlock()
	app.procMu.Unlock()
	liveMemoryWriteMu.Unlock()
	if err != nil || lease == nil {
		t.Fatalf("install inactive floor-query capture: lease=%+v error=%v", lease, err)
	}

	deadline := time.Now().Add(5 * time.Second)
	var diagnostics runtimeSpatialFlightHookDiagnostics
	for time.Now().Before(deadline) {
		diagnostics, err = readRuntimeSpatialFlightHookDiagnostics(memory, lease.CaveAddr)
		if err != nil {
			t.Fatal(err)
		}
		if diagnostics.ContactTemplateReady && diagnostics.FloorQueries > 0 && diagnostics.SnapshotSequence > 0 {
			t.Logf("captured live native floor contact: %+v", diagnostics)
			return
		}
		time.Sleep(16 * time.Millisecond)
	}
	t.Fatalf("no complete native floor contact was captured before takeoff: %+v", diagnostics)
}

// This opt-in action-layer acceptance test requires one real user jump. It
// proves that hover supplies a native-looking floor contact: Y stays above the
// real floor while ExFall returns to grounded locomotion instead of remaining
// in the jump/fall action set.
func TestRuntimeSpatialFlightLiveVirtualFloorActionLayer203(t *testing.T) {
	if os.Getenv("GBFR_LIVE_SPATIAL_FLIGHT_ACTION") != "1" {
		t.Skip("set GBFR_LIVE_SPATIAL_FLIGHT_ACTION=1 while GAME 2.0.3 is idle on level ground")
	}

	app := NewApp()
	info, err := app.CharaAcquire(1)
	if err != nil {
		t.Fatal(err)
	}
	var recoveryBinding runtimeSpatialFlightBinding
	var recoveryGroundY float32
	var recoveryReady bool
	t.Cleanup(func() {
		_ = app.restoreRuntimeSpatialFlightHookOwned(info.OwnerToken, true)
		if recoveryReady && app.hProcess != 0 && processHandleAlive(app.hProcess) {
			memory := remoteRuntimeSpatialMemory{app: app}
			rawY := make([]byte, 4)
			binary.LittleEndian.PutUint32(rawY, math.Float32bits(recoveryGroundY))
			_ = memory.WriteAt(recoveryBinding.TransformNode+runtimePatchPartyPositionYOffset, rawY)
			_ = memory.WriteAt(recoveryBinding.Controller+runtimeSpatialControllerVelocityOffset, []byte{0, 0, 0, 0})
			_ = memory.WriteAt(recoveryBinding.Controller+runtimeSpatialControllerGroundedOffset, []byte{1})
		}
		_ = app.CharaRelease(info.OwnerToken)
	})

	liveMemoryWriteMu.Lock()
	app.procMu.Lock()
	if err := app.ensureLiveMemoryWritesSafe(); err != nil {
		app.procMu.Unlock()
		liveMemoryWriteMu.Unlock()
		t.Fatal(err)
	}
	layout, err := detectRuntimeGameLayout(remoteRuntimePatchPartyMemory{app: app}, app.moduleBase)
	if err != nil {
		app.procMu.Unlock()
		liveMemoryWriteMu.Unlock()
		t.Fatal(err)
	}
	memory := remoteRuntimeSpatialMemory{app: app}
	binding, err := resolveRuntimeSpatialFlightBinding(memory, app.moduleBase, layout)
	if err != nil {
		app.procMu.Unlock()
		liveMemoryWriteMu.Unlock()
		t.Fatal(err)
	}
	groundY, err := readRuntimeSpatialFlightFloat(memory, binding.TransformNode+runtimePatchPartyPositionYOffset, "live ground height")
	if err != nil {
		app.procMu.Unlock()
		liveMemoryWriteMu.Unlock()
		t.Fatal(err)
	}
	grounded := []byte{0}
	if err := memory.ReadAt(binding.Controller+runtimeSpatialControllerGroundedOffset, grounded); err != nil {
		app.procMu.Unlock()
		liveMemoryWriteMu.Unlock()
		t.Fatal(err)
	}
	if grounded[0] == 0 {
		app.procMu.Unlock()
		liveMemoryWriteMu.Unlock()
		t.Fatal("live action-layer probe must start with the player grounded")
	}
	recoveryBinding = binding
	recoveryGroundY = groundY
	recoveryReady = true
	hoverState := runtimeSpatialFlightAnchorState{Active: true, TakeoffSeen: true, GroundY: groundY, PeakY: groundY, PreviousY: groundY, TargetY: groundY + 4, Binding: binding}
	app.runtimePatchMu.Lock()
	err = app.syncRuntimeSpatialFlightHookLocked(info.OwnerToken, binding, hoverState, runtimeSpatialFlightModeVirtualGround)
	lease := app.runtimeSpatialFlightHookLease
	app.runtimePatchMu.Unlock()
	app.procMu.Unlock()
	liveMemoryWriteMu.Unlock()
	if err != nil || lease == nil {
		t.Fatalf("install bounded action-layer probe: lease=%+v error=%v", lease, err)
	}

	waitSnapshot := func(after uint64) runtimeSpatialFloorQuerySnapshot {
		for attempt := 0; attempt < 3000; attempt++ {
			snapshot, readErr := readRuntimeSpatialFloorQuerySnapshot(memory, lease.CaveAddr)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if snapshot.Sequence > after {
				return snapshot
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("floor-query snapshot sequence did not advance beyond %d", after)
		return runtimeSpatialFloorQuerySnapshot{}
	}
	t.Log("press the game's real jump key once; the probe will restore the starting height on exit")
	hoverSnapshot := waitSnapshot(0)
	currentY, err := readRuntimeSpatialFlightFloat(memory, lease.HeightAddr, "live hover height")
	if err != nil {
		t.Fatal(err)
	}
	if err := memory.ReadAt(binding.Controller+runtimeSpatialControllerGroundedOffset, grounded); err != nil {
		t.Fatal(err)
	}
	t.Logf("hover returned=%t seq=%d grounded=%d y=%v target=%v hit=%s", hoverSnapshot.Returned, hoverSnapshot.Sequence, grounded[0], currentY, hoverState.TargetY, hex.EncodeToString(hoverSnapshot.Data[:]))
	if math.Abs(float64(currentY-hoverState.TargetY)) > 0.5 {
		t.Fatalf("position layer did not hold before action-layer assertion: current=%v target=%v", currentY, hoverState.TargetY)
	}
	if grounded[0] == 0 {
		t.Fatal("hover position is stable but ExFall remains airborne; ground locomotion and skills cannot activate")
	}
}
