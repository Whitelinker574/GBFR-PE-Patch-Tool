package backend

import (
	"fmt"
	"math"
	"testing"
)

func TestFormulaSamplerRequiresThreeBitExactPanelSnapshots(t *testing.T) {
	frames := []RuntimeCharacterPanelStats{
		{CharacterHash: "AABBCCDD", HP: 100, Attack: 200, StunPower: 0, CritRate: 5},
		{CharacterHash: "AABBCCDD", HP: 100, Attack: 200, StunPower: math.Float32frombits(0x80000000), CritRate: 5},
		{CharacterHash: "AABBCCDD", HP: 100, Attack: 200, StunPower: 0, CritRate: 5},
	}
	index := 0
	_, err := readStableFormulaPanelSnapshots(func() (RuntimeCharacterPanelStats, error) {
		frame := frames[index]
		index++
		return frame, nil
	})
	if err == nil {
		t.Fatal("numerically equal but bit-different float snapshots were accepted")
	}
	if index != 3 {
		t.Fatalf("sampler read %d frames, want exactly 3", index)
	}
}

func TestFormulaSamplerCompletesReversibleABABExperiment(t *testing.T) {
	baseline := RuntimeCharacterPanelStats{CharacterHash: "AABBCCDD", HP: 100, Attack: 200, StunPower: 3, CritRate: 4}
	changed := baseline
	changed.Attack = 250
	frames := []RuntimeCharacterPanelStats{
		baseline, baseline, baseline,
		changed, changed, changed,
		baseline, baseline, baseline,
		changed, changed, changed,
	}
	readIndex := 0
	validateCalls := 0
	sampler := newFormulaSampler(
		func() error { validateCalls++; return nil },
		func() (RuntimeCharacterPanelStats, error) {
			if readIndex >= len(frames) {
				return RuntimeCharacterPanelStats{}, fmt.Errorf("unexpected read")
			}
			frame := frames[readIndex]
			readIndex++
			return frame, nil
		},
	)

	for _, phase := range []FormulaSamplePhase{FormulaPhaseA1, FormulaPhaseB1, FormulaPhaseA2, FormulaPhaseB2} {
		if _, err := sampler.Capture(phase); err != nil {
			t.Fatalf("capture %s: %v", phase, err)
		}
	}
	if !sampler.Complete() {
		t.Fatal("A/B/A/B experiment did not complete")
	}
	if readIndex != 12 {
		t.Fatalf("sampler read %d frames, want 12", readIndex)
	}
	if validateCalls != 8 {
		t.Fatalf("sampler validated process %d times, want before and after each phase", validateCalls)
	}
}

func TestFormulaSamplerDoesNotTreatMetadataOnlyChangesAsPhaseB(t *testing.T) {
	baseline := RuntimeCharacterPanelStats{
		CharacterHash: "AABBCCDD", HP: 100, Attack: 200, StunPower: 3, CritRate: 4, Source: "first",
	}
	metadataOnly := baseline
	metadataOnly.Source = "second"
	frames := []RuntimeCharacterPanelStats{
		baseline, baseline, baseline,
		metadataOnly, metadataOnly, metadataOnly,
	}
	index := 0
	sampler := newFormulaSampler(func() error { return nil }, func() (RuntimeCharacterPanelStats, error) {
		frame := frames[index]
		index++
		return frame, nil
	})
	if _, err := sampler.Capture(FormulaPhaseA1); err != nil {
		t.Fatal(err)
	}
	if _, err := sampler.Capture(FormulaPhaseB1); err == nil {
		t.Fatal("metadata-only change was accepted as a formula transition")
	}
}

func TestFormulaSamplerControlExperimentRequiresFourUnchangedPhases(t *testing.T) {
	panel := RuntimeCharacterPanelStats{CharacterHash: "AABBCCDD", HP: 100, Attack: 200, StunPower: 3, CritRate: 4}
	sampler := newFormulaSamplerForExperiment("control", func() error { return nil }, func() (RuntimeCharacterPanelStats, error) {
		return panel, nil
	})
	for _, phase := range formulaSamplePhaseOrder {
		if _, err := sampler.Capture(phase); err != nil {
			t.Fatalf("control capture %s: %v", phase, err)
		}
	}
	if !sampler.Complete() {
		t.Fatal("unchanged control experiment did not complete")
	}
}

func TestFormulaSamplerAllowsUnchangedKnownPanelForDefenseCandidateScan(t *testing.T) {
	panel := RuntimeCharacterPanelStats{CharacterHash: "AABBCCDD", HP: 100, Attack: 200, StunPower: 3, CritRate: 4}
	sampler := newFormulaSamplerForExperiment("defense", func() error { return nil }, func() (RuntimeCharacterPanelStats, error) {
		return panel, nil
	})
	for _, phase := range formulaSamplePhaseOrder {
		if _, err := sampler.Capture(phase); err != nil {
			t.Fatalf("defense scan capture %s: %v", phase, err)
		}
	}
	if !sampler.Complete() {
		t.Fatal("unchanged known panel prevented the defense candidate scan from completing")
	}
}

func TestFormulaSamplerAllowsUnchangedKnownPanelForMasteryCandidateScan(t *testing.T) {
	panel := RuntimeCharacterPanelStats{CharacterHash: "AABBCCDD", HP: 100, Attack: 200, StunPower: 3, CritRate: 4}
	sampler := newFormulaSamplerForExperiment("mastery", func() error { return nil }, func() (RuntimeCharacterPanelStats, error) {
		return panel, nil
	})
	for _, phase := range formulaSamplePhaseOrder {
		if _, err := sampler.Capture(phase); err != nil {
			t.Fatalf("mastery scan capture %s: %v", phase, err)
		}
	}
}
