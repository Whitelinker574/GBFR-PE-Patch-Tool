package backend

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnhancementResetNativeAcquisitionMask(t *testing.T) {
	for _, tc := range []struct{ before, all, granted uint32 }{
		{0x0307, 0x0300, 0x0303}, // Issue #26: one paid sigil slot, two granted slots.
		{0xFFFF, 0xFF00, 0xFFFF},
		{0xA57E0307, 0xA57E0300, 0xA57E0303}, // preserve unknown upper bits.
		{0x0300, 0x0300, 0x0303},             // restoring grants after an all-unlearn operation.
		{0, 0, 0},
	} {
		for mode, want := range map[string]uint32{"all": tc.all, "granted": tc.granted} {
			got, err := enhancementResetValue(tc.before, mode)
			if err != nil || got != want {
				t.Fatalf("%X %s => %X, want %X: %v", tc.before, mode, got, want, err)
			}
		}
	}
	if _, err := enhancementResetValue(0x0307, ""); err == nil {
		t.Fatal("implicit mode accepted")
	}
}

func enhancementUnitsFixture() *SaveData {
	s := &SaveData{data: make([]byte, 32), unitsByType: make(map[uint32][]*unitEntry)}
	add := func(kind, unit, value uint32, off int) {
		binary.LittleEndian.PutUint32(s.data[off:], value)
		s.unitsByType[kind] = append(s.unitsByType[kind], &unitEntry{IDType: kind, UnitID: unit, ValueCnt: 1, ValueOff: off, data: s.data})
	}
	add(1301, 10038, 0x25D46F4B, 0)
	add(1601, 10038029, 0x4FEE00FA, 4)
	add(1602, 10038029, 0x0307, 8)
	add(1601, 10039029, 0x4FEE00FA, 12)
	add(1602, 10039029, 0xFFFF, 16)
	return s
}

func TestEnhancementResetScopesAndMalformedPairs(t *testing.T) {
	s := enhancementUnitsFixture()
	entries, err := characterEnhancementEntries(s, 0x25D46F4B)
	if err != nil || len(entries) != 1 || entries[0].UnitID != 10038029 {
		t.Fatalf("scope: %v %+v", err, entries)
	}
	if _, err := characterEnhancementEntries(s, 0xDEADBEEF); err == nil {
		t.Fatal("unknown character accepted")
	}
	for _, kind := range []string{"duplicate-hash", "missing-hash", "vector-flags", "duplicate-flags", "missing-flags", "duplicate-character"} {
		t.Run(kind, func(t *testing.T) {
			s := enhancementUnitsFixture()
			switch kind {
			case "duplicate-hash":
				s.unitsByType[1601] = append(s.unitsByType[1601], s.unitsByType[1601][0])
			case "missing-hash":
				s.unitsByType[1601] = s.unitsByType[1601][1:]
			case "vector-flags":
				s.unitsByType[1602][0].ValueCnt = 2
			case "duplicate-flags":
				s.unitsByType[1602] = append(s.unitsByType[1602], s.unitsByType[1602][0])
			case "missing-flags":
				s.unitsByType[1602] = s.unitsByType[1602][1:]
			case "duplicate-character":
				e := *s.unitsByType[1301][0]
				e.UnitID = 10039
				s.unitsByType[1301] = append(s.unitsByType[1301], &e)
			}
			if _, err := characterEnhancementEntries(s, 0x25D46F4B); err == nil {
				t.Fatal("malformed scope accepted")
			}
		})
	}
}

func TestUnlearnedEnhancementCannotRetainFullLegacyStatBonuses(t *testing.T) {
	save := enhancementUnitsFixture()
	save.unitsByType[1602][0].SetUint32(0x0300)
	old := LoadoutLegacyMasteryGrowth{OwnerCode: "PL2500", SaveProgress: 9999999, Complete: true,
		Total: LoadoutPermanentPanelStats{Attack: 8582, HP: 55700}}
	got := clearUnlearnedLegacyMastery(save, 0x25D46F4B, old)
	if got.Total != (LoadoutPermanentPanelStats{}) || got.Complete || got.SaveProgress != old.SaveProgress || got.Evidence != "save-unlearned-enhancement-mask" {
		t.Fatalf("reset still presents full enhancement bonuses: %+v", got)
	}
}

func TestEnhancementResetRealSaveTransaction(t *testing.T) {
	source := os.Getenv("GBFR_ISSUE26_SAMPLE_DIR")
	if source == "" {
		t.Skip("set GBFR_ISSUE26_SAMPLE_DIR for copy-only author-sample regression")
	}
	original, err := os.ReadFile(filepath.Join(source, "SaveData2 B.dat"))
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"granted", "all"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "EnhancementCopy.dat")
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			snapshot, err := (&App{}).CharacterEnhancementInspect(path)
			if err != nil {
				t.Fatal(err)
			}
			request := CharacterEnhancementResetRequest{Path: path, ExpectedRevision: snapshot.Revision, CharacterHash: "25D46F4B", Mode: mode}
			result, err := resetCharacterEnhancementLocked(request, LoadSave)
			if err != nil {
				t.Fatal(err)
			}
			wantChanged := 1
			if mode == "all" {
				wantChanged = 117
			}
			if result.Changed != wantChanged || result.Verified != 400 || result.BackupPath == "" {
				t.Fatalf("result %+v", result)
			}
			backup, err := os.ReadFile(result.BackupPath)
			if err != nil || !bytes.Equal(backup, original) {
				t.Fatalf("backup mismatch %v", err)
			}
			after, err := LoadSave(path)
			if err != nil {
				t.Fatal(err)
			}
			before, err := newSaveData(path, append([]byte(nil), original...))
			if err != nil {
				t.Fatal(err)
			}
			// Every serialized value outside the selected character's low byte must survive.
			parsedBefore, err := ParseSaveData(original)
			if err != nil {
				t.Fatal(err)
			}
			parsedAfter, err := ParseSaveData(after.data)
			if err != nil {
				t.Fatal(err)
			}
			rowsBefore, rowsAfter := flattenSaveRecords(parsedBefore), flattenSaveRecords(parsedAfter)
			if len(rowsBefore) != len(rowsAfter) {
				t.Fatal("record count changed")
			}
			for i, row := range rowsBefore {
				other := rowsAfter[i]
				if row.idType == SaveID_HashSeed {
					continue
				}
				if row.idType == 1602 && row.unitID >= 10038000 && row.unitID < 10039000 {
					continue
				}
				if row.key != other.key || row.hash != other.hash {
					t.Fatalf("unexpected field change %d/%d", row.idType, row.unitID)
				}
			}
			afterNodes := entriesByUnitID(after.findAllUnitsByType(1602))
			for _, old := range before.findAllUnitsByType(1602) {
				entry := afterNodes[old.UnitID]
				if entry == nil {
					t.Fatal("lost node")
				}
				want := old.Uint32()
				if old.UnitID >= 10038000 && old.UnitID < 10039000 {
					want, _ = enhancementResetValue(want, mode)
				}
				if entry.Uint32() != want {
					t.Fatalf("node %d: %X != %X", old.UnitID, entry.Uint32(), want)
				}
			}
			request.ExpectedRevision = result.Revision
			again, err := resetCharacterEnhancementLocked(request, LoadSave)
			if err != nil || again.Changed != 0 || again.BackupPath != "" {
				t.Fatalf("not idempotent %+v %v", again, err)
			}
			request.ExpectedRevision = strings.Repeat("0", 64)
			if _, err := resetCharacterEnhancementLocked(request, LoadSave); err == nil {
				t.Fatal("stale revision accepted")
			}
		})
	}
	path := filepath.Join(t.TempDir(), "RollbackCopy.dat")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	request := CharacterEnhancementResetRequest{Path: path, ExpectedRevision: fateEpisodeRevision(original), CharacterHash: "25D46F4B", Mode: "all"}
	if _, err := resetCharacterEnhancementLocked(request, func(string) (*SaveData, error) { return nil, errors.New("injected readback failure") }); err == nil {
		t.Fatal("readback failure ignored")
	}
	restored, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(restored, original) {
		t.Fatal("rollback failed")
	}
}
