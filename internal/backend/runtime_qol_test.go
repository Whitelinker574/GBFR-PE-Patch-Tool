package backend

import (
	"context"
	"encoding/binary"
	"os"
	"sync"
	"testing"
	"time"
)

func TestNormalizeRuntimeQOLConfig(t *testing.T) {
	value := defaultRuntimeQOLConfig()
	if _, err := normalizeRuntimeQOLConfig(value); err != nil {
		t.Fatal(err)
	}
	value.EnemyHPPrecision = 5
	if _, err := normalizeRuntimeQOLConfig(value); err == nil {
		t.Fatal("expected invalid enemy precision")
	}
	value = RuntimeQOLConfig{}
	if _, err := normalizeRuntimeQOLConfig(value); err == nil {
		t.Fatal("expected empty feature selection to fail")
	}
}

func TestRuntimeQOLSessionWatcherConcurrentLifecycle(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()
	const operations = 64
	start := make(chan struct{})
	var wait sync.WaitGroup
	for index := 0; index < operations; index++ {
		wait.Add(1)
		go func(startWatcher bool) {
			defer wait.Done()
			<-start
			if startWatcher {
				app.startRuntimeQOLSessionWatcher(0)
			} else {
				app.stopRuntimeQOLSessionWatcher()
			}
		}(index%2 == 0)
	}
	close(start)
	done := make(chan struct{})
	go func() {
		wait.Wait()
		app.stopRuntimeQOLSessionWatcher()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("concurrent QOL watcher replacement deadlocked")
	}
}

func TestRuntimeQOLLiveLifecycle(t *testing.T) {
	if os.Getenv("GBFR_RUNTIME_QOL_QA") != "1" {
		t.Skip("set GBFR_RUNTIME_QOL_QA=1 with game 2.0.2 running")
	}
	app := NewApp()
	for name, config := range map[string]RuntimeQOLConfig{
		"session-only":        {SessionCapture: true, EnemyHPPrecision: 2, SBAPrecision: 2},
		"display-and-session": defaultRuntimeQOLConfig(),
		"free-captain":        {FreeCaptain: true, EnemyHPPrecision: 2, SBAPrecision: 2},
		"all-features": {
			DamageCapPercentage: true, DetailedEnemyHP: true, DetailedSBA: true, SessionCapture: true,
			FreeCaptain: true, EnemyHPPrecision: 2, SBAPrecision: 2,
		},
	} {
		t.Run(name, func(t *testing.T) {
			workspace, err := app.DeployRuntimeQOL(config)
			if err != nil {
				t.Fatal(err)
			}
			if !workspace.Active || workspace.PID == 0 {
				t.Fatalf("workspace = %+v", workspace)
			}
			if err := app.RemoveRuntimeQOL(""); err != nil {
				t.Fatal(err)
			}
			if app.runtimeCompanionActive("qol") {
				t.Fatal("QOL runtime remained active after restoration")
			}
		})
	}
}

func TestRuntimeQOLRejectsUnverifiedMutatingFeatures(t *testing.T) {
	for name, config := range map[string]RuntimeQOLConfig{
		"normal-quest-level-sync": {NormalQuestLevelSync: true, EnemyHPPrecision: 2, SBAPrecision: 2},
		"wrightstone-return":      {ReturnWrightstone: true, EnemyHPPrecision: 2, SBAPrecision: 2},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := normalizeRuntimeQOLConfig(config); err == nil {
				t.Fatal("unverified mutating feature was accepted")
			}
		})
	}
}

func TestDecodeRuntimeQOLMapping(t *testing.T) {
	data := make([]byte, runtimeQOLMappingSize)
	binary.LittleEndian.PutUint64(data[0:8], runtimeQOLMagic)
	binary.LittleEndian.PutUint32(data[8:12], runtimeQOLVersion)
	binary.LittleEndian.PutUint32(data[12:16], 1234)
	binary.LittleEndian.PutUint64(data[16:24], 4)
	copy(data[24:56], "ABCD EFGH IJKL MNOP")
	binary.LittleEndian.PutUint64(data[56:64], 7)
	pid, sequence, session, err := decodeRuntimeQOLMapping(data)
	if err != nil {
		t.Fatal(err)
	}
	if pid != 1234 || sequence != 7 || session != "ABCD EFGH IJKL MNOP" {
		t.Fatalf("decoded = %d %d %q", pid, sequence, session)
	}
}

func TestDecodeRuntimeQOLMappingRejectsInFlightSessionWrite(t *testing.T) {
	data := make([]byte, runtimeQOLMappingSize)
	binary.LittleEndian.PutUint64(data[0:8], runtimeQOLMagic)
	binary.LittleEndian.PutUint32(data[8:12], runtimeQOLVersion)
	binary.LittleEndian.PutUint64(data[16:24], 3)
	if _, _, _, err := decodeRuntimeQOLMapping(data); err == nil {
		t.Fatal("expected odd session generation to be rejected")
	}
}

func TestRuntimeQOLSessionEventRequiresCurrentProcessAndNewSequence(t *testing.T) {
	process := processInstanceID{PID: 42, Created: 100}
	status := runtimeCompanionStatus{PID: 42, Created: 100, State: "active"}
	event, ok := runtimeQOLSessionEventAfter(6, status, process, 42, 7, "ABCD EFGH IJKL MNOP")
	if !ok || event.Sequence != 7 || event.SessionID != "ABCD EFGH IJKL MNOP" {
		t.Fatalf("event = %+v, %v", event, ok)
	}
	for name, candidate := range map[string]struct {
		last       uint64
		status     runtimeCompanionStatus
		mappingPID uint32
		sequence   uint64
		session    string
	}{
		"same sequence": {last: 7, status: status, mappingPID: 42, sequence: 7, session: event.SessionID},
		"reused pid":    {last: 0, status: runtimeCompanionStatus{PID: 42, Created: 99, State: "active"}, mappingPID: 42, sequence: 7, session: event.SessionID},
		"wrong mapping": {last: 0, status: status, mappingPID: 43, sequence: 7, session: event.SessionID},
		"empty session": {last: 0, status: status, mappingPID: 42, sequence: 7},
	} {
		t.Run(name, func(t *testing.T) {
			if _, ok := runtimeQOLSessionEventAfter(candidate.last, candidate.status, process, candidate.mappingPID, candidate.sequence, candidate.session); ok {
				t.Fatal("unexpected session event")
			}
		})
	}
}
