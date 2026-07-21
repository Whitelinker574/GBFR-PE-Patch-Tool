package backend

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

type fakeReadOnlyProcessBackend struct {
	pid        uint32
	openAccess uint32
	handle     *fakeReadOnlyProcessHandle
	findErr    error
	openErr    error
}

func (backend *fakeReadOnlyProcessBackend) FindProcess(string) (uint32, error) {
	return backend.pid, backend.findErr
}

func (backend *fakeReadOnlyProcessBackend) OpenProcess(access uint32, _ uint32) (readOnlyProcessHandle, error) {
	backend.openAccess = access
	if backend.openErr != nil {
		return nil, backend.openErr
	}
	return backend.handle, nil
}

type fakeReadOnlyProcessHandle struct {
	memory      *fakeRuntimePanelMemory
	moduleBase  uintptr
	created     uint64
	alive       bool
	closed      bool
	identityErr error
	mapped      map[uint64]bool
}

func (handle *fakeReadOnlyProcessHandle) ReadAt(address uintptr, destination []byte) error {
	return handle.memory.ReadAt(address, destination)
}

func (handle *fakeReadOnlyProcessHandle) ModuleBase() (uintptr, error) { return handle.moduleBase, nil }
func (handle *fakeReadOnlyProcessHandle) CreationTime() (uint64, error) {
	return handle.created, handle.identityErr
}
func (handle *fakeReadOnlyProcessHandle) Alive() bool { return handle.alive }
func (handle *fakeReadOnlyProcessHandle) IsMappedAddress(value uint64) (bool, error) {
	return handle.mapped[value], nil
}
func (handle *fakeReadOnlyProcessHandle) Close() error {
	handle.closed = true
	return nil
}

func TestReadOnlyGameProcessAccessCannotWriteOrInject(t *testing.T) {
	const forbidden = windows.PROCESS_VM_WRITE |
		windows.PROCESS_VM_OPERATION |
		windows.PROCESS_CREATE_THREAD |
		windows.PROCESS_DUP_HANDLE |
		windows.PROCESS_SET_INFORMATION |
		windows.PROCESS_SET_QUOTA |
		windows.PROCESS_TERMINATE

	want := uint32(windows.PROCESS_QUERY_INFORMATION | windows.PROCESS_VM_READ)
	if readonlyGameProcessAccess != want {
		t.Fatalf("read-only process access = 0x%X, want exactly 0x%X", readonlyGameProcessAccess, want)
	}
	if readonlyGameProcessAccess&forbidden != 0 {
		t.Fatalf("read-only process access contains write/injection rights: 0x%X", readonlyGameProcessAccess&forbidden)
	}
}

func TestOpenReadOnlyGameProcessRejectsEmptyGuardSetBeforeOpeningHandle(t *testing.T) {
	backend := &fakeReadOnlyProcessBackend{
		pid: 42,
		handle: &fakeReadOnlyProcessHandle{
			memory: newFakeRuntimePanelMemory(), moduleBase: 0x140000000, created: 99, alive: true,
		},
	}

	process, err := openReadOnlyGameProcess(backend, "game.exe", nil)
	if err == nil {
		_ = process.Close()
		t.Fatal("empty version guard set was accepted")
	}
	if process != nil {
		t.Fatalf("failed open returned a process: %+v", process)
	}
	if backend.openAccess != 0 {
		t.Fatalf("empty guard set opened a process with access 0x%X", backend.openAccess)
	}
}

func TestOpenReadOnlyGameProcessClosesHandleWhenVersionGuardFails(t *testing.T) {
	memory := newFakeRuntimePanelMemory()
	handle := &fakeReadOnlyProcessHandle{
		memory: memory, moduleBase: 0x140000000, created: 99, alive: true,
	}
	backend := &fakeReadOnlyProcessBackend{pid: 42, handle: handle}
	guards := []runtimeCharacterPanelVersionGuard{{RVA: 0x100, Bytes: []byte{1, 2, 3}}}
	memory.put(handle.moduleBase+0x100, []byte{1, 2, 4})

	process, err := openReadOnlyGameProcess(backend, "game.exe", guards)
	if err == nil {
		_ = process.Close()
		t.Fatal("mismatched version guard was accepted")
	}
	if process != nil {
		t.Fatalf("failed open returned a process: %+v", process)
	}
	if !handle.closed {
		t.Fatal("handle was not closed after version verification failed")
	}
	if backend.openAccess != readonlyGameProcessAccess {
		t.Fatalf("OpenProcess access = 0x%X, want 0x%X", backend.openAccess, readonlyGameProcessAccess)
	}
	if got := fmt.Sprint(err); got == "" {
		t.Fatal("version failure did not return a diagnostic")
	}
}

func TestReadOnlyGameProcessRejectsChangedProcessIdentity(t *testing.T) {
	memory := newFakeRuntimePanelMemory()
	handle := &fakeReadOnlyProcessHandle{
		memory: memory, moduleBase: 0x140000000, created: 99, alive: true,
	}
	backend := &fakeReadOnlyProcessBackend{pid: 42, handle: handle}
	guards := []runtimeCharacterPanelVersionGuard{{RVA: 0x100, Bytes: []byte{1, 2, 3}}}
	memory.put(handle.moduleBase+0x100, []byte{1, 2, 3})
	process, err := openReadOnlyGameProcess(backend, "game.exe", guards)
	if err != nil {
		t.Fatal(err)
	}
	defer process.Close()

	handle.created++
	if err := process.Validate(); err == nil {
		t.Fatal("changed process creation identity was accepted")
	}
}

func TestReadOnlyGameProcessRejectsChangedVersionGuard(t *testing.T) {
	memory := newFakeRuntimePanelMemory()
	handle := &fakeReadOnlyProcessHandle{
		memory: memory, moduleBase: 0x140000000, created: 99, alive: true,
	}
	backend := &fakeReadOnlyProcessBackend{pid: 42, handle: handle}
	guards := []runtimeCharacterPanelVersionGuard{{RVA: 0x100, Bytes: []byte{1, 2, 3}}}
	memory.put(handle.moduleBase+0x100, []byte{1, 2, 3})
	process, err := openReadOnlyGameProcess(backend, "game.exe", guards)
	if err != nil {
		t.Fatal(err)
	}
	defer process.Close()

	memory.put(handle.moduleBase+0x100, []byte{1, 2, 4})
	if err := process.Validate(); err == nil {
		t.Fatal("changed game version guard was accepted")
	}
}

func TestFormulaSamplerSourcesContainNoWriteInjectionOrNetworkSymbols(t *testing.T) {
	for _, path := range []string{
		"readonly_game_process.go",
		"formula_sampler.go",
		"formula_sampler_scan.go",
		"formula_sampler_app.go",
		"formula_sample_bundle.go",
	} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{
			"WriteProcessMemory",
			"VirtualProtectEx",
			"VirtualAllocEx",
			"CreateRemoteThread",
			"PROCESS_ALL_ACCESS",
			"net/http",
		} {
			if strings.Contains(string(source), forbidden) {
				t.Fatalf("%s contains forbidden formula-sampler symbol %q", path, forbidden)
			}
		}
	}
}
