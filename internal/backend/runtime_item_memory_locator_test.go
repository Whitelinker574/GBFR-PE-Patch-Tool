package backend

import (
	"os"
	"testing"
)

func TestRuntimeItemLocatorPatternsAreCanonicalAndGuardTheirEntries(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		prefix []byte
	}{
		{name: "inventory material", raw: runtimeInventoryMaterialAOB, prefix: materialConsumeOrig},
		{name: "sigil hook", raw: runtimeSigilHookAOB, prefix: sigilMemoryOriginalBytes},
		{name: "wrightstone hook", raw: runtimeWrightstoneHookAOB, prefix: wrightstoneMemoryOriginalBytes},
		{name: "item save function", raw: runtimeItemSaveFunctionAOB, prefix: gameSaveFunctionPrologue},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pattern, err := parseRuntimePatchPattern(test.raw)
			if err != nil {
				t.Fatal(err)
			}
			if canonicalRuntimePatchAOB(pattern) != test.raw {
				t.Fatalf("AOB is not canonical: %q", canonicalRuntimePatchAOB(pattern))
			}
			if len(pattern.Values) < len(test.prefix) {
				t.Fatalf("pattern length=%d, entry length=%d", len(pattern.Values), len(test.prefix))
			}
			for index, value := range test.prefix {
				if pattern.Mask[index] != 0xFF || pattern.Values[index] != value {
					t.Fatalf("entry byte[%d]=%02X/%02X, want exact %02X", index, pattern.Values[index], pattern.Mask[index], value)
				}
			}
		})
	}
}

func TestRuntimeItemLocatorPatternsMatchOneKnownSiteInLocalGame(t *testing.T) {
	path := os.Getenv("GBFR_GAME_EXE_TEST")
	if path == "" {
		t.Skip("set GBFR_GAME_EXE_TEST to verify a local 2.0.2 or 2.0.3 executable")
	}
	sections, err := readRuntimePatchLocalExecutableSections(path)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		raw       string
		selectRVA func(runtimeGameLayout) uintptr
	}{
		{name: "inventory material", raw: runtimeInventoryMaterialAOB, selectRVA: func(layout runtimeGameLayout) uintptr { return layout.InventoryMaterialRVA }},
		{name: "sigil hook", raw: runtimeSigilHookAOB, selectRVA: func(layout runtimeGameLayout) uintptr { return layout.SigilHookRVA }},
		{name: "wrightstone hook", raw: runtimeWrightstoneHookAOB, selectRVA: func(layout runtimeGameLayout) uintptr { return layout.WrightstoneHookRVA }},
		{name: "item save function", raw: runtimeItemSaveFunctionAOB, selectRVA: func(layout runtimeGameLayout) uintptr { return layout.SaveFunctionRVA }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pattern, err := parseRuntimePatchPattern(test.raw)
			if err != nil {
				t.Fatal(err)
			}
			matches := findRuntimePatchLocalPatternMatches(sections, pattern)
			if len(matches) != 1 {
				t.Fatalf("matches=%s, want exactly one", formatRuntimePatchLocalMatchLocations(matches))
			}
			matchedKnownLayout := false
			for _, layout := range runtimeGameLayouts {
				if uintptr(matches[0].rva) == test.selectRVA(layout) {
					matchedKnownLayout = true
					break
				}
			}
			if !matchedKnownLayout {
				t.Fatalf("unique match RVA 0x%X is not one of the audited layouts", matches[0].rva)
			}
		})
	}
}
