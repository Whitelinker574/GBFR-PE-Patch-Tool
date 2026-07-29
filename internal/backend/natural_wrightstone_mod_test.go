package backend

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func localNaturalWrightstoneTableDir(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(filepath.Dir(root), "field-extracted", "system-tables-full-202", "system", "table")
	if _, err := os.Stat(filepath.Join(dir, "item_pendulum.tbl")); err != nil {
		t.Skipf("local 2.0.2 extracted wrightstone tables are unavailable: %v", err)
	}
	return dir
}

func naturalWrightstoneRow(t *testing.T, data []byte, key uint32) int {
	t.Helper()
	count, err := tableRowCount(data, naturalWrightstoneItemRowSize)
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < count; index++ {
		offset := 8 + index*naturalWrightstoneItemRowSize
		if binary.LittleEndian.Uint32(data[offset+32:]) == key {
			return offset
		}
	}
	t.Fatalf("item_pendulum.tbl is missing 0x%08X", key)
	return 0
}

func naturalWrightstoneLotWeight(t *testing.T, data []byte, pool, key uint32) uint32 {
	t.Helper()
	count, err := tableRowCount(data, naturalWrightstoneLotRowSize)
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < count; index++ {
		offset := 8 + index*naturalWrightstoneLotRowSize
		if binary.LittleEndian.Uint32(data[offset+8:]) == pool && binary.LittleEndian.Uint32(data[offset+12:]) == key {
			return binary.LittleEndian.Uint32(data[offset+16:])
		}
	}
	t.Fatalf("gacha_lot.tbl is missing pool 0x%08X item 0x%08X", pool, key)
	return 0
}

func naturalWrightstoneRateWeight(t *testing.T, data []byte, pool uint32) uint32 {
	t.Helper()
	count, err := tableRowCount(data, naturalWrightstoneRateRowSize)
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < count; index++ {
		offset := 8 + index*naturalWrightstoneRateRowSize
		if binary.LittleEndian.Uint32(data[offset:]) == naturalWrightstoneRateGroup && binary.LittleEndian.Uint32(data[offset+4:]) == pool {
			return binary.LittleEndian.Uint32(data[offset+8:])
		}
	}
	t.Fatalf("gacha_rate_group.tbl is missing pool 0x%08X", pool)
	return 0
}

func TestNaturalWrightstoneRealTablesPinLegitimateTransmarvelResult(t *testing.T) {
	source, statuses, err := loadNaturalWrightstoneTables(localNaturalWrightstoneTableDir(t), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 4 {
		t.Fatalf("validated tables=%d, want 4", len(statuses))
	}
	for _, status := range statuses {
		if !status.Valid {
			t.Fatalf("table did not pass exact 2.0.2 validation: %+v", status)
		}
	}
	catalog, err := buildNaturalWrightstoneCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) != 4 || len(catalog[0].SubTraits) < 60 {
		t.Fatalf("wrightstone catalog is incomplete: families=%d traits=%d", len(catalog), len(catalog[0].SubTraits))
	}
	dread := catalog[0]
	originalItems := append([]byte(nil), source.Items...)
	originalLots := append([]byte(nil), source.Lots...)
	originalRates := append([]byte(nil), source.RateGroups...)
	originalGacha := append([]byte(nil), source.Gacha...)
	patched, count, err := patchNaturalWrightstoneTables(source, []NaturalDropWrightstoneSelection{{
		MainTrait: dread.MainTrait.Hash,
		SubTrait1: dread.SubTraits[0].Hash,
		SubTrait2: dread.SubTraits[1].Hash,
	}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("patched variants=%d, want 1", count)
	}
	if !bytes.Equal(source.Items, originalItems) || !bytes.Equal(source.Lots, originalLots) || !bytes.Equal(source.RateGroups, originalRates) || !bytes.Equal(source.Gacha, originalGacha) {
		t.Fatal("wrightstone patch mutated source tables")
	}
	family := naturalWrightstoneFamilies[0]
	offset := naturalWrightstoneRow(t, patched.Items, family.Slots[0])
	main, _ := ParseHashHex(dread.MainTrait.Hash)
	sub1, _ := ParseHashHex(dread.SubTraits[0].Hash)
	sub2, _ := ParseHashHex(dread.SubTraits[1].Hash)
	if binary.LittleEndian.Uint32(patched.Items[offset+36:]) != main || binary.LittleEndian.Uint32(patched.Items[offset:]) != sub1 || binary.LittleEndian.Uint32(patched.Items[offset+4:]) != sub2 {
		t.Fatal("pinned wrightstone traits do not match the requested legitimate roll")
	}
	if binary.LittleEndian.Uint32(patched.Items[offset+40:]) != 20 || binary.LittleEndian.Uint32(patched.Items[offset+8:]) != 15 || binary.LittleEndian.Uint32(patched.Items[offset+12:]) != 10 {
		t.Fatal("pinned wrightstone levels are not 20/15/10")
	}
	if got := naturalWrightstoneLotWeight(t, patched.Lots, naturalWrightstonePools[0], family.Slots[0]); got != 50 {
		t.Fatalf("selected lot weight=%d, want 50", got)
	}
	if got := naturalWrightstoneLotWeight(t, patched.Lots, naturalWrightstonePools[0], naturalWrightstoneFamilies[1].Slots[0]); got != 0 {
		t.Fatalf("unselected lot weight=%d, want 0", got)
	}
	if got := naturalWrightstoneRateWeight(t, patched.RateGroups, naturalWrightstonePools[0]); got != 5000 {
		t.Fatalf("active pool weight=%d, want 5000", got)
	}
	for _, pool := range naturalWrightstonePools[1:] {
		if got := naturalWrightstoneRateWeight(t, patched.RateGroups, pool); got != 0 {
			t.Fatalf("inactive pool 0x%08X weight=%d, want 0", pool, got)
		}
	}
	gachaCount, _ := tableRowCount(patched.Gacha, naturalWrightstoneGachaRowSize)
	found := false
	for index := 0; index < gachaCount; index++ {
		offset := 8 + index*naturalWrightstoneGachaRowSize
		if binary.LittleEndian.Uint32(patched.Gacha[offset+8:]) == naturalWrightstoneGachaKey {
			found = true
			if binary.LittleEndian.Uint32(patched.Gacha[offset:]) != 0 || binary.LittleEndian.Uint32(patched.Gacha[offset+4:]) != 100 {
				t.Fatal("Transmarvel was not switched to 100% wrightstones")
			}
		}
	}
	if !found {
		t.Fatal("Transmarvel gacha row was not found")
	}
}

func TestNaturalWrightstoneRejectsMainTraitAsSubTrait(t *testing.T) {
	source, _, err := loadNaturalWrightstoneTables(localNaturalWrightstoneTableDir(t), true)
	if err != nil {
		t.Fatal(err)
	}
	main := "0xCEB700EE"
	_, _, err = patchNaturalWrightstoneTables(source, []NaturalDropWrightstoneSelection{{MainTrait: main, SubTrait1: main, SubTrait2: "0x50079A1C"}}, false)
	if err == nil {
		t.Fatal("main trait was accepted as a natural sub trait")
	}
}
