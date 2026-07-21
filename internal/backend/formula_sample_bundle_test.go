package backend

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
	"testing"
)

func completeFormulaExperimentFixture(t *testing.T) ([]FormulaSampleEvent, map[FormulaSamplePhase][]byte) {
	t.Helper()
	panels := []RuntimeCharacterPanelStats{
		{CharacterHash: "AABBCCDD", HP: 100, Attack: 200, StunPower: 30, CritRate: 40, RuntimeVerified: true},
		{CharacterHash: "AABBCCDD", HP: 100, Attack: 250, StunPower: 30, CritRate: 40, RuntimeVerified: true},
		{CharacterHash: "AABBCCDD", HP: 100, Attack: 200, StunPower: 30, CritRate: 40, RuntimeVerified: true},
		{CharacterHash: "AABBCCDD", HP: 100, Attack: 250, StunPower: 30, CritRate: 40, RuntimeVerified: true},
	}
	for index := range panels {
		panels[index].RuntimeID = "AABBCCDD"
		panels[index].IdentitySource = "map_key"
		panels[index].RawStunPower = panels[index].StunPower / runtimeCharacterPanelStunDisplayScale
		panels[index].HPField = RuntimeCharacterPanelFieldReading{RawType: "i32", RelativeOffset: 4, RawBits: fmt.Sprintf("0x%08X", uint32(panels[index].HP)), DisplayScale: 1, StableReads: 3}
		panels[index].AttackField = RuntimeCharacterPanelFieldReading{RawType: "i32", RelativeOffset: 8, RawBits: fmt.Sprintf("0x%08X", uint32(panels[index].Attack)), DisplayScale: 1, StableReads: 3}
		panels[index].StunField = RuntimeCharacterPanelFieldReading{RawType: "f32", RelativeOffset: 0x10, RawBits: fmt.Sprintf("0x%08X", math.Float32bits(panels[index].RawStunPower)), DisplayScale: runtimeCharacterPanelStunDisplayScale, StableReads: 3}
		panels[index].CritField = RuntimeCharacterPanelFieldReading{RawType: "f32", RelativeOffset: 0x14, RawBits: fmt.Sprintf("0x%08X", math.Float32bits(panels[index].CritRate)), DisplayScale: 1, StableReads: 3}
	}
	events := make([]FormulaSampleEvent, len(formulaSamplePhaseOrder))
	raw := make(map[FormulaSamplePhase][]byte, len(events))
	for index, phase := range formulaSamplePhaseOrder {
		events[index] = FormulaSampleEvent{Phase: phase, Panel: panels[index]}
		raw[phase] = make([]byte, formulaStatusObjectScanSize)
	}
	for _, phase := range []FormulaSamplePhase{FormulaPhaseB1, FormulaPhaseB2} {
		binary.LittleEndian.PutUint32(raw[phase][0x1234:], 250)
	}
	for _, phase := range []FormulaSamplePhase{FormulaPhaseA1, FormulaPhaseA2} {
		binary.LittleEndian.PutUint32(raw[phase][0x1234:], 200)
	}
	return events, raw
}

func TestFormulaSampleBundleRejectsIncompleteExperiment(t *testing.T) {
	events, raw := completeFormulaExperimentFixture(t)
	if _, err := buildFormulaSampleBundle("sigil", events[:3], raw, func(uint64) (bool, error) { return false, nil }, nil); err == nil {
		t.Fatal("incomplete A/B/A experiment was exported")
	}
}

func TestFormulaSampleBundleRejectsUnknownExperimentTypeAndExcessCandidates(t *testing.T) {
	events, raw := completeFormulaExperimentFixture(t)
	if _, err := buildFormulaSampleBundle("", events, raw, func(uint64) (bool, error) { return false, nil }, nil); err == nil {
		t.Fatal("bundle accepted an unspecified experiment type")
	}
	for offset := 0; offset+4 <= formulaStatusObjectScanSize; offset += 4 {
		binary.LittleEndian.PutUint32(raw[FormulaPhaseA1][offset:], 100)
		binary.LittleEndian.PutUint32(raw[FormulaPhaseA2][offset:], 100)
		binary.LittleEndian.PutUint32(raw[FormulaPhaseB1][offset:], 125)
		binary.LittleEndian.PutUint32(raw[FormulaPhaseB2][offset:], 125)
	}
	if _, err := buildFormulaSampleBundle("sigil", events, raw, func(uint64) (bool, error) { return false, nil }, nil); err == nil || !strings.Contains(err.Error(), "上限") {
		t.Fatalf("bundle accepted an excessive candidate set: %v", err)
	}
}

func TestFormulaSampleBundleContainsOnlyRedactedEvidenceAndCandidates(t *testing.T) {
	events, raw := completeFormulaExperimentFixture(t)
	bundle, err := buildFormulaSampleBundle("sigil", events, raw, func(uint64) (bool, error) { return false, nil }, nil)
	if err != nil {
		t.Fatal(err)
	}

	reader, err := zip.NewReader(bytes.NewReader(bundle), int64(len(bundle)))
	if err != nil {
		t.Fatal(err)
	}
	wantEntries := map[string]bool{
		"manifest.json": false, "events.json": false, "observations.json": false,
		"candidates.json": false, "formula-model.json": false, "redaction-report.json": false,
		"runtime-layout.json": false, "README.txt": false, "SHA256SUMS": false,
	}
	contents := make(map[string][]byte, len(reader.File))
	for _, file := range reader.File {
		if _, ok := wantEntries[file.Name]; !ok {
			t.Fatalf("unexpected bundle entry %q", file.Name)
		}
		stream, openErr := file.Open()
		if openErr != nil {
			t.Fatal(openErr)
		}
		payload, readErr := io.ReadAll(stream)
		_ = stream.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		wantEntries[file.Name] = true
		contents[file.Name] = payload
	}
	for name, seen := range wantEntries {
		if !seen {
			t.Fatalf("bundle is missing %s", name)
		}
	}

	var manifest FormulaSampleManifest
	if err := json.Unmarshal(contents["manifest.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != "gbfr-formula-sample/v3" || !manifest.StrictReadOnly || manifest.PhaseCount != 4 {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	var layout RuntimeCharacterPanelLayoutDescriptor
	if err := json.Unmarshal(contents["runtime-layout.json"], &layout); err != nil {
		t.Fatal(err)
	}
	if layout.GameExecutableSHA256 != runtimeCharacterPanelGameEXESHA256 || len(layout.Fields) != 17 || layout.Fields[2].DisplayScale != 10 {
		t.Fatalf("incomplete runtime layout evidence: %+v", layout)
	}
	var candidates []FormulaScalarCandidate
	if err := json.Unmarshal(contents["candidates.json"], &candidates); err != nil {
		t.Fatal(err)
	}
	var eventsPayload []FormulaRedactedEvent
	if err := json.Unmarshal(contents["events.json"], &eventsPayload); err != nil {
		t.Fatal(err)
	}
	if len(eventsPayload) != 4 || len(candidates) == 0 {
		t.Fatalf("incomplete evidence: events=%d candidates=%d", len(eventsPayload), len(candidates))
	}
	found := false
	for _, candidate := range candidates {
		if candidate.Offset == 0x1234 && candidate.Kind == FormulaScalarI32 && candidate.Delta == 50 {
			found = true
		}
	}
	if !found {
		t.Fatal("reversible raw-field candidate was not exported")
	}
	if strings.Contains(string(contents["candidates.json"]), "aBits") || strings.Contains(string(contents["candidates.json"]), "bBits") {
		t.Fatal("candidate export retained absolute A/B bit patterns")
	}

	joined := string(contents["manifest.json"]) + string(contents["events.json"]) + string(contents["observations.json"]) + string(contents["candidates.json"]) + string(contents["runtime-layout.json"])
	for _, forbidden := range []string{"processId", "moduleBase", "address", "absoluteTime", "sourcePath", "customName", "notes", "24576"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("redacted evidence leaked %q", forbidden)
		}
	}
	for name, payload := range contents {
		if name == "SHA256SUMS" {
			continue
		}
		digest := sha256.Sum256(payload)
		want := fmt.Sprintf("%x  %s", digest, name)
		if !strings.Contains(string(contents["SHA256SUMS"]), want) {
			t.Fatalf("SHA256SUMS is missing %s", name)
		}
	}
}

func TestFormulaSampleBundleAcceptsDefenseExperimentFromRawReversibleCandidate(t *testing.T) {
	events, raw := completeFormulaExperimentFixture(t)
	for index := range events {
		events[index].Panel = events[0].Panel
	}
	bundle, err := buildFormulaSampleBundle("defense", events, raw, func(uint64) (bool, error) { return false, nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle) == 0 {
		t.Fatal("defense candidate scan produced an empty evidence bundle")
	}
	reader, err := zip.NewReader(bytes.NewReader(bundle), int64(len(bundle)))
	if err != nil {
		t.Fatal(err)
	}
	var model FormulaEvidenceModel
	for _, file := range reader.File {
		if file.Name != "formula-model.json" {
			continue
		}
		stream, openErr := file.Open()
		if openErr != nil {
			t.Fatal(openErr)
		}
		payload, readErr := io.ReadAll(stream)
		_ = stream.Close()
		if readErr != nil || json.Unmarshal(payload, &model) != nil {
			t.Fatalf("read formula model: %v", readErr)
		}
	}
	if model.Kind != "reversible-status-candidate-scan" || !model.CandidateABABVerified {
		t.Fatalf("scan-only model overclaimed panel evidence: %+v", model)
	}
}

func TestFormulaSampleBundleKeepsHonestNoChangeMasteryObservation(t *testing.T) {
	events, _ := completeFormulaExperimentFixture(t)
	raw := make(map[FormulaSamplePhase][]byte, len(events))
	for index := range events {
		events[index].Panel = events[0].Panel
		raw[events[index].Phase] = make([]byte, formulaStatusObjectScanSize)
	}
	bundle, err := buildFormulaSampleBundle("mastery", events, raw, func(uint64) (bool, error) { return false, nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle) == 0 {
		t.Fatal("negative mastery observation was discarded")
	}
}

func TestFormulaSampleBundleRejectsNoChangeForOrdinaryPanelExperiment(t *testing.T) {
	events, _ := completeFormulaExperimentFixture(t)
	raw := make(map[FormulaSamplePhase][]byte, len(events))
	for index := range events {
		events[index].Panel = events[0].Panel
		raw[events[index].Phase] = make([]byte, formulaStatusObjectScanSize)
	}
	if _, err := buildFormulaSampleBundle("sigil", events, raw, func(uint64) (bool, error) { return false, nil }, nil); err == nil {
		t.Fatal("ordinary panel experiment accepted no observed change")
	}
}
