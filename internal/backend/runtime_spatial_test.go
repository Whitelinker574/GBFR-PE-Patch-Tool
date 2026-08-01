package backend

import (
	"bytes"
	"errors"
	"math"
	"testing"
)

type fakeRuntimeSpatialMemory struct {
	*fakeRuntimePanelMemory
	writes            int
	corruptFirstWrite bool
	failRestore       bool
	changeTopology    bool
}

func (memory *fakeRuntimeSpatialMemory) WriteAt(address uintptr, source []byte) error {
	memory.writes++
	if memory.failRestore && memory.writes > 1 {
		return errors.New("restore failed")
	}
	value := append([]byte(nil), source...)
	if memory.corruptFirstWrite && memory.writes == 1 {
		value[0] ^= 0x80
	}
	memory.put(address, value)
	if memory.changeTopology && memory.writes == 1 {
		memory.putPtr(uintptr(0x21000000)+runtimePatchPartyTransformRootOffset, uintptr(0x21009000))
	}
	return nil
}

func TestWriteRuntimeSpatialPlayerPositionUsesVerifiedZYXBlock(t *testing.T) {
	base, moduleBase := newRuntimePatchPartyFixture(t)
	memory := &fakeRuntimeSpatialMemory{fakeRuntimePanelMemory: base}
	target := RuntimePatchVector3{X: 101.25, Y: -202.5, Z: 303.75}

	result, err := writeRuntimeSpatialPlayerPosition(memory, moduleBase, target)
	if err != nil {
		t.Fatal(err)
	}
	if memory.writes != 1 || result.Before != (RuntimePatchVector3{X: 10, Y: 20, Z: 30}) || result.Observed != target || !result.RuntimeVerified || result.SnapshotCount != 3 {
		t.Fatalf("result=%+v writes=%d", result, memory.writes)
	}
	actual := make([]byte, runtimeSpatialVectorBytes)
	if err := memory.ReadAt(uintptr(0x21007000)+runtimePatchPartyPositionZOffset, actual); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, encodeRuntimeSpatialVector(target)) {
		t.Fatalf("position bytes=%X", actual)
	}
}

func TestWriteRuntimeSpatialPlayerPositionReportsDetected203Layout(t *testing.T) {
	base, moduleBase := newRuntimePatchPartyFixtureForLayout(t, runtimeGameLayouts[1])
	memory := &fakeRuntimeSpatialMemory{fakeRuntimePanelMemory: base}

	result, err := writeRuntimeSpatialPlayerPosition(memory, moduleBase, RuntimePatchVector3{X: 101.25, Y: -202.5, Z: 303.75})
	if err != nil {
		t.Fatal(err)
	}
	if result.GameVersion != "2.0.3" || result.Source != "game_runtime_spatial_2.0.3" {
		t.Fatalf("2.0.3 spatial result retained a legacy version gate: %+v", result)
	}
}

func TestWriteRuntimeSpatialPlayerDeltaReportsDetected203Layout(t *testing.T) {
	base, moduleBase := newRuntimePatchPartyFixtureForLayout(t, runtimeGameLayouts[1])
	memory := &fakeRuntimeSpatialMemory{fakeRuntimePanelMemory: base}

	result, err := writeRuntimeSpatialPlayerDelta(memory, moduleBase, RuntimePatchVector3{X: 2.5, Z: 4})
	if err != nil {
		t.Fatal(err)
	}
	if result.GameVersion != "2.0.3" || result.Source != "game_runtime_spatial_2.0.3" {
		t.Fatalf("2.0.3 spatial delta result retained a legacy version gate: %+v", result)
	}
}

func TestWriteRuntimeSpatialPlayerDeltaUsesCurrentStablePosition(t *testing.T) {
	base, moduleBase := newRuntimePatchPartyFixture(t)
	memory := &fakeRuntimeSpatialMemory{fakeRuntimePanelMemory: base}
	result, err := writeRuntimeSpatialPlayerDelta(memory, moduleBase, RuntimePatchVector3{X: 2.5, Y: -1, Z: 4})
	if err != nil {
		t.Fatal(err)
	}
	want := RuntimePatchVector3{X: 12.5, Y: 19, Z: 34}
	if result.Before != (RuntimePatchVector3{X: 10, Y: 20, Z: 30}) || result.Requested != want || result.Observed != want || memory.writes != 1 {
		t.Fatalf("result=%+v writes=%d", result, memory.writes)
	}
}

func TestWriteRuntimeSpatialPlayerDeltaRejectsZeroAndOversizedStep(t *testing.T) {
	for _, delta := range []RuntimePatchVector3{{}, {X: runtimeSpatialMaximumStep + 0.01}} {
		base, moduleBase := newRuntimePatchPartyFixture(t)
		memory := &fakeRuntimeSpatialMemory{fakeRuntimePanelMemory: base}
		if _, err := writeRuntimeSpatialPlayerDelta(memory, moduleBase, delta); err == nil || memory.writes != 0 {
			t.Fatalf("delta=%+v error=%v writes=%d", delta, err, memory.writes)
		}
	}
}

func TestWriteRuntimeSpatialPlayerPositionRejectsInvalidTargetBeforeWrite(t *testing.T) {
	base, moduleBase := newRuntimePatchPartyFixture(t)
	memory := &fakeRuntimeSpatialMemory{fakeRuntimePanelMemory: base}
	_, err := writeRuntimeSpatialPlayerPosition(memory, moduleBase, RuntimePatchVector3{X: float32(math.Inf(1))})
	if err == nil || memory.writes != 0 {
		t.Fatalf("error=%v writes=%d", err, memory.writes)
	}
}

func TestWriteRuntimeSpatialPlayerPositionRestoresOnReadbackMismatch(t *testing.T) {
	base, moduleBase := newRuntimePatchPartyFixture(t)
	memory := &fakeRuntimeSpatialMemory{fakeRuntimePanelMemory: base, corruptFirstWrite: true}
	_, err := writeRuntimeSpatialPlayerPosition(memory, moduleBase, RuntimePatchVector3{X: 1, Y: 2, Z: 3})
	if err == nil || memory.writes != 2 || errors.Is(err, errLiveMemoryRollbackUnproven) {
		t.Fatalf("error=%v writes=%d", err, memory.writes)
	}
	actual := make([]byte, runtimeSpatialVectorBytes)
	_ = memory.ReadAt(uintptr(0x21007000)+runtimePatchPartyPositionZOffset, actual)
	if !bytes.Equal(actual, encodeRuntimeSpatialVector(RuntimePatchVector3{X: 10, Y: 20, Z: 30})) {
		t.Fatalf("original position was not restored: %X", actual)
	}
}

func TestWriteRuntimeSpatialPlayerPositionPoisonsWhenRestoreCannotBeProven(t *testing.T) {
	base, moduleBase := newRuntimePatchPartyFixture(t)
	memory := &fakeRuntimeSpatialMemory{fakeRuntimePanelMemory: base, corruptFirstWrite: true, failRestore: true}
	_, err := writeRuntimeSpatialPlayerPosition(memory, moduleBase, RuntimePatchVector3{X: 1, Y: 2, Z: 3})
	if !errors.Is(err, errLiveMemoryRollbackUnproven) || memory.writes != 2 {
		t.Fatalf("error=%v writes=%d", err, memory.writes)
	}
}

func TestWriteRuntimeSpatialPlayerPositionPoisonsWhenTopologyChangesAfterWrite(t *testing.T) {
	base, moduleBase := newRuntimePatchPartyFixture(t)
	memory := &fakeRuntimeSpatialMemory{fakeRuntimePanelMemory: base, changeTopology: true}
	target := RuntimePatchVector3{X: 1, Y: 2, Z: 3}
	_, err := writeRuntimeSpatialPlayerPosition(memory, moduleBase, target)
	if !errors.Is(err, errLiveMemoryRollbackUnproven) || memory.writes != 1 {
		t.Fatalf("error=%v writes=%d", err, memory.writes)
	}
	actual := make([]byte, runtimeSpatialVectorBytes)
	_ = memory.ReadAt(uintptr(0x21007000)+runtimePatchPartyPositionZOffset, actual)
	if !bytes.Equal(actual, encodeRuntimeSpatialVector(target)) {
		t.Fatalf("the verified write should remain untouched after topology ownership is lost: %X", actual)
	}
}

func runtimeSpatialGravityFixture(t *testing.T) (uintptr, uintptr, *runtimePatchFakeMemory) {
	return runtimeSpatialGravityFixtureForLayout(t, runtimeGameLayouts[0])
}

func runtimeSpatialGravityFixtureForLayout(t *testing.T, layout runtimeGameLayout) (uintptr, uintptr, *runtimePatchFakeMemory) {
	t.Helper()
	moduleBase := uintptr(0x140000000)
	address, err := runtimeSpatialGravityAddress(moduleBase, layout)
	if err != nil {
		t.Fatal(err)
	}
	memory := newRuntimePatchFakeMemory(map[uintptr][]byte{
		address: runtimeSpatialGravityOriginal,
		address - runtimeSpatialGravityContextBack: runtimeSpatialGravityContext,
	})
	return moduleBase, address, memory
}

func TestRuntimeSpatialGravityStatusReportsDetected203Layout(t *testing.T) {
	moduleBase, _, memory := runtimeSpatialGravityFixtureForLayout(t, runtimeGameLayouts[1])
	status := readRuntimeSpatialGravityStatus(memory, moduleBase, processInstanceID{PID: 42, Created: 84}, "owner", nil, runtimeGameLayouts[1])
	if !status.Available || status.Error != "" || status.GameVersion != "2.0.3" || status.Source != "game_runtime_gravity_patch_2.0.3" || status.RVA != uint64(runtimeGameLayouts[1].SpatialGravityRVA) {
		t.Fatalf("2.0.3 gravity status retained a legacy layout: %+v", status)
	}
}

func TestRuntimeSpatialGravityVerifiedSiteEnablesAndRestores(t *testing.T) {
	moduleBase, address, memory := runtimeSpatialGravityFixture(t)
	site, err := prepareRuntimeSpatialGravitySite(memory, moduleBase, moduleBase+0x8200000)
	if err != nil {
		t.Fatal(err)
	}
	if site.Address != address || site.RVA != uint64(runtimeSpatialGravityRVA) {
		t.Fatalf("site=%+v", site)
	}
	if err := installRuntimePatchSites(memory, []runtimePatchPatchSiteLease{site}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(memory.data[address], runtimeSpatialGravityPatch) {
		t.Fatalf("enabled bytes=% X", memory.data[address])
	}
	if err := restoreRuntimePatchSites(memory, []runtimePatchPatchSiteLease{site}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(memory.data[address], runtimeSpatialGravityOriginal) {
		t.Fatalf("restored bytes=% X", memory.data[address])
	}
}

func TestRuntimeSpatialGravityRejectsExternalAndUnknownBytes(t *testing.T) {
	for name, value := range map[string][]byte{
		"external patch": runtimeSpatialGravityPatch,
		"unknown bytes":  {0xCC, 0xCC, 0xCC, 0xCC, 0xCC, 0xCC, 0xCC, 0xCC},
	} {
		t.Run(name, func(t *testing.T) {
			moduleBase, address, memory := runtimeSpatialGravityFixture(t)
			memory.data[address] = append([]byte(nil), value...)
			if _, err := prepareRuntimeSpatialGravitySite(memory, moduleBase, moduleBase+0x8200000); err == nil {
				t.Fatal("prepare error=nil")
			}
			if len(memory.writes) != 0 {
				t.Fatalf("writes=%v", memory.writes)
			}
		})
	}
}

func TestRuntimeSpatialGravityStatusRequiresMatchingOwnedLease(t *testing.T) {
	moduleBase, address, memory := runtimeSpatialGravityFixture(t)
	process := processInstanceID{PID: 42, Created: 84}
	site := runtimePatchPatchSiteLease{
		Address: address, RVA: uint64(runtimeSpatialGravityRVA),
		Original: append([]byte(nil), runtimeSpatialGravityOriginal...),
		Patch:    append([]byte(nil), runtimeSpatialGravityPatch...),
	}
	lease := runtimePatchPatchLease{
		FeatureID: runtimeSpatialGravityFeatureID, OwnerToken: "owner", Process: process,
		State: runtimePatchPatchEnabled, Sites: []runtimePatchPatchSiteLease{site},
	}
	memory.data[address] = append([]byte(nil), runtimeSpatialGravityPatch...)
	status := readRuntimeSpatialGravityStatus(memory, moduleBase, process, "owner", &lease)
	if !status.Enabled || !status.Available || !status.Owned || status.Error != "" {
		t.Fatalf("owned status=%+v", status)
	}
	foreign := readRuntimeSpatialGravityStatus(memory, moduleBase, process, "other", &lease)
	if foreign.Available || foreign.Owned || foreign.Error == "" {
		t.Fatalf("foreign status=%+v", foreign)
	}
}

func TestRuntimeSpatialGravityLeaseCountsAsActiveRuntimeHook(t *testing.T) {
	lease := runtimePatchPatchLease{FeatureID: runtimeSpatialGravityFeatureID}
	app := &App{runtimeSpatialGravityLease: &lease}
	if !app.hasActiveRuntimeHookLeaseLocked() {
		t.Fatal("gravity recovery lease must keep the shared process connection alive")
	}
}

func TestRuntimeSpatialGravityLeaseBlocksOwnerRotation(t *testing.T) {
	lease := runtimePatchPatchLease{FeatureID: runtimeSpatialGravityFeatureID}
	app := &App{runtimeSpatialGravityLease: &lease}
	if _, err := app.CharaAttach(); err == nil {
		t.Fatal("unowned attach must not replace a gravity recovery owner")
	}
	if _, err := app.CharaAcquire(1); err == nil {
		t.Fatal("owned acquire must not rotate the owner while gravity recovery is pending")
	}
}
