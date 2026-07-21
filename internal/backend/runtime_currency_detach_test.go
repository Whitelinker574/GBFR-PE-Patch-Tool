package backend

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func newCurrencyDetachTestApp(t *testing.T) (*App, uintptr, uintptr, []byte) {
	t.Helper()
	current := windows.CurrentProcess()
	page, err := virtualAllocRemote(current, 0x1000, windows.PAGE_EXECUTE_READWRITE)
	if err != nil {
		t.Fatal(err)
	}
	hProcess, err := windows.OpenProcess(windows.PROCESS_ALL_ACCESS, false, uint32(os.Getpid()))
	if err != nil {
		_ = virtualFreeRemote(current, page)
		t.Fatal(err)
	}
	hook := page
	cave := page + 0x100
	original := []byte{0xBA, 0x78, 0x56, 0x34, 0x12}
	code, err := buildCurrencyCaptureCave(cave, hook+currencyHookSize, original)
	if err != nil {
		windows.CloseHandle(hProcess)
		_ = virtualFreeRemote(current, page)
		t.Fatal(err)
	}
	patch, err := makeRelJump(hook, cave, currencyHookSize)
	if err != nil {
		windows.CloseHandle(hProcess)
		_ = virtualFreeRemote(current, page)
		t.Fatal(err)
	}
	if err := writeProcessMemory(current, cave, unsafe.Pointer(&code[0]), uintptr(len(code))); err != nil {
		windows.CloseHandle(hProcess)
		_ = virtualFreeRemote(current, page)
		t.Fatal(err)
	}
	if err := writeProcessMemory(current, hook, unsafe.Pointer(&patch[0]), uintptr(len(patch))); err != nil {
		windows.CloseHandle(hProcess)
		_ = virtualFreeRemote(current, page)
		t.Fatal(err)
	}
	app := &App{
		hProcess:         hProcess,
		moduleBase:       page,
		charaPID:         uint32(os.Getpid()),
		currencyHookAddr: hook,
		currencyCaveAddr: cave,
		currencyOriginal: append([]byte(nil), original...),
	}
	t.Cleanup(func() {
		_ = writeProcessMemory(current, hook, unsafe.Pointer(&original[0]), uintptr(len(original)))
		if app.hProcess != 0 {
			windows.CloseHandle(app.hProcess)
			app.hProcess = 0
		}
		if err := virtualFreeRemote(current, page); err != nil {
			t.Errorf("free currency detach test page: %v", err)
		}
	})
	return app, hook, cave, original
}

func TestCharaDetachRestoresOnlyOwnedCurrencyHook(t *testing.T) {
	app, hook, _, original := newCurrencyDetachTestApp(t)

	if err := app.CharaDetach(); err != nil {
		t.Fatal(err)
	}
	if got := readOverLimitDetachTestBytes(t, hook, len(original)); !bytes.Equal(got, original) {
		t.Fatalf("currency hook entry after detach = % X, want % X", got, original)
	}
	if app.hProcess != 0 || app.currencyHookAddr != 0 || app.currencyCaveAddr != 0 || app.currencyOriginal != nil {
		t.Fatalf("successful detach retained currency state: handle=%v hook=0x%X cave=0x%X original=% X", app.hProcess, app.currencyHookAddr, app.currencyCaveAddr, app.currencyOriginal)
	}
}

func TestCharaReleaseRestoresCurrencyHookInstalledByItsOwnedPage(t *testing.T) {
	app, hook, _, original := newCurrencyDetachTestApp(t)
	app.charaOwnerToken = "misc-current"

	if err := app.CharaRelease("misc-current"); err != nil {
		t.Fatal(err)
	}
	if got := readOverLimitDetachTestBytes(t, hook, len(original)); !bytes.Equal(got, original) {
		t.Fatalf("currency hook entry after owned release = % X, want % X", got, original)
	}
	if app.hProcess != 0 || app.currencyHookAddr != 0 || app.currencyCaveAddr != 0 || app.currencyOriginal != nil || app.charaOwnerToken != "" {
		t.Fatalf("owned release retained currency/process state: handle=%v hook=0x%X cave=0x%X owner=%q", app.hProcess, app.currencyHookAddr, app.currencyCaveAddr, app.charaOwnerToken)
	}
}

func TestCharaDetachRejectsForeignCurrencyJumpWithoutOverwritingIt(t *testing.T) {
	app, hook, cave, _ := newCurrencyDetachTestApp(t)
	foreignCave := cave + 0x100
	foreignPatch, err := makeRelJump(hook, foreignCave, currencyHookSize)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeProcessMemory(windows.CurrentProcess(), hook, unsafe.Pointer(&foreignPatch[0]), uintptr(len(foreignPatch))); err != nil {
		t.Fatal(err)
	}

	err = app.CharaDetach()
	if err == nil || !strings.Contains(err.Error(), "currency") {
		t.Fatalf("detach should reject a foreign currency jump, got %v", err)
	}
	if app.hProcess == 0 || app.currencyHookAddr != hook || app.currencyCaveAddr != cave {
		t.Fatalf("failed detach discarded recovery state: handle=%v hook=0x%X cave=0x%X", app.hProcess, app.currencyHookAddr, app.currencyCaveAddr)
	}
	if got := readOverLimitDetachTestBytes(t, hook, len(foreignPatch)); !bytes.Equal(got, foreignPatch) {
		t.Fatalf("failed detach overwrote foreign jump: got % X want % X", got, foreignPatch)
	}
}

func TestCharaDetachRejectsCorruptOwnedCurrencyCave(t *testing.T) {
	app, hook, cave, _ := newCurrencyDetachTestApp(t)
	corrupt := []byte{0xCC}
	if err := writeProcessMemory(windows.CurrentProcess(), cave, unsafe.Pointer(&corrupt[0]), uintptr(len(corrupt))); err != nil {
		t.Fatal(err)
	}

	err := app.CharaDetach()
	if err == nil || !strings.Contains(err.Error(), "currency") {
		t.Fatalf("detach should reject a corrupt currency cave, got %v", err)
	}
	if app.hProcess == 0 || app.currencyHookAddr != hook || app.currencyCaveAddr != cave {
		t.Fatalf("failed detach discarded recovery state: handle=%v hook=0x%X cave=0x%X", app.hProcess, app.currencyHookAddr, app.currencyCaveAddr)
	}
	if entry := readOverLimitDetachTestBytes(t, hook, currencyHookSize); entry[0] != 0xE9 {
		t.Fatalf("failed detach unexpectedly rewrote hook entry: % X", entry)
	}
}

func TestInstallCurrencyHookRetainsRecoveryStateAfterPartialEntryWrite(t *testing.T) {
	current := windows.CurrentProcess()
	page, err := virtualAllocRemote(current, 0x1000, windows.PAGE_EXECUTE_READWRITE)
	if err != nil {
		t.Fatal(err)
	}
	hProcess, err := windows.OpenProcess(windows.PROCESS_ALL_ACCESS, false, uint32(os.Getpid()))
	if err != nil {
		_ = virtualFreeRemote(current, page)
		t.Fatal(err)
	}
	original := []byte{0xBA, 0x78, 0x56, 0x34, 0x12}
	if err := writeCodeMemory(hProcess, page, original); err != nil {
		windows.CloseHandle(hProcess)
		_ = virtualFreeRemote(current, page)
		t.Fatal(err)
	}

	app := &App{
		hProcess:         hProcess,
		moduleBase:       page,
		charaPID:         uint32(os.Getpid()),
		currencyHookAddr: page,
	}
	var allocatedCave uintptr
	injected := errors.New("injected partial currency hook write")
	previousInstaller := currencyInstallRemoteCodeHook
	currencyInstallRemoteCodeHook = func(h windows.Handle, addr uintptr, oldBytes, patch []byte) (codeHookInstallResult, error) {
		allocatedCave = relJumpTarget(addr, patch)
		if err := writeCodeMemory(h, addr, patch[:3]); err != nil {
			return codeHookInstallResult{State: codeHookEntryRecoveryRequired}, err
		}
		return codeHookInstallResult{State: codeHookEntryRecoveryRequired}, injected
	}
	t.Cleanup(func() {
		currencyInstallRemoteCodeHook = previousInstaller
		_ = writeCodeMemory(hProcess, page, original)
		if allocatedCave != 0 {
			_ = virtualFreeRemote(hProcess, allocatedCave)
		}
		windows.CloseHandle(hProcess)
		_ = virtualFreeRemote(current, page)
	})

	err = app.installCurrencyHook(original)
	if !errors.Is(err, injected) {
		t.Fatalf("install error = %v, want injected failure", err)
	}
	if allocatedCave == 0 {
		t.Fatal("test installer did not observe the allocated currency cave")
	}
	if app.currencyHookAddr != page || app.currencyCaveAddr != allocatedCave || !bytes.Equal(app.currencyOriginal, original) {
		t.Fatalf("partial install discarded recovery state: hook=0x%X cave=0x%X original=% X, want hook=0x%X cave=0x%X original=% X", app.currencyHookAddr, app.currencyCaveAddr, app.currencyOriginal, page, allocatedCave, original)
	}
	if err := app.releaseCurrencyHook(); err == nil {
		t.Fatal("corrupt partial entry must fail closed instead of pretending the hook was detached")
	}
	if app.currencyHookAddr != page || app.currencyCaveAddr != allocatedCave || !bytes.Equal(app.currencyOriginal, original) {
		t.Fatal("failed recovery discarded the retained currency hook lease")
	}
}
