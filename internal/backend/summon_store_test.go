package backend

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
	if !inventory.Unlocked || inventory.Capacity != SummonSaveCapacity || inventory.Occupied == 0 || inventory.MaxSlotID < uint32(inventory.Occupied) {
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
	before, err := save.InspectSummonInventory()
	if err != nil {
		t.Fatal(err)
	}
	usedUnits := make(map[uint32]bool, len(before.Records))
	for _, item := range before.Records {
		usedUnits[item.UnitID] = true
	}
	wantUnitID := uint32(0)
	for usedUnits[wantUnitID] {
		wantUnitID++
	}

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
	if record.UnitID != wantUnitID || record.SlotID != before.MaxSlotID+1 || record.State != draft {
		t.Fatalf("created summon = %+v", record)
	}
	max, _ := save.strictSummonUnit(SummonMaxSlotIDType, 0, 1)
	if max.Uint32() != record.SlotID {
		t.Fatalf("1454 = %d, want %d", max.Uint32(), record.SlotID)
	}
	flags, err := save.summonRegistrationFlags(draft.TypeHash)
	if err != nil || len(flags) == 0 {
		t.Fatalf("new summon type registrations = %v err=%v", flags, err)
	}
	for _, flag := range flags {
		if flag.Uint32() != 1 {
			t.Fatalf("new summon type registration was not synchronized: %v", flags)
		}
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

func TestUpdateSummonRecordRebuildsTypeWithFreshSlotAndEquippedReference(t *testing.T) {
	path := copyStatsSave(t)
	save, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := save.InspectSummonInventory()
	if err != nil {
		t.Fatal(err)
	}
	equipped, err := save.requireStrictSummonUnit(SummonEquippedIDType, 0, 4)
	if err != nil {
		t.Fatal(err)
	}
	equippedSlot, _ := equipped.Uint32At(0)
	var existing SummonSaveRecord
	for _, record := range inventory.Records {
		if record.SlotID == equippedSlot {
			existing = record
			break
		}
	}
	if existing.SlotID == 0 {
		t.Fatalf("fixture equipped summon SlotID %d has no inventory record", equippedSlot)
	}
	draft := existing.State
	const preferredReplacement = uint32(0xAE52556D) // 2.0.2 解包表中的强袭型机械武装·主机
	for _, item := range save.findAllUnitsByType(SummonCatalogIDType) {
		if item.Uint32() == preferredReplacement {
			draft.TypeHash = item.Uint32()
			break
		}
	}
	if draft.TypeHash == existing.State.TypeHash {
		for _, item := range save.findAllUnitsByType(SummonCatalogIDType) {
			if item.Uint32() == existing.State.TypeHash {
				continue
			}
			if _, registrationErr := save.summonRegistrationFlags(item.Uint32()); registrationErr == nil {
				draft.TypeHash = item.Uint32()
				break
			}
		}
	}
	if draft.TypeHash == existing.State.TypeHash {
		t.Fatal("fixture has no uniquely registered replacement summon type")
	}
	updated, err := save.UpdateSummonRecord(existing, draft)
	if err != nil {
		t.Fatal(err)
	}
	if updated.UnitID != existing.UnitID || updated.SlotID != inventory.MaxSlotID+1 || updated.State != draft {
		t.Fatalf("type replacement did not create a fresh item identity: got=%+v old=%+v max=%d", updated, existing, inventory.MaxSlotID)
	}
	remapped, _ := equipped.Uint32At(0)
	if remapped != updated.SlotID {
		t.Fatalf("equipped reference was not remapped: got %d want %d", remapped, updated.SlotID)
	}
	if err := save.VerifySummonRecord(updated); err != nil {
		t.Fatal(err)
	}
	after, err := save.InspectSummonInventory()
	if err != nil {
		t.Fatal(err)
	}
	if after.Occupied != inventory.Occupied || after.MaxSlotID != updated.SlotID {
		t.Fatalf("replacement changed inventory cardinality or max slot: before=%+v after=%+v", inventory, after)
	}
	for _, record := range after.Records {
		if record.SlotID == existing.SlotID {
			t.Fatalf("old item identity %d survived replacement", existing.SlotID)
		}
	}
}

func TestSummonSaveGeneratorWritesReopensAndVerifies(t *testing.T) {
	path := copyStatsSave(t)
	save, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	draft := firstAuditedSummonDraft(t, save)
	before, err := save.InspectSummonInventory()
	if err != nil {
		t.Fatal(err)
	}
	gen := NewSummonSaveGen()
	if _, err := gen.LoadSaveFile(path); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "SaveData2_summons.dat")
	result, err := gen.Apply(SummonSaveWriteRequest{Operation: "create", Draft: draft}, output)
	if err != nil {
		t.Fatal(err)
	}
	if result.OutputPath == "" || result.Inventory.Occupied != before.Occupied+1 || result.Record.SlotID != before.MaxSlotID+1 {
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

func TestSummonSaveGeneratorPersistsTypeReplacementAfterReload(t *testing.T) {
	path := copyStatsSave(t)
	gen := NewSummonSaveGen()
	info, err := gen.LoadSaveFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Inventory.Records) == 0 {
		t.Fatal("fixture has no summon to replace")
	}
	existing := info.Inventory.Records[0]
	draft := existing.State
	for _, summonType := range []uint32{0xAE52556D, 0x81ECEC7F, 0x6E5968FC} {
		if summonType != existing.State.TypeHash {
			draft.TypeHash = summonType
			break
		}
	}
	output := filepath.Join(t.TempDir(), "SaveData2_type-replaced.dat")
	result, err := gen.Apply(SummonSaveWriteRequest{Operation: "update", Expected: &existing, Draft: draft}, output)
	if err != nil {
		t.Fatal(err)
	}
	if result.Record.UnitID != existing.UnitID || result.Record.SlotID != info.Inventory.MaxSlotID+1 || result.Record.State.TypeHash != draft.TypeHash {
		t.Fatalf("type replacement result mismatch: %+v", result.Record)
	}
	reopened, err := LoadSave(output)
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.VerifySummonRecord(result.Record); err != nil {
		t.Fatalf("replacement did not survive disk reopen: %v", err)
	}
}
