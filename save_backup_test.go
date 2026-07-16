package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func prepareBackupTestEnv(t *testing.T) (string, string) {
	t.Helper()
	base := t.TempDir()
	t.Setenv("LOCALAPPDATA", filepath.Join(base, "local"))
	t.Setenv("APPDATA", filepath.Join(base, "roaming"))
	saveDir := defaultSaveGamesDir()
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := saveSnapshotRoot()
	if err != nil {
		t.Fatal(err)
	}
	return saveDir, root
}

func writeTestSave(t *testing.T, dir string, slot int, data string) string {
	t.Helper()
	path := filepath.Join(dir, fmt.Sprintf("SaveData%d.dat", slot))
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readTestSave(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSaveSnapshotCapturesOnlyExistingManagedSlots(t *testing.T) {
	saveDir, root := prepareBackupTestEnv(t)
	writeTestSave(t, saveDir, 1, "slot-one-before")
	writeTestSave(t, saveDir, 3, "slot-three-before")
	if err := os.WriteFile(filepath.Join(saveDir, "SaveData4.dat"), []byte("ignore"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saveDir, "notes.dat"), []byte("ignore"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	snapshot, err := app.CreateSaveSnapshot("手动安全点")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Slots) != 2 || snapshot.Slots[0].Slot != 1 || snapshot.Slots[1].Slot != 3 {
		t.Fatalf("unexpected slots: %#v", snapshot.Slots)
	}
	if snapshot.DisplayTime == "" || snapshot.Reason != "手动安全点" || snapshot.TotalSize == 0 {
		t.Fatalf("incomplete snapshot: %#v", snapshot)
	}
	for _, slot := range snapshot.Slots {
		if slot.SHA256 == "" {
			t.Fatalf("slot %d missing hash", slot.Slot)
		}
		if _, err := os.Stat(filepath.Join(root, snapshot.ID, slot.FileName)); err != nil {
			t.Fatalf("missing backup %s: %v", slot.FileName, err)
		}
	}
	listed, err := app.ListSaveSnapshots()
	if err != nil || len(listed) != 1 || listed[0].ID != snapshot.ID {
		t.Fatalf("unexpected timeline: %#v, %v", listed, err)
	}
}

func TestRestoreSnapshotCreatesSafetyPointAndKeepsUnlistedSlot(t *testing.T) {
	saveDir, _ := prepareBackupTestEnv(t)
	slot1 := writeTestSave(t, saveDir, 1, "slot-one-before")
	slot3 := writeTestSave(t, saveDir, 3, "slot-three-before")
	app := NewApp()
	snapshot, err := app.CreateSaveSnapshot("修改前")
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(slot1, []byte("slot-one-after"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(slot3, []byte("slot-three-after"), 0o644); err != nil {
		t.Fatal(err)
	}
	slot2 := writeTestSave(t, saveDir, 2, "new-slot-two")

	result, err := app.RestoreSaveSnapshot(snapshot.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Restored != 2 || result.SafetySnapshotID == "" {
		t.Fatalf("unexpected restore result: %#v", result)
	}
	if got := readTestSave(t, slot1); got != "slot-one-before" {
		t.Fatalf("slot 1 not restored: %q", got)
	}
	if got := readTestSave(t, slot3); got != "slot-three-before" {
		t.Fatalf("slot 3 not restored: %q", got)
	}
	if got := readTestSave(t, slot2); got != "new-slot-two" {
		t.Fatalf("unlisted slot 2 should remain untouched: %q", got)
	}
	listed, err := app.ListSaveSnapshots()
	if err != nil || len(listed) != 2 {
		t.Fatalf("expected original and safety snapshots: %#v, %v", listed, err)
	}
	if listed[0].Reason != "恢复前自动备份" {
		t.Fatalf("newest snapshot should be recovery safety point: %#v", listed[0])
	}
}

func TestRestoreRejectsTamperedSnapshot(t *testing.T) {
	saveDir, root := prepareBackupTestEnv(t)
	slot1 := writeTestSave(t, saveDir, 1, "known-good")
	app := NewApp()
	snapshot, err := app.CreateSaveSnapshot("校验测试")
	if err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(root, snapshot.ID, "SaveData1.dat")
	if err := os.WriteFile(backup, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(slot1, []byte("current-save"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = app.RestoreSaveSnapshot(snapshot.ID)
	if err == nil || !strings.Contains(err.Error(), "校验不一致") {
		t.Fatalf("expected checksum error, got %v", err)
	}
	if got := readTestSave(t, slot1); got != "current-save" {
		t.Fatalf("current save changed after rejected restore: %q", got)
	}
}

func TestAutomaticWriteGateSnapshotsSiblingSlots(t *testing.T) {
	saveDir, _ := prepareBackupTestEnv(t)
	writeTestSave(t, saveDir, 1, "one")
	writeTestSave(t, saveDir, 3, "three")

	snapshot, err := autoSnapshotBeforeSaveWrite(filepath.Join(saveDir, "SaveData2.dat"))
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Slots) != 2 || snapshot.Slots[0].Slot != 1 || snapshot.Slots[1].Slot != 3 {
		t.Fatalf("write gate did not protect sibling slots: %#v", snapshot.Slots)
	}
	if !strings.Contains(snapshot.Reason, "SaveData2.dat") {
		t.Fatalf("write gate reason should identify the edited slot: %q", snapshot.Reason)
	}
}

func TestFindSaveFilesUsesRealSlotNumbers(t *testing.T) {
	saveDir, _ := prepareBackupTestEnv(t)
	writeTestSave(t, saveDir, 2, "two")
	writeTestSave(t, saveDir, 3, "three")
	if err := os.WriteFile(filepath.Join(saveDir, "SaveData9.dat"), []byte("ignore"), 0o644); err != nil {
		t.Fatal(err)
	}
	slots := NewApp().FindSaveFiles()
	if len(slots) != 2 || slots[0].Index != 2 || slots[1].Index != 3 {
		t.Fatalf("unexpected save slots: %#v", slots)
	}
}
