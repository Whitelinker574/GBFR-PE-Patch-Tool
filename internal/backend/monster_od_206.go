package backend

import "fmt"

var odInline206Sites = []struct {
	RVA  uintptr
	Tail []byte
}{
	{0x2B3FFFE, []byte{0x48, 0x83, 0xC4, 0x20, 0x5B, 0x5F, 0x5E, 0xC3}},
	{0x2AAC2F2, []byte{0x48, 0x83, 0xC4, 0x28, 0x5F, 0x5E, 0xC3}},
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
