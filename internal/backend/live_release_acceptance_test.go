package backend

import (
	"os"
	"strings"
	"testing"
)

func TestLiveReleaseAcceptance(t *testing.T) {
	if os.Getenv("GBFR_LIVE_RELEASE_ACCEPTANCE") != "1" {
		t.Skip("set GBFR_LIVE_RELEASE_ACCEPTANCE=1 to run against the current game process")
	}

	app := NewApp()

	t.Run("camera lifecycle", func(t *testing.T) {
		before, err := app.GetCameraWorkspace("")
		if err != nil {
			t.Fatal(err)
		}
		if before.RecoveryRequired || (before.Installed && !before.Owned) {
			t.Fatalf("camera starts in an unsafe state: %+v", before)
		}
		if _, err := app.DeployCameraMod(CameraDeployRequest{
			MaxDistance:  before.Config.MaxDistance,
			TargetHeight: before.Config.TargetHeight,
			ZoomStep:     before.Config.ZoomStep,
		}); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = app.RemoveCameraMod("") })
		active, err := app.GetCameraWorkspace("")
		if err != nil {
			t.Fatal(err)
		}
		if !active.Installed || !active.Owned || active.RecoveryRequired || !strings.EqualFold(active.State, "active") {
			t.Fatalf("camera did not reach an owned active state: %+v", active)
		}
		if err := app.RemoveCameraMod(""); err != nil {
			t.Fatal(err)
		}
		stopped, err := app.GetCameraWorkspace("")
		if err != nil {
			t.Fatal(err)
		}
		if stopped.Installed || stopped.Owned || stopped.RecoveryRequired || !strings.EqualFold(stopped.State, "inactive") {
			t.Fatalf("camera did not restore cleanly: %+v", stopped)
		}
	})

	t.Run("audio lifecycle", func(t *testing.T) {
		before, err := app.GetAudioMixerWorkspace("")
		if err != nil {
			t.Fatal(err)
		}
		if before.RecoveryRequired || (before.Installed && !before.Owned) {
			t.Fatalf("audio starts in an unsafe state: %+v", before)
		}
		if err := app.DeployAudioMixerMod(AudioMixerDeployRequest{
			Diagnostic: before.Diagnostic,
			Volumes:    before.Volumes,
			UIVolume:   before.UIVolume,
		}); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = app.RemoveAudioMixerMod("") })
		active, err := app.GetAudioMixerWorkspace("")
		if err != nil {
			t.Fatal(err)
		}
		if !active.Installed || !active.Owned || active.RecoveryRequired || !strings.EqualFold(active.State, "active") {
			t.Fatalf("audio did not reach an owned active state: %+v", active)
		}
		if err := app.RemoveAudioMixerMod(""); err != nil {
			t.Fatal(err)
		}
		stopped, err := app.GetAudioMixerWorkspace("")
		if err != nil {
			t.Fatal(err)
		}
		if stopped.Installed || stopped.Owned || stopped.RecoveryRequired || !strings.EqualFold(stopped.State, "inactive") {
			t.Fatalf("audio did not restore cleanly: %+v", stopped)
		}
	})

	t.Run("qol lifecycle", func(t *testing.T) {
		before, err := app.GetRuntimeQOLWorkspace()
		if err != nil {
			t.Fatal(err)
		}
		if before.RecoveryRequired || (before.Installed && !before.Owned) {
			t.Fatalf("qol starts in an unsafe state: %+v", before)
		}
		config := before.Config
		config.NormalQuestLevelSync = false
		config.ReturnWrightstone = false
		active, err := app.DeployRuntimeQOL(config)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = app.RemoveRuntimeQOL("") })
		if !active.Active {
			t.Fatalf("qol did not become active: %+v", active)
		}
		observed, err := app.GetRuntimeQOLWorkspace()
		if err != nil {
			t.Fatal(err)
		}
		if !observed.Active || !observed.Installed || !observed.Owned || observed.RecoveryRequired || !strings.EqualFold(observed.State, "active") {
			t.Fatalf("qol did not reach an owned active state: %+v", observed)
		}
		if err := app.RemoveRuntimeQOL(""); err != nil {
			t.Fatal(err)
		}
		stopped, err := app.GetRuntimeQOLWorkspace()
		if err != nil {
			t.Fatal(err)
		}
		if stopped.Active || stopped.Installed || stopped.Owned || stopped.RecoveryRequired || !strings.EqualFold(stopped.State, "inactive") {
			t.Fatalf("qol did not restore cleanly: %+v", stopped)
		}
	})

	t.Run("damage capture lifecycle", func(t *testing.T) {
		if snapshot, err := app.RuntimeDamageCaptureStart(); err != nil {
			t.Fatal(err)
		} else if !snapshot.Active || snapshot.PID == 0 {
			t.Fatalf("damage capture did not become active: %+v", snapshot)
		}
		t.Cleanup(func() { _ = app.RuntimeDamageCaptureStop() })
		if _, err := app.RuntimeDamageCaptureSnapshot(32); err != nil {
			t.Fatal(err)
		}
		if err := app.RuntimeDamageCaptureStop(); err != nil {
			t.Fatal(err)
		}
		stopped, err := app.RuntimeDamageCaptureSnapshot(32)
		if err != nil {
			t.Fatal(err)
		}
		if stopped.Active {
			t.Fatalf("damage capture remained active: %+v", stopped)
		}
	})

	t.Run("virtual sigil entrypoint", func(t *testing.T) {
		before, err := app.GetVirtualSigilWorkspace("", "")
		if err != nil {
			t.Fatal(err)
		}
		if before.RecoveryRequired || (before.Installed && !before.Owned) {
			t.Fatalf("virtual sigils start in an unsafe state: %+v", before)
		}
		if !before.Available || before.UnavailableReason != "" {
			t.Fatalf("virtual-sigil entrypoint remains locked: %+v", before)
		}
	})

	t.Run("spatial and gravity lifecycle", func(t *testing.T) {
		info, err := app.CharaAcquire(1)
		if err != nil {
			t.Fatal(err)
		}
		if info.OwnerToken == "" || info.PID == 0 {
			t.Fatalf("runtime lease is incomplete: %+v", info)
		}
		t.Cleanup(func() { _ = app.CharaRelease(info.OwnerToken) })
		party, err := app.RuntimePatchPartyMonitorOwned(info.OwnerToken)
		if err != nil {
			t.Fatal(err)
		}
		if len(party.Entities) == 0 || !party.Entities[0].Present {
			t.Fatal("live release acceptance requires a scene with a stable player entity")
		}
		position := party.Entities[0].Position
		move, err := app.RuntimeSpatialTeleportOwned(info.OwnerToken, position)
		if err != nil {
			t.Fatal(err)
		}
		if !move.RuntimeVerified || move.Before != move.Observed {
			t.Fatalf("same-position spatial write did not verify: %+v", move)
		}
		available, err := app.RuntimeSpatialGravityStatusOwned(info.OwnerToken)
		if err != nil {
			t.Fatal(err)
		}
		if !available.Available || available.Enabled || available.Error != "" {
			t.Fatalf("gravity entrypoint is unavailable: %+v", available)
		}
		enabled, err := app.RuntimeSpatialGravitySetEnabledOwned(info.OwnerToken, true)
		if err != nil {
			t.Fatal(err)
		}
		if !enabled.Available || !enabled.Enabled || !enabled.Owned || enabled.Error != "" {
			t.Fatalf("gravity suppression did not become active: %+v", enabled)
		}
		restored, err := app.RuntimeSpatialGravitySetEnabledOwned(info.OwnerToken, false)
		if err != nil {
			t.Fatal(err)
		}
		if !restored.Available || restored.Enabled || restored.Owned || restored.Error != "" {
			t.Fatalf("gravity suppression did not restore cleanly: %+v", restored)
		}
		if err := app.CharaRelease(info.OwnerToken); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("2.0.3 runtime patch 045 and 054 lifecycle", func(t *testing.T) {
		info, err := app.CharaAcquire(2)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = app.CharaRelease(info.OwnerToken) })
		for _, id := range []string{"runtime-patch-045", "runtime-patch-054"} {
			active, err := app.RuntimePatchSetEnabledOwned(info.OwnerToken, id, true)
			if err != nil {
				t.Fatalf("enable %s: %v", id, err)
			}
			if !active.Available || !active.Enabled || active.Error != "" || len(active.RVAs) == 0 {
				t.Fatalf("enable %s did not verify a live patch: %+v", id, active)
			}
			stopped, err := app.RuntimePatchSetEnabledOwned(info.OwnerToken, id, false)
			if err != nil {
				t.Fatalf("disable %s: %v", id, err)
			}
			if !stopped.Available || stopped.Enabled || stopped.Error != "" {
				t.Fatalf("disable %s did not restore cleanly: %+v", id, stopped)
			}
		}
		if err := app.CharaRelease(info.OwnerToken); err != nil {
			t.Fatal(err)
		}
	})
}
