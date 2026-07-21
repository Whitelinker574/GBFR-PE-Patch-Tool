package backend

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestBuildWrightstoneMemoryCavePreservesR10AndCapturesRDX(t *testing.T) {
	cave := uintptr(0x140120000)
	returnAddr := uintptr(0x1403222D7)
	original := append([]byte(nil), wrightstoneMemoryOriginalBytes...)

	code, err := buildWrightstoneMemoryCave(cave, returnAddr, original)
	if err != nil {
		t.Fatal(err)
	}
	if len(code) < int(wrightstoneMemoryCaveDataOffset)+8 {
		t.Fatalf("code cave too short: %d", len(code))
	}
	if !bytes.Equal(code[:2], []byte{0x41, 0x52}) {
		t.Fatalf("code cave must push r10 first: % X", code[:2])
	}
	if !bytes.Equal(code[2:4], []byte{0x49, 0xBA}) {
		t.Fatalf("missing mov r10, imm64: % X", code[2:4])
	}
	if got := uintptr(binary.LittleEndian.Uint64(code[4:12])); got != cave+wrightstoneMemoryCaveDataOffset {
		t.Fatalf("capture data address = 0x%X, want 0x%X", got, cave+wrightstoneMemoryCaveDataOffset)
	}
	if !bytes.Equal(code[12:15], []byte{0x49, 0x89, 0x12}) {
		t.Fatalf("missing mov [r10], rdx: % X", code[12:15])
	}
	if !bytes.Equal(code[15:17], []byte{0x41, 0x5A}) {
		t.Fatalf("code cave must restore r10 before original instructions: % X", code[15:17])
	}
	if !bytes.Equal(code[wrightstoneMemoryOriginalOffset:wrightstoneMemoryOriginalOffset+wrightstoneMemoryHookSize], original) {
		t.Fatalf("displaced instructions are not preserved at the owned offset")
	}
	jumpOffset := wrightstoneMemoryOriginalOffset + wrightstoneMemoryHookSize
	if got := relJumpTarget(cave+jumpOffset, code[jumpOffset:jumpOffset+5]); got != returnAddr {
		t.Fatalf("return jump target = 0x%X, want 0x%X", got, returnAddr)
	}
	if !bytes.Equal(code[wrightstoneMemoryMarkerOffset:wrightstoneMemoryMarkerOffset+uintptr(len(wrightstoneMemoryMarker))], wrightstoneMemoryMarker) {
		t.Fatal("owned code cave marker missing")
	}
}

func TestDecodeWrightstoneMemoryCaveRejectsForeignOrRegisterClobberingCode(t *testing.T) {
	cave := uintptr(0x140220000)
	code, err := buildWrightstoneMemoryCave(cave, 0x1403222D7, wrightstoneMemoryOriginalBytes)
	if err != nil {
		t.Fatal(err)
	}
	original, err := decodeWrightstoneMemoryCave(cave, code)
	if err != nil || !bytes.Equal(original, wrightstoneMemoryOriginalBytes) {
		t.Fatalf("owned safe cave should decode: original=% X err=%v", original, err)
	}

	foreign := append([]byte(nil), code...)
	foreign[wrightstoneMemoryMarkerOffset] ^= 0xFF
	if _, err := decodeWrightstoneMemoryCave(cave, foreign); err == nil {
		t.Fatal("foreign cave marker must be rejected")
	}

	unsafeLegacy := append([]byte(nil), code...)
	copy(unsafeLegacy[:13], append([]byte{0x49, 0xBA}, code[4:12]...))
	unsafeLegacy[10], unsafeLegacy[11], unsafeLegacy[12] = 0x49, 0x89, 0x12
	if _, err := decodeWrightstoneMemoryCave(cave, unsafeLegacy); err == nil {
		t.Fatal("cave that clobbers r10 must not be adopted")
	}
}

func TestWrightstoneMemoryOriginalSignatureIsExact(t *testing.T) {
	wantV202 := []byte{0x48, 0x89, 0xD7, 0x48, 0x89, 0xCE}
	if !bytes.Equal(wrightstoneMemoryOriginalBytes, wantV202) {
		t.Fatalf("wrightstone hook signature = % X, want DLC 2.0.2 runtime catalog current-view bytes % X", wrightstoneMemoryOriginalBytes, wantV202)
	}
	if !isWrightstoneMemoryOriginal(wrightstoneMemoryOriginalBytes) {
		t.Fatal("verified original bytes should match")
	}
	for i := range wrightstoneMemoryOriginalBytes {
		mutated := append([]byte(nil), wrightstoneMemoryOriginalBytes...)
		mutated[i] ^= 0x01
		if isWrightstoneMemoryOriginal(mutated) {
			t.Fatalf("signature accepted a mismatch at byte %d", i)
		}
	}
}

func TestWrightstoneMemoryDLCSupplementGuardIsExactAndRejectsPartialMatches(t *testing.T) {
	want := []byte{
		0x48, 0x89, 0xD7, 0x48, 0x89, 0xCE, 0xE8, 0xF1, 0x05, 0xFC, 0xFF,
		0x48, 0x39, 0xFE, 0x74, 0x4C, 0x48, 0x8D, 0x4E, 0x18, 0x8B, 0x47, 0x18,
	}
	if wrightstoneMemoryHookRVA != 0x361CB4 {
		t.Fatalf("wrightstone current-view RVA = 0x%X, want 0x361CB4", wrightstoneMemoryHookRVA)
	}
	if !bytes.Equal(wrightstoneMemoryGuardBytes, want) {
		t.Fatalf("wrightstone DLC 2.0.2 runtime catalog guard = % X, want % X", wrightstoneMemoryGuardBytes, want)
	}
	if !isWrightstoneMemoryGuard(want, false) {
		t.Fatal("verified DLC 2.0.2 runtime catalog guard should match")
	}
	for i := range want {
		mutated := append([]byte(nil), want...)
		mutated[i] ^= 0x01
		if isWrightstoneMemoryGuard(mutated, false) {
			t.Fatalf("guard accepted mismatch at byte %d", i)
		}
	}

	hooked := append([]byte(nil), want...)
	copy(hooked[:wrightstoneMemoryHookSize], []byte{0xE9, 0, 0, 0, 0, 0x90})
	if !isWrightstoneMemoryGuard(hooked, true) {
		t.Fatal("owned-hook entry with intact guarded tail should match")
	}
	hooked[len(hooked)-1] ^= 0x01
	if isWrightstoneMemoryGuard(hooked, true) {
		t.Fatal("owned-hook entry with damaged guarded tail must be rejected")
	}
}

func TestWrightstoneMemoryStatusIdentifiesDLCSupplementCaptureSource(t *testing.T) {
	status := newWrightstoneMemoryStatus(true, false, 0x140361CB4, 0x140000000, wrightstoneMemoryOriginalBytes)
	if status.CaptureSource != "dlcSupplement-current-view" || status.SourceVersion != "2.0.2" {
		t.Fatalf("capture provenance = %q/%q", status.CaptureSource, status.SourceVersion)
	}
}

func TestWrightstoneSaveFunctionSignatureIsExact(t *testing.T) {
	wantV202 := []byte{
		0x55, 0x48, 0x83, 0xEC, 0x60, 0x48, 0x8D, 0x6C, 0x24, 0x60,
		0x48, 0xC7, 0x45, 0xF8, 0xFE, 0xFF, 0xFF, 0xFF,
	}
	if !bytes.Equal(gameSaveFunctionPrologue, wantV202) {
		t.Fatalf("save signature = % X, want local game v2.0.2 bytes % X", gameSaveFunctionPrologue, wantV202)
	}
}

func TestReadWrightstoneMemoryStatusRejectsChangedJumpTargetAfterCaching(t *testing.T) {
	hProcess, page := allocateSigilMemoryTestPage(t)
	hook := page
	ownedCave := page + 0x100
	wrongCachedCave := page + 0x300
	code, err := buildWrightstoneMemoryCave(ownedCave, hook+wrightstoneMemoryHookSize, wrightstoneMemoryOriginalBytes)
	if err != nil {
		t.Fatal(err)
	}
	patch, err := makeRelJump(hook, ownedCave, int(wrightstoneMemoryHookSize))
	if err != nil {
		t.Fatal(err)
	}
	writeSigilMemoryTestBytes(t, hProcess, ownedCave, code)
	writeSigilMemoryTestBytes(t, hProcess, hook, patch)

	app := &App{
		hProcess:                  hProcess,
		moduleBase:                hook - wrightstoneMemoryHookRVA,
		wrightstoneMemoryHookAddr: hook,
		wrightstoneMemoryCaveAddr: wrongCachedCave,
		wrightstoneMemoryOriginal: append([]byte(nil), wrightstoneMemoryOriginalBytes...),
	}
	if _, err := app.readWrightstoneMemoryStatusLocked(); err == nil {
		t.Fatal("changed hook target was accepted because a stale cave was cached")
	}
}

func TestReadWrightstoneMemoryStatusRevalidatesOwnedCaveEveryTime(t *testing.T) {
	hProcess, page := allocateSigilMemoryTestPage(t)
	hook := page
	cave := page + 0x100
	code, err := buildWrightstoneMemoryCave(cave, hook+wrightstoneMemoryHookSize, wrightstoneMemoryOriginalBytes)
	if err != nil {
		t.Fatal(err)
	}
	patch, err := makeRelJump(hook, cave, int(wrightstoneMemoryHookSize))
	if err != nil {
		t.Fatal(err)
	}
	code[wrightstoneMemoryMarkerOffset] ^= 0xFF
	writeSigilMemoryTestBytes(t, hProcess, cave, code)
	writeSigilMemoryTestBytes(t, hProcess, hook, patch)

	app := &App{
		hProcess:                  hProcess,
		moduleBase:                hook - wrightstoneMemoryHookRVA,
		wrightstoneMemoryHookAddr: hook,
		wrightstoneMemoryCaveAddr: cave,
		wrightstoneMemoryOriginal: append([]byte(nil), wrightstoneMemoryOriginalBytes...),
	}
	if _, err := app.readWrightstoneMemoryStatusLocked(); err == nil {
		t.Fatal("cached cave with a damaged ownership marker was accepted")
	}
}

func TestReleaseWrightstoneMemoryHookRetainsOrphanedRecoveryState(t *testing.T) {
	app := &App{
		wrightstoneMemoryCaveAddr: 0x12340000,
		wrightstoneMemoryOriginal: append([]byte(nil), wrightstoneMemoryOriginalBytes...),
	}
	if err := app.releaseWrightstoneMemoryHookLocked(); err == nil {
		t.Fatal("release silently accepted a cave lease without a live entry/handle")
	}
	if app.wrightstoneMemoryCaveAddr != 0x12340000 || !bytes.Equal(app.wrightstoneMemoryOriginal, wrightstoneMemoryOriginalBytes) {
		t.Fatalf("failed release discarded recovery state: cave=0x%X original=% X", app.wrightstoneMemoryCaveAddr, app.wrightstoneMemoryOriginal)
	}
}

func TestReleaseWrightstoneMemoryHookRejectsCorruptCaveAndRetainsLease(t *testing.T) {
	hProcess, page := allocateSigilMemoryTestPage(t)
	hook := page
	cave := page + 0x100
	code, err := buildWrightstoneMemoryCave(cave, hook+wrightstoneMemoryHookSize, wrightstoneMemoryOriginalBytes)
	if err != nil {
		t.Fatal(err)
	}
	patch, err := makeRelJump(hook, cave, int(wrightstoneMemoryHookSize))
	if err != nil {
		t.Fatal(err)
	}
	code[wrightstoneMemoryMarkerOffset] ^= 0xFF
	writeSigilMemoryTestBytes(t, hProcess, cave, code)
	writeSigilMemoryTestBytes(t, hProcess, hook, patch)

	app := &App{
		hProcess:                  hProcess,
		wrightstoneMemoryHookAddr: hook,
		wrightstoneMemoryCaveAddr: cave,
		wrightstoneMemoryOriginal: append([]byte(nil), wrightstoneMemoryOriginalBytes...),
	}
	if err := app.releaseWrightstoneMemoryHookLocked(); err == nil {
		t.Fatal("release accepted a corrupt owned-cave marker")
	}
	if app.wrightstoneMemoryHookAddr != hook || app.wrightstoneMemoryCaveAddr != cave || !bytes.Equal(app.wrightstoneMemoryOriginal, wrightstoneMemoryOriginalBytes) {
		t.Fatalf("failed release discarded recovery lease: hook=0x%X cave=0x%X original=% X", app.wrightstoneMemoryHookAddr, app.wrightstoneMemoryCaveAddr, app.wrightstoneMemoryOriginal)
	}
}
