package backend

import (
	"bytes"
	"os"
	"testing"
)

const runtimePatchInfiniteLinkAOB = "C5 ?? ?? ?? ?? ?? ?? ?? C4 ?? ?? ?? ?? ?? ?? ?? ?? ?? C5 ?? ?? ?? ?? ?? ?? ?? C5 ?? ?? ?? C5 ?? ?? ?? 0F 86"

func TestRuntimePatchInfiniteLinkSignatureEvidence(t *testing.T) {
	pattern, err := parseRuntimePatchPattern(runtimePatchInfiniteLinkAOB)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		name       string
		env        string
		size       int64
		sha256     string
		wantRVA    uint32
		wantBefore []byte
	}{
		{name: "2.0.2", env: "GBFR_GAME_EXE_TEST", size: runtimePatchLocalGame202Size, sha256: runtimePatchLocalGame202SHA256, wantRVA: 0x19ABDC, wantBefore: []byte{0xC5, 0xFA, 0x59, 0x05, 0xB4, 0x95, 0x30, 0x05}},
		{name: "2.0.3", env: "GBFR_GAME_EXE_203_TEST", size: runtimePatchLocalGame203Size, sha256: runtimePatchLocalGame203SHA256, wantRVA: 0x193CCC, wantBefore: []byte{0xC5, 0xFA, 0x59, 0x05, 0x44, 0xC4, 0x30, 0x05}},
		{name: "2.0.4", env: "GBFR_GAME_EXE_204_TEST", size: runtimePatchLocalGame204Size, sha256: runtimePatchLocalGame204SHA256, wantRVA: 0x193CCC, wantBefore: []byte{0xC5, 0xFA, 0x59, 0x05, 0x44, 0xD4, 0x30, 0x05}},
		{name: "2.0.5", env: "GBFR_GAME_EXE_205_TEST", size: runtimePatchLocalGame205Size, sha256: runtimePatchLocalGame205SHA256, wantRVA: 0x193CCC, wantBefore: []byte{0xC5, 0xFA, 0x59, 0x05, 0x44, 0xD4, 0x30, 0x05}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			path := os.Getenv(fixture.env)
			if path == "" {
				t.Skipf("set %s to verify the locked game executable", fixture.env)
			}
			if err := verifyRuntimePatchLocalGameIdentityExact(path, fixture.size, fixture.sha256); err != nil {
				t.Fatal(err)
			}
			sections, err := readRuntimePatchLocalExecutableSections(path)
			if err != nil {
				t.Fatal(err)
			}
			matches := findRuntimePatchLocalPatternMatches(sections, pattern)
			if len(matches) != 1 {
				t.Fatalf("matches=%d: %s", len(matches), formatRuntimePatchLocalMatchLocations(matches))
			}
			match := matches[0]
			if fixture.wantRVA == 0 {
				t.Fatalf("lock evidence: rva=0x%X", match.rva)
			}
			if match.rva != fixture.wantRVA {
				t.Fatalf("rva=0x%X, want 0x%X", match.rva, fixture.wantRVA)
			}
			for _, section := range sections {
				if section.name != match.section {
					continue
				}
				offset := int(match.rva - section.rva)
				if offset < 0 || offset+8 > len(section.data) {
					t.Fatal("patch slice is outside executable section")
				}
				before := section.data[offset : offset+8]
				if len(fixture.wantBefore) == 0 {
					t.Fatalf("lock evidence: original=% X", before)
				}
				if !bytes.Equal(before, fixture.wantBefore) {
					t.Fatalf("original=% X, want % X", before, fixture.wantBefore)
				}
				return
			}
			t.Fatal("matched executable section not found")
		})
	}
}

func TestRuntimePatchInfiniteLinkInstallAndExactRestore(t *testing.T) {
	catalog, err := decodeRuntimePatchCatalog(runtimePatchCatalogJSON)
	if err != nil {
		t.Fatal(err)
	}
	feature := findRuntimePatchCatalogFeature(catalog, "runtime-patch-060")
	if feature == nil || len(feature.Sites) != 1 {
		t.Fatalf("infinite Link catalog feature is missing or malformed: %+v", feature)
	}
	site, err := runtimePatchSiteForExecutable(feature.Sites[0], runtimePatchLocalGame203SHA256)
	if err != nil {
		t.Fatal(err)
	}
	address := uintptr(0x140000000 + 0x193CCC)
	lease := runtimePatchPatchSiteLease{
		Address:  address,
		RVA:      0x193CCC,
		Original: append([]byte(nil), site.ExpectedOriginalBytes...),
		Patch:    append([]byte(nil), site.EnableBytes...),
	}
	memory := newRuntimePatchFakeMemory(map[uintptr][]byte{address: lease.Original})
	if err := installRuntimePatchSites(memory, []runtimePatchPatchSiteLease{lease}); err != nil {
		t.Fatalf("install infinite Link patch: %v", err)
	}
	if got := memory.data[address]; !bytes.Equal(got, lease.Patch) {
		t.Fatalf("installed bytes=% X, want % X", got, lease.Patch)
	}
	if err := restoreRuntimePatchSites(memory, []runtimePatchPatchSiteLease{lease}); err != nil {
		t.Fatalf("restore infinite Link patch: %v", err)
	}
	if got := memory.data[address]; !bytes.Equal(got, lease.Original) {
		t.Fatalf("restored bytes=% X, want % X", got, lease.Original)
	}
}
