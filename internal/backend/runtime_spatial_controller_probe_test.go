package backend

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func installRuntimeSpatialControllerProbeFixture(memory *fakeRuntimePanelMemory) (uintptr, uintptr) {
	entity := uintptr(0x21000000)
	owner := uintptr(0x31000000)
	controller := uintptr(0x32000000)
	memory.putPtr(entity+runtimeSpatialControllerOwnerOffset, owner)
	memory.putPtr(owner+runtimeSpatialControllerStateOffset, controller)
	for index, offset := range runtimeSpatialControllerCandidateOffsets {
		raw := make([]byte, runtimeSpatialControllerFieldBytes)
		binary.LittleEndian.PutUint32(raw[:4], math.Float32bits(float32(index)+0.5))
		binary.LittleEndian.PutUint32(raw[4:], uint32(0xA0+index))
		memory.put(controller+offset, raw)
	}
	return owner, controller
}

func TestReadRuntimeSpatialControllerProbeReturnsBoundedRawInterpretations(t *testing.T) {
	memory, moduleBase := newRuntimePatchPartyFixtureForLayout(t, runtimeGameLayouts[1])
	owner, controller := installRuntimeSpatialControllerProbeFixture(memory)

	result, err := readRuntimeSpatialControllerProbe(memory, moduleBase)
	if err != nil {
		t.Fatal(err)
	}
	if result.GameVersion != "2.0.3" || result.Source != "game_runtime_spatial_controller_probe_2.0.3" || !result.RuntimeVerified || result.SnapshotCount != runtimePatchPartySnapshotCount {
		t.Fatalf("verification metadata=%+v", result)
	}
	if result.EntityAddress != 0x21000000 || result.EntityGeneration != 0xA000 || result.OwnerAddress != uint64(owner) || result.ControllerAddress != uint64(controller) || result.CurrentY != 20 {
		t.Fatalf("identity binding=%+v", result)
	}
	if len(result.Fields) != len(runtimeSpatialControllerCandidateOffsets) {
		t.Fatalf("field count=%d", len(result.Fields))
	}
	for index, field := range result.Fields {
		wantFloat := float32(index) + 0.5
		if field.Offset != uint64(runtimeSpatialControllerCandidateOffsets[index]) || field.Address != uint64(controller+runtimeSpatialControllerCandidateOffsets[index]) || field.Float32 == nil || *field.Float32 != wantFloat || field.Float32Kind != "finite" {
			t.Fatalf("field %d=%+v", index, field)
		}
		if field.UInt32 != math.Float32bits(wantFloat) || field.Int32 != int32(field.UInt32) || field.RawBytes == "" {
			t.Fatalf("field %d interpretations=%+v", index, field)
		}
	}
}

func TestReadRuntimeSpatialControllerProbePreservesNonFiniteRawValues(t *testing.T) {
	memory, moduleBase := newRuntimePatchPartyFixture(t)
	_, controller := installRuntimeSpatialControllerProbeFixture(memory)
	raw := make([]byte, runtimeSpatialControllerFieldBytes)
	binary.LittleEndian.PutUint32(raw[:4], math.Float32bits(float32(math.Inf(1))))
	memory.put(controller+runtimeSpatialControllerCandidateOffsets[0], raw)

	result, err := readRuntimeSpatialControllerProbe(memory, moduleBase)
	if err != nil {
		t.Fatal(err)
	}
	if result.Fields[0].Float32 != nil || result.Fields[0].Float32Kind != "+inf" || result.Fields[0].RawBytes == "" {
		t.Fatalf("non-finite field was not kept JSON-safe: %+v", result.Fields[0])
	}
}

func TestReadRuntimeSpatialControllerProbeRejectsBrokenPointerChain(t *testing.T) {
	memory, moduleBase := newRuntimePatchPartyFixture(t)
	memory.putPtr(uintptr(0x21000000)+runtimeSpatialControllerOwnerOffset, 0)

	_, err := readRuntimeSpatialControllerProbe(memory, moduleBase)
	if err == nil || !strings.Contains(err.Error(), "controller owner") {
		t.Fatalf("broken owner pointer error=%v", err)
	}
}

func TestReadRuntimeSpatialControllerProbeRejectsEntityGenerationChange(t *testing.T) {
	memory, moduleBase := newRuntimePatchPartyFixture(t)
	installRuntimeSpatialControllerProbeFixture(memory)
	// The stable snapshot stores handle ID 0xA000. Changing the ID array after
	// those snapshots is simulated by a memory wrapper that mutates on the first
	// candidate read.
	wrapped := &runtimeSpatialControllerGenerationChangeMemory{
		fakeRuntimePanelMemory: memory,
		triggerAddress:         uintptr(0x32000000) + runtimeSpatialControllerCandidateOffsets[0],
		moduleBase:             moduleBase,
		layout:                 runtimeGameLayouts[0],
	}
	_, err := readRuntimeSpatialControllerProbe(wrapped, moduleBase)
	if err == nil || !strings.Contains(err.Error(), "generation") && !strings.Contains(err.Error(), "世代") {
		t.Fatalf("generation change error=%v", err)
	}
}

type runtimeSpatialControllerGenerationChangeMemory struct {
	*fakeRuntimePanelMemory
	triggerAddress uintptr
	moduleBase     uintptr
	layout         runtimeGameLayout
	changed        bool
}

func (memory *runtimeSpatialControllerGenerationChangeMemory) ReadAt(address uintptr, destination []byte) error {
	err := memory.fakeRuntimePanelMemory.ReadAt(address, destination)
	if err == nil && address == memory.triggerAddress && !memory.changed {
		memory.changed = true
		entityTable, _ := readRuntimePatchPointer(memory.fakeRuntimePanelMemory, memory.moduleBase+memory.layout.PartyEntityTableRVA)
		idArray, _ := readRuntimePatchPointer(memory.fakeRuntimePanelMemory, entityTable+runtimePatchPartyIDArrayOffset)
		memory.putU64(idArray, 0xDEADBEEF)
	}
	return err
}
