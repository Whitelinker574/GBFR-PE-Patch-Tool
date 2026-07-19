package main

import (
	"bytes"
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
const ct084CatalogSHA256 = "C6B09186CE1E349DC334B8A1C33659C996B7B09D092A9C05857435E7DCD9FF83"

func readCT084CatalogFile(t *testing.T) CT084Catalog {
	t.Helper()
	raw, err := os.ReadFile("data/ct084_patches.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog CT084Catalog
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	return catalog
}

func cloneCT084CatalogForTest(t *testing.T, source CT084Catalog) CT084Catalog {
	t.Helper()
	raw, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	var cloned CT084Catalog
	if err := json.Unmarshal(raw, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
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

func TestCT084ProductionCatalogEmbedsLockedContent(t *testing.T) {
	raw, err := os.ReadFile("data/ct084_patches.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ct084CatalogJSON, raw) {
		t.Fatal("embedded CT084 catalog differs from data/ct084_patches.json")
	}
	got := fmt.Sprintf("%X", sha256.Sum256(ct084CatalogJSON))
	if got != ct084CatalogSHA256 {
		t.Fatalf("embedded catalog SHA256=%s, want %s", got, ct084CatalogSHA256)
	}
}

func TestCT084ProductionCatalogLoadsAndValidates(t *testing.T) {
	catalog, err := loadCT084Catalog()
	if err != nil {
		t.Fatalf("loadCT084Catalog() error = %v", err)
	}
	if err := validateCT084Catalog(catalog); err != nil {
		t.Fatalf("validateCT084Catalog() error = %v", err)
	}
	if len(catalog.Features) != 58 {
		t.Fatalf("features=%d, want 58", len(catalog.Features))
	}
	siteCount := 0
	distinctAOBs := make(map[string]struct{})
	for _, feature := range catalog.Features {
		siteCount += len(feature.Sites)
		for _, site := range feature.Sites {
			distinctAOBs[site.AOB] = struct{}{}
		}
	}
	if siteCount != 81 {
		t.Fatalf("sites=%d, want 81", siteCount)
	}
	if len(distinctAOBs) != 79 {
		t.Fatalf("distinct AOBs=%d, want 79", len(distinctAOBs))
	}
}

func TestCT084ProductionCatalogStrictJSON(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "unknown field", raw: `{"schemaVersion":2,"sourceVersion":"0.8.4","sourceSha256":"` + ct084SourceSHA256 + `","features":[],"unknown":true}`},
		{name: "trailing value", raw: `{"schemaVersion":2,"sourceVersion":"0.8.4","sourceSha256":"` + ct084SourceSHA256 + `","features":[]} {}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := decodeCT084Catalog([]byte(tt.raw)); err == nil {
				t.Fatal("decodeCT084Catalog() error = nil, want strict decoding error")
			}
		})
	}
}

func TestCT084ProductionCatalogRejectsInvalidMutations(t *testing.T) {
	base := readCT084CatalogFile(t)
	featureWithConflicts := -1
	for featureIndex := range base.Features {
		if len(base.Features[featureIndex].Conflicts) > 0 && featureWithConflicts < 0 {
			featureWithConflicts = featureIndex
		}
	}
	if featureWithConflicts < 0 {
		t.Fatal("catalog fixture lacks conflict examples")
	}

	tests := []struct {
		name   string
		mutate func(*CT084Catalog)
	}{
		{name: "schema version", mutate: func(c *CT084Catalog) { c.SchemaVersion++ }},
		{name: "source version", mutate: func(c *CT084Catalog) { c.SourceVersion = "0.8.5" }},
		{name: "source SHA", mutate: func(c *CT084Catalog) { c.SourceSHA256 = strings.Repeat("0", 64) }},
		{name: "feature count", mutate: func(c *CT084Catalog) { c.Features = c.Features[:len(c.Features)-1] }},
		{name: "stable ID", mutate: func(c *CT084Catalog) { c.Features[0].ID = "unstable" }},
		{name: "duplicate ID and CTID", mutate: func(c *CT084Catalog) {
			c.Features[1].ID = c.Features[0].ID
			c.Features[1].CTID = c.Features[0].CTID
		}},
		{name: "empty name", mutate: func(c *CT084Catalog) { c.Features[0].Name = " " }},
		{name: "empty group", mutate: func(c *CT084Catalog) { c.Features[0].Group = "\t" }},
		{name: "invalid mode", mutate: func(c *CT084Catalog) { c.Features[0].Mode = "menu" }},
		{name: "empty sites", mutate: func(c *CT084Catalog) { c.Features[0].Sites = nil }},
		{name: "empty symbol", mutate: func(c *CT084Catalog) { c.Features[0].Sites[0].Symbol = "" }},
		{name: "empty module", mutate: func(c *CT084Catalog) { c.Features[0].Sites[0].Module = " " }},
		{name: "empty AOB", mutate: func(c *CT084Catalog) { c.Features[0].Sites[0].AOB = "" }},
		{name: "invalid AOB", mutate: func(c *CT084Catalog) { c.Features[0].Sites[0].AOB = "GG" }},
		{name: "noncanonical AOB", mutate: func(c *CT084Catalog) { c.Features[0].Sites[0].AOB = strings.ToLower(c.Features[0].Sites[0].AOB) }},
		{name: "pattern values mismatch", mutate: func(c *CT084Catalog) { c.Features[0].Sites[0].PatternValues[0] ^= 1 }},
		{name: "pattern masks mismatch", mutate: func(c *CT084Catalog) { c.Features[0].Sites[0].PatternMasks[0] = 0 }},
		{name: "noncanonical wildcard bits", mutate: func(c *CT084Catalog) {
			for index, mask := range c.Features[0].Sites[0].PatternMasks {
				if mask != 0xff {
					c.Features[0].Sites[0].PatternValues[index] |= ^mask
					return
				}
			}
		}},
		{name: "negative offset", mutate: func(c *CT084Catalog) { c.Features[0].Sites[0].Offset = -1 }},
		{name: "empty enable", mutate: func(c *CT084Catalog) { c.Features[0].Sites[0].EnableBytes = nil }},
		{name: "empty expected original", mutate: func(c *CT084Catalog) { c.Features[0].Sites[0].ExpectedOriginalBytes = nil }},
		{name: "expected original length mismatch", mutate: func(c *CT084Catalog) {
			c.Features[0].Sites[0].ExpectedOriginalBytes = append(c.Features[0].Sites[0].ExpectedOriginalBytes, 0)
		}},
		{name: "expected original equals enable", mutate: func(c *CT084Catalog) {
			c.Features[0].Sites[0].ExpectedOriginalBytes = append([]byte(nil), c.Features[0].Sites[0].EnableBytes...)
		}},
		{name: "expected original violates AOB", mutate: func(c *CT084Catalog) {
			c.Features[0].Sites[0].ExpectedOriginalBytes[0] ^= 1
		}},
		{name: "patch past pattern", mutate: func(c *CT084Catalog) { c.Features[0].Sites[0].Offset = len(c.Features[0].Sites[0].PatternValues) }},
		{name: "runtime capture with disable", mutate: func(c *CT084Catalog) { c.Features[0].Sites[0].DisableBytes = []byte{0} }},
		{name: "static patch missing disable", mutate: func(c *CT084Catalog) {
			c.Features[0].Sites[0].RequiresRuntimeCapture = false
			c.Features[0].Sites[0].DisableBytes = nil
		}},
		{name: "static disable length mismatch", mutate: func(c *CT084Catalog) {
			c.Features[0].Sites[0].RequiresRuntimeCapture = false
			c.Features[0].Sites[0].DisableBytes = make([]byte, len(c.Features[0].Sites[0].EnableBytes)+1)
		}},
		{name: "static disable differs from expected original", mutate: func(c *CT084Catalog) {
			c.Features[0].Sites[0].RequiresRuntimeCapture = false
			c.Features[0].Sites[0].DisableBytes = append([]byte(nil), c.Features[0].Sites[0].ExpectedOriginalBytes...)
			c.Features[0].Sites[0].DisableBytes[0] ^= 0xff
		}},
		{name: "overlapping patch slices", mutate: func(c *CT084Catalog) {
			site := c.Features[0].Sites[0]
			site.Offset = c.Features[0].Sites[0].Offset
			c.Features[0].Sites = append(c.Features[0].Sites, site)
		}},
		{name: "cross-feature overlapping patch slices", mutate: func(c *CT084Catalog) {
			c.Features[1].Sites[0] = c.Features[0].Sites[0]
		}},
		{name: "missing conflict target", mutate: func(c *CT084Catalog) { c.Features[featureWithConflicts].Conflicts[0] = "ct084-999999" }},
		{name: "self conflict", mutate: func(c *CT084Catalog) {
			c.Features[featureWithConflicts].Conflicts[0] = c.Features[featureWithConflicts].ID
		}},
		{name: "duplicate conflict", mutate: func(c *CT084Catalog) {
			c.Features[featureWithConflicts].Conflicts[1] = c.Features[featureWithConflicts].Conflicts[0]
		}},
		{name: "asymmetric conflict", mutate: func(c *CT084Catalog) {
			c.Features[featureWithConflicts].Conflicts = c.Features[featureWithConflicts].Conflicts[1:]
		}},
		{name: "required conflict group", mutate: func(c *CT084Catalog) {
			for index := range c.Features {
				if c.Features[index].CTID == 31979 {
					c.Features[index].ConflictGroup = "different"
				}
			}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			catalog := cloneCT084CatalogForTest(t, base)
			tt.mutate(&catalog)
			err := validateCT084Catalog(&catalog)
			if err == nil {
				t.Fatal("validateCT084Catalog() error = nil, want validation error")
			}
			if len(err.Error()) > 300 {
				t.Fatalf("validation error leaked excessive catalog content (%d bytes): %v", len(err.Error()), err)
			}
		})
	}
}

func TestCT084ProductionCatalogReturnsDeepDefensiveCopies(t *testing.T) {
	app := NewApp()
	first, err := app.CT084GetCatalog()
	if err != nil {
		t.Fatal(err)
	}
	second, err := app.CT084GetCatalog()
	if err != nil {
		t.Fatal(err)
	}
	want := cloneCT084CatalogForTest(t, CT084Catalog{Features: second}).Features

	first[0].ID = "poisoned"
	first[0].GroupPath[0] = "poisoned"
	first[0].Sites[0].PatternValues[0] ^= 0xff
	first[0].Sites[0].PatternMasks[0] ^= 0xff
	first[0].Sites[0].EnableBytes[0] ^= 0xff
	first[0].Sites[0].ExpectedOriginalBytes[0] ^= 0xff
	if len(first[0].Sites[0].DisableBytes) > 0 {
		first[0].Sites[0].DisableBytes[0] ^= 0xff
	}
	for index := range first {
		if len(first[index].Conflicts) > 0 {
			first[index].Conflicts[0] = "poisoned"
			break
		}
	}
	second[0].Name = "also poisoned"

	third, err := app.CT084GetCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(third, want) {
		t.Fatal("CT084GetCatalog() returned data mutated through an earlier result")
	}

	loaded, err := loadCT084Catalog()
	if err != nil {
		t.Fatal(err)
	}
	loaded.Features[0].Name = "load poison"
	reloaded, err := loadCT084Catalog()
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Features[0].Name == "load poison" {
		t.Fatal("loadCT084Catalog() exposed its process-global cache")
	}
}

func TestCT084ProductionCatalogWailsJSONPreservesEmptyArrays(t *testing.T) {
	features, err := NewApp().CT084GetCatalog()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(features)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{
		"groupPath",
		"conflicts",
		"sites",
		"patternValues",
		"patternMasks",
		"enableBytes",
		"expectedOriginalBytes",
		"disableBytes",
	} {
		if bytes.Contains(raw, []byte(`"`+field+`":null`)) {
			t.Errorf("CT084GetCatalog() JSON contains null array for %s", field)
		}
	}
}

func TestCT084CatalogMetadataAndStableFeatureIdentity(t *testing.T) {
	catalog := readCT084CatalogFile(t)
	if catalog.SchemaVersion != 2 {
		t.Fatalf("schemaVersion=%d, want 2", catalog.SchemaVersion)
	}
	if catalog.SourceVersion != "0.8.4" {
		t.Fatalf("sourceVersion=%q, want 0.8.4", catalog.SourceVersion)
	}
	if catalog.SourceSHA256 != ct084SourceSHA256 {
		t.Fatalf("sourceSha256=%q, want %s", catalog.SourceSHA256, ct084SourceSHA256)
	}
	// The source contains 64 direct AOB byte patches. Four product-level
	// exclusions leave 60 initial candidates. The exact game 2.0.2 executable
	// proves two more features non-unique, so production ships 58. CT 32556
	// must not be lost merely because its AssemblerScript has an Async attribute.
	if len(catalog.Features) != 58 {
		t.Fatalf("features=%d, want 58", len(catalog.Features))
	}

	seenIDs := make(map[string]bool, len(catalog.Features))
	byCTID := make(map[int]CT084Feature, len(catalog.Features))
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

	for _, excluded := range []int{31935, 33086, 31060, 31456, 31066, 31960} {
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
	siteCount := 0
	wildcardPatchSlices := 0
	fullyWildcardPatchSlices := make([]string, 0, 2)
	for _, feature := range catalog.Features {
		for siteIndex, site := range feature.Sites {
			siteCount++
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
			if len(site.ExpectedOriginalBytes) != len(site.EnableBytes) {
				t.Errorf("%s expected original bytes=%d, want enable length %d", label, len(site.ExpectedOriginalBytes), len(site.EnableBytes))
			}
			if bytes.Equal(site.ExpectedOriginalBytes, site.EnableBytes) {
				t.Errorf("%s expected original bytes already equal the enable patch", label)
			}
			if site.Offset < 0 {
				t.Errorf("%s offset=%d, want non-negative", label, site.Offset)
			} else if site.Offset+len(site.EnableBytes) > len(site.PatternValues) {
				t.Errorf("%s patch range [%d,%d) exceeds pattern length %d", label, site.Offset, site.Offset+len(site.EnableBytes), len(site.PatternValues))
			} else {
				hasWildcard := false
				allWildcard := true
				for _, mask := range site.PatternMasks[site.Offset : site.Offset+len(site.EnableBytes)] {
					hasWildcard = hasWildcard || mask != 0xff
					allWildcard = allWildcard && mask == 0
				}
				if hasWildcard {
					wildcardPatchSlices++
				}
				if allWildcard {
					fullyWildcardPatchSlices = append(fullyWildcardPatchSlices, feature.ID+"/"+site.Symbol)
				}
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
	if len(catalog.Features) != 58 || siteCount != 81 {
		t.Fatalf("catalog coverage=%d features/%d sites, want 58/81", len(catalog.Features), siteCount)
	}
	if wildcardPatchSlices != 75 {
		t.Fatalf("wildcard-bearing patch slices=%d, want 75 (all must be guarded by expectedOriginalBytes)", wildcardPatchSlices)
	}
	wantFullyWildcard := []string{"ct084-31985/NBGFR054", "ct084-31064/NBGFR016B"}
	if !reflect.DeepEqual(fullyWildcardPatchSlices, wantFullyWildcard) {
		t.Fatalf("fully wildcard patch slices=%v, want %v", fullyWildcardPatchSlices, wantFullyWildcard)
	}
}

func TestCT084GeneratorRejectsRuntimeCaptureWithoutLockedOriginalEvidence(t *testing.T) {
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

	input := `<?xml version="1.0" encoding="utf-8"?>
<CheatTable><CheatEntries><CheatEntry>
<ID>900008</ID><Description>unproven wildcard original</Description>
<AssemblerScript><![CDATA[
[ENABLE]
aobscanmodule(UNPROVEN,$process,48 8B ?? 89 54 24 10 90)
alloc(UNPROVEN_bak,1)
UNPROVEN_bak:
  readMem(UNPROVEN+2,1)
UNPROVEN+2:
  db 90
[DISABLE]
UNPROVEN+2:
  readMem(UNPROVEN_bak,1)
]]></AssemblerScript>
</CheatEntry></CheatEntries></CheatTable>`
	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "unproven.ct")
	outputPath := filepath.Join(tempDir, "catalog.json")
	if err := os.WriteFile(inputPath, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	scriptPath, err := filepath.Abs("tools/generate_ct084_patches.ps1")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(powerShell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", scriptPath, "-InputCT", inputPath, "-Output", outputPath)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("generator accepted a runtime-captured wildcard original without locked evidence; output=%s", output)
	}
	if !strings.Contains(strings.ToLower(string(output)), "original") {
		t.Fatalf("generator rejection did not explain missing original-byte evidence: %s", output)
	}
}

func TestCT084GeneratorUsesLockedOriginalEvidenceWithoutReclassifyingCTCapture(t *testing.T) {
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

	input := `<?xml version="1.0" encoding="utf-8"?>
<CheatTable><CheatEntries><CheatEntry>
<ID>900009</ID><Description>proven wildcard original</Description>
<AssemblerScript><![CDATA[
[ENABLE]
aobscanmodule(PROVEN,$process,48 8B ?? 89 54 24 10 90)
alloc(PROVEN_bak,1)
PROVEN_bak:
  readMem(PROVEN+2,1)
PROVEN+2:
  db 90
[DISABLE]
PROVEN+2:
  readMem(PROVEN_bak,1)
]]></AssemblerScript>
</CheatEntry></CheatEntries></CheatTable>`
	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "proven.ct")
	outputPath := filepath.Join(tempDir, "catalog.json")
	evidencePath := filepath.Join(tempDir, "originals.json")
	if err := os.WriteFile(inputPath, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	sourceHash := fmt.Sprintf("%X", sha256.Sum256([]byte(input)))
	evidence := fmt.Sprintf(`{
  "schemaVersion": 1,
  "sourceVersion": "0.8.4",
  "sourceSha256": %q,
  "executableSha256": %q,
  "executableSize": %d,
  "sites": [{
    "featureId": "ct084-900009",
    "ctId": 900009,
    "siteIndex": 0,
    "symbol": "PROVEN",
    "aob": "48 8B ?? 89 54 24 10 90",
    "offset": 2,
    "patchRva": 4098,
    "expectedOriginalBytes": [204]
  }]
}`, sourceHash, ct084LocalGame202SHA256, ct084LocalGame202Size)
	if err := os.WriteFile(evidencePath, []byte(evidence), 0o600); err != nil {
		t.Fatal(err)
	}
	scriptPath, err := filepath.Abs("tools/generate_ct084_patches.ps1")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(powerShell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", scriptPath, "-InputCT", inputPath, "-OriginalEvidence", evidencePath, "-Output", outputPath)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("generator rejected locked original evidence: %v\n%s", err, output)
	}
	raw, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	var catalog CT084Catalog
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Features) != 1 || len(catalog.Features[0].Sites) != 1 {
		t.Fatalf("generated catalog shape=%+v", catalog.Features)
	}
	site := catalog.Features[0].Sites[0]
	if !reflect.DeepEqual(site.ExpectedOriginalBytes, []byte{0xCC}) {
		t.Fatalf("expected original bytes=% X, want CC", site.ExpectedOriginalBytes)
	}
	if !site.RequiresRuntimeCapture || len(site.DisableBytes) != 0 {
		t.Fatalf("generator rewrote CT capture provenance: requiresRuntimeCapture=%t disable=% X", site.RequiresRuntimeCapture, site.DisableBytes)
	}
}

func runCT084GeneratorFixture(t *testing.T, name, input string) CT084Catalog {
	t.Helper()
	catalog, _ := runCT084GeneratorFixtureWithArgs(t, name, input)
	return catalog
}

func runCT084GeneratorFixtureWithArgs(t *testing.T, name, input string, extraArgs ...string) (CT084Catalog, string) {
	t.Helper()
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
	inputPath := filepath.Join(tempDir, name+".ct")
	outputPath := filepath.Join(tempDir, "catalog.json")
	if err := os.WriteFile(inputPath, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	scriptPath, err := filepath.Abs("tools/generate_ct084_patches.ps1")
	if err != nil {
		t.Fatal(err)
	}
	arguments := []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", scriptPath, "-InputCT", inputPath, "-Output", outputPath}
	arguments = append(arguments, extraArgs...)
	command := exec.Command(powerShell, arguments...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("generator failed: %v\n%s", err, output)
	}
	raw, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	var catalog CT084Catalog
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	return catalog, string(output)
}

func TestCT084GeneratorIgnoresCommentedInstructions(t *testing.T) {
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
	catalog := runCT084GeneratorFixture(t, "comments", input)
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

type ct084GeneratorExclusionFixture struct {
	id             int
	classification string
	evidence       string
}

var ct084GeneratorExclusionFixtures = []ct084GeneratorExclusionFixture{
	{id: 31935, classification: "known unsafe", evidence: "disabling Eugen's instant Detonator can crash"},
	{id: 33086, classification: "known unsafe", evidence: "infinite repeat quest is experimental and potentially buggy"},
	{id: 31066, classification: "unsafe or unverified", evidence: "NBGFR019B has two matches in the locked game 2.0.2 EXE"},
	{id: 31960, classification: "unsafe or unverified", evidence: "NBGFR040 has three matches in the locked game 2.0.2 EXE"},
	{id: 31060, classification: "already implemented", evidence: "Infinite Link Time has an independently owned implementation"},
	{id: 31456, classification: "already implemented", evidence: "Terminus weapon drop has a safer independently owned implementation"},
}

func makeCT084GeneratorExclusionFixture(remapExcluded bool) string {
	var input strings.Builder
	input.WriteString("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<CheatTable><CheatEntries>\n")
	entries := append([]ct084GeneratorExclusionFixture(nil), ct084GeneratorExclusionFixtures...)
	entries = append(entries, ct084GeneratorExclusionFixture{id: 900007, classification: "control", evidence: "must remain eligible"})
	for index, fixture := range entries {
		ctID := fixture.id
		if remapExcluded && index < len(ct084GeneratorExclusionFixtures) {
			ctID = 910000 + index
		}
		symbol := fmt.Sprintf("EXCLUSION_FIXTURE_%d", index)
		fmt.Fprintf(&input, `<CheatEntry>
  <ID>%d</ID>
  <Description>%s: %s</Description>
  <AssemblerScript><![CDATA[
[ENABLE]
aobscanmodule(%s,$process,48 8B %02X 89 54 24 10 90)
%s+3:
  db 90
[DISABLE]
%s+3:
  db 89
]]></AssemblerScript>
</CheatEntry>
`, ctID, fixture.classification, fixture.evidence, symbol, 0x40+index, symbol, symbol)
	}
	input.WriteString("</CheatEntries></CheatTable>\n")
	return input.String()
}

func TestCT084GeneratorBehaviorExcludesEveryClassifiedFeature(t *testing.T) {
	catalog, audit := runCT084GeneratorFixtureWithArgs(t, "classified-exclusions", makeCT084GeneratorExclusionFixture(false), "-Verbose")
	if len(catalog.Features) != 1 || catalog.Features[0].CTID != 900007 {
		t.Fatalf("classified exclusion fixture generated CT IDs %v, want only control CT 900007", ct084CatalogIDs(catalog.Features))
	}
	for _, fixture := range ct084GeneratorExclusionFixtures {
		for _, feature := range catalog.Features {
			if feature.CTID == fixture.id {
				t.Errorf("generator emitted CT %d (%s: %s)", fixture.id, fixture.classification, fixture.evidence)
			}
		}
		wantAudit := fmt.Sprintf("Excluded CT %d [%s]: %s", fixture.id, fixture.classification, fixture.evidence)
		if !strings.Contains(audit, wantAudit) {
			t.Errorf("generator verbose audit does not contain %q; output=%q", wantAudit, audit)
		}
	}

	// Remapping only the six IDs proves that every script body is independently
	// eligible; the first result cannot pass because another generator filter
	// happened to reject malformed fixtures.
	remapped := runCT084GeneratorFixture(t, "remapped-exclusions", makeCT084GeneratorExclusionFixture(true))
	if len(remapped.Features) != len(ct084GeneratorExclusionFixtures)+1 {
		t.Fatalf("remapped direct-patch fixtures generated CT IDs %v, want all seven fixtures", ct084CatalogIDs(remapped.Features))
	}
	for _, feature := range remapped.Features {
		if len(feature.Sites) != 1 || len(feature.Sites[0].EnableBytes) != 1 {
			t.Errorf("remapped control CT %d was not parsed as a one-site direct patch", feature.CTID)
		}
	}
}

func ct084CatalogIDs(features []CT084Feature) []int {
	ids := make([]int, len(features))
	for index, feature := range features {
		ids[index] = feature.CTID
	}
	return ids
}

func TestCT084CatalogKnownMultiSiteAndConflicts(t *testing.T) {
	catalog := readCT084CatalogFile(t)
	byCTID := make(map[int]CT084Feature, len(catalog.Features))
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
