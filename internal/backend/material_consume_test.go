package backend

import (
	"bytes"
	"os"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestMaterialConsumeCaveKeepsPositiveGainsAndSkipsOnlyNegativeChanges(t *testing.T) {
	const cave = uintptr(0x180000000)
	const entry = uintptr(0x180001000)
	code, err := buildMaterialConsumeCave(cave, entry+materialConsumeHookSize)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateMaterialConsumeCaveBytes(cave, entry, code); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(code[4:11], materialConsumeOrig) {
		t.Fatalf("positive path must preserve add+mov instructions: % X", code[4:11])
	}
	if !bytes.Equal(code[16:19], materialConsumeOrig[4:]) {
		t.Fatalf("negative path must still preserve mov rcx,r12: % X", code[16:19])
	}
	if code[2] != 0x78 {
		t.Fatalf("expected signed-negative branch, got opcode %02X", code[2])
	}
}

func TestMaterialConsumeEntryIsDistinctFromInventoryQuantityHook(t *testing.T) {
	entry := uintptr(0x180001000)
	cave := uintptr(0x180010000)
	patch, err := makeMaterialConsumeEntry(entry, cave)
	if err != nil {
		t.Fatal(err)
	}
	if !isMaterialConsumeEntry(patch) {
		t.Fatalf("material entry was not recognized: % X", patch)
	}
	if got := classifySharedRuntimePatch(patch); got != sharedRuntimePatchOwnerMaterialConsume {
		t.Fatalf("classify material entry = %q", got)
	}
	inventory := append([]byte(nil), patch...)
	inventory[5], inventory[6] = 0x90, 0x90
	if got := classifySharedRuntimePatch(inventory); got != sharedRuntimePatchOwnerInventoryQuantity {
		t.Fatalf("classify inventory entry = %q", got)
	}
}

func TestMaterialConsumeEnableDisableRestoresExactEntry(t *testing.T) {
	handle := windows.CurrentProcess()
	page, err := virtualAllocRemote(handle, 0x1000, windows.PAGE_EXECUTE_READWRITE)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = writeCodeMemory(handle, page, materialConsumeOrig)
		_ = virtualFreeRemote(handle, page)
	}()
	if err := writeCodeMemory(handle, page, materialConsumeOrig); err != nil {
		t.Fatal(err)
	}
	created, err := processCreationTime(handle)
	if err != nil {
		t.Fatal(err)
	}
	const owner = "material-test-owner"
	app := &App{
		hProcess:            handle,
		moduleBase:          page - materialConsumeRVA,
		charaPID:            uint32(os.Getpid()),
		charaCreated:        created,
		charaOwnerToken:     owner,
		materialConsumeAddr: page,
	}

	status, err := app.materialConsumeSetEnabledLocked(owner, true)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Enabled || app.materialConsumeLease == nil {
		t.Fatalf("conditional hook was not enabled: %+v", status)
	}
	entry := make([]byte, materialConsumeHookSize)
	if err := readProcessMemory(handle, page, unsafe.Pointer(&entry[0]), uintptr(len(entry))); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(entry, app.materialConsumeLease.Installed) || !isMaterialConsumeEntry(entry) {
		t.Fatalf("unexpected installed entry: % X", entry)
	}

	status, err = app.materialConsumeSetEnabledLocked(owner, false)
	if err != nil {
		t.Fatal(err)
	}
	if status.Enabled || app.materialConsumeLease != nil {
		t.Fatalf("conditional hook was not released: %+v", status)
	}
	restored := make([]byte, materialConsumeHookSize)
	if err := readProcessMemory(handle, page, unsafe.Pointer(&restored[0]), uintptr(len(restored))); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, materialConsumeOrig) {
		t.Fatalf("entry was not restored exactly: % X", restored)
	}
	if len(app.retiredRuntimeCaves) != 1 {
		t.Fatalf("released reachable cave must be retired, got %d records", len(app.retiredRuntimeCaves))
	}
}

func TestMaterialConsumeLeaseBlocksSharedConnectionDetach(t *testing.T) {
	app := &App{materialConsumeLease: &materialConsumeHookLease{EntryAddr: 1}}
	if !app.hasActiveRuntimeHookLeaseLocked() {
		t.Fatal("material-consume recovery lease must keep the shared process connection alive")
	}
}
