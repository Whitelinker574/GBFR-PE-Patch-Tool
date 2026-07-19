package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func newOverLimitDetachTestApp(t *testing.T) (*App, uintptr, uintptr) {
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
	code, err := buildOverLimitSelectedCave(cave, hook+uintptr(len(overLimitSelectedOrig)))
	if err != nil {
		windows.CloseHandle(hProcess)
		_ = virtualFreeRemote(current, page)
		t.Fatal(err)
	}
	patch, err := makeRelJump(hook, cave, len(overLimitSelectedOrig))
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
		hProcess:          hProcess,
		moduleBase:        page,
		charaPID:          uint32(os.Getpid()),
		overLimitHookAddr: hook,
		overLimitCaveAddr: cave,
	}
	t.Cleanup(func() {
		_ = writeProcessMemory(current, hook, unsafe.Pointer(&overLimitSelectedOrig[0]), uintptr(len(overLimitSelectedOrig)))
		if app.hProcess != 0 {
			windows.CloseHandle(app.hProcess)
			app.hProcess = 0
		}
		if err := virtualFreeRemote(current, page); err != nil {
			t.Errorf("free over-limit detach test page: %v", err)
		}
	})
	return app, hook, cave
}

func readOverLimitDetachTestBytes(t *testing.T, address uintptr, size int) []byte {
	t.Helper()
	data := make([]byte, size)
	if err := readProcessMemory(windows.CurrentProcess(), address, unsafe.Pointer(&data[0]), uintptr(len(data))); err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCharaDetachRestoresOwnedOverLimitHookBeforeClosingProcess(t *testing.T) {
	app, hook, _ := newOverLimitDetachTestApp(t)

	if err := app.CharaDetach(); err != nil {
		t.Fatal(err)
	}
	if got := readOverLimitDetachTestBytes(t, hook, len(overLimitSelectedOrig)); !bytes.Equal(got, overLimitSelectedOrig) {
		t.Fatalf("over-limit hook entry after detach = % X, want original % X", got, overLimitSelectedOrig)
	}
	if app.hProcess != 0 || app.moduleBase != 0 || app.charaPID != 0 {
		t.Fatalf("successful detach retained process state: handle=%v module=0x%X pid=%d", app.hProcess, app.moduleBase, app.charaPID)
	}
	if app.overLimitHookAddr != 0 || app.overLimitCaveAddr != 0 {
		t.Fatalf("successful detach retained over-limit state: hook=0x%X cave=0x%X", app.overLimitHookAddr, app.overLimitCaveAddr)
	}
}

func TestCharaDetachKeepsLeaseAndHookStateWhenOverLimitRestoreCannotBeProven(t *testing.T) {
	app, hook, cave := newOverLimitDetachTestApp(t)
	corrupt := []byte{0xCC}
	if err := writeProcessMemory(windows.CurrentProcess(), cave, unsafe.Pointer(&corrupt[0]), uintptr(len(corrupt))); err != nil {
		t.Fatal(err)
	}

	err := app.CharaDetach()
	if err == nil || !strings.Contains(err.Error(), "OverLimit") {
		t.Fatalf("detach should reject an unverified OverLimit cave, got %v", err)
	}
	if app.hProcess == 0 || app.moduleBase == 0 || app.charaPID == 0 {
		t.Fatalf("failed detach discarded the process lease: handle=%v module=0x%X pid=%d", app.hProcess, app.moduleBase, app.charaPID)
	}
	if app.overLimitHookAddr != hook || app.overLimitCaveAddr != cave {
		t.Fatalf("failed detach discarded hook recovery state: hook=0x%X cave=0x%X", app.overLimitHookAddr, app.overLimitCaveAddr)
	}
	entry := readOverLimitDetachTestBytes(t, hook, len(overLimitSelectedOrig))
	if entry[0] != 0xE9 {
		t.Fatalf("failed detach unexpectedly rewrote unverified hook entry: % X", entry)
	}
}
