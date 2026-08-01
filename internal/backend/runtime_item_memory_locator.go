package backend

import (
	"bytes"
	"fmt"
	"unsafe"
)

// These signatures deliberately include stable instructions after each entry
// and wildcard only relative displacements. Known RVAs remain a fast path, but
// a game update that merely moves the code no longer requires a new build.
const (
	runtimeInventoryMaterialAOB = "41 01 76 04 4C 89 E1 E8 ?? ?? ?? ?? 41 8B 0C 24 31 C0 85 C9 0F 4F C1"
	runtimeSigilHookAOB         = "31 C9 81 38 B0 E0 7A 88 0F 95 C1 31 D2 81 78 08 B0 E0 7A 88 0F 95 C2 01 ?? 31 ?? 83 FA 02 0F 93 C0 EB 02 31 C0 49 8B 4D 18"
	runtimeWrightstoneHookAOB   = "48 89 D7 48 89 CE E8 ?? ?? ?? ?? 48 39 FE 74 4C 48 8D 4E 18 8B 47 18 39 01 74 07 89 01 E8 ?? ?? ?? ?? 8B 47 1C 39 46 1C"
	runtimeItemSaveFunctionAOB  = "55 48 83 EC 60 48 8D 6C 24 60 48 C7 45 F8 FE FF FF FF 48 8B 05 ?? ?? ?? ?? 48 85 C0"
)

type runtimeInstalledPrefix func([]byte) bool

func (a *App) resolveRuntimeItemSite(
	rawPattern, label string,
	selectRVA func(runtimeGameLayout) uintptr,
	expectedOriginal []byte,
	hookSize int,
	acceptInstalled runtimeInstalledPrefix,
) (uintptr, error) {
	if a.hProcess == 0 || a.moduleBase == 0 {
		return 0, fmt.Errorf("%s：未连接游戏进程", label)
	}
	pattern, err := parseRuntimePatchPattern(rawPattern)
	if err != nil {
		return 0, fmt.Errorf("%s特征无效: %w", label, err)
	}

	for _, layout := range runtimeGameLayouts {
		address, ok := checkedRuntimePatchMonitorAddress(a.moduleBase, selectRVA(layout))
		if !ok {
			continue
		}
		actual := make([]byte, len(pattern.Values))
		if err := readProcessMemory(a.hProcess, address, unsafe.Pointer(&actual[0]), uintptr(len(actual))); err != nil {
			continue
		}
		if matchRuntimePatchPattern(actual, pattern) || (len(expectedOriginal) != 0 && len(actual) >= len(expectedOriginal) && bytes.Equal(actual[:len(expectedOriginal)], expectedOriginal)) {
			return address, nil
		}
		if hookSize <= 0 || hookSize >= len(actual) || acceptInstalled == nil || !acceptInstalled(actual[:hookSize]) {
			continue
		}
		suffix := runtimePatchPattern{
			Values: append([]byte(nil), pattern.Values[hookSize:]...),
			Mask:   append([]byte(nil), pattern.Mask[hookSize:]...),
		}
		if matchRuntimePatchPattern(actual[hookSize:], suffix) {
			return address, nil
		}
	}

	address, err := a.scanRuntimePatchPatternUnique(pattern, label)
	if err != nil {
		return 0, fmt.Errorf("%s固定 RVA 已变化且 AOB 唯一定位失败: %w", label, err)
	}
	return address, nil
}

func (a *App) resolveSigilMemoryHookLocked() (uintptr, error) {
	if a.sigilMemoryHookAddr != 0 {
		return a.sigilMemoryHookAddr, nil
	}
	return a.resolveRuntimeItemSite(
		runtimeSigilHookAOB,
		"因子实时编辑入口",
		func(layout runtimeGameLayout) uintptr { return layout.SigilHookRVA },
		sigilMemoryOriginalBytes,
		int(sigilMemoryHookSize),
		isSigilMemoryJump,
	)
}

func (a *App) resolveWrightstoneMemoryHookLocked() (uintptr, error) {
	if a.wrightstoneMemoryHookAddr != 0 {
		return a.wrightstoneMemoryHookAddr, nil
	}
	return a.resolveRuntimeItemSite(
		runtimeWrightstoneHookAOB,
		"祝福实时编辑入口",
		func(layout runtimeGameLayout) uintptr { return layout.WrightstoneHookRVA },
		wrightstoneMemoryOriginalBytes,
		int(wrightstoneMemoryHookSize),
		isWrightstoneMemoryJump,
	)
}

func (a *App) resolveItemSaveFunctionLocked() (uintptr, error) {
	pattern, err := parseRuntimePatchPattern(runtimeItemSaveFunctionAOB)
	if err != nil {
		return 0, err
	}
	if a.itemSaveFunctionAddr != 0 {
		actual := make([]byte, len(pattern.Values))
		if err := readProcessMemory(a.hProcess, a.itemSaveFunctionAddr, unsafe.Pointer(&actual[0]), uintptr(len(actual))); err == nil && matchRuntimePatchPattern(actual, pattern) {
			return a.itemSaveFunctionAddr, nil
		}
		a.itemSaveFunctionAddr = 0
	}

	address, err := a.resolveRuntimeItemSite(
		runtimeItemSaveFunctionAOB,
		"游戏内物品保存函数",
		func(layout runtimeGameLayout) uintptr { return layout.SaveFunctionRVA },
		gameSaveFunctionPrologue,
		0,
		nil,
	)
	if err != nil {
		return 0, err
	}
	if err := a.validateRemoteFunctionStart(address, "游戏内物品保存函数"); err != nil {
		return 0, err
	}
	a.itemSaveFunctionAddr = address
	return address, nil
}

func (a *App) itemSaveRVAForStatusLocked(knownRVA uintptr) (uintptr, error) {
	if a.itemSaveFunctionAddr != 0 {
		return a.itemSaveFunctionAddr - a.moduleBase, nil
	}
	// Status reads must remain side-effect free and usable by the synthetic
	// hook-lifecycle tests. A known hook RVA already selects an audited layout;
	// only an AOB-relocated hook needs the save function to be scanned here.
	if knownRVA != 0 {
		return knownRVA, nil
	}
	address, err := a.resolveItemSaveFunctionLocked()
	if err != nil {
		return 0, err
	}
	return address - a.moduleBase, nil
}
