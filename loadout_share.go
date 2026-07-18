package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	loadoutShareFormat  = "gbfr-loadout"
	loadoutShareVersion = 1
	loadoutShareMaxSize = 1024 * 1024
)

// LoadoutShareSigil 使用“因子本体 + 等级 + 主副词条 + 词条等级”指纹。
// 分享文件不保存 SlotID；导入时在目标存档背包内寻找完全相同的真实物品。
type LoadoutShareSigil struct {
	Hash                string `json:"hash"`
	Name                string `json:"name"`
	Level               int    `json:"level"`
	PrimaryTraitHash    string `json:"primaryTraitHash"`
	PrimaryTraitLevel   int    `json:"primaryTraitLevel"`
	SecondaryTraitHash  string `json:"secondaryTraitHash,omitempty"`
	SecondaryTraitLevel int    `json:"secondaryTraitLevel,omitempty"`
}

type LoadoutShare struct {
	Format        string              `json:"format"`
	Version       int                 `json:"version"`
	CharaHash     string              `json:"charaHash"`
	CharaName     string              `json:"charaName"`
	OwnerCode     string              `json:"ownerCode"`
	Name          string              `json:"name"`
	WeaponHash    string              `json:"weaponHash,omitempty"`
	WeaponName    string              `json:"weaponName,omitempty"`
	Sigils        []LoadoutShareSigil `json:"sigils"`
	Skills        []LoadoutSkill      `json:"skills"`
	MasteryHashes []string            `json:"masteryHashes"`
}

// LoadoutImportDraft 是导入文件在当前存档中解析后的“草稿”，不会自动写档。
type LoadoutImportDraft struct {
	Name          string   `json:"name"`
	WeaponSlotID  uint32   `json:"weaponSlotId"`
	SigilSlotIDs  []uint32 `json:"sigilSlotIds"`
	SkillHashes   []string `json:"skillHashes"`
	MasteryHashes []string `json:"masteryHashes"`
	Missing       []string `json:"missing"`
}

func shareHex(value uint32) string {
	if value == 0 || value == EmptyHash {
		return ""
	}
	return fmt.Sprintf("%08X", value)
}

func buildLoadoutShare(path string, unitID uint32) (*LoadoutShare, error) {
	app := &App{}
	groups, err := app.LoadoutList(path)
	if err != nil {
		return nil, err
	}
	var source *LoadoutEntry
	for gi := range groups {
		for li := range groups[gi].Loadouts {
			item := &groups[gi].Loadouts[li]
			if item.UnitID == unitID && !item.IsParty {
				source = item
				break
			}
		}
	}
	if source == nil {
		return nil, fmt.Errorf("没有找到可导出的配装槽 UnitID=%d", unitID)
	}

	save, err := LoadSave(path)
	if err != nil {
		return nil, err
	}
	ix := buildLoadoutIndex(save)
	charaHash, err := ParseHashHex(source.CharaHash)
	if err != nil {
		return nil, err
	}
	share := &LoadoutShare{
		Format: loadoutShareFormat, Version: loadoutShareVersion,
		CharaHash: source.CharaHash, CharaName: source.CharaName,
		OwnerCode: ix.deriveOwnerCode(save, charaHash), Name: source.Name,
		WeaponHash: source.WeaponHash, WeaponName: source.WeaponName,
		Skills: append([]LoadoutSkill(nil), source.Skills...),
	}
	for _, node := range source.Mastery {
		share.MasteryHashes = append(share.MasteryHashes, node.Hash)
	}
	for _, sigil := range source.Sigils {
		gemUnitID, ok := ix.gemBySlotID[sigil.SlotID]
		if !ok {
			return nil, fmt.Errorf("因子 %s 的存档槽引用 %d 已失效，无法导出", sigil.Name, sigil.SlotID)
		}
		primaryHash, primaryLevel, secondaryHash, secondaryLevel := readSigilTraits(save, gemUnitID)
		share.Sigils = append(share.Sigils, LoadoutShareSigil{
			Hash: sigil.Hash, Name: sigil.Name, Level: sigil.Level,
			PrimaryTraitHash: shareHex(primaryHash), PrimaryTraitLevel: primaryLevel,
			SecondaryTraitHash: shareHex(secondaryHash), SecondaryTraitLevel: secondaryLevel,
		})
	}
	return share, nil
}

func sameSharedSigil(save *SaveData, ix *loadoutIndex, pick LoadoutPickSigil, want LoadoutShareSigil) bool {
	if !strings.EqualFold(pick.Hash, want.Hash) || pick.Level != want.Level {
		return false
	}
	gemUnitID, ok := ix.gemBySlotID[pick.SlotID]
	if !ok {
		return false
	}
	primaryHash, primaryLevel, secondaryHash, secondaryLevel := readSigilTraits(save, gemUnitID)
	return strings.EqualFold(shareHex(primaryHash), want.PrimaryTraitHash) && primaryLevel == want.PrimaryTraitLevel &&
		strings.EqualFold(shareHex(secondaryHash), want.SecondaryTraitHash) && secondaryLevel == want.SecondaryTraitLevel
}

func resolveLoadoutShare(path, expectCharaHash string, share *LoadoutShare) (*LoadoutImportDraft, error) {
	if share == nil || share.Format != loadoutShareFormat || share.Version != loadoutShareVersion {
		return nil, fmt.Errorf("不是受支持的单套配装文件（需要 %s v%d）", loadoutShareFormat, loadoutShareVersion)
	}
	app := &App{}
	ctx, err := app.LoadoutEditContext(path, expectCharaHash)
	if err != nil {
		return nil, err
	}
	if share.OwnerCode != "" && ctx.OwnerCode != "" && share.OwnerCode != ctx.OwnerCode {
		return nil, fmt.Errorf("配装属于 %s（%s），当前角色是 %s（%s）", share.CharaName, share.OwnerCode, ctx.CharaName, ctx.OwnerCode)
	}
	if share.OwnerCode == "" && !strings.EqualFold(share.CharaHash, expectCharaHash) {
		return nil, fmt.Errorf("配装角色 %s 与当前角色 %s 不一致", share.CharaName, ctx.CharaName)
	}

	draft := &LoadoutImportDraft{Name: share.Name}
	if share.WeaponHash != "" {
		for _, weapon := range ctx.Weapons {
			if strings.EqualFold(weapon.Hash, share.WeaponHash) {
				draft.WeaponSlotID = weapon.SlotID
				break
			}
		}
		if draft.WeaponSlotID == 0 {
			draft.Missing = append(draft.Missing, "武器："+share.WeaponName)
		}
	}

	save, err := LoadSave(path)
	if err != nil {
		return nil, err
	}
	ix := buildLoadoutIndex(save)
	used := map[uint32]bool{}
	for _, want := range share.Sigils {
		matched := uint32(0)
		for _, pick := range ctx.Sigils {
			if used[pick.SlotID] || !sameSharedSigil(save, ix, pick, want) {
				continue
			}
			matched = pick.SlotID
			break
		}
		if matched == 0 {
			draft.Missing = append(draft.Missing, fmt.Sprintf("因子：%s Lv%d", want.Name, want.Level))
			continue
		}
		used[matched] = true
		draft.SigilSlotIDs = append(draft.SigilSlotIDs, matched)
	}

	for _, skill := range share.Skills {
		hash, parseErr := ParseHashHex(skill.Hash)
		if parseErr != nil || !skillBelongsToOwner(hash, ctx.OwnerCode) {
			draft.Missing = append(draft.Missing, "技能："+skill.Name)
			continue
		}
		draft.SkillHashes = append(draft.SkillHashes, strings.ToUpper(skill.Hash))
	}
	mastery := make([]uint32, 0, len(share.MasteryHashes))
	for _, value := range share.MasteryHashes {
		hash, parseErr := ParseHashHex(value)
		if parseErr != nil {
			return nil, fmt.Errorf("分享文件含无效专精节点 %q", value)
		}
		mastery = append(mastery, hash)
	}
	if _, err := validateMasteryQuota(mastery, ctx.OwnerCode, len(mastery) > 0); err != nil {
		return nil, fmt.Errorf("分享文件的专精配置无效: %w", err)
	}
	for _, hash := range mastery {
		draft.MasteryHashes = append(draft.MasteryHashes, fmt.Sprintf("%08X", hash))
	}
	return draft, nil
}

func safeLoadoutFilename(name string) string {
	name = strings.TrimSpace(name)
	name = strings.NewReplacer("\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(name)
	if name == "" {
		return "GBFR配装"
	}
	return name
}

// LoadoutExport 只导出当前角色的一个预设槽，绝不包含批量或整份存档数据。
func (a *App) LoadoutExport(savePath string, unitID uint32) (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("Wails 上下文未初始化")
	}
	share, err := buildLoadoutShare(savePath, unitID)
	if err != nil {
		return "", err
	}
	outputPath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title: "导出当前单套配装", DefaultDirectory: filepath.Dir(savePath),
		DefaultFilename: safeLoadoutFilename(share.CharaName+"_"+share.Name) + ".gbfr-loadout.json",
		Filters:         []runtime.FileFilter{{DisplayName: "GBFR 单套配装 (*.gbfr-loadout.json)", Pattern: "*.gbfr-loadout.json"}},
	})
	if err != nil || outputPath == "" {
		return outputPath, err
	}
	payload, err := json.MarshalIndent(share, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(outputPath, payload, 0600); err != nil {
		return "", err
	}
	return outputPath, nil
}

// LoadoutImport 读取一份单套配装并映射到当前存档已有资源，只返回草稿，不自动写档。
func (a *App) LoadoutImport(savePath, expectCharaHash string) (*LoadoutImportDraft, error) {
	if a.ctx == nil {
		return nil, fmt.Errorf("Wails 上下文未初始化")
	}
	inputPath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "导入单套配装", Filters: []runtime.FileFilter{{DisplayName: "GBFR 单套配装 (*.gbfr-loadout.json)", Pattern: "*.gbfr-loadout.json"}},
	})
	if err != nil || inputPath == "" {
		return nil, err
	}
	info, err := os.Stat(inputPath)
	if err != nil {
		return nil, err
	}
	if info.Size() > loadoutShareMaxSize {
		return nil, fmt.Errorf("配装文件超过 1 MiB，拒绝读取")
	}
	payload, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, err
	}
	var share LoadoutShare
	if err := json.Unmarshal(payload, &share); err != nil {
		return nil, fmt.Errorf("配装 JSON 无效: %w", err)
	}
	return resolveLoadoutShare(savePath, expectCharaHash, &share)
}
