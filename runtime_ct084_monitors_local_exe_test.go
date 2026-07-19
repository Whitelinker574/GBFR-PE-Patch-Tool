package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

func ct084LocalBytesAtRVA(sections []ct084LocalExecutableSection, rva uint32, size int) []byte {
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

func TestCT084ReadOnlyMonitorsMatchLocalGame202(t *testing.T) {
	path := os.Getenv("GBFR_GAME_EXE_TEST")
	if path == "" {
		t.Skip("set GBFR_GAME_EXE_TEST to verify the locally supplied game 2.0.2 executable")
	}
	if err := verifyCT084LocalGameIdentity(path); err != nil {
		t.Fatal(err)
	}
	sections, err := readCT084LocalExecutableSections(path)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		raw      string
		wantRVA  uintptr
		original []byte
	}{
		{name: "party pointer / CT node 30967", raw: ct084PartyPointerAOB, wantRVA: ct084PartyPointerRVA},
		{name: "selected material / CT node 33552 A", raw: ct084SelectedMaterialAOB, wantRVA: ct084SelectedMaterialRVA, original: ct084SelectedMaterialOriginal},
		{name: "selected key item / CT node 33552 B", raw: ct084SelectedKeyItemAOB, wantRVA: ct084SelectedKeyItemRVA, original: ct084SelectedKeyItemOriginal},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pattern, err := parseCT084Pattern(test.raw)
			if err != nil {
				t.Fatal(err)
			}
			matches := findCT084LocalPatternMatches(sections, pattern)
			if len(matches) != 1 || uintptr(matches[0].rva) != test.wantRVA {
				t.Fatalf("matches=%s, want one match at RVA 0x%X", formatCT084LocalMatchLocations(matches), test.wantRVA)
			}
			if len(test.original) != 0 {
				entry := ct084LocalBytesAtRVA(sections, uint32(test.wantRVA), len(test.original))
				if !bytes.Equal(entry, test.original) {
					t.Fatalf("entry=% X, want % X", entry, test.original)
				}
			}
		})
	}

	partyEntry := ct084LocalBytesAtRVA(sections, uint32(ct084PartyPointerRVA), 7)
	if len(partyEntry) != 7 {
		t.Fatal("party RIP-relative entry is unavailable")
	}
	displacement := int64(int32(binary.LittleEndian.Uint32(partyEntry[3:7])))
	resolved := int64(ct084PartyPointerRVA) + 7 + displacement
	if resolved != int64(ct084PartySlotTableRVA) {
		t.Fatalf("party root target RVA=0x%X, want 0x%X", resolved, ct084PartySlotTableRVA)
	}
}
