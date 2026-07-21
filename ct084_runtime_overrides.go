package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	ct084RuntimeOverrideSchemaVersion = 1
	ct084RuntimeOverrideGameVersion   = "2.0.2"
)

//go:embed data/ct084_runtime_overrides_2.0.2.json
var ct084RuntimeOverridesJSON []byte

type ct084RuntimeOverrideManifest struct {
	SchemaVersion        int                    `json:"schemaVersion"`
	GameVersion          string                 `json:"gameVersion"`
	GameExecutableSHA256 string                 `json:"gameExecutableSha256"`
	Overrides            []ct084RuntimeOverride `json:"overrides"`
}

type ct084RuntimeOverride struct {
	FeatureID     string           `json:"featureId"`
	EvidenceLevel string           `json:"evidenceLevel"`
	EvidenceNote  string           `json:"evidenceNote"`
	AppendSites   []CT084PatchSite `json:"appendSites"`
}

func decodeCT084RuntimeOverrides(raw []byte) (*ct084RuntimeOverrideManifest, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	var manifest ct084RuntimeOverrideManifest
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("decode CT084 runtime overrides: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("decode CT084 runtime overrides: unexpected trailing JSON value")
		}
		return nil, fmt.Errorf("decode CT084 runtime overrides trailing data: %w", err)
	}
	if manifest.SchemaVersion != ct084RuntimeOverrideSchemaVersion {
		return nil, fmt.Errorf("CT084 runtime override schemaVersion=%d, want %d", manifest.SchemaVersion, ct084RuntimeOverrideSchemaVersion)
	}
	if manifest.GameVersion != ct084RuntimeOverrideGameVersion {
		return nil, fmt.Errorf("CT084 runtime override gameVersion=%q, want %q", manifest.GameVersion, ct084RuntimeOverrideGameVersion)
	}
	if !strings.EqualFold(manifest.GameExecutableSHA256, runtimeCharacterPanelGameEXESHA256) {
		return nil, fmt.Errorf("CT084 runtime override executable identity mismatch")
	}
	if len(manifest.Overrides) == 0 {
		return nil, fmt.Errorf("CT084 runtime overrides are empty")
	}
	return &manifest, nil
}

func applyCT084RuntimeOverrides(catalog *CT084Catalog, raw []byte) error {
	if catalog == nil {
		return fmt.Errorf("CT084 runtime override target is nil")
	}
	manifest, err := decodeCT084RuntimeOverrides(raw)
	if err != nil {
		return err
	}

	byID := make(map[string]*CT084Feature, len(catalog.Features))
	for index := range catalog.Features {
		byID[catalog.Features[index].ID] = &catalog.Features[index]
	}
	seen := make(map[string]struct{}, len(manifest.Overrides))
	for index, override := range manifest.Overrides {
		if _, duplicate := seen[override.FeatureID]; duplicate {
			return fmt.Errorf("CT084 runtime override[%d] duplicates feature %q", index, override.FeatureID)
		}
		seen[override.FeatureID] = struct{}{}
		feature, exists := byID[override.FeatureID]
		if !exists {
			return fmt.Errorf("CT084 runtime override[%d] references unknown feature %q", index, override.FeatureID)
		}
		if strings.TrimSpace(override.EvidenceLevel) == "" || strings.TrimSpace(override.EvidenceNote) == "" {
			return fmt.Errorf("CT084 runtime override[%d] has incomplete evidence metadata", index)
		}
		if len(override.AppendSites) == 0 {
			return fmt.Errorf("CT084 runtime override[%d] has no appended sites", index)
		}
		feature.EvidenceLevel = override.EvidenceLevel
		feature.EvidenceNote = override.EvidenceNote
		feature.Sites = append(feature.Sites, override.AppendSites...)
	}
	return validateCT084Catalog(catalog)
}
