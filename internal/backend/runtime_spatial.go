package backend

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"unsafe"
)

const (
	runtimeSpatialVectorBytes                = 12
	runtimeSpatialMaximumStep        float32 = 50
	runtimeSpatialGravityFeatureID           = "runtime-spatial-gravity"
	runtimeSpatialGravityRVA         uintptr = 0x39DD964
	runtimeSpatialGravityContextBack         = 4
	// A direct NOP patch cannot restore itself after the desktop application is
	// forcibly terminated. Keep the read/recovery path available for sessions
	// created by earlier test builds, but do not create a new gravity lease in
	// the stable release until an in-process owner watchdog is field-proven.
	runtimeSpatialGravityStableReleaseEnabled = true
)

var (
	runtimeSpatialGravityOriginal = []byte{0xC5, 0xF8, 0x29, 0x81, 0xD0, 0x00, 0x00, 0x00}
	runtimeSpatialGravityPatch    = []byte{0x90, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90}
	// The surrounding pair of tiny setters is unique in the verified 2.0.2 image.
	runtimeSpatialGravityContext = []byte{
		0xC5, 0xF8, 0x28, 0x02,
		0xC5, 0xF8, 0x29, 0x81, 0xD0, 0x00, 0x00, 0x00,
		0xC3, 0xCC, 0xCC, 0xCC,
		0xC5, 0xF8, 0x28, 0x02,
		0xC5, 0xF8, 0x29, 0x81, 0x70, 0x01, 0x00, 0x00,
		0xC3, 0xCC, 0xCC, 0xCC,
	}
)

type RuntimeSpatialTeleportResult struct {
	OwnerLeaseID    string              `json:"ownerToken"`
	Before          RuntimePatchVector3 `json:"before"`
	Requested       RuntimePatchVector3 `json:"requested"`
	Observed        RuntimePatchVector3 `json:"observed"`
	PID             uint32              `json:"pid"`
	ProcessCreated  uint64              `json:"processCreated"`
	GameVersion     string              `json:"gameVersion"`
	Source          string              `json:"source"`
	SnapshotCount   int                 `json:"snapshotCount"`
	RuntimeVerified bool                `json:"runtimeVerified"`
}

type RuntimeSpatialGravityStatus struct {
	OwnerLeaseID    string `json:"ownerToken"`
	Enabled         bool   `json:"enabled"`
	Available       bool   `json:"available"`
	Owned           bool   `json:"owned"`
	RecoveryPending bool   `json:"recoveryPending"`
	Address         uint64 `json:"address"`
	RVA             uint64 `json:"rva"`
	CurrentBytes    string `json:"currentBytes"`
	PID             uint32 `json:"pid"`
	ProcessCreated  uint64 `json:"processCreated"`
	GameVersion     string `json:"gameVersion"`
	Source          string `json:"source"`
	Error           string `json:"error"`
}

type runtimeSpatialMemory interface {
	runtimePatchPartyMemory
	WriteAt(address uintptr, source []byte) error
}

type remoteRuntimeSpatialMemory struct {
	app *App
}

func (memory remoteRuntimeSpatialMemory) ReadAt(address uintptr, destination []byte) error {
	return remoteRuntimePatchPartyMemory(memory).ReadAt(address, destination)
}

func (memory remoteRuntimeSpatialMemory) WriteAt(address uintptr, source []byte) error {
	if memory.app == nil || memory.app.hProcess == 0 || address == 0 || len(source) == 0 {
		return fmt.Errorf("invalid spatial memory write")
	}
	return writeProcessMemory(memory.app.hProcess, address, unsafe.Pointer(&source[0]), uintptr(len(source)))
}

func encodeRuntimeSpatialVector(value RuntimePatchVector3) []byte {
	encoded := make([]byte, runtimeSpatialVectorBytes)
	binary.LittleEndian.PutUint32(encoded[0:4], math.Float32bits(value.Z))
	binary.LittleEndian.PutUint32(encoded[4:8], math.Float32bits(value.Y))
	binary.LittleEndian.PutUint32(encoded[8:12], math.Float32bits(value.X))
	return encoded
}

func runtimeSpatialPlayerNode(snapshot runtimePatchPartySnapshot) (uintptr, error) {
	if len(snapshot.Result.Entities) == 0 || !snapshot.Result.Entities[0].Present {
		return 0, fmt.Errorf("%s", runtimePatchMonitorText("当前场景没有可用的玩家实体", "The current scene does not expose a player entity"))
	}
	if snapshot.Topology.Entities[0] == 0 || snapshot.Topology.TransformNodes[0][0] == 0 || snapshot.Topology.TransformNodes[0][1] == 0 {
		return 0, fmt.Errorf("%s", runtimePatchMonitorText("玩家坐标拓扑不完整", "The player coordinate topology is incomplete"))
	}
	return snapshot.Topology.TransformNodes[0][1], nil
}

func verifyRuntimeSpatialTopology(memory runtimePatchPartyMemory, snapshot runtimePatchPartySnapshot) error {
	entity, err := readRuntimePatchPointer(memory, snapshot.Topology.Root)
	if err != nil || entity != snapshot.Topology.Entities[0] {
		return fmt.Errorf("%s", runtimePatchMonitorText("写入前玩家实体已经变化，请等待场景稳定后重试", "The player entity changed before the write; wait for a stable scene and retry"))
	}
	root, err := readRuntimePatchPointer(memory, entity+runtimePatchPartyTransformRootOffset)
	if err != nil || root != snapshot.Topology.TransformNodes[0][0] {
		return fmt.Errorf("%s", runtimePatchMonitorText("写入前玩家坐标根节点已经变化", "The player transform root changed before the write"))
	}
	node, err := readRuntimePatchPointer(memory, root+runtimePatchPartyTransformNodeOffset)
	if err != nil || node != snapshot.Topology.TransformNodes[0][1] {
		return fmt.Errorf("%s", runtimePatchMonitorText("写入前玩家坐标节点已经变化", "The player transform node changed before the write"))
	}
	return nil
}

func writeRuntimeSpatialPlayerPosition(memory runtimeSpatialMemory, moduleBase uintptr, target RuntimePatchVector3) (RuntimeSpatialTeleportResult, error) {
	if memory == nil || moduleBase == 0 {
		return RuntimeSpatialTeleportResult{}, fmt.Errorf("%s", runtimePatchMonitorText("空间写入参数无效", "Invalid spatial write parameters"))
	}
	if err := validateRuntimePatchVector3(target); err != nil {
		return RuntimeSpatialTeleportResult{}, err
	}
	return writeRuntimeSpatialPlayerPositionResolved(memory, moduleBase, func(RuntimePatchVector3) (RuntimePatchVector3, error) {
		return target, nil
	})
}

func validateRuntimeSpatialDelta(delta RuntimePatchVector3) error {
	if err := validateRuntimePatchVector3(delta); err != nil {
		return err
	}
	if delta == (RuntimePatchVector3{}) {
		return fmt.Errorf("%s", runtimePatchMonitorText("移动步长不能全部为零", "The movement step cannot be zero on every axis"))
	}
	for _, coordinate := range []float32{delta.X, delta.Y, delta.Z} {
		if float32(math.Abs(float64(coordinate))) > runtimeSpatialMaximumStep {
			return fmt.Errorf("%s", runtimePatchMonitorText("单次移动步长不能超过 50", "A single movement step cannot exceed 50"))
		}
	}
	return nil
}

func writeRuntimeSpatialPlayerDelta(memory runtimeSpatialMemory, moduleBase uintptr, delta RuntimePatchVector3) (RuntimeSpatialTeleportResult, error) {
	if memory == nil || moduleBase == 0 {
		return RuntimeSpatialTeleportResult{}, fmt.Errorf("%s", runtimePatchMonitorText("空间写入参数无效", "Invalid spatial write parameters"))
	}
	if err := validateRuntimeSpatialDelta(delta); err != nil {
		return RuntimeSpatialTeleportResult{}, err
	}
	return writeRuntimeSpatialPlayerPositionResolved(memory, moduleBase, func(before RuntimePatchVector3) (RuntimePatchVector3, error) {
		target := RuntimePatchVector3{X: before.X + delta.X, Y: before.Y + delta.Y, Z: before.Z + delta.Z}
		if err := validateRuntimePatchVector3(target); err != nil {
			return RuntimePatchVector3{}, err
		}
		return target, nil
	})
}

func writeRuntimeSpatialPlayerPositionResolved(memory runtimeSpatialMemory, moduleBase uintptr, resolveTarget func(RuntimePatchVector3) (RuntimePatchVector3, error)) (RuntimeSpatialTeleportResult, error) {
	if resolveTarget == nil {
		return RuntimeSpatialTeleportResult{}, fmt.Errorf("%s", runtimePatchMonitorText("空间目标解析器为空", "The spatial target resolver is nil"))
	}
	stable, err := readStableRuntimePatchPartySnapshots(func() (runtimePatchPartySnapshot, error) {
		return readRuntimePatchPartySnapshot(memory, moduleBase)
	})
	if err != nil {
		return RuntimeSpatialTeleportResult{}, err
	}
	frame, err := readRuntimePatchPartySnapshot(memory, moduleBase)
	if err != nil {
		return RuntimeSpatialTeleportResult{}, err
	}
	node, err := runtimeSpatialPlayerNode(frame)
	if err != nil {
		return RuntimeSpatialTeleportResult{}, err
	}
	if stable.RootAddress != uint64(frame.Topology.Root) || stable.Entities[0].Address != uint64(frame.Topology.Entities[0]) {
		return RuntimeSpatialTeleportResult{}, fmt.Errorf("%s", runtimePatchMonitorText("玩家实体在确认写入时发生变化", "The player entity changed while confirming the write"))
	}
	if err := verifyRuntimeSpatialTopology(memory, frame); err != nil {
		return RuntimeSpatialTeleportResult{}, err
	}
	address := node + runtimePatchPartyPositionZOffset
	original := make([]byte, runtimeSpatialVectorBytes)
	if err := memory.ReadAt(address, original); err != nil {
		return RuntimeSpatialTeleportResult{}, fmt.Errorf("%s: %w", runtimePatchMonitorText("读取原坐标失败", "Read original position"), err)
	}
	before := frame.Result.Entities[0].Position
	target, err := resolveTarget(before)
	if err != nil {
		return RuntimeSpatialTeleportResult{}, err
	}
	if !bytes.Equal(original, encodeRuntimeSpatialVector(before)) {
		return RuntimeSpatialTeleportResult{}, fmt.Errorf("%s", runtimePatchMonitorText("坐标字节与稳定快照不一致，请停止移动后重试", "Position bytes no longer match the stable snapshot; stop moving and retry"))
	}
	encoded := encodeRuntimeSpatialVector(target)
	if err := memory.WriteAt(address, encoded); err != nil {
		return RuntimeSpatialTeleportResult{}, fmt.Errorf("%s: %w", runtimePatchMonitorText("一次性传送写入失败", "One-shot teleport write failed"), err)
	}
	actual := make([]byte, runtimeSpatialVectorBytes)
	if err := memory.ReadAt(address, actual); err != nil || !bytes.Equal(actual, encoded) {
		restoreErr := memory.WriteAt(address, original)
		verify := make([]byte, runtimeSpatialVectorBytes)
		verifyErr := memory.ReadAt(address, verify)
		if restoreErr != nil || verifyErr != nil || !bytes.Equal(verify, original) {
			return RuntimeSpatialTeleportResult{}, errors.Join(
				fmt.Errorf("%s", runtimePatchMonitorText("传送回读失败且无法证明原坐标已恢复；本进程后续写入已阻止", "Teleport verification failed and restoration could not be proven; further writes to this process are blocked")),
				errLiveMemoryRollbackUnproven,
			)
		}
		if err != nil {
			return RuntimeSpatialTeleportResult{}, fmt.Errorf("%s: %w", runtimePatchMonitorText("传送回读失败，已恢复原坐标", "Teleport read-back failed; the original position was restored"), err)
		}
		return RuntimeSpatialTeleportResult{}, fmt.Errorf("%s", runtimePatchMonitorText("游戏在回读前改写了坐标，已恢复原坐标", "The game changed the position before verification; the original position was restored"))
	}
	if err := verifyRuntimeSpatialTopology(memory, frame); err != nil {
		return RuntimeSpatialTeleportResult{}, errors.Join(
			fmt.Errorf("%s: %w", runtimePatchMonitorText("传送完成后玩家坐标拓扑发生变化，无法证明原坐标可安全恢复；本进程后续写入已阻止", "The player transform topology changed after teleportation, so restoration cannot be proven safe; further writes to this process are blocked"), err),
			errLiveMemoryRollbackUnproven,
		)
	}
	return RuntimeSpatialTeleportResult{
		Before: before, Requested: target, Observed: target,
		GameVersion: "2.0.2", Source: "game_runtime_spatial_2.0.2",
		SnapshotCount: runtimePatchPartySnapshotCount, RuntimeVerified: true,
	}, nil
}

func (a *App) RuntimeSpatialTeleportOwned(leaseID string, target RuntimePatchVector3) (RuntimeSpatialTeleportResult, error) {
	liveMemoryWriteMu.Lock()
	defer liveMemoryWriteMu.Unlock()
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, leaseID); err != nil {
		return RuntimeSpatialTeleportResult{}, err
	}
	defer a.procMu.Unlock()
	if err := a.ensureLiveMemoryWritesSafe(); err != nil {
		return RuntimeSpatialTeleportResult{}, err
	}
	process := a.currentProcessInstance()
	if err := a.verifyRuntimePatchExecutableLocked(process, runtimePatchMonitorText("空间与移动实验", "Spatial movement experiment")); err != nil {
		return RuntimeSpatialTeleportResult{}, err
	}
	result, err := writeRuntimeSpatialPlayerPosition(remoteRuntimeSpatialMemory{app: a}, a.moduleBase, target)
	if err != nil {
		if errors.Is(err, errLiveMemoryRollbackUnproven) {
			a.poisonCurrentLiveMemoryWrites()
		}
		return RuntimeSpatialTeleportResult{}, err
	}
	result.PID = a.charaPID
	result.ProcessCreated = a.charaCreated
	result.OwnerLeaseID = leaseID
	return result, nil
}

func (a *App) RuntimeSpatialMoveOwned(leaseID string, delta RuntimePatchVector3) (RuntimeSpatialTeleportResult, error) {
	liveMemoryWriteMu.Lock()
	defer liveMemoryWriteMu.Unlock()
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, leaseID); err != nil {
		return RuntimeSpatialTeleportResult{}, err
	}
	defer a.procMu.Unlock()
	if err := a.ensureLiveMemoryWritesSafe(); err != nil {
		return RuntimeSpatialTeleportResult{}, err
	}
	process := a.currentProcessInstance()
	if err := a.verifyRuntimePatchExecutableLocked(process, runtimePatchMonitorText("持续坐标飞行", "Continuous coordinate flight")); err != nil {
		return RuntimeSpatialTeleportResult{}, err
	}
	result, err := writeRuntimeSpatialPlayerDelta(remoteRuntimeSpatialMemory{app: a}, a.moduleBase, delta)
	if err != nil {
		if errors.Is(err, errLiveMemoryRollbackUnproven) {
			a.poisonCurrentLiveMemoryWrites()
		}
		return RuntimeSpatialTeleportResult{}, err
	}
	result.PID = a.charaPID
	result.ProcessCreated = a.charaCreated
	result.OwnerLeaseID = leaseID
	result.Source = "game_runtime_spatial_continuous_2.0.2"
	return result, nil
}

func runtimeSpatialGravityAddress(moduleBase uintptr, layouts ...runtimeGameLayout) (uintptr, error) {
	layout := runtimeGameLayouts[0]
	if len(layouts) > 0 {
		layout = layouts[0]
	}
	if moduleBase == 0 || moduleBase > ^uintptr(0)-layout.SpatialGravityRVA {
		return 0, fmt.Errorf("%s", runtimePatchMonitorText("重力入口地址无效", "The gravity entry address is invalid"))
	}
	return moduleBase + layout.SpatialGravityRVA, nil
}

func cloneRuntimeSpatialGravityLease(lease runtimePatchPatchLease) *runtimePatchPatchLease {
	cloned := cloneRuntimePatchPatchLease(lease)
	return &cloned
}

func validateRuntimeSpatialGravityLease(lease runtimePatchPatchLease, owner string, process processInstanceID, moduleBase uintptr, layouts ...runtimeGameLayout) error {
	layout := runtimeGameLayouts[0]
	if len(layouts) > 0 {
		layout = layouts[0]
	}
	if lease.FeatureID != runtimeSpatialGravityFeatureID || len(lease.Sites) != 1 {
		return errors.Join(fmt.Errorf("invalid spatial gravity recovery lease"), errLiveMemoryRollbackUnproven)
	}
	if err := validateRuntimePatchOwnedLease(lease, owner, process); err != nil {
		return err
	}
	address, err := runtimeSpatialGravityAddress(moduleBase, layout)
	if err != nil {
		return err
	}
	site := lease.Sites[0]
	if site.Address != address || site.RVA != uint64(layout.SpatialGravityRVA) ||
		!bytes.Equal(site.Original, runtimeSpatialGravityOriginal) || !bytes.Equal(site.Patch, runtimeSpatialGravityPatch) {
		return errors.Join(fmt.Errorf("spatial gravity recovery lease does not match the verified site"), errLiveMemoryRollbackUnproven)
	}
	return nil
}

func prepareRuntimeSpatialGravitySite(memory runtimePatchMemory, moduleBase, moduleEnd uintptr, layouts ...runtimeGameLayout) (runtimePatchPatchSiteLease, error) {
	layout := runtimeGameLayouts[0]
	if len(layouts) > 0 {
		layout = layouts[0]
	}
	address, err := runtimeSpatialGravityAddress(moduleBase, layout)
	if err != nil {
		return runtimePatchPatchSiteLease{}, err
	}
	if address < moduleBase+runtimeSpatialGravityContextBack || address+uintptr(len(runtimeSpatialGravityOriginal)) > moduleEnd {
		return runtimePatchPatchSiteLease{}, fmt.Errorf("%s", runtimePatchMonitorText("重力入口超出游戏模块范围", "The gravity entry is outside the game module"))
	}
	current, err := memory.ReadCode(address, len(runtimeSpatialGravityOriginal))
	if err != nil {
		return runtimePatchPatchSiteLease{}, fmt.Errorf("%s: %w", runtimePatchMonitorText("读取重力入口失败", "Read gravity entry"), err)
	}
	if bytes.Equal(current, runtimeSpatialGravityPatch) {
		return runtimePatchPatchSiteLease{}, fmt.Errorf("%s", runtimePatchMonitorText("重力入口已被其他工具修改，本工具不会接管", "The gravity entry was already changed by another tool and will not be claimed"))
	}
	if !bytes.Equal(current, runtimeSpatialGravityOriginal) {
		return runtimePatchPatchSiteLease{}, fmt.Errorf("%s: %s", runtimePatchMonitorText("重力入口字节未知", "Unknown gravity entry bytes"), bytesToHex(current))
	}
	contextAddress := address - runtimeSpatialGravityContextBack
	context, err := memory.ReadCode(contextAddress, len(runtimeSpatialGravityContext))
	if err != nil {
		return runtimePatchPatchSiteLease{}, fmt.Errorf("%s: %w", runtimePatchMonitorText("读取重力入口上下文失败", "Read gravity entry context"), err)
	}
	if !bytes.Equal(context, runtimeSpatialGravityContext) {
		return runtimePatchPatchSiteLease{}, fmt.Errorf("%s", runtimePatchMonitorText("重力入口上下文与已识别游戏布局不一致", "The gravity entry context does not match the identified game layout"))
	}
	return runtimePatchPatchSiteLease{
		Address: address, RVA: uint64(layout.SpatialGravityRVA),
		Original: append([]byte(nil), runtimeSpatialGravityOriginal...),
		Patch:    append([]byte(nil), runtimeSpatialGravityPatch...),
	}, nil
}

func readRuntimeSpatialGravityStatus(memory runtimePatchMemory, moduleBase uintptr, process processInstanceID, owner string, lease *runtimePatchPatchLease, layouts ...runtimeGameLayout) RuntimeSpatialGravityStatus {
	layout := runtimeGameLayouts[0]
	if len(layouts) > 0 {
		layout = layouts[0]
	}
	status := RuntimeSpatialGravityStatus{
		OwnerLeaseID: owner, RVA: uint64(layout.SpatialGravityRVA), PID: process.PID, ProcessCreated: process.Created,
		GameVersion: layout.Version, Source: "game_runtime_gravity_patch_" + layout.Version,
	}
	address, err := runtimeSpatialGravityAddress(moduleBase, layout)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.Address = uint64(address)
	if lease != nil {
		status.Owned = runtimeOwnerTokenMatches(lease.OwnerToken, owner) && sameProcessInstance(lease.Process, process)
		status.RecoveryPending = lease.State == runtimePatchPatchRecovery
		if err := validateRuntimeSpatialGravityLease(*lease, owner, process, moduleBase, layout); err != nil {
			status.Error = err.Error()
			return status
		}
	}
	current, err := memory.ReadCode(address, len(runtimeSpatialGravityOriginal))
	if err != nil {
		status.Error = fmt.Sprintf("%s: %v", runtimePatchMonitorText("读取重力入口失败", "Read gravity entry"), err)
		return status
	}
	status.CurrentBytes = bytesToHex(current)
	if bytes.Equal(current, runtimeSpatialGravityPatch) {
		status.Enabled = true
		status.Available = lease != nil && status.Owned
		if lease == nil {
			status.Error = runtimePatchMonitorText("重力入口已被其他工具修改，本工具不会接管", "The gravity entry was already changed by another tool and will not be claimed")
		}
		return status
	}
	if !bytes.Equal(current, runtimeSpatialGravityOriginal) {
		status.Error = fmt.Sprintf("%s: %s", runtimePatchMonitorText("重力入口字节未知", "Unknown gravity entry bytes"), bytesToHex(current))
		return status
	}
	if lease != nil {
		status.Available = status.Owned
		return status
	}
	context, err := memory.ReadCode(address-runtimeSpatialGravityContextBack, len(runtimeSpatialGravityContext))
	if err != nil {
		status.Error = fmt.Sprintf("%s: %v", runtimePatchMonitorText("读取重力入口上下文失败", "Read gravity entry context"), err)
		return status
	}
	if !bytes.Equal(context, runtimeSpatialGravityContext) {
		status.Error = runtimePatchMonitorText("重力入口上下文与已验证的 2.0.2 指令不一致", "The gravity entry context does not match the verified 2.0.2 instructions")
		return status
	}
	status.Available = true
	return status
}

func (a *App) RuntimeSpatialGravityStatusOwned(owner string) (RuntimeSpatialGravityStatus, error) {
	liveMemoryWriteMu.Lock()
	defer liveMemoryWriteMu.Unlock()
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, owner); err != nil {
		return RuntimeSpatialGravityStatus{}, err
	}
	defer a.procMu.Unlock()
	process := a.currentProcessInstance()
	if err := a.verifyRuntimePatchExecutableLocked(process, runtimePatchMonitorText("重力抑制", "Gravity suppression")); err != nil {
		return RuntimeSpatialGravityStatus{}, err
	}
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	layout, err := detectRuntimeGameLayout(remoteRuntimePatchPartyMemory{app: a}, a.moduleBase)
	if err != nil {
		return RuntimeSpatialGravityStatus{}, err
	}
	status := readRuntimeSpatialGravityStatus(runtimePatchProcessMemory{handle: a.hProcess}, a.moduleBase, process, owner, a.runtimeSpatialGravityLease, layout)
	if !runtimeSpatialGravityStableReleaseEnabled && a.runtimeSpatialGravityLease == nil {
		status.Available = false
		if status.Error == "" {
			status.Error = runtimePatchMonitorText(
				"正式版暂不开放重力抑制：工具异常退出时无法证明游戏原指令会自动恢复",
				"Gravity suppression is unavailable in the stable build because restoration after a forced app exit is not yet proven",
			)
		}
	}
	return status, nil
}

func (a *App) RuntimeSpatialGravitySetEnabledOwned(owner string, enabled bool) (RuntimeSpatialGravityStatus, error) {
	liveMemoryWriteMu.Lock()
	defer liveMemoryWriteMu.Unlock()
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, owner); err != nil {
		return RuntimeSpatialGravityStatus{}, err
	}
	defer a.procMu.Unlock()
	if enabled && !runtimeSpatialGravityStableReleaseEnabled {
		return RuntimeSpatialGravityStatus{}, fmt.Errorf("%s", runtimePatchMonitorText(
			"正式版暂不开放重力抑制：工具异常退出时无法证明游戏原指令会自动恢复",
			"Gravity suppression is unavailable in the stable build because restoration after a forced app exit is not yet proven",
		))
	}
	if enabled {
		if err := a.ensureLiveMemoryWritesSafe(); err != nil {
			return RuntimeSpatialGravityStatus{}, err
		}
	}
	process := a.currentProcessInstance()
	if err := a.verifyRuntimePatchExecutableLocked(process, runtimePatchMonitorText("重力抑制", "Gravity suppression")); err != nil {
		return RuntimeSpatialGravityStatus{}, err
	}
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	memory := runtimePatchProcessMemory{handle: a.hProcess}
	layout, err := detectRuntimeGameLayout(remoteRuntimePatchPartyMemory{app: a}, a.moduleBase)
	if err != nil {
		return RuntimeSpatialGravityStatus{}, err
	}
	if a.runtimeSpatialGravityLease != nil {
		lease := *a.runtimeSpatialGravityLease
		if err := validateRuntimeSpatialGravityLease(lease, owner, process, a.moduleBase, layout); err != nil {
			return readRuntimeSpatialGravityStatus(memory, a.moduleBase, process, owner, a.runtimeSpatialGravityLease, layout), err
		}
		if enabled {
			status := readRuntimeSpatialGravityStatus(memory, a.moduleBase, process, owner, a.runtimeSpatialGravityLease, layout)
			if lease.State == runtimePatchPatchEnabled && status.Enabled && status.Available {
				return status, nil
			}
			return status, errors.Join(fmt.Errorf("%s", runtimePatchMonitorText("重力入口需要先恢复，不能重复启用", "The gravity entry requires recovery before it can be enabled again")), errLiveMemoryRollbackUnproven)
		}
		if err := a.restoreRuntimeSpatialGravityOwnedLocked(owner, false); err != nil {
			if errors.Is(err, errLiveMemoryRollbackUnproven) {
				a.poisonCurrentLiveMemoryWrites()
			}
			return readRuntimeSpatialGravityStatus(memory, a.moduleBase, process, owner, a.runtimeSpatialGravityLease, layout), err
		}
		return readRuntimeSpatialGravityStatus(memory, a.moduleBase, process, owner, nil, layout), nil
	}
	if !enabled {
		return readRuntimeSpatialGravityStatus(memory, a.moduleBase, process, owner, nil, layout), nil
	}
	moduleSize, err := getRemoteModuleSize(a.hProcess, a.moduleBase)
	if err != nil {
		return RuntimeSpatialGravityStatus{}, fmt.Errorf("%s: %w", runtimePatchMonitorText("读取游戏模块范围失败", "Read game module bounds"), err)
	}
	moduleEnd := a.moduleBase + moduleSize
	if moduleEnd < a.moduleBase {
		return RuntimeSpatialGravityStatus{}, fmt.Errorf("%s", runtimePatchMonitorText("游戏模块范围溢出", "The game module range overflowed"))
	}
	site, err := prepareRuntimeSpatialGravitySite(memory, a.moduleBase, moduleEnd, layout)
	if err != nil {
		return readRuntimeSpatialGravityStatus(memory, a.moduleBase, process, owner, nil, layout), err
	}
	if overlap := findRuntimePatchActiveAddressOverlap([]runtimePatchPatchSiteLease{site}, a.runtimePatchPatchLeases, runtimeSpatialGravityFeatureID); overlap != "" {
		return RuntimeSpatialGravityStatus{}, fmt.Errorf("%s %s", runtimePatchMonitorText("重力入口与当前补丁重叠：", "The gravity entry overlaps active patch:"), overlap)
	}
	candidate := runtimePatchPatchLease{
		FeatureID: runtimeSpatialGravityFeatureID, OwnerToken: owner, Process: process,
		State: runtimePatchPatchRecovery, Sites: []runtimePatchPatchSiteLease{site},
	}
	a.runtimeSpatialGravityLease = cloneRuntimeSpatialGravityLease(candidate)
	if err := installRuntimePatchSites(memory, candidate.Sites); err != nil {
		if errors.Is(err, errLiveMemoryRollbackUnproven) {
			a.poisonCurrentLiveMemoryWrites()
		} else {
			a.runtimeSpatialGravityLease = nil
		}
		return readRuntimeSpatialGravityStatus(memory, a.moduleBase, process, owner, a.runtimeSpatialGravityLease, layout), err
	}
	candidate.State = runtimePatchPatchEnabled
	a.runtimeSpatialGravityLease = cloneRuntimeSpatialGravityLease(candidate)
	return readRuntimeSpatialGravityStatus(memory, a.moduleBase, process, owner, a.runtimeSpatialGravityLease, layout), nil
}

func (a *App) restoreRuntimeSpatialGravityOwnedLocked(owner string, force bool) error {
	if a.runtimeSpatialGravityLease == nil {
		return nil
	}
	lease := *a.runtimeSpatialGravityLease
	if !force && !runtimeOwnerTokenMatches(lease.OwnerToken, owner) {
		return nil
	}
	process := a.currentProcessInstance()
	validationOwner := owner
	if force {
		validationOwner = lease.OwnerToken
	}
	layout, err := detectRuntimeGameLayout(remoteRuntimePatchPartyMemory{app: a}, a.moduleBase)
	if err != nil {
		return err
	}
	if err := validateRuntimeSpatialGravityLease(lease, validationOwner, process, a.moduleBase, layout); err != nil {
		return err
	}
	lease.State = runtimePatchPatchRecovery
	a.runtimeSpatialGravityLease = cloneRuntimeSpatialGravityLease(lease)
	if err := restoreRuntimePatchSites(runtimePatchProcessMemory{handle: a.hProcess}, lease.Sites); err != nil {
		return err
	}
	a.runtimeSpatialGravityLease = nil
	return nil
}

func (a *App) dropRuntimeSpatialGravityOwnerLocked(owner string) {
	if a.runtimeSpatialGravityLease != nil && runtimeOwnerTokenMatches(a.runtimeSpatialGravityLease.OwnerToken, owner) {
		a.runtimeSpatialGravityLease = nil
	}
}
