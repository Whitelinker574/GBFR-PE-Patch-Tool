package backend

import (
	"bytes"
	"encoding/binary"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestFirmStancePlusMatchesBundledGemTable(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	gem, err := naturalDropBundledTable("gem.tbl")
	if err != nil {
		t.Fatal(err)
	}
	// The bundled loader checks the original table size and SHA-256. These
	// fields come from its 64-byte gem rows, not a translated runtime name.
	for _, test := range []struct {
		id        string
		hash      uint32
		lot       int32
		secondary bool
	}{
		{"GEEN_087_04", 0x80DC0FB8, -1, false},
		{"GEEN_087_24", 0xB1CCC211, 7, true},
	} {
		sigil, err := catalog.RequireSigil(test.id)
		if err != nil {
			t.Fatal(err)
		}
		hash, err := ParseHashHex(sigil.Hash)
		if err != nil || hash != test.hash {
			t.Fatalf("%s hash=%s err=%v", test.id, sigil.Hash, err)
		}
		offset, ok := naturalSigilGemRowOffset(gem, test.hash)
		if !ok {
			t.Fatalf("%s missing from original gem.tbl", test.id)
		}
		if binary.LittleEndian.Uint32(gem[offset:]) != 0xB6E31F76 ||
			binary.LittleEndian.Uint32(gem[offset+4:]) != EmptyHash ||
			int32(binary.LittleEndian.Uint32(gem[offset+36:])) != test.lot {
			t.Fatalf("%s primary/fixed-secondary/random-lot disagrees with raw gem.tbl", test.id)
		}
		if sigil.PrimaryTraitID != "SKILL_087_00" || supportsGeneratedPlusSigil(sigil) != test.secondary || !catalog.IsSigilConstructible(sigil) {
			t.Fatalf("%s catalog metadata disagrees with original row", test.id)
		}
	}
	plus, _ := catalog.RequireSigil("GEEN_087_24")
	// Improved Dodge+ uses the same selector 7. Its exact joined pool is
	// already independently checked by the historical 189-row table audit.
	reference, _ := catalog.RequireSigil("GEEN_063_24")
	offset, ok := naturalSigilGemRowOffset(gem, 0xE89224A1)
	if !ok || int32(binary.LittleEndian.Uint32(gem[offset+36:])) != 7 {
		t.Fatal("audited lot-7 reference changed")
	}
	if len(plus.AllowedSecondaryTraitIDs) != 16 || !slices.Equal(plus.AllowedSecondaryTraitIDs, reference.AllowedSecondaryTraitIDs) {
		t.Fatal("Firm Stance V+ must use the exact audited lot-7 pool")
	}
	if !slices.Equal(plus.AllowedSigilLevels, []int{15}) || !slices.Equal(plus.AllowedFirstTraitLevels, []int{15}) {
		t.Fatal("Firm Stance V+ must retain the highest-tier constructor's Lv15 defaults")
	}
	if sigilCN["Firm Stance V+"] != "不动 V+" {
		t.Fatal("Firm Stance V+ Chinese picker label is missing")
	}
}

func TestFirmStancePlusSecondaryAcrossEditors(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	info := sigilInfoJSONByID(t, "GEEN_087_24")
	if !boolField(t, info, "supportsSecondaryTrait") || !boolField(t, info, "constructible") {
		t.Fatal("frontend must receive a constructible two-slot V+ entry")
	}
	for _, test := range []struct {
		name   string
		hash   uint32
		status string
	}{
		{"audited natural pool", 0xE69A4694, LegalityLegal},        // Guts
		{"issue 27 synthesis preview", 0xA898E283, LegalityForced}, // Celestial Aqua
	} {
		t.Run(test.name, func(t *testing.T) {
			secondary := catalog.LookupTraitByHash(test.hash)
			if secondary == nil {
				t.Fatalf("missing secondary 0x%08X", test.hash)
			}
			if test.status == LegalityForced && !catalog.SynthesisTraitHashes[test.hash] {
				t.Fatal("screenshot secondary must also be present in the synthesis table")
			}
			item := QueueItem{SigilID: "GEEN_087_24", Level: 15, PrimaryLevel: 15,
				SecondaryTraitID: secondary.InternalID, SecondaryLevel: 15, Quantity: 1}
			gen := &SigilGen{catalog: catalog}
			choices, err := gen.GetCompatibleSecondaryTraits(item.SigilID)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, choice := range choices {
				found = found || choice.InternalID == secondary.InternalID
			}
			if !found {
				t.Fatal("frontend secondary choices omit the verified combination")
			}
			report, err := gen.CheckLegality(item)
			if err != nil || !report.Writable || report.Status != test.status {
				t.Fatalf("combination rejected or misclassified: %+v err=%v", report, err)
			}
			// The existing policy reports out-of-drop-pool synthesis as an
			// advisory warning; it must not become an impossible write.
			if err := gen.AddToQueue(item); err != nil {
				t.Fatal(err)
			}
			_, update := customSecondaryUpdate(t, item.SigilID, secondary.InternalID)
			if err := validateSigilMemoryWriteRequest(catalog, update); err != nil {
				t.Fatalf("runtime rejected combination: %v", err)
			}
			if _, err := prepareLoadoutSigil(catalog, LoadoutConstructedSigil{Index: 0, Item: item}); err != nil {
				t.Fatalf("loadout constructor rejected combination: %v", err)
			}
			original := bytes.Repeat([]byte{0x5A}, sigilMemoryRecordSize)
			encoded, err := encodeSigilMemoryRecord(original, update)
			if err != nil {
				t.Fatal(err)
			}
			if binary.LittleEndian.Uint32(encoded[0:]) != 0xB6E31F76 ||
				binary.LittleEndian.Uint32(encoded[8:]) != test.hash ||
				binary.LittleEndian.Uint32(encoded[16:]) != 0xB1CCC211 ||
				!bytes.Equal(encoded[20:24], original[20:24]) || !bytes.Equal(original, bytes.Repeat([]byte{0x5A}, sigilMemoryRecordSize)) {
				t.Fatal("two-slot record encoding changed identities, reserved bytes, or original snapshot")
			}
		})
	}
}

func TestFirmStanceRecoveryKeepsInvalidSecondariesBlocked(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, sigilID, traitID string
		hash                   uint32
	}{
		{"original Firm Stance V remains single-slot", "GEEN_087_04", "SKILL_045_00", 0xE69A4694},
		{"quest-only single-slot remains single-slot", "49434696", "SKILL_045_00", 0xE69A4694},
		{"unknown secondary", "GEEN_087_24", "UNKNOWN_TRAIT", 0xDEADBEEF},
		{"catalogued quest trait absent from synthesis pool", "GEEN_087_24", "MEMORY_TRAIT_BF78FBFC", 0xBF78FBFC},
	} {
		t.Run(test.name, func(t *testing.T) {
			gen := &SigilGen{catalog: catalog}
			item := QueueItem{SigilID: test.sigilID, Level: 15, PrimaryLevel: 15,
				SecondaryTraitID: test.traitID, SecondaryLevel: 15, Quantity: 1}
			if test.hash == 0xBF78FBFC {
				trait := catalog.LookupTraitByHash(test.hash)
				if trait == nil || catalog.SynthesisTraitHashes[test.hash] || strings.HasPrefix(trait.InternalID, "SKILL_") {
					t.Fatal("negative test requires a known non-synthesis trait")
				}
				item.SecondaryTraitID = trait.InternalID
			}
			report, err := gen.CheckLegality(item)
			if err != nil || report.Writable || report.Status != LegalityImpossible {
				t.Fatalf("invalid secondary allowed: %+v err=%v", report, err)
			}
			if err := gen.AddToQueue(item); err == nil || len(gen.GetQueue()) != 0 {
				t.Fatal("invalid secondary entered the queue")
			}
			sigil, _ := catalog.RequireSigil(test.sigilID)
			sigilHash, _ := ParseHashHex(sigil.Hash)
			primary, _ := catalog.RequireTrait(sigil.PrimaryTraitID)
			primaryHash, _ := ParseHashHex(primary.Hash)
			update := SigilMemoryUpdate{SigilHash: sigilHash, SigilLevel: 15,
				PrimaryTraitHash: primaryHash, PrimaryTraitLevel: 15,
				SecondaryTraitHash: test.hash, SecondaryTraitLevel: 15}
			if err := validateSigilMemoryWriteRequest(catalog, update); err == nil {
				t.Fatal("runtime accepted invalid secondary")
			}
			if _, err := prepareLoadoutSigil(catalog, LoadoutConstructedSigil{Index: 0, Item: item}); err == nil {
				t.Fatal("loadout constructor accepted invalid secondary")
			}
		})
	}
}

func TestFirmStancePlusSaveCopyBacksUpAndReadsBothTraits(t *testing.T) {
	// Opt in with GBFR_TEST_LOADOUT_SAVE. Never modify the supplied sample:
	// the shared helper copies it to a temporary directory first.
	gen, work, slotID := queuedSigilGeneratorOnSaveCopy(t)
	gen.ClearQueue()
	before, err := os.ReadFile(work)
	if err != nil {
		t.Fatal(err)
	}
	secondary := gen.catalog.LookupTraitByHash(0xA898E283)
	if secondary == nil {
		t.Fatal("missing Celestial Aqua")
	}
	if err := gen.AddToQueue(QueueItem{SigilID: "GEEN_087_24", Level: 15, PrimaryLevel: 15,
		SecondaryTraitID: secondary.InternalID, SecondaryLevel: 15, Quantity: 1}); err != nil {
		t.Fatal(err)
	}
	result, err := gen.ApplyQueue(work)
	if err != nil {
		t.Fatal(err)
	}
	if result.CreatedCount != 1 || result.VerifiedCount != 1 || result.BackupPath == "" {
		t.Fatalf("missing creation/readback/backup: %+v", result)
	}
	backup, err := os.ReadFile(result.BackupPath)
	if err != nil || !bytes.Equal(backup, before) {
		t.Fatalf("backup did not preserve the original sample copy: %v", err)
	}
	written, err := LoadSave(work)
	if err != nil {
		t.Fatal(err)
	}
	var unitID uint32
	for _, entry := range written.findAllUnitsByType(GemSlotIDType) {
		if entry.Uint32() == slotID {
			unitID = entry.UnitID
			break
		}
	}
	if unitID < GemSlotBaseID {
		t.Fatal("new factor slot missing after readback")
	}
	primaryUnit := uint32(TraitSlotBase + (int(unitID)-GemSlotBaseID)*2)
	for _, field := range []struct{ kind, unit, value uint32 }{
		{GemIDType, unitID, 0xB1CCC211},
		{GemLevelIDType, unitID, 15},
		{GemFlagsIDType, unitID, NormalSigilFlags},
		{TraitHashIDType, primaryUnit, 0xB6E31F76},
		{TraitLevelIDType, primaryUnit, 15},
		{TraitHashIDType, primaryUnit + 1, 0xA898E283},
		{TraitLevelIDType, primaryUnit + 1, 15},
	} {
		entry, ok := written.findUnitExact(field.kind, field.unit)
		if !ok || entry.Uint32() != field.value {
			t.Fatalf("save field %d/%d did not roundtrip as 0x%08X", field.kind, field.unit, field.value)
		}
	}
}
