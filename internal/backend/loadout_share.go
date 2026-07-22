package backend

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
	loadoutShareVersion       = 3
	loadoutShareMaxSize       = 1024 * 1024
)

// LoadoutShareSigil 使用“因子本体 + 等级 + 主副词条 + 词条等级”指纹。
// 分享文件不保存 SlotID；导入时转为原子构造草稿，避免复用已被其他配装占用的物品实例。
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

type LoadoutShareCharacterProgression struct {
	MasterTotalMSP int                             `json:"masterTotalMsp"`
	LegacyProgress int                             `json:"legacyProgress"`
	Weapons        []LoadoutShareProgressionWeapon `json:"weapons,omitempty"`
}

type LoadoutShareProgressionWeapon struct {
	Hash               string `json:"hash"`
	BaseHash           string `json:"baseHash,omitempty"`
	InternalID         string `json:"internalId"`
	Level              int    `json:"level"`
	Uncap              int    `json:"uncap"`
	Mirage             int    `json:"mirage"`
	Awakening          int    `json:"awakening"`
	Transcendence      int    `json:"transcendence"`
	TranscendenceSkill string `json:"transcendenceSkill,omitempty"`
}

type LoadoutShareWeaponState struct {
	StoredHash    string                    `json:"storedHash"`
	XP            uint32                    `json:"xp"`
	Uncap         int                       `json:"uncap"`
	Mirage        int                       `json:"mirage"`
	Awakening     int                       `json:"awakening"`
	Transcendence int                       `json:"transcendence"`
	SkillHashes   []string                  `json:"skillHashes"`
	Wrightstone   *LoadoutWeaponWrightstone `json:"wrightstone,omitempty"`
}

type LoadoutConstructedSummon struct {
	Index int              `json:"index"`
	Name  string           `json:"name"`
	State SummonTraitState `json:"state"`
}

type LoadoutImportApplyPayload struct {
	Character          *LoadoutShareCharacterProgression `json:"character,omitempty"`
	Weapon             *LoadoutShareWeaponState          `json:"weapon,omitempty"`
	ConstructedSummons []LoadoutConstructedSummon        `json:"constructedSummons,omitempty"`
}

type LoadoutShare struct {
	Format        string                            `json:"format"`
	Version       int                               `json:"version"`
	CharaHash     string                            `json:"charaHash"`
	CharaName     string                            `json:"charaName"`
	OwnerCode     string                            `json:"ownerCode"`
	Name          string                            `json:"name"`
	WeaponHash    string                            `json:"weaponHash,omitempty"`
	WeaponName    string                            `json:"weaponName,omitempty"`
	Sigils        []LoadoutShareSigil               `json:"sigils"`
	Summons       []LoadoutShareSummon              `json:"summons,omitempty"`
	Skills        []LoadoutSkill                    `json:"skills"`
	MasteryHashes []string                          `json:"masteryHashes"`
	Character     *LoadoutShareCharacterProgression `json:"character,omitempty"`
	Weapon        *LoadoutShareWeaponState          `json:"weapon,omitempty"`
}

// LoadoutImportDraft 是导入文件在当前存档中解析后的“草稿”，不会自动写档。
type LoadoutImportDraft struct {
	Name              string                     `json:"name"`
	WeaponSlotID      uint32                     `json:"weaponSlotId"`
	SigilSlotIDs      []uint32                   `json:"sigilSlotIds"`
	ConstructedSigils []LoadoutConstructedSigil  `json:"constructedSigils,omitempty"`
	SummonSlotIDs     []uint32                   `json:"summonSlotIds,omitempty"`
	SkillHashes       []string                   `json:"skillHashes"`
	MasteryHashes     []string                   `json:"masteryHashes"`
	ApplyPayload      *LoadoutImportApplyPayload `json:"applyPayload,omitempty"`
	Missing           []string                   `json:"missing"`
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
	share.Character = &LoadoutShareCharacterProgression{
		MasterTotalMSP: statContext.PermanentGrowth.MasterTotalMSP,
		LegacyProgress: statContext.PermanentGrowth.LegacyProgress,
	}
	for _, weapon := range progressionInventory(save, path).Weapons {
		if weapon.OwnerCode != statContext.OwnerCode || weapon.InternalID == "" {
			continue
		}
		share.Character.Weapons = append(share.Character.Weapons, LoadoutShareProgressionWeapon{
			Hash: weapon.Hash, BaseHash: weapon.BaseHash, InternalID: weapon.InternalID,
			Level: weapon.Level, Uncap: weapon.Uncap, Mirage: weapon.Mirage,
			Awakening: weapon.Awakening, Transcendence: weapon.Transcendence,
			TranscendenceSkill: weapon.TranscendenceSkill,
		})
	}
	if source.WeaponSlotID != 0 {
		weapon, weaponErr := readLoadoutWeaponContext(save, source.WeaponSlotID)
		if weaponErr != nil {
			return nil, fmt.Errorf("读取武器强化与祝福失败: %w", weaponErr)
		}
		state := &LoadoutShareWeaponState{
			StoredHash: weapon.StoredHash, XP: weapon.XP, Uncap: weapon.Uncap,
			Mirage: weapon.Mirage, Awakening: weapon.Awakening, Transcendence: weapon.Transcendence,
			Wrightstone: weapon.Wrightstone,
		}
		if extra, ok := save.findUnitExact(weaponExtraIDType, weapon.UnitID); ok && extra.ValueCnt >= 5 {
			for index := 0; index < 5; index++ {
				hash, readErr := extra.Uint32At(index)
				if readErr != nil {
					return nil, fmt.Errorf("读取武器技能槽 %d 失败: %w", index+1, readErr)
				}
				state.SkillHashes = append(state.SkillHashes, hashText(hash))
			}
		} else {
			return nil, fmt.Errorf("武器缺少完整的 2818 五技能向量")
		}
		share.Weapon = state
	}
	return share, nil
}

func sameSharedSummon(pick LoadoutSummon, want LoadoutShareSummon) bool {
	return strings.EqualFold(pick.TypeHash, want.TypeHash) &&
		strings.EqualFold(pick.MainTraitHash, want.MainTraitHash) && pick.MainTraitLevel == want.MainTraitLevel &&
		strings.EqualFold(pick.SubParamHash, want.SubParamHash) && pick.SubParamLevel == want.SubParamLevel &&
		pick.Rank == want.Rank
}

func loadoutShareConstructedSigil(cat *Catalog, want LoadoutShareSigil, index int) (LoadoutConstructedSigil, error) {
	sigilHash, err := ParseHashHex(want.Hash)
	if err != nil {
		return LoadoutConstructedSigil{}, fmt.Errorf("因子 %q 的哈希无效: %w", want.Name, err)
	}
	primaryHash, err := ParseHashHex(want.PrimaryTraitHash)
	if err != nil {
		return LoadoutConstructedSigil{}, fmt.Errorf("因子 %s 的主词条哈希无效: %w", want.Name, err)
	}
	primary := cat.LookupTraitByHash(primaryHash)
	if primary == nil {
		return LoadoutConstructedSigil{}, fmt.Errorf("因子 %s 的主词条 0x%08X 不在当前游戏目录中", want.Name, primaryHash)
	}
	sigil := cat.LookupSigilByHash(sigilHash)
	if sigil == nil {
		// 组合因子可能使用 gem.tbl 中没有的实例哈希；按主词条、等级和加号槽形态还原目录本体。
		var matches []*SigilDef
		wantsSecondary := strings.TrimSpace(want.SecondaryTraitHash) != ""
		for candidateIndex := range cat.Sigils {
			candidate := &cat.Sigils[candidateIndex]
			candidatePrimary, requireErr := cat.RequireTrait(candidate.PrimaryTraitID)
			if requireErr != nil {
				continue
			}
			candidatePrimaryHash, parseErr := ParseHashHex(candidatePrimary.Hash)
			if parseErr != nil || candidatePrimaryHash != primaryHash || supportsGeneratedPlusSigil(candidate) != wantsSecondary {
				continue
			}
			if len(candidate.AllowedSigilLevels) > 0 && !containsNaturalLevel(candidate.AllowedSigilLevels, want.Level) {
				continue
			}
			matches = append(matches, candidate)
		}
		if len(matches) != 1 {
			return LoadoutConstructedSigil{}, fmt.Errorf("因子 %s（0x%08X）不在独立目录中，且按主词条 0x%08X / Lv%d 解析到 %d 个候选", want.Name, sigilHash, primaryHash, want.Level, len(matches))
		}
		sigil = matches[0]
	}
	item := QueueItem{
		SigilID: sigil.InternalID, SigilName: displaySigilName(sigil), Level: want.Level,
		PrimaryTraitID: primary.InternalID, PrimaryTraitName: cnTrait(primary.DisplayName), PrimaryLevel: want.PrimaryTraitLevel,
		Quantity: 1,
	}
	if strings.TrimSpace(want.SecondaryTraitHash) != "" {
		secondaryHash, parseErr := ParseHashHex(want.SecondaryTraitHash)
		if parseErr != nil {
			return LoadoutConstructedSigil{}, fmt.Errorf("因子 %s 的副词条哈希无效: %w", want.Name, parseErr)
		}
		secondary := cat.LookupTraitByHash(secondaryHash)
		if secondary == nil {
			return LoadoutConstructedSigil{}, fmt.Errorf("因子 %s 的副词条 0x%08X 不在当前游戏目录中", want.Name, secondaryHash)
		}
		item.SecondaryTraitID = secondary.InternalID
		item.SecondaryTraitName = cnTrait(secondary.DisplayName)
		item.SecondaryLevel = want.SecondaryTraitLevel
	}
	draft := LoadoutConstructedSigil{Index: index, Item: item}
	if _, err := prepareLoadoutSigil(cat, draft); err != nil {
		return LoadoutConstructedSigil{}, fmt.Errorf("因子第 %d 格无法构造: %w", index+1, err)
	}
	return draft, nil
}

func resolveLoadoutShare(path, expectCharaHash string, share *LoadoutShare) (*LoadoutImportDraft, error) {
	if share == nil || share.Format != loadoutShareFormat || share.Version < loadoutShareLegacyVersion || share.Version > loadoutShareVersion {
		return nil, fmt.Errorf("不是受支持的单套配装文件（需要 %s v%d..v%d）", loadoutShareFormat, loadoutShareLegacyVersion, loadoutShareVersion)
	}
	if len(share.Summons) > 0 && len(share.Summons) != 4 {
		return nil, fmt.Errorf("分享文件的召唤石指纹需要恰好 4 个，得到 %d 个", len(share.Summons))
	}
	indexedSigils := share.Version >= 2
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
	if share.Version >= 3 {
		draft.ApplyPayload = &LoadoutImportApplyPayload{Character: share.Character, Weapon: share.Weapon}
	}
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
			if len(matches) >= 1 {
				draft.SummonSlotIDs[index] = matches[0].SlotID
				usedSummons[matches[0].SlotID] = true
				continue
			}
			label := strings.TrimSpace(want.Name)
			if label == "" {
				label = want.TypeHash
			}
			if share.Version >= 3 && draft.ApplyPayload != nil {
				typeHash, typeErr := ParseHashHex(want.TypeHash)
				mainHash, mainErr := ParseHashHex(want.MainTraitHash)
				subHash, subErr := ParseHashHex(want.SubParamHash)
				if typeErr != nil || mainErr != nil || subErr != nil || want.MainTraitLevel < 0 || want.SubParamLevel < 0 || want.Rank < 0 {
					return nil, fmt.Errorf("召唤石第 %d 槽的分享字段无效", index+1)
				}
				draft.ApplyPayload.ConstructedSummons = append(draft.ApplyPayload.ConstructedSummons, LoadoutConstructedSummon{
					Index: index, Name: label, State: SummonTraitState{TypeHash: typeHash, MainTraitHash: mainHash, SubParamHash: subHash,
						MainTraitLevel: uint32(want.MainTraitLevel), SubParamLevel: uint32(want.SubParamLevel), Rank: uint32(want.Rank)},
				})
			} else {
				draft.Missing = append(draft.Missing, fmt.Sprintf("召唤石第 %d 槽：%s", index+1, label))
			}
		}
	}
	if indexedSigils {
		draft.SigilSlotIDs = make([]uint32, loadoutMaxSigils)
	} else {
		draft.SigilSlotIDs = make([]uint32, len(share.Sigils))
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

	cat, err := LoadCatalog()
	if err != nil {
		return nil, err
	}
	for fallbackIndex, want := range share.Sigils {
		index := fallbackIndex
		if indexedSigils {
			index = *want.Index
		}
		constructed, err := loadoutShareConstructedSigil(cat, want, index)
		if err != nil {
			return nil, err
		}
		draft.ConstructedSigils = append(draft.ConstructedSigils, constructed)
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

// LoadoutImport 读取一份单套配装；因子转为独立构造草稿，其他资源映射到当前存档。
// 本方法只返回草稿，不自动写档。
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
