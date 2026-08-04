package backend

import (
	"bytes"
	"os"
	"testing"
)

func TestMonsterEnhanceLocal203Signatures(t *testing.T) {
	path := os.Getenv("GBFR_GAME_EXE_203_TEST")
	if path == "" {
		t.Skip("GBFR_GAME_EXE_203_TEST is not configured")
	}
	if err := verifyRuntimePatchLocalGameIdentityExact(path, runtimePatchLocalGame203Size, runtimePatchLocalGame203SHA256); err != nil {
		t.Fatal(err)
	}
	sections, err := readRuntimePatchLocalExecutableSections(path)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		aob       string
		matchRVA  uint32
		targetRVA uint32
		original  []byte
	}{
		{"monster hp", "48 8B 41 10 45 31 C9 48 29 D0 4C 0F 43 C8 B8 01 00 00 00 49 0F 47 C1 45 85 C0 49 0F 44 C1 48 89 41 10 C3", 0x1F74710, 0x1F74710, []byte{0x48, 0x8B, 0x41, 0x10, 0x45, 0x31, 0xC9}},
		{"party monster damage", "48 89 51 18 48 89 51 10 C3 CC CC CC CC CC CC CC 48 89 51 18 C3 CC CC CC CC CC CC CC CC CC CC CC 48 89 51 10 C3", 0x1F746E0, 0x1F74700, []byte{0x48, 0x89, 0x51, 0x10, 0xC3, 0xCC, 0xCC, 0xCC, 0xCC, 0xCC, 0xCC, 0xCC, 0xCC, 0xCC}},
		{"monster stun", "C5 FA 58 86 60 ?? ?? ?? C5 FA 5D 86 64 ?? ?? ?? C5 FA 11 86 60 ?? ?? ??", 0xB228A8, 0xB228A8, []byte{0xC5, 0xFA, 0x58, 0x86, 0x60, 0x08, 0x00, 0x00}},
		{"overdrive state", "8B 46 10 83 F8 03 0F 84 ?? ?? ?? ?? 83 F8 01 0F 84 ?? ?? ?? ??", 0x22C5986, 0x22C5986, []byte{0x8B, 0x46, 0x10, 0x83, 0xF8, 0x03}},
		{"OD gauge rate", "80 79 50 00 74 13 48 03 51 18 48 C7 C0 FF FF FF FF 48 0F 43 C2 48 89 41 18 C3", 0x22C5E50, 0x22C5E50, []byte{0x80, 0x79, 0x50, 0x00, 0x74, 0x13, 0x48, 0x03, 0x51, 0x18, 0x48, 0xC7, 0xC0, 0xFF, 0xFF, 0xFF, 0xFF, 0x48, 0x0F, 0x43, 0xC2, 0x48, 0x89, 0x41, 0x18, 0xC3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern, err := parseRuntimePatchPattern(tt.aob)
			if err != nil {
				t.Fatal(err)
			}
			matches := findRuntimePatchLocalPatternMatches(sections, pattern)
			if len(matches) != 1 || matches[0].rva != tt.matchRVA {
				t.Fatalf("matches = %#x, want unique RVA %#x", matches, tt.matchRVA)
			}
			var got []byte
			for _, section := range sections {
				if tt.targetRVA >= section.rva && uint64(tt.targetRVA-section.rva)+uint64(len(tt.original)) <= uint64(len(section.data)) {
					offset := int(tt.targetRVA - section.rva)
					got = section.data[offset : offset+len(tt.original)]
					break
				}
			}
			if !bytes.Equal(got, tt.original) {
				t.Fatalf("target RVA %#x bytes = % X, want % X", tt.targetRVA, got, tt.original)
			}
		})
	}
	auxMask := make([]byte, len(monsterDamagePlayerPointerMask))
	for index, exact := range monsterDamagePlayerPointerMask {
		if exact {
			auxMask[index] = 0xFF
		}
	}
	auxMatches := findRuntimePatchLocalPatternMatches(sections, runtimePatchPattern{
		Values: monsterDamagePlayerPointerPattern,
		Mask:   auxMask,
	})
	if len(auxMatches) != 1 {
		t.Fatalf("party player-pointer auxiliary matches = %s, want one", formatRuntimePatchLocalMatchLocations(auxMatches))
	}
	if auxMatches[0].rva+0x14 != 0x2607DDE {
		t.Fatalf("party player-pointer auxiliary target = %#x, want 0x2607DDE", auxMatches[0].rva+0x14)
	}
	wantAux := []byte{0x48, 0x81, 0xC1, 0x50, 0x01, 0x00, 0x00}
	var gotAux []byte
	for _, section := range sections {
		if 0x2607DDE >= section.rva && uint64(0x2607DDE-section.rva)+uint64(len(wantAux)) <= uint64(len(section.data)) {
			offset := int(0x2607DDE - section.rva)
			gotAux = section.data[offset : offset+len(wantAux)]
			break
		}
	}
	if !bytes.Equal(gotAux, wantAux) {
		t.Fatalf("party player-pointer auxiliary original = % X, want % X", gotAux, wantAux)
	}
}
