package backend

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"unsafe"
)

const (
	taskRewardMultiplierHookSize       = 14
	taskRewardMultiplierCaveSize       = 0xD0
	taskRewardMultiplierMarkerOffset   = 0xB0
	taskRewardMultiplierValueOffset    = 0xC0
	taskRewardMultiplierCounterOffset  = 0xC8
	taskRewardMultiplierRecordSize     = 0x24
	taskRewardMultiplierItemTypeOffset = 0x08
	taskRewardMultiplierQuantityOffset = 0x04
	taskRewardMultiplierItemType       = 1
	taskRewardMultiplierMaximum        = 999
)

var (
	taskRewardMultiplierMarker  = [...]byte{'G', 'B', 'F', 'R', 'R', 'E', 'W', '1'}
	taskRewardMultiplierPattern = []byte{
		0x48, 0x8B, 0x0D, 0, 0, 0, 0, 0x31, 0xF6, 0x31, 0xD2, 0x45, 0x31, 0xC0,
		0xE8, 0, 0, 0, 0, 0x48, 0x8D, 0x1D, 0, 0, 0, 0, 0x4C, 0x8D, 0x35,
	}
	taskRewardMultiplierMask = []bool{
		true, true, true, false, false, false, false, true, true, true, true, true, true, true,
		true, false, false, false, false, true, true, true, false, false, false, false, true, true, true,
	}
)

type TaskRewardMultiplierStatus struct {
	Enabled      bool   `json:"enabled"`
	Multiplier   int    `json:"multiplier"`
	MatchedItems uint64 `json:"matchedItems"`
	RVA          uint64 `json:"rva"`
	GameVersion  string `json:"gameVersion"`
	Detail       string `json:"detail"`
}

type taskRewardMultiplierLease struct {
	OwnerToken string
	Process    processInstanceID
	EntryAddr  uintptr
	Original   []byte
	Installed  []byte
	CaveAddr   uintptr
	Multiplier int
}

func validTaskRewardMultiplier(value int) bool {
	return value == 1 || value == 2 || value == 4 || value == 8 || value == 16
}

func buildTaskRewardMultiplierCave(cave, returnAddr, managerSlot uintptr, multiplier int, matched uint64) ([]byte, error) {
	if cave == 0 || returnAddr == 0 || managerSlot == 0 || !validTaskRewardMultiplier(multiplier) || multiplier == 1 {
		return nil, fmt.Errorf("invalid task reward multiplier cave parameters")
	}
	code := make([]byte, taskRewardMultiplierCaveSize)
	for index := range code {
		code[index] = 0x90
	}
	position := 0
	appendBytes := func(value ...byte) { copy(code[position:], value); position += len(value) }
	appendRel32 := func(opcode []byte, target uintptr) error {
		appendBytes(opcode...)
		delta := int64(target) - int64(cave+uintptr(position)+4)
		if delta < -0x80000000 || delta > 0x7fffffff {
			return fmt.Errorf("task reward multiplier relative branch is out of range")
		}
		binary.LittleEndian.PutUint32(code[position:position+4], uint32(int32(delta)))
		position += 4
		return nil
	}

	appendBytes(0x9C, 0x50, 0x53, 0x52, 0x41, 0x51)
	appendBytes(0x4C, 0x8B, 0x8D, 0x08, 0x01, 0x00, 0x00)
	appendBytes(0x49, 0x8B, 0x81, 0x40, 0x0A, 0x00, 0x00)
	appendBytes(0x49, 0x8B, 0x91, 0x48, 0x0A, 0x00, 0x00)
	loopOffset := position
	appendBytes(0x48, 0x39, 0xD0)
	doneJump := position
	appendBytes(0x0F, 0x83, 0, 0, 0, 0)
	appendBytes(0x83, 0x78, taskRewardMultiplierItemTypeOffset, taskRewardMultiplierItemType)
	nextJump := position
	appendBytes(0x0F, 0x85, 0, 0, 0, 0)
	appendBytes(0x8B, 0x58, taskRewardMultiplierQuantityOffset)
	appendBytes(0x85, 0xDB)
	nonPositiveJump := position
	appendBytes(0x0F, 0x8E, 0, 0, 0, 0)
	appendBytes(0x81, 0xFB, 0xE7, 0x03, 0x00, 0x00)
	clampJump := position
	appendBytes(0x0F, 0x8D, 0, 0, 0, 0)
	appendBytes(0x0F, 0xAF, 0x1D)
	multiplierDisp := position
	position += 4
	appendBytes(0x81, 0xFB, 0xE7, 0x03, 0x00, 0x00)
	withinMaximumJump := position
	appendBytes(0x0F, 0x8E, 0, 0, 0, 0)
	clampOffset := position
	appendBytes(0xBB, 0xE7, 0x03, 0x00, 0x00)
	storeOffset := position
	appendBytes(0x89, 0x58, taskRewardMultiplierQuantityOffset)
	appendBytes(0x48, 0xFF, 0x05)
	counterDisp := position
	position += 4
	nextOffset := position
	appendBytes(0x48, 0x83, 0xC0, taskRewardMultiplierRecordSize)
	appendBytes(0xE9)
	loopDisp := position
	position += 4
	doneOffset := position
	appendBytes(0x41, 0x59, 0x5A, 0x5B, 0x58, 0x9D)
	appendBytes(0x48, 0xB9)
	binary.LittleEndian.PutUint64(code[position:position+8], uint64(managerSlot))
	position += 8
	appendBytes(0x48, 0x8B, 0x09, 0x31, 0xF6, 0x31, 0xD2, 0x45, 0x31, 0xC0)
	if err := appendRel32([]byte{0xE9}, returnAddr); err != nil {
		return nil, err
	}

	writeRel := func(at, next int, target int) {
		binary.LittleEndian.PutUint32(code[at:at+4], uint32(int32(target-next)))
	}
	writeRel(doneJump+2, doneJump+6, doneOffset)
	writeRel(nextJump+2, nextJump+6, nextOffset)
	writeRel(nonPositiveJump+2, nonPositiveJump+6, nextOffset)
	writeRel(clampJump+2, clampJump+6, clampOffset)
	writeRel(withinMaximumJump+2, withinMaximumJump+6, storeOffset)
	writeRel(loopDisp, loopDisp+4, loopOffset)
	writeRIP := func(at int, target uintptr) error {
		delta := int64(target) - int64(cave+uintptr(at)+4)
		if delta < -0x80000000 || delta > 0x7fffffff {
			return fmt.Errorf("task reward multiplier data reference is out of range")
		}
		binary.LittleEndian.PutUint32(code[at:at+4], uint32(int32(delta)))
		return nil
	}
	if err := writeRIP(multiplierDisp, cave+taskRewardMultiplierValueOffset); err != nil {
		return nil, err
	}
	if err := writeRIP(counterDisp, cave+taskRewardMultiplierCounterOffset); err != nil {
		return nil, err
	}
	copy(code[taskRewardMultiplierMarkerOffset:], taskRewardMultiplierMarker[:])
	binary.LittleEndian.PutUint32(code[taskRewardMultiplierValueOffset:], uint32(multiplier))
	binary.LittleEndian.PutUint64(code[taskRewardMultiplierCounterOffset:], matched)
	return code, nil
}

func taskRewardMultiplierManagerSlot(entry uintptr, original []byte) (uintptr, error) {
	if len(original) != taskRewardMultiplierHookSize || !bytes.Equal(original[:3], []byte{0x48, 0x8B, 0x0D}) ||
		!bytes.Equal(original[7:], []byte{0x31, 0xF6, 0x31, 0xD2, 0x45, 0x31, 0xC0}) {
		return 0, fmt.Errorf("task reward multiplier entry bytes do not match the audited boundary")
	}
	displacement := int32(binary.LittleEndian.Uint32(original[3:7]))
	return uintptr(int64(entry+7) + int64(displacement)), nil
}

func (a *App) TaskRewardMultiplierStatusOwned(token string) (TaskRewardMultiplierStatus, error) {
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, token); err != nil {
		return TaskRewardMultiplierStatus{}, err
	}
	defer a.procMu.Unlock()
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	return a.readTaskRewardMultiplierStatusLocked(token)
}

func (a *App) TaskRewardMultiplierSetOwned(token string, multiplier int) (TaskRewardMultiplierStatus, error) {
	if !validTaskRewardMultiplier(multiplier) {
		return TaskRewardMultiplierStatus{}, errors.New("任务奖励倍率必须是 1、2、4、8 或 16")
	}
	liveMemoryWriteMu.Lock()
	defer liveMemoryWriteMu.Unlock()
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, token); err != nil {
		return TaskRewardMultiplierStatus{}, err
	}
	defer a.procMu.Unlock()
	if err := a.ensureLiveMemoryWritesSafe(); err != nil {
		return TaskRewardMultiplierStatus{}, err
	}
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	if multiplier == 1 {
		if err := a.restoreTaskRewardMultiplierOwnedLocked(token, false); err != nil {
			return TaskRewardMultiplierStatus{}, err
		}
		return TaskRewardMultiplierStatus{Multiplier: 1, GameVersion: a.currentRuntimeGameVersionLocked(), Detail: "任务奖励倍率已恢复为游戏默认"}, nil
	}
	if a.taskRewardMultiplierLease != nil {
		status, err := a.readTaskRewardMultiplierStatusLocked(token)
		if err != nil {
			return TaskRewardMultiplierStatus{}, err
		}
		if status.Multiplier == multiplier {
			return status, nil
		}
		if err := a.restoreTaskRewardMultiplierOwnedLocked(token, false); err != nil {
			return TaskRewardMultiplierStatus{}, err
		}
	}
	return a.enableTaskRewardMultiplierLocked(token, multiplier)
}

func (a *App) currentRuntimeGameVersionLocked() string {
	if layout, err := detectRuntimeGameLayout(remoteRuntimePatchPartyMemory{app: a}, a.moduleBase); err == nil {
		return layout.Version
	}
	return ""
}

func (a *App) enableTaskRewardMultiplierLocked(token string, multiplier int) (TaskRewardMultiplierStatus, error) {
	if err := a.verifyRuntimePatchExecutableLocked(a.currentProcessInstance(), "任务奖励倍率"); err != nil {
		return TaskRewardMultiplierStatus{}, err
	}
	entry, err := a.scanPatternUnique(taskRewardMultiplierPattern, taskRewardMultiplierMask, "任务结算奖励提交边界")
	if err != nil {
		return TaskRewardMultiplierStatus{}, err
	}
	original := make([]byte, taskRewardMultiplierHookSize)
	if err := readProcessMemory(a.hProcess, entry, unsafe.Pointer(&original[0]), uintptr(len(original))); err != nil {
		return TaskRewardMultiplierStatus{}, err
	}
	managerSlot, err := taskRewardMultiplierManagerSlot(entry, original)
	if err != nil {
		return TaskRewardMultiplierStatus{}, err
	}
	cave, err := virtualAllocRemoteNear(a.hProcess, entry, taskRewardMultiplierCaveSize)
	if err != nil {
		return TaskRewardMultiplierStatus{}, err
	}
	code, err := buildTaskRewardMultiplierCave(cave, entry+taskRewardMultiplierHookSize, managerSlot, multiplier, 0)
	if err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return TaskRewardMultiplierStatus{}, err
	}
	if err := writeProcessMemory(a.hProcess, cave, unsafe.Pointer(&code[0]), uintptr(len(code))); err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return TaskRewardMultiplierStatus{}, err
	}
	confirmed := make([]byte, len(code))
	if err := readProcessMemory(a.hProcess, cave, unsafe.Pointer(&confirmed[0]), uintptr(len(confirmed))); err != nil || !bytes.Equal(confirmed, code) {
		_ = virtualFreeRemote(a.hProcess, cave)
		return TaskRewardMultiplierStatus{}, fmt.Errorf("任务奖励倍率代码洞写后回读失败: %v", err)
	}
	patch, err := makeRelJump(entry, cave, taskRewardMultiplierHookSize)
	if err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return TaskRewardMultiplierStatus{}, err
	}
	lease := &taskRewardMultiplierLease{OwnerToken: token, Process: a.currentProcessInstance(), EntryAddr: entry, Original: original, Installed: patch, CaveAddr: cave, Multiplier: multiplier}
	result, installErr := installRemoteCodeHook(a.hProcess, entry, original, patch)
	if installErr != nil {
		return TaskRewardMultiplierStatus{}, runtimeHookInstallFailure(
			"任务奖励倍率", result, installErr,
			func() { _ = virtualFreeRemote(a.hProcess, cave) },
			func() { a.retireRuntimeCaveLocked(cave, "task reward multiplier install rollback") },
			func() { a.taskRewardMultiplierLease = lease },
			func() { a.poisonCurrentLiveMemoryWrites() },
		)
	}
	a.taskRewardMultiplierLease = lease
	return finalizeRuntimeHookEnable(
		"任务奖励倍率",
		func() (TaskRewardMultiplierStatus, error) {
			status, readErr := a.readTaskRewardMultiplierStatusLocked(token)
			if readErr == nil && !status.Enabled {
				readErr = fmt.Errorf("任务奖励倍率未进入启用状态")
			}
			return status, readErr
		},
		func() error { return a.restoreTaskRewardMultiplierOwnedLocked(token, false) },
		a.poisonCurrentLiveMemoryWrites,
	)
}

func (a *App) readTaskRewardMultiplierStatusLocked(token string) (TaskRewardMultiplierStatus, error) {
	lease := a.taskRewardMultiplierLease
	if lease == nil {
		return TaskRewardMultiplierStatus{Multiplier: 1, GameVersion: a.currentRuntimeGameVersionLocked(), Detail: "任务奖励倍率未开启"}, nil
	}
	if lease.OwnerToken != token || !sameProcessInstance(lease.Process, a.currentProcessInstance()) {
		return TaskRewardMultiplierStatus{}, errRuntimeOwnerLeaseStale
	}
	entry := make([]byte, len(lease.Installed))
	if err := readProcessMemory(a.hProcess, lease.EntryAddr, unsafe.Pointer(&entry[0]), uintptr(len(entry))); err != nil {
		return TaskRewardMultiplierStatus{}, err
	}
	if !bytes.Equal(entry, lease.Installed) || relJumpTarget(lease.EntryAddr, entry) != lease.CaveAddr {
		return TaskRewardMultiplierStatus{}, fmt.Errorf("任务奖励倍率入口已被外部修改: %s", bytesToHex(entry))
	}
	cave := make([]byte, taskRewardMultiplierCaveSize)
	if err := readProcessMemory(a.hProcess, lease.CaveAddr, unsafe.Pointer(&cave[0]), uintptr(len(cave))); err != nil {
		return TaskRewardMultiplierStatus{}, err
	}
	matched := binary.LittleEndian.Uint64(cave[taskRewardMultiplierCounterOffset:])
	managerSlot, err := taskRewardMultiplierManagerSlot(lease.EntryAddr, lease.Original)
	if err != nil {
		return TaskRewardMultiplierStatus{}, err
	}
	expected, err := buildTaskRewardMultiplierCave(lease.CaveAddr, lease.EntryAddr+taskRewardMultiplierHookSize, managerSlot, lease.Multiplier, matched)
	if err != nil || !bytes.Equal(cave, expected) {
		return TaskRewardMultiplierStatus{}, fmt.Errorf("任务奖励倍率代码洞所有权校验失败")
	}
	return TaskRewardMultiplierStatus{
		Enabled: true, Multiplier: lease.Multiplier, MatchedItems: matched,
		RVA: uint64(lease.EntryAddr - a.moduleBase), GameVersion: a.currentRuntimeGameVersionLocked(),
		Detail: "只放大任务结算中的普通可堆叠物品；因子、祝福石、武器等实例奖励保持原数量",
	}, nil
}

func (a *App) restoreTaskRewardMultiplierOwnedLocked(owner string, force bool) error {
	lease := a.taskRewardMultiplierLease
	if lease == nil {
		return nil
	}
	if !force && lease.OwnerToken != owner {
		return errRuntimeOwnerLeaseStale
	}
	if !sameProcessInstance(lease.Process, a.currentProcessInstance()) {
		return errRuntimeOwnerLeaseStale
	}
	entry := make([]byte, len(lease.Installed))
	if err := readProcessMemory(a.hProcess, lease.EntryAddr, unsafe.Pointer(&entry[0]), uintptr(len(entry))); err != nil {
		return err
	}
	if !bytes.Equal(entry, lease.Original) {
		if !bytes.Equal(entry, lease.Installed) || relJumpTarget(lease.EntryAddr, entry) != lease.CaveAddr {
			return fmt.Errorf("任务奖励倍率恢复前入口字节未知: %s", bytesToHex(entry))
		}
		if err := writeAndVerifyRuntimeHookEntry(a.hProcess, lease.EntryAddr, lease.Original); err != nil {
			return err
		}
	}
	a.retireRuntimeCaveLocked(lease.CaveAddr, "task reward multiplier release")
	a.taskRewardMultiplierLease = nil
	return nil
}

func (a *App) dropTaskRewardMultiplierOwnerLocked(owner string) {
	if a.taskRewardMultiplierLease != nil && a.taskRewardMultiplierLease.OwnerToken == owner {
		a.taskRewardMultiplierLease = nil
	}
}
