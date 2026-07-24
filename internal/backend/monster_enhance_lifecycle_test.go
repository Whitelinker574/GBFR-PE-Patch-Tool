package backend

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

func TestMonsterPatchCatalogOnlyEnablesVerifiedCurrentLayouts(t *testing.T) {
	wantAvailable := map[string]bool{
		"monster_hp":       true,
		"monster_damage":   true,
		"monster_stun":     true,
		"overdrive_state":  true,
		"inventory_set_45": true,
	}
	for _, point := range monsterPatchPoints {
		if point.Available != wantAvailable[point.ID] {
			t.Errorf("%s available = %v, want %v", point.ID, point.Available, wantAvailable[point.ID])
		}
		if !point.Available && point.UnavailableReason == "" {
			t.Errorf("%s is unavailable without an evidence boundary", point.ID)
		}
	}

	damage := findMonsterPatchPoint("monster_damage")
	if damage.RVA != 0x1FBDEB4 || !bytes.Equal(damage.Original, []byte{0x81, 0xBE, 0xD4, 0x00, 0x00, 0x00, 0x00, 0xE1, 0xF5, 0x05}) {
		t.Fatalf("monster damage layout is not the v1.8.6/current-EXE layout: %+v", damage)
	}
	stun := findMonsterPatchPoint("monster_stun")
	if stun.RVA != 0xB29128 || !bytes.Equal(stun.Original, []byte{0xC5, 0xFA, 0x58, 0x86, 0x60, 0x08, 0x00, 0x00}) {
		t.Fatalf("monster stun layout is not the v1.8.6/current-EXE layout: %+v", stun)
	}
	overdrive := findMonsterPatchPoint("overdrive_state")
	if overdrive.RVA != 0x22CB316 || !bytes.Equal(overdrive.Original, []byte{0x8B, 0x46, 0x10, 0x83, 0xF8, 0x03, 0x0F, 0x84, 0xC7, 0x00, 0x00, 0x00}) {
		t.Fatalf("overdrive layout is not the v1.8.6/current-EXE layout: %+v", overdrive)
	}
}

func TestFailedMonsterEnableRollbackRestoresMarkedHook(t *testing.T) {
	app, hook, _, original, _ := newMonsterEnhanceDetachTestApp(t)
	app.monsterEnhanceOwned = nil
	point := findMonsterPatchPoint("monster_hp")
	app.moduleBase = hook - point.RVA
	if err := app.rollbackMonsterEnhanceFailedEnable("monster-current", point, original); err != nil {
		t.Fatal(err)
	}
	if got := readOverLimitDetachTestBytes(t, hook, len(original)); !bytes.Equal(got, original) {
		t.Fatalf("failed enable rollback = % X, want % X", got, original)
	}
	if len(app.monsterEnhanceOwned) != 0 {
		t.Fatal("successful failed-enable rollback retained an ownership record")
	}
}

func TestFailedMonsterDamageEnableRollsBackAuxiliaryPlayerHook(t *testing.T) {
	current := windows.CurrentProcess()
	page, err := virtualAllocRemote(current, 0x4000, windows.PAGE_EXECUTE_READWRITE)
	if err != nil {
		t.Fatal(err)
	}
	hProcess, err := windows.OpenProcess(windows.PROCESS_ALL_ACCESS, false, uint32(os.Getpid()))
	if err != nil {
		_ = virtualFreeRemote(current, page)
		t.Fatal(err)
	}
	t.Cleanup(func() {
		windows.CloseHandle(hProcess)
		if err := virtualFreeRemote(current, page); err != nil {
			t.Errorf("free monster damage rollback page: %v", err)
		}
	})

	point := findMonsterPatchPoint("monster_damage")
	mainTarget, mainCave := page, page+0x800
	auxTarget, auxCave := page+0x100, page+0x1000
	mainOriginal := append([]byte(nil), point.Original...)
	auxOriginal := []byte{0x48, 0x81, 0xC1, 0x34, 0x12, 0x00, 0x00}
	mainPatch, err := makeRelJump(mainTarget, mainCave, len(mainOriginal))
	if err != nil {
		t.Fatal(err)
	}
	auxPatch, err := makeRelJump(auxTarget, auxCave, len(auxOriginal))
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []uintptr{
		monsterEnhanceCaveMarkerAddress(mainCave, monsterEnhanceCaveSize(point.ID)),
		monsterEnhanceCaveMarkerAddress(auxCave, 96),
	} {
		if err := writeProcessMemory(current, marker, unsafe.Pointer(&monsterEnhanceCaveMarker[0]), uintptr(len(monsterEnhanceCaveMarker))); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeCodeMemory(current, mainTarget, mainPatch); err != nil {
		t.Fatal(err)
	}
	if err := writeCodeMemory(current, auxTarget, auxPatch); err != nil {
		t.Fatal(err)
	}

	app := &App{hProcess: hProcess, moduleBase: mainTarget - point.RVA}
	aux := &monsterEnhanceAuxPreflight{Target: auxTarget, Original: auxOriginal, CaveSize: 96}
	if err := app.rollbackMonsterEnhanceFailedEnableWithAux("monster-current", point, mainOriginal, aux); err != nil {
		t.Fatal(err)
	}
	if got := readOverLimitDetachTestBytes(t, mainTarget, len(mainOriginal)); !bytes.Equal(got, mainOriginal) {
		t.Fatalf("monster damage main rollback = % X, want % X", got, mainOriginal)
	}
	if got := readOverLimitDetachTestBytes(t, auxTarget, len(auxOriginal)); !bytes.Equal(got, auxOriginal) {
		t.Fatalf("monster damage auxiliary rollback = % X, want % X", got, auxOriginal)
	}
	if len(app.monsterEnhanceOwned) != 0 {
		t.Fatal("monster damage rollback retained an ownership record")
	}
}

func TestMonsterEnhanceLiveInstallRestore(t *testing.T) {
	if os.Getenv("GBFR_RUN_MONSTER_INTEGRATION") != "1" {
		t.Skip("set GBFR_RUN_MONSTER_INTEGRATION=1 with GBFR 2.0.2 running")
	}
	app := NewApp()
	info, err := app.CharaAcquire(1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if releaseErr := app.CharaRelease(info.OwnerToken); releaseErr != nil {
			t.Errorf("release live monster integration lease: %v", releaseErr)
		}
	}()
	for _, test := range []struct {
		id    string
		value float64
	}{
		{id: "monster_hp", value: 1},
		{id: "monster_damage", value: 1},
		{id: "monster_stun", value: 1},
		{id: "overdrive_state", value: 9},
	} {
		t.Run(test.id, func(t *testing.T) {
			status, err := app.MonsterEnhanceSetPatchValueEnabledOwned(info.OwnerToken, test.id, true, test.value)
			if err != nil {
				t.Fatal(err)
			}
			if !monsterStatusHasPatch(status, test.id) {
				t.Fatalf("%s did not report enabled", test.id)
			}
			status, err = app.MonsterEnhanceSetPatchValueEnabledOwned(info.OwnerToken, test.id, false, test.value)
			if err != nil {
				t.Fatal(err)
			}
			if monsterStatusHasPatch(status, test.id) {
				t.Fatalf("%s remained enabled after restore", test.id)
			}
		})
	}
}

func TestScanMonsterPatchOriginals(t *testing.T) {
	path := os.Getenv("GBFR_EXE_PATH")
	if path == "" {
		t.Skip("set GBFR_EXE_PATH to scan a local executable")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, point := range monsterPatchPoints {
		matches := make([]int, 0, 2)
		for offset := 0; offset <= len(data)-len(point.Original); {
			relative := bytes.Index(data[offset:], point.Original)
			if relative < 0 {
				break
			}
			match := offset + relative
			matches = append(matches, match)
			offset = match + 1
		}
		t.Logf("%s configured=0x%X matches=%#x", point.ID, point.RVA, matches)
		if point.Available && len(matches) != 1 {
			t.Errorf("available %s original matched %d locations, want exactly one", point.ID, len(matches))
		}
	}
	playerMatches := findPatternMatches(data, 0, monsterDamagePlayerPointerPattern, monsterDamagePlayerPointerMask)
	if len(playerMatches) != 1 {
		t.Fatalf("monster damage player-pointer signature matched %d locations, want one", len(playerMatches))
	}
	target := playerMatches[0] + 0x14
	if target+7 > uintptr(len(data)) {
		t.Fatalf("monster damage player-pointer target %#x is outside the executable", target)
	}
	if got := data[target : target+7]; got[0] != 0x48 || got[1] != 0x81 || got[2] != 0xC1 || got[5] != 0 || got[6] != 0 {
		t.Fatalf("unexpected monster damage player-pointer instruction at raw offset %#x: % X", target, got)
	}
	t.Logf("monster damage player-pointer target raw offset=%#x", target)
}
