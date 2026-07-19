package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
)

const (
	ct084CatalogSchemaVersion  = 1
	ct084CatalogSourceVersion  = "0.8.4"
	ct084CatalogSourceSHA256   = "B75DF049E27D1423FC5ECDD47CC85DBAC241BEE582A49CEBA30CF020E150B659"
	ct084CatalogFeatureCount   = 60
	ct084DamageCapConflictName = "damage-cap-display"
)

//go:embed data/ct084_patches.json
var ct084CatalogJSON []byte

// CT084Catalog is the validated catalog extracted from the CT 0.8.4 source.
type CT084Catalog struct {
	SchemaVersion int            `json:"schemaVersion"`
	SourceVersion string         `json:"sourceVersion"`
	SourceSHA256  string         `json:"sourceSha256"`
	Features      []CT084Feature `json:"features"`
}

// CT084Feature describes one user-facing CT feature and its patch sites.
type CT084Feature struct {
	ID            string           `json:"id"`
	CTID          int              `json:"ctId"`
	Name          string           `json:"name"`
	DisplayName   string           `json:"displayName"`
	Mode          string           `json:"mode"`
	Category      string           `json:"category"`
	Group         string           `json:"group"`
	GroupPath     []string         `json:"groupPath"`
	Character     string           `json:"character"`
	Conflicts     []string         `json:"conflicts"`
	ConflictGroup string           `json:"conflictGroup"`
	Sites         []CT084PatchSite `json:"sites"`
}

// CT084PatchSite contains the signature and bytes for one direct patch.
type CT084PatchSite struct {
	Symbol                 string `json:"symbol"`
	Module                 string `json:"module"`
	AOB                    string `json:"aob"`
	Offset                 int    `json:"offset"`
	PatternValues          []byte `json:"patternValues"`
	PatternMasks           []byte `json:"patternMasks"`
	EnableBytes            []byte `json:"enableBytes"`
	DisableBytes           []byte `json:"disableBytes"`
	RequiresRuntimeCapture bool   `json:"requiresRuntimeCapture"`
}

type ct084CatalogPatchRange struct {
	start        int
	end          int
	featureID    string
	featureIndex int
	siteIndex    int
}

var (
	ct084CatalogOnce sync.Once
	ct084CatalogData *CT084Catalog
	ct084CatalogErr  error
)

func decodeCT084Catalog(raw []byte) (*CT084Catalog, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	var catalog CT084Catalog
	if err := decoder.Decode(&catalog); err != nil {
		return nil, fmt.Errorf("decode CT084 catalog: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("decode CT084 catalog: unexpected trailing JSON value")
		}
		return nil, fmt.Errorf("decode CT084 catalog trailing data: %w", err)
	}
	if err := validateCT084Catalog(&catalog); err != nil {
		return nil, err
	}
	return &catalog, nil
}

func loadCT084Catalog() (*CT084Catalog, error) {
	ct084CatalogOnce.Do(func() {
		ct084CatalogData, ct084CatalogErr = decodeCT084Catalog(ct084CatalogJSON)
	})
	if ct084CatalogErr != nil {
		return nil, ct084CatalogErr
	}
	return cloneCT084Catalog(ct084CatalogData), nil
}

// CT084GetCatalog returns an independently mutable copy for the Wails caller.
func (a *App) CT084GetCatalog() ([]CT084Feature, error) {
	catalog, err := loadCT084Catalog()
	if err != nil {
		return nil, err
	}
	return catalog.Features, nil
}

func validateCT084Catalog(catalog *CT084Catalog) error {
	if catalog == nil {
		return fmt.Errorf("CT084 catalog is nil")
	}
	if catalog.SchemaVersion != ct084CatalogSchemaVersion {
		return fmt.Errorf("CT084 catalog schemaVersion=%d, want %d", catalog.SchemaVersion, ct084CatalogSchemaVersion)
	}
	if catalog.SourceVersion != ct084CatalogSourceVersion {
		return fmt.Errorf("CT084 catalog sourceVersion=%q, want %q", catalog.SourceVersion, ct084CatalogSourceVersion)
	}
	if catalog.SourceSHA256 != ct084CatalogSourceSHA256 {
		return fmt.Errorf("CT084 catalog sourceSha256 does not match the 0.8.4 source")
	}
	if len(catalog.Features) != ct084CatalogFeatureCount {
		return fmt.Errorf("CT084 catalog has %d features, want %d", len(catalog.Features), ct084CatalogFeatureCount)
	}

	byID := make(map[string]*CT084Feature, len(catalog.Features))
	seenCTIDs := make(map[int]string, len(catalog.Features))
	patchRanges := make(map[[3]string][]ct084CatalogPatchRange)
	for featureIndex := range catalog.Features {
		feature := &catalog.Features[featureIndex]
		label := fmt.Sprintf("feature[%d] %q", featureIndex, feature.ID)
		wantID := fmt.Sprintf("ct084-%d", feature.CTID)
		if feature.ID != wantID {
			return fmt.Errorf("%s: id must be %q", label, wantID)
		}
		if previous, exists := byID[feature.ID]; exists {
			return fmt.Errorf("%s: duplicate id also used by CT %d", label, previous.CTID)
		}
		if previousID, exists := seenCTIDs[feature.CTID]; exists {
			return fmt.Errorf("%s: duplicate ctId also used by %q", label, previousID)
		}
		byID[feature.ID] = feature
		seenCTIDs[feature.CTID] = feature.ID

		if strings.TrimSpace(feature.Name) == "" {
			return fmt.Errorf("%s: name is empty", label)
		}
		if feature.DisplayName != feature.Name {
			return fmt.Errorf("%s: displayName must match name", label)
		}
		switch feature.Mode {
		case "combat", "characters", "quest":
		default:
			return fmt.Errorf("%s: invalid mode %q", label, feature.Mode)
		}
		if feature.Category != feature.Mode {
			return fmt.Errorf("%s: category must match mode", label)
		}
		if strings.TrimSpace(feature.Group) == "" {
			return fmt.Errorf("%s: group is empty", label)
		}
		if len(feature.Sites) == 0 {
			return fmt.Errorf("%s: patch sites are empty", label)
		}
		if err := validateCT084PatchSites(feature, featureIndex, label, patchRanges); err != nil {
			return err
		}
	}

	for featureIndex := range catalog.Features {
		feature := &catalog.Features[featureIndex]
		label := fmt.Sprintf("feature[%d] %q", featureIndex, feature.ID)
		seen := make(map[string]struct{}, len(feature.Conflicts))
		for _, targetID := range feature.Conflicts {
			if targetID == feature.ID {
				return fmt.Errorf("%s: conflict target cannot reference itself", label)
			}
			if _, duplicate := seen[targetID]; duplicate {
				return fmt.Errorf("%s: duplicate conflict target %q", label, targetID)
			}
			seen[targetID] = struct{}{}
			target, exists := byID[targetID]
			if !exists {
				return fmt.Errorf("%s: conflict target %q does not exist", label, targetID)
			}
			if !containsCT084String(target.Conflicts, feature.ID) {
				return fmt.Errorf("%s: conflict with %q is not symmetric", label, targetID)
			}
		}
	}

	return validateCT084DamageCapConflicts(byID)
}

func validateCT084PatchSites(feature *CT084Feature, featureIndex int, featureLabel string, ranges map[[3]string][]ct084CatalogPatchRange) error {
	for siteIndex := range feature.Sites {
		site := &feature.Sites[siteIndex]
		label := fmt.Sprintf("%s site[%d]", featureLabel, siteIndex)
		if strings.TrimSpace(site.Symbol) == "" {
			return fmt.Errorf("%s: symbol is empty", label)
		}
		if strings.TrimSpace(site.Module) == "" {
			return fmt.Errorf("%s: module is empty", label)
		}
		if strings.TrimSpace(site.AOB) == "" {
			return fmt.Errorf("%s: AOB is empty", label)
		}

		pattern, err := parseCT084Pattern(site.AOB)
		if err != nil {
			return fmt.Errorf("%s: invalid AOB: %w", label, err)
		}
		if err := validateCT084Pattern(pattern); err != nil {
			return fmt.Errorf("%s: invalid AOB: %w", label, err)
		}
		if canonicalCT084AOB(pattern) != site.AOB {
			return fmt.Errorf("%s: AOB is not canonical", label)
		}
		if !bytes.Equal(site.PatternValues, pattern.Values) {
			return fmt.Errorf("%s: patternValues do not match AOB", label)
		}
		if !bytes.Equal(site.PatternMasks, pattern.Mask) {
			return fmt.Errorf("%s: patternMasks do not match AOB", label)
		}
		if err := validateCT084Pattern(ct084Pattern{Values: site.PatternValues, Mask: site.PatternMasks}); err != nil {
			return fmt.Errorf("%s: noncanonical pattern arrays: %w", label, err)
		}
		if site.Offset < 0 {
			return fmt.Errorf("%s: offset=%d is negative", label, site.Offset)
		}
		if len(site.EnableBytes) == 0 {
			return fmt.Errorf("%s: enableBytes are empty", label)
		}
		if site.Offset > len(pattern.Values) || len(site.EnableBytes) > len(pattern.Values)-site.Offset {
			return fmt.Errorf("%s: patch range [%d,%d) exceeds AOB length %d", label, site.Offset, site.Offset+len(site.EnableBytes), len(pattern.Values))
		}
		if site.RequiresRuntimeCapture {
			if len(site.DisableBytes) != 0 {
				return fmt.Errorf("%s: runtime-capture site must not contain disableBytes", label)
			}
		} else if len(site.DisableBytes) != len(site.EnableBytes) {
			return fmt.Errorf("%s: disableBytes length=%d, want %d", label, len(site.DisableBytes), len(site.EnableBytes))
		}

		key := [3]string{site.Module, site.Symbol, site.AOB}
		current := ct084CatalogPatchRange{
			start:        site.Offset,
			end:          site.Offset + len(site.EnableBytes),
			featureID:    feature.ID,
			featureIndex: featureIndex,
			siteIndex:    siteIndex,
		}
		for _, previous := range ranges[key] {
			if current.start < previous.end && previous.start < current.end {
				return fmt.Errorf(
					"%s: patch range overlaps feature[%d] %q site[%d] for the same module/symbol/AOB",
					label, previous.featureIndex, previous.featureID, previous.siteIndex,
				)
			}
		}
		ranges[key] = append(ranges[key], current)
	}
	return nil
}

func validateCT084DamageCapConflicts(byID map[string]*CT084Feature) error {
	ids := []string{"ct084-31967", "ct084-31979", "ct084-31995"}
	for _, id := range ids {
		feature, exists := byID[id]
		if !exists {
			return fmt.Errorf("required damage-cap feature %q is missing", id)
		}
		if feature.ConflictGroup != ct084DamageCapConflictName {
			return fmt.Errorf("feature %q: conflictGroup=%q, want %q", id, feature.ConflictGroup, ct084DamageCapConflictName)
		}
		for _, otherID := range ids {
			if id != otherID && !containsCT084String(feature.Conflicts, otherID) {
				return fmt.Errorf("feature %q: missing required conflict with %q", id, otherID)
			}
		}
	}
	return nil
}

func canonicalCT084AOB(pattern ct084Pattern) string {
	tokens := make([]string, len(pattern.Values))
	const hex = "0123456789ABCDEF"
	for index, value := range pattern.Values {
		switch pattern.Mask[index] {
		case 0xff:
			tokens[index] = string([]byte{hex[value>>4], hex[value&0x0f]})
		case 0xf0:
			tokens[index] = string([]byte{hex[value>>4], '?'})
		case 0x0f:
			tokens[index] = string([]byte{'?', hex[value&0x0f]})
		case 0x00:
			tokens[index] = "??"
		}
	}
	return strings.Join(tokens, " ")
}

func cloneCT084Catalog(source *CT084Catalog) *CT084Catalog {
	if source == nil {
		return nil
	}
	cloned := *source
	cloned.Features = make([]CT084Feature, len(source.Features))
	for featureIndex := range source.Features {
		cloned.Features[featureIndex] = source.Features[featureIndex]
		feature := &cloned.Features[featureIndex]
		feature.GroupPath = make([]string, len(source.Features[featureIndex].GroupPath))
		copy(feature.GroupPath, source.Features[featureIndex].GroupPath)
		feature.Conflicts = make([]string, len(source.Features[featureIndex].Conflicts))
		copy(feature.Conflicts, source.Features[featureIndex].Conflicts)
		feature.Sites = make([]CT084PatchSite, len(source.Features[featureIndex].Sites))
		for siteIndex := range source.Features[featureIndex].Sites {
			feature.Sites[siteIndex] = source.Features[featureIndex].Sites[siteIndex]
			site := &feature.Sites[siteIndex]
			sourceSite := &source.Features[featureIndex].Sites[siteIndex]
			site.PatternValues = make([]byte, len(sourceSite.PatternValues))
			copy(site.PatternValues, sourceSite.PatternValues)
			site.PatternMasks = make([]byte, len(sourceSite.PatternMasks))
			copy(site.PatternMasks, sourceSite.PatternMasks)
			site.EnableBytes = make([]byte, len(sourceSite.EnableBytes))
			copy(site.EnableBytes, sourceSite.EnableBytes)
			site.DisableBytes = make([]byte, len(sourceSite.DisableBytes))
			copy(site.DisableBytes, sourceSite.DisableBytes)
		}
	}
	return &cloned
}

func containsCT084String(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
