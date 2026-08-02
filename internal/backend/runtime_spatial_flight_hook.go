package backend

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"unsafe"
)

const (
	runtimeSpatialFlightHookRVA                    = uintptr(0xA63230)
	runtimeSpatialFlightHookSize                   = 15
	runtimeSpatialFlightCaveSize                   = 0x3E0
	runtimeSpatialFlightMarkerOffset               = 0x280
	runtimeSpatialFlightControllerDataOffset       = 0x288
	runtimeSpatialFlightHeightDataOffset           = 0x290
	runtimeSpatialFlightTargetDataOffset           = 0x298
	runtimeSpatialFlightActiveDataOffset           = 0x29C
	runtimeSpatialFlightVirtualModeDataOffset      = 0x29D
	runtimeSpatialFlightSnapshotReturnOffset       = 0x29E
	runtimeSpatialFlightContactTemplateValidOffset = 0x29F
	runtimeSpatialFlightHitCountDataOffset         = 0x2A0
	runtimeSpatialFlightAcceptCountDataOffset      = 0x2A8
	runtimeSpatialFlightSnapshotSeqOffset          = 0x2B0
	runtimeSpatialFlightContactClearanceOffset     = 0x2B8
	runtimeSpatialFlightHeightAnchorDataOffset     = 0x2BC
	runtimeSpatialFlightClearVelocityDataOffset    = 0x2BD
	runtimeSpatialFlightSnapshotDataOffset         = 0x2C0
	runtimeSpatialFlightSnapshotSize               = 0x60
	runtimeSpatialFlightContactTemplateOffset      = 0x320
	runtimeSpatialFlightTrampolineOffset           = 0x3A0
	runtimeSpatialVirtualGroundHookRVA             = uintptr(0x26F455C)
	runtimeSpatialVirtualGroundHookSize            = 15
	runtimeSpatialVirtualGroundMarkerOffset        = 0x50
)

var (
	runtimeSpatialFlightHookOriginal        = []byte{0x48, 0x83, 0xEC, 0x38, 0x4C, 0x89, 0xC8, 0xC7, 0x44, 0x24, 0x28, 0x00, 0x00, 0x00, 0x00}
	runtimeSpatialFlightMarker              = [...]byte{'G', 'B', 'F', 'R', 'F', 'L', 'Y', '5'}
	runtimeSpatialFlightContactClearance    = float32(0.011)
	runtimeSpatialVirtualGroundHookOriginal = []byte{0x48, 0x8B, 0x87, 0x18, 0x32, 0x00, 0x00, 0xC5, 0xFA, 0x10, 0x80, 0x54, 0x04, 0x00, 0x00}
	runtimeSpatialVirtualGroundMarker       = [...]byte{'G', 'B', 'F', 'R', 'G', 'R', 'D', '1'}
)

type runtimeSpatialFloorQuerySnapshot struct {
	Sequence uint64
	Returned bool
	Data     [runtimeSpatialFlightSnapshotSize]byte
}

type runtimeSpatialFlightHookDiagnostics struct {
	LastQueryHit         bool
	ContactTemplateReady bool
	FloorQueries         uint64
	AcceptedContacts     uint64
	SnapshotSequence     uint64
}

type runtimeSpatialFlightHookLease struct {
	OwnerToken    string
	Process       processInstanceID
	EntryAddr     uintptr
	Original      []byte
	Installed     []byte
	CaveAddr      uintptr
	Controller    uintptr
	HeightAddr    uintptr
	TargetHeight  float32
	ContactReplay bool
}

func buildRuntimeSpatialFlightCave(cave, originalFunction, controller, heightAddr uintptr, targetHeight float32, active, contactReplay, clearVelocity, heightAnchor bool) ([]byte, error) {
	if cave == 0 || originalFunction == 0 || controller == 0 || heightAddr == 0 ||
		math.IsNaN(float64(targetHeight)) || math.IsInf(float64(targetHeight), 0) {
		return nil, fmt.Errorf("invalid runtime spatial flight cave parameters")
	}
	code := bytes.Repeat([]byte{0x90}, runtimeSpatialFlightCaveSize)
	position := 0
	appendBytes := func(values ...byte) { copy(code[position:], values); position += len(values) }
	appendRel32 := func(opcode byte, target uintptr) error {
		appendBytes(opcode)
		delta := int64(target) - int64(cave+uintptr(position)+4)
		if delta < -0x80000000 || delta > 0x7fffffff {
			return fmt.Errorf("runtime spatial flight relative branch is out of range")
		}
		binary.LittleEndian.PutUint32(code[position:position+4], uint32(int32(delta)))
		position += 4
		return nil
	}
	appendRel32Jump := func(opcodes ...byte) int {
		appendBytes(opcodes...)
		disp := position
		position += 4
		return disp
	}
	patchLocalJump := func(disp, target int) {
		binary.LittleEndian.PutUint32(code[disp:disp+4], uint32(int32(target-(disp+4))))
	}
	writeRIP := func(at int, target uintptr) error {
		delta := int64(target) - int64(cave+uintptr(at)+4)
		if delta < -0x80000000 || delta > 0x7fffffff {
			return fmt.Errorf("runtime spatial flight data reference is out of range")
		}
		binary.LittleEndian.PutUint32(code[at:at+4], uint32(int32(delta)))
		return nil
	}
	writeRIPWithTrailing := func(at int, target uintptr, trailing int) error {
		delta := int64(target) - int64(cave+uintptr(at)+4+uintptr(trailing))
		if delta < -0x80000000 || delta > 0x7fffffff {
			return fmt.Errorf("runtime spatial flight data reference is out of range")
		}
		binary.LittleEndian.PutUint32(code[at:at+4], uint32(int32(delta)))
		return nil
	}

	// Wrap the shared ExFall::floorQuery virtual function. Before takeoff, a
	// complete real floor hit is retained as the contact template. Once virtual
	// ground is active, a native query miss receives that full result (surface,
	// mask, normal and owner metadata included), while only point X/Y/Z and
	// distance are changed for the current target plane. This lets the game's
	// existing contact resolver drive Jump -> Landing -> Wait instead of writing
	// ExFall::grounded or re-entering the action manager from a second hook.
	appendBytes(0x48, 0x83, 0xEC, 0x58)       // sub rsp,58h
	appendBytes(0x48, 0x89, 0x4C, 0x24, 0x28) // mov [rsp+28h],rcx
	appendBytes(0x48, 0x89, 0x54, 0x24, 0x30) // mov [rsp+30h],rdx
	appendBytes(0x4C, 0x89, 0x44, 0x24, 0x38) // mov [rsp+38h],r8
	appendBytes(0x4C, 0x89, 0x4C, 0x24, 0x40) // mov [rsp+40h],r9
	if err := appendRel32(0xE8, cave+runtimeSpatialFlightTrampolineOffset); err != nil {
		return nil, err
	}
	appendBytes(0x4C, 0x8B, 0x5C, 0x24, 0x28)                                     // mov r11,[rsp+28h]
	appendBytes(0x4C, 0x8B, 0x54, 0x24, 0x30)                                     // mov r10,[rsp+30h]
	appendBytes(0x4C, 0x8B, 0x44, 0x24, 0x38)                                     // mov r8,[rsp+38h]
	appendBytes(0x4C, 0x8B, 0x4C, 0x24, 0x40)                                     // mov r9,[rsp+40h]
	appendBytes(0x48, 0x83, 0xC4, 0x58)                                           // add rsp,58h
	appendBytes(0x9C, 0x50, 0x52, 0x41, 0x50, 0x41, 0x51, 0x41, 0x52, 0x41, 0x53) // save flags,rax,rdx,r8-r11
	appendBytes(0x48, 0xFF, 0x05)
	hitCountDisp := position
	position += 4
	appendBytes(0x48, 0x8B, 0x15)
	controllerDisp := position
	position += 4
	appendBytes(0x49, 0x39, 0xD3) // cmp r11,rdx
	controllerMismatchJump := appendRel32Jump(0x0F, 0x85)
	appendBytes(0x8A, 0x44, 0x24, 0x28) // mov al,[rsp+28h] (saved query return)
	appendBytes(0x88, 0x05)
	snapshotReturnDisp := position
	position += 4
	for offset := 0; offset < runtimeSpatialFlightSnapshotSize; offset += 16 {
		appendBytes(0xF3, 0x41, 0x0F, 0x6F, 0x42, byte(offset)) // movdqu xmm0,[r10+offset]
		appendBytes(0xF3, 0x0F, 0x7F, 0x05)
		snapshotDisp := position
		position += 4
		if err := writeRIP(snapshotDisp, cave+runtimeSpatialFlightSnapshotDataOffset+uintptr(offset)); err != nil {
			return nil, err
		}
	}
	appendBytes(0x48, 0xFF, 0x05)
	snapshotSeqDisp := position
	position += 4
	appendBytes(0x80, 0x3D)
	activeDisp := position
	position += 4
	appendBytes(0x00)
	inactiveJump := appendRel32Jump(0x0F, 0x84)
	appendBytes(0x80, 0x3D)
	virtualModeDisp := position
	position += 4
	appendBytes(0x00)
	notVirtualJump := appendRel32Jump(0x0F, 0x84)
	appendBytes(0x80, 0x7C, 0x24, 0x28, 0x00) // cmp saved query return,0
	queryHitJump := appendRel32Jump(0x0F, 0x85)
	appendBytes(0x80, 0x3D)
	templateValidDisp := position
	position += 4
	appendBytes(0x00)
	noTemplateJump := appendRel32Jump(0x0F, 0x84)
	// A virtual plane is a real query result only while it lies inside the
	// native downward floor-query segment. Returning a zero-distance hit while
	// the character is well above the plane makes ExFall believe it never left
	// the ground and strands Jump/AirAttack. Once the jump rises beyond the
	// query segment this path must remain a miss; it becomes a hit again only as
	// the character descends back within landing range.
	appendBytes(0xF3, 0x0F, 0x10, 0x05)
	contactRangeTargetDisp := position
	position += 4
	appendBytes(0x41, 0x0F, 0x2F, 0x40, 0x04) // comiss xmm0,[r8+4] (target > startY)
	contactAboveStartJump := appendRel32Jump(0x0F, 0x87)
	appendBytes(0x41, 0x0F, 0x2F, 0x41, 0x04) // comiss xmm0,[r9+4] (target < endY)
	contactBelowEndJump := appendRel32Jump(0x0F, 0x82)
	for offset := 0; offset < runtimeSpatialFlightSnapshotSize; offset += 16 {
		appendBytes(0xF3, 0x0F, 0x6F, 0x05)
		templateDisp := position
		position += 4
		appendBytes(0xF3, 0x41, 0x0F, 0x7F, 0x42, byte(offset))
		if err := writeRIP(templateDisp, cave+runtimeSpatialFlightContactTemplateOffset+uintptr(offset)); err != nil {
			return nil, err
		}
	}
	appendBytes(0xC6, 0x44, 0x24, 0x28, 0x01) // saved AL=true
	contactOffset := position
	appendBytes(0x41, 0xC6, 0x02, 0x01) // result.valid=true
	appendBytes(0x41, 0x8B, 0x00)       // eax=[queryStart.x]
	appendBytes(0x41, 0x89, 0x42, 0x10) // result.point.x=eax
	appendBytes(0xF3, 0x0F, 0x10, 0x05)
	virtualTargetDisp := position
	position += 4
	appendBytes(0xF3, 0x0F, 0x5C, 0x05)
	contactClearanceDisp := position
	position += 4
	appendBytes(0xF3, 0x41, 0x0F, 0x11, 0x42, 0x14) // movss [r10+14h],xmm0
	appendBytes(0x41, 0x8B, 0x40, 0x08)             // eax=[queryStart.z]
	appendBytes(0x41, 0x89, 0x42, 0x18)             // result.point.z=eax
	appendBytes(0xF3, 0x41, 0x0F, 0x10, 0x40, 0x04) // xmm0=queryStart.y
	appendBytes(0xF3, 0x0F, 0x5C, 0x05)
	contactDistanceTargetDisp := position
	position += 4
	appendBytes(0xF3, 0x41, 0x0F, 0x11, 0x42, 0x30) // result.distance=startY-targetY
	appendBytes(0x48, 0xFF, 0x05)
	acceptCountDisp := position
	position += 4
	anchorOffset := position
	appendBytes(0x80, 0x3D)
	heightAnchorDisp := position
	position += 4
	appendBytes(0x00)
	skipHeightAnchorJump := appendRel32Jump(0x0F, 0x84)
	appendBytes(0x4C, 0x8B, 0x15)
	heightDisp := position
	position += 4
	appendBytes(0x8B, 0x05)
	targetDisp := position
	position += 4
	appendBytes(0x41, 0x89, 0x02) // mov [r10],eax
	appendBytes(0x80, 0x3D)
	clearVelocityDisp := position
	position += 4
	appendBytes(0x00)
	skipVelocityClearJump := appendRel32Jump(0x0F, 0x84)
	appendBytes(0x41, 0xC7, 0x43, 0x24, 0, 0, 0, 0) // virtual ground: mov dword [r11+24h],0
	velocityClearEnd := position
	heightAnchorEnd := position
	anchorReturnJump := appendRel32Jump(0xE9)

	captureOffset := position
	appendBytes(0x80, 0x7C, 0x24, 0x28, 0x00)
	captureMissJump := appendRel32Jump(0x0F, 0x84)
	for offset := 0; offset < runtimeSpatialFlightSnapshotSize; offset += 16 {
		appendBytes(0xF3, 0x41, 0x0F, 0x6F, 0x42, byte(offset))
		appendBytes(0xF3, 0x0F, 0x7F, 0x05)
		templateDisp := position
		position += 4
		if err := writeRIP(templateDisp, cave+runtimeSpatialFlightContactTemplateOffset+uintptr(offset)); err != nil {
			return nil, err
		}
	}
	appendBytes(0xC6, 0x05)
	templateValidWriteDisp := position
	position += 4
	appendBytes(0x01)
	captureReturnJump := appendRel32Jump(0xE9)

	restoreOffset := position
	appendBytes(0x41, 0x5B, 0x41, 0x5A, 0x41, 0x59, 0x41, 0x58, 0x5A, 0x58, 0x9D, 0xC3)
	patchLocalJump(controllerMismatchJump, restoreOffset)
	patchLocalJump(inactiveJump, captureOffset)
	patchLocalJump(notVirtualJump, anchorOffset)
	patchLocalJump(queryHitJump, contactOffset)
	patchLocalJump(noTemplateJump, anchorOffset)
	patchLocalJump(contactAboveStartJump, anchorOffset)
	patchLocalJump(contactBelowEndJump, anchorOffset)
	patchLocalJump(skipHeightAnchorJump, heightAnchorEnd)
	patchLocalJump(skipVelocityClearJump, velocityClearEnd)
	patchLocalJump(anchorReturnJump, restoreOffset)
	patchLocalJump(captureMissJump, restoreOffset)
	patchLocalJump(captureReturnJump, restoreOffset)
	if err := writeRIP(controllerDisp, cave+runtimeSpatialFlightControllerDataOffset); err != nil {
		return nil, err
	}
	if err := writeRIPWithTrailing(activeDisp, cave+runtimeSpatialFlightActiveDataOffset, 1); err != nil {
		return nil, err
	}
	if err := writeRIPWithTrailing(virtualModeDisp, cave+runtimeSpatialFlightVirtualModeDataOffset, 1); err != nil {
		return nil, err
	}
	if err := writeRIPWithTrailing(templateValidDisp, cave+runtimeSpatialFlightContactTemplateValidOffset, 1); err != nil {
		return nil, err
	}
	if err := writeRIPWithTrailing(templateValidWriteDisp, cave+runtimeSpatialFlightContactTemplateValidOffset, 1); err != nil {
		return nil, err
	}
	if err := writeRIP(snapshotReturnDisp, cave+runtimeSpatialFlightSnapshotReturnOffset); err != nil {
		return nil, err
	}
	if err := writeRIP(snapshotSeqDisp, cave+runtimeSpatialFlightSnapshotSeqOffset); err != nil {
		return nil, err
	}
	if err := writeRIP(virtualTargetDisp, cave+runtimeSpatialFlightTargetDataOffset); err != nil {
		return nil, err
	}
	if err := writeRIP(contactRangeTargetDisp, cave+runtimeSpatialFlightTargetDataOffset); err != nil {
		return nil, err
	}
	if err := writeRIP(contactDistanceTargetDisp, cave+runtimeSpatialFlightTargetDataOffset); err != nil {
		return nil, err
	}
	if err := writeRIP(contactClearanceDisp, cave+runtimeSpatialFlightContactClearanceOffset); err != nil {
		return nil, err
	}
	if err := writeRIP(heightDisp, cave+runtimeSpatialFlightHeightDataOffset); err != nil {
		return nil, err
	}
	if err := writeRIP(targetDisp, cave+runtimeSpatialFlightTargetDataOffset); err != nil {
		return nil, err
	}
	if err := writeRIPWithTrailing(clearVelocityDisp, cave+runtimeSpatialFlightClearVelocityDataOffset, 1); err != nil {
		return nil, err
	}
	if err := writeRIPWithTrailing(heightAnchorDisp, cave+runtimeSpatialFlightHeightAnchorDataOffset, 1); err != nil {
		return nil, err
	}
	if err := writeRIP(hitCountDisp, cave+runtimeSpatialFlightHitCountDataOffset); err != nil {
		return nil, err
	}
	if err := writeRIP(acceptCountDisp, cave+runtimeSpatialFlightAcceptCountDataOffset); err != nil {
		return nil, err
	}
	if position > runtimeSpatialFlightMarkerOffset {
		return nil, fmt.Errorf("runtime spatial flight cave code overlaps its data section")
	}
	copy(code[runtimeSpatialFlightTrampolineOffset:], runtimeSpatialFlightHookOriginal)
	trampolineJump := runtimeSpatialFlightTrampolineOffset + len(runtimeSpatialFlightHookOriginal)
	code[trampolineJump] = 0xE9
	delta := int64(originalFunction+runtimeSpatialFlightHookSize) - int64(cave+uintptr(trampolineJump)+5)
	if delta < -0x80000000 || delta > 0x7fffffff {
		return nil, fmt.Errorf("runtime spatial flight trampoline branch is out of range")
	}
	binary.LittleEndian.PutUint32(code[trampolineJump+1:trampolineJump+5], uint32(int32(delta)))
	copy(code[runtimeSpatialFlightMarkerOffset:], runtimeSpatialFlightMarker[:])
	clear(code[runtimeSpatialFlightSnapshotDataOffset : runtimeSpatialFlightSnapshotDataOffset+runtimeSpatialFlightSnapshotSize])
	clear(code[runtimeSpatialFlightContactTemplateOffset : runtimeSpatialFlightContactTemplateOffset+runtimeSpatialFlightSnapshotSize])
	binary.LittleEndian.PutUint64(code[runtimeSpatialFlightControllerDataOffset:], uint64(controller))
	binary.LittleEndian.PutUint64(code[runtimeSpatialFlightHeightDataOffset:], uint64(heightAddr))
	binary.LittleEndian.PutUint32(code[runtimeSpatialFlightTargetDataOffset:], math.Float32bits(targetHeight))
	binary.LittleEndian.PutUint64(code[runtimeSpatialFlightHitCountDataOffset:], 0)
	binary.LittleEndian.PutUint64(code[runtimeSpatialFlightAcceptCountDataOffset:], 0)
	binary.LittleEndian.PutUint64(code[runtimeSpatialFlightSnapshotSeqOffset:], 0)
	binary.LittleEndian.PutUint32(code[runtimeSpatialFlightContactClearanceOffset:], math.Float32bits(runtimeSpatialFlightContactClearance))
	code[runtimeSpatialFlightActiveDataOffset] = 0
	code[runtimeSpatialFlightVirtualModeDataOffset] = 0
	code[runtimeSpatialFlightSnapshotReturnOffset] = 0
	code[runtimeSpatialFlightContactTemplateValidOffset] = 0
	code[runtimeSpatialFlightHeightAnchorDataOffset] = 0
	code[runtimeSpatialFlightClearVelocityDataOffset] = 0
	if active {
		code[runtimeSpatialFlightActiveDataOffset] = 1
	}
	if contactReplay {
		code[runtimeSpatialFlightVirtualModeDataOffset] = 1
	}
	if clearVelocity {
		code[runtimeSpatialFlightClearVelocityDataOffset] = 1
	}
	if heightAnchor {
		code[runtimeSpatialFlightHeightAnchorDataOffset] = 1
	}
	return code, nil
}

func readRuntimeSpatialFloorQuerySnapshot(memory runtimePatchPartyMemory, cave uintptr) (runtimeSpatialFloorQuerySnapshot, error) {
	if memory == nil || cave == 0 {
		return runtimeSpatialFloorQuerySnapshot{}, fmt.Errorf("invalid runtime spatial floor-query snapshot parameters")
	}
	raw := make([]byte, runtimeSpatialFlightSnapshotSize)
	if err := memory.ReadAt(cave+runtimeSpatialFlightSnapshotDataOffset, raw); err != nil {
		return runtimeSpatialFloorQuerySnapshot{}, fmt.Errorf("read floor-query snapshot data: %w", err)
	}
	meta := make([]byte, 16)
	if err := memory.ReadAt(cave+runtimeSpatialFlightSnapshotReturnOffset, meta[:1]); err != nil {
		return runtimeSpatialFloorQuerySnapshot{}, fmt.Errorf("read floor-query snapshot return: %w", err)
	}
	if err := memory.ReadAt(cave+runtimeSpatialFlightSnapshotSeqOffset, meta[8:]); err != nil {
		return runtimeSpatialFloorQuerySnapshot{}, fmt.Errorf("read floor-query snapshot sequence: %w", err)
	}
	var snapshot runtimeSpatialFloorQuerySnapshot
	snapshot.Returned = meta[0] != 0
	snapshot.Sequence = binary.LittleEndian.Uint64(meta[8:])
	copy(snapshot.Data[:], raw)
	return snapshot, nil
}

func readRuntimeSpatialFlightHookDiagnostics(memory runtimePatchPartyMemory, cave uintptr) (runtimeSpatialFlightHookDiagnostics, error) {
	if memory == nil || cave == 0 {
		return runtimeSpatialFlightHookDiagnostics{}, fmt.Errorf("invalid runtime spatial flight diagnostics parameters")
	}
	const diagnosticSize = runtimeSpatialFlightContactClearanceOffset - runtimeSpatialFlightSnapshotReturnOffset
	raw := make([]byte, diagnosticSize)
	if err := memory.ReadAt(cave+runtimeSpatialFlightSnapshotReturnOffset, raw); err != nil {
		return runtimeSpatialFlightHookDiagnostics{}, fmt.Errorf("read runtime spatial flight diagnostics: %w", err)
	}
	return runtimeSpatialFlightHookDiagnostics{
		LastQueryHit:         raw[0] != 0,
		ContactTemplateReady: raw[1] != 0,
		FloorQueries:         binary.LittleEndian.Uint64(raw[2:10]),
		AcceptedContacts:     binary.LittleEndian.Uint64(raw[10:18]),
		SnapshotSequence:     binary.LittleEndian.Uint64(raw[18:26]),
	}, nil
}

func (a *App) syncRuntimeSpatialFlightHookLocked(owner string, binding runtimeSpatialFlightBinding, state runtimeSpatialFlightAnchorState, mode string) error {
	flightMode, err := normalizeRuntimeSpatialFlightMode(mode)
	if err != nil {
		return err
	}
	wantContactReplay := flightMode == runtimeSpatialFlightModeVirtualGround || state.AerialRecovery
	// Preserve native air motion during normal aerial play. Once a verified
	// action enters the landing handshake, match virtual-ground semantics long
	// enough for ExFall to accept the replayed contact and emit grounded 0->1.
	wantClearVelocity := flightMode == runtimeSpatialFlightModeVirtualGround || state.AerialRecovery
	lease := a.runtimeSpatialFlightHookLease
	heightAddr := binding.TransformNode + runtimePatchPartyPositionYOffset
	if lease != nil && (lease.OwnerToken != owner || !sameProcessInstance(lease.Process, a.currentProcessInstance()) ||
		lease.Controller != binding.Controller || lease.HeightAddr != heightAddr) {
		if err := a.restoreRuntimeSpatialFlightHookOwnedLocked(owner, false); err != nil {
			return err
		}
		lease = nil
	}
	if lease != nil {
		entry := make([]byte, len(lease.Installed))
		if err := readProcessMemory(a.hProcess, lease.EntryAddr, unsafe.Pointer(&entry[0]), uintptr(len(entry))); err != nil {
			return err
		}
		if !bytes.Equal(entry, lease.Installed) || relJumpTarget(lease.EntryAddr, entry) != lease.CaveAddr {
			return fmt.Errorf("悬空飞行同帧 Hook 入口已被外部修改: %s", bytesToHex(entry))
		}
		activeByte := byte(0)
		if state.Active {
			activeByte = 1
		}
		modeByte := byte(0)
		if wantContactReplay {
			modeByte = 1
		}
		control := []byte{activeByte, modeByte}
		controlAddress := lease.CaveAddr + runtimeSpatialFlightActiveDataOffset
		actualControl := make([]byte, len(control))
		if err := readProcessMemory(a.hProcess, controlAddress, unsafe.Pointer(&actualControl[0]), uintptr(len(actualControl))); err != nil {
			return err
		}
		if !bytes.Equal(actualControl, control) {
			if err := writeProcessMemory(a.hProcess, controlAddress, unsafe.Pointer(&control[0]), uintptr(len(control))); err != nil {
				return err
			}
			if err := readProcessMemory(a.hProcess, controlAddress, unsafe.Pointer(&actualControl[0]), uintptr(len(actualControl))); err != nil || !bytes.Equal(actualControl, control) {
				return fmt.Errorf("悬空飞行模式写后回读失败: %v", err)
			}
		}
		anchorControl := []byte{0, 0}
		if state.HeightAnchor {
			anchorControl[0] = 1
		}
		if wantClearVelocity {
			anchorControl[1] = 1
		}
		anchorAddress := lease.CaveAddr + runtimeSpatialFlightHeightAnchorDataOffset
		actualAnchor := []byte{0, 0}
		if err := readProcessMemory(a.hProcess, anchorAddress, unsafe.Pointer(&actualAnchor[0]), uintptr(len(actualAnchor))); err != nil {
			return err
		}
		if !bytes.Equal(actualAnchor, anchorControl) {
			if err := writeProcessMemory(a.hProcess, anchorAddress, unsafe.Pointer(&anchorControl[0]), uintptr(len(anchorControl))); err != nil {
				return err
			}
			if err := readProcessMemory(a.hProcess, anchorAddress, unsafe.Pointer(&actualAnchor[0]), uintptr(len(actualAnchor))); err != nil || !bytes.Equal(actualAnchor, anchorControl) {
				return fmt.Errorf("悬空飞行高度锚定模式写后回读失败: %v", err)
			}
		}
		lease.ContactReplay = wantContactReplay
		if lease.TargetHeight == state.TargetY {
			return nil
		}
		encoded := make([]byte, 4)
		binary.LittleEndian.PutUint32(encoded, math.Float32bits(state.TargetY))
		address := lease.CaveAddr + runtimeSpatialFlightTargetDataOffset
		if err := writeProcessMemory(a.hProcess, address, unsafe.Pointer(&encoded[0]), uintptr(len(encoded))); err != nil {
			return err
		}
		actual := make([]byte, len(encoded))
		if err := readProcessMemory(a.hProcess, address, unsafe.Pointer(&actual[0]), uintptr(len(actual))); err != nil || !bytes.Equal(actual, encoded) {
			return fmt.Errorf("悬空飞行目标高度写后回读失败: %v", err)
		}
		lease.TargetHeight = state.TargetY
		return nil
	}
	if err := a.verifyRuntimePatchExecutableLocked(a.currentProcessInstance(), runtimePatchMonitorText("角色悬空飞行同帧 Hook", "Character flight same-frame hook")); err != nil {
		return err
	}
	entry := a.moduleBase + runtimeSpatialFlightHookRVA
	original := make([]byte, runtimeSpatialFlightHookSize)
	if err := readProcessMemory(a.hProcess, entry, unsafe.Pointer(&original[0]), uintptr(len(original))); err != nil {
		return err
	}
	if !bytes.Equal(original, runtimeSpatialFlightHookOriginal) {
		return fmt.Errorf("悬空飞行移动提交入口原字节不匹配: %s", bytesToHex(original))
	}
	cave, err := virtualAllocRemoteNear(a.hProcess, entry, runtimeSpatialFlightCaveSize)
	if err != nil {
		return err
	}
	code, err := buildRuntimeSpatialFlightCave(cave, entry, binding.Controller, heightAddr, state.TargetY, state.Active, wantContactReplay, wantClearVelocity, state.HeightAnchor)
	if err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return err
	}
	if err := writeProcessMemory(a.hProcess, cave, unsafe.Pointer(&code[0]), uintptr(len(code))); err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return err
	}
	confirmed := make([]byte, len(code))
	if err := readProcessMemory(a.hProcess, cave, unsafe.Pointer(&confirmed[0]), uintptr(len(confirmed))); err != nil || !bytes.Equal(confirmed, code) {
		_ = virtualFreeRemote(a.hProcess, cave)
		return fmt.Errorf("悬空飞行同帧代码洞写后回读失败: %v", err)
	}
	installed, err := makeRelJump(entry, cave, runtimeSpatialFlightHookSize)
	if err != nil {
		_ = virtualFreeRemote(a.hProcess, cave)
		return err
	}
	newLease := &runtimeSpatialFlightHookLease{
		OwnerToken: owner, Process: a.currentProcessInstance(), EntryAddr: entry,
		Original: original, Installed: installed, CaveAddr: cave, Controller: binding.Controller,
		HeightAddr: heightAddr, TargetHeight: state.TargetY, ContactReplay: wantContactReplay,
	}
	result, installErr := installRemoteCodeHook(a.hProcess, entry, original, installed)
	if installErr != nil {
		return runtimeHookInstallFailure(
			"悬空飞行同帧 Hook", result, installErr,
			func() { _ = virtualFreeRemote(a.hProcess, cave) },
			func() { a.retireRuntimeCaveLocked(cave, "spatial flight install rollback") },
			func() { a.runtimeSpatialFlightHookLease = newLease },
			func() { a.poisonCurrentLiveMemoryWrites() },
		)
	}
	a.runtimeSpatialFlightHookLease = newLease
	return nil
}

func (a *App) restoreRuntimeSpatialFlightHookOwnedLocked(owner string, force bool) error {
	lease := a.runtimeSpatialFlightHookLease
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
			return fmt.Errorf("悬空飞行恢复前入口字节未知: %s", bytesToHex(entry))
		}
		if err := writeAndVerifyRuntimeHookEntry(a.hProcess, lease.EntryAddr, lease.Original); err != nil {
			return err
		}
	}
	a.retireRuntimeCaveLocked(lease.CaveAddr, "spatial flight release")
	a.runtimeSpatialFlightHookLease = nil
	return nil
}

func (a *App) restoreRuntimeSpatialFlightHookOwned(owner string, force bool) error {
	liveMemoryWriteMu.Lock()
	defer liveMemoryWriteMu.Unlock()
	a.procMu.Lock()
	defer a.procMu.Unlock()
	if a.hProcess == 0 || !processHandleAlive(a.hProcess) {
		a.runtimeSpatialFlightHookLease = nil
		return nil
	}
	a.runtimePatchMu.Lock()
	defer a.runtimePatchMu.Unlock()
	return a.restoreRuntimeSpatialFlightHookOwnedLocked(owner, force)
}

func (a *App) dropRuntimeSpatialFlightHookOwnerLocked(owner string) {
	if a.runtimeSpatialFlightHookLease != nil && a.runtimeSpatialFlightHookLease.OwnerToken == owner {
		a.runtimeSpatialFlightHookLease = nil
	}
}
