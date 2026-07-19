package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func newMonsterEnhanceDetachTestApp(t *testing.T) (*App, uintptr, uintptr, []byte, []byte) {
	t.Helper()
	current := windows.CurrentProcess()
	page, err := virtualAllocRemote(current, 0x2000, windows.PAGE_EXECUTE_READWRITE)
	if err != nil {
		t.Fatal(err)
	}
	hProcess, err := windows.OpenProcess(windows.PROCESS_ALL_ACCESS, false, uint32(os.Getpid()))
	if err != nil {
		_ = virtualFreeRemote(current, page)
		t.Fatal(err)
	}
	hook := page
	cave := page + 0x400
	original := append([]byte(nil), findMonsterPatchPoint("monster_hp").Original...)
	patch, err := makeRelJump(hook, cave, len(original))
	if err != nil {
		windows.CloseHandle(hProcess)
		_ = virtualFreeRemote(current, page)
		t.Fatal(err)
	}
	markerAddr := monsterEnhanceCaveMarkerAddress(cave, monsterEnhanceCaveSize("monster_hp"))
	if err := writeProcessMemory(current, markerAddr, unsafe.Pointer(&monsterEnhanceCaveMarker[0]), uintptr(len(monsterEnhanceCaveMarker))); err != nil {
		windows.CloseHandle(hProcess)
		_ = virtualFreeRemote(current, page)
		t.Fatal(err)
	}
	if err := writeCodeMemory(hProcess, hook, patch); err != nil {
		windows.CloseHandle(hProcess)
		_ = virtualFreeRemote(current, page)
		t.Fatal(err)
	}
	app := &App{
		hProcess:        hProcess,
		moduleBase:      page,
		charaPID:        uint32(os.Getpid()),
		charaCreated:    1,
		charaOwnerToken: "monster-current",
		monsterEnhanceOwned: map[string]monsterEnhanceOwnedPatch{
			"monster_hp": {
				OwnerToken: "monster-current",
				Target:     hook,
				Original:   append([]byte(nil), original...),
				Patched:    append([]byte(nil), patch...),
				Cave:       cave,
				CaveSize:   monsterEnhanceCaveSize("monster_hp"),
			},
		},
	}
	t.Cleanup(func() {
		_ = writeCodeMemory(windows.CurrentProcess(), hook, original)
		if app.hProcess != 0 {
			windows.CloseHandle(app.hProcess)
			app.hProcess = 0
		}
		if err := virtualFreeRemote(current, page); err != nil {
			t.Errorf("free monster-enhance test page: %v", err)
		}
	})
	return app, hook, cave, original, patch
}

func TestCharaReleaseRestoresOnlyItsOwnedMonsterHook(t *testing.T) {
	app, hook, _, original, _ := newMonsterEnhanceDetachTestApp(t)

	if err := app.CharaRelease("monster-current"); err != nil {
		t.Fatal(err)
	}
	if got := readOverLimitDetachTestBytes(t, hook, len(original)); !bytes.Equal(got, original) {
		t.Fatalf("monster hook after release = % X, want % X", got, original)
	}
	if app.hProcess != 0 || len(app.monsterEnhanceOwned) != 0 || app.charaOwnerToken != "" {
		t.Fatalf("successful release retained monster/process state: handle=%v records=%d owner=%q", app.hProcess, len(app.monsterEnhanceOwned), app.charaOwnerToken)
	}
}

func TestStaleCharaReleaseCannotRestoreCurrentMonsterHook(t *testing.T) {
	app, hook, _, _, patch := newMonsterEnhanceDetachTestApp(t)

	if err := app.CharaRelease("monster-stale"); err != nil {
		t.Fatal(err)
	}
	if got := readOverLimitDetachTestBytes(t, hook, len(patch)); !bytes.Equal(got, patch) {
		t.Fatalf("stale release changed monster hook: got % X want % X", got, patch)
	}
	if app.hProcess == 0 || len(app.monsterEnhanceOwned) != 1 || app.charaOwnerToken != "monster-current" {
		t.Fatal("stale release discarded the current monster recovery lease")
	}
}

func TestCharaReleaseClearsMonsterRecoveryLeaseAfterGameAlreadyExited(t *testing.T) {
	app := &App{
		moduleBase:      0x140000000,
		charaPID:        42,
		charaCreated:    100,
		charaOwnerToken: "monster-current",
		monsterEnhanceOwned: map[string]monsterEnhanceOwnedPatch{
			"monster_hp": {OwnerToken: "monster-current", Target: 0x140000100, Original: []byte{1}, Patched: []byte{2}},
		},
	}

	if err := app.CharaRelease("monster-current"); err != nil {
		t.Fatalf("an exited game no longer needs in-process hook restoration: %v", err)
	}
	if app.moduleBase != 0 || app.charaPID != 0 || app.charaOwnerToken != "" || len(app.monsterEnhanceOwned) != 0 {
		t.Fatalf("exited-game release retained state: module=0x%X pid=%d owner=%q records=%d", app.moduleBase, app.charaPID, app.charaOwnerToken, len(app.monsterEnhanceOwned))
	}
}

func TestMonsterHookReleaseRejectsCorruptCaveMarkerAndKeepsRetryState(t *testing.T) {
	app, hook, cave, _, patch := newMonsterEnhanceDetachTestApp(t)
	corrupt := []byte{0xCC}
	markerAddr := monsterEnhanceCaveMarkerAddress(cave, monsterEnhanceCaveSize("monster_hp"))
	if err := writeProcessMemory(windows.CurrentProcess(), markerAddr, unsafe.Pointer(&corrupt[0]), uintptr(len(corrupt))); err != nil {
		t.Fatal(err)
	}

	err := app.CharaRelease("monster-current")
	if err == nil || !strings.Contains(err.Error(), "monster") {
		t.Fatalf("release should reject a corrupt monster cave marker, got %v", err)
	}
	if got := readOverLimitDetachTestBytes(t, hook, len(patch)); !bytes.Equal(got, patch) {
		t.Fatalf("failed release overwrote hook entry: got % X want % X", got, patch)
	}
	if app.hProcess == 0 || len(app.monsterEnhanceOwned) != 1 || app.charaOwnerToken != "monster-current" {
		t.Fatal("failed release discarded retryable monster recovery state")
	}
}

func TestMonsterHookReleaseRejectsForeignEntryAndKeepsRetryState(t *testing.T) {
	app, hook, cave, _, _ := newMonsterEnhanceDetachTestApp(t)
	foreignPatch, err := makeRelJump(hook, cave+0x100, len(findMonsterPatchPoint("monster_hp").Original))
	if err != nil {
		t.Fatal(err)
	}
	if err := writeCodeMemory(windows.CurrentProcess(), hook, foreignPatch); err != nil {
		t.Fatal(err)
	}

	err = app.CharaRelease("monster-current")
	if err == nil || !strings.Contains(err.Error(), "monster") {
		t.Fatalf("release should reject a foreign monster jump, got %v", err)
	}
	if got := readOverLimitDetachTestBytes(t, hook, len(foreignPatch)); !bytes.Equal(got, foreignPatch) {
		t.Fatalf("failed release overwrote foreign entry: got % X want % X", got, foreignPatch)
	}
	if app.hProcess == 0 || len(app.monsterEnhanceOwned) != 1 || app.charaOwnerToken != "monster-current" {
		t.Fatal("foreign entry failure discarded retryable monster recovery state")
	}
}

func TestCharaDetachRestoresOwnedMonsterHookDuringShutdown(t *testing.T) {
	app, hook, _, original, _ := newMonsterEnhanceDetachTestApp(t)

	if err := app.CharaDetach(); err != nil {
		t.Fatal(err)
	}
	if got := readOverLimitDetachTestBytes(t, hook, len(original)); !bytes.Equal(got, original) {
		t.Fatalf("monster hook after detach = % X, want % X", got, original)
	}
	if app.hProcess != 0 || len(app.monsterEnhanceOwned) != 0 {
		t.Fatal("shutdown detach retained monster recovery state")
	}
}

func TestEmbeddedPatchCoreCarriesMonsterHookOwnershipMarker(t *testing.T) {
	if !bytes.Contains(patchCoreDLL, monsterEnhanceCaveMarker) {
		t.Fatalf("embedded patch_core.dll does not carry marker %q", monsterEnhanceCaveMarker)
	}
}
