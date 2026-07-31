package backend

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestNaturalSigilCatalogIncludesVerifiedNativeAndAddedRows(t *testing.T) {
	sourceDir := localNaturalWrightstoneTableDir(t)
	shared, _, err := loadNaturalWrightstoneTables(sourceDir, true)
	if err != nil {
		t.Fatal(err)
	}
	sigilTables, statuses, err := loadNaturalSigilTables(sourceDir, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 1 || !statuses[0].Valid {
		t.Fatalf("sigil table validation=%+v", statuses)
	}
	catalog, err := buildNaturalSigilCatalog(shared.Lots, sigilTables.Gem)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) < 120 {
		t.Fatalf("verified sigil catalog=%d, want broad 2.0.2 coverage", len(catalog))
	}
	var precise *NaturalDropSigilOption
	var native *NaturalDropSigilOption
	for index := range catalog {
		switch catalog[index].SigilHash {
		case "0xCE6C62CF":
			precise = &catalog[index]
		case "0x54D8EA04":
			native = &catalog[index]
		}
	}
	if precise == nil || precise.NativeTransmarvel {
		t.Fatalf("Precise Wrath V+ must be a verified add-to-pool option: %+v", precise)
	}
	if precise.NameZh != "怒发冲冠 V+" || len(precise.SecondaryTraits) == 0 {
		t.Fatalf("Precise Wrath V+ metadata=%+v", precise)
	}
	if native == nil || !native.NativeTransmarvel {
		t.Fatalf("Damage Cap V+ must remain a native Transmarvel option: %+v", native)
	}
}

func TestNaturalSigilPatchPinsOneLegitimateTransmarvelResult(t *testing.T) {
	sourceDir := localNaturalWrightstoneTableDir(t)
	shared, _, err := loadNaturalWrightstoneTables(sourceDir, true)
	if err != nil {
		t.Fatal(err)
	}
	sigilTables, _, err := loadNaturalSigilTables(sourceDir, true)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := buildNaturalSigilCatalog(shared.Lots, sigilTables.Gem)
	if err != nil {
		t.Fatal(err)
	}
	var target NaturalDropSigilOption
	for _, item := range catalog {
		if item.SigilHash == "0xCE6C62CF" {
			target = item
			break
		}
	}
	if target.SigilHash == "" || len(target.SecondaryTraits) == 0 {
		t.Fatal("test target is missing from the verified catalog")
	}
	originalLots := append([]byte(nil), shared.Lots...)
	originalRates := append([]byte(nil), shared.RateGroups...)
	originalGacha := append([]byte(nil), shared.Gacha...)
	originalGem := append([]byte(nil), sigilTables.Gem...)
	selection := NaturalDropSigilSelection{
		SigilHash:      target.SigilHash,
		SecondaryTrait: target.SecondaryTraits[0].Hash,
	}
	patchedShared, patchedSigils, count, err := patchNaturalSigilTables(shared, sigilTables, []NaturalDropSigilSelection{selection}, true)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("patched sigils=%d, want 1", count)
	}
	if !bytes.Equal(shared.Lots, originalLots) || !bytes.Equal(shared.RateGroups, originalRates) ||
		!bytes.Equal(shared.Gacha, originalGacha) || !bytes.Equal(sigilTables.Gem, originalGem) {
		t.Fatal("sigil patch mutated source tables")
	}
	targetHash, _ := ParseHashHex(target.SigilHash)
	traitHash, _ := ParseHashHex(selection.SecondaryTrait)
	foundLot := false
	lotCount, _ := tableRowCount(patchedShared.Lots, naturalWrightstoneLotRowSize)
	for index := 0; index < lotCount; index++ {
		offset := 8 + index*naturalWrightstoneLotRowSize
		pool := binary.LittleEndian.Uint32(patchedShared.Lots[offset+8:])
		if !naturalSigilPoolSet[pool] {
			continue
		}
		weight := binary.LittleEndian.Uint32(patchedShared.Lots[offset+16:])
		if weight > 0 {
			if foundLot || binary.LittleEndian.Uint32(patchedShared.Lots[offset+12:]) != targetHash {
				t.Fatal("Transmarvel sigil pool did not collapse to the selected factor")
			}
			foundLot = true
		}
	}
	if !foundLot {
		t.Fatal("selected sigil was not written into a Transmarvel pool")
	}
	gemOffset, ok := naturalSigilGemRowOffset(patchedSigils.Gem, targetHash)
	if !ok {
		t.Fatal("patched gem row is missing")
	}
	if binary.LittleEndian.Uint32(patchedSigils.Gem[gemOffset+4:]) != traitHash ||
		int32(binary.LittleEndian.Uint32(patchedSigils.Gem[gemOffset+36:])) != -1 {
		t.Fatal("selected secondary trait was not pinned in gem.tbl")
	}
	gachaOffset, ok := naturalSigilTransmarvelGachaOffset(patchedShared.Gacha)
	if !ok || binary.LittleEndian.Uint32(patchedShared.Gacha[gachaOffset:]) != 100 ||
		binary.LittleEndian.Uint32(patchedShared.Gacha[gachaOffset+4:]) != 0 {
		t.Fatal("Transmarvel was not switched to 100% sigils")
	}
}

func TestNaturalSigilRejectsTraitOutsideTheFactorPool(t *testing.T) {
	sourceDir := localNaturalWrightstoneTableDir(t)
	shared, _, err := loadNaturalWrightstoneTables(sourceDir, true)
	if err != nil {
		t.Fatal(err)
	}
	sigils, _, err := loadNaturalSigilTables(sourceDir, true)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = patchNaturalSigilTables(shared, sigils, []NaturalDropSigilSelection{{
		SigilHash: "0xCE6C62CF", SecondaryTrait: "0xDEADBEEF",
	}}, false)
	if err == nil {
		t.Fatal("an unknown trait was accepted as a sigil secondary trait")
	}
}
