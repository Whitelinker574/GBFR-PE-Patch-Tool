package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type fieldRuntimeCalibrationEvidence struct {
	SchemaVersion        int    `json:"schemaVersion"`
	GameExecutableSHA256 string `json:"gameExecutableSha256"`
	LayoutID             string `json:"layoutId"`
	RoleSamples          []struct {
		Role          string `json:"role"`
		DirectoryHash string `json:"directoryHash"`
		StableReads   int    `json:"stableReads"`
	} `json:"roleSamples"`
	Experiments []struct {
		NodeHash       string `json:"nodeHash"`
		Classification string `json:"classification"`
		Repeated       bool   `json:"repeated"`
		DisplayDelta   struct {
			HP        int     `json:"hp"`
			Attack    int     `json:"attack"`
			StunPower float64 `json:"stunPower"`
			CritRate  float64 `json:"critRate"`
		} `json:"displayDelta"`
	} `json:"experiments"`
}

func TestFieldRuntimeCalibrationEvidenceIsVersionedRedactedAndReproducible(t *testing.T) {
	raw, err := os.ReadFile("data/runtime_field_calibration_2.0.2.json")
	if err != nil {
		t.Fatal(err)
	}
	var evidence fieldRuntimeCalibrationEvidence
	if err := json.Unmarshal(raw, &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence.SchemaVersion != 1 || evidence.GameExecutableSHA256 != runtimeCharacterPanelGameEXESHA256 || evidence.LayoutID != runtimeCharacterPanelLayoutID {
		t.Fatalf("calibration identity is incomplete: %+v", evidence)
	}
	if len(evidence.RoleSamples) < 2 {
		t.Fatalf("role samples=%d, want at least two", len(evidence.RoleSamples))
	}
	for _, sample := range evidence.RoleSamples {
		if sample.Role == "" || len(sample.DirectoryHash) != 8 || sample.StableReads < 3 {
			t.Fatalf("invalid role sample: %+v", sample)
		}
	}
	byNode := make(map[string]struct {
		classification string
		repeated       bool
		hp             int
		stun           float64
	})
	for _, experiment := range evidence.Experiments {
		byNode[experiment.NodeHash] = struct {
			classification string
			repeated       bool
			hp             int
			stun           float64
		}{experiment.Classification, experiment.Repeated, experiment.DisplayDelta.HP, experiment.DisplayDelta.StunPower}
	}
	if got := byNode["1F52146F"]; got.classification != "verified_final_panel" || !got.repeated || got.stun != 4 {
		t.Fatalf("Io stun calibration is incomplete: %+v", got)
	}
	if got := byNode["A66398F9"]; got.classification != "verified_final_panel" || !got.repeated || got.hp != 18900 {
		t.Fatalf("defense HP calibration is incomplete: %+v", got)
	}
	if got := byNode["E0E6FF0C"]; got.classification != "negative_town_panel_observation" || got.repeated {
		t.Fatalf("conditional defense evidence was overclaimed: %+v", got)
	}
	lower := strings.ToLower(string(raw))
	for _, forbidden := range []string{"\"pid\"", "modulebase", "\"absoluteaddress\"", "c:\\\\users", "d:\\\\gbf", "administrator"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("calibration evidence leaked %q", forbidden)
		}
	}
}
