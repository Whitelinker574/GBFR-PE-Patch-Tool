package main

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestOverLimitOwnedCaveValidationRejectsEveryOwnershipBoundary(t *testing.T) {
	cave := uintptr(0x20000000)
	hook := uintptr(0x20001000)
	code, err := buildOverLimitSelectedCave(cave, hook+uintptr(len(overLimitSelectedOrig)))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateOverLimitSelectedCave(cave, hook, code[:overLimitCaveDataOffset]); err != nil {
		t.Fatalf("fresh owned cave rejected: %v", err)
	}

	mutations := map[string]func([]byte){
		"instruction":  func(v []byte) { v[15] ^= 0x01 },
		"data address": func(v []byte) { v[7] ^= 0x01 },
		"return jump":  func(v []byte) { v[40] ^= 0x01 },
		"padding":      func(v []byte) { v[45] = 0xCC },
		"owner marker": func(v []byte) { v[overLimitCaveMarkerOffset] ^= 0x01 },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			changed := append([]byte(nil), code[:overLimitCaveDataOffset]...)
			mutate(changed)
			if err := validateOverLimitSelectedCave(cave, hook, changed); err == nil {
				t.Fatalf("mutated %s cave was accepted", name)
			}
		})
	}
}

func TestBuildOverLimitSelectedCavePreservesScratchState(t *testing.T) {
	cave := uintptr(0x20000000)
	returnAddr := uintptr(0x20001007)
	code, err := buildOverLimitSelectedCave(cave, returnAddr)
	if err != nil {
		t.Fatal(err)
	}

	// The hook runs in the middle of the game's function, not at an ABI call
	// boundary. Preserve flags plus both scratch registers, and restore the
	// stack exactly before replaying any displaced instruction.
	wantPrefix := []byte{
		0x9C,       // pushfq
		0x41, 0x52, // push r10
		0x41, 0x53, // push r11
		0x49, 0xBA, // mov r10, imm64
	}
	if !bytes.Equal(code[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("OverLimit cave does not preserve flags/R10/R11 before capture: % X", code[:len(wantPrefix)])
	}
	if got := uintptr(binary.LittleEndian.Uint64(code[7:15])); got != cave+overLimitCaveDataOffset {
		t.Fatalf("capture data address = 0x%X, want 0x%X", got, cave+overLimitCaveDataOffset)
	}
	wantCapture := []byte{
		0x4C, 0x8B, 0xDA, // mov r11,rdx
		0x49, 0x29, 0xDB, // sub r11,rbx
		0x49, 0x01, 0xF3, // add r11,rsi
		0x4D, 0x89, 0x1A, // mov [r10],r11
		0x41, 0x5B, // pop r11
		0x41, 0x5A, // pop r10
		0x9D, // popfq
	}
	if !bytes.Equal(code[15:32], wantCapture) {
		t.Fatalf("OverLimit cave does not restore R11/R10/flags with a balanced stack: % X", code[15:32])
	}
	if !bytes.Equal(code[32:32+len(overLimitSelectedOrig)], overLimitSelectedOrig) {
		t.Fatalf("displaced OverLimit instructions were not replayed after restoring scratch state: % X", code[32:32+len(overLimitSelectedOrig)])
	}
	jumpOffset := 32 + len(overLimitSelectedOrig)
	if got := relJumpTarget(cave+uintptr(jumpOffset), code[jumpOffset:jumpOffset+5]); got != returnAddr {
		t.Fatalf("OverLimit cave return target = 0x%X, want 0x%X", got, returnAddr)
	}
}

func TestValidateLegacyOverLimitCaveRecognizesExactClobberingLayoutForRestore(t *testing.T) {
	cave := uintptr(0x21000000)
	hook := uintptr(0x21001000)
	legacy := make([]byte, 0, overLimitCaveDataOffset+8)
	legacy = append(legacy, 0x49, 0xBA) // mov r10,data (old unsafe layout)
	legacy = binary.LittleEndian.AppendUint64(legacy, uint64(cave+overLimitCaveDataOffset))
	legacy = append(legacy,
		0x4C, 0x8B, 0xDA, // mov r11,rdx
		0x49, 0x29, 0xDB, // sub r11,rbx
		0x49, 0x01, 0xF3, // add r11,rsi
		0x4D, 0x89, 0x1A, // mov [r10],r11
	)
	legacy = append(legacy, overLimitSelectedOrig...)
	jump, err := makeRelJump(cave+uintptr(len(legacy)), hook+uintptr(len(overLimitSelectedOrig)), 5)
	if err != nil {
		t.Fatal(err)
	}
	legacy = append(legacy, jump...)
	for len(legacy) < int(overLimitCaveMarkerOffset) {
		legacy = append(legacy, 0)
	}
	legacy = append(legacy, []byte("GBFROLM2")...)
	for len(legacy) < int(overLimitCaveDataOffset)+8 {
		legacy = append(legacy, 0)
	}

	if err := validateLegacyOverLimitSelectedCave(cave, hook, legacy[:overLimitCaveDataOffset]); err != nil {
		t.Fatalf("exact old register-clobbering cave must remain recognizable for restore-only recovery: %v", err)
	}
	markerless := append([]byte(nil), legacy[:overLimitCaveDataOffset]...)
	clear(markerless[overLimitCaveMarkerOffset : overLimitCaveMarkerOffset+8])
	if err := validateLegacyOverLimitSelectedCave(cave, hook, markerless); err != nil {
		t.Fatalf("exact markerless legacy cave must remain recognizable for restore-only recovery: %v", err)
	}
}

func TestOverLimitStatusFailsClosedBeforeReadingExternalSelectedAddress(t *testing.T) {
	app, hook, cave := newOverLimitDetachTestApp(t)
	// Model a hook discovered in a fresh process: no cave has been adopted by
	// this App instance yet.
	app.overLimitCaveAddr = 0
	badMarker := []byte{0xCC}
	if err := writeProcessMemory(windows.CurrentProcess(), cave+overLimitCaveMarkerOffset, unsafe.Pointer(&badMarker[0]), uintptr(len(badMarker))); err != nil {
		t.Fatal(err)
	}
	invalidSelected := uintptr(1)
	if err := writeProcessMemory(windows.CurrentProcess(), cave+overLimitCaveDataOffset, unsafe.Pointer(&invalidSelected), unsafe.Sizeof(invalidSelected)); err != nil {
		t.Fatal(err)
	}

	status, err := app.readOverLimitStatusLocked()
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "ownership") {
		t.Fatalf("external cave should fail ownership validation, status=%+v err=%v", status, err)
	}
	if status.SelectedAddr != 0 || len(status.Slots) != 0 {
		t.Fatalf("external cave leaked selected data: %+v", status)
	}
	if app.overLimitHookAddr != 0 || app.overLimitCaveAddr != 0 {
		t.Fatalf("external hook was retained as owned recovery state: hook=0x%X cave=0x%X", app.overLimitHookAddr, app.overLimitCaveAddr)
	}
	entry := readOverLimitDetachTestBytes(t, hook, len(overLimitSelectedOrig))
	if !isOverLimitSelectedJump(entry) {
		t.Fatalf("fail-closed validation rewrote an external hook: % X", entry)
	}
	if err := app.CharaDetach(); err != nil {
		t.Fatalf("external hook should not poison process detach: %v", err)
	}
	entry = readOverLimitDetachTestBytes(t, hook, len(overLimitSelectedOrig))
	if !isOverLimitSelectedJump(entry) {
		t.Fatalf("detach rewrote a hook without an owner marker: % X", entry)
	}
}

func TestOverLimitLegacyMarkerlessCaveIsRestoredWithoutAdoption(t *testing.T) {
	app, hook, cave := newOverLimitDetachTestApp(t)
	legacy, err := buildLegacyOverLimitSelectedCave(cave, hook+uintptr(len(overLimitSelectedOrig)))
	if err != nil {
		t.Fatal(err)
	}
	if err := writeProcessMemory(windows.CurrentProcess(), cave, unsafe.Pointer(&legacy[0]), uintptr(len(legacy))); err != nil {
		t.Fatal(err)
	}
	zeros := make([]byte, len(overLimitLegacyCaveMarker))
	if err := writeProcessMemory(windows.CurrentProcess(), cave+overLimitCaveMarkerOffset, unsafe.Pointer(&zeros[0]), uintptr(len(zeros))); err != nil {
		t.Fatal(err)
	}
	invalidSelected := uintptr(1)
	if err := writeProcessMemory(windows.CurrentProcess(), cave+overLimitCaveDataOffset, unsafe.Pointer(&invalidSelected), unsafe.Sizeof(invalidSelected)); err != nil {
		t.Fatal(err)
	}

	status, err := app.readOverLimitStatusLocked()
	if err != nil {
		t.Fatalf("exact legacy cave should be recoverable: %v", err)
	}
	if status.Hooked || status.SelectedAddr != 0 || len(status.Slots) != 0 {
		t.Fatalf("legacy cave was adopted instead of restore-only: %+v", status)
	}
	if got := readOverLimitDetachTestBytes(t, hook, len(overLimitSelectedOrig)); !bytes.Equal(got, overLimitSelectedOrig) {
		t.Fatalf("legacy hook entry after recovery = % X, want % X", got, overLimitSelectedOrig)
	}
	if app.overLimitCaveAddr != 0 {
		t.Fatalf("legacy cave address remained adopted: 0x%X", app.overLimitCaveAddr)
	}
}
