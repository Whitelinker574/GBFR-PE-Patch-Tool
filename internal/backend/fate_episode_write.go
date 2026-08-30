package backend

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	fateEpisodeStateField      = "episodeState"
	fateEpisodeMissionField    = "missionState"
	fateEpisodeMaxFieldChanges = fateEpisodeCount + fateMissionCount
)

// FateEpisodeEditableField exposes only the two field families whose exact
// layout is already proven by the 2.0.2 catalog and save parser. The allowed
// targets are intentionally monotonic completion values; this endpoint is not
// a raw arbitrary-unit editor.
type FateEpisodeEditableField struct {
	Field               string   `json:"field"`
	EpisodeKey          string   `json:"episodeKey,omitempty"`
	CharacterCode       string   `json:"characterCode,omitempty"`
	TitleZh             string   `json:"titleZh,omitempty"`
	TitleEn             string   `json:"titleEn,omitempty"`
	MissionID           uint32   `json:"missionId,omitempty"`
	MissionCode         string   `json:"missionCode,omitempty"`
	VectorIndex         int      `json:"vectorIndex,omitempty"`
	CurrentValue        uint32   `json:"currentValue"`
	AllowedTargetValues []uint32 `json:"allowedTargetValues"`
}

// FateEpisodeEditableSnapshot binds a user's field selection to one exact
// on-disk save revision. A later write must return this Revision, otherwise a
// save changed by the game or another editor is rejected.
type FateEpisodeEditableSnapshot struct {
	Path                string                     `json:"path"`
	Revision            string                     `json:"revision"`
	Status              FateEpisodeStatus          `json:"status"`
	Fields              []FateEpisodeEditableField `json:"fields"`
	StoryArchives       []FateStoryArchiveStatus   `json:"storyArchives"`
	FieldWriteVerified  bool                       `json:"fieldWriteVerified"`
	GameEffectVerified  bool                       `json:"gameEffectVerified"`
	RewardClaimVerified bool                       `json:"rewardClaimVerified"`
}

type FateEpisodeFieldChange struct {
	Field         string `json:"field"`
	EpisodeKey    string `json:"episodeKey,omitempty"`
	MissionID     uint32 `json:"missionId,omitempty"`
	ExpectedValue uint32 `json:"expectedValue"`
	TargetValue   uint32 `json:"targetValue"`
}

type FateEpisodeFieldWriteRequest struct {
	Path             string                   `json:"path"`
	ExpectedRevision string                   `json:"expectedRevision"`
	Changes          []FateEpisodeFieldChange `json:"changes"`
}

type FateEpisodeFieldReadback struct {
	Field         string `json:"field"`
	EpisodeKey    string `json:"episodeKey,omitempty"`
	MissionID     uint32 `json:"missionId,omitempty"`
	PreviousValue uint32 `json:"previousValue"`
	Value         uint32 `json:"value"`
}

type FateEpisodeFieldWriteResult struct {
	Path                string                     `json:"path"`
	BackupPath          string                     `json:"backupPath,omitempty"`
	PreviousRevision    string                     `json:"previousRevision"`
	Revision            string                     `json:"revision"`
	Requested           int                        `json:"requested"`
	Changed             int                        `json:"changed"`
	Verified            int                        `json:"verified"`
	Readback            []FateEpisodeFieldReadback `json:"readback"`
	Status              FateEpisodeStatus          `json:"status"`
	FieldWriteVerified  bool                       `json:"fieldWriteVerified"`
	GameEffectVerified  bool                       `json:"gameEffectVerified"`
	RewardClaimVerified bool                       `json:"rewardClaimVerified"`
}

func fateEpisodeRevision(data []byte) string {
	sum := sha256.Sum256(data)
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

func readFateEpisodeRaw(path string) (string, []byte, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil, fmt.Errorf("请选择目标存档")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", nil, fmt.Errorf("解析目标存档路径失败: %w", err)
	}
	file, err := os.Open(absolute)
	if err != nil {
		return "", nil, fmt.Errorf("读取目标存档失败: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", nil, fmt.Errorf("读取目标存档信息失败: %w", err)
	}
	if info.IsDir() {
		return "", nil, fmt.Errorf("目标路径不是存档文件")
	}
	if info.Size() > saveDiffMaximumFileBytes {
		return "", nil, fmt.Errorf("目标存档超过 64 MiB 的处理上限")
	}
	data, err := io.ReadAll(io.LimitReader(file, saveDiffMaximumFileBytes+1))
	if err != nil {
		return "", nil, fmt.Errorf("读取目标存档失败: %w", err)
	}
	if int64(len(data)) > saveDiffMaximumFileBytes {
		return "", nil, fmt.Errorf("目标存档超过 64 MiB 的处理上限")
	}
	return absolute, data, nil
}

func fateEpisodeEditableFields(layout *fateEpisodeLayout) ([]FateEpisodeEditableField, error) {
	catalog, err := loadFateEpisodeCatalog()
	if err != nil {
		return nil, err
	}
	fields := make([]FateEpisodeEditableField, 0, fateEpisodeMaxFieldChanges)
	for _, episode := range catalog.Episodes {
		hash := gbfrHash32(episode.Key)
		state := layout.stateByHash[hash]
		if state == nil {
			return nil, fmt.Errorf("命运篇章 %s 缺少状态字段", episode.Key)
		}
		fields = append(fields, FateEpisodeEditableField{
			Field: fateEpisodeStateField, EpisodeKey: episode.Key,
			CharacterCode: episode.CharacterCode, TitleZh: episode.TitleZh, TitleEn: episode.TitleEn,
			CurrentValue: state.Uint32(), AllowedTargetValues: []uint32{fateCompletedState},
		})
	}
	for index := 0; index < fateMissionVectorLength; index++ {
		missionID, err := layout.missionIDs.Uint32At(index)
		if err != nil {
			return nil, err
		}
		if missionID == 0 {
			continue
		}
		state, err := layout.missionStates.Uint32At(index)
		if err != nil {
			return nil, err
		}
		fields = append(fields, FateEpisodeEditableField{
			Field: fateEpisodeMissionField, MissionID: missionID,
			MissionCode: fmt.Sprintf("%08X", missionID), VectorIndex: index,
			CurrentValue: state, AllowedTargetValues: []uint32{1},
		})
	}
	if len(fields) != fateEpisodeMaxFieldChanges {
		return nil, fmt.Errorf("可写命运篇章字段为 %d 条，期望 %d 条", len(fields), fateEpisodeMaxFieldChanges)
	}
	return fields, nil
}

func inspectFateEpisodeEditable(path string) (*FateEpisodeEditableSnapshot, error) {
	absolute, raw, err := readFateEpisodeRaw(path)
	if err != nil {
		return nil, err
	}
	save, err := newSaveData(absolute, append([]byte(nil), raw...))
	if err != nil {
		return nil, err
	}
	layout, err := inspectFateEpisodeLayout(save)
	if err != nil {
		return nil, err
	}
	fields, err := fateEpisodeEditableFields(layout)
	if err != nil {
		return nil, err
	}
	archiveVector, err := requireFateStoryArchiveVector(save)
	if err != nil {
		return nil, err
	}
	storyArchives, err := inspectFateStoryArchives(layout, archiveVector)
	if err != nil {
		return nil, err
	}
	status := layout.status
	status.Path = absolute
	return &FateEpisodeEditableSnapshot{
		Path: absolute, Revision: fateEpisodeRevision(raw), Status: status, Fields: fields, StoryArchives: storyArchives,
		FieldWriteVerified: false, GameEffectVerified: false, RewardClaimVerified: false,
	}, nil
}

// FateEpisodeEditableInspect is the read side of the field-write API. Its
// revision is mandatory for WriteFateEpisodeFields, so a UI cannot write from a
// stale inspection.
func (a *App) FateEpisodeEditableInspect(path string) (*FateEpisodeEditableSnapshot, error) {
	return inspectFateEpisodeEditable(path)
}

type resolvedFateEpisodeChange struct {
	request FateEpisodeFieldChange
	entry   *unitEntry
	index   int
	before  uint32
}

func normalizeFateEpisodeField(field string) string {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case strings.ToLower(fateEpisodeStateField):
		return fateEpisodeStateField
	case strings.ToLower(fateEpisodeMissionField):
		return fateEpisodeMissionField
	default:
		return ""
	}
}

func validateFateEpisodeRevision(revision string) (string, error) {
	revision = strings.ToUpper(strings.TrimSpace(revision))
	if len(revision) != sha256.Size*2 {
		return "", fmt.Errorf("存档修订标识无效；请重新检查目标存档")
	}
	if _, err := hex.DecodeString(revision); err != nil {
		return "", fmt.Errorf("存档修订标识无效；请重新检查目标存档")
	}
	return revision, nil
}

func missionStateIndex(layout *fateEpisodeLayout, missionID uint32) (int, uint32, error) {
	if _, known := fateMissionIDs()[missionID]; !known {
		return -1, 0, fmt.Errorf("任务 %08X 不在命运篇章 2.0.2 目录中", missionID)
	}
	found := -1
	var value uint32
	for index := 0; index < fateMissionVectorLength; index++ {
		currentID, err := layout.missionIDs.Uint32At(index)
		if err != nil {
			return -1, 0, err
		}
		if currentID != missionID {
			continue
		}
		if found >= 0 {
			return -1, 0, fmt.Errorf("任务 %08X 在状态向量中重复", missionID)
		}
		current, err := layout.missionStates.Uint32At(index)
		if err != nil {
			return -1, 0, err
		}
		found, value = index, current
	}
	if found < 0 {
		return -1, 0, fmt.Errorf("目标存档缺少命运篇章任务 %08X", missionID)
	}
	return found, value, nil
}

func resolveFateEpisodeChanges(layout *fateEpisodeLayout, changes []FateEpisodeFieldChange) ([]resolvedFateEpisodeChange, error) {
	if len(changes) == 0 {
		return nil, fmt.Errorf("没有选择要写入的命运篇章字段")
	}
	if len(changes) > fateEpisodeMaxFieldChanges {
		return nil, fmt.Errorf("一次最多写入 %d 个命运篇章字段", fateEpisodeMaxFieldChanges)
	}
	catalog, err := loadFateEpisodeCatalog()
	if err != nil {
		return nil, err
	}
	knownEpisodes := make(map[string]uint32, len(catalog.Episodes))
	for _, episode := range catalog.Episodes {
		knownEpisodes[strings.ToUpper(episode.Key)] = gbfrHash32(episode.Key)
	}
	seen := make(map[string]struct{}, len(changes))
	resolved := make([]resolvedFateEpisodeChange, 0, len(changes))
	for _, raw := range changes {
		change := raw
		change.Field = normalizeFateEpisodeField(change.Field)
		switch change.Field {
		case fateEpisodeStateField:
			change.EpisodeKey = strings.ToUpper(strings.TrimSpace(change.EpisodeKey))
			hash, known := knownEpisodes[change.EpisodeKey]
			if !known {
				return nil, fmt.Errorf("命运篇章 %q 不在 2.0.2 目录中", strings.TrimSpace(raw.EpisodeKey))
			}
			if change.MissionID != 0 {
				return nil, fmt.Errorf("篇章状态 %s 不接受任务 ID", change.EpisodeKey)
			}
			identity := fateEpisodeStateField + ":" + change.EpisodeKey
			if _, duplicate := seen[identity]; duplicate {
				return nil, fmt.Errorf("命运篇章字段 %s 被重复提交", change.EpisodeKey)
			}
			seen[identity] = struct{}{}
			entry := layout.stateByHash[hash]
			if entry == nil {
				return nil, fmt.Errorf("目标存档缺少命运篇章 %s", change.EpisodeKey)
			}
			current := entry.Uint32()
			if current != change.ExpectedValue {
				return nil, fmt.Errorf("命运篇章 %s 当前值已从 %d 变为 %d；请重新检查后再写入", change.EpisodeKey, change.ExpectedValue, current)
			}
			if change.TargetValue != current && change.TargetValue != fateCompletedState {
				return nil, fmt.Errorf("命运篇章 %s 只允许写入已验证的完成值 %d", change.EpisodeKey, fateCompletedState)
			}
			resolved = append(resolved, resolvedFateEpisodeChange{request: change, entry: entry, index: -1, before: current})
		case fateEpisodeMissionField:
			if strings.TrimSpace(change.EpisodeKey) != "" {
				return nil, fmt.Errorf("任务状态 %08X 不接受篇章 Key", change.MissionID)
			}
			index, current, err := missionStateIndex(layout, change.MissionID)
			if err != nil {
				return nil, err
			}
			identity := fateEpisodeMissionField + ":" + strconv.FormatUint(uint64(change.MissionID), 16)
			if _, duplicate := seen[identity]; duplicate {
				return nil, fmt.Errorf("命运篇章任务字段 %08X 被重复提交", change.MissionID)
			}
			seen[identity] = struct{}{}
			if current != change.ExpectedValue {
				return nil, fmt.Errorf("命运篇章任务 %08X 当前值已从 %d 变为 %d；请重新检查后再写入", change.MissionID, change.ExpectedValue, current)
			}
			if change.TargetValue != current && !(current == 0 && change.TargetValue == 1) {
				return nil, fmt.Errorf("命运篇章任务 %08X 只允许从 0 写入已验证的完成值 1", change.MissionID)
			}
			resolved = append(resolved, resolvedFateEpisodeChange{request: change, entry: layout.missionStates, index: index, before: current})
		default:
			return nil, fmt.Errorf("未知命运篇章字段 %q", strings.TrimSpace(raw.Field))
		}
	}
	return resolved, nil
}

func applyResolvedFateEpisodeChanges(changes []resolvedFateEpisodeChange) (int, error) {
	changed := 0
	for _, change := range changes {
		if change.before == change.request.TargetValue {
			continue
		}
		if change.index >= 0 {
			if err := change.entry.SetUint32At(change.index, change.request.TargetValue); err != nil {
				return 0, err
			}
		} else {
			change.entry.SetUint32(change.request.TargetValue)
		}
		changed++
	}
	return changed, nil
}

func verifyResolvedFateEpisodeChanges(layout *fateEpisodeLayout, changes []resolvedFateEpisodeChange) ([]FateEpisodeFieldReadback, error) {
	readback := make([]FateEpisodeFieldReadback, 0, len(changes))
	for _, change := range changes {
		actual := uint32(0)
		switch change.request.Field {
		case fateEpisodeStateField:
			entry := layout.stateByHash[gbfrHash32(change.request.EpisodeKey)]
			if entry == nil {
				return nil, fmt.Errorf("回读时缺少命运篇章 %s", change.request.EpisodeKey)
			}
			actual = entry.Uint32()
		case fateEpisodeMissionField:
			_, value, err := missionStateIndex(layout, change.request.MissionID)
			if err != nil {
				return nil, err
			}
			actual = value
		default:
			return nil, fmt.Errorf("回读时遇到未知字段 %q", change.request.Field)
		}
		if actual != change.request.TargetValue {
			return nil, fmt.Errorf("命运篇章字段回读为 %d，期望 %d", actual, change.request.TargetValue)
		}
		readback = append(readback, FateEpisodeFieldReadback{
			Field: change.request.Field, EpisodeKey: change.request.EpisodeKey, MissionID: change.request.MissionID,
			PreviousValue: change.before, Value: actual,
		})
	}
	return readback, nil
}

func fateEpisodeWriteFailure(path string, original, expected []byte, verifyErr error) error {
	current, readErr := os.ReadFile(path)
	if readErr != nil {
		return fmt.Errorf("命运篇章字段写入回读失败: %v；恢复前无法重新读取目标存档，请保留备份并停止继续操作: %w", verifyErr, readErr)
	}
	if !bytes.Equal(current, expected) {
		return fmt.Errorf("命运篇章字段写入回读失败: %v；目标存档在写入后又被其他程序修改，为避免覆盖新变化未自动恢复，请保留备份并停止继续操作", verifyErr)
	}
	if rollbackErr := rollbackSaveDiffTarget(path, original); rollbackErr != nil {
		return fmt.Errorf("命运篇章字段写入回读失败: %v；自动恢复也失败，请保留备份并停止继续操作: %w", verifyErr, rollbackErr)
	}
	return fmt.Errorf("命运篇章字段写入回读失败，已自动恢复写入前存档: %w", verifyErr)
}

func writeFateEpisodeFieldsLocked(request FateEpisodeFieldWriteRequest, loadForVerification func(string) (*SaveData, error)) (*FateEpisodeFieldWriteResult, error) {
	expectedRevision, err := validateFateEpisodeRevision(request.ExpectedRevision)
	if err != nil {
		return nil, err
	}
	absolute, original, err := readFateEpisodeRaw(request.Path)
	if err != nil {
		return nil, err
	}
	currentRevision := fateEpisodeRevision(original)
	if currentRevision != expectedRevision {
		return nil, fmt.Errorf("目标存档在检查后已被游戏或其他程序修改；为避免覆盖新进度，请重新检查后再写入")
	}
	save, err := newSaveData(absolute, append([]byte(nil), original...))
	if err != nil {
		return nil, err
	}
	layout, err := inspectFateEpisodeLayout(save)
	if err != nil {
		return nil, err
	}
	resolved, err := resolveFateEpisodeChanges(layout, request.Changes)
	if err != nil {
		return nil, err
	}
	changed, err := applyResolvedFateEpisodeChanges(resolved)
	if err != nil {
		return nil, err
	}
	if changed == 0 {
		readback, err := verifyResolvedFateEpisodeChanges(layout, resolved)
		if err != nil {
			return nil, err
		}
		status := layout.status
		status.Path = absolute
		return &FateEpisodeFieldWriteResult{
			Path: absolute, PreviousRevision: currentRevision, Revision: currentRevision,
			Requested: len(resolved), Changed: 0, Verified: len(readback), Readback: readback, Status: status,
			FieldWriteVerified: true, GameEffectVerified: false, RewardClaimVerified: false,
		}, nil
	}
	if err := save.FixChecksums(); err != nil {
		return nil, fmt.Errorf("修复存档校验失败: %w", err)
	}
	_, latest, err := readFateEpisodeRaw(absolute)
	if err != nil {
		return nil, fmt.Errorf("写入前重新读取目标存档失败: %w", err)
	}
	if !bytes.Equal(latest, original) {
		return nil, fmt.Errorf("准备写入时目标存档又发生了变化，已取消操作")
	}
	expectedFile := append([]byte(nil), save.data...)
	if err := save.Write(absolute); err != nil {
		return nil, fmt.Errorf("写入命运篇章字段失败: %w", err)
	}
	backupPath := save.LastBackupPath()
	if loadForVerification == nil {
		loadForVerification = LoadSave
	}
	verifiedSave, verifyErr := loadForVerification(absolute)
	if verifyErr == nil && !bytes.Equal(verifiedSave.data, expectedFile) {
		verifyErr = errors.New("重新打开的存档与本次事务预期不一致")
	}
	var verifiedLayout *fateEpisodeLayout
	if verifyErr == nil {
		verifiedLayout, verifyErr = inspectFateEpisodeLayout(verifiedSave)
	}
	var readback []FateEpisodeFieldReadback
	if verifyErr == nil {
		readback, verifyErr = verifyResolvedFateEpisodeChanges(verifiedLayout, resolved)
	}
	if verifyErr == nil && (!equalUint32Map(layout.auxiliaryStates, verifiedLayout.auxiliaryStates) || !equalUint32Map(layout.placeholder, verifiedLayout.placeholder)) {
		verifyErr = errors.New("REMI 辅助记录或占位记录发生了意外变化")
	}
	if verifyErr != nil {
		return nil, fateEpisodeWriteFailure(absolute, original, expectedFile, verifyErr)
	}
	status := verifiedLayout.status
	status.Path = absolute
	return &FateEpisodeFieldWriteResult{
		Path: absolute, BackupPath: backupPath,
		PreviousRevision: currentRevision, Revision: fateEpisodeRevision(expectedFile),
		Requested: len(resolved), Changed: changed, Verified: len(readback), Readback: readback, Status: status,
		FieldWriteVerified: true, GameEffectVerified: false, RewardClaimVerified: false,
	}, nil
}

// WriteFateEpisodeFields writes only explicitly selected 3502 episode states
// and 2561 mission states. It proves the file transaction and per-field
// readback, not reward claiming or in-game effect activation.
func (a *App) WriteFateEpisodeFields(request FateEpisodeFieldWriteRequest) (*FateEpisodeFieldWriteResult, error) {
	offlineSaveMutationMu.Lock()
	defer offlineSaveMutationMu.Unlock()
	if err := ensureGeneratorWriteAllowed(request.Path); err != nil {
		return nil, err
	}
	return writeFateEpisodeFieldsLocked(request, LoadSave)
}
