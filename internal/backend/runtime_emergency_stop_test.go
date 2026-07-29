package backend

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestRuntimeEmergencyStopLiveLifecycle(t *testing.T) {
	if os.Getenv("GBFR_RUNTIME_EMERGENCY_QA") != "1" {
		t.Skip("set GBFR_RUNTIME_EMERGENCY_QA=1 with game 2.0.2 running")
	}
	app := NewApp()
	config := RuntimeQOLConfig{
		DamageCapPercentage: true, DetailedEnemyHP: true, DetailedSBA: true, SessionCapture: true,
		FreeCaptain: true, EnemyHPPrecision: 2, SBAPrecision: 2,
	}
	workspace, err := app.DeployRuntimeQOL(config)
	if err != nil {
		t.Fatal(err)
	}
	if !workspace.Active {
		t.Fatalf("workspace = %+v", workspace)
	}
	result, err := app.runtimeEmergencyStop("QA")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Restored || app.runtimeCompanionActive("qol") {
		t.Fatalf("emergency result = %+v, qol active = %v", result, app.runtimeCompanionActive("qol"))
	}
}

func TestRunRuntimeEmergencyRestorationReportsEveryFailure(t *testing.T) {
	firstErr := errors.New("formula sampler restore failed")
	secondErr := errors.New("loadout detector restore failed")
	var calls atomic.Int32
	err := runRuntimeEmergencyRestoration(
		func() error { calls.Add(1); return firstErr },
		func() error { calls.Add(1); return secondErr },
		func() error { calls.Add(1); return nil },
	)
	if calls.Load() != 3 {
		t.Fatalf("restoration calls = %d, want 3", calls.Load())
	}
	if !errors.Is(err, firstErr) || !errors.Is(err, secondErr) {
		t.Fatalf("joined error = %v", err)
	}
}

func TestRuntimeEmergencyWatcherIsEdgeTriggered(t *testing.T) {
	app := NewApp()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var down atomic.Bool
	var reads atomic.Int32
	var triggers atomic.Int32
	pressed := func(key int) bool {
		if key != virtualKeyF12 {
			t.Fatalf("key = %d", key)
		}
		reads.Add(1)
		return down.Load()
	}

	done := make(chan struct{})
	go func() {
		app.runRuntimeEmergencyWatcher(ctx, pressed, time.Millisecond, func() { triggers.Add(1) })
		close(done)
	}()

	down.Store(true)
	time.Sleep(8 * time.Millisecond)
	down.Store(false)
	time.Sleep(3 * time.Millisecond)
	down.Store(true)
	time.Sleep(8 * time.Millisecond)
	cancel()
	<-done

	if reads.Load() < 2 {
		t.Fatalf("watcher reads = %d", reads.Load())
	}
	if triggers.Load() != 2 {
		t.Fatalf("edge-trigger count = %d, want 2", triggers.Load())
	}
}

func TestRuntimeEmergencyStopIsIdempotentWithoutGame(t *testing.T) {
	app := NewApp()
	for index := 0; index < 2; index++ {
		result, err := app.RuntimeEmergencyStop()
		if err != nil {
			t.Fatalf("attempt %d: %v", index, err)
		}
		if !result.Restored || result.TriggeredAt == "" || result.Detail == "" {
			t.Fatalf("attempt %d result = %+v", index, result)
		}
	}
}
