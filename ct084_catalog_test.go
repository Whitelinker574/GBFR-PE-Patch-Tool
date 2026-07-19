package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const ct084SourceSHA256 = "B75DF049E27D1423FC5ECDD47CC85DBAC241BEE582A49CEBA30CF020E150B659"
const ct084CatalogSHA256 = "F3B940E644CC6CF0B9BDF2EB11B7B7466D3097053DB8900FEFA97229C9EE82A8"

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

func parseCT084AOB(t *testing.T, pattern string) ([]byte, []byte) {
	t.Helper()
	compact := strings.Join(strings.Fields(pattern), "")
	if compact == "" || len(compact)%2 != 0 {
		t.Fatalf("invalid AOB %q", pattern)
	}
	values := make([]byte, len(compact)/2)
	masks := make([]byte, len(compact)/2)
	for index := 0; index < len(compact); index++ {
		character := compact[index]
		shift := uint(4)
		if index%2 == 1 {
			shift = 0
		}
		var nibble byte
		switch {
		case character >= '0' && character <= '9':
			nibble = character - '0'
		case character >= 'a' && character <= 'f':
			nibble = character - 'a' + 10
		case character >= 'A' && character <= 'F':
			nibble = character - 'A' + 10
		case character == '?' || character == 'x' || character == 'X':
			continue
		default:
			t.Fatalf("invalid AOB nibble %q in %q", character, pattern)
		}
		byteIndex := index / 2
		values[byteIndex] |= nibble << shift
		masks[byteIndex] |= 0x0f << shift
	}
	return values, masks
}

func TestCT084CatalogContentSHA256(t *testing.T) {
	raw, err := os.ReadFile("data/ct084_patches.json")
	if err != nil {
		t.Fatal(err)
	}
	got := fmt.Sprintf("%X", sha256.Sum256(raw))
	if got != ct084CatalogSHA256 {
		t.Fatalf("catalog SHA256=%s, want %s", got, ct084CatalogSHA256)
	}
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
			wantValues, wantMasks := parseCT084AOB(t, site.AOB)
			if !reflect.DeepEqual(site.PatternValues, wantValues) {
				t.Errorf("%s patternValues=%v, want %v parsed from AOB", label, site.PatternValues, wantValues)
			}
			if !reflect.DeepEqual(site.PatternMasks, wantMasks) {
				t.Errorf("%s patternMasks=%v, want %v parsed from AOB", label, site.PatternMasks, wantMasks)
			}
			for _, mask := range site.PatternMasks {
				if !validMask[mask] {
					t.Errorf("%s has invalid nibble mask 0x%02X", label, mask)
				}
			}
			if len(site.EnableBytes) == 0 {
				t.Errorf("%s has no enable bytes", label)
			}
			if site.Offset < 0 {
				t.Errorf("%s offset=%d, want non-negative", label, site.Offset)
			} else if site.Offset+len(site.EnableBytes) > len(site.PatternValues) {
				t.Errorf("%s patch range [%d,%d) exceeds pattern length %d", label, site.Offset, site.Offset+len(site.EnableBytes), len(site.PatternValues))
			}
			if len(site.DisableBytes) > 0 && len(site.DisableBytes) != len(site.EnableBytes) {
				t.Errorf("%s disable bytes=%d, want enable length %d", label, len(site.DisableBytes), len(site.EnableBytes))
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

func TestCT084GeneratorIgnoresCommentedInstructions(t *testing.T) {
	powerShell := ""
	for _, candidate := range []string{"powershell", "pwsh"} {
		if path, err := exec.LookPath(candidate); err == nil {
			powerShell = path
			break
		}
	}
	if powerShell == "" {
		t.Skip("PowerShell is not installed")
	}

	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "comments.ct")
	outputPath := filepath.Join(tempDir, "catalog.json")
	input := `<?xml version="1.0" encoding="utf-8"?>
<CheatTable>
  <CheatEntries>
    <CheatEntry>
      <ID>900001</ID>
      <Description>comment fixture</Description>
      <AssemblerScript><![CDATA[
[ENABLE]
{
aobscanmodule(FAKE_BLOCK,$process,11 22 33 44)
FAKE_BLOCK:
  db FF
}
// aobscanmodule(FAKE_LINE,$process,55 66 77 88)
// FAKE_LINE:
//   nop 2
aobscanmodule(REAL,$process,A? ?B CC DD)
REAL+1:
  db 90 90
[DISABLE]
{
REAL+1:
  db 11 22
}
// REAL+1:
//   db 33 44
REAL+1:
  db AB CC
]]></AssemblerScript>
    </CheatEntry>
  </CheatEntries>
</CheatTable>`
	if err := os.WriteFile(inputPath, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	scriptPath, err := filepath.Abs("tools/generate_ct084_patches.ps1")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(powerShell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", scriptPath, "-InputCT", inputPath, "-Output", outputPath)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generator failed: %v\n%s", err, output)
	}
	raw, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	var catalog ct084CatalogFile
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Features) != 1 {
		t.Fatalf("features=%d, want 1", len(catalog.Features))
	}
	if len(catalog.Features[0].Sites) != 1 {
		t.Fatalf("sites=%d, want 1; commented instructions were parsed", len(catalog.Features[0].Sites))
	}
	site := catalog.Features[0].Sites[0]
	if site.Symbol != "REAL" || site.Offset != 1 || !reflect.DeepEqual(site.EnableBytes, []byte{0x90, 0x90}) || !reflect.DeepEqual(site.DisableBytes, []byte{0xab, 0xcc}) {
		t.Fatalf("site=%+v, want only the real patch", site)
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
