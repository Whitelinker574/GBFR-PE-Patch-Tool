package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"strings"
	"testing"
)

func TestOverLimitCatalogIsSharedByRuntimeAndSaveViews(t *testing.T) {
	if got, want := len(overLimitCatalog), 23; got != want {
		t.Fatalf("超限能力目录条目数=%d，期望 11 个标准 hash + 12 个高阶 alias=%d", got, want)
	}
	if got, want := len(overLimitAttributeOptions), len(overLimitCatalog); got != want {
		t.Fatalf("运行时选项=%d，统一目录=%d；运行时仍有目录漂移", got, want)
	}
	if got, want := len(overLimitDefinitions), len(overLimitCatalog); got != want {
		t.Fatalf("存档计算目录=%d，统一目录=%d；存档读取仍有目录漂移", got, want)
	}
	for hash, entry := range overLimitCatalog {
		if !validOverLimitAttribute(hash) {
			t.Errorf("统一目录 hash %08X 没有被运行时写入校验接受", hash)
		}
		definition, ok := overLimitDefinitions[hash]
		if !ok {
			t.Errorf("统一目录 hash %08X 没有被存档计算目录接受", hash)
			continue
		}
		if definition.name != entry.name || definition.unit != entry.unit || definition.values != entry.values {
			t.Errorf("hash %08X 的运行时/存档定义不一致: runtime=%+v save=%+v", hash, entry, definition)
		}
	}
}

func TestSaveOverLimitReaderKeepsKnownAliasAndIgnoresUnknownHash(t *testing.T) {
	const base = uint32(10000000)
	data := &SaveDataBinary{}
	attributes := []uint32{0xCB63BE55, 0xDEADBEEF, 0, 0}
	levels := []int32{0x200, 1, 0, 0}
	for index := 0; index < overLimitSlotCount; index++ {
		unitID := base + uint32(index)
		data.UIntTable = append(data.UIntTable, UIntSaveDataUnit{IDType: 1606, UnitID: unitID, ValueData: []uint32{attributes[index]}})
		data.IntTable = append(data.IntTable, IntSaveDataUnit{IDType: 1607, UnitID: unitID, ValueData: []int32{levels[index]}})
	}
	warnings := []string{}
	bonuses, err := readOverLimit(data, 10000, &warnings)
	if err != nil {
		t.Fatal(err)
	}
	if len(bonuses) != 1 || bonuses[0].AttributeHash != "CB63BE55" || bonuses[0].Name != "攻击力" {
		t.Fatalf("已审计 alias 没有按原 hash 读取: %+v", bonuses)
	}
	if len(warnings) == 0 || !strings.Contains(strings.Join(warnings, "\n"), "DEADBEEF") {
		t.Fatalf("未知 hash 应被忽略并明确告警，实际: %v", warnings)
	}
}

func TestPrepareOverLimitBatchRejectsUnknownAndPreservesKnownAliasHash(t *testing.T) {
	updates := []OverLimitUpdate{
		{Index: 0, Attribute: 0xCB63BE55, Level: 0x200, Value: 1000},
		{Index: 1, Attribute: 0x52A207B5, Level: 0x200, Value: 2000},
		{Index: 2, Attribute: 0x45C65767, Level: 0x200, Value: 20},
		{Index: 3, Attribute: 0x6CB38EF3, Level: 0x200, Value: 2},
	}
	encoded, err := prepareOverLimitBatch(updates)
	if err != nil {
		t.Fatal(err)
	}
	if got := binary.LittleEndian.Uint32(encoded[0][0:4]); got != updates[0].Attribute {
		t.Fatalf("未编辑的高阶 alias 被改写为 %08X，期望原样保留 %08X", got, updates[0].Attribute)
	}

	unknown := append([]OverLimitUpdate(nil), updates...)
	unknown[2].Attribute = 0xDEADBEEF
	if _, err := prepareOverLimitBatch(unknown); err == nil || !strings.Contains(err.Error(), "不在已审计目录") {
		t.Fatalf("未知 hash 必须 fail closed，实际错误: %v", err)
	}
}

func TestPrepareOverLimitBatchValidatesAllFourSlotsBeforeWriting(t *testing.T) {
	valid := []OverLimitUpdate{
		{Index: 0, Attribute: 0xC4925BD7, Level: 1, Value: 100},
		{Index: 1, Attribute: 0x52A207B5, Level: 2, Value: 200},
		{Index: 2, Attribute: 0x45C65767, Level: 4, Value: 2},
		{Index: 3, Attribute: 0x6CB38EF3, Level: 8, Value: 0.4},
	}
	for name, mutate := range map[string]func([]OverLimitUpdate){
		"少于四槽": func(v []OverLimitUpdate) { v[3].Index = 2 },
		"重复槽位": func(v []OverLimitUpdate) { v[1].Index = 0 },
		"非法等级": func(v []OverLimitUpdate) { v[1].Level = 3 },
		"超过上限": func(v []OverLimitUpdate) { v[2].Value = 21 },
		"非正数":  func(v []OverLimitUpdate) { v[2].Value = 0 },
		"非有限值": func(v []OverLimitUpdate) { v[2].Value = float32(math.NaN()) },
	} {
		t.Run(name, func(t *testing.T) {
			updates := append([]OverLimitUpdate(nil), valid...)
			mutate(updates)
			if _, err := prepareOverLimitBatch(updates); err == nil {
				t.Fatal("非法四槽批量输入应在任何写入前被拒绝")
			}
		})
	}
	if _, err := prepareOverLimitBatch(valid[:3]); err == nil {
		t.Fatal("批量接口必须恰好接收四槽")
	}
}

func TestValidateOverLimitBatchTargetRequiresCurrentCompleteResultScreen(t *testing.T) {
	complete := OverLimitStatus{Found: true, Hooked: true, SelectedAddr: 0x1234, Slots: []OverLimitSlot{
		{Index: 0}, {Index: 1}, {Index: 2}, {Index: 3},
	}}
	if err := validateOverLimitBatchTarget(complete); err != nil {
		t.Fatalf("完整的当前结果界面被拒绝: %v", err)
	}
	for name, mutate := range map[string]func(*OverLimitStatus){
		"未挂钩":  func(v *OverLimitStatus) { v.Hooked = false },
		"地址为空": func(v *OverLimitStatus) { v.SelectedAddr = 0 },
		"槽位缺失": func(v *OverLimitStatus) { v.Slots = v.Slots[:3] },
		"槽序异常": func(v *OverLimitStatus) { v.Slots[2].Index = 3 },
	} {
		t.Run(name, func(t *testing.T) {
			status := complete
			status.Slots = append([]OverLimitSlot(nil), complete.Slots...)
			mutate(&status)
			if err := validateOverLimitBatchTarget(status); err == nil {
				t.Fatal("非当前/不完整结果界面必须在快照和写入前被拒绝")
			}
		})
	}
}

func TestWriteOverLimitBatchAtomicRollsBackEarlierSlotsWhenLaterWriteFails(t *testing.T) {
	base := uintptr(0x1000)
	oldSnapshot := make([]byte, overLimitSlotCount*int(overLimitSlotStride))
	for i := range oldSnapshot {
		oldSnapshot[i] = byte(i + 1)
	}
	memory := append([]byte(nil), oldSnapshot...)
	updates := []OverLimitUpdate{
		{Index: 0, Attribute: 0xC4925BD7, Level: 1, Value: 100},
		{Index: 1, Attribute: 0x52A207B5, Level: 2, Value: 200},
		{Index: 2, Attribute: 0x45C65767, Level: 4, Value: 2},
		{Index: 3, Attribute: 0x6CB38EF3, Level: 8, Value: 0.4},
	}
	encoded, err := prepareOverLimitBatch(updates)
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("injected third-slot failure")
	writer := func(addr uintptr, data []byte) error {
		offset := int(addr - base)
		if offset == 2*int(overLimitSlotStride) && !bytes.Equal(data, oldSnapshot[offset:offset+int(overLimitSlotStride)]) {
			return failure
		}
		copy(memory[offset:offset+len(data)], data)
		return nil
	}
	reader := func(addr uintptr, size int) ([]byte, error) {
		offset := int(addr - base)
		return append([]byte(nil), memory[offset:offset+size]...), nil
	}

	err = writeOverLimitBatchAtomic(base, oldSnapshot, encoded, writer, reader)
	if !errors.Is(err, failure) {
		t.Fatalf("期望保留后槽注入错误，实际: %v", err)
	}
	if !bytes.Equal(memory, oldSnapshot) {
		t.Fatalf("后槽失败后前槽未完整回滚\nactual=% X\nwant=% X", memory, oldSnapshot)
	}
}

func TestWriteOverLimitBatchAtomicRollsBackPostWriteReadbackMismatch(t *testing.T) {
	base := uintptr(0x2000)
	snapshot := make([]byte, overLimitSlotCount*int(overLimitSlotStride))
	for i := range snapshot {
		snapshot[i] = byte(i + 1)
	}
	memory := append([]byte(nil), snapshot...)
	encoded, err := prepareOverLimitBatch([]OverLimitUpdate{
		{Index: 0, Attribute: 0xC4925BD7, Level: 1, Value: 100},
		{Index: 1, Attribute: 0x52A207B5, Level: 2, Value: 200},
		{Index: 2, Attribute: 0x45C65767, Level: 4, Value: 2},
		{Index: 3, Attribute: 0x6CB38EF3, Level: 8, Value: 0.4},
	})
	if err != nil {
		t.Fatal(err)
	}
	writer := func(addr uintptr, data []byte) error {
		offset := int(addr - base)
		copy(memory[offset:offset+len(data)], data)
		oldSlot := snapshot[offset : offset+int(overLimitSlotStride)]
		if offset == int(overLimitSlotStride) && !bytes.Equal(data, oldSlot) {
			memory[offset+len(data)-1] ^= 0xFF
		}
		return nil
	}
	reader := func(addr uintptr, size int) ([]byte, error) {
		offset := int(addr - base)
		return append([]byte(nil), memory[offset:offset+size]...), nil
	}

	err = writeOverLimitBatchAtomic(base, snapshot, encoded, writer, reader)
	if err == nil || !strings.Contains(err.Error(), "写后回读不一致") {
		t.Fatalf("未拒绝写入后字节不一致: %v", err)
	}
	if !bytes.Equal(memory, snapshot) {
		t.Fatalf("写后回读不一致时未恢复完整快照\nactual=% X\nwant=% X", memory, snapshot)
	}
}

func TestWriteOverLimitBatchAtomicMarksRollbackReadbackMismatchUnproven(t *testing.T) {
	base := uintptr(0x3000)
	snapshot := make([]byte, overLimitSlotCount*int(overLimitSlotStride))
	for i := range snapshot {
		snapshot[i] = byte(0x80 + i)
	}
	memory := append([]byte(nil), snapshot...)
	encoded, err := prepareOverLimitBatch([]OverLimitUpdate{
		{Index: 0, Attribute: 0xC4925BD7, Level: 1, Value: 100},
		{Index: 1, Attribute: 0x52A207B5, Level: 2, Value: 200},
		{Index: 2, Attribute: 0x45C65767, Level: 4, Value: 2},
		{Index: 3, Attribute: 0x6CB38EF3, Level: 8, Value: 0.4},
	})
	if err != nil {
		t.Fatal(err)
	}
	writer := func(addr uintptr, data []byte) error {
		offset := int(addr - base)
		copy(memory[offset:offset+len(data)], data)
		oldSlot := snapshot[offset : offset+int(overLimitSlotStride)]
		switch {
		case offset == int(overLimitSlotStride) && !bytes.Equal(data, oldSlot):
			// Force the primary post-write verification to fail.
			memory[offset+len(data)-1] ^= 0xFF
		case offset == 0 && bytes.Equal(data, oldSlot):
			// Report a successful rollback write while leaving different bytes.
			memory[offset] ^= 0xFF
		}
		return nil
	}
	reader := func(addr uintptr, size int) ([]byte, error) {
		offset := int(addr - base)
		return append([]byte(nil), memory[offset:offset+size]...), nil
	}

	err = writeOverLimitBatchAtomic(base, snapshot, encoded, writer, reader)
	if !errors.Is(err, errLiveMemoryRollbackUnproven) {
		t.Fatalf("回滚回读不一致未标记为不可证明: %v", err)
	}
	if bytes.Equal(memory, snapshot) {
		t.Fatal("测试未实际制造回滚回读不一致")
	}
}

func TestExpectedOverLimitSelectionRequiresOneCapturedAddressForAllSlots(t *testing.T) {
	const expected = uint64(0x12345000)
	updates := []OverLimitUpdate{
		{Index: 0, ExpectedSelectedAddr: expected},
		{Index: 1, ExpectedSelectedAddr: expected},
		{Index: 2, ExpectedSelectedAddr: expected},
		{Index: 3, ExpectedSelectedAddr: expected},
	}
	got, err := expectedOverLimitSelection(updates)
	if err != nil {
		t.Fatalf("stable expected selection rejected: %v", err)
	}
	if got != uintptr(expected) {
		t.Fatalf("expected selection = 0x%X, want 0x%X", got, expected)
	}

	updates[2].ExpectedSelectedAddr = expected + 0x10
	if _, err := expectedOverLimitSelection(updates); err == nil {
		t.Fatal("mixed expected selections were accepted")
	}
	updates[2].ExpectedSelectedAddr = expected
	updates[0].ExpectedSelectedAddr = 0
	if _, err := expectedOverLimitSelection(updates); err == nil {
		t.Fatal("missing expected selection was accepted")
	}
}

func TestValidateOverLimitSelectionRejectsStaleExpectedAddress(t *testing.T) {
	const expected = uintptr(0x12345000)
	if _, err := validateOverLimitSelection(expected, expected); err != nil {
		t.Fatalf("stable selected address rejected: %v", err)
	}
	for name, values := range map[string][2]uintptr{
		"missing expected token": {0, expected},
		"selection changed":      {expected, expected + 0x10},
		"selection cleared":      {expected, 0},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := validateOverLimitSelection(values[0], values[1]); err == nil {
				t.Fatal("stale or missing selected address must be rejected")
			}
		})
	}
}

func TestWriteOverLimitBatchLockedRevalidatesFullSnapshotAfterBackup(t *testing.T) {
	body := overLimitFunctionBodies(t)["writeOverLimitBatchLocked"]
	if body == nil {
		t.Fatal("writeOverLimitBatchLocked not found")
	}
	if got := countCallsIdent(body, "readProcessMemory"); got < 2 {
		t.Fatalf("writeOverLimitBatchLocked calls readProcessMemory %d time(s), want an initial snapshot and a full post-backup re-read", got)
	}
	if got := countCallsSelector(body, "bytes", "Equal"); got == 0 {
		t.Fatal("writeOverLimitBatchLocked does not reject a record that changed in place during backup")
	}
}
