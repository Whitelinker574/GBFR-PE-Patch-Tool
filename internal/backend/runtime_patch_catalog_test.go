package backend

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

const runtimePatchCatalogSHA256 = "FAD8E9D5515C74C6326D98AD0DEE42E39259A1CE76EEAA6AC70467E24142581D"

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
	if len(catalog.Features) != 60 {
		t.Fatalf("features=%d, want 60", len(catalog.Features))
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
	if sites != 83 || len(aobs) != 81 {
		t.Fatalf("coverage=%d sites/%d AOBs, want 83/81", sites, len(aobs))
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

func TestRuntimePatchExpectedOriginalBytesSelectsKnown203RIPDisplacements(t *testing.T) {
	catalog := readRuntimePatchCatalogFile(t)
	seen := 0
	for _, feature := range catalog.Features {
		for _, site := range feature.Sites {
			expected203, compatible := runtimePatch203OriginalBytes[site.Symbol]
			if !compatible {
				continue
			}
			seen++
			got := runtimePatchExpectedOriginalBytes(site, runtimePatchLocalGame203SHA256)
			if !bytes.Equal(got, expected203) || bytes.Equal(got, site.ExpectedOriginalBytes) {
				t.Fatalf("%s 2.0.3 original=% X, want version variant % X", site.Symbol, got, expected203)
			}
			got[0] ^= 0xff
			if bytes.Equal(got, runtimePatch203OriginalBytes[site.Symbol]) {
				t.Fatalf("%s returned shared 2.0.3 original-byte storage", site.Symbol)
			}
			if got202 := runtimePatchExpectedOriginalBytes(site, runtimePatchLocalGame202SHA256); !bytes.Equal(got202, site.ExpectedOriginalBytes) {
				t.Fatalf("%s 2.0.2 original=% X, want catalog % X", site.Symbol, got202, site.ExpectedOriginalBytes)
			}
		}
	}
	if seen != 4 {
		t.Fatalf("2.0.3 RuntimePatch original-byte variants=%d, want 4", seen)
	}
}

func TestRuntimePatchExpectedOriginalBytesSelectsKnown204RIPDisplacements(t *testing.T) {
	catalog := readRuntimePatchCatalogFile(t)
	seen := 0
	for _, feature := range catalog.Features {
		for _, site := range feature.Sites {
			expected204, compatible := runtimePatch204OriginalBytes[site.Symbol]
			if !compatible {
				continue
			}
			seen++
			got := runtimePatchExpectedOriginalBytes(site, runtimePatchLocalGame204SHA256)
			if !bytes.Equal(got, expected204) || bytes.Equal(got, site.ExpectedOriginalBytes) {
				t.Fatalf("%s 2.0.4 original=% X, want version variant % X", site.Symbol, got, expected204)
			}
			got[0] ^= 0xff
			if bytes.Equal(got, runtimePatch204OriginalBytes[site.Symbol]) {
				t.Fatalf("%s returned shared 2.0.4 original-byte storage", site.Symbol)
			}
		}
	}
	if seen != 4 {
		t.Fatalf("2.0.4 RuntimePatch original-byte variants=%d, want 4", seen)
	}
}

func TestRuntimePatchExpectedOriginalBytesSelectsKnown205RIPDisplacements(t *testing.T) {
	catalog := readRuntimePatchCatalogFile(t)
	seen := 0
	for _, feature := range catalog.Features {
		for _, site := range feature.Sites {
			expected205, compatible := runtimePatch205OriginalBytes[site.Symbol]
			if !compatible {
				continue
			}
			seen++
			got := runtimePatchExpectedOriginalBytes(site, runtimePatchLocalGame205SHA256)
			if !bytes.Equal(got, expected205) {
				t.Fatalf("%s 2.0.5 original=% X, want version variant % X", site.Symbol, got, expected205)
			}
			got[0] ^= 0xff
			if bytes.Equal(got, runtimePatch205OriginalBytes[site.Symbol]) {
				t.Fatalf("%s returned shared 2.0.5 original-byte storage", site.Symbol)
			}
		}
	}
	if seen != 4 {
		t.Fatalf("2.0.5 RuntimePatch original-byte variants=%d, want 4", seen)
	}
}

func TestRuntimePatchCatalogContainsInfiniteLinkTime(t *testing.T) {
	catalog := readRuntimePatchCatalogFile(t)
	var feature *RuntimePatchFeature
	for index := range catalog.Features {
		if catalog.Features[index].CatalogID == 60 {
			feature = &catalog.Features[index]
			break
		}
	}
	if feature == nil {
		t.Fatal("catalog entry 60 is missing")
	}
	if feature.ID != "runtime-patch-060" || feature.Name != "Link time 持续不减" ||
		feature.Mode != "combat" || feature.Group != "战斗功能" || len(feature.Sites) != 1 {
		t.Fatalf("catalog entry 60 is malformed: %+v", feature)
	}
	site := feature.Sites[0]
	if site.Symbol != "GBFR_PATCH_060_1" || site.Offset != 0 ||
		!bytes.Equal(site.ExpectedOriginalBytes, []byte{0xC5, 0xFA, 0x59, 0x05, 0xB4, 0x95, 0x30, 0x05}) ||
		!bytes.Equal(site.EnableBytes, []byte{0x0F, 0x57, 0xC0, 0x90, 0x90, 0x90, 0x90, 0x90}) {
		t.Fatalf("catalog entry 60 site is malformed: %+v", site)
	}
}

func TestRuntimePatchSiteForExecutableSelectsKnown203SignatureVariants(t *testing.T) {
	catalog := readRuntimePatchCatalogFile(t)
	bySymbol := make(map[string]RuntimePatchSite)
	for _, feature := range catalog.Features {
		for _, site := range feature.Sites {
			bySymbol[site.Symbol] = site
		}
	}
	for symbol, variant := range runtimePatch203SiteVariants {
		catalogSite, exists := bySymbol[symbol]
		if !exists {
			t.Fatalf("2.0.3 signature variant %s has no catalog site", symbol)
		}
		resolved, err := runtimePatchSiteForExecutable(catalogSite, runtimePatchLocalGame203SHA256)
		if err != nil {
			t.Fatalf("resolve %s: %v", symbol, err)
		}
		pattern, err := parseRuntimePatchPattern(variant.AOB)
		if err != nil {
			t.Fatalf("parse locked variant %s: %v", symbol, err)
		}
		if resolved.AOB != canonicalRuntimePatchAOB(pattern) || resolved.Offset != variant.Offset {
			t.Fatalf("%s resolved AOB/offset=%q/%d, want %q/%d", symbol, resolved.AOB, resolved.Offset, canonicalRuntimePatchAOB(pattern), variant.Offset)
		}
		resolved205, err := runtimePatchSiteForExecutable(catalogSite, runtimePatchLocalGame205SHA256)
		if err != nil {
			t.Fatalf("resolve 2.0.5 %s: %v", symbol, err)
		}
		if resolved205.AOB != canonicalRuntimePatchAOB(pattern) || resolved205.Offset != variant.Offset {
			t.Fatalf("%s did not retain the verified 2.0.3+ signature on 2.0.5", symbol)
		}
		if !bytes.Equal(resolved.PatternValues, pattern.Values) || !bytes.Equal(resolved.PatternMasks, pattern.Mask) {
			t.Fatalf("%s resolved pattern arrays do not match its AOB", symbol)
		}
		resolved202, err := runtimePatchSiteForExecutable(catalogSite, runtimePatchLocalGame202SHA256)
		if err != nil {
			t.Fatalf("resolve 2.0.2 %s: %v", symbol, err)
		}
		if resolved202.AOB != catalogSite.AOB || resolved202.Offset != catalogSite.Offset {
			t.Fatalf("%s changed the locked 2.0.2 definition", symbol)
		}
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

func TestRuntimePatchCatalogContainsGlassCannonStunRemoval(t *testing.T) {
	catalog := readRuntimePatchCatalogFile(t)
	var feature *RuntimePatchFeature
	for index := range catalog.Features {
		if catalog.Features[index].CatalogID == 59 {
			feature = &catalog.Features[index]
			break
		}
	}
	if feature == nil {
		t.Fatal("catalog entry 59 is missing")
	}
	if feature.ID != "runtime-patch-059" || feature.Name != "刀上舞：免除自身眩晕" ||
		feature.Mode != "combat" || feature.Group != "战斗功能" || len(feature.Sites) != 1 {
		t.Fatalf("catalog entry 59 is malformed: %+v", feature)
	}
	site := feature.Sites[0]
	if site.Symbol != "GBFR_PATCH_059_1" || site.Offset != 0 ||
		!bytes.Equal(site.ExpectedOriginalBytes, []byte{0x75}) ||
		!bytes.Equal(site.EnableBytes, []byte{0xEB}) {
		t.Fatalf("catalog entry 59 site is malformed: %+v", site)
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
