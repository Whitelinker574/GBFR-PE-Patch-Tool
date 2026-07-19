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
	loadoutShareFormat        = "gbfr-loadout"
	loadoutShareLegacyVersion = 1
	loadoutShareVersion       = 2
	loadoutShareMaxSize       = 1024 * 1024
)

// LoadoutShareSigil 使用“因子本体 + 等级 + 主副词条 + 词条等级”指纹。
// 分享文件不保存 SlotID；导入时在目标存档背包内寻找完全相同的真实物品。
type LoadoutShareSigil struct {
	Index               *int   `json:"index,omitempty"`
	Hash                string `json:"hash"`
	Name                string `json:"name"`
	Level               int    `json:"level"`
	PrimaryTraitHash    string `json:"primaryTraitHash"`
	PrimaryTraitLevel   int    `json:"primaryTraitLevel"`
	SecondaryTraitHash  string `json:"secondaryTraitHash,omitempty"`
	SecondaryTraitLevel int    `json:"secondaryTraitLevel,omitempty"`
}

// LoadoutShareSummon deliberately excludes UnitID and SlotID: those values are
// local to one save. Import resolves this stable fingerprint against the target
// save's real summon inventory.
type LoadoutShareSummon struct {
	TypeHash       string `json:"typeHash"`
	Name           string `json:"name"`
	MainTraitHash  string `json:"mainTraitHash"`
	MainTraitLevel int    `json:"mainTraitLevel"`
	SubParamHash   string `json:"subParamHash"`
	SubParamLevel  int    `json:"subParamLevel"`
	Rank           int    `json:"rank"`
}

type LoadoutShare struct {
	Format        string               `json:"format"`
	Version       int                  `json:"version"`
	CharaHash     string               `json:"charaHash"`
	CharaName     string               `json:"charaName"`
	OwnerCode     string               `json:"ownerCode"`
	Name          string               `json:"name"`
	WeaponHash    string               `json:"weaponHash,omitempty"`
	WeaponName    string               `json:"weaponName,omitempty"`
	Sigils        []LoadoutShareSigil  `json:"sigils"`
	Summons       []LoadoutShareSummon `json:"summons,omitempty"`
	Skills        []LoadoutSkill       `json:"skills"`
	MasteryHashes []string             `json:"masteryHashes"`
}

// LoadoutImportDraft 是导入文件在当前存档中解析后的“草稿”，不会自动写档。
type LoadoutImportDraft struct {
	Name          string   `json:"name"`
	WeaponSlotID  uint32   `json:"weaponSlotId"`
	SigilSlotIDs  []uint32 `json:"sigilSlotIds"`
	SummonSlotIDs []uint32 `json:"summonSlotIds,omitempty"`
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

func buildLoadoutShareSummons(context *LoadoutStatContext) ([]LoadoutShareSummon, error) {
	if context == nil || len(context.EquippedSummonSlotIDs) != 4 {
		count := 0
		if context != nil {
			count = len(context.EquippedSummonSlotIDs)
		}
		return nil, fmt.Errorf("当前召唤石配置不完整：需要 4 个槽位，得到 %d 个", count)
	}
	bySlot := make(map[uint32]LoadoutSummon, len(context.Summons))
	for _, summon := range context.Summons {
		bySlot[summon.SlotID] = summon
	}
	seen := make(map[uint32]bool, 4)
	result := make([]LoadoutShareSummon, 0, 4)
	for index, slotID := range context.EquippedSummonSlotIDs {
		if slotID == 0 || slotID == EmptyHash {
			return nil, fmt.Errorf("当前召唤石配置不完整：第 %d 槽为空", index+1)
		}
		if seen[slotID] {
			return nil, fmt.Errorf("当前召唤石配置重复引用 SlotID %d", slotID)
		}
		seen[slotID] = true
		summon, ok := bySlot[slotID]
		if !ok {
			return nil, fmt.Errorf("当前召唤石配置不完整：第 %d 槽 SlotID %d 没有真实实例", index+1, slotID)
		}
		result = append(result, LoadoutShareSummon{
			TypeHash: summon.TypeHash, Name: summon.Name,
			MainTraitHash: summon.MainTraitHash, MainTraitLevel: summon.MainTraitLevel,
			SubParamHash: summon.SubParamHash, SubParamLevel: summon.SubParamLevel,
			Rank: summon.Rank,
		})
	}
	return result, nil
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
		index := sigil.Index
		share.Sigils = append(share.Sigils, LoadoutShareSigil{
			Index: &index, Hash: sigil.Hash, Name: sigil.Name, Level: sigil.Level,
			PrimaryTraitHash: shareHex(primaryHash), PrimaryTraitLevel: primaryLevel,
			SecondaryTraitHash: shareHex(secondaryHash), SecondaryTraitLevel: secondaryLevel,
		})
	}
	statContext, err := app.LoadoutStatContext(path, source.CharaHash)
	if err != nil {
		return nil, fmt.Errorf("读取召唤石配置失败: %w", err)
	}
	share.Summons, err = buildLoadoutShareSummons(statContext)
	if err != nil {
		return nil, err
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

func sameSharedSummon(pick LoadoutSummon, want LoadoutShareSummon) bool {
	return strings.EqualFold(pick.TypeHash, want.TypeHash) &&
		strings.EqualFold(pick.MainTraitHash, want.MainTraitHash) && pick.MainTraitLevel == want.MainTraitLevel &&
		strings.EqualFold(pick.SubParamHash, want.SubParamHash) && pick.SubParamLevel == want.SubParamLevel &&
		pick.Rank == want.Rank
}

func resolveLoadoutShare(path, expectCharaHash string, share *LoadoutShare) (*LoadoutImportDraft, error) {
	if share == nil || share.Format != loadoutShareFormat ||
		(share.Version != loadoutShareLegacyVersion && share.Version != loadoutShareVersion) {
		return nil, fmt.Errorf("不是受支持的单套配装文件（需要 %s v%d 或 v%d）", loadoutShareFormat, loadoutShareLegacyVersion, loadoutShareVersion)
	}
	if len(share.Summons) > 0 && len(share.Summons) != 4 {
		return nil, fmt.Errorf("分享文件的召唤石指纹需要恰好 4 个，得到 %d 个", len(share.Summons))
	}
	indexedSigils := share.Version == loadoutShareVersion
	if indexedSigils {
		if len(share.Sigils) > loadoutMaxSigils {
			return nil, fmt.Errorf("v%d 配装含 %d 个因子，超过 %d 格上限", share.Version, len(share.Sigils), loadoutMaxSigils)
		}
		seenIndices := make(map[int]bool, len(share.Sigils))
		for i, sigil := range share.Sigils {
			if sigil.Index == nil {
				return nil, fmt.Errorf("v%d 配装的第 %d 个因子缺少原始槽位索引", share.Version, i+1)
			}
			index := *sigil.Index
			if index < 0 || index >= loadoutMaxSigils {
				return nil, fmt.Errorf("v%d 配装的因子槽位索引 %d 越界（应为 0..%d）", share.Version, index, loadoutMaxSigils-1)
			}
			if seenIndices[index] {
				return nil, fmt.Errorf("v%d 配装的因子槽位索引 %d 重复", share.Version, index)
			}
			seenIndices[index] = true
		}
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
	if len(share.Summons) > 0 {
		summonContext, err := app.LoadoutStatContext(path, expectCharaHash)
		if err != nil {
			return nil, fmt.Errorf("读取当前召唤石背包失败: %w", err)
		}
		draft.SummonSlotIDs = make([]uint32, len(share.Summons))
		usedSummons := make(map[uint32]bool, len(share.Summons))
		for index, want := range share.Summons {
			matches := make([]LoadoutSummon, 0, 2)
			for _, summon := range summonContext.Summons {
				if usedSummons[summon.SlotID] || !sameSharedSummon(summon, want) {
					continue
				}
				matches = append(matches, summon)
			}
			if len(matches) == 1 {
				draft.SummonSlotIDs[index] = matches[0].SlotID
				usedSummons[matches[0].SlotID] = true
				continue
			}
			label := strings.TrimSpace(want.Name)
			if label == "" {
				label = want.TypeHash
			}
			if len(matches) == 0 {
				draft.Missing = append(draft.Missing, fmt.Sprintf("召唤石第 %d 槽：%s", index+1, label))
			} else {
				draft.Missing = append(draft.Missing, fmt.Sprintf("召唤石第 %d 槽：%s（存在多个相同实例，映射歧义）", index+1, label))
			}
		}
	}
	if indexedSigils {
		draft.SigilSlotIDs = make([]uint32, loadoutMaxSigils)
	}
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
		if indexedSigils {
			draft.SigilSlotIDs[*want.Index] = matched
		} else {
			draft.SigilSlotIDs = append(draft.SigilSlotIDs, matched)
		}
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
