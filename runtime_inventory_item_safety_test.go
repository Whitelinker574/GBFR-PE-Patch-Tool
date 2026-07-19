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

type fakeCT084SelectedMemory struct {
	bytes            map[uintptr]byte
	recordAddress    uintptr
	recordReads      int
	beforeRecordRead func(readNumber int, memory *fakeCT084SelectedMemory)
}

func newFakeCT084SelectedMemory() *fakeCT084SelectedMemory {
	return &fakeCT084SelectedMemory{bytes: make(map[uintptr]byte)}
}

func (memory *fakeCT084SelectedMemory) ReadAt(address uintptr, destination []byte) error {
	if address == memory.recordAddress && len(destination) == ct084SelectedItemRecordSize {
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

func (memory *fakeCT084SelectedMemory) WriteAt(address uintptr, source []byte) error {
	for index, value := range source {
		memory.bytes[address+uintptr(index)] = value
	}
	return nil
}

func (memory *fakeCT084SelectedMemory) put(address uintptr, value []byte) {
	_ = memory.WriteAt(address, value)
}

func (memory *fakeCT084SelectedMemory) putPointer(address, value uintptr) {
	encoded := make([]byte, 8)
	binary.LittleEndian.PutUint64(encoded, uint64(value))
	memory.put(address, encoded)
}

func selectedRecordBytes(hash, quantity, flags uint32) []byte {
	result := make([]byte, ct084SelectedItemRecordSize)
	binary.LittleEndian.PutUint32(result[0:4], hash)
	binary.LittleEndian.PutUint32(result[4:8], quantity)
	binary.LittleEndian.PutUint32(result[8:12], flags)
	return result
}

func TestBuildCT084SelectedMaterialCaveCapturesZeroWithoutClobberingFlagsOrRegisters(t *testing.T) {
	const cave = uintptr(0x10000000)
	process := processInstanceID{PID: 42, Created: 100}
	original := append([]byte(nil), ct084SelectedMaterialOriginal...)
	code, err := buildCT084SelectedCaptureCave(CT084SelectedItemMaterial, cave, cave+0x100, original, process, "owner-1")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(code[:ct084SelectedHookSize], original) {
		t.Fatalf("material cave must execute test rax,rax before capture: % X", code[:ct084SelectedHookSize])
	}
	capture := code[ct084SelectedHookSize : ct084SelectedHookSize+10]
	if !bytes.Equal(capture[:2], []byte{0x48, 0xA3}) {
		t.Fatalf("capture=% X, want unconditional mov [moffs64],rax", capture)
	}
	if got, want := uintptr(binary.LittleEndian.Uint64(capture[2:])), cave+ct084SelectedCaveDataOffset; got != want {
		t.Fatalf("capture target=0x%X, want 0x%X", got, want)
	}
	// MOV moffs64,RAX changes neither flags nor a scratch register. Because it
	// is unconditional, the material hook writes zero after test rax,rax and
	// therefore clears a stale selection before the original JE executes.
	if err := validateCT084SelectedCaptureCaveBytes(CT084SelectedItemMaterial, cave, cave+0x100, original, process, "owner-1", code); err != nil {
		t.Fatal(err)
	}
}

func TestBuildCT084SelectedKeyItemCavePreservesExactDisplacedInstructions(t *testing.T) {
	const cave = uintptr(0x20000000)
	process := processInstanceID{PID: 77, Created: 900}
	original := append([]byte(nil), ct084SelectedKeyItemOriginal...)
	code, err := buildCT084SelectedCaptureCave(CT084SelectedItemKeyItem, cave, cave+0x100, original, process, "owner-2")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(code[:ct084SelectedHookSize], original) {
		t.Fatalf("key-item displaced instructions=% X, want % X", code[:ct084SelectedHookSize], original)
	}
	if err := validateCT084SelectedCaptureCaveBytes(CT084SelectedItemKeyItem, cave, cave+0x100, original, process, "owner-2", code); err != nil {
		t.Fatal(err)
	}
	if err := validateCT084SelectedCaptureCaveBytes(CT084SelectedItemKeyItem, cave, cave+0x100, original, processInstanceID{PID: 77, Created: 901}, "owner-2", code); err == nil {
		t.Fatal("cave accepted a reused PID with a different creation time")
	}
	if err := validateCT084SelectedCaptureCaveBytes(CT084SelectedItemKeyItem, cave, cave+0x100, original, process, "foreign-owner", code); err == nil {
		t.Fatal("cave accepted a foreign owner token")
	}
}

func TestInstallPreparedCT084SelectedHookCanFreeCaveWhenJumpBuildFails(t *testing.T) {
	want := errors.New("rel32 out of range")
	installerCalled := false
	canFree, err := installPreparedCT084SelectedHook(
		0,
		ct084SelectedCaptureLease{HookAddr: 0x1000, CaveAddr: 0x2000, Original: append([]byte(nil), ct084SelectedKeyItemOriginal...)},
		func(uintptr, uintptr, int) ([]byte, error) { return nil, want },
		func(windows.Handle, uintptr, []byte, []byte) (bool, error) {
			installerCalled = true
			return false, nil
		},
	)
	if !errors.Is(err, want) || !canFree || installerCalled {
		t.Fatalf("jump failure=(canFree=%v err=%v installer=%v), want true/%v/false", canFree, err, installerCalled, want)
	}
}

func TestConsumeCT084SelectedItemRequiresExpectedAddressAndFullStableRecord(t *testing.T) {
	const (
		cave     = uintptr(0x30000000)
		selected = uintptr(0x40000000)
	)
	request := CT084SelectedItemReadRequest{Kind: CT084SelectedItemMaterial, ExpectedSelectedAddr: uint64(selected)}

	t.Run("success clears capture and requires reselection", func(t *testing.T) {
		memory := newFakeCT084SelectedMemory()
		memory.recordAddress = selected
		memory.putPointer(cave+ct084SelectedCaveDataOffset, selected)
		memory.put(selected, selectedRecordBytes(0xDB1D4F35, 17, 2))
		result, err := consumeCT084SelectedItemRecord(memory, cave, request)
		if err != nil {
			t.Fatal(err)
		}
		if result.Hash != 0xDB1D4F35 || result.Quantity != 17 || result.Flags != 2 || result.SelectedAddr != uint64(selected) {
			t.Fatalf("record=%+v", result)
		}
		pointer, err := readCT084SelectedPointer(memory, cave+ct084SelectedCaveDataOffset)
		if err != nil || pointer != 0 {
			t.Fatalf("successful read retained cave pointer=0x%X err=%v", pointer, err)
		}
		if _, err := consumeCT084SelectedItemRecord(memory, cave, request); err == nil {
			t.Fatal("second read reused a consumed selection without requiring reselection")
		}
	})

	t.Run("expected address mismatch", func(t *testing.T) {
		memory := newFakeCT084SelectedMemory()
		memory.recordAddress = selected
		memory.putPointer(cave+ct084SelectedCaveDataOffset, selected)
		memory.put(selected, selectedRecordBytes(1, 2, 3))
		wrong := request
		wrong.ExpectedSelectedAddr++
		if _, err := consumeCT084SelectedItemRecord(memory, cave, wrong); err == nil {
			t.Fatal("mismatched ExpectedSelectedAddr was accepted")
		}
	})

	t.Run("flags-only mutation proves all 0x0C bytes are revalidated", func(t *testing.T) {
		memory := newFakeCT084SelectedMemory()
		memory.recordAddress = selected
		memory.putPointer(cave+ct084SelectedCaveDataOffset, selected)
		memory.put(selected, selectedRecordBytes(1, 2, 3))
		memory.beforeRecordRead = func(readNumber int, current *fakeCT084SelectedMemory) {
			if readNumber == 2 {
				current.put(selected, selectedRecordBytes(1, 2, 4))
			}
		}
		if _, err := consumeCT084SelectedItemRecord(memory, cave, request); err == nil || !strings.Contains(strings.ToLower(err.Error()), "record") {
			t.Fatalf("record mutation error=%v", err)
		}
		pointer, _ := readCT084SelectedPointer(memory, cave+ct084SelectedCaveDataOffset)
		if pointer != selected {
			t.Fatal("failed record verification consumed the selection")
		}
	})

	t.Run("capture pointer mutation", func(t *testing.T) {
		memory := newFakeCT084SelectedMemory()
		memory.recordAddress = selected
		memory.putPointer(cave+ct084SelectedCaveDataOffset, selected)
		memory.put(selected, selectedRecordBytes(1, 2, 3))
		memory.beforeRecordRead = func(readNumber int, current *fakeCT084SelectedMemory) {
			if readNumber == 1 {
				current.putPointer(cave+ct084SelectedCaveDataOffset, selected+0x100)
			}
		}
		if _, err := consumeCT084SelectedItemRecord(memory, cave, request); err == nil {
			t.Fatal("capture pointer mutation was accepted")
		}
	})
}

func newCT084SelectedReleaseFixture(t *testing.T) (*App, uintptr, uintptr, []byte) {
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
	original := append([]byte(nil), ct084SelectedMaterialOriginal...)
	code, err := buildCT084SelectedCaptureCave(CT084SelectedItemMaterial, cave, hook+ct084SelectedHookSize, original, process, owner)
	if err != nil {
		t.Fatal(err)
	}
	patch, err := makeRelJump(hook, cave, ct084SelectedHookSize)
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
	if err := writeProcessMemory(current, cave+ct084SelectedCaveDataOffset, unsafe.Pointer(&selected), unsafe.Sizeof(selected)); err != nil {
		t.Fatal(err)
	}
	app := &App{
		hProcess:        hProcess,
		moduleBase:      page,
		charaPID:        process.PID,
		charaCreated:    process.Created,
		charaOwnerToken: owner,
		ct084SelectedMaterialHook: ct084SelectedCaptureLease{
			Kind: CT084SelectedItemMaterial, HookAddr: hook, CaveAddr: cave,
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

func TestReleaseCT084SelectedCaptureClearsCaveAndRestoresExactEntry(t *testing.T) {
	app, hook, cave, original := newCT084SelectedReleaseFixture(t)
	if err := app.releaseCT084SelectedCaptureHookLocked(&app.ct084SelectedMaterialHook, "chara-current", false); err != nil {
		t.Fatal(err)
	}
	if got := readSigilMemoryTestBytes(t, windows.CurrentProcess(), hook, len(original)); !bytes.Equal(got, original) {
		t.Fatalf("restored entry=% X, want % X", got, original)
	}
	if got := readSigilMemoryTestBytes(t, windows.CurrentProcess(), cave+ct084SelectedCaveDataOffset, 8); !bytes.Equal(got, make([]byte, 8)) {
		t.Fatalf("released cave retained selected pointer: % X", got)
	}
	if app.ct084SelectedMaterialHook.active() {
		t.Fatal("successful release retained material recovery lease")
	}
}

func TestCharaReleaseRestoresOwnedCT084SelectedCaptureBeforeClosingProcess(t *testing.T) {
	app, hook, cave, original := newCT084SelectedReleaseFixture(t)
	if err := app.CharaRelease("chara-current"); err != nil {
		t.Fatal(err)
	}
	if got := readSigilMemoryTestBytes(t, windows.CurrentProcess(), hook, len(original)); !bytes.Equal(got, original) {
		t.Fatalf("owned release entry=% X, want % X", got, original)
	}
	if got := readSigilMemoryTestBytes(t, windows.CurrentProcess(), cave+ct084SelectedCaveDataOffset, 8); !bytes.Equal(got, make([]byte, 8)) {
		t.Fatalf("owned release retained capture=% X", got)
	}
	if app.hProcess != 0 || app.ct084SelectedMaterialHook.active() || app.charaOwnerToken != "" {
		t.Fatalf("owned release retained process or lease: handle=%v owner=%q lease=%+v", app.hProcess, app.charaOwnerToken, app.ct084SelectedMaterialHook)
	}
}

func TestCharaDetachForceRestoresCT084SelectedCapture(t *testing.T) {
	app, hook, cave, original := newCT084SelectedReleaseFixture(t)
	app.charaOwnerToken = "different-current-owner"
	if err := app.CharaDetach(); err != nil {
		t.Fatal(err)
	}
	if got := readSigilMemoryTestBytes(t, windows.CurrentProcess(), hook, len(original)); !bytes.Equal(got, original) {
		t.Fatalf("force detach entry=% X, want % X", got, original)
	}
	if got := readSigilMemoryTestBytes(t, windows.CurrentProcess(), cave+ct084SelectedCaveDataOffset, 8); !bytes.Equal(got, make([]byte, 8)) {
		t.Fatalf("force detach retained capture=% X", got)
	}
	if app.hProcess != 0 || app.ct084SelectedMaterialHook.active() {
		t.Fatalf("force detach retained process or lease: handle=%v lease=%+v", app.hProcess, app.ct084SelectedMaterialHook)
	}
}

func TestReleaseCT084SelectedCaptureRejectsForeignOwnerWithoutMutation(t *testing.T) {
	app, hook, cave, _ := newCT084SelectedReleaseFixture(t)
	entryBefore := readSigilMemoryTestBytes(t, windows.CurrentProcess(), hook, ct084SelectedHookSize)
	captureBefore := readSigilMemoryTestBytes(t, windows.CurrentProcess(), cave+ct084SelectedCaveDataOffset, 8)
	if err := app.releaseCT084SelectedCaptureHookLocked(&app.ct084SelectedMaterialHook, "foreign-owner", false); !errors.Is(err, errRuntimeOwnerLeaseStale) {
		t.Fatalf("foreign owner error=%v", err)
	}
	if got := readSigilMemoryTestBytes(t, windows.CurrentProcess(), hook, ct084SelectedHookSize); !bytes.Equal(got, entryBefore) {
		t.Fatalf("foreign owner changed hook entry: % X", got)
	}
	if got := readSigilMemoryTestBytes(t, windows.CurrentProcess(), cave+ct084SelectedCaveDataOffset, 8); !bytes.Equal(got, captureBefore) {
		t.Fatalf("foreign owner changed cave capture: % X", got)
	}
	if !app.ct084SelectedMaterialHook.active() {
		t.Fatal("foreign owner discarded recovery lease")
	}
}

func TestCT084SelectedItemAPIsAreOwnedReadOnlyAndDoNotCallGameSave(t *testing.T) {
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
	for _, name := range []string{"CT084SelectedItemsEnableOwned", "CT084SelectedItemsStatusOwned", "CT084SelectedItemReadOwned", "CT084SelectedItemsDisableOwned"} {
		body := bodies[name]
		if body == nil || !blockCallsSelector(body, "a", "acquireOwnedRuntimeWriteLease") {
			t.Errorf("%s must validate the Chara owner inside the stable process lease", name)
		}
	}
	for _, name := range []string{"CT084SelectedItemsEnableOwned", "CT084SelectedItemReadOwned", "CT084SelectedItemsDisableOwned"} {
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
	var enable func(*App, string) (CT084SelectedItemsStatus, error) = (*App).CT084SelectedItemsEnableOwned
	var status func(*App, string) (CT084SelectedItemsStatus, error) = (*App).CT084SelectedItemsStatusOwned
	var read func(*App, string, CT084SelectedItemReadRequest) (CT084SelectedItemRecord, error) = (*App).CT084SelectedItemReadOwned
	var disable func(*App, string) (CT084SelectedItemsStatus, error) = (*App).CT084SelectedItemsDisableOwned
	if enable == nil || status == nil || read == nil || disable == nil {
		t.Fatal("selected-item API surface is incomplete")
	}
}
