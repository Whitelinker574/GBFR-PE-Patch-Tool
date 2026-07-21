package backend

import (
	"bytes"
	"encoding/binary"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func allocateSigilMemoryTestPage(t *testing.T) (windows.Handle, uintptr) {
	t.Helper()
	hProcess := windows.CurrentProcess()
	page, err := virtualAllocRemote(hProcess, 0x1000, windows.PAGE_EXECUTE_READWRITE)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := virtualFreeRemote(hProcess, page); err != nil {
			t.Errorf("free test page: %v", err)
		}
	})
	return hProcess, page
}

func writeSigilMemoryTestBytes(t *testing.T, hProcess windows.Handle, address uintptr, data []byte) {
	t.Helper()
	if err := writeProcessMemory(hProcess, address, unsafe.Pointer(&data[0]), uintptr(len(data))); err != nil {
		t.Fatal(err)
	}
}

func readSigilMemoryTestBytes(t *testing.T, hProcess windows.Handle, address uintptr, size int) []byte {
	t.Helper()
	data := make([]byte, size)
	if err := readProcessMemory(hProcess, address, unsafe.Pointer(&data[0]), uintptr(len(data))); err != nil {
		t.Fatal(err)
	}
	return data
}

func buildLegacySigilMemoryTestCave(t *testing.T, cave, returnAddr uintptr, original []byte) []byte {
	t.Helper()
	code := []byte{0x49, 0xBA}
	code = binary.LittleEndian.AppendUint64(code, uint64(cave+sigilMemoryCaveDataOffset))
	code = append(code, 0x49, 0x89, 0x02)
	code = append(code, original...)
	jump, err := makeRelJump(cave+uintptr(len(code)), returnAddr, 5)
	if err != nil {
		t.Fatal(err)
	}
	return append(code, jump...)
}

func TestBuildSigilMemoryCavePreservesR10(t *testing.T) {
	const cave = uintptr(0x10000000)
	original := append([]byte(nil), sigilMemoryOriginalBytes...)

	code, err := buildSigilMemoryCave(cave, cave+0x100, original)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := code[:4], []byte{0x41, 0x52, 0x49, 0xBA}; !bytes.Equal(got, want) {
		t.Fatalf("code cave prefix = % X, want push r10; mov r10 (% X)", got, want)
	}
	if got, want := uintptr(binary.LittleEndian.Uint64(code[4:12])), cave+sigilMemoryCaveDataOffset; got != want {
		t.Fatalf("code cave data address = 0x%X, want 0x%X", got, want)
	}
	if got, want := code[12:17], []byte{0x49, 0x89, 0x02, 0x41, 0x5A}; !bytes.Equal(got, want) {
		t.Fatalf("capture sequence = % X, want mov [r10], rax; pop r10 (% X)", got, want)
	}
	if got := code[17 : 17+len(original)]; !bytes.Equal(got, original) {
		t.Fatalf("displaced instructions = % X, want % X", got, original)
	}
	if !bytes.Equal(code[sigilMemoryMarkerOffset:sigilMemoryMarkerOffset+uintptr(len(sigilMemoryMarker))], sigilMemoryMarker) {
		t.Fatal("owned code cave marker missing")
	}
}

func TestRecoverSigilMemoryHookReadsPreservingCave(t *testing.T) {
	hProcess, cave := allocateSigilMemoryTestPage(t)
	original := append([]byte(nil), sigilMemoryOriginalBytes...)
	code, err := buildSigilMemoryCave(cave, cave+0x100, original)
	if err != nil {
		t.Fatal(err)
	}
	writeSigilMemoryTestBytes(t, hProcess, cave, code)

	got, err := (&App{hProcess: hProcess, sigilMemoryHookAddr: cave + 0x100 - sigilMemoryHookSize}).recoverSigilMemoryHook(cave)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("recovered instructions = % X, want % X", got, original)
	}
}

func TestReadSigilMemoryStatusUnloadsExactLegacyCaveWithoutAdoptingIt(t *testing.T) {
	hProcess, page := allocateSigilMemoryTestPage(t)
	hook := page
	cave := page + 0x100
	original := append([]byte(nil), sigilMemoryOriginalBytes...)
	legacy := buildLegacySigilMemoryTestCave(t, cave, hook+sigilMemoryHookSize, original)
	patch, err := makeRelJump(hook, cave, sigilMemoryHookSize)
	if err != nil {
		t.Fatal(err)
	}
	writeSigilMemoryTestBytes(t, hProcess, cave, legacy)
	writeSigilMemoryTestBytes(t, hProcess, hook, patch)

	app := &App{
		hProcess:            hProcess,
		moduleBase:          hook - sigilMemoryHookRVA,
		sigilMemoryHookAddr: hook,
	}
	status, err := app.readSigilMemoryStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.Hooked || app.sigilMemoryCaveAddr != 0 {
		t.Fatalf("legacy cave remained active or was adopted: status=%+v cave=0x%X", status, app.sigilMemoryCaveAddr)
	}
	if got := readSigilMemoryTestBytes(t, hProcess, hook, len(original)); !bytes.Equal(got, original) {
		t.Fatalf("legacy hook was not restored: % X", got)
	}
}

func TestReleaseSigilMemoryHookRestoresExactLegacyCaveEvenWithCachedOriginal(t *testing.T) {
	hProcess, page := allocateSigilMemoryTestPage(t)
	hook := page
	cave := page + 0x100
	original := append([]byte(nil), sigilMemoryOriginalBytes...)
	legacy := buildLegacySigilMemoryTestCave(t, cave, hook+sigilMemoryHookSize, original)
	patch, err := makeRelJump(hook, cave, sigilMemoryHookSize)
	if err != nil {
		t.Fatal(err)
	}
	writeSigilMemoryTestBytes(t, hProcess, cave, legacy)
	writeSigilMemoryTestBytes(t, hProcess, hook, patch)

	app := &App{
		hProcess:            hProcess,
		sigilMemoryHookAddr: hook,
		sigilMemoryCaveAddr: cave,
		sigilMemoryOriginal: append([]byte(nil), original...),
	}
	if err := app.releaseSigilMemoryHook(); err != nil {
		t.Fatal(err)
	}
	if got := readSigilMemoryTestBytes(t, hProcess, hook, len(original)); !bytes.Equal(got, original) {
		t.Fatalf("legacy hook was not restored: % X", got)
	}
	if app.sigilMemoryHookAddr != 0 || app.sigilMemoryCaveAddr != 0 || app.sigilMemoryOriginal != nil {
		t.Fatalf("release retained legacy state: %+v", app)
	}
}

func TestReleaseSigilMemoryHookRejectsDamagedJumpAndRetainsRecoveryState(t *testing.T) {
	hProcess, page := allocateSigilMemoryTestPage(t)
	hook := page
	cave := page + 0x100
	original := append([]byte(nil), sigilMemoryOriginalBytes...)
	code, err := buildSigilMemoryCave(cave, hook+sigilMemoryHookSize, original)
	if err != nil {
		t.Fatal(err)
	}
	patch, err := makeRelJump(hook, cave, sigilMemoryHookSize)
	if err != nil {
		t.Fatal(err)
	}
	patch[len(patch)-1] = 0xCC // model a partial/corrupt eight-byte hook write
	writeSigilMemoryTestBytes(t, hProcess, cave, code)
	writeSigilMemoryTestBytes(t, hProcess, hook, patch)

	app := &App{
		hProcess:            hProcess,
		sigilMemoryHookAddr: hook,
		sigilMemoryCaveAddr: cave,
		sigilMemoryOriginal: append([]byte(nil), original...),
	}
	if err := app.releaseSigilMemoryHook(); err == nil {
		t.Fatal("damaged E9 entry was treated as a successful detach")
	}
	if app.sigilMemoryHookAddr != hook || app.sigilMemoryCaveAddr != cave || !bytes.Equal(app.sigilMemoryOriginal, original) {
		t.Fatalf("failed detach discarded recovery state: hook=0x%X cave=0x%X original=% X", app.sigilMemoryHookAddr, app.sigilMemoryCaveAddr, app.sigilMemoryOriginal)
	}
	if got := readSigilMemoryTestBytes(t, hProcess, hook, len(patch)); !bytes.Equal(got, patch) {
		t.Fatalf("fail-closed detach rewrote an unverified entry: got % X want % X", got, patch)
	}
}

func TestSigilMemoryOriginalSignatureIsExactForLocalV202(t *testing.T) {
	want := []byte{0x31, 0xC9, 0x81, 0x38, 0xB0, 0xE0, 0x7A, 0x88}
	if !bytes.Equal(sigilMemoryOriginalBytes, want) || !isSigilMemoryOriginal(want) {
		t.Fatalf("signature = % X, want % X", sigilMemoryOriginalBytes, want)
	}
	for i := range want {
		mutated := append([]byte(nil), want...)
		mutated[i] ^= 1
		if isSigilMemoryOriginal(mutated) {
			t.Fatalf("signature accepted mismatch at byte %d", i)
		}
	}
}

func TestSigilMemoryDisableIsIdempotentWithoutProcess(t *testing.T) {
	status, err := (&App{}).SigilMemoryDisable()
	if err != nil {
		t.Fatal(err)
	}
	if status != (SigilMemoryStatus{}) {
		t.Fatalf("disable status = %#v, want zero status", status)
	}
}
