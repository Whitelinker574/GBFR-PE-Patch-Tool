package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNonNegativeInlineValueRejectsUint32Overflow(t *testing.T) {
	if ^uint(0) <= uint(^uint32(0)) {
		t.Skip("test requires a 64-bit int")
	}
	overflow := int(uint64(^uint32(0)) + 1)
	if _, err := nonNegativeInlineValue("summon level", overflow); err == nil || !strings.Contains(err.Error(), "large") {
		t.Fatalf("uint32 overflow must fail before conversion, got: %v", err)
	}
}

type inlineResourceFixture struct {
	write          LoadoutWrite
	weapon         LoadoutWeaponInlineEdit
	summon         LoadoutSummonInlineEdit
	weaponPrefix   []uint32
	summonSnapshot map[uint32][]uint32
}

func setupInlineResourceFixture(t *testing.T, path string) inlineResourceFixture {
	t.Helper()
	loadout := firstIoLoadout(t, path)
	if loadout.WeaponSlotID == 0 {
		t.Skip("real loadout fixture has no selected weapon")
	}
	save, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	index := buildLoadoutIndex(save)
	weaponUnitID, ok := index.wepBySlotID[loadout.WeaponSlotID]
	if !ok {
		t.Fatalf("selected weapon SlotID %d has no owned instance", loadout.WeaponSlotID)
	}
	weaponHashEntry, hashOK := save.findUnitExact(weaponIDType, weaponUnitID)
	transcendence, transOK := save.findUnitExact(weaponTranscendenceIDType, weaponUnitID)
	extra, extraOK := save.findUnitExact(weaponExtraIDType, weaponUnitID)
	if !hashOK || !transOK || !extraOK || extra.ValueCnt < 5 {
		t.Fatalf("weapon UnitID %d lacks required inline fields", weaponUnitID)
	}
	transcendence.SetInt32(7)
	if err := extra.SetUint32At(4, 0xBBD77C33); err != nil {
		t.Fatal(err)
	}
	if err := save.FixChecksums(); err != nil {
		t.Fatal(err)
	}
	if err := save.Write(path); err != nil {
		t.Fatal(err)
	}

	weaponPrefix := make([]uint32, 4)
	for i := range weaponPrefix {
		weaponPrefix[i], _ = extra.Uint32At(i)
	}
	weaponEdit := LoadoutWeaponInlineEdit{
		SlotID: loadout.WeaponSlotID, ExpectUnitID: weaponUnitID,
		ExpectStoredHash: hashText(weaponHashEntry.Uint32()), ExpectTranscendence: 7,
		ExpectTranscendenceSkill: "BBD77C33",
		TranscendenceSkill:       "020DB733",
	}

	stats, err := (&App{}).LoadoutStatContext(path, testIoHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats.EquippedSummons) != 4 {
		t.Fatalf("fixture needs four equipped summons: %+v", stats.EquippedSummons)
	}
	target := stats.EquippedSummons[0]
	catalog, err := loadSummonStatCatalog()
	if err != nil {
		t.Fatal(err)
	}
	newSubHash := uint32(0)
	for hash, option := range catalog.sub {
		if hash != mustParseTestHash(t, target.SubParamHash) && option.MaxLevel >= 0 && option.MaxLevel < len(option.Values) {
			newSubHash = hash
			break
		}
	}
	if newSubHash == 0 {
		t.Fatal("summon catalog has no alternate audited sub parameter")
	}
	newRank := (target.Rank + 1) % 4
	summonEdit := LoadoutSummonInlineEdit{
		SlotID: target.SlotID, ExpectUnitID: target.UnitID, ExpectTypeHash: target.TypeHash,
		ExpectMainTraitHash: target.MainTraitHash, ExpectMainTraitLevel: target.MainTraitLevel,
		ExpectSubParamHash: target.SubParamHash, ExpectSubParamLevel: target.SubParamLevel,
		ExpectRank:    target.Rank,
		MainTraitHash: target.MainTraitHash, MainTraitLevel: target.MainTraitLevel,
		SubParamHash: hashText(newSubHash), SubParamLevel: 0, Rank: newRank,
	}
	write := loadoutWriteFromEntry(loadout, append([]uint32(nil), stats.EquippedSummonSlotIDs...))
	return inlineResourceFixture{
		write: write, weapon: weaponEdit, summon: summonEdit,
		weaponPrefix: weaponPrefix, summonSnapshot: summonInstanceSnapshot(t, save),
	}
}

func mustParseTestHash(t *testing.T, value string) uint32 {
	t.Helper()
	hash, err := ParseHashHex(value)
	if err != nil {
		t.Fatal(err)
	}
	return hash
}

func TestLoadoutApplyWithResourcesRejectsStaleWeaponSnapshotBeforeWrite(t *testing.T) {
	input := copyStatsSave(t)
	fixture := setupInlineResourceFixture(t, input)
	output := filepath.Join(t.TempDir(), "stale-weapon.dat")
	fixture.weapon.ExpectStoredHash = "DEADBEEF"
	_, err := (&App{}).LoadoutApplyWithResources(input, output, LoadoutApplyRequest{
		Changes: []LoadoutWrite{fixture.write}, WeaponEdits: []LoadoutWeaponInlineEdit{fixture.weapon},
	})
	if err == nil || !strings.Contains(err.Error(), "stale weapon") {
		t.Fatalf("stale weapon snapshot must fail before write, got: %v", err)
	}
	if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
		t.Fatalf("stale request created output: %v", statErr)
	}
}

func TestLoadoutApplyWithResourcesRejectsStaleWeaponEffectBeforeWrite(t *testing.T) {
	input := copyStatsSave(t)
	fixture := setupInlineResourceFixture(t, input)
	output := filepath.Join(t.TempDir(), "stale-weapon-effect.dat")
	fixture.weapon.ExpectTranscendenceSkill = "79027FC8"
	_, err := (&App{}).LoadoutApplyWithResources(input, output, LoadoutApplyRequest{
		Changes: []LoadoutWrite{fixture.write}, WeaponEdits: []LoadoutWeaponInlineEdit{fixture.weapon},
	})
	if err == nil || !strings.Contains(err.Error(), "stale weapon") {
		t.Fatalf("stale weapon effect must fail before write, got: %v", err)
	}
	if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
		t.Fatalf("stale request created output: %v", statErr)
	}
}

func TestLoadoutApplyWithResourcesRejectsConflictingWeaponEditsBeforeWrite(t *testing.T) {
	input := copyStatsSave(t)
	fixture := setupInlineResourceFixture(t, input)
	conflict := fixture.weapon
	conflict.TranscendenceSkill = "3F682593"
	output := filepath.Join(t.TempDir(), "conflicting-weapon.dat")
	_, err := (&App{}).LoadoutApplyWithResources(input, output, LoadoutApplyRequest{
		Changes:     []LoadoutWrite{fixture.write},
		WeaponEdits: []LoadoutWeaponInlineEdit{fixture.weapon, fixture.weapon, conflict},
	})
	if err == nil || !strings.Contains(err.Error(), "conflicting weapon") {
		t.Fatalf("conflicting edits to one weapon must fail, got: %v", err)
	}
	if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
		t.Fatalf("conflicting request created output: %v", statErr)
	}
}

func TestLoadoutApplyWithResourcesRejectsStaleSummonSnapshotBeforeWrite(t *testing.T) {
	input := copyStatsSave(t)
	fixture := setupInlineResourceFixture(t, input)
	output := filepath.Join(t.TempDir(), "stale-summon.dat")
	fixture.summon.ExpectRank = (fixture.summon.ExpectRank + 1) % 4
	_, err := (&App{}).LoadoutApplyWithResources(input, output, LoadoutApplyRequest{
		Changes: []LoadoutWrite{fixture.write}, SummonEdits: []LoadoutSummonInlineEdit{fixture.summon},
	})
	if err == nil || !strings.Contains(err.Error(), "stale summon") {
		t.Fatalf("stale summon snapshot must fail before write, got: %v", err)
	}
	if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
		t.Fatalf("stale request created output: %v", statErr)
	}
}

func TestLoadoutApplyWithResourcesWritesWeaponAndSummonInOneVerifiedTransaction(t *testing.T) {
	input := copyStatsSave(t)
	fixture := setupInlineResourceFixture(t, input)
	inputBefore, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "inline-resources.dat")
	result, err := (&App{}).LoadoutApplyWithResources(input, output, LoadoutApplyRequest{
		Changes:     []LoadoutWrite{fixture.write},
		WeaponEdits: []LoadoutWeaponInlineEdit{fixture.weapon, fixture.weapon},
		SummonEdits: []LoadoutSummonInlineEdit{fixture.summon, fixture.summon},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.VerifiedFields < 9 {
		t.Fatalf("inline resources were not included in strict readback: %+v", result)
	}
	inputAfter, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(inputBefore, inputAfter) {
		t.Fatal("separate-output transaction modified its input save")
	}
	after, err := LoadSave(output)
	if err != nil {
		t.Fatal(err)
	}
	extra, ok := after.findUnitExact(weaponExtraIDType, fixture.weapon.ExpectUnitID)
	if !ok || extra.ValueCnt < 5 {
		t.Fatal("written weapon lost its 2818 vector")
	}
	for index, want := range fixture.weaponPrefix {
		got, _ := extra.Uint32At(index)
		if got != want {
			t.Fatalf("weapon 2818[%d] changed from %08X to %08X", index, want, got)
		}
	}
	if got, _ := extra.Uint32At(4); got != 0x020DB733 {
		t.Fatalf("weapon 2818[4]=%08X, want 020DB733", got)
	}

	afterSummons := summonInstanceSnapshot(t, after)
	for slotID, before := range fixture.summonSnapshot {
		afterState := afterSummons[slotID]
		if slotID != fixture.summon.SlotID {
			if !reflect.DeepEqual(afterState, before) {
				t.Fatalf("unrelated summon SlotID %d was changed", slotID)
			}
			continue
		}
		want := append([]uint32(nil), before...)
		want[3] = mustParseTestHash(t, fixture.summon.SubParamHash)
		want[4] = uint32(fixture.summon.MainTraitLevel)
		want[5] = uint32(fixture.summon.SubParamLevel)
		want[6] = uint32(fixture.summon.Rank)
		if !reflect.DeepEqual(afterState, want) {
			t.Fatalf("target summon writeback=%v, want %v", afterState, want)
		}
	}
}
