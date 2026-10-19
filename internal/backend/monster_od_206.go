package backend

import "fmt"

var odInline206Sites = []struct {
	RVA  uintptr
	Tail []byte
}{
	{0x2B3FFFE, []byte{0x48, 0x83, 0xC4, 0x20, 0x5B, 0x5F, 0x5E, 0xC3}},
	{0x2AAC2F2, []byte{0x48, 0x83, 0xC4, 0x28, 0x5F, 0x5E, 0xC3}},
}

const monsterHPShared206RVA = uintptr(0x3AB7920)
const monsterODShared206RVA = uintptr(0x3AB6460)

var monsterHPShared206Original = []byte{0x89, 0xD9, 0x48, 0x8B, 0x50, 0x10}
var monsterHPShared206Tail = []byte{0x45, 0x31, 0xC0, 0x48, 0x29, 0xCA, 0x4C, 0x0F, 0x43, 0xC2, 0xB9, 0x01, 0, 0, 0, 0x49, 0x0F, 0x47, 0xC8, 0x85, 0xFF, 0x49, 0x0F, 0x44, 0xC8, 0x48, 0x89, 0x48, 0x10}
var monsterODShared206Original = []byte{0x48, 0x03, 0x70, 0x18, 0x48, 0xC7, 0xC1, 0xFF, 0xFF, 0xFF, 0xFF, 0x48, 0x0F, 0x43, 0xCE, 0x48, 0x89, 0x48, 0x18}
var monsterODShared206Prefix = []byte{0x80, 0x78, 0x50, 0, 0x74, 0x13}

func (a *App) prepareSharedMonster206(od bool) (*monsterEnhanceAuxPreflight, error) {
	rva, original, tail := monsterHPShared206RVA, monsterHPShared206Original, monsterHPShared206Tail
	if od {
		rva, original, tail = monsterODShared206RVA, monsterODShared206Original, odInline206Sites[0].Tail
	}
	target := a.moduleBase + rva
	got, err := a.readMonsterEnhanceEntry(target+uintptr(len(original)), len(tail))
	if err != nil || !bytesEqual(got, tail) {
		return nil, fmt.Errorf("极星辅助路径 %X 的后续指令不符", rva)
	}
	if od {
		got, err = a.readMonsterEnhanceEntry(target-6, 6)
		if err != nil || !bytesEqual(got, monsterODShared206Prefix) {
			return nil, fmt.Errorf("极星 OD 前置条件指令不符")
		}
	}
	preflight := monsterEnhanceAuxPreflight{Target: target, Original: append([]byte(nil), original...), CaveSize: 128}
	if _, err := a.captureMonsterAuxiliary(preflight, false); err != nil {
		return nil, err
	}
	return &preflight, nil
}

func (a *App) prepareOdRate206Auxiliaries() (*monsterEnhanceAuxPreflight, error) {
	rows := make([]monsterEnhanceAuxPreflight, 0, len(odInline206Sites))
	for _, site := range odInline206Sites {
		target := a.moduleBase + site.RVA
		tail, err := a.readMonsterEnhanceEntry(target+uintptr(len(odRateInlineOriginal)), len(site.Tail))
		if err != nil {
			return nil, err
		}
		if !bytesEqual(tail, site.Tail) {
			return nil, fmt.Errorf("OD 辅助入口 %X 的栈恢复指令不符", site.RVA)
		}
		preflight := monsterEnhanceAuxPreflight{Target: target, Original: append([]byte(nil), odRateInlineOriginal...), CaveSize: 128}
		if _, err := a.captureMonsterAuxiliary(preflight, false); err != nil {
			return nil, err
		}
		rows = append(rows, preflight)
	}
	shared, err := a.prepareSharedMonster206(true)
	if err != nil {
		return nil, err
	}
	rows = append(rows, *shared)
	result := rows[0]
	result.Additional = rows[1:]
	return &result, nil
}

func (a *App) captureMonsterAuxiliary(preflight monsterEnhanceAuxPreflight, requirePatched bool) (monsterEnhanceOwnedPatch, error) {
	current, err := a.readMonsterEnhanceEntry(preflight.Target, len(preflight.Original))
	if err != nil {
		return monsterEnhanceOwnedPatch{}, err
	}
	record := monsterEnhanceOwnedPatch{Target: preflight.Target, Original: append([]byte(nil), preflight.Original...), Patched: current}
	if bytesEqual(current, preflight.Original) {
		if requirePatched {
			return record, fmt.Errorf("辅助 Hook 未安装: %X", preflight.Target)
		}
		return record, nil
	}
	cave, ok := monsterEnhanceRelJumpTarget(preflight.Target, current)
	if !ok {
		return record, fmt.Errorf("auxiliary hook is not a rel32 jump: %X", preflight.Target)
	}
	record.Cave, record.CaveSize = cave, preflight.CaveSize
	if err := a.verifyMonsterEnhanceCaveMarker(record); err != nil {
		return record, err
	}
	return record, nil
}

func (a *App) restoreMonsterAuxiliary(record monsterEnhanceOwnedPatch) error {
	current, err := a.readMonsterEnhanceEntry(record.Target, len(record.Original))
	if err != nil {
		return err
	}
	if bytesEqual(current, record.Original) {
		return nil
	}
	if !bytesEqual(current, record.Patched) {
		return fmt.Errorf("辅助 Hook 已被其他程序修改: %X", record.Target)
	}
	cave, ok := monsterEnhanceRelJumpTarget(record.Target, current)
	if !ok || cave != record.Cave {
		return fmt.Errorf("辅助 Hook 跳转目标变化: %X", record.Target)
	}
	if err := a.verifyMonsterEnhanceCaveMarker(record); err != nil {
		return err
	}
	return a.writeAndVerifyMonsterEnhanceEntry(record.Target, record.Original, "OD auxiliary restore")
}
