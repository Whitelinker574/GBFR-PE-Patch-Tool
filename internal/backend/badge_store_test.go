package backend

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func badgeTestSaveCopy(t *testing.T) string {
	t.Helper()
	if !haveSave(testLoadoutSave) {
		t.Skipf("测试存档不存在: %s", testLoadoutSave)
	}
	raw, err := os.ReadFile(testLoadoutSave)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "SaveData2.dat")
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBadgeVectorValidationRejectsWrongLengthAndNonBooleanData(t *testing.T) {
	path := badgeTestSaveCopy(t)
	save, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	entry := requireBadgeVectorForTest(t, save, SaveID_BadgeUnlocked)
	binary.LittleEndian.PutUint32(save.data[entry.ValueOff-4:], 1699)
	if _, err := requireBadgeVector(save, SaveID_BadgeUnlocked, "称号解锁"); err == nil {
		t.Fatal("badge vector with wrong length must fail closed")
	}

	save, err = LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	entry = requireBadgeVectorForTest(t, save, SaveID_BadgeUnlocked)
	entry.Bytes()[0] = 2
	if _, err := requireBadgeVector(save, SaveID_BadgeUnlocked, "称号解锁"); err == nil {
		t.Fatal("badge vector with non-boolean byte must fail closed")
	}
}

func requireBadgeVectorForTest(t *testing.T, save *SaveData, idType uint32) *unitEntry {
	t.Helper()
	matches := make([]*unitEntry, 0, 1)
	for _, entry := range save.findAllUnitsByType(idType) {
		if entry.UnitID == 0 {
			matches = append(matches, entry)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("badge vector %d exact matches=%d, want 1", idType, len(matches))
	}
	if matches[0].ValueCnt != 1700 || len(matches[0].Bytes()) != 1700 {
		t.Fatalf("badge vector %d length=%d/%d, want 1700", idType, matches[0].ValueCnt, len(matches[0].Bytes()))
	}
	return matches[0]
}

func TestBadgeStateReadsLockedCatalogAndThreeRealSaveVectors(t *testing.T) {
	path := badgeTestSaveCopy(t)
	state, err := (&App{}).LoadBadgeState(path)
	if err != nil {
		t.Fatal(err)
	}
	if state.Total != 1616 || len(state.Badges) != 1616 {
		t.Fatalf("badge state total=%d rows=%d, want 1616", state.Total, len(state.Badges))
	}
	if state.Badges[0].ID != 0 || state.Badges[0].NameZH == "" || state.Badges[0].NameEN == "" {
		t.Fatalf("first localized badge row invalid: %+v", state.Badges[0])
	}
	if state.UnlockedCount < 0 || state.UnlockedCount > state.Total || state.ViewedCount < 0 || state.ViewedCount > state.Total || state.RewardClaimedCount < 0 || state.RewardClaimedCount > state.Total {
		t.Fatalf("invalid badge counters: %+v", state)
	}
}

func TestSetBadgeStateWritesIntentAndViewedButPreservesRewards(t *testing.T) {
	path := badgeTestSaveCopy(t)
	originalFinder := badgeFindProcessByName
	badgeFindProcessByName = func(string) (uint32, error) { return 0, errors.New("not running") }
	t.Cleanup(func() { badgeFindProcessByName = originalFinder })

	before, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	unlockedBefore := append([]byte(nil), requireBadgeVectorForTest(t, before, SaveID_BadgeUnlocked).Bytes()...)
	rewardBefore := append([]byte(nil), requireBadgeVectorForTest(t, before, SaveID_BadgeRewardClaimed).Bytes()...)
	target := 0
	wantUnlocked := unlockedBefore[target] == 0

	result, err := (&App{}).SetBadgeState(path, target, wantUnlocked, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed != 1 || result.Verified != 1 || result.BackupPath == "" {
		t.Fatalf("unexpected write result: %+v", result)
	}

	after, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	unlockedAfter := requireBadgeVectorForTest(t, after, SaveID_BadgeUnlocked).Bytes()
	viewedAfter := requireBadgeVectorForTest(t, after, SaveID_BadgeViewed).Bytes()
	rewardAfter := requireBadgeVectorForTest(t, after, SaveID_BadgeRewardClaimed).Bytes()
	wantByte := byte(0)
	if wantUnlocked {
		wantByte = 1
	}
	if unlockedAfter[target] != wantByte || viewedAfter[target] != 1 {
		t.Fatalf("badge %d writeback unlock/viewed=%d/%d, want %d/1", target, unlockedAfter[target], viewedAfter[target], wantByte)
	}
	if !bytes.Equal(rewardBefore, rewardAfter) {
		t.Fatal("badge reward-claimed vector changed")
	}
}

func TestBadgeWriteRejectsRunningGameBeforeBackupOrMutation(t *testing.T) {
	path := badgeTestSaveCopy(t)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	originalFinder := badgeFindProcessByName
	badgeFindProcessByName = func(string) (uint32, error) { return 1234, nil }
	t.Cleanup(func() { badgeFindProcessByName = originalFinder })

	if _, err := (&App{}).SetBadgeState(path, 0, true, true); err == nil {
		t.Fatal("running game must reject badge save write")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("running-game rejection changed the save")
	}
	backups, err := filepath.Glob(path + ".pre-edit-*.bak")
	if err != nil || len(backups) != 0 {
		t.Fatalf("running-game rejection created backups: %v %v", backups, err)
	}
}

func TestSetAllBadgeStatesOnlyChangesCataloguedUnlockAndOptionalViewedBits(t *testing.T) {
	path := badgeTestSaveCopy(t)
	originalFinder := badgeFindProcessByName
	badgeFindProcessByName = func(string) (uint32, error) { return 0, errors.New("not running") }
	t.Cleanup(func() { badgeFindProcessByName = originalFinder })

	before, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	rewards := append([]byte(nil), requireBadgeVectorForTest(t, before, SaveID_BadgeRewardClaimed).Bytes()...)
	result, err := (&App{}).SetAllBadgeStates(path, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed != 1616 || result.Verified != 1616 {
		t.Fatalf("bulk result=%+v, want 1616 verified catalog records", result)
	}
	state, err := (&App{}).LoadBadgeState(path)
	if err != nil {
		t.Fatal(err)
	}
	if state.UnlockedCount != state.Total || state.ViewedCount != state.Total {
		t.Fatalf("bulk state=%+v, want every catalogued record unlocked and viewed", state)
	}
	after, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rewards, requireBadgeVectorForTest(t, after, SaveID_BadgeRewardClaimed).Bytes()) {
		t.Fatal("bulk unlock changed reward records")
	}
}

func TestConcurrentBadgeWritesAreSerializedWithoutLostUpdate(t *testing.T) {
	path := badgeTestSaveCopy(t)
	originalFinder := badgeFindProcessByName
	badgeFindProcessByName = func(string) (uint32, error) { return 0, errors.New("not running") }
	t.Cleanup(func() { badgeFindProcessByName = originalFinder })
	app := &App{}
	for _, id := range []int{0, 1} {
		if _, err := app.SetBadgeState(path, id, false, false); err != nil {
			t.Fatal(err)
		}
	}

	var wait sync.WaitGroup
	errs := make(chan error, 2)
	for _, id := range []int{0, 1} {
		wait.Add(1)
		go func(id int) {
			defer wait.Done()
			_, err := app.SetBadgeState(path, id, true, true)
			errs <- err
		}(id)
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	state, err := app.LoadBadgeState(path)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[int]BadgeEntry{}
	for _, badge := range state.Badges {
		byID[badge.ID] = badge
	}
	for _, id := range []int{0, 1} {
		if !byID[id].Unlocked || !byID[id].Viewed {
			t.Fatalf("concurrent badge %d update was lost: %+v", id, byID[id])
		}
	}
}

func TestBadgeFeatureOnIsolatedRealSaveCopy(t *testing.T) {
	fixture := requireIsolatedSaveQA(t)
	fixtureDigest := isolatedSaveDigest(t, fixture)
	payload, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "SaveData2.dat")
	if err := os.WriteFile(path, payload, 0644); err != nil {
		t.Fatal(err)
	}
	originalFinder := badgeFindProcessByName
	badgeFindProcessByName = func(string) (uint32, error) { return 0, errors.New("not running") }
	t.Cleanup(func() { badgeFindProcessByName = originalFinder })

	state, err := (&App{}).LoadBadgeState(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Badges) != badgeCatalogCount {
		t.Fatalf("real save badge rows=%d", len(state.Badges))
	}
	target := state.Badges[len(state.Badges)/2]
	if _, err := (&App{}).SetBadgeState(path, target.ID, !target.Unlocked, true); err != nil {
		t.Fatal(err)
	}
	verified, err := (&App{}).LoadBadgeState(path)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Badges[len(state.Badges)/2].Unlocked == target.Unlocked || !verified.Badges[len(state.Badges)/2].Viewed {
		t.Fatalf("real-save-copy toggle not reflected: before=%+v after=%+v", target, verified.Badges[len(state.Badges)/2])
	}
	if got := isolatedSaveDigest(t, fixture); got != fixtureDigest {
		t.Fatal("temp-copy badge write changed the isolated source fixture")
	}
}
