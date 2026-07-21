package main

import (
	"bytes"
	"path/filepath"
	"testing"
)

func firstAuditedSummonDraft(t *testing.T, save *SaveData) SummonTraitState {
	t.Helper()
	inventory, err := save.InspectSummonInventory()
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Records) == 0 {
		t.Fatal("fixture has no summon records")
	}
	return inventory.Records[0].State
}

func TestInspectSummonInventoryMatchesRealPreallocatedLayout(t *testing.T) {
	path := copyStatsSave(t)
	save, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := save.InspectSummonInventory()
	if err != nil {
		t.Fatal(err)
	}
	if !inventory.Unlocked || inventory.Capacity != 1000 || inventory.Occupied != 102 || inventory.MaxSlotID != 102 {
		t.Fatalf("summon inventory = %+v", inventory)
	}
	if len(inventory.Records) != inventory.Occupied || inventory.Records[0].UnitID != 0 || inventory.Records[0].SlotID != 1 {
		t.Fatalf("summon records are not stable and ordered: %+v", inventory.Records[:1])
	}
}

func TestCreateSummonUsesExactEmptyTemplateAndRegistersType(t *testing.T) {
	path := copyStatsSave(t)
	save, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	draft := firstAuditedSummonDraft(t, save)

	// Pick a catalog type that the fixture has not registered yet, proving the
	// create path updates 1453 instead of relying on an already-set flag.
	for _, item := range save.findAllUnitsByType(SummonCatalogIDType) {
		flag, ok := save.strictSummonUnit(SummonRegisteredIDType, item.UnitID, 1)
		if ok && flag.Uint32() == 0 {
			draft.TypeHash = item.Uint32()
			break
		}
	}
	record, err := save.CreateSummonRecord(draft)
	if err != nil {
		t.Fatal(err)
	}
	if record.UnitID != 102 || record.SlotID != 103 || record.State != draft {
		t.Fatalf("created summon = %+v", record)
	}
	max, _ := save.strictSummonUnit(SummonMaxSlotIDType, 0, 1)
	if max.Uint32() != 103 {
		t.Fatalf("1454 = %d, want 103", max.Uint32())
	}
	flag, err := save.summonRegistrationFlag(draft.TypeHash)
	if err != nil || flag.Uint32() != 1 {
		t.Fatalf("new summon type registration = %v err=%v", flag, err)
	}
	if err := save.VerifySummonRecord(record); err != nil {
		t.Fatal(err)
	}
}

func TestCreateSummonRejectsLockedDLCWithoutMutatingSave(t *testing.T) {
	path := copyStatsSave(t)
	save, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	draft := firstAuditedSummonDraft(t, save)
	unlocked, ok := save.strictSummonUnit(SummonUnlockedIDType, 0, 1)
	if !ok {
		t.Fatal("fixture lacks 1455")
	}
	unlocked.SetUint32(0)
	before := append([]byte(nil), save.data...)
	if _, err := save.CreateSummonRecordWithPolicy(draft, true); err == nil {
		t.Fatal("locked summon system accepted a created record")
	}
	if !bytes.Equal(before, save.data) {
		t.Fatal("failed create mutated a DLC-locked save")
	}
}

func TestCreateSummonDefaultsToWritingPreallocatedLockedDLCRecord(t *testing.T) {
	path := copyStatsSave(t)
	save, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	draft := firstAuditedSummonDraft(t, save)
	unlocked, ok := save.strictSummonUnit(SummonUnlockedIDType, 0, 1)
	if !ok {
		t.Fatal("fixture lacks summon unlock flag")
	}
	unlocked.SetUint32(0)
	record, err := save.CreateSummonRecord(draft)
	if err != nil {
		t.Fatalf("default write was blocked by advisory DLC availability: %v", err)
	}
	if record.SlotID == 0 || record.State != draft {
		t.Fatalf("forced record mismatch: %+v", record)
	}
	if err := save.VerifySummonRecord(record); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateSummonRecordCanChangeAuditedTypeAndKeepsSlotIdentity(t *testing.T) {
	path := copyStatsSave(t)
	save, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := save.InspectSummonInventory()
	if err != nil {
		t.Fatal(err)
	}
	existing := inventory.Records[0]
	draft := existing.State
	for _, item := range save.findAllUnitsByType(SummonCatalogIDType) {
		if item.Uint32() != existing.State.TypeHash {
			draft.TypeHash = item.Uint32()
			break
		}
	}
	updated, err := save.UpdateSummonRecord(existing, draft)
	if err != nil {
		t.Fatal(err)
	}
	if updated.UnitID != existing.UnitID || updated.SlotID != existing.SlotID || updated.State != draft {
		t.Fatalf("updated summon identity/state mismatch: %+v", updated)
	}
	if err := save.VerifySummonRecord(updated); err != nil {
		t.Fatal(err)
	}
}

func TestSummonSaveGeneratorWritesReopensAndVerifies(t *testing.T) {
	path := copyStatsSave(t)
	save, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	draft := firstAuditedSummonDraft(t, save)
	gen := NewSummonSaveGen()
	if _, err := gen.LoadSaveFile(path); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "SaveData2_summons.dat")
	result, err := gen.Apply(SummonSaveWriteRequest{Operation: "create", Draft: draft}, output)
	if err != nil {
		t.Fatal(err)
	}
	if result.OutputPath == "" || result.Inventory.Occupied != 103 || result.Record.SlotID != 103 {
		t.Fatalf("summon save apply result = %+v", result)
	}
	verify, err := LoadSave(output)
	if err != nil {
		t.Fatal(err)
	}
	if err := verify.VerifySummonRecord(result.Record); err != nil {
		t.Fatal(err)
	}
}
