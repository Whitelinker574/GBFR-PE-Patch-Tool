package backend

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestNaturalDropSnapshotExistingFileDistinguishesMissingFromReadFailure(t *testing.T) {
	sentinel := errors.New("read blocked")
	readFile := func(path string) ([]byte, error) {
		switch path {
		case "missing":
			return nil, os.ErrNotExist
		case "blocked":
			return nil, sentinel
		default:
			return []byte("original"), nil
		}
	}

	missing, err := naturalDropSnapshotExistingFile("missing", readFile)
	if err != nil || missing != nil {
		t.Fatalf("missing snapshot = %q, %v; want nil, nil", missing, err)
	}
	if _, err := naturalDropSnapshotExistingFile("blocked", readFile); !errors.Is(err, sentinel) {
		t.Fatalf("blocked snapshot error = %v; want %v", err, sentinel)
	}
	existing, err := naturalDropSnapshotExistingFile("existing", readFile)
	if err != nil || string(existing) != "original" {
		t.Fatalf("existing snapshot = %q, %v", existing, err)
	}
}

func TestNaturalDropCleanupBackupPreservesCauseAndCleanupFailure(t *testing.T) {
	cause := errors.New("deployment failed")
	cleanup := errors.New("backup is locked")
	err := naturalDropCleanupBackup("backup", cause, func(string) error { return cleanup })
	if !errors.Is(err, cause) || !errors.Is(err, cleanup) {
		t.Fatalf("cleanup error = %v; want both deployment and cleanup failures", err)
	}
	if err := naturalDropCleanupBackup("backup", cause, func(string) error { return os.ErrNotExist }); !errors.Is(err, cause) {
		t.Fatalf("missing backup changed original failure: %v", err)
	}
}

func localNaturalDropTableDir(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(filepath.Dir(root), "field-extracted", "summon-tables-202", "system", "table")
	if _, err := os.Stat(filepath.Join(dir, "summon.tbl")); err != nil {
		t.Skipf("local 2.0.2 extracted summon tables are unavailable: %v", err)
	}
	return dir
}

func TestNaturalDropBundledTablesNeedNoExternalExtraction(t *testing.T) {
	source, statuses, err := loadNaturalDropTables("", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != len(naturalDropRequiredTables) {
		t.Fatalf("bundled summon statuses=%d, want %d", len(statuses), len(naturalDropRequiredTables))
	}
	for _, status := range statuses {
		if !status.Valid {
			t.Fatalf("bundled summon table failed exact validation: %+v", status)
		}
	}
	catalog, err := buildNaturalDropCatalog(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) < 80 {
		t.Fatalf("bundled summon catalog=%d, want broad 2.0.2 coverage", len(catalog))
	}
	wrightstones, wrightstoneStatuses, err := loadNaturalWrightstoneTables(naturalDropBundledSourceID, true)
	if err != nil {
		t.Fatal(err)
	}
	if wrightstones == nil || len(wrightstoneStatuses) != len(naturalWrightstoneRequiredTables) {
		t.Fatalf("bundled wrightstone tables are incomplete: tables=%v statuses=%d", wrightstones != nil, len(wrightstoneStatuses))
	}
	for _, status := range wrightstoneStatuses {
		if !status.Valid {
			t.Fatalf("bundled wrightstone table failed exact validation: %+v", status)
		}
	}
	itemTables, itemStatuses, err := loadNaturalDropItemTables(naturalDropBundledSourceID, true)
	if err != nil {
		t.Fatal(err)
	}
	if itemTables == nil || len(itemStatuses) != len(naturalDropItemRequiredTables) {
		t.Fatalf("bundled item-drop tables are incomplete: tables=%v statuses=%d", itemTables != nil, len(itemStatuses))
	}
	for _, status := range itemStatuses {
		if !status.Valid {
			t.Fatalf("bundled item-drop table failed exact validation: %+v", status)
		}
	}
	if pool, err := resolveNaturalDropItemRewardPool(itemTables); err != nil || pool != naturalDropDefaultItemPool {
		t.Fatalf("bundled item reward target = 0x%08X, %v", pool, err)
	}
}

func TestNaturalDropItemPatchUsesVerifiedDefaultPool(t *testing.T) {
	tables, _, err := loadNaturalDropItemTables("", true)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := buildNaturalDropItemCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) < 100 {
		t.Fatalf("item catalog=%d, want searchable 2.0.2 coverage", len(catalog))
	}
	original := append([]byte(nil), tables.RewardLots...)
	count, err := tableRowCount(original, rewardLotTableRowSize)
	if err != nil {
		t.Fatal(err)
	}
	existing := make(map[uint32]bool)
	for i := 0; i < count; i++ {
		offset := 8 + i*rewardLotTableRowSize
		if binary.LittleEndian.Uint32(original[offset+8:]) == naturalDropDefaultItemPool {
			existing[binary.LittleEndian.Uint32(original[offset+12:])] = true
		}
	}
	var chosen NaturalDropItemOption
	for _, option := range catalog {
		hash, _ := ParseHashHex(option.Hash)
		if !existing[hash] {
			chosen = option
			break
		}
	}
	if chosen.Hash == "" {
		t.Fatal("catalog contains no item outside the default reward pool")
	}
	patched, err := patchNaturalDropItemTable(tables, []NaturalDropItemSelection{{
		ItemHash: chosen.Hash,
		Quantity: 7,
		Weight:   23456,
	}}, 4)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(tables.RewardLots, original) {
		t.Fatal("item patch mutated the embedded source table")
	}
	patchedCount, err := tableRowCount(patched, rewardLotTableRowSize)
	if err != nil {
		t.Fatal(err)
	}
	if patchedCount != count+1 {
		t.Fatalf("reward lot rows=%d, want %d", patchedCount, count+1)
	}
	chosenHash, _ := ParseHashHex(chosen.Hash)
	found := false
	for i := 0; i < patchedCount; i++ {
		offset := 8 + i*rewardLotTableRowSize
		if binary.LittleEndian.Uint32(patched[offset+8:]) != naturalDropDefaultItemPool ||
			binary.LittleEndian.Uint32(patched[offset+12:]) != chosenHash {
			continue
		}
		found = true
		if got := binary.LittleEndian.Uint32(patched[offset:]); got != 28 {
			t.Fatalf("item quantity=%d, want 28 after 4x multiplier", got)
		}
		if got := binary.LittleEndian.Uint32(patched[offset+48:]); got != 23456 {
			t.Fatalf("item weight=%d, want 23456", got)
		}
		if binary.LittleEndian.Uint32(patched[offset+16:]) != summonInvalidTypeHash ||
			binary.LittleEndian.Uint32(patched[offset+20:]) != summonInvalidTypeHash {
			t.Fatal("generated item row did not retain the item-only sentinels")
		}
	}
	if !found {
		t.Fatalf("generated reward row for %s was not found", chosen.Hash)
	}
}

func TestNaturalDropItemPatchRejectsDuplicateAndInvalidBounds(t *testing.T) {
	tables, _, err := loadNaturalDropItemTables("", true)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := buildNaturalDropItemCatalog()
	if err != nil || len(catalog) == 0 {
		t.Fatalf("catalog: %v", err)
	}
	valid := NaturalDropItemSelection{ItemHash: catalog[0].Hash, Quantity: 1, Weight: 1}
	for name, selections := range map[string][]NaturalDropItemSelection{
		"duplicate":     {valid, valid},
		"zero quantity": {{ItemHash: valid.ItemHash, Quantity: 0, Weight: 1}},
		"zero weight":   {{ItemHash: valid.ItemHash, Quantity: 1, Weight: 0}},
	} {
		if _, err := patchNaturalDropItemTable(tables, selections, 1); err == nil {
			t.Fatalf("%s selection was accepted", name)
		}
	}
	for _, multiplier := range []int{0, 3, 6, 32} {
		if _, err := patchNaturalDropItemTable(tables, []NaturalDropItemSelection{valid}, multiplier); err == nil {
			t.Fatalf("unsupported reward multiplier %d was accepted", multiplier)
		}
	}
	overflow := valid
	overflow.Quantity = 999
	if _, err := patchNaturalDropItemTable(tables, []NaturalDropItemSelection{overflow}, 16); err == nil {
		t.Fatal("quantity exceeding the table limit after multiplication was accepted")
	}
}

func findNaturalDropCatalogOption(t *testing.T, options []NaturalDropSummonOption, typeHash string) NaturalDropSummonOption {
	t.Helper()
	for _, option := range options {
		if strings.EqualFold(option.TypeHash, typeHash) {
			return option
		}
	}
	t.Fatalf("natural-drop catalog is missing %s", typeHash)
	return NaturalDropSummonOption{}
}

func lotWeightFor(t *testing.T, data []byte, pool, value uint32) uint32 {
	t.Helper()
	count, err := tableRowCount(data, summonLotRowSize)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < count; i++ {
		offset := 8 + i*summonLotRowSize
		if binary.LittleEndian.Uint32(data[offset:]) == pool && binary.LittleEndian.Uint32(data[offset+4:]) == value {
			return binary.LittleEndian.Uint32(data[offset+12:])
		}
	}
	t.Fatalf("lot pool 0x%08X does not contain 0x%08X", pool, value)
	return 0
}

func TestNaturalDropRealTablesBuildIndependentNaturalPools(t *testing.T) {
	source, statuses, err := loadNaturalDropTables(localNaturalDropTableDir(t), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 3 {
		t.Fatalf("validated tables=%d, want 3", len(statuses))
	}
	for _, status := range statuses {
		if !status.Valid {
			t.Fatalf("table did not pass exact 2.0.2 validation: %+v", status)
		}
	}
	catalog, err := buildNaturalDropCatalog(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) < 80 {
		t.Fatalf("droppable random summon catalog=%d, want broad 2.0.2 coverage", len(catalog))
	}
	lucilius := findNaturalDropCatalogOption(t, catalog, "0x6E5968FC")
	beelzebub := findNaturalDropCatalogOption(t, catalog, "0xA7EFF558")
	if len(lucilius.MainTraits) < 2 || len(lucilius.SubParams) < 2 || len(beelzebub.MainTraits) < 2 || len(beelzebub.SubParams) < 2 {
		t.Fatal("endgame summon options did not retain their actual natural pools")
	}
	originalSummon := append([]byte(nil), source.Summon...)
	originalLot := append([]byte(nil), source.SummonLot...)
	originalReward := append([]byte(nil), source.RewardSummonLot...)
	selections := []NaturalDropSelection{
		{TypeHash: lucilius.TypeHash, MainTrait: lucilius.MainTraits[1].Hash, SubParam: lucilius.SubParams[0].Hash},
		{TypeHash: beelzebub.TypeHash, MainTrait: beelzebub.MainTraits[0].Hash, SubParam: beelzebub.SubParams[1].Hash},
	}
	patched, affectedPools, err := patchNaturalDropTables(source, selections)
	if err != nil {
		t.Fatal(err)
	}
	if affectedPools == 0 {
		t.Fatal("no natural reward pool was affected")
	}
	if !bytes.Equal(source.Summon, originalSummon) || !bytes.Equal(source.SummonLot, originalLot) || !bytes.Equal(source.RewardSummonLot, originalReward) {
		t.Fatal("patch function mutated user source tables")
	}
	rows, err := naturalDropSummonRows(patched.Summon)
	if err != nil {
		t.Fatal(err)
	}
	luciliusHash, _ := ParseHashHex(lucilius.TypeHash)
	beelzebubHash, _ := ParseHashHex(beelzebub.TypeHash)
	if rows[luciliusHash].EquipPool == rows[beelzebubHash].EquipPool {
		t.Fatalf("shared astral bonus pool was not split: 0x%08X", rows[luciliusHash].EquipPool)
	}
	for _, selection := range selections {
		typeHash, _ := ParseHashHex(selection.TypeHash)
		mainHash, _ := ParseHashHex(selection.MainTrait)
		subHash, _ := ParseHashHex(selection.SubParam)
		row := rows[typeHash]
		if got := lotWeightFor(t, patched.SummonLot, row.SkillPool, mainHash); got != naturalDropForcedWeight {
			t.Fatalf("main trait weight=%d, want %d", got, naturalDropForcedWeight)
		}
		if got := lotWeightFor(t, patched.SummonLot, row.EquipPool, subHash); got != naturalDropForcedWeight {
			t.Fatalf("sub parameter weight=%d, want %d", got, naturalDropForcedWeight)
		}
	}
	originalCount, _ := tableRowCount(originalLot, summonLotRowSize)
	patchedCount, err := tableRowCount(patched.SummonLot, summonLotRowSize)
	if err != nil {
		t.Fatal(err)
	}
	if patchedCount <= originalCount {
		t.Fatalf("summon lot rows=%d, want more than source %d", patchedCount, originalCount)
	}
	rewardCount, _ := tableRowCount(patched.RewardSummonLot, rewardSummonLotRowSize)
	selected := map[uint32]bool{luciliusHash: true, beelzebubHash: true}
	foundForced := map[uint32]bool{}
	for i := 0; i < rewardCount; i++ {
		offset := 8 + i*rewardSummonLotRowSize
		typeHash := binary.LittleEndian.Uint32(patched.RewardSummonLot[offset+4:])
		if selected[typeHash] && binary.LittleEndian.Uint32(patched.RewardSummonLot[offset+8:]) == naturalDropForcedWeight {
			foundForced[typeHash] = true
		}
	}
	if !foundForced[luciliusHash] || !foundForced[beelzebubHash] {
		t.Fatalf("selected summons were not forced inside their existing reward pools: %+v", foundForced)
	}
}

func TestNaturalDropSplitsSharedMainTraitPools(t *testing.T) {
	source, _, err := loadNaturalDropTables(localNaturalDropTableDir(t), true)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := buildNaturalDropCatalog(source)
	if err != nil {
		t.Fatal(err)
	}
	var first, second NaturalDropSummonOption
	for i, left := range catalog {
		if len(left.MainTraits) < 2 || len(left.SubParams) == 0 {
			continue
		}
		for _, right := range catalog[i+1:] {
			if len(right.SubParams) > 0 {
				first, second = left, right
				break
			}
		}
		if first.TypeHash != "" {
			break
		}
	}
	if first.TypeHash == "" {
		t.Fatal("2.0.2 catalog did not expose two configurable summons")
	}
	sharedSource := &naturalDropTables{
		Summon: append([]byte(nil), source.Summon...), SummonLot: source.SummonLot, RewardSummonLot: source.RewardSummonLot,
	}
	sharedRows, err := naturalDropSummonRows(sharedSource.Summon)
	if err != nil {
		t.Fatal(err)
	}
	firstHash, _ := ParseHashHex(first.TypeHash)
	secondHash, _ := ParseHashHex(second.TypeHash)
	binary.LittleEndian.PutUint32(sharedSource.Summon[sharedRows[secondHash].Offset:sharedRows[secondHash].Offset+4], sharedRows[firstHash].SkillPool)
	patched, _, err := patchNaturalDropTables(sharedSource, []NaturalDropSelection{
		{TypeHash: first.TypeHash, MainTrait: first.MainTraits[0].Hash, SubParam: first.SubParams[0].Hash},
		{TypeHash: second.TypeHash, MainTrait: first.MainTraits[1].Hash, SubParam: second.SubParams[0].Hash},
	})
	if err != nil {
		t.Fatal(err)
	}
	patchedRows, err := naturalDropSummonRows(patched.Summon)
	if err != nil {
		t.Fatal(err)
	}
	if patchedRows[firstHash].SkillPool == patchedRows[secondHash].SkillPool {
		t.Fatal("shared main-trait pool was not split per summon")
	}
}

func TestNaturalDropRejectsNon202Tables(t *testing.T) {
	dir := t.TempDir()
	for _, required := range naturalDropRequiredTables {
		if err := os.WriteFile(filepath.Join(dir, required.Name), make([]byte, required.Size), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, statuses, err := loadNaturalDropTables(dir, true)
	if err == nil || !strings.Contains(err.Error(), "不是已验证") {
		t.Fatalf("non-2.0.2 tables were not rejected: %v", err)
	}
	if len(statuses) != 1 || statuses[0].Valid {
		t.Fatalf("invalid source status=%+v", statuses)
	}
}

func TestNaturalDropConflictDetectsExistingExternalTarget(t *testing.T) {
	gameDir := t.TempDir()
	index := &gbfrDataIndex{Codename: "relink"}
	hash := naturalDropGamePathHash(naturalDropSummonTablePaths[0])
	registerGBFRExternalFile(index, hash, 100)
	conflicts := naturalDropConflicts(gameDir, index, false)
	if len(conflicts) != 1 || conflicts[0].File != naturalDropRelativeTarget(naturalDropSummonTablePaths[0]) || conflicts[0].Scope != "summon" {
		t.Fatalf("conflicts=%+v", conflicts)
	}
	if owned := naturalDropConflicts(gameDir, index, true); len(owned) != 0 {
		t.Fatalf("owned deployment reported conflicts: %+v", owned)
	}
}

func TestNaturalDropBuildIndexRegistersAllTables(t *testing.T) {
	base, err := buildGBFRDataIndex(&gbfrDataIndex{
		Codename:            "relink",
		ArchiveFileHashes:   []uint64{naturalDropGamePathHash(naturalDropSummonTablePaths[0]), 0xffffffffffffffff},
		FileToChunkIndexers: []gbfrFileToChunkIndexer{{ChunkEntryIndex: 1}, {ChunkEntryIndex: 2}},
	})
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for index, path := range naturalDropSummonTablePaths {
		files[path] = make([]byte, 100+index)
	}
	patched, err := naturalDropBuildIndex(base, files)
	if err != nil {
		t.Fatal(err)
	}
	index, err := parseGBFRDataIndex(patched)
	if err != nil {
		t.Fatal(err)
	}
	for path, data := range files {
		hash := naturalDropGamePathHash(path)
		at := sort.Search(len(index.ExternalFileHashes), func(i int) bool { return index.ExternalFileHashes[i] >= hash })
		if at >= len(index.ExternalFileHashes) || index.ExternalFileHashes[at] != hash || index.ExternalFileSizes[at] != uint64(len(data)) {
			t.Fatalf("%s was not registered correctly", path)
		}
		archiveAt := sort.Search(len(index.ArchiveFileHashes), func(i int) bool { return index.ArchiveFileHashes[i] >= hash })
		if archiveAt < len(index.ArchiveFileHashes) && index.ArchiveFileHashes[archiveAt] == hash {
			t.Fatalf("%s remained in archive hashes", path)
		}
	}
	if len(index.ArchiveFileHashes) != 1 || len(index.FileToChunkIndexers) != 1 || index.FileToChunkIndexers[0].ChunkEntryIndex != 2 {
		t.Fatalf("archive hash/indexer alignment changed: %+v %+v", index.ArchiveFileHashes, index.FileToChunkIndexers)
	}
}

func TestNaturalDropOwnedInstallRequiresMatchingBackup(t *testing.T) {
	gameDir := t.TempDir()
	backup := []byte("index-backup")
	backupPath := filepath.Join(gameDir, naturalDropBackupName)
	if err := os.WriteFile(backupPath, backup, 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := naturalDropManifest{
		SchemaVersion:     2,
		Owner:             naturalDropModID,
		GameVersion:       naturalDropGameVersion,
		GameExecutableSHA: runtimePatchCatalogGameSHA256,
		OriginalIndexSHA:  fileSHA256(backup),
		GeneratedFiles:    map[string]string{},
	}
	data, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(gameDir, naturalDropManifestName), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, owned := naturalDropOwnedInstall(gameDir); owned {
		t.Fatal("an arbitrary backup that is not a valid data.i was accepted")
	}
	validBackup, err := buildGBFRDataIndex(&gbfrDataIndex{Codename: "relink"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backupPath, validBackup, 0o644); err != nil {
		t.Fatal(err)
	}
	manifest.OriginalIndexSHA = fileSHA256(validBackup)
	data, _ = json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(gameDir, naturalDropManifestName), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, owned := naturalDropOwnedInstall(gameDir); !owned {
		t.Fatal("valid tool manifest and backup were not recognized")
	}
}

func naturalDropTestIndex(t *testing.T, external map[string][]byte) []byte {
	t.Helper()
	index := &gbfrDataIndex{Codename: "relink"}
	for path, data := range external {
		registerGBFRExternalFile(index, naturalDropGamePathHash(path), uint64(len(data)))
	}
	result, err := buildGBFRDataIndex(index)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestNaturalDropPreparedTransactionRecoversForcedInterruption(t *testing.T) {
	gameDir := t.TempDir()
	indexPath := filepath.Join(gameDir, "data.i")
	beforeIndex := naturalDropTestIndex(t, nil)
	afterIndex := naturalDropTestIndex(t, map[string][]byte{
		naturalDropSummonTablePaths[0]: []byte("new summon"),
		naturalDropSummonTablePaths[1]: []byte("new summon lot"),
	})
	if err := os.WriteFile(indexPath, beforeIndex, 0o644); err != nil {
		t.Fatal(err)
	}
	existingRelative := naturalDropRelativeTarget(naturalDropSummonTablePaths[0])
	newRelative := naturalDropRelativeTarget(naturalDropSummonTablePaths[1])
	existingPath := filepath.Join(gameDir, filepath.FromSlash(existingRelative))
	if err := os.MkdirAll(filepath.Dir(existingPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existingPath, []byte("before summon"), 0o644); err != nil {
		t.Fatal(err)
	}
	beforeFiles := map[string][]byte{
		existingRelative: []byte("before summon"),
		newRelative:      nil,
	}
	afterFiles := map[string][]byte{
		existingRelative: []byte("new summon"),
		newRelative:      []byte("new summon lot"),
	}
	if err := naturalDropBeginPreparedTransaction(gameDir, beforeIndex, afterIndex, beforeFiles, nil, afterFiles, true, nil); err != nil {
		t.Fatal(err)
	}
	if err := naturalDropCreateBackup(filepath.Join(gameDir, naturalDropBackupName), beforeIndex); err != nil {
		t.Fatal(err)
	}
	if err := naturalDropWriteAtomic(existingPath, afterFiles[existingRelative]); err != nil {
		t.Fatal(err)
	}

	if err := naturalDropRecoverPreparedTransaction(gameDir); err != nil {
		t.Fatal(err)
	}
	recoveredIndex, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(recoveredIndex, beforeIndex) {
		t.Fatal("forced-interruption recovery changed the pre-deployment data.i")
	}
	recoveredExisting, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(recoveredExisting) != "before summon" {
		t.Fatalf("existing generated target = %q; want pre-deployment content", recoveredExisting)
	}
	if _, err := os.Stat(filepath.Join(gameDir, filepath.FromSlash(newRelative))); !os.IsNotExist(err) {
		t.Fatalf("unwritten target was touched during recovery: %v", err)
	}
	for _, path := range []string{
		filepath.Join(gameDir, naturalDropPrepareJournalName),
		filepath.Join(gameDir, naturalDropPrepareDirectoryName),
		filepath.Join(gameDir, naturalDropBackupName),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("transaction artifact was not removed: %s: %v", path, err)
		}
	}
}

func TestNaturalDropTransactionLeaseIsExclusiveAcrossAppInstances(t *testing.T) {
	gameDir := t.TempDir()
	releaseFirst, err := acquireNaturalDropTransactionLease(gameDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := acquireNaturalDropTransactionLease(gameDir); err == nil {
		releaseFirst()
		t.Fatal("second app instance acquired the same natural-drop transaction")
	}
	releaseFirst()
	if _, err := os.Stat(filepath.Join(gameDir, naturalDropTransactionLockName)); !os.IsNotExist(err) {
		t.Fatalf("delete-on-close transaction lock survived release: %v", err)
	}
	releaseSecond, err := acquireNaturalDropTransactionLease(gameDir)
	if err != nil {
		t.Fatalf("transaction lock could not be reacquired after release: %v", err)
	}
	releaseSecond()
}

func TestNaturalDropPreparedTransactionRecoversTruncatedBackupArtifacts(t *testing.T) {
	for _, tc := range []struct {
		artifact string
		wantErr  bool
	}{
		{artifact: naturalDropBackupName, wantErr: true},
		{artifact: naturalDropBackupPartialName},
	} {
		t.Run(tc.artifact, func(t *testing.T) {
			gameDir := t.TempDir()
			beforeIndex := naturalDropTestIndex(t, nil)
			afterIndex := naturalDropTestIndex(t, map[string][]byte{
				naturalDropItemTablePaths[0]: []byte("new rewards"),
			})
			relative := naturalDropRelativeTarget(naturalDropItemTablePaths[0])
			if err := os.WriteFile(filepath.Join(gameDir, "data.i"), beforeIndex, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := naturalDropBeginPreparedTransaction(
				gameDir,
				beforeIndex,
				afterIndex,
				map[string][]byte{relative: nil},
				nil,
				map[string][]byte{relative: []byte("new rewards")},
				true,
				nil,
			); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(gameDir, tc.artifact), []byte("truncated"), 0o644); err != nil {
				t.Fatal(err)
			}
			err := naturalDropRecoverPreparedTransaction(gameDir)
			if tc.wantErr {
				if err == nil {
					t.Fatal("unknown final backup contents were deleted instead of failing closed")
				}
				if _, statErr := os.Stat(filepath.Join(gameDir, naturalDropBackupName)); statErr != nil {
					t.Fatalf("rejected final backup was not preserved: %v", statErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("truncated tool backup blocked recovery: %v", err)
			}
			for _, name := range []string{
				naturalDropBackupName,
				naturalDropBackupPartialName,
				naturalDropPrepareJournalName,
				naturalDropPrepareDirectoryName,
			} {
				if _, err := os.Stat(filepath.Join(gameDir, name)); !os.IsNotExist(err) {
					t.Fatalf("recovery left %s behind: %v", name, err)
				}
			}
			recovered, err := os.ReadFile(filepath.Join(gameDir, "data.i"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(recovered, beforeIndex) {
				t.Fatal("truncated backup recovery changed the pre-deployment data.i")
			}
		})
	}
}

func TestNaturalDropPreparedRestoreRecoversAfterManifestAndBackupDeletion(t *testing.T) {
	gameDir := t.TempDir()
	originalIndex := naturalDropTestIndex(t, nil)
	generatedBody := []byte("deployed rewards")
	deployedIndex := naturalDropTestIndex(t, map[string][]byte{
		naturalDropItemTablePaths[0]: generatedBody,
	})
	relative := naturalDropRelativeTarget(naturalDropItemTablePaths[0])
	target := filepath.Join(gameDir, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gameDir, "data.i"), deployedIndex, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, generatedBody, 0o644); err != nil {
		t.Fatal(err)
	}
	manifestBody := []byte("{\"owner\":\"test\"}\n")
	if err := os.WriteFile(filepath.Join(gameDir, naturalDropManifestName), manifestBody, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gameDir, naturalDropBackupName), originalIndex, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := naturalDropBeginPreparedTransaction(
		gameDir,
		deployedIndex,
		originalIndex,
		map[string][]byte{relative: generatedBody},
		manifestBody,
		map[string][]byte{relative: nil},
		false,
		nil,
	); err != nil {
		t.Fatal(err)
	}
	if err := naturalDropAddPreparedBackupTransition(gameDir, originalIndex, nil); err != nil {
		t.Fatal(err)
	}
	if err := naturalDropWriteAtomic(filepath.Join(gameDir, "data.i"), originalIndex); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		target,
		filepath.Join(gameDir, naturalDropManifestName),
		filepath.Join(gameDir, naturalDropBackupName),
	} {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}

	if err := naturalDropRecoverPreparedTransaction(gameDir); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string][]byte{
		filepath.Join(gameDir, "data.i"): deployedIndex,
		target:                           generatedBody,
		filepath.Join(gameDir, naturalDropManifestName): manifestBody,
		filepath.Join(gameDir, naturalDropBackupName):   originalIndex,
	} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read restored %s: %v", path, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("restored %s differs from its pre-restore snapshot", path)
		}
	}
}

func TestAppStartupInvokesNaturalDropRecoveryBeforeRuntimeServices(t *testing.T) {
	source, err := os.ReadFile("app.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(source)
	start := strings.Index(body, "func (a *App) startup")
	end := strings.Index(body[start:], "\nfunc (a *App) beforeClose")
	if start < 0 || end < 0 {
		t.Fatal("App.startup source block was not found")
	}
	block := body[start : start+end]
	recoverAt := strings.Index(block, "recoverNaturalDropTransactionsAtStartup")
	watcherAt := strings.Index(block, "startRuntimeEmergencyWatcher")
	if recoverAt < 0 || watcherAt < 0 || recoverAt > watcherAt {
		t.Fatal("startup must recover prepared natural-drop transactions before starting runtime services")
	}
}

func TestNaturalDropStartupRecoveryErrorRemainsVisibleUntilCleared(t *testing.T) {
	app := NewApp()
	app.setNaturalDropStartupRecoveryError(errors.New("test recovery conflict"))
	status := app.GetNaturalDropStartupRecoveryStatus()
	if !status.Blocked {
		t.Fatal("startup recovery failure was not exposed as a blocking global status")
	}
	if !strings.Contains(status.Detail, "test recovery conflict") ||
		!strings.Contains(status.Detail, "不要启动游戏") {
		t.Fatalf("startup recovery status is not actionable: %q", status.Detail)
	}
	app.clearNaturalDropStartupRecoveryError()
	status = app.GetNaturalDropStartupRecoveryStatus()
	if status.Blocked || status.Detail != "" {
		t.Fatalf("successful recovery did not clear global status: %#v", status)
	}
}

func TestNaturalDropPreparedTransactionRejectsJournalMovedToAnotherGame(t *testing.T) {
	sourceDir := t.TempDir()
	otherDir := t.TempDir()
	beforeIndex := naturalDropTestIndex(t, nil)
	afterIndex := naturalDropTestIndex(t, map[string][]byte{
		naturalDropItemTablePaths[0]: []byte("new rewards"),
	})
	relative := naturalDropRelativeTarget(naturalDropItemTablePaths[0])
	afterFiles := map[string][]byte{relative: []byte("new rewards")}
	if err := os.WriteFile(filepath.Join(sourceDir, "data.i"), beforeIndex, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := naturalDropBeginPreparedTransaction(sourceDir, beforeIndex, afterIndex, map[string][]byte{relative: nil}, nil, afterFiles, true, nil); err != nil {
		t.Fatal(err)
	}
	journal, err := os.ReadFile(filepath.Join(sourceDir, naturalDropPrepareJournalName))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherDir, "data.i"), beforeIndex, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherDir, naturalDropPrepareJournalName), journal, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := naturalDropRecoverPreparedTransaction(otherDir); err == nil {
		t.Fatal("journal copied from another game directory was accepted")
	}
	otherIndex, err := os.ReadFile(filepath.Join(otherDir, "data.i"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(otherIndex, beforeIndex) {
		t.Fatal("rejected orphan journal modified the unrelated data.i")
	}
	if _, err := os.Stat(filepath.Join(otherDir, filepath.FromSlash(relative))); !os.IsNotExist(err) {
		t.Fatalf("rejected orphan journal touched an unrelated generated path: %v", err)
	}
}

func TestNaturalDropPreparedTransactionRejectsOutOfScopeGeneratedFile(t *testing.T) {
	gameDir := t.TempDir()
	beforeIndex := naturalDropTestIndex(t, nil)
	afterIndex := naturalDropTestIndex(t, map[string][]byte{
		naturalDropItemTablePaths[0]: []byte("new rewards"),
	})
	relative := naturalDropRelativeTarget(naturalDropItemTablePaths[0])
	if err := os.WriteFile(filepath.Join(gameDir, "data.i"), beforeIndex, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := naturalDropBeginPreparedTransaction(
		gameDir,
		beforeIndex,
		afterIndex,
		map[string][]byte{relative: nil},
		nil,
		map[string][]byte{relative: []byte("new rewards")},
		true,
		nil,
	); err != nil {
		t.Fatal(err)
	}
	journalPath := filepath.Join(gameDir, naturalDropPrepareJournalName)
	data, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	var journal naturalDropPrepareJournal
	if err := json.Unmarshal(data, &journal); err != nil {
		t.Fatal(err)
	}
	journal.GeneratedFiles["../unrelated.txt"] = naturalDropPreparedFile{
		AfterPresent: true,
		AfterSHA:     fileSHA256([]byte("transaction content")),
	}
	data, err = json.Marshal(journal)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(journalPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	unrelatedPath := filepath.Join(filepath.Dir(gameDir), "unrelated.txt")
	if err := os.WriteFile(unrelatedPath, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(unrelatedPath) })

	if err := naturalDropRecoverPreparedTransaction(gameDir); err == nil {
		t.Fatal("journal containing an out-of-scope generated file was accepted")
	}
	unrelated, err := os.ReadFile(unrelatedPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(unrelated) != "keep me" {
		t.Fatalf("out-of-scope file = %q; recovery must not touch it", unrelated)
	}
	currentIndex, err := os.ReadFile(filepath.Join(gameDir, "data.i"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(currentIndex, beforeIndex) {
		t.Fatal("rejected out-of-scope journal modified data.i")
	}
}

func TestNaturalDropPreparedTransactionRecoversWithoutManifest(t *testing.T) {
	gameDir := t.TempDir()
	beforeIndex := naturalDropTestIndex(t, nil)
	afterIndex := naturalDropTestIndex(t, map[string][]byte{
		naturalDropItemTablePaths[0]: []byte("deployed rewards"),
	})
	relative := naturalDropRelativeTarget(naturalDropItemTablePaths[0])
	afterFiles := map[string][]byte{relative: []byte("deployed rewards")}
	if err := os.WriteFile(filepath.Join(gameDir, "data.i"), beforeIndex, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := naturalDropBeginPreparedTransaction(gameDir, beforeIndex, afterIndex, map[string][]byte{relative: nil}, nil, afterFiles, true, []byte("{\"future\":\"manifest\"}\n")); err != nil {
		t.Fatal(err)
	}
	if err := naturalDropCreateBackup(filepath.Join(gameDir, naturalDropBackupName), beforeIndex); err != nil {
		t.Fatal(err)
	}
	if err := naturalDropWriteAtomic(filepath.Join(gameDir, filepath.FromSlash(relative)), afterFiles[relative]); err != nil {
		t.Fatal(err)
	}
	if err := naturalDropWriteAtomic(filepath.Join(gameDir, "data.i"), afterIndex); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(gameDir, naturalDropManifestName)); !os.IsNotExist(err) {
		t.Fatalf("test setup unexpectedly wrote a manifest: %v", err)
	}

	if err := naturalDropRecoverPreparedTransaction(gameDir); err != nil {
		t.Fatal(err)
	}
	recovered, err := os.ReadFile(filepath.Join(gameDir, "data.i"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(recovered, beforeIndex) {
		t.Fatal("manifest-less recovery did not restore the pre-deployment data.i")
	}
	if _, err := os.Stat(filepath.Join(gameDir, filepath.FromSlash(relative))); !os.IsNotExist(err) {
		t.Fatalf("manifest-less recovery left a generated file behind: %v", err)
	}
	if _, err := os.Stat(filepath.Join(gameDir, naturalDropBackupName)); !os.IsNotExist(err) {
		t.Fatalf("manifest-less initial deployment left an orphan backup: %v", err)
	}
}

func TestNaturalDropCompletedJournalNeverRequiresDeletedSnapshots(t *testing.T) {
	gameDir := t.TempDir()
	beforeIndex := naturalDropTestIndex(t, nil)
	afterIndex := naturalDropTestIndex(t, map[string][]byte{
		naturalDropItemTablePaths[0]: []byte("deployed rewards"),
	})
	relative := naturalDropRelativeTarget(naturalDropItemTablePaths[0])
	if err := os.WriteFile(filepath.Join(gameDir, "data.i"), beforeIndex, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := naturalDropBeginPreparedTransaction(
		gameDir,
		beforeIndex,
		afterIndex,
		map[string][]byte{relative: nil},
		nil,
		map[string][]byte{relative: []byte("deployed rewards")},
		true,
		[]byte("{\"committed\":true}\n"),
	); err != nil {
		t.Fatal(err)
	}

	journalPath := filepath.Join(gameDir, naturalDropPrepareJournalName)
	completedPath := filepath.Join(gameDir, naturalDropCompletedJournalName)
	from, err := windows.UTF16PtrFromString(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	to, err := windows.UTF16PtrFromString(completedPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(gameDir, naturalDropPrepareDirectoryName)); err != nil {
		t.Fatal(err)
	}

	if err := naturalDropRecoverPreparedTransaction(gameDir); err != nil {
		t.Fatalf("completed transaction residue was treated as interrupted: %v", err)
	}
	if _, err := os.Stat(journalPath); !os.IsNotExist(err) {
		t.Fatalf("completed transaction recreated an active journal: %v", err)
	}

	if err := naturalDropBeginPreparedTransaction(
		gameDir,
		beforeIndex,
		afterIndex,
		map[string][]byte{relative: nil},
		nil,
		map[string][]byte{relative: []byte("deployed rewards")},
		true,
		nil,
	); err != nil {
		t.Fatalf("completed marker blocked the next transaction: %v", err)
	}
	if _, err := os.Stat(completedPath); !os.IsNotExist(err) {
		t.Fatalf("next transaction retained stale completed marker: %v", err)
	}
}

func copyNaturalDropTestFile(t *testing.T, source, target string) {
	t.Helper()
	input, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := os.Create(target)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNaturalDropDeployAndRestoreLocalGameCopy(t *testing.T) {
	exeSource := os.Getenv("GBFR_GAME_EXE_TEST")
	indexSource := os.Getenv("GBFR_DATA_INDEX_TEST")
	if exeSource == "" || indexSource == "" {
		t.Skip("set GBFR_GAME_EXE_TEST and GBFR_DATA_INDEX_TEST for the local deployment transaction test")
	}
	if _, err := findProcessByName(charaProcessName); err == nil {
		t.Skip("close the game before running the natural-drop deployment transaction test")
	}
	gameDir := t.TempDir()
	exePath := filepath.Join(gameDir, gameExeName)
	indexPath := filepath.Join(gameDir, "data.i")
	copyNaturalDropTestFile(t, exeSource, exePath)
	copyNaturalDropTestFile(t, indexSource, indexPath)
	originalIndex, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	sourceDir := localNaturalWrightstoneTableDir(t)
	tables, _, err := loadNaturalDropTables(sourceDir, true)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := buildNaturalDropCatalog(tables)
	if err != nil || len(catalog) == 0 {
		t.Fatalf("catalog: %v", err)
	}
	option := catalog[0]
	wrightstoneCatalog, err := buildNaturalWrightstoneCatalog()
	if err != nil || len(wrightstoneCatalog) == 0 || len(wrightstoneCatalog[0].SubTraits) < 2 {
		t.Fatalf("wrightstone catalog: %v (%d records)", err, len(wrightstoneCatalog))
	}
	wrightstone := wrightstoneCatalog[0]
	request := NaturalDropDeployRequest{
		SourceDir:   sourceDir,
		GameExePath: exePath,
		Selections: []NaturalDropSelection{{
			TypeHash:  option.TypeHash,
			MainTrait: option.MainTraits[0].Hash,
			SubParam:  option.SubParams[0].Hash,
		}},
		Wrightstones: []NaturalDropWrightstoneSelection{{
			MainTrait: wrightstone.MainTrait.Hash,
			SubTrait1: wrightstone.SubTraits[0].Hash,
			SubTrait2: wrightstone.SubTraits[1].Hash,
		}},
		WrightstoneOnly: true,
	}
	app := &App{}
	result, err := app.DeployNaturalDropMod(request)
	if err != nil {
		t.Fatal(err)
	}
	if result.SelectedSummons != 1 || result.SelectedWrightstones != 1 || len(result.GeneratedFiles) != 8 {
		t.Fatalf("deploy result=%+v", result)
	}
	workspace, err := app.GetNaturalDropWorkspace(sourceDir, exePath)
	if err != nil {
		t.Fatal(err)
	}
	if !workspace.Owned || !workspace.Installed || !workspace.BackupReady {
		t.Fatalf("deployed workspace=%+v", workspace)
	}

	request.Selections = nil
	result, err = app.DeployNaturalDropMod(request)
	if err != nil {
		t.Fatal(err)
	}
	if result.SelectedSummons != 0 || result.SelectedWrightstones != 1 || len(result.GeneratedFiles) != 5 {
		t.Fatalf("wrightstone-only redeploy result=%+v", result)
	}
	redeployedIndexData, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	redeployedIndex, err := parseGBFRDataIndex(redeployedIndexData)
	if err != nil {
		t.Fatal(err)
	}
	for _, gamePath := range naturalDropSummonTablePaths {
		hash := naturalDropGamePathHash(gamePath)
		at := sort.Search(len(redeployedIndex.ExternalFileHashes), func(i int) bool { return redeployedIndex.ExternalFileHashes[i] >= hash })
		if at < len(redeployedIndex.ExternalFileHashes) && redeployedIndex.ExternalFileHashes[at] == hash {
			t.Fatalf("stale summon table remains registered after redeploy: %s", gamePath)
		}
		if _, err := os.Stat(naturalDropTargetPath(gameDir, gamePath)); !os.IsNotExist(err) {
			t.Fatalf("stale summon table was not removed after redeploy: %s", gamePath)
		}
	}
	for _, gamePath := range naturalDropWrightstoneTablePaths {
		if _, err := os.Stat(naturalDropTargetPath(gameDir, gamePath)); err != nil {
			t.Fatalf("wrightstone table is missing after redeploy: %s: %v", gamePath, err)
		}
	}
	if err := app.RestoreNaturalDropDefaults(NaturalDropRestoreRequest{GameExePath: exePath}); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, originalIndex) {
		t.Fatal("restored data.i differs from the original copy")
	}
	for _, path := range append(append([]string(nil), naturalDropSummonTablePaths...), naturalDropWrightstoneTablePaths...) {
		if _, err := os.Stat(naturalDropTargetPath(gameDir, path)); !os.IsNotExist(err) {
			t.Fatalf("generated table was not removed: %s", path)
		}
	}
	for _, name := range []string{naturalDropBackupName, naturalDropManifestName} {
		if _, err := os.Stat(filepath.Join(gameDir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s was not removed", name)
		}
	}
}
