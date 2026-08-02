package backend

import (
	"bytes"
	"os"
	"testing"
)

func runtimeSpatialJumpFixture() (uintptr, *runtimePatchFakeMemory) {
	moduleBase := uintptr(0x140000000)
	gate := moduleBase + runtimeSpatialJumpGateRVA
	check := moduleBase + runtimeSpatialJumpCheckRVA
	return moduleBase, &runtimePatchFakeMemory{data: map[uintptr][]byte{
		gate - 7:   append([]byte(nil), runtimeSpatialJumpGateContext...),
		check - 22: append([]byte(nil), runtimeSpatialJumpCheckContext...),
		gate:       append([]byte(nil), runtimeSpatialJumpGateOriginal...),
		check:      append([]byte(nil), runtimeSpatialJumpCheckOriginal...),
	}}
}

func TestRuntimeSpatialJumpPreparesBothGuarded203SitesAndRestoresThem(t *testing.T) {
	moduleBase, memory := runtimeSpatialJumpFixture()
	sites, err := prepareRuntimeSpatialJumpSites(memory, moduleBase, moduleBase+0x8000000, runtimeGameLayouts[1])
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 2 || sites[0].RVA != uint64(runtimeSpatialJumpGateRVA) || sites[1].RVA != uint64(runtimeSpatialJumpCheckRVA) {
		t.Fatalf("jump sites = %+v", sites)
	}
	if err := installRuntimePatchSites(memory, sites); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(memory.data[moduleBase+runtimeSpatialJumpGateRVA], runtimeSpatialJumpGatePatch) ||
		!bytes.Equal(memory.data[moduleBase+runtimeSpatialJumpCheckRVA], runtimeSpatialJumpCheckPatch) {
		t.Fatal("continuous jump did not atomically install both sites")
	}
	if err := restoreRuntimePatchSites(memory, sites); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(memory.data[moduleBase+runtimeSpatialJumpGateRVA], runtimeSpatialJumpGateOriginal) ||
		!bytes.Equal(memory.data[moduleBase+runtimeSpatialJumpCheckRVA], runtimeSpatialJumpCheckOriginal) {
		t.Fatal("continuous jump did not restore both original sites")
	}
}

func TestRuntimeSpatialJumpRejectsOldLayoutAndChangedContext(t *testing.T) {
	moduleBase, memory := runtimeSpatialJumpFixture()
	if _, err := prepareRuntimeSpatialJumpSites(memory, moduleBase, moduleBase+0x8000000, runtimeGameLayouts[0]); err == nil {
		t.Fatal("2.0.2 was accepted without a calibrated two-site entry")
	}
	memory.data[moduleBase+runtimeSpatialJumpGateRVA-7][0] ^= 0xFF
	if _, err := prepareRuntimeSpatialJumpSites(memory, moduleBase, moduleBase+0x8000000, runtimeGameLayouts[1]); err == nil {
		t.Fatal("changed 2.0.3 jump context was accepted")
	}
}

func TestRuntimeSpatialJumpStatusNeverTreatsOnePatchedSiteAsEnabled(t *testing.T) {
	moduleBase, memory := runtimeSpatialJumpFixture()
	process := processInstanceID{PID: 15208, Created: 99}
	memory.data[moduleBase+runtimeSpatialJumpGateRVA] = append([]byte(nil), runtimeSpatialJumpGatePatch...)
	status := readRuntimeSpatialJumpStatus(memory, moduleBase, process, "owner", nil, runtimeGameLayouts[1])
	if status.Enabled || status.Error == "" {
		t.Fatalf("partial jump patch masqueraded as enabled: %+v", status)
	}
}

func TestRuntimeSpatialJumpLive203Lifecycle(t *testing.T) {
	if os.Getenv("GBFR_SPATIAL_JUMP_QA") != "1" {
		t.Skip("set GBFR_SPATIAL_JUMP_QA=1 with GAME 2.0.3 running")
	}
	app := NewApp()
	info, err := app.CharaAcquire(1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.CharaRelease(info.OwnerToken) })
	before, err := app.RuntimeSpatialJumpStatusOwned(info.OwnerToken)
	if err != nil {
		t.Fatal(err)
	}
	if !before.Available || before.Enabled || before.Error != "" || before.GameVersion != "2.0.3" {
		t.Fatalf("continuous jump preflight = %+v", before)
	}
	active, err := app.RuntimeSpatialJumpSetEnabledOwned(info.OwnerToken, true)
	if err != nil {
		t.Fatal(err)
	}
	if !active.Enabled || !active.Available || !active.Owned || active.RecoveryPending || active.Error != "" {
		t.Fatalf("continuous jump active = %+v", active)
	}
	restored, err := app.RuntimeSpatialJumpSetEnabledOwned(info.OwnerToken, false)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Enabled || !restored.Available || restored.Owned || restored.RecoveryPending || restored.Error != "" {
		t.Fatalf("continuous jump restored = %+v", restored)
	}
	if err := app.CharaRelease(info.OwnerToken); err != nil {
		t.Fatal(err)
	}
}
