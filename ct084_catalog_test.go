package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const ct084SourceSHA256 = "B75DF049E27D1423FC5ECDD47CC85DBAC241BEE582A49CEBA30CF020E150B659"

type ct084CatalogFile struct {
	SchemaVersion int                   `json:"schemaVersion"`
	SourceVersion string                `json:"sourceVersion"`
	SourceSHA256  string                `json:"sourceSha256"`
	Features      []ct084CatalogFeature `json:"features"`
}

type ct084CatalogFeature struct {
	ID            string             `json:"id"`
	CTID          int                `json:"ctId"`
	Name          string             `json:"name"`
	DisplayName   string             `json:"displayName"`
	Mode          string             `json:"mode"`
	Category      string             `json:"category"`
	Group         string             `json:"group"`
	GroupPath     []string           `json:"groupPath"`
	Character     string             `json:"character"`
	Conflicts     []string           `json:"conflicts"`
	ConflictGroup string             `json:"conflictGroup"`
	Sites         []ct084CatalogSite `json:"sites"`
}

type ct084CatalogSite struct {
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

func readCT084CatalogFile(t *testing.T) ct084CatalogFile {
	t.Helper()
	raw, err := os.ReadFile("data/ct084_patches.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog ct084CatalogFile
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	return catalog
}

func TestCT084CatalogMetadataAndStableFeatureIdentity(t *testing.T) {
	catalog := readCT084CatalogFile(t)
	if catalog.SchemaVersion != 1 {
		t.Fatalf("schemaVersion=%d, want 1", catalog.SchemaVersion)
	}
	if catalog.SourceVersion != "0.8.4" {
		t.Fatalf("sourceVersion=%q, want 0.8.4", catalog.SourceVersion)
	}
	if catalog.SourceSHA256 != ct084SourceSHA256 {
		t.Fatalf("sourceSha256=%q, want %s", catalog.SourceSHA256, ct084SourceSHA256)
	}
	// The source contains 64 direct AOB byte patches. Four product-level
	// exclusions leave 60; CT 32556 must not be lost merely because its
	// AssemblerScript element has an Async attribute.
	if len(catalog.Features) != 60 {
		t.Fatalf("features=%d, want 60", len(catalog.Features))
	}

	seenIDs := make(map[string]bool, len(catalog.Features))
	byCTID := make(map[int]ct084CatalogFeature, len(catalog.Features))
	seenCategories := make(map[string]bool, 3)
	validModes := map[string]bool{"combat": true, "characters": true, "quest": true}
	orderKeys := make([]string, 0, len(catalog.Features))
	for _, feature := range catalog.Features {
		wantID := fmt.Sprintf("ct084-%d", feature.CTID)
		if feature.ID != wantID {
			t.Errorf("CT %d id=%q, want stable id %q", feature.CTID, feature.ID, wantID)
		}
		if seenIDs[feature.ID] {
			t.Errorf("duplicate feature id %q", feature.ID)
		}
		seenIDs[feature.ID] = true
		byCTID[feature.CTID] = feature
		if strings.TrimSpace(feature.Name) == "" {
			t.Errorf("feature %q has an empty name", feature.ID)
		}
		if feature.DisplayName != feature.Name {
			t.Errorf("feature %q displayName=%q, want alias of name %q", feature.ID, feature.DisplayName, feature.Name)
		}
		if strings.TrimSpace(feature.Mode) == "" {
			t.Errorf("feature %q has an empty mode", feature.ID)
		}
		if !validModes[feature.Mode] {
			t.Errorf("feature %q mode=%q, want combat, characters, or quest", feature.ID, feature.Mode)
		}
		if feature.Category != feature.Mode {
			t.Errorf("feature %q category=%q, want alias of mode %q", feature.ID, feature.Category, feature.Mode)
		}
		if strings.TrimSpace(feature.Group) == "" {
			t.Errorf("feature %q has an empty group", feature.ID)
		}
		seenCategories[feature.Mode] = true
		if len(feature.Sites) == 0 {
			t.Errorf("feature %q has no patch sites", feature.ID)
		}
		orderKeys = append(orderKeys, fmt.Sprintf("%s/%08d", feature.Mode, feature.CTID))
	}
	for _, category := range []string{"combat", "characters", "quest"} {
		if !seenCategories[category] {
			t.Errorf("catalog has no %q category", category)
		}
	}
	sortedKeys := append([]string(nil), orderKeys...)
	sort.Strings(sortedKeys)
	if !reflect.DeepEqual(orderKeys, sortedKeys) {
		t.Errorf("features are not deterministically sorted by category and CT ID")
	}

	for _, excluded := range []int{31935, 33086, 31060, 31456} {
		if _, ok := byCTID[excluded]; ok {
			t.Errorf("excluded CT %d is present", excluded)
		}
	}
	if _, ok := byCTID[32556]; !ok {
		t.Error("CT 32556 is missing (AssemblerScript has an Async attribute)")
	}
}

func TestCT084CatalogPatchSitesAreComplete(t *testing.T) {
	catalog := readCT084CatalogFile(t)
	validMask := map[byte]bool{0x00: true, 0x0f: true, 0xf0: true, 0xff: true}
	for _, feature := range catalog.Features {
		for siteIndex, site := range feature.Sites {
			label := fmt.Sprintf("%s site %d", feature.ID, siteIndex)
			if strings.TrimSpace(site.Symbol) == "" || strings.TrimSpace(site.Module) == "" || strings.TrimSpace(site.AOB) == "" {
				t.Errorf("%s has empty symbol/module/aob", label)
			}
			if len(site.PatternValues) == 0 || len(site.PatternValues) != len(site.PatternMasks) {
				t.Errorf("%s pattern lengths values=%d masks=%d", label, len(site.PatternValues), len(site.PatternMasks))
			}
			for _, mask := range site.PatternMasks {
				if !validMask[mask] {
					t.Errorf("%s has invalid nibble mask 0x%02X", label, mask)
				}
			}
			if len(site.EnableBytes) == 0 {
				t.Errorf("%s has no enable bytes", label)
			}
			if site.RequiresRuntimeCapture {
				if len(site.DisableBytes) != 0 {
					t.Errorf("%s requires runtime capture but also invents disable bytes", label)
				}
			} else if len(site.DisableBytes) == 0 {
				t.Errorf("%s has neither disable bytes nor runtime capture", label)
			}
		}
	}
}

func TestCT084CatalogKnownMultiSiteAndConflicts(t *testing.T) {
	catalog := readCT084CatalogFile(t)
	byCTID := make(map[int]ct084CatalogFeature, len(catalog.Features))
	for _, feature := range catalog.Features {
		byCTID[feature.CTID] = feature
	}

	feature := byCTID[31053]
	offsets := make([]int, 0, len(feature.Sites))
	for _, site := range feature.Sites {
		offsets = append(offsets, site.Offset)
	}
	sort.Ints(offsets)
	if !reflect.DeepEqual(offsets, []int{0, 0x16}) {
		t.Fatalf("CT 31053 offsets=%v, want [0 22]", offsets)
	}

	const conflictGroup = "damage-cap-display"
	conflictIDs := []int{31967, 31979, 31995}
	for _, ctID := range conflictIDs {
		if _, ok := byCTID[ctID]; !ok {
			t.Fatalf("required conflict CT %d is missing", ctID)
		}
	}
	for _, ctID := range conflictIDs {
		feature := byCTID[ctID]
		if feature.ConflictGroup != conflictGroup {
			t.Errorf("CT %d conflictGroup=%q, want %q", ctID, feature.ConflictGroup, conflictGroup)
		}
		wantConflicts := make([]string, 0, 2)
		for _, otherID := range conflictIDs {
			if otherID != ctID {
				if _, exists := byCTID[otherID]; exists {
					wantConflicts = append(wantConflicts, fmt.Sprintf("ct084-%d", otherID))
				}
			}
		}
		if !reflect.DeepEqual(feature.Conflicts, wantConflicts) {
			t.Errorf("CT %d conflicts=%v, want %v", ctID, feature.Conflicts, wantConflicts)
		}
	}
}
