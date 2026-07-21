package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

type fakeRuntimePatchSelectedMemory struct {
	bytes            map[uintptr]byte
	recordAddress    uintptr
	recordReads      int
	beforeRecordRead func(readNumber int, memory *fakeRuntimePatchSelectedMemory)
}

func newFakeRuntimePatchSelectedMemory() *fakeRuntimePatchSelectedMemory {
	return &fakeRuntimePatchSelectedMemory{bytes: make(map[uintptr]byte)}
}

func (memory *fakeRuntimePatchSelectedMemory) ReadAt(address uintptr, destination []byte) error {
	if address == memory.recordAddress && len(destination) == runtimePatchSelectedItemRecordSize {
		memory.recordReads++
		if memory.beforeRecordRead != nil {
			memory.beforeRecordRead(memory.recordReads, memory)
		}
	}
	for index := range destination {
		value, ok := memory.bytes[address+uintptr(index)]
		if !ok {
			return errors.New("unmapped selected-item read")
		}
		destination[index] = value
	}
	return nil
}

func (memory *fakeRuntimePatchSelectedMemory) WriteAt(address uintptr, source []byte) error {
	for index, value := range source {
		memory.bytes[address+uintptr(index)] = value
	}
	return nil
}

func (memory *fakeRuntimePatchSelectedMemory) put(address uintptr, value []byte) {
	_ = memory.WriteAt(address, value)
}

func (memory *fakeRuntimePatchSelectedMemory) putPointer(address, value uintptr) {
	encoded := make([]byte, 8)
	binary.LittleEndian.PutUint64(encoded, uint64(value))
	memory.put(address, encoded)
}

func selectedRecordBytes(hash, quantity, flags uint32) []byte {
	result := make([]byte, runtimePatchSelectedItemRecordSize)
	binary.LittleEndian.PutUint32(result[0:4], hash)
	binary.LittleEndian.PutUint32(result[4:8], quantity)
	binary.LittleEndian.PutUint32(result[8:12], flags)
	return result
}

func TestBuildRuntimePatchSelectedMaterialCaveCapturesZeroWithoutClobberingFlagsOrRegisters(t *testing.T) {
	const cave = uintptr(0x10000000)
	process := processInstanceID{PID: 42, Created: 100}
	original := append([]byte(nil), runtimePatchSelectedMaterialOriginal...)
	code, err := buildRuntimePatchSelectedCaptureCave(RuntimePatchSelectedItemMaterial, cave, cave+0x100, original, process, "owner-1")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(code[:runtimePatchSelectedHookSize], original) {
		t.Fatalf("material cave must execute test rax,rax before capture: % X", code[:runtimePatchSelectedHookSize])
	}
	capture := code[runtimePatchSelectedHookSize : runtimePatchSelectedHookSize+10]
	if !bytes.Equal(capture[:2], []byte{0x48, 0xA3}) {
		t.Fatalf("capture=% X, want unconditional mov [moffs64],rax", capture)
	}
	if got, want := uintptr(binary.LittleEndian.Uint64(capture[2:])), cave+runtimePatchSelectedCaveDataOffset; got != want {
		t.Fatalf("capture target=0x%X, want 0x%X", got, want)
	}
	// MOV moffs64,RAX changes neither flags nor a scratch register. Because it
	// is unconditional, the material hook writes zero after test rax,rax and
	// therefore clears a stale selection before the original JE executes.
	if err := validateRuntimePatchSelectedCaptureCaveBytes(RuntimePatchSelectedItemMaterial, cave, cave+0x100, original, process, "owner-1", code); err != nil {
		t.Fatal(err)
	}
}

func TestBuildRuntimePatchSelectedKeyItemCavePreservesExactDisplacedInstructions(t *testing.T) {
	const cave = uintptr(0x20000000)
	process := processInstanceID{PID: 77, Created: 900}
	original := append([]byte(nil), runtimePatchSelectedKeyItemOriginal...)
	code, err := buildRuntimePatchSelectedCaptureCave(RuntimePatchSelectedItemKeyItem, cave, cave+0x100, original, process, "owner-2")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(code[:runtimePatchSelectedHookSize], original) {
		t.Fatalf("key-item displaced instructions=% X, want % X", code[:runtimePatchSelectedHookSize], original)
	}
	if err := validateRuntimePatchSelectedCaptureCaveBytes(RuntimePatchSelectedItemKeyItem, cave, cave+0x100, original, process, "owner-2", code); err != nil {
		t.Fatal(err)
	}
	if err := validateRuntimePatchSelectedCaptureCaveBytes(RuntimePatchSelectedItemKeyItem, cave, cave+0x100, original, processInstanceID{PID: 77, Created: 901}, "owner-2", code); err == nil {
		t.Fatal("cave accepted a reused PID with a different creation time")
	}
	if err := validateRuntimePatchSelectedCaptureCaveBytes(RuntimePatchSelectedItemKeyItem, cave, cave+0x100, original, process, "foreign-owner", code); err == nil {
		t.Fatal("cave accepted a foreign owner token")
	}
}

func TestInstallPreparedRuntimePatchSelectedHookCanFreeCaveWhenJumpBuildFails(t *testing.T) {
	want := errors.New("rel32 out of range")
	installerCalled := false
	result, err := installPreparedRuntimePatchSelectedHook(
		0,
		runtimePatchSelectedCaptureLease{HookAddr: 0x1000, CaveAddr: 0x2000, Original: append([]byte(nil), runtimePatchSelectedKeyItemOriginal...)},
		func(uintptr, uintptr, int) ([]byte, error) { return nil, want },
		func(windows.Handle, uintptr, []byte, []byte) (codeHookInstallResult, error) {
			installerCalled = true
			return codeHookInstallResult{State: codeHookEntryInstalled}, nil
		},
	)
	if !errors.Is(err, want) || result.State != codeHookEntryNeverPublished || !result.CanFreePreparedCave() || installerCalled {
		t.Fatalf("jump failure=(result=%+v err=%v installer=%v), want never-published/%v/false", result, err, installerCalled, want)
	}
}

func TestConsumeRuntimePatchSelectedItemRequiresExpectedAddressAndFullStableRecord(t *testing.T) {
	const (
		cave     = uintptr(0x30000000)
		selected = uintptr(0x40000000)
	)
	request := RuntimePatchSelectedItemReadRequest{Kind: RuntimePatchSelectedItemMaterial, ExpectedSelectedAddr: uint64(selected)}

	t.Run("success clears capture and requires reselection", func(t *testing.T) {
		memory := newFakeRuntimePatchSelectedMemory()
		memory.recordAddress = selected
		memory.putPointer(cave+runtimePatchSelectedCaveDataOffset, selected)
		memory.put(selected, selectedRecordBytes(0xDB1D4F35, 17, 2))
		result, err := consumeRuntimePatchSelectedItemRecord(memory, cave, request)
		if err != nil {
			t.Fatal(err)
		}
		if result.Hash != 0xDB1D4F35 || result.Quantity != 17 || result.Flags != 2 || result.SelectedAddr != uint64(selected) {
			t.Fatalf("record=%+v", result)
		}
		pointer, err := readRuntimePatchSelectedPointer(memory, cave+runtimePatchSelectedCaveDataOffset)
		if err != nil || pointer != 0 {
			t.Fatalf("successful read retained cave pointer=0x%X err=%v", pointer, err)
		}
		if _, err := consumeRuntimePatchSelectedItemRecord(memory, cave, request); err == nil {
			t.Fatal("second read reused a consumed selection without requiring reselection")
		}
	})

	t.Run("expected address mismatch", func(t *testing.T) {
		memory := newFakeRuntimePatchSelectedMemory()
		memory.recordAddress = selected
		memory.putPointer(cave+runtimePatchSelectedCaveDataOffset, selected)
		memory.put(selected, selectedRecordBytes(1, 2, 3))
		wrong := request
		wrong.ExpectedSelectedAddr++
		if _, err := consumeRuntimePatchSelectedItemRecord(memory, cave, wrong); err == nil {
			t.Fatal("mismatched ExpectedSelectedAddr was accepted")
		}
	})

	t.Run("flags-only mutation proves all 0x0C bytes are revalidated", func(t *testing.T) {
		memory := newFakeRuntimePatchSelectedMemory()
		memory.recordAddress = selected
		memory.putPointer(cave+runtimePatchSelectedCaveDataOffset, selected)
		memory.put(selected, selectedRecordBytes(1, 2, 3))
		memory.beforeRecordRead = func(readNumber int, current *fakeRuntimePatchSelectedMemory) {
			if readNumber == 2 {
				current.put(selected, selectedRecordBytes(1, 2, 4))
			}
		}
		if _, err := consumeRuntimePatchSelectedItemRecord(memory, cave, request); err == nil || !strings.Contains(strings.ToLower(err.Error()), "record") {
			t.Fatalf("record mutation error=%v", err)
		}
		pointer, _ := readRuntimePatchSelectedPointer(memory, cave+runtimePatchSelectedCaveDataOffset)
		if pointer != selected {
			t.Fatal("failed record verification consumed the selection")
		}
	})

	t.Run("capture pointer mutation", func(t *testing.T) {
		memory := newFakeRuntimePatchSelectedMemory()
		memory.recordAddress = selected
		memory.putPointer(cave+runtimePatchSelectedCaveDataOffset, selected)
		memory.put(selected, selectedRecordBytes(1, 2, 3))
		memory.beforeRecordRead = func(readNumber int, current *fakeRuntimePatchSelectedMemory) {
			if readNumber == 1 {
				current.putPointer(cave+runtimePatchSelectedCaveDataOffset, selected+0x100)
			}
		}
		if _, err := consumeRuntimePatchSelectedItemRecord(memory, cave, request); err == nil {
			t.Fatal("capture pointer mutation was accepted")
		}
	})
}

func newRuntimePatchSelectedReleaseFixture(t *testing.T) (*App, uintptr, uintptr, []byte) {
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
	hook, cave := page, page+0x100
	process := processInstanceID{PID: uint32(os.Getpid()), Created: 1234}
	owner := "chara-current"
	original := append([]byte(nil), runtimePatchSelectedMaterialOriginal...)
	code, err := buildRuntimePatchSelectedCaptureCave(RuntimePatchSelectedItemMaterial, cave, hook+runtimePatchSelectedHookSize, original, process, owner)
	if err != nil {
		t.Fatal(err)
	}
	patch, err := makeRelJump(hook, cave, runtimePatchSelectedHookSize)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeProcessMemory(current, cave, unsafe.Pointer(&code[0]), uintptr(len(code))); err != nil {
		t.Fatal(err)
	}
	if err := writeProcessMemory(current, hook, unsafe.Pointer(&patch[0]), uintptr(len(patch))); err != nil {
		t.Fatal(err)
	}
	selected := uintptr(0x55667788)
	if err := writeProcessMemory(current, cave+runtimePatchSelectedCaveDataOffset, unsafe.Pointer(&selected), unsafe.Sizeof(selected)); err != nil {
		t.Fatal(err)
	}
	app := &App{
		hProcess:        hProcess,
		moduleBase:      page,
		charaPID:        process.PID,
		charaCreated:    process.Created,
		charaOwnerToken: owner,
		runtimePatchSelectedMaterialHook: runtimePatchSelectedCaptureLease{
			Kind: RuntimePatchSelectedItemMaterial, HookAddr: hook, CaveAddr: cave,
			Original: original, Process: process, OwnerToken: owner,
		},
	}
	t.Cleanup(func() {
		_ = writeCodeMemory(windows.CurrentProcess(), hook, original)
		if app.hProcess != 0 {
			windows.CloseHandle(app.hProcess)
			app.hProcess = 0
		}
		if err := virtualFreeRemote(current, page); err != nil {
			t.Errorf("free selected-item test page: %v", err)
		}
	})
	return app, hook, cave, original
}

func TestReleaseRuntimePatchSelectedCaptureClearsCaveAndRestoresExactEntry(t *testing.T) {
	app, hook, cave, original := newRuntimePatchSelectedReleaseFixture(t)
	if err := app.releaseRuntimePatchSelectedCaptureHookLocked(&app.runtimePatchSelectedMaterialHook, "chara-current", false); err != nil {
		t.Fatal(err)
	}
	if got := readSigilMemoryTestBytes(t, windows.CurrentProcess(), hook, len(original)); !bytes.Equal(got, original) {
		t.Fatalf("restored entry=% X, want % X", got, original)
	}
	if got := readSigilMemoryTestBytes(t, windows.CurrentProcess(), cave+runtimePatchSelectedCaveDataOffset, 8); !bytes.Equal(got, make([]byte, 8)) {
		t.Fatalf("released cave retained selected pointer: % X", got)
	}
	if app.runtimePatchSelectedMaterialHook.active() {
		t.Fatal("successful release retained material recovery lease")
	}
	if got := len(app.retiredRuntimeCaves); got != 1 || app.retiredRuntimeCaves[0].Address != cave {
		t.Fatalf("retired caves=%+v, want released cave 0x%X retained until process detach", app.retiredRuntimeCaves, cave)
	}
}

func TestCharaReleaseRestoresOwnedRuntimePatchSelectedCaptureBeforeClosingProcess(t *testing.T) {
	app, hook, cave, original := newRuntimePatchSelectedReleaseFixture(t)
	if err := app.CharaRelease("chara-current"); err != nil {
		t.Fatal(err)
	}
	if got := readSigilMemoryTestBytes(t, windows.CurrentProcess(), hook, len(original)); !bytes.Equal(got, original) {
		t.Fatalf("owned release entry=% X, want % X", got, original)
	}
	if got := readSigilMemoryTestBytes(t, windows.CurrentProcess(), cave+runtimePatchSelectedCaveDataOffset, 8); !bytes.Equal(got, make([]byte, 8)) {
		t.Fatalf("owned release retained capture=% X", got)
	}
	if app.hProcess != 0 || app.runtimePatchSelectedMaterialHook.active() || app.charaOwnerToken != "" {
		t.Fatalf("owned release retained process or lease: handle=%v owner=%q lease=%+v", app.hProcess, app.charaOwnerToken, app.runtimePatchSelectedMaterialHook)
	}
}

func TestCharaDetachForceRestoresRuntimePatchSelectedCapture(t *testing.T) {
	app, hook, cave, original := newRuntimePatchSelectedReleaseFixture(t)
	app.charaOwnerToken = "different-current-owner"
	if err := app.CharaDetach(); err != nil {
		t.Fatal(err)
	}
	if got := readSigilMemoryTestBytes(t, windows.CurrentProcess(), hook, len(original)); !bytes.Equal(got, original) {
		t.Fatalf("force detach entry=% X, want % X", got, original)
	}
	if got := readSigilMemoryTestBytes(t, windows.CurrentProcess(), cave+runtimePatchSelectedCaveDataOffset, 8); !bytes.Equal(got, make([]byte, 8)) {
		t.Fatalf("force detach retained capture=% X", got)
	}
	if app.hProcess != 0 || app.runtimePatchSelectedMaterialHook.active() {
		t.Fatalf("force detach retained process or lease: handle=%v lease=%+v", app.hProcess, app.runtimePatchSelectedMaterialHook)
	}
	if len(app.retiredRuntimeCaves) != 0 {
		t.Fatalf("force detach retained retired-cave metadata: %+v", app.retiredRuntimeCaves)
	}
}

func TestReleaseRuntimePatchSelectedCaptureRejectsForeignOwnerWithoutMutation(t *testing.T) {
	app, hook, cave, _ := newRuntimePatchSelectedReleaseFixture(t)
	entryBefore := readSigilMemoryTestBytes(t, windows.CurrentProcess(), hook, runtimePatchSelectedHookSize)
	captureBefore := readSigilMemoryTestBytes(t, windows.CurrentProcess(), cave+runtimePatchSelectedCaveDataOffset, 8)
	if err := app.releaseRuntimePatchSelectedCaptureHookLocked(&app.runtimePatchSelectedMaterialHook, "foreign-owner", false); !errors.Is(err, errRuntimeOwnerLeaseStale) {
		t.Fatalf("foreign owner error=%v", err)
	}
	if got := readSigilMemoryTestBytes(t, windows.CurrentProcess(), hook, runtimePatchSelectedHookSize); !bytes.Equal(got, entryBefore) {
		t.Fatalf("foreign owner changed hook entry: % X", got)
	}
	if got := readSigilMemoryTestBytes(t, windows.CurrentProcess(), cave+runtimePatchSelectedCaveDataOffset, 8); !bytes.Equal(got, captureBefore) {
		t.Fatalf("foreign owner changed cave capture: % X", got)
	}
	if !app.runtimePatchSelectedMaterialHook.active() {
		t.Fatal("foreign owner discarded recovery lease")
	}
}

func TestRuntimePatchSelectedItemAPIsAreOwnedReadOnlyAndDoNotCallGameSave(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "runtime_inventory_item.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	bodies := make(map[string]*ast.BlockStmt)
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil {
			bodies[fn.Name.Name] = fn.Body
		}
	}
	for _, name := range []string{"RuntimePatchSelectedItemsEnableOwned", "RuntimePatchSelectedItemsStatusOwned", "RuntimePatchSelectedItemReadOwned", "RuntimePatchSelectedItemsDisableOwned"} {
		body := bodies[name]
		if body == nil || !blockCallsSelector(body, "a", "acquireOwnedRuntimeWriteLease") {
			t.Errorf("%s must validate the Chara owner inside the stable process lease", name)
		}
	}
	for _, name := range []string{"RuntimePatchSelectedItemsEnableOwned", "RuntimePatchSelectedItemReadOwned", "RuntimePatchSelectedItemsDisableOwned"} {
		if !blockCallsSelector(bodies[name], "liveMemoryWriteMu", "Lock") {
			t.Errorf("%s must follow liveMemoryWriteMu -> procMu -> runtimePatchMu lock order", name)
		}
	}
	source, err := os.ReadFile("runtime_inventory_item.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"sigilMemorySaveRVA", "callRemoteOneArg", "snapshotBeforeLiveSaveChange"} {
		if bytes.Contains(source, []byte(forbidden)) {
			t.Errorf("read-only selected-item implementation contains forbidden save/write primitive %q", forbidden)
		}
	}
	var enable func(*App, string) (RuntimePatchSelectedItemsStatus, error) = (*App).RuntimePatchSelectedItemsEnableOwned
	var status func(*App, string) (RuntimePatchSelectedItemsStatus, error) = (*App).RuntimePatchSelectedItemsStatusOwned
	var read func(*App, string, RuntimePatchSelectedItemReadRequest) (RuntimePatchSelectedItemRecord, error) = (*App).RuntimePatchSelectedItemReadOwned
	var disable func(*App, string) (RuntimePatchSelectedItemsStatus, error) = (*App).RuntimePatchSelectedItemsDisableOwned
	if enable == nil || status == nil || read == nil || disable == nil {
		t.Fatal("selected-item API surface is incomplete")
	}
}
