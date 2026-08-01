package backend

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

func runtimePatchLocalBytesAtRVA(sections []runtimePatchLocalExecutableSection, rva uint32, size int) []byte {
	for _, section := range sections {
		if rva < section.rva {
			continue
		}
		offset := uint64(rva - section.rva)
		if offset+uint64(size) <= uint64(len(section.data)) {
			return append([]byte(nil), section.data[int(offset):int(offset)+size]...)
		}
	}
	return nil
}

func TestRuntimePatchReadOnlyMonitorsMatchLocalGame202(t *testing.T) {
	path := os.Getenv("GBFR_GAME_EXE_TEST")
	if path == "" {
		t.Skip("set GBFR_GAME_EXE_TEST to verify the locally supplied game 2.0.2 executable")
	}
	if err := verifyRuntimePatchLocalGameIdentity(path); err != nil {
		t.Fatal(err)
	}
	sections, err := readRuntimePatchLocalExecutableSections(path)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		raw      string
		wantRVA  uintptr
		original []byte
	}{
		{name: "party pointer", raw: runtimePatchPartyPointerAOB, wantRVA: runtimeGameLayouts[0].PartyPointerRVA},
		{name: "selected material", raw: runtimePatchSelectedMaterialAOB, wantRVA: runtimeGameLayouts[0].SelectedMaterialRVA, original: runtimePatchSelectedMaterialOriginal},
		{name: "selected key item", raw: runtimePatchSelectedKeyItemAOB, wantRVA: runtimeGameLayouts[0].SelectedKeyItemRVA, original: runtimePatchSelectedKeyItemOriginal},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pattern, err := parseRuntimePatchPattern(test.raw)
			if err != nil {
				t.Fatal(err)
			}
			matches := findRuntimePatchLocalPatternMatches(sections, pattern)
			if len(matches) != 1 || uintptr(matches[0].rva) != test.wantRVA {
				t.Fatalf("matches=%s, want one match at RVA 0x%X", formatRuntimePatchLocalMatchLocations(matches), test.wantRVA)
			}
			if len(test.original) != 0 {
				entry := runtimePatchLocalBytesAtRVA(sections, uint32(test.wantRVA), len(test.original))
				if !bytes.Equal(entry, test.original) {
					t.Fatalf("entry=% X, want % X", entry, test.original)
				}
			}
		})
	}

	partyEntry := runtimePatchLocalBytesAtRVA(sections, uint32(runtimeGameLayouts[0].PartyPointerRVA), 7)
	if len(partyEntry) != 7 {
		t.Fatal("party RIP-relative entry is unavailable")
	}
	displacement := int64(int32(binary.LittleEndian.Uint32(partyEntry[3:7])))
	resolved := int64(runtimeGameLayouts[0].PartyPointerRVA) + 7 + displacement
	if resolved != int64(runtimeGameLayouts[0].PartySlotTableRVA) {
		t.Fatalf("party root target RVA=0x%X, want 0x%X", resolved, runtimeGameLayouts[0].PartySlotTableRVA)
	}
}

func TestRuntimePatchReadOnlyMonitorsMatchLocalGame203(t *testing.T) {
	path := os.Getenv("GBFR_GAME_EXE_203_TEST")
	if path == "" {
		t.Skip("set GBFR_GAME_EXE_203_TEST to verify the locally supplied game 2.0.3 executable")
	}
	if err := verifyRuntimePatchLocalGameIdentityExact(path, runtimePatchLocalGame203Size, runtimePatchLocalGame203SHA256); err != nil {
		t.Fatal(err)
	}
	sections, err := readRuntimePatchLocalExecutableSections(path)
	if err != nil {
		t.Fatal(err)
	}
	layout := runtimeGameLayouts[1]
	tests := []struct {
		name     string
		raw      string
		wantRVA  uintptr
		original []byte
	}{
		{name: "party pointer", raw: runtimePatchPartyPointerAOB, wantRVA: layout.PartyPointerRVA},
		{name: "selected material", raw: runtimePatchSelectedMaterialAOB, wantRVA: layout.SelectedMaterialRVA, original: runtimePatchSelectedMaterialOriginal},
		{name: "selected key item", raw: runtimePatchSelectedKeyItemAOB, wantRVA: layout.SelectedKeyItemRVA, original: runtimePatchSelectedKeyItemOriginal},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pattern, err := parseRuntimePatchPattern(test.raw)
			if err != nil {
				t.Fatal(err)
			}
			matches := findRuntimePatchLocalPatternMatches(sections, pattern)
			if len(matches) != 1 || uintptr(matches[0].rva) != test.wantRVA {
				t.Fatalf("matches=%s, want one match at 2.0.3 RVA 0x%X", formatRuntimePatchLocalMatchLocations(matches), test.wantRVA)
			}
			if len(test.original) != 0 {
				entry := runtimePatchLocalBytesAtRVA(sections, uint32(test.wantRVA), len(test.original))
				if !bytes.Equal(entry, test.original) {
					t.Fatalf("entry=% X, want % X", entry, test.original)
				}
			}
		})
	}

	partyEntry := runtimePatchLocalBytesAtRVA(sections, uint32(layout.PartyPointerRVA), 7)
	if len(partyEntry) != 7 {
		t.Fatal("2.0.3 party RIP-relative entry is unavailable")
	}
	displacement := int64(int32(binary.LittleEndian.Uint32(partyEntry[3:7])))
	if resolved := int64(layout.PartyPointerRVA) + 7 + displacement; resolved != int64(layout.PartySlotTableRVA) {
		t.Fatalf("2.0.3 party root target RVA=0x%X, want 0x%X", resolved, layout.PartySlotTableRVA)
	}

	gravityEntry := runtimePatchLocalBytesAtRVA(sections, uint32(layout.SpatialGravityRVA), len(runtimeSpatialGravityOriginal))
	if !bytes.Equal(gravityEntry, runtimeSpatialGravityOriginal) {
		t.Fatalf("2.0.3 gravity entry=% X, want % X", gravityEntry, runtimeSpatialGravityOriginal)
	}
	gravityContext := runtimePatchLocalBytesAtRVA(sections, uint32(layout.SpatialGravityRVA-runtimeSpatialGravityContextBack), len(runtimeSpatialGravityContext))
	if !bytes.Equal(gravityContext, runtimeSpatialGravityContext) {
		t.Fatalf("2.0.3 gravity context=% X, want % X", gravityContext, runtimeSpatialGravityContext)
	}
}
