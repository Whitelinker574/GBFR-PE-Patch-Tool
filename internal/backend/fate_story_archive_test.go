package backend

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestFateStoryArchiveCatalogIsCompleteAndUnique(t *testing.T) {
	if len(fateStoryArchiveCatalog) != fateStoryArchiveCount {
		t.Fatalf("catalog=%d, want %d", len(fateStoryArchiveCatalog), fateStoryArchiveCount)
	}
	ids := make(map[string]struct{}, fateStoryArchiveCount)
	indexes := make(map[int]struct{}, fateStoryArchiveCount)
	for offset, row := range fateStoryArchiveCatalog {
		if row.archiveID != fmt.Sprintf("ARC_OTHER_%03d", 40+offset) || row.vectorIndex != 52+offset {
			t.Fatalf("unexpected catalog row %d: %+v", offset, row)
		}
		if _, duplicate := ids[row.archiveID]; duplicate {
			t.Fatalf("duplicate archive ID %s", row.archiveID)
		}
		if _, duplicate := indexes[row.vectorIndex]; duplicate {
			t.Fatalf("duplicate archive vector index %d", row.vectorIndex)
		}
		ids[row.archiveID] = struct{}{}
		indexes[row.vectorIndex] = struct{}{}
	}
}

func prepareFateStoryArchiveFixture(t *testing.T) (string, []byte) {
	t.Helper()
	source := filepath.Join(defaultSaveGamesDir(), "SaveData3.dat")
	sourceData, err := os.ReadFile(source)
	if err != nil {
		t.Skipf("real SaveData3 fixture is unavailable: %v", err)
	}
	work := filepath.Join(t.TempDir(), "FateStoryArchiveFixture.dat")
	if err := os.WriteFile(work, sourceData, 0o644); err != nil {
		t.Fatal(err)
	}
	save, err := LoadSave(work)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := requireFateStoryArchiveVector(save)
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range []int{52, 55, 68} {
		entry.Bytes()[index] = 0
	}
	if err := save.FixChecksums(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(work, save.data, 0o644); err != nil {
		t.Fatal(err)
	}
	return work, sourceData
}

func TestFateStoryArchiveInspectFindsOnlyCompletedMissingArchives(t *testing.T) {
	work, _ := prepareFateStoryArchiveFixture(t)
	snapshot, err := (&App{}).FateEpisodeEditableInspect(work)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.StoryArchives) != fateStoryArchiveCount {
		t.Fatalf("story archives=%d, want %d", len(snapshot.StoryArchives), fateStoryArchiveCount)
	}
	missing := make(map[string]bool)
	for _, archive := range snapshot.StoryArchives {
		if archive.Missing {
			missing[archive.ArchiveID] = true
		}
	}
	for _, id := range []string{"ARC_OTHER_040", "ARC_OTHER_043", "ARC_OTHER_056"} {
		if !missing[id] {
			t.Fatalf("expected completed archive %s to be reported missing: %+v", id, snapshot.StoryArchives)
		}
	}
}

func TestRepairFateStoryArchivesBacksUpAndReadsBackWithoutChangingFate(t *testing.T) {
	work, sourceData := prepareFateStoryArchiveFixture(t)
	app := &App{}
	beforeSnapshot, err := app.FateEpisodeEditableInspect(work)
	if err != nil {
		t.Fatal(err)
	}
	beforeSave, err := LoadSave(work)
	if err != nil {
		t.Fatal(err)
	}
	beforeLayout, err := inspectFateEpisodeLayout(beforeSave)
	if err != nil {
		t.Fatal(err)
	}
	result, err := app.RepairFateStoryArchives(FateStoryArchiveRepairRequest{
		Path: work, ExpectedRevision: beforeSnapshot.Revision,
		ArchiveIDs: []string{"ARC_OTHER_040", "ARC_OTHER_043", "ARC_OTHER_056"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed != 3 || result.Verified != 3 || result.BackupPath == "" {
		t.Fatalf("unexpected repair result: %+v", result)
	}
	if _, err := os.Stat(result.BackupPath); err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	afterSave, err := LoadSave(work)
	if err != nil {
		t.Fatal(err)
	}
	afterVector, err := requireFateStoryArchiveVector(afterSave)
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range []int{52, 55, 68} {
		if afterVector.Bytes()[index] != 1 {
			t.Fatalf("archive vector index %d was not repaired", index)
		}
	}
	afterLayout, err := inspectFateEpisodeLayout(afterSave)
	if err != nil {
		t.Fatal(err)
	}
	if !equalUint32Map(beforeLayout.auxiliaryStates, afterLayout.auxiliaryStates) ||
		!equalUint32Map(beforeLayout.placeholder, afterLayout.placeholder) ||
		beforeLayout.status.Completed != afterLayout.status.Completed ||
		beforeLayout.status.MissionCompleted != afterLayout.status.MissionCompleted {
		t.Fatal("archive repair changed Fate Episode state")
	}
	originalAfter, err := os.ReadFile(filepath.Join(defaultSaveGamesDir(), "SaveData3.dat"))
	if err == nil && !bytes.Equal(originalAfter, sourceData) {
		t.Fatal("real source save was modified")
	}
}

func TestRepairFateStoryArchivesRejectsUnfinishedOrUnknownArchive(t *testing.T) {
	work, _ := prepareFateStoryArchiveFixture(t)
	snapshot, err := (&App{}).FateEpisodeEditableInspect(work)
	if err != nil {
		t.Fatal(err)
	}
	_, err = (&App{}).RepairFateStoryArchives(FateStoryArchiveRepairRequest{
		Path: work, ExpectedRevision: snapshot.Revision, ArchiveIDs: []string{"ARC_OTHER_999"},
	})
	if err == nil {
		t.Fatal("unknown archive must be rejected")
	}
}

func TestRepairFateStoryArchivesRejectsStaleRevisionWithoutTouchingFile(t *testing.T) {
	work, _ := prepareFateStoryArchiveFixture(t)
	before, err := os.ReadFile(work)
	if err != nil {
		t.Fatal(err)
	}
	_, err = (&App{}).RepairFateStoryArchives(FateStoryArchiveRepairRequest{
		Path: work, ExpectedRevision: fateEpisodeRevision([]byte("stale")), ArchiveIDs: []string{"ARC_OTHER_040"},
	})
	if err == nil {
		t.Fatal("stale revision must be rejected")
	}
	after, err := os.ReadFile(work)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("stale request changed the save")
	}
}

func TestRepairFateStoryArchivesReadbackFailureRestoresOriginalCopy(t *testing.T) {
	work, _ := prepareFateStoryArchiveFixture(t)
	snapshot, err := (&App{}).FateEpisodeEditableInspect(work)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(work)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repairFateStoryArchivesLocked(FateStoryArchiveRepairRequest{
		Path: work, ExpectedRevision: snapshot.Revision, ArchiveIDs: []string{"ARC_OTHER_040"},
	}, func(string) (*SaveData, error) { return nil, errors.New("forced readback failure") })
	if err == nil {
		t.Fatal("forced readback failure must be returned")
	}
	after, readErr := os.ReadFile(work)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("failed archive repair did not restore the original save")
	}
}
