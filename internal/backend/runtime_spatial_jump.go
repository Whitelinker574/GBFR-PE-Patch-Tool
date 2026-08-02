package backend

import (
	"bytes"
	"errors"
	"fmt"
)

const (
	runtimeSpatialJumpFeatureID = "runtime-spatial-infinite-jump"
	runtimeSpatialJumpGateRVA   = uintptr(0x1FA00AA)
	runtimeSpatialJumpCheckRVA  = uintptr(0x1FA00DC)
)

var (
	runtimeSpatialJumpGateOriginal  = []byte{0x75}
	runtimeSpatialJumpGatePatch     = []byte{0xEB}
	runtimeSpatialJumpCheckOriginal = []byte{0x85, 0xC0}
	runtimeSpatialJumpCheckPatch    = []byte{0x0C, 0x01}
	runtimeSpatialJumpGateContext   = []byte{
		0x80, 0xB9, 0x69, 0x04, 0x00, 0x00, 0x00, 0x75, 0x0D,
		0x48, 0x8B, 0x91, 0x40, 0x5D, 0x00, 0x00, 0x83, 0x7A, 0x40, 0x51, 0x75, 0xBB,
	}
	runtimeSpatialJumpCheckContext = []byte{
		0x48, 0x81, 0xC1, 0x70, 0x01, 0x00, 0x00, 0x31, 0xFF, 0x31, 0xD2,
		0x41, 0xB8, 0x03, 0x00, 0x00, 0x00, 0xE8, 0x04, 0x41, 0x23, 0x00,
		0x85, 0xC0, 0x74, 0x96, 0x48, 0x8B, 0x46, 0x10,
	}
)

type RuntimeSpatialJumpStatus struct {
	OwnerLeaseID    string   `json:"ownerToken"`
	Enabled         bool     `json:"enabled"`
	Available       bool     `json:"available"`
	Owned           bool     `json:"owned"`
	RecoveryPending bool     `json:"recoveryPending"`
	RVAs            []uint64 `json:"rvas"`
	CurrentBytes    []string `json:"currentBytes"`
	PID             uint32   `json:"pid"`
	ProcessCreated  uint64   `json:"processCreated,string"`
	GameVersion     string   `json:"gameVersion"`
	Source          string   `json:"source"`
	Error           string   `json:"error"`
}

func cloneRuntimeSpatialJumpLease(lease runtimePatchPatchLease) *runtimePatchPatchLease {
	cloned := cloneRuntimePatchPatchLease(lease)
	return &cloned
}

func runtimeSpatialJumpAddresses(moduleBase uintptr) (uintptr, uintptr, error) {
	if moduleBase == 0 || moduleBase > ^uintptr(0)-runtimeSpatialJumpCheckRVA {
		return 0, 0, fmt.Errorf("%s", runtimePatchMonitorText("连续跳跃入口地址无效", "The continuous-jump addresses are invalid"))
	}
	return moduleBase + runtimeSpatialJumpGateRVA, moduleBase + runtimeSpatialJumpCheckRVA, nil
}

func validateRuntimeSpatialJumpLease(lease runtimePatchPatchLease, owner string, process processInstanceID, moduleBase uintptr) error {
	if lease.FeatureID != runtimeSpatialJumpFeatureID || len(lease.Sites) != 2 {
		return errors.Join(fmt.Errorf("invalid continuous-jump recovery lease"), errLiveMemoryRollbackUnproven)
	}
	if err := validateRuntimePatchOwnedLease(lease, owner, process); err != nil {
		return err
	}
	gate, check, err := runtimeSpatialJumpAddresses(moduleBase)
	if err != nil {
		return err
	}
	expected := []runtimePatchPatchSiteLease{
		{Address: gate, RVA: uint64(runtimeSpatialJumpGateRVA), Original: runtimeSpatialJumpGateOriginal, Patch: runtimeSpatialJumpGatePatch},
		{Address: check, RVA: uint64(runtimeSpatialJumpCheckRVA), Original: runtimeSpatialJumpCheckOriginal, Patch: runtimeSpatialJumpCheckPatch},
	}
	for index := range expected {
		actual := lease.Sites[index]
		want := expected[index]
		if actual.Address != want.Address || actual.RVA != want.RVA || !bytes.Equal(actual.Original, want.Original) || !bytes.Equal(actual.Patch, want.Patch) {
			return errors.Join(fmt.Errorf("continuous-jump recovery lease site %d changed", index), errLiveMemoryRollbackUnproven)
		}
	}
	return nil
}

func prepareRuntimeSpatialJumpSites(memory runtimePatchMemory, moduleBase, moduleEnd uintptr, layout runtimeGameLayout) ([]runtimePatchPatchSiteLease, error) {
	if layout.Version != "2.0.3" {
		return nil, fmt.Errorf("%s", runtimePatchMonitorText("连续跳跃目前只完成 GAME 2.0.3 双入口标定", "Continuous jump is currently calibrated only for GAME 2.0.3"))
	}
	gate, check, err := runtimeSpatialJumpAddresses(moduleBase)
	if err != nil {
		return nil, err
	}
	type definition struct {
		name        string
		address     uintptr
		contextBack uintptr
		context     []byte
		original    []byte
		patch       []byte
	}
	definitions := []definition{
		{name: "状态前置判定", address: gate, contextBack: 7, context: runtimeSpatialJumpGateContext, original: runtimeSpatialJumpGateOriginal, patch: runtimeSpatialJumpGatePatch},
		{name: "动作条件判定", address: check, contextBack: 22, context: runtimeSpatialJumpCheckContext, original: runtimeSpatialJumpCheckOriginal, patch: runtimeSpatialJumpCheckPatch},
	}
	sites := make([]runtimePatchPatchSiteLease, 0, len(definitions))
	for _, definition := range definitions {
		if definition.address < moduleBase+definition.contextBack || definition.address+uintptr(len(definition.patch)) > moduleEnd {
			return nil, fmt.Errorf("%s%s", definition.name, runtimePatchMonitorText("入口超出游戏模块范围", " entry is outside the game module"))
		}
		current, err := memory.ReadCode(definition.address, len(definition.original))
		if err != nil {
			return nil, fmt.Errorf("读取%s失败: %w", definition.name, err)
		}
		if bytes.Equal(current, definition.patch) {
			return nil, fmt.Errorf("%s", runtimePatchMonitorText("连续跳跃入口已被其他工具修改，本工具不会抢占", "A continuous-jump site is already modified by another tool and will not be claimed"))
		}
		if !bytes.Equal(current, definition.original) {
			return nil, fmt.Errorf("%s原字节未知: %s", definition.name, bytesToHex(current))
		}
		context, err := memory.ReadCode(definition.address-definition.contextBack, len(definition.context))
		if err != nil || !bytes.Equal(context, definition.context) {
			return nil, fmt.Errorf("%s", runtimePatchMonitorText("连续跳跃入口上下文与 2.0.3 动作函数不一致", "The continuous-jump context does not match the 2.0.3 action function"))
		}
		sites = append(sites, runtimePatchPatchSiteLease{
			Address: definition.address, RVA: uint64(definition.address - moduleBase),
			Original: append([]byte(nil), definition.original...), Patch: append([]byte(nil), definition.patch...),
		})
	}
	return sites, nil
}

func readRuntimeSpatialJumpStatus(memory runtimePatchMemory, moduleBase uintptr, process processInstanceID, owner string, lease *runtimePatchPatchLease, layout runtimeGameLayout) RuntimeSpatialJumpStatus {
	status := RuntimeSpatialJumpStatus{
		OwnerLeaseID: owner, RVAs: []uint64{uint64(runtimeSpatialJumpGateRVA), uint64(runtimeSpatialJumpCheckRVA)},
		CurrentBytes: []string{"", ""}, PID: process.PID, ProcessCreated: process.Created,
		GameVersion: layout.Version, Source: "game_runtime_continuous_jump_" + layout.Version,
	}
	gate, check, err := runtimeSpatialJumpAddresses(moduleBase)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	if layout.Version != "2.0.3" {
		status.Error = runtimePatchMonitorText("连续跳跃目前只完成 GAME 2.0.3 双入口标定", "Continuous jump is currently calibrated only for GAME 2.0.3")
		return status
	}
	if lease != nil {
		status.Owned = runtimeOwnerTokenMatches(lease.OwnerToken, owner) && sameProcessInstance(lease.Process, process)
		status.RecoveryPending = lease.State == runtimePatchPatchRecovery
		if err := validateRuntimeSpatialJumpLease(*lease, owner, process, moduleBase); err != nil {
			status.Error = err.Error()
			return status
		}
	}
	addresses := []uintptr{gate, check}
	originals := [][]byte{runtimeSpatialJumpGateOriginal, runtimeSpatialJumpCheckOriginal}
	patches := [][]byte{runtimeSpatialJumpGatePatch, runtimeSpatialJumpCheckPatch}
	allOriginal, allPatched := true, true
	for index, address := range addresses {
		current, err := memory.ReadCode(address, len(originals[index]))
		if err != nil {
			status.Error = fmt.Sprintf("读取连续跳跃入口 %d 失败: %v", index+1, err)
			return status
		}
		status.CurrentBytes[index] = bytesToHex(current)
		allOriginal = allOriginal && bytes.Equal(current, originals[index])
		allPatched = allPatched && bytes.Equal(current, patches[index])
	}
	if allPatched {
		status.Enabled = true
		status.Available = lease != nil && status.Owned
		if lease == nil {
			status.Error = runtimePatchMonitorText("连续跳跃已由其他工具开启，本工具不会接管", "Continuous jump is already enabled by another tool and will not be claimed")
		}
		return status
	}
	if !allOriginal {
		status.Error = runtimePatchMonitorText("连续跳跃两处入口状态不一致，需要先恢复", "The two continuous-jump sites disagree and require recovery")
		return status
	}
	status.Available = lease == nil || status.Owned
	return status
}

func (a *App) RuntimeSpatialJumpStatusOwned(owner string) (RuntimeSpatialJumpStatus, error) {
	liveMemoryWriteMu.Lock()
	defer liveMemoryWriteMu.Unlock()
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, owner); err != nil {
		return RuntimeSpatialJumpStatus{}, err
	}
	defer a.procMu.Unlock()
	process := a.currentProcessInstance()
	if err := a.verifyRuntimePatchExecutableLocked(process, runtimePatchMonitorText("连续跳跃", "Continuous jump")); err != nil {
		return RuntimeSpatialJumpStatus{}, err
	}
	layout, err := detectRuntimeGameLayout(remoteRuntimePatchPartyMemory{app: a}, a.moduleBase)
	if err != nil {
		return RuntimeSpatialJumpStatus{}, err
	}
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	return readRuntimeSpatialJumpStatus(runtimePatchProcessMemory{handle: a.hProcess}, a.moduleBase, process, owner, a.runtimeSpatialJumpLease, layout), nil
}

func (a *App) RuntimeSpatialJumpSetEnabledOwned(owner string, enabled bool) (RuntimeSpatialJumpStatus, error) {
	liveMemoryWriteMu.Lock()
	defer liveMemoryWriteMu.Unlock()
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, owner); err != nil {
		return RuntimeSpatialJumpStatus{}, err
	}
	defer a.procMu.Unlock()
	if enabled {
		if err := a.ensureLiveMemoryWritesSafe(); err != nil {
			return RuntimeSpatialJumpStatus{}, err
		}
	}
	process := a.currentProcessInstance()
	if err := a.verifyRuntimePatchExecutableLocked(process, runtimePatchMonitorText("连续跳跃", "Continuous jump")); err != nil {
		return RuntimeSpatialJumpStatus{}, err
	}
	layout, err := detectRuntimeGameLayout(remoteRuntimePatchPartyMemory{app: a}, a.moduleBase)
	if err != nil {
		return RuntimeSpatialJumpStatus{}, err
	}
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	memory := runtimePatchProcessMemory{handle: a.hProcess}
	if a.runtimeSpatialJumpLease != nil {
		lease := *a.runtimeSpatialJumpLease
		if err := validateRuntimeSpatialJumpLease(lease, owner, process, a.moduleBase); err != nil {
			return readRuntimeSpatialJumpStatus(memory, a.moduleBase, process, owner, a.runtimeSpatialJumpLease, layout), err
		}
		if enabled {
			status := readRuntimeSpatialJumpStatus(memory, a.moduleBase, process, owner, a.runtimeSpatialJumpLease, layout)
			if lease.State == runtimePatchPatchEnabled && status.Enabled && status.Available {
				return status, nil
			}
			return status, errors.Join(fmt.Errorf("%s", runtimePatchMonitorText("连续跳跃入口需要先恢复", "Continuous-jump sites require recovery first")), errLiveMemoryRollbackUnproven)
		}
		if err := a.restoreRuntimeSpatialJumpOwnedLocked(owner, false); err != nil {
			return readRuntimeSpatialJumpStatus(memory, a.moduleBase, process, owner, a.runtimeSpatialJumpLease, layout), err
		}
		return readRuntimeSpatialJumpStatus(memory, a.moduleBase, process, owner, nil, layout), nil
	}
	if !enabled {
		return readRuntimeSpatialJumpStatus(memory, a.moduleBase, process, owner, nil, layout), nil
	}
	moduleSize, err := getRemoteModuleSize(a.hProcess, a.moduleBase)
	if err != nil {
		return RuntimeSpatialJumpStatus{}, err
	}
	moduleEnd := a.moduleBase + moduleSize
	if moduleEnd < a.moduleBase {
		return RuntimeSpatialJumpStatus{}, fmt.Errorf("游戏模块范围溢出")
	}
	sites, err := prepareRuntimeSpatialJumpSites(memory, a.moduleBase, moduleEnd, layout)
	if err != nil {
		return readRuntimeSpatialJumpStatus(memory, a.moduleBase, process, owner, nil, layout), err
	}
	if overlap := findRuntimePatchActiveAddressOverlap(sites, a.runtimePatchPatchLeases, runtimeSpatialJumpFeatureID); overlap != "" {
		return RuntimeSpatialJumpStatus{}, fmt.Errorf("连续跳跃入口与当前补丁重叠: %s", overlap)
	}
	candidate := runtimePatchPatchLease{FeatureID: runtimeSpatialJumpFeatureID, OwnerToken: owner, Process: process, State: runtimePatchPatchRecovery, Sites: sites}
	a.runtimeSpatialJumpLease = cloneRuntimeSpatialJumpLease(candidate)
	if err := installRuntimePatchSites(memory, sites); err != nil {
		if !errors.Is(err, errLiveMemoryRollbackUnproven) {
			a.runtimeSpatialJumpLease = nil
		}
		return readRuntimeSpatialJumpStatus(memory, a.moduleBase, process, owner, a.runtimeSpatialJumpLease, layout), err
	}
	candidate.State = runtimePatchPatchEnabled
	a.runtimeSpatialJumpLease = cloneRuntimeSpatialJumpLease(candidate)
	return readRuntimeSpatialJumpStatus(memory, a.moduleBase, process, owner, a.runtimeSpatialJumpLease, layout), nil
}

func (a *App) restoreRuntimeSpatialJumpOwnedLocked(owner string, force bool) error {
	if a.runtimeSpatialJumpLease == nil {
		return nil
	}
	lease := *a.runtimeSpatialJumpLease
	if !force && !runtimeOwnerTokenMatches(lease.OwnerToken, owner) {
		return nil
	}
	validationOwner := owner
	if force {
		validationOwner = lease.OwnerToken
	}
	if err := validateRuntimeSpatialJumpLease(lease, validationOwner, a.currentProcessInstance(), a.moduleBase); err != nil {
		return err
	}
	lease.State = runtimePatchPatchRecovery
	a.runtimeSpatialJumpLease = cloneRuntimeSpatialJumpLease(lease)
	if err := restoreRuntimePatchSites(runtimePatchProcessMemory{handle: a.hProcess}, lease.Sites); err != nil {
		return err
	}
	a.runtimeSpatialJumpLease = nil
	return nil
}

func (a *App) dropRuntimeSpatialJumpOwnerLocked(owner string) {
	if a.runtimeSpatialJumpLease != nil && runtimeOwnerTokenMatches(a.runtimeSpatialJumpLease.OwnerToken, owner) {
		a.runtimeSpatialJumpLease = nil
	}
}
