package backend

import (
	"bytes"
	"os"
	"testing"
)

// This opt-in probe attaches read-only, verifies the guarded 2.0.3 entry and
// closes only our process handle. It never installs the Hook and never writes
// the selected weapon record.
func TestWeaponMemoryLiveReadOnly203(t *testing.T) {
	if os.Getenv("GBFR_LIVE_WEAPON_READONLY") != "1" {
		t.Skip("set GBFR_LIVE_WEAPON_READONLY=1 while GAME 2.0.3 is running")
	}
	app := NewApp()
	t.Cleanup(func() {
		if err := app.CharaDetach(); err != nil {
			t.Errorf("read-only probe detach: %v", err)
		}
	})
	status, err := app.WeaponMemoryScan()
	if err != nil {
		t.Fatal(err)
	}
	if !status.Found || status.Hooked || status.SourceVersion != "2.0.3" || status.RVA != uint64(weaponMemoryHookRVA) {
		t.Fatalf("unexpected live read-only status: %+v", status)
	}
	if !bytes.Equal(weaponMemoryOriginalBytes, weaponMemoryGuardBytes[:weaponMemoryHookSize]) {
		t.Fatal("verified entry no longer starts with the displaced instruction pair")
	}
}

// This second opt-in probe installs only the pointer-capture Hook, confirms
// ownership, restores the exact displaced bytes, and never calls Update.
func TestWeaponMemoryLiveCaptureHookRestores203(t *testing.T) {
	if os.Getenv("GBFR_LIVE_WEAPON_CAPTURE_READONLY") != "1" {
		t.Skip("set GBFR_LIVE_WEAPON_CAPTURE_READONLY=1 for the read-only Hook lifecycle probe")
	}
	app := NewApp()
	t.Cleanup(func() {
		if app.weaponMemoryOwnerToken != "" {
			_, _ = app.WeaponMemoryRelease(app.weaponMemoryOwnerToken)
		}
		if err := app.CharaDetach(); err != nil {
			t.Errorf("read-only capture probe detach: %v", err)
		}
	})
	acquired, err := app.WeaponMemoryAcquire(1)
	if err != nil {
		t.Fatal(err)
	}
	if !acquired.Hooked || acquired.OwnerToken == "" {
		t.Fatalf("read-only weapon capture Hook was not owned: %+v", acquired)
	}
	if _, err := app.WeaponMemoryRelease(acquired.OwnerToken); err != nil {
		t.Fatal(err)
	}
	app.procMu.Lock()
	if app.weaponMemoryHookAddr != 0 || app.weaponMemoryCaveAddr != 0 || len(app.weaponMemoryOriginal) != 0 {
		app.procMu.Unlock()
		t.Fatal("successful release retained weapon recovery state")
	}
	app.procMu.Unlock()
	restored, err := app.WeaponMemoryScan()
	if err != nil {
		t.Fatal(err)
	}
	if restored.Hooked || restored.CurrentBytes != bytesToHex(weaponMemoryOriginalBytes) {
		t.Fatalf("weapon Hook entry was not restored exactly: %+v", restored)
	}
}
