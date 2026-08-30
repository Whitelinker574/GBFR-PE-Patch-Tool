package backend

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	fateStoryArchiveIDType       uint32 = 2555
	fateStoryArchiveVectorLength int    = 200
	fateStoryArchiveCount        int    = 18
)

// FateStoryArchiveStatus is the exact save-backed subset of Fate Episode
// archives. In the 2.0.5 story_note_archive table these rows carry an explicit
// save flag index (52..69). Later DLC rows do not carry this explicit index and
// are deliberately not guessed here.
type FateStoryArchiveStatus struct {
	ArchiveID        string   `json:"archiveId"`
	CharacterCodes   []string `json:"characterCodes"`
	FinalEpisodeKeys []string `json:"finalEpisodeKeys"`
	VectorIndex      int      `json:"vectorIndex"`
	FinalCompleted   bool     `json:"finalCompleted"`
	Unlocked         bool     `json:"unlocked"`
	Missing          bool     `json:"missing"`
}

type fateStoryArchiveDefinition struct {
	archiveID        string
	vectorIndex      int
	characterCodes   []string
	finalEpisodeKeys []string
}

var fateStoryArchiveCatalog = []fateStoryArchiveDefinition{
	{archiveID: "ARC_OTHER_040", vectorIndex: 52, characterCodes: []string{"PL0000", "PL0100"}, finalEpisodeKeys: []string{"FATE_PL0000_10", "FATE_PL0100_10"}},
	{archiveID: "ARC_OTHER_041", vectorIndex: 53, characterCodes: []string{"PL0200"}, finalEpisodeKeys: []string{"FATE_PL0200_10"}},
	{archiveID: "ARC_OTHER_042", vectorIndex: 54, characterCodes: []string{"PL0300"}, finalEpisodeKeys: []string{"FATE_PL0300_10"}},
	{archiveID: "ARC_OTHER_043", vectorIndex: 55, characterCodes: []string{"PL0400"}, finalEpisodeKeys: []string{"FATE_PL0400_10"}},
	{archiveID: "ARC_OTHER_044", vectorIndex: 56, characterCodes: []string{"PL0500"}, finalEpisodeKeys: []string{"FATE_PL0500_10"}},
	{archiveID: "ARC_OTHER_045", vectorIndex: 57, characterCodes: []string{"PL0600"}, finalEpisodeKeys: []string{"FATE_PL0600_10"}},
	{archiveID: "ARC_OTHER_046", vectorIndex: 58, characterCodes: []string{"PL0700"}, finalEpisodeKeys: []string{"FATE_PL0700_10"}},
	{archiveID: "ARC_OTHER_047", vectorIndex: 59, characterCodes: []string{"PL0800"}, finalEpisodeKeys: []string{"FATE_PL0800_10"}},
	{archiveID: "ARC_OTHER_048", vectorIndex: 60, characterCodes: []string{"PL0900"}, finalEpisodeKeys: []string{"FATE_PL0900_10"}},
	{archiveID: "ARC_OTHER_049", vectorIndex: 61, characterCodes: []string{"PL1000"}, finalEpisodeKeys: []string{"FATE_PL1000_10"}},
	{archiveID: "ARC_OTHER_050", vectorIndex: 62, characterCodes: []string{"PL1100"}, finalEpisodeKeys: []string{"FATE_PL1100_10"}},
	{archiveID: "ARC_OTHER_051", vectorIndex: 63, characterCodes: []string{"PL1200"}, finalEpisodeKeys: []string{"FATE_PL1200_10"}},
	{archiveID: "ARC_OTHER_052", vectorIndex: 64, characterCodes: []string{"PL1300"}, finalEpisodeKeys: []string{"FATE_PL1300_10"}},
	{archiveID: "ARC_OTHER_053", vectorIndex: 65, characterCodes: []string{"PL1400"}, finalEpisodeKeys: []string{"FATE_PL1400_10"}},
	{archiveID: "ARC_OTHER_054", vectorIndex: 66, characterCodes: []string{"PL1500"}, finalEpisodeKeys: []string{"FATE_PL1500_10"}},
	{archiveID: "ARC_OTHER_055", vectorIndex: 67, characterCodes: []string{"PL1800"}, finalEpisodeKeys: []string{"FATE_PL1800_10"}},
	{archiveID: "ARC_OTHER_056", vectorIndex: 68, characterCodes: []string{"PL1800"}, finalEpisodeKeys: []string{"FATE_PL1800_10"}},
	{archiveID: "ARC_OTHER_057", vectorIndex: 69, characterCodes: []string{"PL1900"}, finalEpisodeKeys: []string{"FATE_PL1900_10"}},
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

func requireFateStoryArchiveVector(save *SaveData) (*unitEntry, error) {
	entry, err := findVectorUnitFast(save, fateStoryArchiveIDType, 0, fateStoryArchiveVectorLength)
	if err != nil {
		return nil, fmt.Errorf("读取命运篇章关联档案解锁位失败: %w", err)
	}
	values := entry.Bytes()
	if len(values) != fateStoryArchiveVectorLength {
		return nil, fmt.Errorf("命运篇章关联档案向量长度为 %d，期望 %d", len(values), fateStoryArchiveVectorLength)
	}
	for index, value := range values {
		if value != 0 && value != 1 {
			return nil, fmt.Errorf("命运篇章关联档案向量第 %d 项不是布尔值", index)
		}
	}
	return entry, nil
}

func inspectFateStoryArchives(layout *fateEpisodeLayout, vector *unitEntry) ([]FateStoryArchiveStatus, error) {
	values := vector.Bytes()
	rows := make([]FateStoryArchiveStatus, 0, len(fateStoryArchiveCatalog))
	for _, definition := range fateStoryArchiveCatalog {
		completed := false
		for _, episodeKey := range definition.finalEpisodeKeys {
			entry := layout.stateByHash[gbfrHash32(episodeKey)]
			if entry == nil {
				return nil, fmt.Errorf("命运篇章关联档案 %s 缺少最终篇章 %s", definition.archiveID, episodeKey)
			}
			if entry.Uint32() == fateCompletedState {
				completed = true
			}
		}
		unlocked := values[definition.vectorIndex] == 1
		rows = append(rows, FateStoryArchiveStatus{
			ArchiveID: definition.archiveID, CharacterCodes: append([]string(nil), definition.characterCodes...),
			FinalEpisodeKeys: append([]string(nil), definition.finalEpisodeKeys...), VectorIndex: definition.vectorIndex,
			FinalCompleted: completed, Unlocked: unlocked, Missing: completed && !unlocked,
		})
	}
	return rows, nil
}

func fateStateValues(layout *fateEpisodeLayout) map[uint32]uint32 {
	values := make(map[uint32]uint32, len(layout.stateByHash))
	for hash, entry := range layout.stateByHash {
		values[hash] = entry.Uint32()
	}
	return values
}

func fateMissionValues(layout *fateEpisodeLayout) ([]uint32, error) {
	values := make([]uint32, fateMissionVectorLength)
	for index := range values {
		value, err := layout.missionStates.Uint32At(index)
		if err != nil {
			return nil, err
		}
		values[index] = value
	}
	return values, nil
}

func fateStoryArchiveWriteFailure(path string, original, expected []byte, verifyErr error) error {
	current, readErr := os.ReadFile(path)
	if readErr != nil {
		return fmt.Errorf("档案补全回读失败: %v；恢复前无法重新读取目标存档，请保留备份并停止继续操作: %w", verifyErr, readErr)
	}
	if !bytes.Equal(current, expected) {
		return fmt.Errorf("档案补全回读失败: %v；目标存档在写入后又被其他程序修改，为避免覆盖新变化未自动恢复，请保留备份并停止继续操作", verifyErr)
	}
	if rollbackErr := rollbackSaveDiffTarget(path, original); rollbackErr != nil {
		return fmt.Errorf("档案补全回读失败: %v；自动恢复也失败，请保留备份并停止继续操作: %w", verifyErr, rollbackErr)
	}
	return fmt.Errorf("档案补全回读失败，已自动恢复写入前存档: %w", verifyErr)
}

func repairFateStoryArchivesLocked(request FateStoryArchiveRepairRequest, loadForVerification func(string) (*SaveData, error)) (*FateStoryArchiveRepairResult, error) {
	expectedRevision, err := validateFateEpisodeRevision(request.ExpectedRevision)
	if err != nil {
		return nil, err
	}
	if len(request.ArchiveIDs) == 0 {
		return nil, fmt.Errorf("没有选择要补全的命运篇章关联档案")
	}
	absolute, original, err := readFateEpisodeRaw(request.Path)
	if err != nil {
		return nil, err
	}
	currentRevision := fateEpisodeRevision(original)
	if currentRevision != expectedRevision {
		return nil, fmt.Errorf("目标存档在检查后已被游戏或其他程序修改；请重新检查后再补全档案")
	}
	save, err := newSaveData(absolute, append([]byte(nil), original...))
	if err != nil {
		return nil, err
	}
	layout, err := inspectFateEpisodeLayout(save)
	if err != nil {
		return nil, err
	}
	vector, err := requireFateStoryArchiveVector(save)
	if err != nil {
		return nil, err
	}
	statuses, err := inspectFateStoryArchives(layout, vector)
	if err != nil {
		return nil, err
	}
	known := make(map[string]FateStoryArchiveStatus, len(statuses))
	for _, status := range statuses {
		known[status.ArchiveID] = status
	}
	selected := make([]FateStoryArchiveStatus, 0, len(request.ArchiveIDs))
	seen := make(map[string]struct{}, len(request.ArchiveIDs))
	for _, rawID := range request.ArchiveIDs {
		id := strings.ToUpper(strings.TrimSpace(rawID))
		status, ok := known[id]
		if !ok {
			return nil, fmt.Errorf("档案 %q 不在游戏 2.0.5 已确认的命运篇章档案目录中", rawID)
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, fmt.Errorf("档案 %s 被重复提交", id)
		}
		seen[id] = struct{}{}
		if !status.FinalCompleted {
			return nil, fmt.Errorf("档案 %s 对应的最终命运篇章尚未完成，未写入", id)
		}
		selected = append(selected, status)
	}

	originalVector := append([]byte(nil), vector.Bytes()...)
	expectedVector := append([]byte(nil), originalVector...)
	changed := 0
	for _, status := range selected {
		if expectedVector[status.VectorIndex] == 0 {
			expectedVector[status.VectorIndex] = 1
			changed++
		}
	}
	if changed == 0 {
		return &FateStoryArchiveRepairResult{
			Path: absolute, PreviousRevision: currentRevision, Revision: currentRevision,
			Requested: len(selected), Changed: 0, Verified: len(selected), StoryArchives: statuses,
		}, nil
	}
	copy(vector.Bytes(), expectedVector)
	beforeFateStates := fateStateValues(layout)
	beforeMissionStates, err := fateMissionValues(layout)
	if err != nil {
		return nil, err
	}
	if err := save.FixChecksums(); err != nil {
		return nil, fmt.Errorf("修复存档校验失败: %w", err)
	}
	_, latest, err := readFateEpisodeRaw(absolute)
	if err != nil {
		return nil, fmt.Errorf("写入前重新读取目标存档失败: %w", err)
	}
	if !bytes.Equal(latest, original) {
		return nil, fmt.Errorf("准备补全档案时目标存档又发生了变化，已取消操作")
	}
	expectedFile := append([]byte(nil), save.data...)
	if err := save.Write(absolute); err != nil {
		return nil, fmt.Errorf("写入命运篇章关联档案失败: %w", err)
	}
	result := &FateStoryArchiveRepairResult{
		Path: absolute, BackupPath: save.LastBackupPath(), PreviousRevision: currentRevision,
		Revision: fateEpisodeRevision(expectedFile), Requested: len(selected), Changed: changed,
	}
	if loadForVerification == nil {
		loadForVerification = LoadSave
	}
	verifiedSave, verifyErr := loadForVerification(absolute)
	if verifyErr == nil && !bytes.Equal(verifiedSave.data, expectedFile) {
		verifyErr = errors.New("重新打开的存档与本次档案事务预期不一致")
	}
	var verifiedStatuses []FateStoryArchiveStatus
	if verifyErr == nil {
		verifiedLayout, layoutErr := inspectFateEpisodeLayout(verifiedSave)
		if layoutErr != nil {
			verifyErr = layoutErr
		} else {
			verifiedVector, vectorErr := requireFateStoryArchiveVector(verifiedSave)
			if vectorErr != nil {
				verifyErr = vectorErr
			} else if !bytes.Equal(verifiedVector.Bytes(), expectedVector) {
				verifyErr = errors.New("档案解锁向量回读与请求不符")
			} else {
				verifiedStatuses, verifyErr = inspectFateStoryArchives(verifiedLayout, verifiedVector)
			}
			if verifyErr == nil && !equalUint32Map(beforeFateStates, fateStateValues(verifiedLayout)) {
				verifyErr = errors.New("档案补全意外改动了命运篇章状态")
			}
			if verifyErr == nil {
				afterMissionStates, missionErr := fateMissionValues(verifiedLayout)
				if missionErr != nil {
					verifyErr = missionErr
				} else if len(afterMissionStates) != len(beforeMissionStates) {
					verifyErr = errors.New("档案补全意外改动了任务状态长度")
				} else {
					for index := range beforeMissionStates {
						if afterMissionStates[index] != beforeMissionStates[index] {
							verifyErr = errors.New("档案补全意外改动了命运篇章任务状态")
							break
						}
					}
				}
			}
		}
	}
	if verifyErr != nil {
		return nil, fateStoryArchiveWriteFailure(absolute, original, expectedFile, verifyErr)
	}
	for _, selectedStatus := range selected {
		found := false
		for _, status := range verifiedStatuses {
			if status.ArchiveID == selectedStatus.ArchiveID {
				found = status.Unlocked
				break
			}
		}
		if !found {
			return nil, fateStoryArchiveWriteFailure(absolute, original, expectedFile, fmt.Errorf("档案 %s 回读仍未解锁", selectedStatus.ArchiveID))
		}
	}
	result.Verified = len(selected)
	result.StoryArchives = verifiedStatuses
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
