package backend

import (
	"encoding/binary"
	"fmt"
)

type runtimeGameLayout struct {
	Version               string
	PartyPointerRVA       uintptr
	PartySlotTableRVA     uintptr
	PartyHandleTableRVA   uintptr
	PartyEntityTableRVA   uintptr
	PartyHandleRootOffset uintptr
	SelectedMaterialRVA   uintptr
	SelectedKeyItemRVA    uintptr
	SigilHookRVA          uintptr
	WrightstoneHookRVA    uintptr
	WeaponHookRVA         uintptr
	SaveFunctionRVA       uintptr
	PartyCharaPowerRVA    uintptr
	SummonInventoryPtrRVA uintptr
	SpatialGravityRVA     uintptr
	InventoryMaterialRVA  uintptr
}

var runtimeGameLayouts = [...]runtimeGameLayout{
	{
		Version:         "2.0.2",
		PartyPointerRVA: 0x22CECA0, PartySlotTableRVA: 0x7036860,
		PartyHandleTableRVA: 0x70367F0, PartyEntityTableRVA: 0x70214E8,
		PartyHandleRootOffset: 0,
		SelectedMaterialRVA:   0x3F4BAC3, SelectedKeyItemRVA: 0x3F2061C,
		SigilHookRVA: 0x345157, WrightstoneHookRVA: 0x361CB4,
		SaveFunctionRVA: 0x79D820, PartyCharaPowerRVA: 0x7C24A78, SummonInventoryPtrRVA: 0x7C23F48,
		SpatialGravityRVA: 0x39DD964, InventoryMaterialRVA: 0x356621,
	},
	{
		Version:         "2.0.3",
		PartyPointerRVA: 0x22C9310, PartySlotTableRVA: 0x7033820,
		PartyHandleTableRVA: 0x70337B0, PartyEntityTableRVA: 0x701E4A8,
		PartyHandleRootOffset: 0x58,
		SelectedMaterialRVA:   0x3F479F3, SelectedKeyItemRVA: 0x3F1C54C,
		SigilHookRVA: 0x33E427, WrightstoneHookRVA: 0x35AF84,
		WeaponHookRVA:   0x415118C,
		SaveFunctionRVA: 0x796E60, PartyCharaPowerRVA: 0x7C21A38, SummonInventoryPtrRVA: 0x7C20F08,
		SpatialGravityRVA: 0x39D8E24, InventoryMaterialRVA: 0x34F8F1,
	},
	{
		Version:         "2.0.4",
		PartyPointerRVA: 0x22CA2B0, PartySlotTableRVA: 0x7034AA0,
		PartyHandleTableRVA: 0x7034A30, PartyEntityTableRVA: 0x701F728,
		PartyHandleRootOffset: 0x58,
		SelectedMaterialRVA:   0x3F48993, SelectedKeyItemRVA: 0x3F1D4EC,
		SigilHookRVA: 0x33E427, WrightstoneHookRVA: 0x35AF84,
		WeaponHookRVA:   0x415212C,
		SaveFunctionRVA: 0x797E00, PartyCharaPowerRVA: 0x7C22CB8, SummonInventoryPtrRVA: 0x7C22188,
		SpatialGravityRVA: 0x39D9DC4, InventoryMaterialRVA: 0x34F8F1,
	},
}

func isKnownRuntimeGameLayout(candidate runtimeGameLayout) bool {
	for _, layout := range runtimeGameLayouts {
		if candidate == layout {
			return true
		}
	}
	return false
}

func detectRuntimeGameLayout(memory runtimePatchPartyMemory, moduleBase uintptr) (runtimeGameLayout, error) {
	if memory == nil || moduleBase == 0 {
		return runtimeGameLayout{}, fmt.Errorf("运行时版本布局检测参数无效")
	}
	pattern, err := parseRuntimePatchPattern(runtimePatchPartyPointerAOB)
	if err != nil {
		return runtimeGameLayout{}, err
	}
	for _, layout := range runtimeGameLayouts {
		site, ok := checkedRuntimePatchMonitorAddress(moduleBase, layout.PartyPointerRVA)
		if !ok {
			continue
		}
		actual := make([]byte, len(pattern.Values))
		if err := memory.ReadAt(site, actual); err != nil || !matchRuntimePatchPattern(actual, pattern) {
			continue
		}
		displacement := int64(int32(binary.LittleEndian.Uint32(actual[3:7])))
		resolved := int64(site) + 7 + displacement
		expected, ok := checkedRuntimePatchMonitorAddress(moduleBase, layout.PartySlotTableRVA)
		if ok && resolved > 0 && uintptr(resolved) == expected {
			return layout, nil
		}
	}
	return runtimeGameLayout{}, fmt.Errorf("当前游戏的运行时布局未能通过逐功能签名定位")
}

func runtimeGameLayoutForHookRVA(moduleBase, address uintptr, name string, selectRVA func(runtimeGameLayout) uintptr) (runtimeGameLayout, error) {
	if moduleBase == 0 || address < moduleBase {
		return runtimeGameLayout{}, fmt.Errorf("%s 地址不在当前模块内", name)
	}
	rva := address - moduleBase
	for _, layout := range runtimeGameLayouts {
		if selectRVA(layout) == rva {
			return layout, nil
		}
	}
	return runtimeGameLayout{}, fmt.Errorf("%s RVA 0x%X 未知，无法选择版本布局", name, rva)
}

func runtimeGameLayoutForSigilHook(moduleBase, address uintptr) (runtimeGameLayout, error) {
	return runtimeGameLayoutForHookRVA(moduleBase, address, "因子 Hook", func(layout runtimeGameLayout) uintptr {
		return layout.SigilHookRVA
	})
}

func runtimeGameLayoutForWrightstoneHook(moduleBase, address uintptr) (runtimeGameLayout, error) {
	return runtimeGameLayoutForHookRVA(moduleBase, address, "祝福 Hook", func(layout runtimeGameLayout) uintptr {
		return layout.WrightstoneHookRVA
	})
}
