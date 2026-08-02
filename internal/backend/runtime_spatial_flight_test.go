package backend

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
	"time"
)

func runtimeSpatialFlightFixture(t *testing.T) (*fakeRuntimeSpatialMemory, uintptr, runtimeGameLayout, runtimeSpatialFlightBinding) {
	t.Helper()
	base, moduleBase := newRuntimePatchPartyFixtureForLayout(t, runtimeGameLayouts[1])
	installRuntimeSpatialControllerProbeFixture(base)
	exFall := uintptr(0x21000000) + runtimeSpatialExFallSubobjectOffset
	vtable := moduleBase + 0x5CB4DE8
	base.putPtr(exFall, vtable)
	base.putPtr(vtable+runtimeSpatialExFallFloorQuerySlot, moduleBase+runtimeSpatialExFallFloorQueryRVA)
	memory := &fakeRuntimeSpatialMemory{fakeRuntimePanelMemory: base}
	binding, err := resolveRuntimeSpatialFlightBinding(memory, moduleBase, runtimeGameLayouts[1])
	if err != nil {
		t.Fatal(err)
	}
	if binding.Controller != 0x21000430 {
		t.Fatalf("flight owner=0x%X want the entity's ExFall subobject", binding.Controller)
	}
	memory.put(binding.Controller+runtimeSpatialControllerGroundedOffset, []byte{0})
	memory.putF32(binding.Controller+runtimeSpatialControllerVelocityOffset, 0)
	return memory, moduleBase, runtimeGameLayouts[1], binding
}

func runtimeSpatialFlightY(t *testing.T, memory runtimePatchPartyMemory, binding runtimeSpatialFlightBinding) float32 {
	t.Helper()
	raw := make([]byte, 4)
	if err := memory.ReadAt(binding.TransformNode+runtimePatchPartyPositionYOffset, raw); err != nil {
		t.Fatal(err)
	}
	return math.Float32frombits(binary.LittleEndian.Uint32(raw))
}

func putRuntimeSpatialFlightVelocity(memory *fakeRuntimeSpatialMemory, binding runtimeSpatialFlightBinding, value float32) {
	memory.putF32(binding.Controller+runtimeSpatialControllerVelocityOffset, value)
}

func TestRuntimeSpatialFlightAnchorWaitsForJumpApexThenHoldsItExactly(t *testing.T) {
	memory, _, _, binding := runtimeSpatialFlightFixture(t)
	initial := runtimeSpatialFlightY(t, memory, binding)
	state := runtimeSpatialFlightAnchorState{}

	memory.put(binding.Controller+runtimeSpatialControllerGroundedOffset, []byte{1})
	state, _, err := applyRuntimeSpatialFlightAnchorTickResolved(memory, binding, state, 0, 16*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	memory.put(binding.Controller+runtimeSpatialControllerGroundedOffset, []byte{0})
	putRuntimeSpatialFlightVelocity(memory, binding, 4)
	state, first, err := applyRuntimeSpatialFlightAnchorTickResolved(memory, binding, state, 0, 16*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if state.Active || first.Wrote || !state.TakeoffSeen {
		t.Fatalf("rising frame was frozen too early: state=%+v result=%+v", state, first)
	}
	memory.putF32(binding.TransformNode+runtimePatchPartyPositionYOffset, initial+2)
	putRuntimeSpatialFlightVelocity(memory, binding, 1)
	state, rising, err := applyRuntimeSpatialFlightAnchorTickResolved(memory, binding, state, 0, 16*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if state.Active || rising.Wrote || state.PeakY != initial+2 {
		t.Fatalf("rising jump was frozen below its apex: state=%+v result=%+v", state, rising)
	}
	memory.putF32(binding.TransformNode+runtimePatchPartyPositionYOffset, initial+1.9)
	putRuntimeSpatialFlightVelocity(memory, binding, -0.1)
	state, apex, err := applyRuntimeSpatialFlightAnchorTickResolved(memory, binding, state, 0, 16*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Active || !apex.Anchored || apex.Wrote || state.TargetY != initial+2 {
		t.Fatalf("apex was not captured: state=%+v result=%+v", state, apex)
	}

	// Simulate the game applying gravity between two watcher ticks. The external
	// watcher must keep the same target without fighting the game Transform; the
	// same-frame code hook consumes this target inside the game update.
	memory.putF32(binding.TransformNode+runtimePatchPartyPositionYOffset, initial+1.25)
	state, second, err := applyRuntimeSpatialFlightAnchorTickResolved(memory, binding, state, 0, 16*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Anchored || second.Wrote || runtimeSpatialFlightY(t, memory, binding) != initial+1.25 || state.TargetY != initial+2 {
		t.Fatalf("hover drifted: state=%+v result=%+v y=%v", state, second, runtimeSpatialFlightY(t, memory, binding))
	}
}

func TestRuntimeSpatialFlightAnchorUsesRealUnitsPerSecondForPageUpAndPageDown(t *testing.T) {
	memory, _, _, binding := runtimeSpatialFlightFixture(t)
	memory.put(binding.Controller+runtimeSpatialControllerGroundedOffset, []byte{1})
	initial := runtimeSpatialFlightY(t, memory, binding)
	state := runtimeSpatialFlightAnchorState{}

	state, up, err := applyRuntimeSpatialFlightAnchorTickResolved(memory, binding, state, 8, 250*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if want := initial + 2; math.Abs(float64(up.TargetHeight-want)) > 0.0001 {
		t.Fatalf("PageUp target=%v want=%v (8 units/s for 250ms)", up.TargetHeight, want)
	}
	_, down, err := applyRuntimeSpatialFlightAnchorTickResolved(memory, binding, state, -4, 250*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if want := initial + 1; math.Abs(float64(down.TargetHeight-want)) > 0.0001 {
		t.Fatalf("PageDown target=%v want=%v", down.TargetHeight, want)
	}
}

func TestRuntimeSpatialFlightAnchorArmsOnGroundUntilJumpOrPageUp(t *testing.T) {
	memory, _, _, binding := runtimeSpatialFlightFixture(t)
	memory.put(binding.Controller+runtimeSpatialControllerGroundedOffset, []byte{1})
	memory.put(binding.Controller+runtimeSpatialControllerGroundedOffset, []byte{1})
	state, result, err := applyRuntimeSpatialFlightAnchorTickResolved(memory, binding, runtimeSpatialFlightAnchorState{}, 0, 16*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if state.Active || result.Wrote || !result.Grounded {
		t.Fatalf("grounded flight should wait for takeoff: state=%+v result=%+v", state, result)
	}
	state, result, err = applyRuntimeSpatialFlightAnchorTickResolved(memory, binding, state, 8, 250*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Active || !result.Anchored || result.Wrote || result.TargetHeight <= result.GroundHeight {
		t.Fatalf("PageUp did not lift from ground: state=%+v result=%+v", state, result)
	}
}

func TestRuntimeSpatialVirtualGroundUsesPageUpWithoutEnteringJumpApexMode(t *testing.T) {
	memory, _, _, binding := runtimeSpatialFlightFixture(t)
	initial := runtimeSpatialFlightY(t, memory, binding)
	state := runtimeSpatialFlightAnchorState{}
	memory.put(binding.Controller+runtimeSpatialControllerGroundedOffset, []byte{1})
	state, _, err := applyRuntimeSpatialFlightAnchorTickResolvedMode(memory, binding, state, 0, 16*time.Millisecond, runtimeSpatialFlightModeVirtualGround)
	if err != nil {
		t.Fatal(err)
	}

	// Space/jump must remain a normal game action in virtual-ground mode. It
	// must not arm the old apex-freeze path that strands the action in Jump.
	memory.put(binding.Controller+runtimeSpatialControllerGroundedOffset, []byte{0})
	memory.putF32(binding.TransformNode+runtimePatchPartyPositionYOffset, initial+2)
	putRuntimeSpatialFlightVelocity(memory, binding, -0.1)
	state, jumped, err := applyRuntimeSpatialFlightAnchorTickResolvedMode(memory, binding, state, 0, 16*time.Millisecond, runtimeSpatialFlightModeVirtualGround)
	if err != nil {
		t.Fatal(err)
	}
	if state.Active || state.TakeoffSeen || jumped.Anchored {
		t.Fatalf("virtual ground incorrectly activated from a jump: state=%+v result=%+v", state, jumped)
	}

	// Once back on real ground, PageUp directly raises the virtual surface.
	memory.put(binding.Controller+runtimeSpatialControllerGroundedOffset, []byte{1})
	memory.putF32(binding.TransformNode+runtimePatchPartyPositionYOffset, initial)
	state, lifted, err := applyRuntimeSpatialFlightAnchorTickResolvedMode(memory, binding, state, 8, 250*time.Millisecond, runtimeSpatialFlightModeVirtualGround)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Active || !lifted.Anchored || lifted.TargetHeight <= initial {
		t.Fatalf("PageUp did not activate the virtual floor from ground: state=%+v result=%+v", state, lifted)
	}
	if !state.HeightAnchor {
		t.Fatalf("PageUp did not request a same-frame height adjustment: state=%+v", state)
	}

	// Releasing PageUp keeps the virtual contact plane active, but it must stop
	// pinning the transform. A subsequent normal jump can then rise away from
	// the plane and return through the game's own Jump -> Landing transition.
	state, released, err := applyRuntimeSpatialFlightAnchorTickResolvedMode(memory, binding, state, 0, 16*time.Millisecond, runtimeSpatialFlightModeVirtualGround)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Active || state.HeightAnchor || !released.Anchored || released.TargetHeight != lifted.TargetHeight {
		t.Fatalf("releasing PageUp did not preserve contact while releasing height: state=%+v result=%+v", state, released)
	}
}

func TestRuntimeSpatialFlightWatcherNeverWritesTransformOutsideTheGameFrame(t *testing.T) {
	memory, _, _, binding := runtimeSpatialFlightFixture(t)
	memory.put(binding.Controller+runtimeSpatialControllerGroundedOffset, []byte{1})
	positionAddress := binding.TransformNode + runtimePatchPartyPositionZOffset
	before := make([]byte, runtimeSpatialVectorBytes)
	if err := memory.ReadAt(positionAddress, before); err != nil {
		t.Fatal(err)
	}
	_, result, err := applyRuntimeSpatialFlightAnchorTickResolved(memory, binding, runtimeSpatialFlightAnchorState{}, 8, 16*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	after := make([]byte, runtimeSpatialVectorBytes)
	if err := memory.ReadAt(positionAddress, after); err != nil {
		t.Fatal(err)
	}
	if !result.Anchored || result.Wrote || memory.writes != 0 || string(before) != string(after) {
		t.Fatalf("external watcher wrote the transform: before=% X after=% X writes=%d result=%+v", before, after, memory.writes, result)
	}
}

func TestRuntimeSpatialFlightPageUpPrimesExactlyOneBoundedTakeoffWrite(t *testing.T) {
	memory, _, _, binding := runtimeSpatialFlightFixture(t)
	memory.put(binding.Controller+runtimeSpatialControllerGroundedOffset, []byte{1})
	previous := runtimeSpatialFlightAnchorState{}
	current, result, err := applyRuntimeSpatialFlightAnchorTickResolved(memory, binding, previous, 4, 250*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	lifted, err := primeRuntimeSpatialFlightTakeoff(memory, binding, previous, current, result, 4)
	if err != nil {
		t.Fatal(err)
	}
	if !lifted || memory.writes != 1 || math.Abs(float64(runtimeSpatialFlightY(t, memory, binding)-current.TargetY)) > 0.0001 {
		t.Fatalf("takeoff prime lifted=%v writes=%d currentY=%v targetY=%v", lifted, memory.writes, runtimeSpatialFlightY(t, memory, binding), current.TargetY)
	}
	lifted, err = primeRuntimeSpatialFlightTakeoff(memory, binding, current, current, result, 4)
	if err != nil || lifted || memory.writes != 1 {
		t.Fatalf("active hover repeated its external takeoff write: lifted=%v writes=%d err=%v", lifted, memory.writes, err)
	}
}

func TestRuntimeSpatialFlightAnchorRejectsStaleTransformBeforeWrite(t *testing.T) {
	memory, _, _, binding := runtimeSpatialFlightFixture(t)
	memory.putPtr(binding.TransformRoot+runtimePatchPartyTransformNodeOffset, 0x44000000)

	_, _, err := applyRuntimeSpatialFlightAnchorTickResolved(memory, binding, runtimeSpatialFlightAnchorState{}, 0, 16*time.Millisecond)
	if err == nil || memory.writes != 0 || !strings.Contains(strings.ToLower(err.Error()), "transform") {
		t.Fatalf("error=%v writes=%d", err, memory.writes)
	}
}

func TestRuntimeSpatialAerialRecoveryUsesOnlyVerifiedFinishedActions(t *testing.T) {
	state := runtimeSpatialFlightAnchorState{Active: true}
	for range 6 {
		state = updateRuntimeSpatialAerialRecovery(state, runtimeSpatialFlightModeAerial, 9, false, -0.1, 0, 100*time.Millisecond)
	}
	if state.AerialRecovery {
		t.Fatal("air attack recovery started before the guarded action duration elapsed")
	}
	state = updateRuntimeSpatialAerialRecovery(state, runtimeSpatialFlightModeAerial, 9, false, -0.1, 0, 100*time.Millisecond)
	if !state.AerialRecovery {
		t.Fatalf("finished air attack did not start a native landing handshake: %+v", state)
	}

	// Landing must be observed before Wait/Run can end the handshake.
	state = updateRuntimeSpatialAerialRecovery(state, runtimeSpatialFlightModeAerial, 0, true, 0, 0, 16*time.Millisecond)
	if !state.AerialRecovery {
		t.Fatal("recovery ended without observing Landing action 3")
	}
	state = updateRuntimeSpatialAerialRecovery(state, runtimeSpatialFlightModeAerial, 3, true, 0, 0, 16*time.Millisecond)
	state = updateRuntimeSpatialAerialRecovery(state, runtimeSpatialFlightModeAerial, 1, true, 0, 0, 16*time.Millisecond)
	if state.AerialRecovery || state.AerialLandingSeen {
		t.Fatalf("Landing -> Run did not complete the handshake: %+v", state)
	}
}

func TestRuntimeSpatialAerialRecoveryNeverTreatsEveryHighActionIDAsFlight(t *testing.T) {
	state := runtimeSpatialFlightAnchorState{Active: true}
	for range 20 {
		state = updateRuntimeSpatialAerialRecovery(state, runtimeSpatialFlightModeAerial, 40, false, -1, 0, 100*time.Millisecond)
	}
	if state.AerialRecovery || state.AerialActionElapsed != 0 {
		t.Fatalf("an unrelated skill/character action was misclassified for landing: %+v", state)
	}
	for range 20 {
		state = updateRuntimeSpatialAerialRecovery(state, runtimeSpatialFlightModeAerial, 2, false, -1, 0, 100*time.Millisecond)
	}
	if state.AerialRecovery || state.AerialActionElapsed != 0 {
		t.Fatalf("normal Jump hover was incorrectly forced to land: %+v", state)
	}

	// A verified action is not complete while it is still rising or while the
	// user is actively changing altitude.
	for range 20 {
		state = updateRuntimeSpatialAerialRecovery(state, runtimeSpatialFlightModeAerial, 4, false, 1, 0, 100*time.Millisecond)
		state = updateRuntimeSpatialAerialRecovery(state, runtimeSpatialFlightModeAerial, 4, false, -1, 8, 100*time.Millisecond)
	}
	if state.AerialRecovery {
		t.Fatalf("active/rising aerial input was interrupted by recovery: %+v", state)
	}
}
