package backend

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	fateIDType              uint32 = 3501
	fateStateIDType         uint32 = 3502
	fateMissionIDType       uint32 = 2560
	fateMissionStateIDType  uint32 = 2561
	fateCompletedState      uint32 = 30
	fatePlaceholderState    uint32 = 5
	fateEpisodeCount               = 319
	fateAuxiliaryCount             = 5
	fatePlaceholderCount           = 496
	fateRecordCount                = 820
	fateMissionCount               = 56
	fateMissionVectorLength        = 100
)

var fateCharacterCodes = [...]string{
	"PL0000", "PL0100", "PL0200", "PL0300", "PL0400", "PL0500", "PL0600", "PL0700", "PL0800", "PL0900",
	"PL1000", "PL1100", "PL1200", "PL1300", "PL1400", "PL1500", "PL1600", "PL1700", "PL1800", "PL1900",
	"PL2100", "PL2200", "PL2300", "PL2400", "PL2500", "PL2600", "PL2700", "PL2800", "PL2900",
}

var fateAuxiliaryHashes = [...]uint32{
	0x404427C6, // REMI_PL0200_00
	0xF7DA61CC, // REMI_PL0300_00
	0x6E246C8D, // REMI_PL0400_00
	0x009202A6, // REMI_PL0500_00
	0x1E539D44, // REMI_PL0600_00
}

func fateMissionIDs() map[uint32]struct{} {
	result := make(map[uint32]struct{}, fateMissionCount)
	for _, base := range [...]uint32{0x00300000, 0x00301000} {
		result[base] = struct{}{}
		for suffix := uint32(2); suffix <= 9; suffix++ {
			result[base+suffix] = struct{}{}
		}
		for suffix := uint32(0x10); suffix <= 0x19; suffix++ {
			result[base+suffix] = struct{}{}
		}
		for suffix := uint32(0x21); suffix <= 0x29; suffix++ {
			result[base+suffix] = struct{}{}
		}
	}
	return result
}

type FateEpisodeCharacterStatus struct {
	Code                  string                   `json:"code"`
	Completed             int                      `json:"completed"`
	Total                 int                      `json:"total"`
	CompletedStaticHP     float64                  `json:"completedStaticHp"`
	CompletedStaticAttack float64                  `json:"completedStaticAttack"`
	Episodes              []FateEpisodeEntryStatus `json:"episodes"`
}

type FateEpisodeStatus struct {
	Path               string                       `json:"path"`
	DataVersion        string                       `json:"dataVersion"`
	Completed          int                          `json:"completed"`
	Total              int                          `json:"total"`
	MissionCompleted   int                          `json:"missionCompleted"`
	MissionTotal       int                          `json:"missionTotal"`
	AuxiliaryPreserved int                          `json:"auxiliaryPreserved"`
	Characters         []FateEpisodeCharacterStatus `json:"characters"`
}

type FateEpisodeWriteResult struct {
	Path                  string            `json:"path"`
	BackupPath            string            `json:"backupPath,omitempty"`
	EpisodesChanged       int               `json:"episodesChanged"`
	MissionsChanged       int               `json:"missionsChanged"`
	VerifiedEpisodes      int               `json:"verifiedEpisodes"`
	VerifiedMissions      int               `json:"verifiedMissions"`
	AuxiliaryPreserved    int               `json:"auxiliaryPreserved"`
	PlaceholdersPreserved int               `json:"placeholdersPreserved"`
	Status                FateEpisodeStatus `json:"status"`
}

type FateEpisodeEvidenceExport struct {
	SchemaVersion      int                          `json:"schemaVersion"`
	DataVersion        string                       `json:"dataVersion"`
	GeneratedAt        string                       `json:"generatedAt"`
	Completed          int                          `json:"completed"`
	Total              int                          `json:"total"`
	MissionCompleted   int                          `json:"missionCompleted"`
	MissionTotal       int                          `json:"missionTotal"`
	AuxiliaryPreserved int                          `json:"auxiliaryPreserved"`
	Characters         []FateEpisodeCharacterStatus `json:"characters"`
}

type FateEpisodeEntryStatus struct {
	Key             string  `json:"key"`
	Hash            string  `json:"hash"`
	Index           int     `json:"index"`
	TitleZh         string  `json:"titleZh"`
	TitleEn         string  `json:"titleEn"`
	RequiredLevel   int     `json:"requiredLevel"`
	RequiredQuestID string  `json:"requiredQuestId,omitempty"`
	MissionQuestID  string  `json:"missionQuestId,omitempty"`
	FinalEpisode    bool    `json:"finalEpisode"`
	State           uint32  `json:"state"`
	Completed       bool    `json:"completed"`
	HasStaticBonus  bool    `json:"hasStaticBonus"`
	StaticHP        float64 `json:"staticHp,omitempty"`
	StaticAttack    float64 `json:"staticAttack,omitempty"`
}

type fateEpisodeCatalogBonus struct {
	HP     float64 `json:"hp"`
	Attack float64 `json:"attack"`
}

type fateEpisodeCatalogEntry struct {
	Key             string                   `json:"key"`
	CharacterCode   string                   `json:"characterCode"`
	Index           int                      `json:"index"`
	RequiredLevel   int                      `json:"requiredLevel"`
	RequiredQuestID string                   `json:"requiredQuestId"`
	MissionQuestID  string                   `json:"missionQuestId"`
	FinalEpisode    bool                     `json:"finalEpisode"`
	TitleZh         string                   `json:"titleZh"`
	TitleEn         string                   `json:"titleEn"`
	StaticBonus     *fateEpisodeCatalogBonus `json:"staticBonus,omitempty"`
}

type fateEpisodeCatalogFile struct {
	SchemaVersion int                       `json:"schemaVersion"`
	DataVersion   string                    `json:"dataVersion"`
	Episodes      []fateEpisodeCatalogEntry `json:"episodes"`
}

//go:embed data/fate_episode_catalog_202.json
var fateEpisodeCatalogJSON []byte

var (
	fateEpisodeCatalogOnce sync.Once
	fateEpisodeCatalogData fateEpisodeCatalogFile
	fateEpisodeCatalogErr  error
)

func loadFateEpisodeCatalog() (*fateEpisodeCatalogFile, error) {
	fateEpisodeCatalogOnce.Do(func() {
		if err := json.Unmarshal(fateEpisodeCatalogJSON, &fateEpisodeCatalogData); err != nil {
			fateEpisodeCatalogErr = fmt.Errorf("解析命运篇章 2.0.2 目录失败: %w", err)
			return
		}
		if fateEpisodeCatalogData.SchemaVersion != 1 || fateEpisodeCatalogData.DataVersion != "2.0.2" || len(fateEpisodeCatalogData.Episodes) != fateEpisodeCount {
			fateEpisodeCatalogErr = fmt.Errorf("命运篇章目录版本或数量无效")
			return
		}
		seen := make(map[uint32]struct{}, fateEpisodeCount)
		counts := make(map[string]int, len(fateCharacterCodes))
		for _, episode := range fateEpisodeCatalogData.Episodes {
			hash := gbfrHash32(episode.Key)
			// A zero XXHash is a valid game identifier (FATE_PL0100_00), not an
			// empty sentinel. Validate the source key and uniqueness instead.
			if episode.Key == "" || episode.CharacterCode == "" || episode.Index < 0 || episode.Index >= 11 {
				fateEpisodeCatalogErr = fmt.Errorf("命运篇章目录包含无效记录 %q", episode.Key)
				return
			}
			if _, duplicate := seen[hash]; duplicate {
				fateEpisodeCatalogErr = fmt.Errorf("命运篇章目录哈希 %08X 重复", hash)
				return
			}
			seen[hash] = struct{}{}
			counts[episode.CharacterCode]++
		}
		for _, code := range fateCharacterCodes {
			if counts[code] != 11 {
				fateEpisodeCatalogErr = fmt.Errorf("命运篇章目录角色 %s 有 %d 篇，期望 11", code, counts[code])
				return
			}
		}
	})
	if fateEpisodeCatalogErr != nil {
		return nil, fateEpisodeCatalogErr
	}
	return &fateEpisodeCatalogData, nil
}

type fateEpisodeLayout struct {
	status          FateEpisodeStatus
	stateByHash     map[uint32]*unitEntry
	missionIDs      *unitEntry
	missionStates   *unitEntry
	auxiliaryStates map[uint32]uint32
	placeholder     map[uint32]uint32
}

func fateHashesByCharacter() map[string][]uint32 {
	result := make(map[string][]uint32, len(fateCharacterCodes))
	for _, code := range fateCharacterCodes {
		rows := make([]uint32, 0, 11)
		for episode := 0; episode < 11; episode++ {
			rows = append(rows, gbfrHash32(fmt.Sprintf("FATE_%s_%02d", code, episode)))
		}
		result[code] = rows
	}
	return result
}

func findUnitsByTypeFast(save *SaveData, idType uint32, expected int) ([]*unitEntry, error) {
	slot := save.slotSpan()
	slotBase := int(save.slotOff)
	idBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(idBytes, idType)
	seen := make(map[int]struct{}, expected)
	results := make([]*unitEntry, 0, expected)
	for searchAt := 0; searchAt <= len(slot)-4; {
		relative := bytes.Index(slot[searchAt:], idBytes)
		if relative < 0 {
			break
		}
		position := searchAt + relative
		searchAt = position + 1
		start := position - 20
		if start < 0 {
			start = 0
		}
		for tableOff := start; tableOff <= position; tableOff++ {
			if _, duplicate := seen[tableOff]; duplicate {
				continue
			}
			entry, ok := tryReadUnitEntry(slot, tableOff, idType, 0)
			if !ok || entry.IDType != idType {
				continue
			}
			seen[tableOff] = struct{}{}
			entry.ValueOff += slotBase
			entry.data = save.data
			results = append(results, entry)
			break
		}
	}
	if expected > 0 && len(results) != expected {
		return nil, fmt.Errorf("存档字段 %d 快速索引找到 %d 条，期望 %d 条", idType, len(results), expected)
	}
	return results, nil
}

func findVectorUnitFast(save *SaveData, idType, unitID uint32, valueCount int) (*unitEntry, error) {
	units, err := findUnitsByTypeFast(save, idType, 0)
	if err != nil {
		return nil, err
	}
	var matched *unitEntry
	for _, unit := range units {
		if unit.UnitID != unitID || unit.ValueCnt != valueCount {
			continue
		}
		if matched != nil {
			return nil, fmt.Errorf("存档字段 %d UnitID %d 长度 %d 重复", idType, unitID, valueCount)
		}
		matched = unit
	}
	if matched == nil {
		return nil, fmt.Errorf("存档缺少字段 %d UnitID %d 长度 %d", idType, unitID, valueCount)
	}
	return matched, nil
}

func inspectFateEpisodeLayout(save *SaveData) (*fateEpisodeLayout, error) {
	catalog, err := loadFateEpisodeCatalog()
	if err != nil {
		return nil, err
	}
	ids, err := findUnitsByTypeFast(save, fateIDType, fateRecordCount)
	if err != nil {
		return nil, err
	}
	states, err := findUnitsByTypeFast(save, fateStateIDType, fateRecordCount)
	if err != nil {
		return nil, err
	}
	stateByUnit := make(map[uint32]*unitEntry, len(states))
	for _, state := range states {
		if state.ValueCnt != 1 {
			return nil, fmt.Errorf("命运篇章状态 unit %d 不是单值记录", state.UnitID)
		}
		if _, duplicate := stateByUnit[state.UnitID]; duplicate {
			return nil, fmt.Errorf("命运篇章状态 unit %d 重复", state.UnitID)
		}
		stateByUnit[state.UnitID] = state
	}

	hashesByCharacter := fateHashesByCharacter()
	wanted := make(map[uint32]string, fateEpisodeCount)
	for code, hashes := range hashesByCharacter {
		for _, hash := range hashes {
			if previous, duplicate := wanted[hash]; duplicate {
				return nil, fmt.Errorf("命运篇章哈希冲突：%s 与 %s 共用 %08X", previous, code, hash)
			}
			wanted[hash] = code
		}
	}

	stateByHash := make(map[uint32]*unitEntry, fateEpisodeCount)
	auxiliaryStates := make(map[uint32]uint32, fateAuxiliaryCount)
	placeholder := make(map[uint32]uint32, fatePlaceholderCount)
	completedByCharacter := make(map[string]int, len(fateCharacterCodes))
	auxiliaryCatalog := make(map[uint32]struct{}, fateAuxiliaryCount)
	for _, hash := range fateAuxiliaryHashes {
		auxiliaryCatalog[hash] = struct{}{}
	}
	seenIDUnits := make(map[uint32]struct{}, len(ids))
	for _, id := range ids {
		if id.ValueCnt != 1 {
			return nil, fmt.Errorf("命运篇章 ID unit %d 不是单值记录", id.UnitID)
		}
		if _, duplicate := seenIDUnits[id.UnitID]; duplicate {
			return nil, fmt.Errorf("命运篇章 ID unit %d 重复", id.UnitID)
		}
		seenIDUnits[id.UnitID] = struct{}{}
		state, ok := stateByUnit[id.UnitID]
		if !ok {
			return nil, fmt.Errorf("命运篇章 ID unit %d 缺少配对状态", id.UnitID)
		}
		hash := id.Uint32()
		if hash == EmptyHash {
			if state.Uint32() != fatePlaceholderState {
				return nil, fmt.Errorf("命运篇章占位 unit %d 状态为 %d，期望 %d", id.UnitID, state.Uint32(), fatePlaceholderState)
			}
			placeholder[id.UnitID] = state.Uint32()
			continue
		}
		if code, isFate := wanted[hash]; isFate {
			if _, duplicate := stateByHash[hash]; duplicate {
				return nil, fmt.Errorf("命运篇章哈希 %08X 在存档中重复", hash)
			}
			stateByHash[hash] = state
			if state.Uint32() == fateCompletedState {
				completedByCharacter[code]++
			}
			continue
		}
		if _, known := auxiliaryCatalog[hash]; !known {
			return nil, fmt.Errorf("命运篇章目录包含未收录哈希 %08X", hash)
		}
		if _, duplicate := auxiliaryStates[hash]; duplicate {
			return nil, fmt.Errorf("命运篇章辅助哈希 %08X 重复", hash)
		}
		auxiliaryStates[hash] = state.Uint32()
	}
	if len(stateByHash) != fateEpisodeCount || len(auxiliaryStates) != fateAuxiliaryCount || len(placeholder) != fatePlaceholderCount {
		return nil, fmt.Errorf("命运篇章目录不完整：FATE/辅助/占位=%d/%d/%d，期望 %d/%d/%d", len(stateByHash), len(auxiliaryStates), len(placeholder), fateEpisodeCount, fateAuxiliaryCount, fatePlaceholderCount)
	}

	missionIDs, err := findVectorUnitFast(save, fateMissionIDType, 0, fateMissionVectorLength)
	if err != nil {
		return nil, err
	}
	missionStates, err := findVectorUnitFast(save, fateMissionStateIDType, 0, fateMissionVectorLength)
	if err != nil {
		return nil, err
	}
	missionCatalog := fateMissionIDs()
	seenMissions := make(map[uint32]struct{}, fateMissionCount)
	missionTotal, missionCompleted := 0, 0
	for index := 0; index < fateMissionVectorLength; index++ {
		missionID, err := missionIDs.Uint32At(index)
		if err != nil {
			return nil, err
		}
		missionState, err := missionStates.Uint32At(index)
		if err != nil {
			return nil, err
		}
		if missionID == 0 {
			if missionState != 0 {
				return nil, fmt.Errorf("命运篇章空任务槽 %d 的状态为 %d", index, missionState)
			}
			continue
		}
		if _, known := missionCatalog[missionID]; !known {
			return nil, fmt.Errorf("命运篇章任务槽 %d 包含未收录任务 %08X", index, missionID)
		}
		if _, duplicate := seenMissions[missionID]; duplicate {
			return nil, fmt.Errorf("命运篇章任务 %08X 重复", missionID)
		}
		seenMissions[missionID] = struct{}{}
		missionTotal++
		if missionState > 0 {
			missionCompleted++
		}
	}
	if missionTotal != fateMissionCount || len(seenMissions) != len(missionCatalog) {
		return nil, fmt.Errorf("命运篇章任务数量为 %d，期望 %d", missionTotal, fateMissionCount)
	}

	episodesByCharacter := make(map[string][]fateEpisodeCatalogEntry, len(fateCharacterCodes))
	for _, episode := range catalog.Episodes {
		episodesByCharacter[episode.CharacterCode] = append(episodesByCharacter[episode.CharacterCode], episode)
	}
	characters := make([]FateEpisodeCharacterStatus, 0, len(fateCharacterCodes))
	completed := 0
	for _, code := range fateCharacterCodes {
		count := completedByCharacter[code]
		completed += count
		character := FateEpisodeCharacterStatus{Code: code, Completed: count, Total: 11, Episodes: make([]FateEpisodeEntryStatus, 0, 11)}
		for _, episode := range episodesByCharacter[code] {
			hash := gbfrHash32(episode.Key)
			state := stateByHash[hash].Uint32()
			entry := FateEpisodeEntryStatus{
				Key: episode.Key, Hash: fmt.Sprintf("%08X", hash), Index: episode.Index,
				TitleZh: episode.TitleZh, TitleEn: episode.TitleEn,
				RequiredLevel: episode.RequiredLevel, RequiredQuestID: episode.RequiredQuestID,
				MissionQuestID: episode.MissionQuestID, FinalEpisode: episode.FinalEpisode,
				State: state, Completed: state == fateCompletedState,
			}
			if episode.StaticBonus != nil {
				entry.HasStaticBonus = true
				entry.StaticHP = episode.StaticBonus.HP
				entry.StaticAttack = episode.StaticBonus.Attack
				if entry.Completed {
					character.CompletedStaticHP += entry.StaticHP
					character.CompletedStaticAttack += entry.StaticAttack
				}
			}
			character.Episodes = append(character.Episodes, entry)
		}
		characters = append(characters, character)
	}
	return &fateEpisodeLayout{
		status:      FateEpisodeStatus{DataVersion: catalog.DataVersion, Completed: completed, Total: fateEpisodeCount, MissionCompleted: missionCompleted, MissionTotal: missionTotal, AuxiliaryPreserved: len(auxiliaryStates), Characters: characters},
		stateByHash: stateByHash, missionIDs: missionIDs, missionStates: missionStates, auxiliaryStates: auxiliaryStates, placeholder: placeholder,
	}, nil
}

func (a *App) FateEpisodeInspect(path string) (*FateEpisodeStatus, error) {
	save, err := LoadSave(path)
	if err != nil {
		return nil, err
	}
	layout, err := inspectFateEpisodeLayout(save)
	if err != nil {
		return nil, err
	}
	status := layout.status
	status.Path, _ = filepath.Abs(path)
	return &status, nil
}

func (a *App) CompleteAllFateEpisodes(path string) (*FateEpisodeWriteResult, error) {
	return nil, fmt.Errorf("命运篇章批量写入仍处于只读研究阶段；完成独立存档、领取状态和游戏内重读验证前不会开放")
}

func completeAllFateEpisodesLocked(path string) (*FateEpisodeWriteResult, error) {
	save, err := LoadSave(path)
	if err != nil {
		return nil, err
	}
	originalFile := append([]byte(nil), save.data...)
	layout, err := inspectFateEpisodeLayout(save)
	if err != nil {
		return nil, err
	}
	result := &FateEpisodeWriteResult{
		AuxiliaryPreserved:    len(layout.auxiliaryStates),
		PlaceholdersPreserved: len(layout.placeholder),
	}
	for _, state := range layout.stateByHash {
		if state.Uint32() == fateCompletedState {
			continue
		}
		state.SetUint32(fateCompletedState)
		result.EpisodesChanged++
	}
	for index := 0; index < fateMissionVectorLength; index++ {
		missionID, err := layout.missionIDs.Uint32At(index)
		if err != nil {
			return nil, err
		}
		missionState, err := layout.missionStates.Uint32At(index)
		if err != nil {
			return nil, err
		}
		if missionID != 0 && missionState == 0 {
			if err := layout.missionStates.SetUint32At(index, 1); err != nil {
				return nil, err
			}
			result.MissionsChanged++
		}
	}
	if result.EpisodesChanged == 0 && result.MissionsChanged == 0 {
		status := layout.status
		status.Path, _ = filepath.Abs(path)
		result.Path = status.Path
		result.VerifiedEpisodes = status.Completed
		result.VerifiedMissions = status.MissionCompleted
		result.Status = status
		return result, nil
	}
	if err := save.FixChecksums(); err != nil {
		return nil, fmt.Errorf("修复存档校验失败: %w", err)
	}
	currentFile, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("写入前重新读取存档失败: %w", err)
	}
	if !bytes.Equal(currentFile, originalFile) {
		return nil, fmt.Errorf("存档在检查后已被其他程序修改，请重新读取后再试")
	}
	expectedFile := append([]byte(nil), save.data...)
	if err := save.Write(path); err != nil {
		return nil, err
	}
	result.BackupPath = save.LastBackupPath()
	result.Path, _ = filepath.Abs(path)

	verifiedSave, err := LoadSave(path)
	if err != nil {
		return nil, fmt.Errorf("命运篇章已写入，但重新读取失败；请从备份恢复: %w", err)
	}
	if !bytes.Equal(verifiedSave.data, expectedFile) {
		return nil, fmt.Errorf("命运篇章写入后的文件与预期事务不一致；请从备份恢复")
	}
	verifiedLayout, err := inspectFateEpisodeLayout(verifiedSave)
	if err != nil {
		return nil, fmt.Errorf("命运篇章写入后的结构校验失败；请从备份恢复: %w", err)
	}
	if verifiedLayout.status.Completed != fateEpisodeCount || verifiedLayout.status.MissionCompleted != fateMissionCount {
		return nil, fmt.Errorf("命运篇章写入后仅完成 %d/%d 篇、%d/%d 个任务；请从备份恢复", verifiedLayout.status.Completed, fateEpisodeCount, verifiedLayout.status.MissionCompleted, fateMissionCount)
	}
	if !equalUint32Map(layout.auxiliaryStates, verifiedLayout.auxiliaryStates) || !equalUint32Map(layout.placeholder, verifiedLayout.placeholder) {
		return nil, fmt.Errorf("命运篇章写入影响了 REMI 或占位记录；请从备份恢复")
	}
	for index := 0; index < fateMissionVectorLength; index++ {
		beforeID, beforeErr := layout.missionIDs.Uint32At(index)
		afterID, afterErr := verifiedLayout.missionIDs.Uint32At(index)
		afterState, stateErr := verifiedLayout.missionStates.Uint32At(index)
		if beforeErr != nil || afterErr != nil || stateErr != nil || beforeID != afterID || (afterID == 0 && afterState != 0) {
			return nil, fmt.Errorf("命运篇章任务向量在槽位 %d 校验失败；请从备份恢复", index)
		}
	}
	status := verifiedLayout.status
	status.Path = result.Path
	result.VerifiedEpisodes = status.Completed
	result.VerifiedMissions = status.MissionCompleted
	result.AuxiliaryPreserved = len(verifiedLayout.auxiliaryStates)
	result.PlaceholdersPreserved = len(verifiedLayout.placeholder)
	result.Status = status
	return result, nil
}

func equalUint32Map(left, right map[uint32]uint32) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func fateEpisodeEvidenceJSON(status *FateEpisodeStatus, generatedAt time.Time) ([]byte, error) {
	if status == nil {
		return nil, fmt.Errorf("命运篇章状态为空")
	}
	payload := FateEpisodeEvidenceExport{
		SchemaVersion: 1, DataVersion: status.DataVersion, GeneratedAt: generatedAt.UTC().Format(time.RFC3339),
		Completed: status.Completed, Total: status.Total,
		MissionCompleted: status.MissionCompleted, MissionTotal: status.MissionTotal,
		AuxiliaryPreserved: status.AuxiliaryPreserved,
		Characters:         append([]FateEpisodeCharacterStatus(nil), status.Characters...),
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("生成命运篇章证据 JSON 失败: %w", err)
	}
	return data, nil
}

func (a *App) ExportFateEpisodeEvidence(path string) (string, error) {
	status, err := a.FateEpisodeInspect(path)
	if err != nil {
		return "", err
	}
	data, err := fateEpisodeEvidenceJSON(status, time.Now())
	if err != nil {
		return "", err
	}
	outputPath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出命运篇章只读证据",
		DefaultFilename: "gbfr-fate-evidence.json",
		Filters:         []runtime.FileFilter{{DisplayName: "JSON", Pattern: "*.json"}},
	})
	if err != nil || outputPath == "" {
		return "", err
	}
	if err := writeSaveDiffFile(outputPath, data); err != nil {
		return "", err
	}
	return outputPath, nil
}

func sortedFateHashes() []uint32 {
	result := make([]uint32, 0, fateEpisodeCount)
	for _, hashes := range fateHashesByCharacter() {
		result = append(result, hashes...)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
