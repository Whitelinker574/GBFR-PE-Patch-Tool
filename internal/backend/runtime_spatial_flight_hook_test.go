package backend

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

type localFlightHarnessMemory struct{ handle windows.Handle }

func (memory localFlightHarnessMemory) ReadAt(address uintptr, destination []byte) error {
	if len(destination) == 0 {
		return nil
	}
	return readProcessMemory(memory.handle, address, unsafe.Pointer(&destination[0]), uintptr(len(destination)))
}

func TestRuntimeSpatialFlightHookWrapsTheSharedExFallFloorQuery(t *testing.T) {
	if runtimeSpatialFlightHookRVA != 0xA63230 {
		t.Fatalf("flight hook RVA=0x%X; character-specific 0x9DFxxx paths are not shared by every ExFall owner", runtimeSpatialFlightHookRVA)
	}
	want := []byte{0x48, 0x83, 0xEC, 0x38, 0x4C, 0x89, 0xC8, 0xC7, 0x44, 0x24, 0x28, 0x00, 0x00, 0x00, 0x00}
	if !bytes.Equal(runtimeSpatialFlightHookOriginal, want) {
		t.Fatalf("flight hook original=% X want shared floor-query prologue=% X", runtimeSpatialFlightHookOriginal, want)
	}
}

func TestRuntimeSpatialFlightHooksMatchLocalGame203(t *testing.T) {
	path := os.Getenv("GBFR_GAME_EXE_203_TEST")
	if path == "" {
		t.Skip("set GBFR_GAME_EXE_203_TEST to verify the local 2.0.3 executable")
	}
	checks := []struct {
		name string
		rva  uintptr
		want []byte
	}{
		{name: "height anchor", rva: runtimeSpatialFlightHookRVA, want: runtimeSpatialFlightHookOriginal},
		{name: "legacy action-hook recovery identity", rva: runtimeSpatialVirtualGroundHookRVA, want: runtimeSpatialVirtualGroundHookOriginal},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			got, err := readPEImageRVA(path, check.rva, len(check.want))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, check.want) {
				t.Fatalf("RVA 0x%X=% X want=% X", check.rva, got, check.want)
			}
		})
	}
}

func TestRuntimeSpatialFlightCaveWrapsSharedQueryAndOwnsExactTarget(t *testing.T) {
	cave := uintptr(0x20000000)
	originalFunction := uintptr(0x20002000)
	controller := uintptr(0x31000000)
	height := uintptr(0x320000D4)
	code, err := buildRuntimeSpatialFlightCave(cave, originalFunction, controller, height, 42.5, true, true, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != runtimeSpatialFlightCaveSize || !bytes.Equal(code[:4], []byte{0x48, 0x83, 0xEC, 0x58}) {
		t.Fatalf("cave entry=% X len=%d", code[:8], len(code))
	}
	callOffset := bytes.IndexByte(code[:runtimeSpatialFlightMarkerOffset], 0xE8)
	if callOffset < 0 {
		t.Fatal("shared-query trampoline call missing")
	}
	callTarget := uintptr(int64(cave+uintptr(callOffset)+5) + int64(int32(binary.LittleEndian.Uint32(code[callOffset+1:callOffset+5]))))
	if callTarget != cave+runtimeSpatialFlightTrampolineOffset {
		t.Fatalf("trampoline call target=0x%X", callTarget)
	}
	if !bytes.Equal(code[runtimeSpatialFlightTrampolineOffset:runtimeSpatialFlightTrampolineOffset+len(runtimeSpatialFlightHookOriginal)], runtimeSpatialFlightHookOriginal) {
		t.Fatal("trampoline does not replay the displaced shared-query prologue")
	}
	jumpOffset := runtimeSpatialFlightTrampolineOffset + len(runtimeSpatialFlightHookOriginal)
	if code[jumpOffset] != 0xE9 {
		t.Fatal("trampoline return jump missing")
	}
	jumpTarget := uintptr(int64(cave+uintptr(jumpOffset)+5) + int64(int32(binary.LittleEndian.Uint32(code[jumpOffset+1:jumpOffset+5]))))
	if jumpTarget != originalFunction+runtimeSpatialFlightHookSize {
		t.Fatalf("trampoline return target=0x%X want=0x%X", jumpTarget, originalFunction+runtimeSpatialFlightHookSize)
	}
	if !bytes.Equal(code[runtimeSpatialFlightMarkerOffset:runtimeSpatialFlightMarkerOffset+len(runtimeSpatialFlightMarker)], runtimeSpatialFlightMarker[:]) {
		t.Fatal("flight cave marker missing")
	}
	if got := uintptr(binary.LittleEndian.Uint64(code[runtimeSpatialFlightControllerDataOffset:])); got != controller {
		t.Fatalf("controller=0x%X", got)
	}
	if got := uintptr(binary.LittleEndian.Uint64(code[runtimeSpatialFlightHeightDataOffset:])); got != height {
		t.Fatalf("height address=0x%X", got)
	}
	if got := math.Float32frombits(binary.LittleEndian.Uint32(code[runtimeSpatialFlightTargetDataOffset:])); got != 42.5 {
		t.Fatalf("target=%v", got)
	}
	if code[runtimeSpatialFlightActiveDataOffset] != 1 {
		t.Fatal("flight cave did not start active")
	}
	if code[runtimeSpatialFlightVirtualModeDataOffset] != 1 {
		t.Fatal("flight cave did not start in virtual-contact mode")
	}
	if code[runtimeSpatialFlightHeightAnchorDataOffset] != 1 {
		t.Fatal("flight cave did not start with the requested height anchor")
	}
	if code[runtimeSpatialFlightContactTemplateValidOffset] != 0 {
		t.Fatal("flight cave must not claim a real contact template before observing one")
	}
	if !bytes.Equal(code[runtimeSpatialFlightContactTemplateOffset:runtimeSpatialFlightContactTemplateOffset+runtimeSpatialFlightSnapshotSize], make([]byte, runtimeSpatialFlightSnapshotSize)) {
		t.Fatal("flight cave contact template must start empty")
	}
	if !bytes.Contains(code[:runtimeSpatialFlightMarkerOffset], []byte{0x41, 0xC7, 0x43, 0x24, 0x00, 0x00, 0x00, 0x00}) {
		t.Fatal("flight cave does not clear ExFall+0x24 vertical velocity")
	}
	if binary.LittleEndian.Uint64(code[runtimeSpatialFlightHitCountDataOffset:]) != 0 || binary.LittleEndian.Uint64(code[runtimeSpatialFlightAcceptCountDataOffset:]) != 0 {
		t.Fatal("flight cave counters must start at zero")
	}
	if binary.LittleEndian.Uint64(code[runtimeSpatialFlightSnapshotSeqOffset:]) != 0 {
		t.Fatal("flight cave snapshot sequence must start at zero")
	}
	if !bytes.Contains(code[:runtimeSpatialFlightMarkerOffset], []byte{0x48, 0x89, 0x54, 0x24, 0x30}) {
		t.Fatal("flight cave does not preserve the floor-query result pointer")
	}
	if bytes.Count(code[:runtimeSpatialFlightMarkerOffset], []byte{0xF3, 0x41, 0x0F, 0x6F, 0x42}) != 2*runtimeSpatialFlightSnapshotSize/16 {
		t.Fatal("flight cave does not snapshot and retain the complete bounded hit-result")
	}
	if !bytes.Contains(code[:runtimeSpatialFlightMarkerOffset], []byte{0xF3, 0x41, 0x0F, 0x11, 0x42, 0x14}) {
		t.Fatal("flight cave does not move the owned floor hit to the virtual contact plane")
	}
	if got := math.Float32frombits(binary.LittleEndian.Uint32(code[runtimeSpatialFlightContactClearanceOffset:])); got != runtimeSpatialFlightContactClearance {
		t.Fatalf("virtual floor contact clearance=%v", got)
	}
}

func TestRuntimeSpatialFlightCaveRejectsInvalidOwnershipData(t *testing.T) {
	if _, err := buildRuntimeSpatialFlightCave(0x20000000, 0x20002000, 0, 0x320000D4, 1, true, true, true, true); err == nil {
		t.Fatal("zero controller unexpectedly accepted")
	}
	if _, err := buildRuntimeSpatialFlightCave(0x20000000, 0x20002000, 0x31000000, 0x320000D4, float32(math.NaN()), true, true, true, true); err == nil {
		t.Fatal("NaN target unexpectedly accepted")
	}
}

// This native harness is the regression seam for the reported virtual-ground
// failure. The original floor query returns a miss while the cave owns
// a previously captured, valid ground-contact template. A correct virtual
// floor must turn that miss into a complete contact at targetY; merely pinning
// the transform/vertical speed leaves the character in Jump/Fall.
func TestRuntimeSpatialFlightCaveSynthesizesOwnedVirtualContactOnQueryMiss(t *testing.T) {
	const (
		contactTemplateOffset = uintptr(runtimeSpatialFlightContactTemplateOffset)
		contactTemplateValid  = uintptr(runtimeSpatialFlightContactTemplateValidOffset)
		virtualModeOffset     = uintptr(runtimeSpatialFlightVirtualModeDataOffset)
	)
	handle := windows.CurrentProcess()
	page, err := virtualAllocRemote(handle, 0x4000, windows.PAGE_EXECUTE_READWRITE)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = virtualFreeRemote(handle, page) })

	entry := page + 0x100
	cave := page + 0x800
	controller := page + 0x1800
	height := page + 0x1900
	targetY := float32(12.5)
	code, err := buildRuntimeSpatialFlightCave(cave, entry, controller, height, targetY, true, true, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeCodeMemory(handle, cave, code); err != nil {
		t.Fatal(err)
	}
	// The trampoline replays the displaced prologue, then lands here. This
	// bounded stub represents the game's query returning AL=0 (no floor).
	missStub := append(bytes.Repeat([]byte{0x90}, runtimeSpatialFlightHookSize),
		0x31, 0xC0, // xor eax,eax
		0x48, 0x83, 0xC4, 0x38, // add rsp,38h
		0xC3, // ret
	)
	if err := writeCodeMemory(handle, entry, missStub); err != nil {
		t.Fatal(err)
	}

	template := make([]byte, runtimeSpatialFlightSnapshotSize)
	template[0] = 1
	binary.LittleEndian.PutUint32(template[0x10:], math.Float32bits(1.25))
	binary.LittleEndian.PutUint32(template[0x14:], math.Float32bits(2.5))
	binary.LittleEndian.PutUint32(template[0x18:], math.Float32bits(3.75))
	binary.LittleEndian.PutUint32(template[0x20:], math.Float32bits(0))
	binary.LittleEndian.PutUint32(template[0x24:], math.Float32bits(1))
	binary.LittleEndian.PutUint32(template[0x28:], math.Float32bits(0))
	binary.LittleEndian.PutUint32(template[0x30:], math.Float32bits(0.25))
	binary.LittleEndian.PutUint32(template[0x34:], 0x40)
	if err := writeProcessMemory(handle, cave+contactTemplateOffset, unsafe.Pointer(&template[0]), uintptr(len(template))); err != nil {
		t.Fatal(err)
	}
	if err := writeProcessMemory(handle, cave+contactTemplateValid, unsafe.Pointer(&[]byte{1}[0]), 1); err != nil {
		t.Fatal(err)
	}
	if err := writeProcessMemory(handle, cave+virtualModeOffset, unsafe.Pointer(&[]byte{1}[0]), 1); err != nil {
		t.Fatal(err)
	}
	velocity := make([]byte, 4)
	binary.LittleEndian.PutUint32(velocity, math.Float32bits(2.75))
	if err := writeProcessMemory(handle, controller+runtimeSpatialControllerVelocityOffset, unsafe.Pointer(&velocity[0]), 4); err != nil {
		t.Fatal(err)
	}

	start := [4]float32{10, 20, 30, 1}
	end := [4]float32{10, -30, 30, 1}
	output := make([]byte, runtimeSpatialFlightSnapshotSize)
	returned, _, callErr := syscall.SyscallN(
		cave,
		controller,
		uintptr(unsafe.Pointer(&output[0])),
		uintptr(unsafe.Pointer(&start[0])),
		uintptr(unsafe.Pointer(&end[0])),
	)
	if callErr != 0 {
		t.Fatalf("execute virtual-contact cave: %v", callErr)
	}
	if returned&0xFF == 0 {
		meta := make([]byte, 0x20)
		if err := readProcessMemory(handle, cave+runtimeSpatialFlightActiveDataOffset, unsafe.Pointer(&meta[0]), uintptr(len(meta))); err != nil {
			t.Fatal(err)
		}
		heightRaw := make([]byte, 4)
		if err := readProcessMemory(handle, height, unsafe.Pointer(&heightRaw[0]), uintptr(len(heightRaw))); err != nil {
			t.Fatal(err)
		}
		t.Fatalf("query miss remained a miss; virtual contact did not reach native Landing/Wait: active=%d virtual=%d snapshotReturn=%d templateValid=%d hits=%d accepted=%d height=%v output=% X",
			meta[0], meta[1], meta[2], meta[3], binary.LittleEndian.Uint64(meta[4:12]), binary.LittleEndian.Uint64(meta[12:20]), math.Float32frombits(binary.LittleEndian.Uint32(heightRaw)), output)
	}
	if output[0] == 0 {
		t.Fatal("virtual contact result flag was not copied from the grounded template")
	}
	if got := math.Float32frombits(binary.LittleEndian.Uint32(output[0x10:])); got != start[0] {
		t.Fatalf("virtual contact X=%v want current query X=%v", got, start[0])
	}
	if got := math.Float32frombits(binary.LittleEndian.Uint32(output[0x14:])); math.Abs(float64(got-(targetY-runtimeSpatialFlightContactClearance))) > 0.0001 {
		t.Fatalf("virtual contact Y=%v want=%v", got, targetY-runtimeSpatialFlightContactClearance)
	}
	if got := math.Float32frombits(binary.LittleEndian.Uint32(output[0x18:])); got != start[2] {
		t.Fatalf("virtual contact Z=%v want current query Z=%v", got, start[2])
	}
	if got := math.Float32frombits(binary.LittleEndian.Uint32(output[0x24:])); got < 0.7 {
		t.Fatalf("virtual contact upward normal=%v", got)
	}
	if got := binary.LittleEndian.Uint32(output[0x34:]); got == 0 {
		t.Fatal("virtual contact lost the real surface/mask metadata")
	}
	heightRaw := make([]byte, 4)
	if err := readProcessMemory(handle, height, unsafe.Pointer(&heightRaw[0]), 4); err != nil {
		t.Fatal(err)
	}
	if got := math.Float32frombits(binary.LittleEndian.Uint32(heightRaw)); got != 0 {
		t.Fatalf("virtual floor without PageUp/PageDown pinned the jumping character to %v", got)
	}
	if err := readProcessMemory(handle, controller+runtimeSpatialControllerVelocityOffset, unsafe.Pointer(&velocity[0]), 4); err != nil {
		t.Fatal(err)
	}
	if got := math.Float32frombits(binary.LittleEndian.Uint32(velocity)); got != 2.75 {
		t.Fatalf("virtual floor without height adjustment erased jump velocity: %v", got)
	}

	// Once the character has jumped above the native floor-query segment, the
	// same virtual plane must stop reporting contact. Otherwise every frame is
	// seen as grounded and Jump/AirAttack can never progress naturally.
	aboveStart := [4]float32{10, 30, 30, 1}
	aboveEnd := [4]float32{10, 20, 30, 1}
	clear(output)
	returned, _, callErr = syscall.SyscallN(
		cave,
		controller,
		uintptr(unsafe.Pointer(&output[0])),
		uintptr(unsafe.Pointer(&aboveStart[0])),
		uintptr(unsafe.Pointer(&aboveEnd[0])),
	)
	if callErr != 0 {
		t.Fatalf("execute out-of-range virtual-contact cave: %v", callErr)
	}
	if returned&0xFF != 0 || !bytes.Equal(output, make([]byte, len(output))) {
		t.Fatalf("virtual plane outside the native query segment still reported contact: returned=%d output=% X", returned&0xFF, output)
	}
	acceptedRaw := make([]byte, 8)
	if err := readProcessMemory(handle, cave+runtimeSpatialFlightAcceptCountDataOffset, unsafe.Pointer(&acceptedRaw[0]), uintptr(len(acceptedRaw))); err != nil {
		t.Fatal(err)
	}
	if accepted := binary.LittleEndian.Uint64(acceptedRaw); accepted != 1 {
		t.Fatalf("out-of-range query was counted as a virtual-contact write: accepted=%d", accepted)
	}
}

func TestRuntimeSpatialFlightCaveCapturesCompleteRealContactBeforeTakeoff(t *testing.T) {
	handle := windows.CurrentProcess()
	page, err := virtualAllocRemote(handle, 0x4000, windows.PAGE_EXECUTE_READWRITE)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = virtualFreeRemote(handle, page) })

	entry := page + 0x100
	cave := page + 0x800
	controller := page + 0x1800
	height := page + 0x1900
	code, err := buildRuntimeSpatialFlightCave(cave, entry, controller, height, 7.5, false, true, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeCodeMemory(handle, cave, code); err != nil {
		t.Fatal(err)
	}
	hitStub := append(bytes.Repeat([]byte{0x90}, runtimeSpatialFlightHookSize),
		0xB0, 0x01, // mov al,1
		0x48, 0x83, 0xC4, 0x38, // add rsp,38h
		0xC3,
	)
	if err := writeCodeMemory(handle, entry, hitStub); err != nil {
		t.Fatal(err)
	}

	start := [4]float32{3, 4, 5, 1}
	end := [4]float32{3, -20, 5, 1}
	output := make([]byte, runtimeSpatialFlightSnapshotSize)
	for index := range output {
		output[index] = byte(index + 1)
	}
	returned, _, callErr := syscall.SyscallN(cave, controller, uintptr(unsafe.Pointer(&output[0])), uintptr(unsafe.Pointer(&start[0])), uintptr(unsafe.Pointer(&end[0])))
	if callErr != 0 || returned&0xFF == 0 {
		t.Fatalf("execute real-contact capture cave: returned=%d error=%v", returned&0xFF, callErr)
	}
	diagnostics, err := readRuntimeSpatialFlightHookDiagnostics(localFlightHarnessMemory{handle: handle}, cave)
	if err != nil {
		t.Fatal(err)
	}
	if !diagnostics.ContactTemplateReady || !diagnostics.LastQueryHit || diagnostics.FloorQueries != 1 || diagnostics.SnapshotSequence != 1 {
		t.Fatalf("real contact was not captured before takeoff: %+v", diagnostics)
	}
	template := make([]byte, runtimeSpatialFlightSnapshotSize)
	if err := readProcessMemory(handle, cave+runtimeSpatialFlightContactTemplateOffset, unsafe.Pointer(&template[0]), uintptr(len(template))); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(template, output) {
		t.Fatalf("captured contact differs from the complete native result: got=% X want=% X", template, output)
	}
}

func TestRuntimeSpatialFlightAerialModePreservesNativeVerticalState(t *testing.T) {
	handle := windows.CurrentProcess()
	page, err := virtualAllocRemote(handle, 0x4000, windows.PAGE_EXECUTE_READWRITE)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = virtualFreeRemote(handle, page) })

	entry := page + 0x100
	cave := page + 0x800
	controller := page + 0x1800
	height := page + 0x1900
	code, err := buildRuntimeSpatialFlightCave(cave, entry, controller, height, 12.5, true, false, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeCodeMemory(handle, cave, code); err != nil {
		t.Fatal(err)
	}
	missStub := append(bytes.Repeat([]byte{0x90}, runtimeSpatialFlightHookSize),
		0x31, 0xC0,
		0x48, 0x83, 0xC4, 0x38,
		0xC3,
	)
	if err := writeCodeMemory(handle, entry, missStub); err != nil {
		t.Fatal(err)
	}
	velocity := make([]byte, 4)
	binary.LittleEndian.PutUint32(velocity, math.Float32bits(-3.25))
	if err := writeProcessMemory(handle, controller+runtimeSpatialControllerVelocityOffset, unsafe.Pointer(&velocity[0]), 4); err != nil {
		t.Fatal(err)
	}
	start := [4]float32{10, 20, 30, 1}
	end := [4]float32{10, -30, 30, 1}
	output := make([]byte, runtimeSpatialFlightSnapshotSize)
	_, _, callErr := syscall.SyscallN(cave, controller, uintptr(unsafe.Pointer(&output[0])), uintptr(unsafe.Pointer(&start[0])), uintptr(unsafe.Pointer(&end[0])))
	if callErr != 0 {
		t.Fatalf("execute aerial cave: %v", callErr)
	}
	actual := make([]byte, 4)
	if err := readProcessMemory(handle, controller+runtimeSpatialControllerVelocityOffset, unsafe.Pointer(&actual[0]), 4); err != nil {
		t.Fatal(err)
	}
	if got := math.Float32frombits(binary.LittleEndian.Uint32(actual)); got != -3.25 {
		t.Fatalf("aerial hover erased the native vertical state needed to leave a consumed air action: got=%v want=-3.25", got)
	}
}
