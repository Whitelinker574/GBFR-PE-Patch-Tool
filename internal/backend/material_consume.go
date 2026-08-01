package backend

import (
	"bytes"
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	materialConsumeRVA              = uintptr(0x356621)
	materialConsumeHookSize         = 7
	materialConsumeCaveMarkerOffset = 0x20
	materialConsumeCaveSize         = materialConsumeCaveMarkerOffset + 8
)

var (
	// Both instructions are displaced because the five-byte rel32 jump cannot
	// split the four-byte add that changes the inventory count.
	materialConsumeOrig = []byte{0x41, 0x01, 0x76, 0x04, 0x4C, 0x89, 0xE1}
	// v1.9.2 and earlier builds used this blanket NOP. Keep it restore-only so a
	// running legacy state can be migrated without treating it as an unknown
	// foreign patch.
	materialConsumeLegacyPatch = []byte{0x90, 0x90, 0x90, 0x90, 0x4C, 0x89, 0xE1}
	materialConsumeCaveMarker  = [...]byte{'G', 'B', 'F', 'R', 'M', 'A', 'T', '1'}
)

type MaterialConsumeStatus struct {
	RVA          uint64 `json:"rva"`
	Enabled      bool   `json:"enabled"`
	CurrentBytes string `json:"currentBytes"`
}

type materialConsumeHookLease struct {
	OwnerToken string
	Process    processInstanceID
	EntryAddr  uintptr
	Original   []byte
	Installed  []byte
	CaveAddr   uintptr
	CaveSize   uintptr
}

func (lease *materialConsumeHookLease) active() bool {
	return lease != nil && (lease.EntryAddr != 0 || lease.CaveAddr != 0 || len(lease.Original) != 0 || len(lease.Installed) != 0)
}

func (a *App) MaterialConsumeGetStatus() (MaterialConsumeStatus, error) {
	if err := a.acquireGameProcessLease(); err != nil {
		return MaterialConsumeStatus{}, err
	}
	defer a.procMu.Unlock()
	return a.materialConsumeGetStatusLocked(a.monsterEnhanceOwnerForCompatibilityCall())
}

func (a *App) MaterialConsumeGetStatusOwned(token string) (MaterialConsumeStatus, error) {
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, token); err != nil {
		return MaterialConsumeStatus{}, err
	}
	defer a.procMu.Unlock()
	return a.materialConsumeGetStatusLocked(token)
}

func (a *App) materialConsumeGetStatusLocked(ownerToken string) (MaterialConsumeStatus, error) {
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	return a.readMaterialConsumeStatusLocked(ownerToken)
}

func (a *App) MaterialConsumeSetEnabled(enabled bool) (MaterialConsumeStatus, error) {
	liveMemoryWriteMu.Lock()
	defer liveMemoryWriteMu.Unlock()
	if err := a.acquireGameProcessLease(); err != nil {
		return MaterialConsumeStatus{}, err
	}
	defer a.procMu.Unlock()
	if err := a.ensureLiveMemoryWritesSafe(); err != nil {
		return MaterialConsumeStatus{}, err
	}
	return a.materialConsumeSetEnabledLocked(a.monsterEnhanceOwnerForCompatibilityCall(), enabled)
}

func (a *App) MaterialConsumeSetEnabledOwned(token string, enabled bool) (MaterialConsumeStatus, error) {
	liveMemoryWriteMu.Lock()
	defer liveMemoryWriteMu.Unlock()
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, token); err != nil {
		return MaterialConsumeStatus{}, err
	}
	defer a.procMu.Unlock()
	if err := a.ensureLiveMemoryWritesSafe(); err != nil {
		return MaterialConsumeStatus{}, err
	}
	return a.materialConsumeSetEnabledLocked(token, enabled)
}

func (a *App) materialConsumeSetEnabledLocked(ownerToken string, enabled bool) (MaterialConsumeStatus, error) {
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	if enabled {
		if a.materialConsumeLease != nil {
			status, err := a.readMaterialConsumeStatusLocked(ownerToken)
			if err == nil && status.Enabled {
				return status, nil
			}
			if restoreErr := a.restoreMaterialConsumeOwnedLocked(ownerToken, false); restoreErr != nil {
				return MaterialConsumeStatus{}, errors.Join(err, restoreErr)
			}
		}
		return a.enableMaterialConsumeLocked(ownerToken)
	}
	if a.materialConsumeLease != nil {
		if err := a.restoreMaterialConsumeOwnedLocked(ownerToken, false); err != nil {
			return MaterialConsumeStatus{}, err
		}
		return a.readMaterialConsumeStatusLocked(ownerToken)
	}

	addr, err := a.locateMaterialConsumeLocked()
	if err != nil {
		return MaterialConsumeStatus{}, err
	}
	current, err := a.readSharedRuntimePatch(addr)
	if err != nil {
		return MaterialConsumeStatus{}, err
	}
	if bytes.Equal(current, materialConsumeLegacyPatch) {
		if err := writeAndVerifyRuntimeHookEntry(a.hProcess, addr, materialConsumeOrig); err != nil {
			return MaterialConsumeStatus{}, fmt.Errorf("恢复旧版素材不消耗补丁失败: %w", err)
		}
	}
	return a.readMaterialConsumeStatusLocked(ownerToken)
}

func (a *App) enableMaterialConsumeLocked(ownerToken string) (MaterialConsumeStatus, error) {
	addr, err := a.locateMaterialConsumeLocked()
	if err != nil {
		return MaterialConsumeStatus{}, err
	}
	current, err := a.readSharedRuntimePatch(addr)
	if err != nil {
		return MaterialConsumeStatus{}, err
	}
	if err := validateSharedRuntimePatchTransition(current, sharedRuntimePatchOwnerMaterialConsume, true); err != nil {
		return MaterialConsumeStatus{}, err
	}
	if bytes.Equal(current, materialConsumeLegacyPatch) {
		if err := writeAndVerifyRuntimeHookEntry(a.hProcess, addr, materialConsumeOrig); err != nil {
			return MaterialConsumeStatus{}, fmt.Errorf("迁移旧版素材不消耗补丁失败: %w", err)
		}
		current = append([]byte(nil), materialConsumeOrig...)
	}
	if !bytes.Equal(current, materialConsumeOrig) {
		return MaterialConsumeStatus{}, fmt.Errorf("素材增减入口不是已验证版本的原始指令: %s", bytesToHex(current))
	}

	cave, err := virtualAllocRemoteNear(a.hProcess, addr, 0x1000)
	if err != nil {
		return MaterialConsumeStatus{}, fmt.Errorf("分配素材不消耗代码洞失败: %w", err)
	}
	code, err := buildMaterialConsumeCave(cave, addr+materialConsumeHookSize)
	if err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return MaterialConsumeStatus{}, err
	}
	if err := writeCodeMemory(a.hProcess, cave, code); err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return MaterialConsumeStatus{}, fmt.Errorf("写入素材不消耗代码洞失败: %w", err)
	}
	if err := a.validateMaterialConsumeCaveLocked(cave, addr); err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return MaterialConsumeStatus{}, fmt.Errorf("素材不消耗代码洞写后校验失败: %w", err)
	}
	patch, err := makeMaterialConsumeEntry(addr, cave)
	if err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return MaterialConsumeStatus{}, err
	}
	lease := &materialConsumeHookLease{
		OwnerToken: ownerToken,
		Process:    a.currentProcessInstance(),
		EntryAddr:  addr,
		Original:   append([]byte(nil), materialConsumeOrig...),
		Installed:  append([]byte(nil), patch...),
		CaveAddr:   cave,
		CaveSize:   materialConsumeCaveSize,
	}
	result, err := installRemoteCodeHook(a.hProcess, addr, materialConsumeOrig, patch)
	if err != nil {
		return MaterialConsumeStatus{}, runtimeHookInstallFailure(
			runtimePatchMonitorText("素材不消耗条件 Hook", "No-material-consumption conditional hook"),
			result,
			err,
			func() { _ = virtualFreeRemote(a.hProcess, cave) },
			func() { a.retireRuntimeCaveLocked(cave, "material-consume install rollback") },
			func() { a.materialConsumeLease = lease },
			a.poisonCurrentLiveMemoryWrites,
		)
	}
	a.materialConsumeLease = lease
	status, verifyErr := a.readMaterialConsumeStatusLocked(ownerToken)
	if verifyErr == nil && status.Enabled {
		return status, nil
	}
	rollbackErr := a.restoreMaterialConsumeOwnedLocked(ownerToken, false)
	if rollbackErr != nil {
		a.poisonCurrentLiveMemoryWrites()
		return MaterialConsumeStatus{}, errors.Join(verifyErr, errRuntimeHookRollbackUnproven, rollbackErr)
	}
	if verifyErr == nil {
		verifyErr = fmt.Errorf("素材不消耗条件 Hook 未进入启用状态")
	}
	return MaterialConsumeStatus{}, fmt.Errorf("素材不消耗安装后验证失败，原始入口已恢复: %w", verifyErr)
}

func (a *App) readMaterialConsumeStatusLocked(ownerToken string) (MaterialConsumeStatus, error) {
	addr, err := a.locateMaterialConsumeLocked()
	if err != nil {
		return MaterialConsumeStatus{}, err
	}
	current, err := a.readSharedRuntimePatch(addr)
	if err != nil {
		return MaterialConsumeStatus{}, err
	}
	owner := classifySharedRuntimePatch(current)
	if owner != sharedRuntimePatchOwnerNone && owner != sharedRuntimePatchOwnerMaterialConsume {
		if owner == sharedRuntimePatchOwnerUnknown {
			return MaterialConsumeStatus{}, fmt.Errorf("素材增减入口字节异常: %s", bytesToHex(current))
		}
		return MaterialConsumeStatus{}, fmt.Errorf("共享补丁地址正由%s占用，请先恢复后再读取素材状态", sharedRuntimePatchOwnerLabel(owner))
	}
	if isMaterialConsumeEntry(current) {
		lease := a.materialConsumeLease
		if lease == nil || !lease.active() {
			return MaterialConsumeStatus{}, fmt.Errorf("检测到非本任务持有的素材不消耗 Hook；请完全退出游戏后重试")
		}
		if !sameProcessInstance(lease.Process, a.currentProcessInstance()) || lease.EntryAddr != addr {
			return MaterialConsumeStatus{}, fmt.Errorf("素材不消耗恢复租约属于另一个游戏进程实例")
		}
		if ownerToken == "" || lease.OwnerToken != ownerToken {
			return MaterialConsumeStatus{}, errRuntimeOwnerLeaseStale
		}
		if !bytes.Equal(current, lease.Installed) || relJumpTarget(addr, current) != lease.CaveAddr {
			return MaterialConsumeStatus{}, fmt.Errorf("素材不消耗入口已被外部修改: %s", bytesToHex(current))
		}
		if err := a.validateMaterialConsumeCaveLocked(lease.CaveAddr, lease.EntryAddr); err != nil {
			return MaterialConsumeStatus{}, fmt.Errorf("素材不消耗代码洞所有权校验失败: %w", err)
		}
	}
	return MaterialConsumeStatus{
		RVA:          uint64(addr - a.moduleBase),
		Enabled:      owner == sharedRuntimePatchOwnerMaterialConsume,
		CurrentBytes: bytesToHex(current),
	}, nil
}

// restoreMaterialConsumeOwnedLocked runs with procMu and runtimePatchMu held.
func (a *App) restoreMaterialConsumeOwnedLocked(ownerToken string, force bool) error {
	lease := a.materialConsumeLease
	if lease == nil || !lease.active() {
		return nil
	}
	if !sameProcessInstance(lease.Process, a.currentProcessInstance()) {
		return fmt.Errorf("素材不消耗恢复租约属于另一个游戏进程实例")
	}
	if !force && (ownerToken == "" || lease.OwnerToken != ownerToken) {
		return errRuntimeOwnerLeaseStale
	}
	if len(lease.Original) != materialConsumeHookSize || len(lease.Installed) != materialConsumeHookSize ||
		lease.EntryAddr == 0 || lease.CaveAddr == 0 {
		return fmt.Errorf("素材不消耗恢复租约不完整")
	}
	current, err := a.readSharedRuntimePatch(lease.EntryAddr)
	if err != nil {
		return err
	}
	originalEntry := bytes.Equal(current, lease.Original)
	if !originalEntry {
		if !bytes.Equal(current, lease.Installed) || !isMaterialConsumeEntry(current) ||
			relJumpTarget(lease.EntryAddr, current) != lease.CaveAddr {
			return fmt.Errorf("素材不消耗入口既不是自有跳转也不是原始指令: %s", bytesToHex(current))
		}
	}
	if err := a.validateMaterialConsumeCaveLocked(lease.CaveAddr, lease.EntryAddr); err != nil {
		return fmt.Errorf("素材不消耗代码洞所有权校验失败: %w", err)
	}
	if !originalEntry {
		if err := writeAndVerifyRuntimeHookEntry(a.hProcess, lease.EntryAddr, lease.Original); err != nil {
			return fmt.Errorf("恢复素材增减原始指令失败: %w", err)
		}
	}
	a.retireRuntimeCaveLocked(lease.CaveAddr, "material-consume release")
	a.materialConsumeLease = nil
	return nil
}

func (a *App) dropMaterialConsumeOwnerLocked(ownerToken string, force bool) {
	if a.materialConsumeLease == nil {
		return
	}
	if force || (ownerToken != "" && a.materialConsumeLease.OwnerToken == ownerToken) {
		a.materialConsumeLease = nil
		a.materialConsumeAddr = 0
	}
}

func (a *App) locateMaterialConsumeLocked() (uintptr, error) {
	if a.materialConsumeAddr != 0 {
		return a.materialConsumeAddr, nil
	}
	addr, err := a.resolveRuntimeItemSite(
		runtimeInventoryMaterialAOB,
		"升级/强化材料增减指令",
		func(layout runtimeGameLayout) uintptr { return layout.InventoryMaterialRVA },
		materialConsumeOrig,
		materialConsumeHookSize,
		func(prefix []byte) bool { return classifySharedRuntimePatch(prefix) != sharedRuntimePatchOwnerUnknown },
	)
	if err != nil {
		return 0, err
	}
	current, err := a.readSharedRuntimePatch(addr)
	if err != nil {
		return 0, err
	}
	owner := classifySharedRuntimePatch(current)
	if owner == sharedRuntimePatchOwnerInventoryQuantity {
		return 0, fmt.Errorf("共享补丁地址正由%s占用，请先恢复", sharedRuntimePatchOwnerLabel(owner))
	}
	if owner != sharedRuntimePatchOwnerNone && owner != sharedRuntimePatchOwnerMaterialConsume {
		return 0, fmt.Errorf("素材增减入口字节异常: %s", bytesToHex(current))
	}
	a.materialConsumeAddr = addr
	return addr, nil
}

func (a *App) readSharedRuntimePatch(addr uintptr) ([]byte, error) {
	buf := make([]byte, len(sharedInventoryMaterialOriginal))
	if err := readProcessMemory(a.hProcess, addr, unsafe.Pointer(&buf[0]), uintptr(len(buf))); err != nil {
		return nil, fmt.Errorf("读取素材/小钳蟹共享指令失败: %w", err)
	}
	return buf, nil
}

func makeMaterialConsumeEntry(entry, cave uintptr) ([]byte, error) {
	patch, err := makeRelJump(entry, cave, materialConsumeHookSize)
	if err != nil {
		return nil, err
	}
	// A two-byte NOP uniquely identifies this Go-owned hook. patch_core's
	// inventory quantity hook uses 90 90 at the same seven-byte entry.
	patch[5], patch[6] = 0x66, 0x90
	return patch, nil
}

func isMaterialConsumeEntry(entry []byte) bool {
	return len(entry) == materialConsumeHookSize && entry[0] == 0xE9 && entry[5] == 0x66 && entry[6] == 0x90
}

func buildMaterialConsumeCave(cave, returnAddr uintptr) ([]byte, error) {
	code := make([]byte, 0, materialConsumeCaveSize)
	code = append(code, 0x85, 0xF6)             // test esi,esi
	code = append(code, 0x78, 0x0C)             // js negative path
	code = append(code, 0x41, 0x01, 0x76, 0x04) // add [r14+04],esi
	code = append(code, 0x4C, 0x89, 0xE1)       // mov rcx,r12
	positiveJump, err := makeRelJump(cave+uintptr(len(code)), returnAddr, 5)
	if err != nil {
		return nil, err
	}
	code = append(code, positiveJump...)
	code = append(code, 0x4C, 0x89, 0xE1) // negative: mov rcx,r12
	negativeJump, err := makeRelJump(cave+uintptr(len(code)), returnAddr, 5)
	if err != nil {
		return nil, err
	}
	code = append(code, negativeJump...)
	for len(code) < materialConsumeCaveMarkerOffset {
		code = append(code, 0x90)
	}
	code = append(code, materialConsumeCaveMarker[:]...)
	return code, nil
}

func validateMaterialConsumeCaveBytes(cave, entry uintptr, code []byte) error {
	if len(code) < materialConsumeCaveSize {
		return fmt.Errorf("代码洞过短: %d", len(code))
	}
	if !bytes.Equal(code[:4], []byte{0x85, 0xF6, 0x78, 0x0C}) {
		return fmt.Errorf("素材扣减分支指令不匹配")
	}
	if !bytes.Equal(code[4:11], materialConsumeOrig) {
		return fmt.Errorf("代码洞内正向获得指令不匹配")
	}
	if code[11] != 0xE9 || relJumpTarget(cave+11, code[11:16]) != entry+materialConsumeHookSize {
		return fmt.Errorf("正向获得路径回跳地址不匹配")
	}
	if !bytes.Equal(code[16:19], materialConsumeOrig[4:]) {
		return fmt.Errorf("代码洞内负向跳过路径不匹配")
	}
	if code[19] != 0xE9 || relJumpTarget(cave+19, code[19:24]) != entry+materialConsumeHookSize {
		return fmt.Errorf("负向跳过路径回跳地址不匹配")
	}
	if !bytes.Equal(code[materialConsumeCaveMarkerOffset:materialConsumeCaveSize], materialConsumeCaveMarker[:]) {
		return fmt.Errorf("代码洞所有权标记不匹配")
	}
	return nil
}

func (a *App) validateMaterialConsumeCaveLocked(cave, entry uintptr) error {
	code := make([]byte, materialConsumeCaveSize)
	if err := readProcessMemory(a.hProcess, cave, unsafe.Pointer(&code[0]), uintptr(len(code))); err != nil {
		return fmt.Errorf("读取代码洞失败: %w", err)
	}
	return validateMaterialConsumeCaveBytes(cave, entry, code)
}

func writeAndVerifyRuntimeHookEntry(handle windows.Handle, entry uintptr, desired []byte) error {
	if err := writeCodeMemory(handle, entry, desired); err != nil {
		return err
	}
	actual := make([]byte, len(desired))
	if err := readProcessMemory(handle, entry, unsafe.Pointer(&actual[0]), uintptr(len(actual))); err != nil {
		return fmt.Errorf("写后回读失败: %w", err)
	}
	if !bytes.Equal(actual, desired) {
		return fmt.Errorf("写后回读不一致: %s", bytesToHex(actual))
	}
	return nil
}
