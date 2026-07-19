package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// CT 0.8.4 node 33552 exposes two independent highlighted-item paths. This
// implementation translates only the address capture: it never writes the
// record at +0/+4/+8 and never invokes a game save function.
const (
	ct084SelectedMaterialAOB = "488Bxxxx4885xx74xx448Bxxxx4889xxE8xxxxxxxx498B"
	ct084SelectedKeyItemAOB  = "448Bxxxx4889xx89xxE8xxxxxxxx488Bxxxxxxxxxx488Bxxxxxxxxxx4839"

	ct084SelectedMaterialRVA = uintptr(0x3F4BAC3)
	ct084SelectedKeyItemRVA  = uintptr(0x3F2061C)
	ct084SelectedHookSize    = 7

	ct084SelectedItemRecordSize    = 0x0C
	ct084SelectedCaveMarkerOffset  = uintptr(0x30)
	ct084SelectedCaveKindOffset    = uintptr(0x38)
	ct084SelectedCavePIDOffset     = uintptr(0x3C)
	ct084SelectedCaveCreatedOffset = uintptr(0x40)
	ct084SelectedCaveOwnerOffset   = uintptr(0x48)
	ct084SelectedCaveDataOffset    = uintptr(0x50)
)

var (
	ct084SelectedMaterialOriginal = []byte{0x48, 0x8B, 0x41, 0x18, 0x48, 0x85, 0xC0}
	ct084SelectedKeyItemOriginal  = []byte{0x44, 0x8B, 0x40, 0x04, 0x48, 0x89, 0xD9}
	ct084SelectedCaveMarker       = [...]byte{'G', 'B', 'F', 'R', 'I', 'T', 'M', '1'}
	ct084SelectedInstallHook      = installRemoteCodeHook
)

type CT084SelectedItemKind string

const (
	CT084SelectedItemMaterial CT084SelectedItemKind = "material"
	CT084SelectedItemKeyItem  CT084SelectedItemKind = "keyItem"
)

type CT084SelectedItemCapture struct {
	Kind         CT084SelectedItemKind `json:"kind"`
	DisplayName  string                `json:"displayName"`
	Found        bool                  `json:"found"`
	Hooked       bool                  `json:"hooked"`
	Address      uint64                `json:"address"`
	RVA          uint64                `json:"rva"`
	SelectedAddr uint64                `json:"selectedAddr"`
	Captured     bool                  `json:"captured"`
}

type CT084SelectedItemsStatus struct {
	OwnerToken     string                   `json:"ownerToken,omitempty"`
	PID            uint32                   `json:"pid"`
	ProcessCreated uint64                   `json:"processCreated"`
	Enabled        bool                     `json:"enabled"`
	ReadOnly       bool                     `json:"readOnly"`
	GameVersion    string                   `json:"gameVersion"`
	Source         string                   `json:"source"`
	Material       CT084SelectedItemCapture `json:"material"`
	KeyItem        CT084SelectedItemCapture `json:"keyItem"`
}

type CT084SelectedItemReadRequest struct {
	Kind                 CT084SelectedItemKind `json:"kind"`
	ExpectedSelectedAddr uint64                `json:"expectedSelectedAddr"`
}

type CT084SelectedItemRecord struct {
	Kind         CT084SelectedItemKind `json:"kind"`
	DisplayName  string                `json:"displayName"`
	SelectedAddr uint64                `json:"selectedAddr"`
	Hash         uint32                `json:"hash"`
	HashHex      string                `json:"hashHex"`
	Name         string                `json:"name"`
	Category     string                `json:"category,omitempty"`
	Quantity     uint32                `json:"quantity"`
	Flags        uint32                `json:"flags"`
	FlagsHex     string                `json:"flagsHex"`
	ReadOnly     bool                  `json:"readOnly"`
	GameVersion  string                `json:"gameVersion"`
}

type ct084SelectedCaptureLease struct {
	Kind       CT084SelectedItemKind
	HookAddr   uintptr
	CaveAddr   uintptr
	Original   []byte
	Process    processInstanceID
	OwnerToken string
}

func (lease ct084SelectedCaptureLease) active() bool {
	return lease.Kind != "" || lease.HookAddr != 0 || lease.CaveAddr != 0 || len(lease.Original) != 0 || lease.Process.PID != 0 || lease.OwnerToken != ""
}

type ct084SelectedItemMemory interface {
	ReadAt(address uintptr, destination []byte) error
	WriteAt(address uintptr, source []byte) error
}

type remoteCT084SelectedItemMemory struct {
	handle windows.Handle
}

func (memory remoteCT084SelectedItemMemory) ReadAt(address uintptr, destination []byte) error {
	if memory.handle == 0 {
		return fmt.Errorf("game process handle is empty")
	}
	if len(destination) == 0 {
		return nil
	}
	return readProcessMemory(memory.handle, address, unsafe.Pointer(&destination[0]), uintptr(len(destination)))
}

func (memory remoteCT084SelectedItemMemory) WriteAt(address uintptr, source []byte) error {
	if memory.handle == 0 {
		return fmt.Errorf("game process handle is empty")
	}
	if len(source) == 0 {
		return nil
	}
	return writeProcessMemory(memory.handle, address, unsafe.Pointer(&source[0]), uintptr(len(source)))
}

func (a *App) CT084SelectedItemsEnableOwned(token string) (CT084SelectedItemsStatus, error) {
	liveMemoryWriteMu.Lock()
	defer liveMemoryWriteMu.Unlock()
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, token); err != nil {
		return CT084SelectedItemsStatus{}, err
	}
	defer a.procMu.Unlock()
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	return a.enableCT084SelectedItemsLocked(token)
}

func (a *App) CT084SelectedItemsStatusOwned(token string) (CT084SelectedItemsStatus, error) {
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, token); err != nil {
		return CT084SelectedItemsStatus{}, err
	}
	defer a.procMu.Unlock()
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	return a.readCT084SelectedItemsStatusLocked(token)
}

func (a *App) CT084SelectedItemReadOwned(token string, request CT084SelectedItemReadRequest) (CT084SelectedItemRecord, error) {
	liveMemoryWriteMu.Lock()
	defer liveMemoryWriteMu.Unlock()
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, token); err != nil {
		return CT084SelectedItemRecord{}, err
	}
	defer a.procMu.Unlock()
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()

	lease, err := a.ct084SelectedLeaseForRequestLocked(token, request.Kind)
	if err != nil {
		return CT084SelectedItemRecord{}, err
	}
	memory := remoteCT084SelectedItemMemory{handle: a.hProcess}
	record, err := consumeCT084SelectedItemRecord(memory, lease.CaveAddr, request)
	if err != nil {
		return CT084SelectedItemRecord{}, err
	}
	decorateCT084SelectedItemRecord(&record)
	return record, nil
}

func (a *App) CT084SelectedItemsDisableOwned(token string) (CT084SelectedItemsStatus, error) {
	liveMemoryWriteMu.Lock()
	defer liveMemoryWriteMu.Unlock()
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, token); err != nil {
		return CT084SelectedItemsStatus{}, err
	}
	defer a.procMu.Unlock()
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	if err := a.releaseCT084SelectedCaptureHooksLocked(token, false); err != nil {
		return CT084SelectedItemsStatus{}, err
	}
	return a.readCT084SelectedItemsStatusLocked(token)
}

func (a *App) enableCT084SelectedItemsLocked(token string) (CT084SelectedItemsStatus, error) {
	if a.ct084SelectedMaterialHook.active() || a.ct084SelectedKeyItemHook.active() {
		status, err := a.readCT084SelectedItemsStatusLocked(token)
		if err == nil && status.Enabled {
			return status, nil
		}
		if releaseErr := a.releaseCT084SelectedCaptureHooksLocked(token, false); releaseErr != nil {
			return CT084SelectedItemsStatus{}, errors.Join(err, fmt.Errorf("clear previous selected-item recovery lease: %w", releaseErr))
		}
	}

	materialAddress, materialOriginal, err := a.locateCT084SelectedHookLocked(CT084SelectedItemMaterial)
	if err != nil {
		return CT084SelectedItemsStatus{}, err
	}
	keyAddress, keyOriginal, err := a.locateCT084SelectedHookLocked(CT084SelectedItemKeyItem)
	if err != nil {
		return CT084SelectedItemsStatus{}, err
	}
	process := a.currentProcessInstance()
	materialLease, err := a.prepareCT084SelectedCaptureLease(CT084SelectedItemMaterial, materialAddress, materialOriginal, process, token)
	if err != nil {
		return CT084SelectedItemsStatus{}, err
	}
	keyLease, err := a.prepareCT084SelectedCaptureLease(CT084SelectedItemKeyItem, keyAddress, keyOriginal, process, token)
	if err != nil {
		_ = virtualFreeRemote(a.hProcess, materialLease.CaveAddr)
		return CT084SelectedItemsStatus{}, err
	}

	materialPatch, err := makeRelJump(materialLease.HookAddr, materialLease.CaveAddr, ct084SelectedHookSize)
	if err != nil {
		_ = virtualFreeRemote(a.hProcess, materialLease.CaveAddr)
		_ = virtualFreeRemote(a.hProcess, keyLease.CaveAddr)
		return CT084SelectedItemsStatus{}, err
	}
	canFree, err := ct084SelectedInstallHook(a.hProcess, materialLease.HookAddr, materialLease.Original, materialPatch)
	if err != nil {
		_ = virtualFreeRemote(a.hProcess, keyLease.CaveAddr)
		return CT084SelectedItemsStatus{}, runtimeHookInstallFailure(
			ct084MonitorText("选中素材捕获 Hook", "Selected-material capture hook"), canFree, err,
			func() { _ = virtualFreeRemote(a.hProcess, materialLease.CaveAddr) },
			func() { a.ct084SelectedMaterialHook = materialLease },
			a.poisonCurrentLiveMemoryWrites,
		)
	}
	a.ct084SelectedMaterialHook = materialLease

	canFree, err = installPreparedCT084SelectedHook(a.hProcess, keyLease, makeRelJump, ct084SelectedInstallHook)
	if err != nil {
		var installErr error
		if canFree {
			_ = virtualFreeRemote(a.hProcess, keyLease.CaveAddr)
			installErr = fmt.Errorf("%s: %w", ct084MonitorText("选中关键物品捕获 Hook 安装失败", "Selected-key-item capture hook install failed"), err)
		} else {
			a.ct084SelectedKeyItemHook = keyLease
			a.poisonCurrentLiveMemoryWrites()
			installErr = errors.Join(fmt.Errorf("%s: %w", ct084MonitorText("选中关键物品捕获 Hook 安装失败", "Selected-key-item capture hook install failed"), err), errRuntimeHookRollbackUnproven)
		}
		rollbackErr := a.releaseCT084SelectedCaptureHookLocked(&a.ct084SelectedMaterialHook, token, false)
		if rollbackErr != nil {
			a.poisonCurrentLiveMemoryWrites()
		}
		return CT084SelectedItemsStatus{}, errors.Join(installErr, rollbackErr)
	}
	a.ct084SelectedKeyItemHook = keyLease

	status, verifyErr := a.readCT084SelectedItemsStatusLocked(token)
	if verifyErr == nil && status.Enabled {
		return status, nil
	}
	rollbackErr := a.releaseCT084SelectedCaptureHooksLocked(token, false)
	if rollbackErr != nil {
		a.poisonCurrentLiveMemoryWrites()
		return CT084SelectedItemsStatus{}, errors.Join(verifyErr, errRuntimeHookRollbackUnproven, rollbackErr)
	}
	if verifyErr == nil {
		verifyErr = fmt.Errorf("selected-item hook pair did not become enabled")
	}
	return CT084SelectedItemsStatus{}, fmt.Errorf("selected-item hook verification failed; both entries restored: %w", verifyErr)
}

func installPreparedCT084SelectedHook(
	handle windows.Handle,
	lease ct084SelectedCaptureLease,
	buildJump func(uintptr, uintptr, int) ([]byte, error),
	install func(windows.Handle, uintptr, []byte, []byte) (bool, error),
) (bool, error) {
	if buildJump == nil || install == nil {
		return true, fmt.Errorf("selected-item hook installer is incomplete")
	}
	patch, err := buildJump(lease.HookAddr, lease.CaveAddr, ct084SelectedHookSize)
	if err != nil {
		// No entry write was attempted, so the prepared but unreachable cave is
		// always safe for the caller to free.
		return true, err
	}
	return install(handle, lease.HookAddr, lease.Original, patch)
}

func (a *App) prepareCT084SelectedCaptureLease(kind CT084SelectedItemKind, hook uintptr, original []byte, process processInstanceID, token string) (ct084SelectedCaptureLease, error) {
	cave, err := virtualAllocRemoteNear(a.hProcess, hook, 0x1000)
	if err != nil {
		return ct084SelectedCaptureLease{}, fmt.Errorf("%s: %w", ct084MonitorText("分配选中物品捕获代码洞失败", "Allocate selected-item capture cave"), err)
	}
	code, err := buildCT084SelectedCaptureCave(kind, cave, hook+ct084SelectedHookSize, original, process, token)
	if err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return ct084SelectedCaptureLease{}, err
	}
	if err := writeCodeMemory(a.hProcess, cave, code); err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return ct084SelectedCaptureLease{}, fmt.Errorf("%s: %w", ct084MonitorText("写入选中物品捕获代码洞失败", "Write selected-item capture cave"), err)
	}
	written := make([]byte, len(code))
	if err := readProcessMemory(a.hProcess, cave, unsafe.Pointer(&written[0]), uintptr(len(written))); err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return ct084SelectedCaptureLease{}, fmt.Errorf("%s: %w", ct084MonitorText("回读选中物品捕获代码洞失败", "Read back selected-item capture cave"), err)
	}
	if !bytes.Equal(written, code) {
		_ = virtualFreeRemote(a.hProcess, cave)
		return ct084SelectedCaptureLease{}, fmt.Errorf("%s", ct084MonitorText("选中物品捕获代码洞写后回读不一致", "Selected-item capture cave readback mismatch"))
	}
	if err := validateCT084SelectedCaptureCaveBytes(kind, cave, hook+ct084SelectedHookSize, original, process, token, written); err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return ct084SelectedCaptureLease{}, err
	}
	return ct084SelectedCaptureLease{Kind: kind, HookAddr: hook, CaveAddr: cave, Original: append([]byte(nil), original...), Process: process, OwnerToken: token}, nil
}

func (a *App) locateCT084SelectedHookLocked(kind CT084SelectedItemKind) (uintptr, []byte, error) {
	rawPattern, rva, expected, err := ct084SelectedHookDefinition(kind)
	if err != nil {
		return 0, nil, err
	}
	pattern, err := parseCT084Pattern(rawPattern)
	if err != nil {
		return 0, nil, err
	}
	address, ok := checkedCT084MonitorAddress(a.moduleBase, rva)
	if !ok {
		return 0, nil, fmt.Errorf("selected-item hook address overflow")
	}
	actualPattern := make([]byte, len(pattern.Values))
	if err := readProcessMemory(a.hProcess, address, unsafe.Pointer(&actualPattern[0]), uintptr(len(actualPattern))); err != nil {
		return 0, nil, fmt.Errorf("%s: %w", ct084SelectedKindName(kind), err)
	}
	if !matchCT084Pattern(actualPattern, pattern) {
		return 0, nil, fmt.Errorf("%s: %s RVA 0x%X", ct084SelectedKindName(kind), ct084MonitorText("签名与游戏 2.0.2 不匹配", "signature does not match game 2.0.2"), rva)
	}
	if !bytes.Equal(actualPattern[:ct084SelectedHookSize], expected) {
		return 0, nil, fmt.Errorf("%s: %s %s", ct084SelectedKindName(kind), ct084MonitorText("入口原始字节不匹配", "entry bytes mismatch"), bytesToHex(actualPattern[:ct084SelectedHookSize]))
	}
	return address, append([]byte(nil), expected...), nil
}

func (a *App) readCT084SelectedItemsStatusLocked(token string) (CT084SelectedItemsStatus, error) {
	result := CT084SelectedItemsStatus{
		OwnerToken: token, PID: a.charaPID, ProcessCreated: a.charaCreated,
		ReadOnly: true, GameVersion: "2.0.2", Source: "ct084_node_33552_read_only",
	}
	material, err := a.readCT084SelectedCaptureStatusLocked(token, CT084SelectedItemMaterial, &a.ct084SelectedMaterialHook)
	if err != nil {
		return CT084SelectedItemsStatus{}, err
	}
	keyItem, err := a.readCT084SelectedCaptureStatusLocked(token, CT084SelectedItemKeyItem, &a.ct084SelectedKeyItemHook)
	if err != nil {
		return CT084SelectedItemsStatus{}, err
	}
	result.Material = material
	result.KeyItem = keyItem
	result.Enabled = material.Hooked && keyItem.Hooked
	return result, nil
}

func (a *App) readCT084SelectedCaptureStatusLocked(token string, kind CT084SelectedItemKind, lease *ct084SelectedCaptureLease) (CT084SelectedItemCapture, error) {
	result := CT084SelectedItemCapture{Kind: kind, DisplayName: ct084SelectedKindName(kind)}
	_, rva, _, definitionErr := ct084SelectedHookDefinition(kind)
	if definitionErr != nil {
		return result, definitionErr
	}
	result.RVA = uint64(rva)
	if lease == nil || !lease.active() {
		address, _, err := a.locateCT084SelectedHookLocked(kind)
		if err != nil {
			return result, err
		}
		result.Found = true
		result.Address = uint64(address)
		return result, nil
	}
	if lease.Kind != kind || !sameProcessInstance(lease.Process, a.currentProcessInstance()) || lease.OwnerToken != token || !runtimeOwnerTokenMatches(a.charaOwnerToken, token) {
		return result, fmt.Errorf("%s", ct084MonitorText("选中物品捕获租约的所有者或进程实例已失效", "Selected-item capture lease owner or process instance is stale"))
	}
	entry := make([]byte, ct084SelectedHookSize)
	if err := readProcessMemory(a.hProcess, lease.HookAddr, unsafe.Pointer(&entry[0]), uintptr(len(entry))); err != nil {
		return result, err
	}
	result.Found = true
	result.Address = uint64(lease.HookAddr)
	if bytes.Equal(entry, lease.Original) {
		return result, nil
	}
	if !isCT084SelectedJump(entry) || relJumpTarget(lease.HookAddr, entry) != lease.CaveAddr {
		return result, fmt.Errorf("%s", ct084MonitorText("选中物品 Hook 入口已被外部修改", "Selected-item hook entry was replaced externally"))
	}
	if err := a.validateCT084SelectedCaptureCaveLocked(*lease); err != nil {
		return result, err
	}
	selected, err := readCT084SelectedPointer(remoteCT084SelectedItemMemory{handle: a.hProcess}, lease.CaveAddr+ct084SelectedCaveDataOffset)
	if err != nil {
		return result, err
	}
	result.Hooked = true
	result.SelectedAddr = uint64(selected)
	result.Captured = selected != 0
	return result, nil
}

func (a *App) ct084SelectedLeaseForRequestLocked(token string, kind CT084SelectedItemKind) (*ct084SelectedCaptureLease, error) {
	var lease *ct084SelectedCaptureLease
	switch kind {
	case CT084SelectedItemMaterial:
		lease = &a.ct084SelectedMaterialHook
	case CT084SelectedItemKeyItem:
		lease = &a.ct084SelectedKeyItemHook
	default:
		return nil, fmt.Errorf("%s: %q", ct084MonitorText("未知选中物品类型", "Unknown selected-item kind"), kind)
	}
	status, err := a.readCT084SelectedCaptureStatusLocked(token, kind, lease)
	if err != nil {
		return nil, err
	}
	if !status.Hooked || !lease.active() {
		return nil, fmt.Errorf("%s", ct084MonitorText("选中物品只读捕获尚未启用", "Selected-item read-only capture is not enabled"))
	}
	return lease, nil
}

func consumeCT084SelectedItemRecord(memory ct084SelectedItemMemory, cave uintptr, request CT084SelectedItemReadRequest) (CT084SelectedItemRecord, error) {
	if memory == nil || cave == 0 || (request.Kind != CT084SelectedItemMaterial && request.Kind != CT084SelectedItemKeyItem) || request.ExpectedSelectedAddr == 0 {
		return CT084SelectedItemRecord{}, fmt.Errorf("invalid selected-item read request")
	}
	expected := uintptr(request.ExpectedSelectedAddr)
	if uint64(expected) != request.ExpectedSelectedAddr || expected > ^uintptr(0)-(ct084SelectedItemRecordSize-1) {
		return CT084SelectedItemRecord{}, fmt.Errorf("ExpectedSelectedAddr is outside the local address range")
	}
	captured, err := readCT084SelectedPointer(memory, cave+ct084SelectedCaveDataOffset)
	if err != nil {
		return CT084SelectedItemRecord{}, fmt.Errorf("read selected-item capture: %w", err)
	}
	if captured == 0 || captured != expected {
		return CT084SelectedItemRecord{}, fmt.Errorf("ExpectedSelectedAddr mismatch: captured=0x%X expected=0x%X; select the item again", captured, expected)
	}
	first := make([]byte, ct084SelectedItemRecordSize)
	if err := memory.ReadAt(expected, first); err != nil {
		return CT084SelectedItemRecord{}, fmt.Errorf("read selected-item record: %w", err)
	}
	confirmedPointer, err := readCT084SelectedPointer(memory, cave+ct084SelectedCaveDataOffset)
	if err != nil {
		return CT084SelectedItemRecord{}, fmt.Errorf("revalidate selected-item capture: %w", err)
	}
	if confirmedPointer != expected {
		return CT084SelectedItemRecord{}, fmt.Errorf("selected-item pointer changed during read: got=0x%X want=0x%X", confirmedPointer, expected)
	}
	confirmed := make([]byte, ct084SelectedItemRecordSize)
	if err := memory.ReadAt(expected, confirmed); err != nil {
		return CT084SelectedItemRecord{}, fmt.Errorf("revalidate selected-item record: %w", err)
	}
	if !bytes.Equal(first, confirmed) {
		return CT084SelectedItemRecord{}, fmt.Errorf("selected-item record changed during full 0x0C revalidation")
	}
	if err := clearCT084SelectedPointer(memory, cave+ct084SelectedCaveDataOffset); err != nil {
		return CT084SelectedItemRecord{}, err
	}
	return CT084SelectedItemRecord{
		Kind: request.Kind, DisplayName: ct084SelectedKindName(request.Kind), SelectedAddr: uint64(expected),
		Hash: binary.LittleEndian.Uint32(confirmed[0:4]), Quantity: binary.LittleEndian.Uint32(confirmed[4:8]), Flags: binary.LittleEndian.Uint32(confirmed[8:12]),
		ReadOnly: true, GameVersion: "2.0.2",
	}, nil
}

func readCT084SelectedPointer(memory interface{ ReadAt(uintptr, []byte) error }, address uintptr) (uintptr, error) {
	encoded := make([]byte, 8)
	if err := memory.ReadAt(address, encoded); err != nil {
		return 0, err
	}
	value := binary.LittleEndian.Uint64(encoded)
	if uint64(uintptr(value)) != value {
		return 0, fmt.Errorf("selected-item pointer is outside the local address width: 0x%X", value)
	}
	return uintptr(value), nil
}

func clearCT084SelectedPointer(memory ct084SelectedItemMemory, address uintptr) error {
	zero := make([]byte, 8)
	if err := memory.WriteAt(address, zero); err != nil {
		return fmt.Errorf("clear selected-item capture: %w", err)
	}
	verified := make([]byte, 8)
	if err := memory.ReadAt(address, verified); err != nil {
		return fmt.Errorf("verify cleared selected-item capture: %w", err)
	}
	if !bytes.Equal(verified, zero) {
		return fmt.Errorf("selected-item capture clear readback mismatch: %s", bytesToHex(verified))
	}
	return nil
}

func decorateCT084SelectedItemRecord(record *CT084SelectedItemRecord) {
	if record == nil {
		return
	}
	record.HashHex = fmt.Sprintf("%08X", record.Hash)
	record.FlagsHex = fmt.Sprintf("%08X", record.Flags)
	record.Name = fmt.Sprintf("0x%08X", record.Hash)
	if _, err := loadProgressionCatalog(); err == nil {
		if definition, ok := progressionItemByHash[record.Hash]; ok {
			record.Name = progressionItemName(definition)
			record.Category = definition.Category
		}
	}
}

func buildCT084SelectedCaptureCave(kind CT084SelectedItemKind, cave, returnAddr uintptr, original []byte, process processInstanceID, ownerToken string) ([]byte, error) {
	_, _, expected, err := ct084SelectedHookDefinition(kind)
	if err != nil {
		return nil, err
	}
	if len(original) != ct084SelectedHookSize || !bytes.Equal(original, expected) || cave == 0 || returnAddr == 0 || process.PID == 0 || process.Created == 0 || strings.TrimSpace(ownerToken) == "" {
		return nil, fmt.Errorf("invalid selected-item capture cave parameters")
	}
	code := make([]byte, 0, ct084SelectedCaveDataOffset+8)
	code = append(code, original...)
	// RAX is the selected record pointer for both CT paths. MOV moffs64,RAX is
	// register- and flag-preserving. On the material path it executes after
	// test rax,rax and unconditionally writes zero before the displaced JE.
	code = append(code, 0x48, 0xA3)
	code = binary.LittleEndian.AppendUint64(code, uint64(cave+ct084SelectedCaveDataOffset))
	jump, err := makeRelJump(cave+uintptr(len(code)), returnAddr, 5)
	if err != nil {
		return nil, err
	}
	code = append(code, jump...)
	for len(code) < int(ct084SelectedCaveDataOffset)+8 {
		code = append(code, 0)
	}
	copy(code[ct084SelectedCaveMarkerOffset:], ct084SelectedCaveMarker[:])
	code[ct084SelectedCaveKindOffset] = ct084SelectedKindByte(kind)
	binary.LittleEndian.PutUint32(code[ct084SelectedCavePIDOffset:ct084SelectedCavePIDOffset+4], process.PID)
	binary.LittleEndian.PutUint64(code[ct084SelectedCaveCreatedOffset:ct084SelectedCaveCreatedOffset+8], process.Created)
	binary.LittleEndian.PutUint64(code[ct084SelectedCaveOwnerOffset:ct084SelectedCaveOwnerOffset+8], ct084SelectedOwnerFingerprint(ownerToken))
	return code, nil
}

func validateCT084SelectedCaptureCaveBytes(kind CT084SelectedItemKind, cave, returnAddr uintptr, original []byte, process processInstanceID, ownerToken string, code []byte) error {
	minimum := int(ct084SelectedCaveDataOffset) + 8
	if len(code) < minimum || len(original) != ct084SelectedHookSize {
		return fmt.Errorf("selected-item cave or original bytes are too short")
	}
	if !bytes.Equal(code[:ct084SelectedHookSize], original) {
		return fmt.Errorf("selected-item cave displaced instructions mismatch")
	}
	captureOffset := ct084SelectedHookSize
	if !bytes.Equal(code[captureOffset:captureOffset+2], []byte{0x48, 0xA3}) || binary.LittleEndian.Uint64(code[captureOffset+2:captureOffset+10]) != uint64(cave+ct084SelectedCaveDataOffset) {
		return fmt.Errorf("selected-item cave capture instruction mismatch")
	}
	jumpOffset := captureOffset + 10
	if relJumpTarget(cave+uintptr(jumpOffset), code[jumpOffset:jumpOffset+5]) != returnAddr {
		return fmt.Errorf("selected-item cave return target mismatch")
	}
	if !bytes.Equal(code[ct084SelectedCaveMarkerOffset:ct084SelectedCaveMarkerOffset+uintptr(len(ct084SelectedCaveMarker))], ct084SelectedCaveMarker[:]) ||
		code[ct084SelectedCaveKindOffset] != ct084SelectedKindByte(kind) {
		return fmt.Errorf("selected-item cave ownership marker mismatch")
	}
	if binary.LittleEndian.Uint32(code[ct084SelectedCavePIDOffset:ct084SelectedCavePIDOffset+4]) != process.PID ||
		binary.LittleEndian.Uint64(code[ct084SelectedCaveCreatedOffset:ct084SelectedCaveCreatedOffset+8]) != process.Created {
		return fmt.Errorf("selected-item cave process identity mismatch")
	}
	if binary.LittleEndian.Uint64(code[ct084SelectedCaveOwnerOffset:ct084SelectedCaveOwnerOffset+8]) != ct084SelectedOwnerFingerprint(ownerToken) {
		return fmt.Errorf("selected-item cave owner fingerprint mismatch")
	}
	return nil
}

func (a *App) validateCT084SelectedCaptureCaveLocked(lease ct084SelectedCaptureLease) error {
	code := make([]byte, int(ct084SelectedCaveDataOffset)+8)
	if err := readProcessMemory(a.hProcess, lease.CaveAddr, unsafe.Pointer(&code[0]), uintptr(len(code))); err != nil {
		return fmt.Errorf("read selected-item capture cave: %w", err)
	}
	return validateCT084SelectedCaptureCaveBytes(lease.Kind, lease.CaveAddr, lease.HookAddr+ct084SelectedHookSize, lease.Original, lease.Process, lease.OwnerToken, code)
}

func (a *App) releaseCT084SelectedCaptureHooksLocked(ownerToken string, force bool) error {
	var result error
	if err := a.releaseCT084SelectedCaptureHookLocked(&a.ct084SelectedKeyItemHook, ownerToken, force); err != nil {
		result = errors.Join(result, fmt.Errorf("key item: %w", err))
	}
	if err := a.releaseCT084SelectedCaptureHookLocked(&a.ct084SelectedMaterialHook, ownerToken, force); err != nil {
		result = errors.Join(result, fmt.Errorf("material: %w", err))
	}
	return result
}

func (a *App) releaseCT084SelectedCaptureHookLocked(lease *ct084SelectedCaptureLease, ownerToken string, force bool) error {
	if lease == nil || !lease.active() {
		return nil
	}
	if !sameProcessInstance(lease.Process, a.currentProcessInstance()) {
		return fmt.Errorf("selected-item recovery lease belongs to another process instance")
	}
	if !force && (ownerToken == "" || lease.OwnerToken != ownerToken || !runtimeOwnerTokenMatches(a.charaOwnerToken, ownerToken)) {
		return errRuntimeOwnerLeaseStale
	}
	if len(lease.Original) != ct084SelectedHookSize || lease.HookAddr == 0 || lease.CaveAddr == 0 {
		return fmt.Errorf("selected-item recovery lease is incomplete")
	}
	entry := make([]byte, ct084SelectedHookSize)
	if err := readProcessMemory(a.hProcess, lease.HookAddr, unsafe.Pointer(&entry[0]), uintptr(len(entry))); err != nil {
		return err
	}
	originalEntry := bytes.Equal(entry, lease.Original)
	if !originalEntry {
		if !isCT084SelectedJump(entry) || relJumpTarget(lease.HookAddr, entry) != lease.CaveAddr {
			return fmt.Errorf("selected-item hook entry is neither the owned jump nor exact original bytes: %s", bytesToHex(entry))
		}
	}
	if err := a.validateCT084SelectedCaptureCaveLocked(*lease); err != nil {
		return fmt.Errorf("selected-item cave ownership validation failed: %w", err)
	}
	memory := remoteCT084SelectedItemMemory{handle: a.hProcess}
	preClearErr := clearCT084SelectedPointer(memory, lease.CaveAddr+ct084SelectedCaveDataOffset)
	if !originalEntry {
		if err := writeCodeMemory(a.hProcess, lease.HookAddr, lease.Original); err != nil {
			return errors.Join(preClearErr, fmt.Errorf("restore selected-item hook entry: %w", err))
		}
		restored := make([]byte, ct084SelectedHookSize)
		if err := readProcessMemory(a.hProcess, lease.HookAddr, unsafe.Pointer(&restored[0]), uintptr(len(restored))); err != nil {
			return errors.Join(preClearErr, fmt.Errorf("read back restored selected-item hook entry: %w", err))
		}
		if !bytes.Equal(restored, lease.Original) {
			return errors.Join(preClearErr, fmt.Errorf("restored selected-item hook entry mismatch: %s", bytesToHex(restored)))
		}
	}
	postClearErr := clearCT084SelectedPointer(memory, lease.CaveAddr+ct084SelectedCaveDataOffset)
	if err := errors.Join(preClearErr, postClearErr); err != nil {
		return err
	}
	*lease = ct084SelectedCaptureLease{}
	return nil
}

func (a *App) dropCT084SelectedCaptureHooksLocked(ownerToken string, force bool) {
	for _, lease := range []*ct084SelectedCaptureLease{&a.ct084SelectedMaterialHook, &a.ct084SelectedKeyItemHook} {
		if !lease.active() || force || (ownerToken != "" && lease.OwnerToken == ownerToken) {
			*lease = ct084SelectedCaptureLease{}
		}
	}
}

func (a *App) hasCT084SelectedCaptureLeaseLocked() bool {
	return a.ct084SelectedMaterialHook.active() || a.ct084SelectedKeyItemHook.active()
}

func isCT084SelectedJump(entry []byte) bool {
	return len(entry) == ct084SelectedHookSize && entry[0] == 0xE9 && entry[5] == 0x90 && entry[6] == 0x90
}

func ct084SelectedHookDefinition(kind CT084SelectedItemKind) (string, uintptr, []byte, error) {
	switch kind {
	case CT084SelectedItemMaterial:
		return ct084SelectedMaterialAOB, ct084SelectedMaterialRVA, ct084SelectedMaterialOriginal, nil
	case CT084SelectedItemKeyItem:
		return ct084SelectedKeyItemAOB, ct084SelectedKeyItemRVA, ct084SelectedKeyItemOriginal, nil
	default:
		return "", 0, nil, fmt.Errorf("unknown selected-item kind %q", kind)
	}
}

func ct084SelectedKindByte(kind CT084SelectedItemKind) byte {
	if kind == CT084SelectedItemMaterial {
		return 1
	}
	if kind == CT084SelectedItemKeyItem {
		return 2
	}
	return 0
}

func ct084SelectedOwnerFingerprint(token string) uint64 {
	sum := sha256.Sum256([]byte(token))
	return binary.LittleEndian.Uint64(sum[:8])
}

func ct084SelectedKindName(kind CT084SelectedItemKind) string {
	if useChinese() {
		if kind == CT084SelectedItemMaterial {
			return "当前选中素材"
		}
		if kind == CT084SelectedItemKeyItem {
			return "当前选中关键物品"
		}
	}
	if kind == CT084SelectedItemMaterial {
		return "Selected Material"
	}
	if kind == CT084SelectedItemKeyItem {
		return "Selected Key Item"
	}
	return string(kind)
}
