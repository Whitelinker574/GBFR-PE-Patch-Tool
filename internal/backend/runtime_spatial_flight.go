package backend

import (
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

const (
	runtimeSpatialExFallSubobjectOffset    = uintptr(0x430)
	runtimeSpatialControllerVelocityOffset = uintptr(0x24)
	runtimeSpatialControllerGroundedOffset = uintptr(0x39)
	runtimeSpatialExFallFloorQuerySlot     = uintptr(0x40)
	runtimeSpatialExFallFloorQueryRVA      = uintptr(0xA63230)
	runtimeSpatialFlightMinimumSpeed       = 0.1
	runtimeSpatialFlightMaximumSpeed       = 20.0
	runtimeSpatialAerialRecoveryDelay      = 650 * time.Millisecond
)

// runtimeSpatialFlightBinding pins every pointer that a height-anchor tick
// touches. The full chain is revalidated before each write so a character or
// scene change cannot redirect the writer to a stale allocation.
type runtimeSpatialFlightBinding struct {
	ModuleBase       uintptr
	Layout           runtimeGameLayout
	Root             uintptr
	Entity           uintptr
	EntityGeneration uint64
	Owner            uintptr
	ControllerState  uintptr
	Controller       uintptr
	TransformRoot    uintptr
	TransformNode    uintptr
}

type runtimeSpatialFlightAnchorState struct {
	Active              bool
	HeightAnchor        bool
	TakeoffSeen         bool
	AerialRecovery      bool
	AerialLandingSeen   bool
	AerialTrackedAction uint32
	AerialActionElapsed time.Duration
	GroundY             float32
	PeakY               float32
	PreviousY           float32
	TargetY             float32
	Binding             runtimeSpatialFlightBinding
}

type RuntimeSpatialFlightTickResult struct {
	OwnerLeaseID         string  `json:"ownerLeaseId"`
	PID                  uint32  `json:"pid"`
	ProcessCreated       uint64  `json:"processCreated,string"`
	GameVersion          string  `json:"gameVersion"`
	Source               string  `json:"source"`
	Mode                 string  `json:"mode"`
	VerticalSpeed        float32 `json:"verticalSpeed"`
	VerticalVelocity     float32 `json:"verticalVelocity"`
	CurrentHeight        float32 `json:"currentHeight"`
	TargetHeight         float32 `json:"targetHeight"`
	GroundHeight         float32 `json:"groundHeight"`
	Grounded             bool    `json:"grounded"`
	Anchored             bool    `json:"anchored"`
	Wrote                bool    `json:"wrote"`
	ActionID             uint32  `json:"actionId"`
	LastFloorQueryHit    bool    `json:"lastFloorQueryHit"`
	ContactTemplateReady bool    `json:"contactTemplateReady"`
	FloorQueries         uint64  `json:"floorQueries,string"`
	AcceptedContacts     uint64  `json:"acceptedContacts,string"`
	RecoveryActive       bool    `json:"recoveryActive"`
	SnapshotSequence     uint64  `json:"snapshotSequence,string"`
	Controller           uint64  `json:"controller"`
	EntityGeneration     uint64  `json:"entityGeneration"`
}

func updateRuntimeSpatialAerialRecovery(state runtimeSpatialFlightAnchorState, mode string, actionID uint32, grounded bool, verticalVelocity float32, verticalInput float64, interval time.Duration) runtimeSpatialFlightAnchorState {
	resetTracking := func() {
		state.AerialTrackedAction = 0
		state.AerialActionElapsed = 0
	}
	if mode != runtimeSpatialFlightModeAerial || !state.Active {
		state.AerialRecovery = false
		state.AerialLandingSeen = false
		resetTracking()
		return state
	}
	if state.AerialRecovery {
		if actionID == 3 {
			state.AerialLandingSeen = true
		}
		if state.AerialLandingSeen && (actionID == 0 || actionID == 1) {
			state.AerialRecovery = false
			state.AerialLandingSeen = false
			resetTracking()
		}
		return state
	}
	if grounded || verticalInput != 0 || verticalVelocity > 0.05 {
		resetTracking()
		return state
	}
	delay := time.Duration(0)
	switch actionID {
	case 4, 9:
		delay = runtimeSpatialAerialRecoveryDelay
	default:
		resetTracking()
		return state
	}
	if state.AerialTrackedAction != actionID {
		state.AerialTrackedAction = actionID
		state.AerialActionElapsed = interval
	} else {
		state.AerialActionElapsed += interval
	}
	if state.AerialActionElapsed >= delay {
		state.AerialRecovery = true
		state.AerialLandingSeen = false
	}
	return state
}

func resolveRuntimeSpatialFlightBinding(memory runtimePatchPartyMemory, moduleBase uintptr, layout runtimeGameLayout) (runtimeSpatialFlightBinding, error) {
	if memory == nil || moduleBase == 0 || !isKnownRuntimeGameLayout(layout) ||
		(layout.Version != "2.0.3" && layout.Version != "2.0.4" && layout.Version != "2.0.5") {
		return runtimeSpatialFlightBinding{}, fmt.Errorf("%s", runtimePatchMonitorText("当前游戏布局没有经过悬空飞行验证", "The current game layout is not verified for hover flight"))
	}
	root, err := verifyRuntimePatchPartyPointerSignatureForLayout(memory, moduleBase, layout)
	if err != nil {
		return runtimeSpatialFlightBinding{}, err
	}
	entity, err := readRuntimePatchPointer(memory, root)
	if err != nil || !plausibleRuntimePatchPartyPointer(entity) {
		return runtimeSpatialFlightBinding{}, fmt.Errorf("%s", runtimePatchMonitorText("当前场景没有可控制的本机角色", "The current scene does not expose a controllable local character"))
	}
	resolved, err := resolveRuntimePatchPartyLoadoutHandleWithLayout(memory, moduleBase, layout, 0)
	if err != nil || resolved.Entity != entity || resolved.ID == 0 {
		return runtimeSpatialFlightBinding{}, fmt.Errorf("%s: %w", runtimePatchMonitorText("本机角色句柄与实体世代不一致", "The local-character handle does not match the entity generation"), err)
	}
	owner, err := readRuntimeSpatialControllerPointer(memory, entity, runtimeSpatialControllerOwnerOffset, "controller owner")
	if err != nil {
		return runtimeSpatialFlightBinding{}, err
	}
	controllerState, err := readRuntimeSpatialControllerPointer(memory, owner, runtimeSpatialControllerStateOffset, "controller state")
	if err != nil {
		return runtimeSpatialFlightBinding{}, err
	}
	controller, ok := checkedRuntimePatchMonitorAddress(entity, runtimeSpatialExFallSubobjectOffset)
	if !ok {
		return runtimeSpatialFlightBinding{}, fmt.Errorf("ExFall subobject address overflow")
	}
	if err := validateRuntimeSpatialExFall(memory, moduleBase, controller, layout); err != nil {
		return runtimeSpatialFlightBinding{}, err
	}
	transformRoot, err := readRuntimePatchPointer(memory, entity+runtimePatchPartyTransformRootOffset)
	if err != nil || !plausibleRuntimePatchPartyPointer(transformRoot) {
		return runtimeSpatialFlightBinding{}, fmt.Errorf("%s", runtimePatchMonitorText("无法解析角色坐标根节点", "Could not resolve the character transform root"))
	}
	transformNode, err := readRuntimePatchPointer(memory, transformRoot+runtimePatchPartyTransformNodeOffset)
	if err != nil || !plausibleRuntimePatchPartyPointer(transformNode) {
		return runtimeSpatialFlightBinding{}, fmt.Errorf("%s", runtimePatchMonitorText("无法解析角色坐标节点", "Could not resolve the character transform node"))
	}
	return runtimeSpatialFlightBinding{
		ModuleBase: moduleBase, Layout: layout, Root: root, Entity: entity,
		EntityGeneration: resolved.ID, Owner: owner, ControllerState: controllerState, Controller: controller,
		TransformRoot: transformRoot, TransformNode: transformNode,
	}, nil
}

func validateRuntimeSpatialFlightBinding(memory runtimePatchPartyMemory, binding runtimeSpatialFlightBinding) error {
	entity, err := readRuntimePatchPointer(memory, binding.Root)
	if err != nil || entity != binding.Entity {
		return fmt.Errorf("%s", runtimePatchMonitorText("飞行期间角色实体已切换，请等待场景稳定", "The character entity changed during flight; wait for the scene to stabilize"))
	}
	resolved, err := resolveRuntimePatchPartyLoadoutHandleWithLayout(memory, binding.ModuleBase, binding.Layout, 0)
	if err != nil || resolved.Entity != binding.Entity || resolved.ID != binding.EntityGeneration {
		return fmt.Errorf("%s", runtimePatchMonitorText("飞行期间角色实体世代已切换，请等待重新绑定", "The character generation changed during flight; wait for rebinding"))
	}
	owner, err := readRuntimeSpatialControllerPointer(memory, binding.Entity, runtimeSpatialControllerOwnerOffset, "controller owner")
	if err != nil || owner != binding.Owner {
		return fmt.Errorf("%s", runtimePatchMonitorText("飞行期间 controller owner 已切换，请等待重新绑定", "The controller owner changed during flight; wait for rebinding"))
	}
	controllerState, err := readRuntimeSpatialControllerPointer(memory, binding.Owner, runtimeSpatialControllerStateOffset, "controller state")
	if err != nil || controllerState != binding.ControllerState {
		return fmt.Errorf("%s", runtimePatchMonitorText("飞行期间 controller 实例已切换，请等待重新绑定", "The controller instance changed during flight; wait for rebinding"))
	}
	controller, ok := checkedRuntimePatchMonitorAddress(binding.Entity, runtimeSpatialExFallSubobjectOffset)
	if !ok || controller != binding.Controller {
		return fmt.Errorf("%s", runtimePatchMonitorText("飞行期间 ExFall 子对象已经变化，请等待重新绑定", "The ExFall subobject changed during flight; wait for rebinding"))
	}
	if err := validateRuntimeSpatialExFall(memory, binding.ModuleBase, binding.Controller, binding.Layout); err != nil {
		return err
	}
	transformRoot, err := readRuntimePatchPointer(memory, binding.Entity+runtimePatchPartyTransformRootOffset)
	if err != nil || transformRoot != binding.TransformRoot {
		return fmt.Errorf("%s", runtimePatchMonitorText("飞行期间角色 transform root 已切换，请等待重新绑定", "The character transform root changed during flight; wait for rebinding"))
	}
	transformNode, err := readRuntimePatchPointer(memory, binding.TransformRoot+runtimePatchPartyTransformNodeOffset)
	if err != nil || transformNode != binding.TransformNode {
		return fmt.Errorf("%s", runtimePatchMonitorText("飞行期间角色 transform node 已切换，请等待重新绑定", "The character transform node changed during flight; wait for rebinding"))
	}
	return nil
}

func runtimeSpatialFlightRVAForLayout(layout runtimeGameLayout) uintptr {
	if layout.Version == "2.0.5" {
		return 0xA63BE0
	}
	if layout.Version == "2.0.4" {
		return runtimeSpatialExFallFloorQueryRVA + 0xFA0
	}
	return runtimeSpatialExFallFloorQueryRVA
}

func validateRuntimeSpatialExFall(memory runtimePatchPartyMemory, moduleBase, controller uintptr, layout runtimeGameLayout) error {
	if memory == nil || moduleBase == 0 || controller == 0 {
		return fmt.Errorf("invalid ExFall validation parameters")
	}
	vtable, err := readRuntimePatchPointer(memory, controller)
	if err != nil || !plausibleRuntimePatchPartyPointer(vtable) {
		return fmt.Errorf("%s", runtimePatchMonitorText("角色 ExFall vtable 不可用", "The character ExFall vtable is unavailable"))
	}
	floorQuery, err := readRuntimePatchPointer(memory, vtable+runtimeSpatialExFallFloorQuerySlot)
	want, ok := checkedRuntimePatchMonitorAddress(moduleBase, runtimeSpatialFlightRVAForLayout(layout))
	if err != nil || !ok || floorQuery != want {
		return fmt.Errorf("%s", runtimePatchMonitorText("角色 ExFall 落地查询入口与当前 2.0.3 / 2.0.4 / 2.0.5 布局不匹配", "The character ExFall floor-query entry does not match the current 2.0.3/2.0.4/2.0.5 layout"))
	}
	return nil
}

func readRuntimeSpatialFlightFloat(memory runtimePatchPartyMemory, address uintptr, label string) (float32, error) {
	raw := make([]byte, 4)
	if err := memory.ReadAt(address, raw); err != nil {
		return 0, fmt.Errorf("read %s: %w", label, err)
	}
	value := math.Float32frombits(binary.LittleEndian.Uint32(raw))
	if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) || float32(math.Abs(float64(value))) > runtimePatchPartyMaximumCoordinateMagnitude {
		return 0, fmt.Errorf("%s", runtimePatchMonitorText("角色高度不是有效数值", "The character height is not a valid value"))
	}
	return value, nil
}

func applyRuntimeSpatialFlightAnchorTickResolved(memory runtimeSpatialMemory, binding runtimeSpatialFlightBinding, state runtimeSpatialFlightAnchorState, verticalSpeed float64, interval time.Duration) (runtimeSpatialFlightAnchorState, RuntimeSpatialFlightTickResult, error) {
	return applyRuntimeSpatialFlightAnchorTickResolvedMode(memory, binding, state, verticalSpeed, interval, runtimeSpatialFlightModeAerial)
}

func applyRuntimeSpatialFlightAnchorTickResolvedMode(memory runtimeSpatialMemory, binding runtimeSpatialFlightBinding, state runtimeSpatialFlightAnchorState, verticalSpeed float64, interval time.Duration, mode string) (runtimeSpatialFlightAnchorState, RuntimeSpatialFlightTickResult, error) {
	if memory == nil || interval <= 0 || !isFiniteFloat64(verticalSpeed) || math.Abs(verticalSpeed) > runtimeSpatialFlightMaximumSpeed {
		return state, RuntimeSpatialFlightTickResult{}, fmt.Errorf("%s", runtimePatchMonitorText("飞行高度控制参数无效", "Invalid flight height-control parameters"))
	}
	flightMode, err := normalizeRuntimeSpatialFlightMode(mode)
	if err != nil {
		return state, RuntimeSpatialFlightTickResult{}, err
	}
	if err := validateRuntimeSpatialFlightBinding(memory, binding); err != nil {
		return state, RuntimeSpatialFlightTickResult{}, err
	}
	groundedAddress := binding.Controller + runtimeSpatialControllerGroundedOffset
	groundedRaw := []byte{0}
	if err := memory.ReadAt(groundedAddress, groundedRaw); err != nil {
		return state, RuntimeSpatialFlightTickResult{}, fmt.Errorf("read controller grounded state: %w", err)
	}
	heightAddress := binding.TransformNode + runtimePatchPartyPositionYOffset
	currentY, err := readRuntimeSpatialFlightFloat(memory, heightAddress, "character height")
	if err != nil {
		return state, RuntimeSpatialFlightTickResult{}, err
	}
	result := RuntimeSpatialFlightTickResult{
		GameVersion: binding.Layout.Version, Source: "game_runtime_spatial_flight_anchor_" + binding.Layout.Version,
		VerticalSpeed: float32(verticalSpeed), CurrentHeight: currentY, Grounded: groundedRaw[0] != 0,
		Controller: uint64(binding.Controller), EntityGeneration: binding.EntityGeneration,
	}
	verticalVelocity, err := readRuntimeSpatialFlightFloat(memory, binding.Controller+runtimeSpatialControllerVelocityOffset, "controller vertical velocity")
	if err != nil {
		return state, RuntimeSpatialFlightTickResult{}, err
	}
	result.VerticalVelocity = verticalVelocity
	if verticalSpeed > 0 {
		result.Mode = "ascend"
	} else if verticalSpeed < 0 {
		result.Mode = "descend"
	} else {
		result.Mode = "hover"
	}
	if !state.Active && result.Grounded && verticalSpeed == 0 {
		state = runtimeSpatialFlightAnchorState{GroundY: currentY, PeakY: currentY, PreviousY: currentY, TargetY: currentY, Binding: binding}
		result.GroundHeight = state.GroundY
		result.TargetHeight = state.TargetY
		return state, result, nil
	}
	if !state.Active && verticalSpeed == 0 {
		if flightMode == runtimeSpatialFlightModeVirtualGround {
			// Virtual ground is raised directly with PageUp from a real grounded
			// contact. A normal jump must never arm the apex-freeze path: once
			// ExActJump owns the action, replaying a floor query does not reliably
			// return every character to Wait/Run.
			state.TakeoffSeen = false
			state.HeightAnchor = false
			state.PreviousY = currentY
			state.Binding = binding
			result.GroundHeight = state.GroundY
			result.TargetHeight = state.TargetY
			return state, result, nil
		}
		if !state.TakeoffSeen {
			state.TakeoffSeen = true
			state.PeakY = currentY
			state.PreviousY = currentY
			state.TargetY = currentY
			state.Binding = binding
			result.GroundHeight = state.GroundY
			result.TargetHeight = state.TargetY
			return state, result, nil
		}
		if currentY > state.PeakY {
			state.PeakY = currentY
		}
		stillRising := verticalVelocity > 0.001 || currentY > state.PreviousY+0.001
		state.PreviousY = currentY
		state.Binding = binding
		if stillRising {
			result.GroundHeight = state.GroundY
			result.TargetHeight = state.PeakY
			return state, result, nil
		}
		state.Active = true
		state.TargetY = state.PeakY
	}
	if !state.Active {
		// Explicit PageUp/PageDown starts direct vertical control immediately.
		state.Active = true
		state.TakeoffSeen = true
		state.GroundY = currentY
		state.PeakY = currentY
		state.PreviousY = currentY
		state.TargetY = currentY
	}
	state.Binding = binding
	state.HeightAnchor = flightMode == runtimeSpatialFlightModeAerial || verticalSpeed != 0
	delta := float32(verticalSpeed * interval.Seconds())
	state.TargetY += delta
	if state.TargetY < state.GroundY {
		state.TargetY = state.GroundY
	}
	result.Anchored = true
	result.GroundHeight = state.GroundY
	result.TargetHeight = state.TargetY
	return state, result, nil
}

func primeRuntimeSpatialFlightTakeoff(memory runtimeSpatialMemory, binding runtimeSpatialFlightBinding, previous, current runtimeSpatialFlightAnchorState, result RuntimeSpatialFlightTickResult, verticalSpeed float64) (bool, error) {
	if memory == nil || previous.Active || !current.Active || !result.Grounded || verticalSpeed <= 0 {
		return false, nil
	}
	heightAddress := binding.TransformNode + runtimePatchPartyPositionYOffset
	encoded := make([]byte, 4)
	binary.LittleEndian.PutUint32(encoded, math.Float32bits(current.TargetY))
	if err := memory.WriteAt(heightAddress, encoded); err != nil {
		return false, fmt.Errorf("%s: %w", runtimePatchMonitorText("起飞高度写入失败", "Takeoff height write failed"), err)
	}
	observed, err := readRuntimeSpatialFlightFloat(memory, heightAddress, "takeoff height readback")
	if err != nil || math.Abs(float64(observed-current.TargetY)) > 0.0001 {
		return false, fmt.Errorf("%s: target=%v observed=%v error=%v", runtimePatchMonitorText("起飞高度写后回读不一致", "Takeoff height readback mismatch"), current.TargetY, observed, err)
	}
	return true, nil
}

func (a *App) RuntimeSpatialFlightAnchorTickOwned(owner string, verticalSpeed float64, intervalMilliseconds int64) (RuntimeSpatialFlightTickResult, error) {
	if intervalMilliseconds <= 0 || intervalMilliseconds > 1000 {
		return RuntimeSpatialFlightTickResult{}, fmt.Errorf("invalid flight watcher interval")
	}
	liveMemoryWriteMu.Lock()
	defer liveMemoryWriteMu.Unlock()
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, owner); err != nil {
		return RuntimeSpatialFlightTickResult{}, err
	}
	defer a.procMu.Unlock()
	if err := a.ensureLiveMemoryWritesSafe(); err != nil {
		return RuntimeSpatialFlightTickResult{}, err
	}
	process := a.currentProcessInstance()
	if err := a.verifyRuntimePatchExecutableLocked(process, runtimePatchMonitorText("角色悬空飞行", "Character hover flight")); err != nil {
		return RuntimeSpatialFlightTickResult{}, err
	}
	layout, err := detectRuntimeGameLayout(remoteRuntimePatchPartyMemory{app: a}, a.moduleBase)
	if err != nil {
		return RuntimeSpatialFlightTickResult{}, err
	}
	a.runtimeSpatialHotkeyMu.Lock()
	config := a.runtimeSpatialHotkey
	a.runtimeSpatialHotkeyMu.Unlock()
	if !config.Enabled || !config.FlightEnabled || !runtimeOwnerTokenMatches(config.Owner, owner) || config.Process != process {
		return RuntimeSpatialFlightTickResult{}, errRuntimeOwnerLeaseStale
	}
	memory := remoteRuntimeSpatialMemory{app: a}
	state := config.FlightAnchor
	binding := state.Binding
	if binding.ModuleBase != a.moduleBase || binding.Layout.Version != layout.Version || validateRuntimeSpatialFlightBinding(memory, binding) != nil {
		binding, err = resolveRuntimeSpatialFlightBinding(memory, a.moduleBase, layout)
		if err != nil {
			return RuntimeSpatialFlightTickResult{}, err
		}
		state = runtimeSpatialFlightAnchorState{Binding: binding}
	}
	previousState := state
	state, result, err := applyRuntimeSpatialFlightAnchorTickResolvedMode(memory, binding, state, verticalSpeed, time.Duration(intervalMilliseconds)*time.Millisecond, config.FlightMode)
	if err != nil {
		return RuntimeSpatialFlightTickResult{}, err
	}
	lifted := false
	if config.FlightMode != runtimeSpatialFlightModeVirtualGround {
		lifted, err = primeRuntimeSpatialFlightTakeoff(memory, binding, previousState, state, result, verticalSpeed)
		if err != nil {
			return RuntimeSpatialFlightTickResult{}, err
		}
	}
	actionRaw := make([]byte, 4)
	if err := memory.ReadAt(binding.Owner+0x40, actionRaw); err != nil {
		return RuntimeSpatialFlightTickResult{}, fmt.Errorf("read current character action: %w", err)
	}
	result.ActionID = binary.LittleEndian.Uint32(actionRaw)
	state = updateRuntimeSpatialAerialRecovery(state, config.FlightMode, result.ActionID, result.Grounded, result.VerticalVelocity, verticalSpeed, time.Duration(intervalMilliseconds)*time.Millisecond)
	result.RecoveryActive = state.AerialRecovery
	a.runtimePatchMu.Lock()
	hookErr := a.syncRuntimeSpatialFlightHookLocked(owner, binding, state, config.FlightMode)
	if hookErr == nil && a.runtimeSpatialFlightHookLease != nil {
		diagnostics, diagnosticsErr := readRuntimeSpatialFlightHookDiagnostics(memory, a.runtimeSpatialFlightHookLease.CaveAddr)
		if diagnosticsErr != nil {
			hookErr = diagnosticsErr
		} else {
			result.LastFloorQueryHit = diagnostics.LastQueryHit
			result.ContactTemplateReady = diagnostics.ContactTemplateReady
			result.FloorQueries = diagnostics.FloorQueries
			result.AcceptedContacts = diagnostics.AcceptedContacts
			result.SnapshotSequence = diagnostics.SnapshotSequence
		}
	}
	a.runtimePatchMu.Unlock()
	if hookErr != nil {
		return RuntimeSpatialFlightTickResult{}, hookErr
	}
	result.Wrote = state.Active || lifted
	result.OwnerLeaseID = owner
	result.PID = a.charaPID
	result.ProcessCreated = a.charaCreated
	a.runtimeSpatialHotkeyMu.Lock()
	if !a.runtimeSpatialHotkey.Enabled || !a.runtimeSpatialHotkey.FlightEnabled || a.runtimeSpatialHotkey.Owner != config.Owner || a.runtimeSpatialHotkey.Process != config.Process {
		a.runtimeSpatialHotkeyMu.Unlock()
		return RuntimeSpatialFlightTickResult{}, errRuntimeOwnerLeaseStale
	}
	a.runtimeSpatialHotkey.FlightAnchor = state
	a.runtimeSpatialHotkey.FlightLastTick = result
	a.runtimeSpatialHotkeyMu.Unlock()
	return result, nil
}
