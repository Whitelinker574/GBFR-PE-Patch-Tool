package backend

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
	"time"
)

func TestVirtualSigilRuntimeBinaryContract(t *testing.T) {
	selection := VirtualSigilSelection{SlotID: 42, GemID: 10, Trait1: 20, Trait1Level: 15, Trait2: 30, Trait2Level: 15, SigilLevel: 15}
	config := defaultVirtualSigilConfig()
	config.Characters["4D0A60C3"] = []VirtualSigilSelection{selection}
	data, err := encodeVirtualSigilRuntime(config, true)
	if err != nil || len(data) != 24+binary.Size(VirtualSigilConfigEntry{}) || !bytes.Equal(data[:8], []byte("GBFRVS02")) {
		t.Fatalf("virtual runtime binary mismatch: bytes=%d err=%v", len(data), err)
	}
	if binary.LittleEndian.Uint32(data[8:12]) != 2 || binary.LittleEndian.Uint32(data[12:16]) != 1 || binary.LittleEndian.Uint32(data[20:24]) != 1 {
		t.Fatalf("virtual runtime header mismatch: %x", data[:24])
	}
}

func TestNormalizeVirtualSigilConfigRejectsStaleAndDuplicateInstances(t *testing.T) {
	selection := VirtualSigilSelection{SlotID: 42, GemID: 10, Trait1: 20, Trait1Level: 15, SigilLevel: 15}
	inventory := []VirtualSigilInventoryItem{{VirtualSigilSelection: selection}}
	valid := defaultVirtualSigilConfig()
	valid.Characters["4D0A60C3"] = []VirtualSigilSelection{selection}
	if _, active, err := normalizeVirtualSigilConfig(valid, inventory); err != nil || active != 1 {
		t.Fatalf("valid virtual sigil config rejected: active=%d err=%v", active, err)
	}
	stale := valid
	stale.Characters = map[string][]VirtualSigilSelection{"4D0A60C3": {{SlotID: 42, GemID: 11, Trait1: 20, Trait1Level: 15, SigilLevel: 15}}}
	if _, _, err := normalizeVirtualSigilConfig(stale, inventory); err == nil {
		t.Fatal("stale physical instance expectation was accepted")
	}
	duplicate := valid
	duplicate.Characters = map[string][]VirtualSigilSelection{"4D0A60C3": {selection}, "0D21B430": {selection}}
	if _, _, err := normalizeVirtualSigilConfig(duplicate, inventory); err == nil {
		t.Fatal("duplicate physical instance ownership was accepted")
	}
}

func TestVirtualSigilInventoryStateMatchesCompanionAcceptance(t *testing.T) {
	entry := func(value uint32) *unitEntry {
		data := make([]byte, 4)
		binary.LittleEndian.PutUint32(data, value)
		return &unitEntry{ValueOff: 0, ValueCnt: 1, data: data}
	}
	if !virtualSigilInventoryStateUsable(entry(EmptyHash), true, entry(NormalSigilFlags), true) {
		t.Fatal("ordinary unequipped sigil was rejected")
	}
	if virtualSigilInventoryStateUsable(entry(0), true, entry(NormalSigilFlags), true) {
		t.Fatal("zero worn-by sentinel was accepted even though the runtime companion rejects it")
	}
	if virtualSigilInventoryStateUsable(entry(0x12345678), true, entry(NormalSigilFlags), true) {
		t.Fatal("equipped sigil was accepted")
	}
	if virtualSigilInventoryStateUsable(entry(EmptyHash), true, entry(virtualSigilDisabledFlag), true) {
		t.Fatal("disabled sigil was accepted")
	}
	if virtualSigilInventoryStateUsable(nil, false, entry(NormalSigilFlags), true) || virtualSigilInventoryStateUsable(entry(EmptyHash), true, nil, false) {
		t.Fatal("incomplete inventory state was accepted")
	}
}

func TestVirtualSigilLiveLifecycle(t *testing.T) {
	if os.Getenv("GBFR_VIRTUAL_SIGIL_QA") != "1" {
		t.Skip("set GBFR_VIRTUAL_SIGIL_QA=1 with the 2.0.2 game running")
	}
	path, err := virtualSigilBinaryPath()
	if err != nil {
		t.Fatal(err)
	}
	original, readErr := os.ReadFile(path)
	if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatal(readErr)
	}
	defer func() {
		if len(original) > 0 {
			if restoreErr := writeRuntimeCompanionFile(path, original); restoreErr != nil {
				t.Errorf("restore virtual-sigil runtime config: %v", restoreErr)
			}
		}
	}()

	config := readVirtualSigilConfig()
	enabled, err := encodeVirtualSigilRuntime(config, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeRuntimeCompanionFile(path, enabled); err != nil {
		t.Fatal(err)
	}
	app := &App{}
	if err := app.startRuntimeCompanion("virtual-sigils", "runtime_virtual_sigils"); err != nil {
		t.Fatal(err)
	}
	process, err := findRuntimeProcessInstance()
	if err != nil {
		t.Fatal(err)
	}
	status := readRuntimeCompanionStatus("virtual-sigils")
	if !runtimeCompanionMatchesProcess(status, process) || status.State != "active" {
		t.Fatalf("virtual-sigil runtime did not become active: %+v", status)
	}

	disabled, err := encodeVirtualSigilRuntime(config, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeRuntimeCompanionFile(path, disabled); err != nil {
		t.Fatal(err)
	}
	if err := waitRuntimeCompanionStopped("virtual-sigils", process); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	status = readRuntimeCompanionStatus("virtual-sigils")
	if !runtimeCompanionMatchesProcess(status, process) || status.State != "inactive" {
		t.Fatalf("virtual-sigil runtime did not restore hooks: %+v", status)
	}
}
