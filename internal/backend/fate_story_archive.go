package backend

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

const (
	fateArchiveHashID  uint32 = 7901
	fateArchiveFlagsID uint32 = 7902
)

const fateArchiveRecordCount = 100
const fateStoryArchiveCount = 29

// FateStoryArchiveStatus reports the native hash/flags record. Bit 0 unlocks;
// bit 1 records viewing. Progression boolean vectors are unrelated.
type FateStoryArchiveStatus struct {
	ArchiveID        string   `json:"archiveId"`
	CharacterCodes   []string `json:"characterCodes"`
	FinalEpisodeKeys []string `json:"finalEpisodeKeys"`
	RecordUnitID     int      `json:"recordUnitId"`
	RawFlags         uint32   `json:"rawFlags"`
	Recorded         bool     `json:"recorded"`
	FinalCompleted   bool     `json:"finalCompleted"`
	Unlocked         bool     `json:"unlocked"`
	Missing          bool     `json:"missing"`
}

type fateStoryArchiveDefinition struct {
	archiveID                        string
	characterCodes, finalEpisodeKeys []string
}

var fateStoryArchiveCatalog = makeFateStoryArchiveCatalog()

func makeFateStoryArchiveCatalog() []fateStoryArchiveDefinition {
	// Exact fate_episode.StoryNoteArchiveId1/2 joins from game 2.0.5.
	codes := []string{"PL0000", "PL0200", "PL0300", "PL0400", "PL0500", "PL0600", "PL0700", "PL0800", "PL0900", "PL1000", "PL1100", "PL1200", "PL1300", "PL1400", "PL1500", "PL1800", "PL1800", "PL1900", "PL1600", "PL1700", "PL2200", "PL2300", "PL2100", "PL2600", "PL2700", "PL2800", "PL2900", "PL2400", "PL2500"}
	rows := make([]fateStoryArchiveDefinition, 0, len(codes))
	for i, code := range codes {
		number := 40 + i
		if i >= 18 {
			number += 3
		}
		row := fateStoryArchiveDefinition{archiveID: fmt.Sprintf("ARC_OTHER_%03d", number), characterCodes: []string{code}, finalEpisodeKeys: []string{"FATE_" + code + "_10"}}
		if i == 0 {
			row.characterCodes = append(row.characterCodes, "PL0100")
			row.finalEpisodeKeys = append(row.finalEpisodeKeys, "FATE_PL0100_10")
		}
		rows = append(rows, row)
	}
	return rows
}

type fateArchiveRecord struct{ hash, flags *unitEntry }
type fateArchiveRecords struct {
	byHash map[uint32]fateArchiveRecord
	empty  []fateArchiveRecord
}

func readFateArchiveRecords(save *SaveData) (*fateArchiveRecords, error) {
	ids, err := findUnitsByTypeFast(save, fateArchiveHashID, fateArchiveRecordCount)
	if err != nil {
		return nil, fmt.Errorf("读取档案编号记录失败: %w", err)
	}
	flags, err := findUnitsByTypeFast(save, fateArchiveFlagsID, fateArchiveRecordCount)
	if err != nil {
		return nil, fmt.Errorf("读取档案解锁记录失败: %w", err)
	}
	fm := map[uint32]*unitEntry{}
	for _, e := range flags {
		if e.ValueCnt != 1 || e.UnitID >= fateArchiveRecordCount || fm[e.UnitID] != nil {
			return nil, errors.New("档案解锁记录结构异常或存在重复槽位")
		}
		fm[e.UnitID] = e
	}
	result := &fateArchiveRecords{byHash: map[uint32]fateArchiveRecord{}}
	seen := map[uint32]bool{}
	for _, id := range ids {
		if id.ValueCnt != 1 || id.UnitID >= fateArchiveRecordCount || seen[id.UnitID] || fm[id.UnitID] == nil {
			return nil, errors.New("档案编号与解锁记录无法一一对应")
		}
		seen[id.UnitID] = true
		record := fateArchiveRecord{id, fm[id.UnitID]}
		hash := id.Uint32()
		if hash == EmptyHash {
			if record.flags.Uint32() == 0 {
				result.empty = append(result.empty, record)
			}
			continue
		}
		if _, ok := result.byHash[hash]; ok {
			return nil, fmt.Errorf("档案编号 %08X 重复", hash)
		}
		result.byHash[hash] = record
	}
	sort.Slice(result.empty, func(i, j int) bool { return result.empty[i].hash.UnitID < result.empty[j].hash.UnitID })
	return result, nil
}
func inspectFateStoryArchives(layout *fateEpisodeLayout, records *fateArchiveRecords) ([]FateStoryArchiveStatus, error) {
	rows := make([]FateStoryArchiveStatus, 0, len(fateStoryArchiveCatalog))
	for _, d := range fateStoryArchiveCatalog {
		row := FateStoryArchiveStatus{ArchiveID: d.archiveID, CharacterCodes: d.characterCodes, FinalEpisodeKeys: d.finalEpisodeKeys, RecordUnitID: -1}
		for _, key := range d.finalEpisodeKeys {
			entry := layout.stateByHash[gbfrHash32(key)]
			if entry == nil {
				return nil, fmt.Errorf("档案 %s 缺少对应篇章记录", d.archiveID)
			}
			row.FinalCompleted = row.FinalCompleted || entry.Uint32() == fateCompletedState
		}
		if record, ok := records.byHash[gbfrHash32(d.archiveID)]; ok {
			row.Recorded = true
			row.RecordUnitID = int(record.hash.UnitID)
			row.RawFlags = record.flags.Uint32()
			row.Unlocked = row.RawFlags&1 != 0
		}
		row.Missing = row.FinalCompleted && !row.Unlocked
		rows = append(rows, row)
	}
	return rows, nil
}

type FateStoryArchiveRepairRequest struct {
	Path             string   `json:"path"`
	ExpectedRevision string   `json:"expectedRevision"`
	ArchiveIDs       []string `json:"archiveIds"`
}
type FateStoryArchiveRepairResult struct {
	Path             string                   `json:"path"`
	BackupPath       string                   `json:"backupPath,omitempty"`
	PreviousRevision string                   `json:"previousRevision"`
	Revision         string                   `json:"revision"`
	Requested        int                      `json:"requested"`
	Changed          int                      `json:"changed"`
	Verified         int                      `json:"verified"`
	StoryArchives    []FateStoryArchiveStatus `json:"storyArchives"`
}

func fateStoryArchiveWriteFailure(path string, original, expected []byte, verifyErr error) error {
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("档案补全回读失败: %v；无法重读目标，请保留备份: %w", verifyErr, err)
	}
	if !bytes.Equal(current, expected) {
		return fmt.Errorf("档案补全回读失败: %v；目标随后又被修改，未覆盖新数据，请保留备份", verifyErr)
	}
	if err := rollbackSaveDiffTarget(path, original); err != nil {
		return fmt.Errorf("档案补全回读失败: %v；恢复失败，请保留备份: %w", verifyErr, err)
	}
	return fmt.Errorf("档案补全回读失败，已自动恢复写入前存档: %w", verifyErr)
}
func repairFateStoryArchivesLocked(request FateStoryArchiveRepairRequest, loadForVerification func(string) (*SaveData, error)) (*FateStoryArchiveRepairResult, error) {
	revision, err := validateFateEpisodeRevision(request.ExpectedRevision)
	if err != nil {
		return nil, err
	}
	if len(request.ArchiveIDs) == 0 || len(request.ArchiveIDs) > fateStoryArchiveCount {
		return nil, errors.New("请选择需要补全的命运篇章档案")
	}
	absolute, original, err := readFateEpisodeRaw(request.Path)
	if err != nil {
		return nil, err
	}
	if fateEpisodeRevision(original) != revision {
		return nil, errors.New("目标存档在检查后已被修改，请重新检查后再补全档案")
	}
	save, err := newSaveData(absolute, append([]byte(nil), original...))
	if err != nil {
		return nil, err
	}
	layout, err := inspectFateEpisodeLayout(save)
	if err != nil {
		return nil, err
	}
	records, err := readFateArchiveRecords(save)
	if err != nil {
		return nil, err
	}
	statuses, err := inspectFateStoryArchives(layout, records)
	if err != nil {
		return nil, err
	}
	known := map[string]FateStoryArchiveStatus{}
	for _, s := range statuses {
		known[s.ArchiveID] = s
	}
	expectedFlags := map[uint32]uint32{}
	seen := map[string]bool{}
	changed := 0
	emptyIndex := 0
	for _, rawID := range request.ArchiveIDs {
		id := strings.ToUpper(strings.TrimSpace(rawID))
		status, ok := known[id]
		if !ok || seen[id] {
			return nil, fmt.Errorf("档案 %q 不在补全目录中或被重复提交", rawID)
		}
		seen[id] = true
		if !status.FinalCompleted {
			return nil, fmt.Errorf("档案 %s 对应篇章尚未完成，未写入", id)
		}
		hash := gbfrHash32(id)
		record, exists := records.byHash[hash]
		if !exists {
			if emptyIndex >= len(records.empty) {
				return nil, errors.New("存档没有可用的空档案记录，已取消本次补全")
			}
			record = records.empty[emptyIndex]
			emptyIndex++
			record.hash.SetUint32(hash)
		}
		before := record.flags.Uint32()
		target := before | 1
		expectedFlags[hash] = target
		if !exists || before != target {
			record.flags.SetUint32(target)
			changed++
		}
	}
	result := &FateStoryArchiveRepairResult{Path: absolute, PreviousRevision: revision, Revision: revision, Requested: len(seen), Changed: changed, StoryArchives: statuses}
	if changed == 0 {
		result.Verified = len(seen)
		return result, nil
	}
	if err := save.FixChecksums(); err != nil {
		return nil, err
	}
	_, latest, err := readFateEpisodeRaw(absolute)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(latest, original) {
		return nil, errors.New("准备写入时存档又发生变化，已取消补全")
	}
	expected := append([]byte(nil), save.data...)
	if err := save.Write(absolute); err != nil {
		return nil, err
	}
	result.BackupPath = save.LastBackupPath()
	if loadForVerification == nil {
		loadForVerification = LoadSave
	}
	verified, err := loadForVerification(absolute)
	if err == nil && (verified == nil || !bytes.Equal(verified.data, expected)) {
		err = errors.New("重新打开的存档与预期不一致")
	}
	if err == nil {
		var after *fateArchiveRecords
		after, err = readFateArchiveRecords(verified)
		if err == nil {
			for hash, want := range expectedFlags {
				record, ok := after.byHash[hash]
				if !ok || record.flags.Uint32() != want {
					err = fmt.Errorf("档案 %08X 解锁标志回读不一致", hash)
					break
				}
			}
			if err == nil {
				result.StoryArchives, err = inspectFateStoryArchives(layout, after)
			}
		}
	}
	if err != nil {
		return nil, fateStoryArchiveWriteFailure(absolute, original, expected, err)
	}
	result.Revision = fateEpisodeRevision(expected)
	result.Verified = len(seen)
	return result, nil
}
func (a *App) RepairFateStoryArchives(request FateStoryArchiveRepairRequest) (*FateStoryArchiveRepairResult, error) {
	offlineSaveMutationMu.Lock()
	defer offlineSaveMutationMu.Unlock()
	if err := ensureGeneratorWriteAllowed(request.Path); err != nil {
		return nil, err
	}
	return repairFateStoryArchivesLocked(request, LoadSave)
}
