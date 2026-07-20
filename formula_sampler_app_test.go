package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStableFormulaStatusCaptureRequiresThreeBitExactSnapshots(t *testing.T) {
	frames := [][]byte{
		bytes.Repeat([]byte{1}, formulaStatusObjectScanSize),
		bytes.Repeat([]byte{2}, formulaStatusObjectScanSize),
		bytes.Repeat([]byte{1}, formulaStatusObjectScanSize),
	}
	reads := 0
	validations := 0
	_, err := captureStableFormulaStatusSnapshots(
		func() error { validations++; return nil },
		func() ([]byte, error) { frame := frames[reads]; reads++; return frame, nil },
	)
	if err == nil {
		t.Fatal("bit-different status snapshots were accepted")
	}
	if reads != 3 || validations != 2 {
		t.Fatalf("reads=%d validations=%d, want 3/2", reads, validations)
	}
}

func TestFormulaSamplerSessionRollsBackPanelEventWhenStatusCaptureFails(t *testing.T) {
	panel := RuntimeCharacterPanelStats{CharacterHash: "AABBCCDD", HP: 1, Attack: 2, StunPower: 3, CritRate: 4}
	sampler := newFormulaSampler(func() error { return nil }, func() (RuntimeCharacterPanelStats, error) { return panel, nil })
	session := &formulaSamplerSession{
		sampler:       sampler,
		raw:           make(map[FormulaSamplePhase][]byte),
		captureStatus: func() ([]byte, error) { return nil, fmt.Errorf("unstable status") },
	}
	if _, err := session.capture(FormulaPhaseA1); err == nil {
		t.Fatal("failed raw capture was accepted")
	}
	if len(sampler.events) != 0 || len(session.raw) != 0 {
		t.Fatalf("failed capture left partial state: events=%d raw=%d", len(sampler.events), len(session.raw))
	}
}

func TestFormulaSamplerSessionRejectsPanelChangeAcrossRawCapture(t *testing.T) {
	before := RuntimeCharacterPanelStats{CharacterHash: "AABBCCDD", HP: 1, Attack: 2, StunPower: 3, CritRate: 4}
	after := before
	after.Attack = 9
	reads := 0
	sampler := newFormulaSampler(func() error { return nil }, func() (RuntimeCharacterPanelStats, error) {
		reads++
		if reads <= formulaSamplerStableSnapshotCount {
			return before, nil
		}
		return after, nil
	})
	session := &formulaSamplerSession{
		sampler:         sampler,
		raw:             make(map[FormulaSamplePhase][]byte),
		isMappedAddress: func(uint64) (bool, error) { return false, nil },
		captureStatus: func() ([]byte, error) {
			return bytes.Repeat([]byte{1}, formulaStatusObjectScanSize), nil
		},
	}
	if _, err := session.capture(FormulaPhaseA1); err == nil || !strings.Contains(err.Error(), "panel") {
		t.Fatalf("panel/raw race was accepted: %v", err)
	}
	if len(sampler.events) != 0 || len(session.raw) != 0 {
		t.Fatalf("raced capture left partial state: events=%d raw=%d", len(sampler.events), len(session.raw))
	}
}

func TestFormulaSamplerStatusReturnsDefensiveEventCopy(t *testing.T) {
	app := &App{}
	if status := app.FormulaSamplerStatus(); status.Connected || len(status.Events) != 0 {
		t.Fatalf("empty app returned connected status: %+v", status)
	}
	events, raw := completeFormulaExperimentFixture(t)
	session := &formulaSamplerSession{
		sampler: &formulaSampler{events: append([]FormulaSampleEvent(nil), events...)},
		raw:     raw,
		token:   "formula-1",
	}
	status := session.status()
	if !status.Connected || !status.Complete || len(status.Events) != 4 {
		t.Fatalf("unexpected sampler status: %+v", status)
	}
	status.Events[0].Panel.HP = 999
	if session.sampler.events[0].Panel.HP == 999 {
		t.Fatal("status exposed mutable session event storage")
	}
}

func TestFormulaSamplerOwnedOperationsRejectEmptyOwnerToken(t *testing.T) {
	app := &App{formulaSamplerSession: &formulaSamplerSession{token: "formula-2", sampler: &formulaSampler{}}}
	if _, err := app.FormulaSamplerCaptureOwned("", FormulaPhaseA1); err == nil {
		t.Fatal("capture accepted an empty owner token")
	}
	if err := app.FormulaSamplerCloseOwned(""); err == nil {
		t.Fatal("close accepted an empty owner token")
	}
	if app.formulaSamplerSession == nil {
		t.Fatal("empty owner token cleared the active session")
	}
}

func TestFormulaSamplerCloseCancelsCaptureWithoutWaitingForCaptureTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	var once sync.Once
	panel := RuntimeCharacterPanelStats{CharacterHash: "AABBCCDD", HP: 1, Attack: 2, StunPower: 3, CritRate: 4}
	session := &formulaSamplerSession{
		token:   "formula-3",
		ctx:     ctx,
		cancel:  cancel,
		sampler: newFormulaSampler(func() error { return nil }, func() (RuntimeCharacterPanelStats, error) { return panel, nil }),
		captureStatus: func() ([]byte, error) {
			once.Do(func() { close(started) })
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	app := &App{formulaSamplerSession: session}
	captured := make(chan error, 1)
	go func() {
		_, err := app.FormulaSamplerCaptureOwned("formula-3", FormulaPhaseA1)
		captured <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("capture did not start")
	}
	closed := make(chan error, 1)
	go func() { closed <- app.FormulaSamplerCloseOwned("formula-3") }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("close could not cancel the in-flight capture")
	}
	select {
	case err := <-captured:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled capture returned %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled capture did not return")
	}
}

func TestFormulaSamplerOwnedCloseCannotCloseReplacementSession(t *testing.T) {
	app := &App{formulaSamplerSession: &formulaSamplerSession{token: "formula-2", sampler: &formulaSampler{}}}
	if err := app.FormulaSamplerCloseOwned("formula-1"); err == nil {
		t.Fatal("stale formula sampler owner closed a replacement session")
	}
	if app.formulaSamplerSession == nil {
		t.Fatal("replacement session was cleared by a stale owner")
	}
	if err := app.FormulaSamplerCloseOwned("formula-2"); err != nil {
		t.Fatal(err)
	}
	if app.formulaSamplerSession != nil {
		t.Fatal("current owner did not close its formula sampler session")
	}
}
