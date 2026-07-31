package backend

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestTaskRewardMultiplierCaveFiltersOrdinaryItemRecords(t *testing.T) {
	cave := uintptr(0x140100000)
	entry := uintptr(0x140200000)
	manager := uintptr(0x147000000)
	code, err := buildTaskRewardMultiplierCave(cave, entry+taskRewardMultiplierHookSize, manager, 16, 23)
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != taskRewardMultiplierCaveSize || !bytes.Equal(code[taskRewardMultiplierMarkerOffset:taskRewardMultiplierMarkerOffset+8], taskRewardMultiplierMarker[:]) {
		t.Fatalf("invalid cave shape or marker")
	}
	if got := binary.LittleEndian.Uint32(code[taskRewardMultiplierValueOffset:]); got != 16 {
		t.Fatalf("multiplier=%d, want 16", got)
	}
	if got := binary.LittleEndian.Uint64(code[taskRewardMultiplierCounterOffset:]); got != 23 {
		t.Fatalf("counter=%d, want 23", got)
	}
	for _, required := range [][]byte{
		{0x9C, 0x50, 0x53, 0x52, 0x41, 0x51},
		{0x4C, 0x8B, 0x8D, 0x08, 0x01, 0x00, 0x00},
		{0x83, 0x78, taskRewardMultiplierItemTypeOffset, taskRewardMultiplierItemType},
		{0x8B, 0x58, taskRewardMultiplierQuantityOffset},
		{0x85, 0xDB},
		{0x81, 0xFB, 0xE7, 0x03, 0x00, 0x00},
		{0x48, 0x83, 0xC0, taskRewardMultiplierRecordSize},
		{0x41, 0x59, 0x5A, 0x5B, 0x58, 0x9D},
	} {
		if !bytes.Contains(code, required) {
			t.Fatalf("cave does not contain % X", required)
		}
	}
}

func TestTaskRewardMultiplierLeaseBlocksConnectionReplacement(t *testing.T) {
	app := &App{taskRewardMultiplierLease: &taskRewardMultiplierLease{EntryAddr: 1}}
	if !app.hasActiveRuntimeHookLeaseLocked() {
		t.Fatal("task reward multiplier lease must keep the shared process connection alive")
	}
}

func TestTaskRewardMultiplierRejectsUnsupportedValues(t *testing.T) {
	for _, value := range []int{-1, 0, 3, 6, 32} {
		if validTaskRewardMultiplier(value) {
			t.Fatalf("unsupported multiplier %d accepted", value)
		}
		if _, err := buildTaskRewardMultiplierCave(0x140100000, 0x140200000, 0x147000000, value, 0); err == nil {
			t.Fatalf("cave accepted multiplier %d", value)
		}
	}
	for _, value := range []int{1, 2, 4, 8, 16} {
		if !validTaskRewardMultiplier(value) {
			t.Fatalf("supported multiplier %d rejected", value)
		}
	}
}

func TestTaskRewardMultiplierManagerSlotDecodesRIPRelativeEntry(t *testing.T) {
	entry := uintptr(0x140100000)
	manager := uintptr(0x147654321)
	original := []byte{0x48, 0x8B, 0x0D, 0, 0, 0, 0, 0x31, 0xF6, 0x31, 0xD2, 0x45, 0x31, 0xC0}
	binary.LittleEndian.PutUint32(original[3:7], uint32(int32(int64(manager)-int64(entry+7))))
	got, err := taskRewardMultiplierManagerSlot(entry, original)
	if err != nil || got != manager {
		t.Fatalf("manager=0x%X err=%v, want 0x%X", got, err, manager)
	}
}
