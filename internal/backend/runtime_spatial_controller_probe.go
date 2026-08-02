package backend

import (
	"encoding/binary"
	"fmt"
	"math"
)

const (
	// The verified 2.0.3 party entity exposes the controller owner at +0xC0.
	// +0x10 belongs to unrelated entity metadata and can contain the sentinel
	// 0xFFFFFFFF00000000; treating it as a pointer was caught by the live
	// read-only probe before any flight write path existed.
	runtimeSpatialControllerOwnerOffset = uintptr(0xC0)
	runtimeSpatialControllerStateOffset = uintptr(0x3218)
	runtimeSpatialControllerFieldBytes  = 8
)

var runtimeSpatialControllerCandidateOffsets = [...]uintptr{
	0x40,
	0x454,
	0x469,
	0x2AD8,
	0x59D4,
	0x5D40,
}

// RuntimeSpatialControllerProbeField deliberately exposes several neutral
// interpretations of the same eight read-only bytes. The offsets are research
// candidates, not named game fields; callers must correlate successive samples
// before assigning semantics to any value.
type RuntimeSpatialControllerProbeField struct {
	Offset      uint64   `json:"offset"`
	Address     uint64   `json:"address"`
	RawBytes    string   `json:"rawBytes"`
	UInt32      uint32   `json:"uint32"`
	Int32       int32    `json:"int32"`
	UInt64      uint64   `json:"uint64"`
	Float32     *float32 `json:"float32,omitempty"`
	Float32Kind string   `json:"float32Kind"`
}

// RuntimeSpatialControllerProbe is a bounded, read-only sample tied to one
// verified Chara owner, process instance, and validated local-player handle ID.
// EntityGeneration is the party handle ID revalidated after all candidate reads.
type RuntimeSpatialControllerProbe struct {
	OwnerToken        string                               `json:"ownerToken"`
	PID               uint32                               `json:"pid"`
	ProcessCreated    uint64                               `json:"processCreated"`
	GameVersion       string                               `json:"gameVersion"`
	Source            string                               `json:"source"`
	SnapshotCount     int                                  `json:"snapshotCount"`
	RuntimeVerified   bool                                 `json:"runtimeVerified"`
	RootAddress       uint64                               `json:"rootAddress"`
	EntityAddress     uint64                               `json:"entityAddress"`
	EntityGeneration  uint64                               `json:"entityGeneration"`
	OwnerAddress      uint64                               `json:"ownerAddress"`
	ControllerAddress uint64                               `json:"controllerAddress"`
	CurrentY          float32                              `json:"currentY"`
	Fields            []RuntimeSpatialControllerProbeField `json:"fields"`
}

func runtimeSpatialControllerFloat(bits uint32) (*float32, string) {
	value := math.Float32frombits(bits)
	switch {
	case math.IsNaN(float64(value)):
		return nil, "nan"
	case math.IsInf(float64(value), 1):
		return nil, "+inf"
	case math.IsInf(float64(value), -1):
		return nil, "-inf"
	default:
		copy := value
		return &copy, "finite"
	}
}

func readRuntimeSpatialControllerPointer(memory runtimePatchPartyMemory, base, offset uintptr, label string) (uintptr, error) {
	address, ok := checkedRuntimePatchMonitorAddress(base, offset)
	if !ok {
		return 0, fmt.Errorf("%s address overflow", label)
	}
	value, err := readRuntimePatchPointer(memory, address)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", label, err)
	}
	if !plausibleRuntimePatchPartyPointer(value) {
		return 0, fmt.Errorf("%s pointer is unavailable or invalid: 0x%X", label, value)
	}
	return value, nil
}

func readRuntimeSpatialControllerProbe(memory runtimePatchPartyMemory, moduleBase uintptr) (RuntimeSpatialControllerProbe, error) {
	if memory == nil || moduleBase == 0 {
		return RuntimeSpatialControllerProbe{}, fmt.Errorf("%s", runtimePatchMonitorText("飞行控制器探针参数无效", "Invalid flight-controller probe parameters"))
	}

	var frame runtimePatchPartySnapshot
	stable, err := readStableRuntimePatchPartySnapshots(func() (runtimePatchPartySnapshot, error) {
		current, readErr := readRuntimePatchPartySnapshot(memory, moduleBase)
		if readErr == nil {
			frame = current
		}
		return current, readErr
	})
	if err != nil {
		return RuntimeSpatialControllerProbe{}, err
	}
	if len(stable.Entities) == 0 || !stable.Entities[0].Present || frame.Topology.Entities[0] == 0 {
		return RuntimeSpatialControllerProbe{}, fmt.Errorf("%s", runtimePatchMonitorText("当前场景没有可探测的本机角色实体", "The current scene does not expose a local-player entity for probing"))
	}
	entity := frame.Topology.Entities[0]
	generation := frame.Topology.LoadoutHandleIDs[0]
	if generation == 0 {
		return RuntimeSpatialControllerProbe{}, fmt.Errorf("%s", runtimePatchMonitorText("本机角色实体世代无法通过队伍句柄验证", "The local-player entity generation could not be validated through the party handle"))
	}

	owner, err := readRuntimeSpatialControllerPointer(memory, entity, runtimeSpatialControllerOwnerOffset, "controller owner")
	if err != nil {
		return RuntimeSpatialControllerProbe{}, err
	}
	controller, err := readRuntimeSpatialControllerPointer(memory, owner, runtimeSpatialControllerStateOffset, "controller state")
	if err != nil {
		return RuntimeSpatialControllerProbe{}, err
	}

	fields := make([]RuntimeSpatialControllerProbeField, 0, len(runtimeSpatialControllerCandidateOffsets))
	for _, offset := range runtimeSpatialControllerCandidateOffsets {
		address, ok := checkedRuntimePatchMonitorAddress(controller, offset)
		if !ok {
			return RuntimeSpatialControllerProbe{}, fmt.Errorf("controller candidate +0x%X address overflow", offset)
		}
		raw := make([]byte, runtimeSpatialControllerFieldBytes)
		if err := memory.ReadAt(address, raw); err != nil {
			return RuntimeSpatialControllerProbe{}, fmt.Errorf("read controller candidate +0x%X: %w", offset, err)
		}
		bits := binary.LittleEndian.Uint32(raw[:4])
		floatValue, floatKind := runtimeSpatialControllerFloat(bits)
		fields = append(fields, RuntimeSpatialControllerProbeField{
			Offset: uint64(offset), Address: uint64(address), RawBytes: bytesToHex(raw),
			UInt32: bits, Int32: int32(bits), UInt64: binary.LittleEndian.Uint64(raw),
			Float32: floatValue, Float32Kind: floatKind,
		})
	}

	// Revalidate the exact root slot, pointer chain, and handle ID after reading
	// all dynamic fields. Candidate values may change, but their owner topology
	// must not be spliced across two entity generations.
	currentEntity, err := readRuntimePatchPointer(memory, frame.Topology.Root)
	if err != nil || currentEntity != entity {
		return RuntimeSpatialControllerProbe{}, fmt.Errorf("%s", runtimePatchMonitorText("探针采样期间本机角色实体发生变化，请重试", "The local-player entity changed during the probe; retry"))
	}
	currentOwner, err := readRuntimeSpatialControllerPointer(memory, entity, runtimeSpatialControllerOwnerOffset, "controller owner")
	if err != nil || currentOwner != owner {
		return RuntimeSpatialControllerProbe{}, fmt.Errorf("%s", runtimePatchMonitorText("探针采样期间控制器 owner 发生变化，请重试", "The controller owner changed during the probe; retry"))
	}
	currentController, err := readRuntimeSpatialControllerPointer(memory, owner, runtimeSpatialControllerStateOffset, "controller state")
	if err != nil || currentController != controller {
		return RuntimeSpatialControllerProbe{}, fmt.Errorf("%s", runtimePatchMonitorText("探针采样期间控制器实例发生变化，请重试", "The controller instance changed during the probe; retry"))
	}
	layout, err := detectRuntimeGameLayout(memory, moduleBase)
	if err != nil {
		return RuntimeSpatialControllerProbe{}, err
	}
	resolved, err := resolveRuntimePatchPartyLoadoutHandleWithLayout(memory, moduleBase, layout, 0)
	if err != nil || resolved.Entity != entity || resolved.ID != generation {
		return RuntimeSpatialControllerProbe{}, fmt.Errorf("%s", runtimePatchMonitorText("探针采样期间本机角色实体世代发生变化，请重试", "The local-player entity generation changed during the probe; retry"))
	}

	return RuntimeSpatialControllerProbe{
		GameVersion: stable.GameVersion, Source: "game_runtime_spatial_controller_probe_" + stable.GameVersion,
		SnapshotCount: stable.SnapshotCount, RuntimeVerified: stable.RuntimeVerified,
		RootAddress: uint64(frame.Topology.Root), EntityAddress: uint64(entity), EntityGeneration: generation,
		OwnerAddress: uint64(owner), ControllerAddress: uint64(controller), CurrentY: stable.Entities[0].Position.Y,
		Fields: fields,
	}, nil
}

// RuntimeSpatialControllerProbeOwned performs bounded reads only. The existing
// Chara owner pins hProcess/moduleBase/{PID, Created} for the entire sample.
func (a *App) RuntimeSpatialControllerProbeOwned(token string) (RuntimeSpatialControllerProbe, error) {
	if err := a.acquireOwnedRuntimeWriteLease(runtimeOwnerChara, token); err != nil {
		return RuntimeSpatialControllerProbe{}, err
	}
	defer a.procMu.Unlock()

	result, err := readRuntimeSpatialControllerProbe(remoteRuntimePatchPartyMemory{app: a}, a.moduleBase)
	if err != nil {
		return RuntimeSpatialControllerProbe{}, err
	}
	result.OwnerToken = token
	result.PID = a.charaPID
	result.ProcessCreated = a.charaCreated
	return result, nil
}
