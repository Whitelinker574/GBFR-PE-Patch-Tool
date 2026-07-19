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
	"strconv"
	"strings"
	"testing"
)

const ct084SourceSHA256 = "B75DF049E27D1423FC5ECDD47CC85DBAC241BEE582A49CEBA30CF020E150B659"
const ct084CatalogSHA256 = "94236B5AF66B2169B3FDF8249FEE26E733E78F332FA96FB52582F3FEE97B6D96"

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
		{name: "unknown field", raw: `{"schemaVersion":1,"sourceVersion":"0.8.4","sourceSha256":"` + ct084SourceSHA256 + `","features":[],"unknown":true}`},
		{name: "trailing value", raw: `{"schemaVersion":1,"sourceVersion":"0.8.4","sourceSha256":"` + ct084SourceSHA256 + `","features":[]} {}`},
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
		"disableBytes",
	} {
		if bytes.Contains(raw, []byte(`"`+field+`":null`)) {
			t.Errorf("CT084GetCatalog() JSON contains null array for %s", field)
		}
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
	var catalog CT084Catalog
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

func readCT084GeneratorExclusionSet(t *testing.T, script, variable string) []int {
	t.Helper()
	declaration := "$" + variable + " = [System.Collections.Generic.HashSet[int]]::new()"
	declarationIndex := strings.Index(script, declaration)
	if declarationIndex < 0 {
		t.Fatalf("generator does not declare $%s as a HashSet[int]", variable)
	}
	remainder := script[declarationIndex+len(declaration):]
	const loopPrefix = "foreach ($excludedCTID in @("
	loopIndex := strings.Index(remainder, loopPrefix)
	if loopIndex < 0 {
		t.Fatalf("generator does not populate $%s", variable)
	}
	remainder = remainder[loopIndex+len(loopPrefix):]
	loopEnd := strings.Index(remainder, ")) {")
	if loopEnd < 0 {
		t.Fatalf("generator has no complete population loop for $%s", variable)
	}

	var ids []int
	for _, line := range strings.Split(remainder[:loopEnd], "\n") {
		if comment := strings.IndexByte(line, '#'); comment >= 0 {
			line = line[:comment]
		}
		for _, field := range strings.FieldsFunc(line, func(char rune) bool {
			return char == ',' || char == ' ' || char == '\t' || char == '\r'
		}) {
			id, err := strconv.Atoi(field)
			if err != nil {
				t.Fatalf("generator $%s contains non-integer exclusion %q", variable, field)
			}
			ids = append(ids, id)
		}
	}
	if !strings.Contains(remainder[loopEnd:], "$"+variable+".Add($excludedCTID)") {
		t.Fatalf("generator population loop does not add IDs to $%s", variable)
	}
	return ids
}

func TestCT084GeneratorClassifiesExclusionsBySafetyEvidence(t *testing.T) {
	raw, err := os.ReadFile("tools/generate_ct084_patches.ps1")
	if err != nil {
		t.Fatal(err)
	}
	script := string(raw)
	wantSets := []struct {
		variable string
		ids      []int
	}{
		{variable: "knownUnsafeCTIDs", ids: []int{31935, 33086}},
		{variable: "unsafeOrUnverifiedCTIDs", ids: []int{31066, 31960}},
		{variable: "alreadyImplementedCTIDs", ids: []int{31060, 31456}},
	}
	for _, want := range wantSets {
		if got := readCT084GeneratorExclusionSet(t, script, want.variable); !reflect.DeepEqual(got, want.ids) {
			t.Errorf("generator $%s=%v, want %v", want.variable, got, want.ids)
		}
		if filter := "$" + want.variable + ".Contains($ctID)"; !strings.Contains(script, filter) {
			t.Errorf("generator does not filter with %s", filter)
		}
	}
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
