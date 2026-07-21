package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const runtimePatchCatalogSHA256 = "99893A11A57EF6324A5489D123088C339DDD12BA1B51DC1D917ABD87C1271DEC"

func readRuntimePatchCatalogFile(t *testing.T) RuntimePatchCatalog {
	t.Helper()
	raw, err := os.ReadFile("data/runtime_patch_catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog RuntimePatchCatalog
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	return catalog
}

func cloneRuntimePatchCatalogForTest(t *testing.T, source RuntimePatchCatalog) RuntimePatchCatalog {
	t.Helper()
	raw, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	var clone RuntimePatchCatalog
	if err := json.Unmarshal(raw, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func TestRuntimePatchCatalogFileIdentityAndCoverage(t *testing.T) {
	raw, err := os.ReadFile("data/runtime_patch_catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%X", sha256.Sum256(raw)); got != runtimePatchCatalogSHA256 {
		t.Fatalf("catalog SHA256=%s, want %s", got, runtimePatchCatalogSHA256)
	}
	catalog, err := decodeRuntimePatchCatalog(raw)
	if err != nil {
		t.Fatal(err)
	}
	if catalog.SchemaVersion != 3 || catalog.GameVersion != "2.0.2" || catalog.GameExecutableSHA256 != runtimePatchCatalogGameSHA256 {
		t.Fatalf("catalog identity=%+v", catalog)
	}
	if len(catalog.Features) != 58 {
		t.Fatalf("features=%d, want 58", len(catalog.Features))
	}
	sites := 0
	aobs := make(map[string]struct{})
	seenIDs := make(map[string]struct{})
	seenNumbers := make(map[int]struct{})
	for _, feature := range catalog.Features {
		wantID := fmt.Sprintf("runtime-patch-%03d", feature.CatalogID)
		if feature.ID != wantID {
			t.Errorf("catalog entry %d id=%q, want %q", feature.CatalogID, feature.ID, wantID)
		}
		if _, duplicate := seenIDs[feature.ID]; duplicate {
			t.Errorf("duplicate feature id %q", feature.ID)
		}
		if _, duplicate := seenNumbers[feature.CatalogID]; duplicate {
			t.Errorf("duplicate catalog number %d", feature.CatalogID)
		}
		seenIDs[feature.ID] = struct{}{}
		seenNumbers[feature.CatalogID] = struct{}{}
		if feature.Mode != "combat" && feature.Mode != "characters" && feature.Mode != "quest" {
			t.Errorf("feature %q mode=%q", feature.ID, feature.Mode)
		}
		for _, site := range feature.Sites {
			sites++
			aobs[site.AOB] = struct{}{}
			if len(site.PatternValues) == 0 || len(site.PatternValues) != len(site.PatternMasks) || len(site.EnableBytes) == 0 || len(site.ExpectedOriginalBytes) != len(site.EnableBytes) {
				t.Errorf("feature %q has malformed site %q", feature.ID, site.Symbol)
			}
		}
	}
	if sites != 81 || len(aobs) != 79 {
		t.Fatalf("coverage=%d sites/%d AOBs, want 81/79", sites, len(aobs))
	}
}

func TestRuntimePatchCatalogUsesStrictJSONAndRejectsWrongIdentity(t *testing.T) {
	valid := fmt.Sprintf(`{"schemaVersion":3,"gameVersion":"2.0.2","gameExecutableSha256":"%s","features":[]}`, runtimePatchCatalogGameSHA256)
	for name, raw := range map[string]string{
		"unknown field":  strings.TrimSuffix(valid, "}") + `,"unknown":true}`,
		"trailing value": valid + ` {}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeRuntimePatchCatalog([]byte(raw)); err == nil {
				t.Fatal("decodeRuntimePatchCatalog() error=nil")
			}
		})
	}

	base := readRuntimePatchCatalogFile(t)
	mutations := []struct {
		name string
		edit func(*RuntimePatchCatalog)
	}{
		{"schema", func(value *RuntimePatchCatalog) { value.SchemaVersion++ }},
		{"game version", func(value *RuntimePatchCatalog) { value.GameVersion = "unknown" }},
		{"executable", func(value *RuntimePatchCatalog) { value.GameExecutableSHA256 = strings.Repeat("0", 64) }},
		{"feature count", func(value *RuntimePatchCatalog) { value.Features = value.Features[:len(value.Features)-1] }},
		{"feature id", func(value *RuntimePatchCatalog) { value.Features[0].ID = "unstable" }},
		{"empty name", func(value *RuntimePatchCatalog) { value.Features[0].Name = " " }},
		{"invalid mode", func(value *RuntimePatchCatalog) { value.Features[0].Mode = "unknown" }},
		{"empty sites", func(value *RuntimePatchCatalog) { value.Features[0].Sites = nil }},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			candidate := cloneRuntimePatchCatalogForTest(t, base)
			mutation.edit(&candidate)
			if err := validateRuntimePatchCatalog(&candidate); err == nil {
				t.Fatal("validateRuntimePatchCatalog() error=nil")
			}
		})
	}
}

func TestRuntimePatchCatalogCloneDoesNotShareMutableSlices(t *testing.T) {
	source := readRuntimePatchCatalogFile(t)
	clone := cloneRuntimePatchCatalog(&source)
	clone.Features[0].GroupPath[0] = "changed"
	clone.Features[0].Sites[0].PatternValues[0] ^= 0xFF
	if source.Features[0].GroupPath[0] == "changed" || bytes.Equal(source.Features[0].Sites[0].PatternValues, clone.Features[0].Sites[0].PatternValues) {
		t.Fatal("clone shares mutable catalog data")
	}
}

func TestRuntimePatchCatalogKnownMultiSiteAndConflicts(t *testing.T) {
	catalog := readRuntimePatchCatalogFile(t)
	byNumber := make(map[int]RuntimePatchFeature, len(catalog.Features))
	for _, feature := range catalog.Features {
		byNumber[feature.CatalogID] = feature
	}
	offsets := make([]int, 0, len(byNumber[40].Sites))
	for _, site := range byNumber[40].Sites {
		offsets = append(offsets, site.Offset)
	}
	sort.Ints(offsets)
	if !reflect.DeepEqual(offsets, []int{0, 0x16}) {
		t.Fatalf("auto-perfect-guard offsets=%v, want [0 22]", offsets)
	}

	conflictIDs := []int{15, 21, 29}
	for _, id := range conflictIDs {
		feature, exists := byNumber[id]
		if !exists || feature.ConflictGroup != runtimePatchDamageCapConflictName {
			t.Fatalf("damage-cap conflict entry %d is missing or malformed", id)
		}
		want := make([]string, 0, 2)
		for _, other := range conflictIDs {
			if other != id {
				want = append(want, fmt.Sprintf("runtime-patch-%03d", other))
			}
		}
		if !reflect.DeepEqual(feature.Conflicts, want) {
			t.Errorf("catalog entry %d conflicts=%v, want %v", id, feature.Conflicts, want)
		}
	}
}

func TestRuntimePatchOverrideIsLockedToGameAndFieldEvidence(t *testing.T) {
	manifest, err := decodeRuntimePatchRuntimeOverrides(runtimePatchRuntimeOverridesJSON)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.GameVersion != "2.0.2" || manifest.GameExecutableSHA256 != runtimePatchCatalogGameSHA256 || len(manifest.Overrides) != 1 {
		t.Fatalf("override identity=%+v", manifest)
	}
	base := readRuntimePatchCatalogFile(t)
	if err := applyRuntimePatchRuntimeOverrides(&base, runtimePatchRuntimeOverridesJSON); err != nil {
		t.Fatal(err)
	}
	var feature *RuntimePatchFeature
	for index := range base.Features {
		if base.Features[index].ID == "runtime-patch-040" {
			feature = &base.Features[index]
			break
		}
	}
	if feature == nil || len(feature.Sites) != 3 || feature.EvidenceLevel != "verified_field_repeat_game_2.0.2" || !feature.Sites[2].RequiresRuntimeCapture {
		t.Fatalf("field override was not applied: %+v", feature)
	}
}
