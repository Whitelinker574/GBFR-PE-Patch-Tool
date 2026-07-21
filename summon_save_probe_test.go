package main

import (
	"os"
	"sort"
	"testing"
)

// Manual read-only field probe. It is skipped in normal test runs and never
// writes the supplied save.
func TestProbeSummonSaveInventory(t *testing.T) {
	path := os.Getenv("GBFR_SUMMON_PROBE_SAVE")
	if path == "" {
		t.Skip("GBFR_SUMMON_PROBE_SAVE is not set")
	}
	save, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	for idType := uint32(1451); idType <= 1460; idType++ {
		entries := save.findAllUnitsByType(idType)
		sort.Slice(entries, func(i, j int) bool { return entries[i].UnitID < entries[j].UnitID })
		empty := 0
		for _, entry := range entries {
			if entry.ValueCnt > 0 && entry.Uint32() == EmptyHash {
				empty++
			}
		}
		if len(entries) == 0 {
			t.Logf("IDType %d: absent", idType)
			continue
		}
		t.Logf("IDType %d: count=%d unit=%d..%d valueCnt(first)=%d emptyFirst=%d", idType, len(entries), entries[0].UnitID, entries[len(entries)-1].UnitID, entries[0].ValueCnt, empty)
		limit := len(entries)
		if limit > 6 {
			limit = 6
		}
		for _, entry := range entries[:limit] {
			values := make([]uint32, entry.ValueCnt)
			for index := range values {
				values[index], _ = entry.Uint32At(index)
			}
			t.Logf("  unit=%d values=%08X", entry.UnitID, values)
		}
	}
	types := make(map[uint32]bool)
	for _, entry := range save.findAllUnitsByType(1457) {
		if entry.UnitID < 1000 && entry.Uint32() != EmptyHash {
			types[entry.Uint32()] = true
		}
	}
	registered, missing, flaggedWithoutInstance := 0, 0, 0
	for _, entry := range save.findAllUnitsByType(1452) {
		flag, ok := save.findUnitExact(1453, entry.UnitID)
		if !ok {
			continue
		}
		if types[entry.Uint32()] {
			if flag.Uint32() != 0 {
				registered++
			} else {
				missing++
			}
		} else if flag.Uint32() != 0 {
			flaggedWithoutInstance++
		}
	}
	t.Logf("registration: unique inventory types=%d registered=%d missing=%d flaggedWithoutInstance=%d", len(types), registered, missing, flaggedWithoutInstance)
}
