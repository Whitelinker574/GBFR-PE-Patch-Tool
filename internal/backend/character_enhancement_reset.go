package backend

import (
	"bytes"
	"fmt"
	"math/bits"
	"os"
	"sort"
)

const (
	enhancementResetAll     = "all"
	enhancementResetGranted = "granted"
)

type CharacterEnhancementSummary struct {
	Hash               string `json:"hash"`
	NodeGroups         int    `json:"nodeGroups"`
	Learned            int    `json:"learned"`
	Granted            int    `json:"granted"`
	ChangedFromGranted int    `json:"changedFromGranted"`
}

type CharacterEnhancementSnapshot struct {
	Path       string                        `json:"path"`
	Revision   string                        `json:"revision"`
	Characters []CharacterEnhancementSummary `json:"characters"`
}

type CharacterEnhancementResetRequest struct {
	Path             string `json:"path"`
	ExpectedRevision string `json:"expectedRevision"`
	CharacterHash    string `json:"characterHash"`
	Mode             string `json:"mode"`
}

type CharacterEnhancementResetResult struct {
	Path       string `json:"path"`
	BackupPath string `json:"backupPath"`
	Changed    int    `json:"changed"`
	Verified   int    `json:"verified"`
	Revision   string `json:"revision"`
}

// The acquisition mask is the low byte, not the complete 1602 word.
// The next byte retains grant provenance. Native evidence and the real
// A/A0/B/B0 purchase comparison are recorded in docs/evidence/issue26-*.
func enhancementResetValue(value uint32, mode string) (uint32, error) {
	switch mode {
	case enhancementResetAll:
		return value &^ 0xFF, nil
	case enhancementResetGranted:
		return (value &^ 0xFF) | ((value >> 8) & 0xFF), nil
	default:
		return 0, fmt.Errorf("请选择全部取消学习或恢复自动解锁")
	}
}

func characterEnhancementEntries(save *SaveData, hash uint32) ([]*unitEntry, error) {
	if _, known := characterNameByHash[hash]; !known {
		return nil, fmt.Errorf("角色 %08X 不在可玩角色目录中", hash)
	}
	unit, err := loadoutCharacterUnitForHash(save, hash)
	if err != nil {
		return nil, err
	}
	if unit < 10000 || unit >= 10100 {
		return nil, fmt.Errorf("角色记录编号 %d 不在已确认的范围内", unit)
	}
	base := unit * 1000
	hashes := make(map[uint32]*unitEntry)
	for _, entry := range save.findAllUnitsByType(1601) {
		if entry.UnitID < base || entry.UnitID >= base+1000 {
			continue
		}
		if entry.ValueCnt != 1 || hashes[entry.UnitID] != nil {
			return nil, fmt.Errorf("角色强化节点标识重复或结构异常，请保留原档")
		}
		hashes[entry.UnitID] = entry
	}
	var entries []*unitEntry
	seen := make(map[uint32]bool)
	for _, entry := range save.findAllUnitsByType(1602) {
		if entry.UnitID < base || entry.UnitID >= base+1000 {
			continue
		}
		if entry.ValueCnt != 1 || seen[entry.UnitID] || hashes[entry.UnitID] == nil {
			return nil, fmt.Errorf("角色强化节点缺少标识、重复或结构异常，请保留原档")
		}
		seen[entry.UnitID] = true
		entries = append(entries, entry)
	}
	if len(entries) == 0 || len(entries) != len(hashes) {
		return nil, fmt.Errorf("角色强化节点记录不完整，未进行修改")
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].UnitID < entries[j].UnitID })
	return entries, nil
}

func (a *App) CharacterEnhancementInspect(path string) (*CharacterEnhancementSnapshot, error) {
	absolute, raw, err := readFateEpisodeRaw(path)
	if err != nil {
		return nil, err
	}
	save, err := newSaveData(absolute, raw)
	if err != nil {
		return nil, err
	}
	result := &CharacterEnhancementSnapshot{Path: absolute, Revision: fateEpisodeRevision(raw), Characters: []CharacterEnhancementSummary{}}
	seen := make(map[uint32]bool)
	for _, identity := range save.findAllUnitsByType(1301) {
		hash := identity.Uint32()
		if _, known := characterNameByHash[hash]; !known || seen[hash] {
			continue
		}
		seen[hash] = true
		entries, err := characterEnhancementEntries(save, hash)
		if err != nil {
			return nil, err
		}
		row := CharacterEnhancementSummary{Hash: fmt.Sprintf("%08X", hash), NodeGroups: len(entries)}
		for _, entry := range entries {
			value := entry.Uint32()
			row.Learned += bits.OnesCount8(uint8(value))
			row.Granted += bits.OnesCount8(uint8(value >> 8))
			if byte(value) != byte(value>>8) {
				row.ChangedFromGranted++
			}
		}
		result.Characters = append(result.Characters, row)
	}
	sort.Slice(result.Characters, func(i, j int) bool { return result.Characters[i].Hash < result.Characters[j].Hash })
	return result, nil
}

func resetCharacterEnhancementLocked(request CharacterEnhancementResetRequest, loadForVerification func(string) (*SaveData, error)) (*CharacterEnhancementResetResult, error) {
	if _, err := enhancementResetValue(0, request.Mode); err != nil {
		return nil, err
	}
	revision, err := validateFateEpisodeRevision(request.ExpectedRevision)
	if err != nil {
		return nil, err
	}
	absolute, original, err := readFateEpisodeRaw(request.Path)
	if err != nil {
		return nil, err
	}
	if fateEpisodeRevision(original) != revision {
		return nil, fmt.Errorf("存档已发生变化，请重新读取后再重置")
	}
	hash, err := ParseHashHex(request.CharacterHash)
	if err != nil {
		return nil, err
	}
	save, err := newSaveData(absolute, append([]byte(nil), original...))
	if err != nil {
		return nil, err
	}
	entries, err := characterEnhancementEntries(save, hash)
	if err != nil {
		return nil, err
	}
	changed := 0
	expectedValues := make([]uint32, len(entries))
	for i, entry := range entries {
		value, _ := enhancementResetValue(entry.Uint32(), request.Mode)
		expectedValues[i] = value
		if value != entry.Uint32() {
			entry.SetUint32(value)
			changed++
		}
	}
	result := &CharacterEnhancementResetResult{Path: absolute, Revision: revision, Changed: changed, Verified: len(entries)}
	if changed == 0 {
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
		return nil, fmt.Errorf("写入前存档又发生了变化，已取消重置")
	}
	if err := save.Write(absolute); err != nil {
		return nil, err
	}
	result.BackupPath = save.LastBackupPath()
	if loadForVerification == nil {
		loadForVerification = LoadSave
	}
	verify, verifyErr := loadForVerification(absolute)
	if verifyErr == nil && (verify == nil || !bytes.Equal(verify.data, save.data)) {
		verifyErr = fmt.Errorf("重新打开的存档与本次预期不一致")
	}
	if verifyErr == nil {
		var actual []*unitEntry
		actual, verifyErr = characterEnhancementEntries(verify, hash)
		if verifyErr == nil && len(actual) != len(entries) {
			verifyErr = fmt.Errorf("强化节点数量回读不一致")
		}
		if verifyErr == nil {
			for i, entry := range actual {
				if entry.UnitID != entries[i].UnitID || entry.Uint32() != expectedValues[i] {
					verifyErr = fmt.Errorf("强化节点 %d 回读不一致", entry.UnitID)
					break
				}
			}
		}
	}
	if verifyErr != nil {
		current, err := os.ReadFile(absolute)
		if err != nil || !bytes.Equal(current, save.data) {
			return nil, fmt.Errorf("重置回读失败：%v；文件又发生变化，未覆盖，请从备份恢复：%s", verifyErr, result.BackupPath)
		}
		if err := rollbackSaveDiffTarget(absolute, original); err != nil {
			return nil, fmt.Errorf("重置回读失败：%v；恢复失败，请从备份恢复：%s：%w", verifyErr, result.BackupPath, err)
		}
		return nil, fmt.Errorf("重置回读失败，已恢复原档：%w", verifyErr)
	}
	result.Revision = fateEpisodeRevision(save.data)
	return result, nil
}

func (a *App) ResetCharacterEnhancement(request CharacterEnhancementResetRequest) (*CharacterEnhancementResetResult, error) {
	offlineSaveMutationMu.Lock()
	defer offlineSaveMutationMu.Unlock()
	if err := ensureGeneratorWriteAllowed(request.Path); err != nil {
		return nil, err
	}
	return resetCharacterEnhancementLocked(request, LoadSave)
}
