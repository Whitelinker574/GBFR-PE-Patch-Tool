package main

import (
	"fmt"
	"path/filepath"
)

// ── 前端交互数据结构 ──

type SigilInfo struct {
	InternalID             string   `json:"internalId"`
	Hash                   string   `json:"hash"`
	DisplayName            string   `json:"displayName"`
	SupportsSecondaryTrait bool     `json:"supportsSecondaryTrait"`
	AllowedSigilLevels     []int    `json:"allowedSigilLevels"`
	DefaultSigilLevel      int      `json:"defaultSigilLevel"`
	PrimaryTraitID         string   `json:"primaryTraitId"`
	PrimaryTraitName       string   `json:"primaryTraitName"`
	AllowedFirstTraitLevels []int   `json:"allowedFirstTraitLevels"`
	FirstTraitMaxLevel     int      `json:"firstTraitMaxLevel"`
}

type TraitInfo struct {
	InternalID   string `json:"internalId"`
	Hash         string `json:"hash"`
	DisplayName  string `json:"displayName"`
	MaxLevel     int    `json:"maxLevel"`
	AllowedLevels []int  `json:"allowedLevels"`
}

type SaveInfo struct {
	Path         string `json:"path"`
	OccupiedSigils int  `json:"occupiedSigils"`
	MaxSlotID    int    `json:"maxSlotId"`
}

type QueueItem struct {
	SigilID            string `json:"sigilId"`
	SigilName          string `json:"sigilName"`
	Level              int    `json:"level"`
	PrimaryTraitID     string `json:"primaryTraitId"`
	PrimaryTraitName   string `json:"primaryTraitName"`
	PrimaryLevel       int    `json:"primaryLevel"`
	SecondaryTraitID   string `json:"secondaryTraitId"`
	SecondaryTraitName string `json:"secondaryTraitName"`
	SecondaryLevel     int    `json:"secondaryLevel"`
	Quantity           int    `json:"quantity"`
}

type ApplyResult struct {
	CreatedCount int    `json:"createdCount"`
	VerifiedCount int   `json:"verifiedCount"`
	OutputPath   string `json:"outputPath"`
}

// ── SigilGen 主体 ──

type SigilGen struct {
	catalog  *Catalog
	save     *SaveData
	savePath string
	queue    []QueueItem
}

func NewSigilGen() *SigilGen {
	return &SigilGen{}
}

// LoadCatalog 加载数据目录（从嵌入的 JSON 文件）
func (sg *SigilGen) LoadCatalog() error {
	c, err := LoadCatalog()
	if err != nil {
		return err
	}
	sg.catalog = c
	return nil
}

// GetSigilList 返回排序后的因子列表
func (sg *SigilGen) GetSigilList() ([]SigilInfo, error) {
	if sg.catalog == nil {
		if err := sg.LoadCatalog(); err != nil {
			return nil, err
		}
	}
	sorted := sg.catalog.GetSigilSortedList()
	result := make([]SigilInfo, len(sorted))
	for i, s := range sorted {
		result[i] = SigilInfo{
			InternalID:              s.InternalID,
			Hash:                    s.Hash,
			DisplayName:             s.DisplayName,
			SupportsSecondaryTrait:  s.SupportsSecondaryTrait != nil && *s.SupportsSecondaryTrait,
			AllowedSigilLevels:      s.AllowedSigilLevels,
			DefaultSigilLevel:       derefInt(s.DefaultSigilLevel),
			PrimaryTraitID:          s.PrimaryTraitID,
			PrimaryTraitName:        derefStr(s.PrimaryTraitName),
			AllowedFirstTraitLevels: s.AllowedFirstTraitLevels,
			FirstTraitMaxLevel:      derefInt(s.FirstTraitMaxLevel),
		}
	}
	return result, nil
}

// GetTraitList 返回所有特性
func (sg *SigilGen) GetTraitList() ([]TraitInfo, error) {
	if sg.catalog == nil {
		if err := sg.LoadCatalog(); err != nil {
			return nil, err
		}
	}
	result := make([]TraitInfo, len(sg.catalog.Traits))
	for i, t := range sg.catalog.Traits {
		result[i] = TraitInfo{
			InternalID:    t.InternalID,
			Hash:          t.Hash,
			DisplayName:   t.DisplayName,
			MaxLevel:      derefInt(t.MaxLevel),
			AllowedLevels: t.AllowedLevels,
		}
	}
	return result, nil
}

// GetCompatibleSecondaryTraits 返回可选副特性列表
func (sg *SigilGen) GetCompatibleSecondaryTraits(sigilID string) ([]TraitInfo, error) {
	if sg.catalog == nil {
		if err := sg.LoadCatalog(); err != nil {
			return nil, err
		}
	}
	sigil, err := sg.catalog.RequireSigil(sigilID)
	if err != nil {
		return nil, err
	}

	secondaries, err := sg.catalog.GetAllowedSecondaryTraits(sigil)
	if err != nil {
		return nil, err
	}

	result := make([]TraitInfo, len(secondaries))
	for i, t := range secondaries {
		result[i] = TraitInfo{
			InternalID:    t.InternalID,
			Hash:          t.Hash,
			DisplayName:   t.DisplayName,
			MaxLevel:      derefInt(t.MaxLevel),
			AllowedLevels: t.AllowedLevels,
		}
	}
	return result, nil
}

// GetAllowedLevels 返回因子可选等级
func (sg *SigilGen) GetAllowedLevels(sigilID string) ([]int, error) {
	if sg.catalog == nil {
		if err := sg.LoadCatalog(); err != nil {
			return nil, err
		}
	}
	sigil, err := sg.catalog.RequireSigil(sigilID)
	if err != nil {
		return nil, err
	}
	return sg.catalog.RequireSigilLevels(sigil)
}

// GetPrimaryTraitLevels 返回主特性可选等级
func (sg *SigilGen) GetPrimaryTraitLevels(sigilID string) ([]int, error) {
	if sg.catalog == nil {
		if err := sg.LoadCatalog(); err != nil {
			return nil, err
		}
	}
	sigil, err := sg.catalog.RequireSigil(sigilID)
	if err != nil {
		return nil, err
	}
	return sg.catalog.RequirePrimaryTraitLevels(sigil)
}

// GetSecondaryTraitLevels 返回副特性可选等级
func (sg *SigilGen) GetSecondaryTraitLevels(sigilID, traitID string) ([]int, error) {
	if sg.catalog == nil {
		if err := sg.LoadCatalog(); err != nil {
			return nil, err
		}
	}
	sigil, err := sg.catalog.RequireSigil(sigilID)
	if err != nil {
		return nil, err
	}
	trait, err := sg.catalog.RequireTrait(traitID)
	if err != nil {
		return nil, err
	}
	return sg.catalog.RequireSecondaryTraitLevels(sigil, trait)
}

// GetDefaultSecondaryTrait 返回因子的默认副特性
func (sg *SigilGen) GetDefaultSecondaryTrait(sigilID string) (*TraitInfo, error) {
	if sg.catalog == nil {
		if err := sg.LoadCatalog(); err != nil {
			return nil, err
		}
	}
	sigil, err := sg.catalog.RequireSigil(sigilID)
	if err != nil {
		return nil, err
	}
	t := sg.catalog.GetDefaultSecondaryTrait(sigil)
	if t == nil {
		return nil, nil
	}
	return &TraitInfo{
		InternalID:    t.InternalID,
		Hash:          t.Hash,
		DisplayName:   t.DisplayName,
		MaxLevel:      derefInt(t.MaxLevel),
		AllowedLevels: t.AllowedLevels,
	}, nil
}

// GetPrimaryTrait 返回因子的主特性
func (sg *SigilGen) GetPrimaryTrait(sigilID string) (*TraitInfo, error) {
	if sg.catalog == nil {
		if err := sg.LoadCatalog(); err != nil {
			return nil, err
		}
	}
	sigil, err := sg.catalog.RequireSigil(sigilID)
	if err != nil {
		return nil, err
	}
	trait, err := sg.catalog.RequireTrait(sigil.PrimaryTraitID)
	if err != nil {
		return nil, err
	}
	return &TraitInfo{
		InternalID:    trait.InternalID,
		Hash:          trait.Hash,
		DisplayName:   trait.DisplayName,
		MaxLevel:      derefInt(trait.MaxLevel),
		AllowedLevels: trait.AllowedLevels,
	}, nil
}

// ── 存档操作 ──

func (sg *SigilGen) LoadSaveFile(path string) (*SaveInfo, error) {
	s, err := LoadSave(path)
	if err != nil {
		return nil, err
	}
	sg.save = s
	sg.savePath = path

	info := &SaveInfo{Path: path, OccupiedSigils: s.GetOccupiedGemCount()}
	if maxID, err := s.GetMaxSlotID(); err == nil {
		info.MaxSlotID = maxID
	}
	return info, nil
}

func (sg *SigilGen) GetLoadedSaveInfo() (*SaveInfo, error) {
	if sg.save == nil {
		return nil, fmt.Errorf("未加载存档")
	}
	info := &SaveInfo{Path: sg.savePath, OccupiedSigils: sg.save.GetOccupiedGemCount()}
	if maxID, err := sg.save.GetMaxSlotID(); err == nil {
		info.MaxSlotID = maxID
	}
	return info, nil
}

// ── 队列操作 ──

func (sg *SigilGen) GetQueue() []QueueItem {
	if sg.queue == nil {
		return []QueueItem{}
	}
	return sg.queue
}

func (sg *SigilGen) AddToQueue(item QueueItem) error {
	if sg.catalog == nil {
		if err := sg.LoadCatalog(); err != nil {
			return err
		}
	}
	if item.Quantity <= 0 {
		return fmt.Errorf("数量至少为 1")
	}

	sigil, err := sg.catalog.RequireSigil(item.SigilID)
	if err != nil {
		return err
	}
	item.SigilName = sigil.DisplayName

	// 验证因子等级
	levels, err := sg.catalog.RequireSigilLevels(sigil)
	if err != nil {
		return err
	}
	if !containsInt(levels, item.Level) {
		return fmt.Errorf("%s 不允许因子等级 %d", sigil.DisplayName, item.Level)
	}

	// 验证主特性
	primaryTrait, err := sg.catalog.RequireTrait(sigil.PrimaryTraitID)
	if err != nil {
		return err
	}
	item.PrimaryTraitID = primaryTrait.InternalID
	item.PrimaryTraitName = primaryTrait.DisplayName

	primaryLevels, err := sg.catalog.RequirePrimaryTraitLevels(sigil)
	if err != nil {
		return err
	}
	if !containsInt(primaryLevels, item.PrimaryLevel) {
		return fmt.Errorf("%s 不允许主特性等级 %d", primaryTrait.DisplayName, item.PrimaryLevel)
	}

	// 验证副特性
	supports := sigil.SupportsSecondaryTrait != nil && *sigil.SupportsSecondaryTrait
	if supports {
		if item.SecondaryTraitID == "" {
			return fmt.Errorf("%s 需要选择副特性", sigil.DisplayName)
		}
		secondaryTrait, err := sg.catalog.RequireTrait(item.SecondaryTraitID)
		if err != nil {
			return err
		}
		item.SecondaryTraitName = secondaryTrait.DisplayName

		// 验证 secondary trait is allowed
		allowed, _ := sg.catalog.GetAllowedSecondaryTraits(sigil)
		found := false
		for _, a := range allowed {
			if a.InternalID == item.SecondaryTraitID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%s 不是 %s 的已验证副特性", secondaryTrait.DisplayName, sigil.DisplayName)
		}

		secondaryLevels, err := sg.catalog.RequireSecondaryTraitLevels(sigil, secondaryTrait)
		if err != nil {
			return err
		}
		if !containsInt(secondaryLevels, item.SecondaryLevel) {
			return fmt.Errorf("%s 不允许副特性等级 %d", secondaryTrait.DisplayName, item.SecondaryLevel)
		}
	} else if item.SecondaryTraitID != "" {
		return fmt.Errorf("%s 不支持副特性", sigil.DisplayName)
	}

	if sg.queue == nil {
		sg.queue = []QueueItem{}
	}
	sg.queue = append(sg.queue, item)
	return nil
}

func (sg *SigilGen) RemoveFromQueue(index int) error {
	if index < 0 || index >= len(sg.queue) {
		return fmt.Errorf("无效的队列索引: %d", index)
	}
	sg.queue = append(sg.queue[:index], sg.queue[index+1:]...)
	return nil
}

func (sg *SigilGen) ClearQueue() {
	sg.queue = nil
}

// ── 写入 ──

func (sg *SigilGen) ApplyQueue(outputPath string) (*ApplyResult, error) {
	if len(sg.queue) == 0 {
		return nil, fmt.Errorf("队列为空，请先添加因子")
	}
	if sg.save == nil {
		return nil, fmt.Errorf("请先加载存档")
	}

	// 展开队列（按 quantity 展开）
	var expanded []QueueItem
	for _, item := range sg.queue {
		for i := 0; i < item.Quantity; i++ {
			expanded = append(expanded, item)
		}
	}

	// 找空槽
	emptySlots, err := sg.save.FindEmptyGemSlots(len(expanded))
	if err != nil {
		return nil, err
	}

	maxSlotID, err := sg.save.GetMaxSlotID()
	if err != nil {
		return nil, err
	}
	firstNewSlotID := maxSlotID + 1

	// 验证所有槽可写（需要找的 entry 必须存在）
	for i, item := range expanded {
		gemUnitID := emptySlots[i]
		gemIndex := gemUnitID - GemSlotBaseID
		primaryTraitUnit := TraitSlotBase + (gemIndex * 100)
		secondaryTraitUnit := primaryTraitUnit + 1

		// 验证必需的 entry 存在
		if _, ok := sg.save.findUnit(GemSlotIDType, uint32(gemUnitID)); !ok {
			return nil, fmt.Errorf("因子槽 %d 缺少 GEMDATA_SLOT_IDS", gemUnitID)
		}
		if _, ok := sg.save.findUnit(GemWornByIDType, uint32(gemUnitID)); !ok {
			return nil, fmt.Errorf("因子槽 %d 缺少 GEMDATA_WORN_BY", gemUnitID)
		}
		if _, ok := sg.save.findUnit(GemFlagsIDType, uint32(gemUnitID)); !ok {
			return nil, fmt.Errorf("因子槽 %d 缺少 GEMDATA_FLAGS", gemUnitID)
		}
		if _, ok := sg.save.findUnit(GemLevelIDType, uint32(gemUnitID)); !ok {
			return nil, fmt.Errorf("因子槽 %d 缺少 GEMDATA_SKILL_1_LEVEL", gemUnitID)
		}
		if _, ok := sg.save.findUnit(TraitHashIDType, uint32(primaryTraitUnit)); !ok {
			return nil, fmt.Errorf("因子槽 %d 缺少主特性哈希", gemUnitID)
		}
		if _, ok := sg.save.findUnit(TraitLevelIDType, uint32(primaryTraitUnit)); !ok {
			return nil, fmt.Errorf("因子槽 %d 缺少主特性等级", gemUnitID)
		}

		_ = item
		_ = secondaryTraitUnit
	}

	// 更新 max slot ID
	newMaxSlotID := firstNewSlotID + len(expanded) - 1
	if err := sg.save.SetMaxSlotID(newMaxSlotID); err != nil {
		return nil, err
	}

	// 写入每个因子
	created := 0
	for i, item := range expanded {
		gemUnitID := emptySlots[i]
		newSlotID := firstNewSlotID + i

		sigil, _ := sg.catalog.RequireSigil(item.SigilID)
		sigilHash, err := ParseHashHex(sigil.Hash)
		if err != nil {
			return nil, fmt.Errorf("%s 哈希无效: %s", sigil.DisplayName, sigil.Hash)
		}

		primaryTrait, _ := sg.catalog.RequireTrait(sigil.PrimaryTraitID)
		primaryHash, err := ParseHashHex(primaryTrait.Hash)
		if err != nil {
			return nil, fmt.Errorf("%s 哈希无效", primaryTrait.DisplayName)
		}

		var secondaryHash uint32
		var secondaryLevel int
		hasSecondary := sigil.SupportsSecondaryTrait != nil && *sigil.SupportsSecondaryTrait
		if hasSecondary {
			secondaryTrait, _ := sg.catalog.RequireTrait(item.SecondaryTraitID)
			secondaryHash, err = ParseHashHex(secondaryTrait.Hash)
			if err != nil {
				return nil, fmt.Errorf("%s 哈希无效", secondaryTrait.DisplayName)
			}
			secondaryLevel = item.SecondaryLevel
		}

		if err := sg.save.PatchSigil(gemUnitID, newSlotID, sigilHash, item.Level,
			primaryHash, item.PrimaryLevel,
			secondaryHash, secondaryLevel, hasSecondary); err != nil {
			return nil, fmt.Errorf("写入 %s 失败: %w", item.SigilName, err)
		}
		created++
	}

	// 修复校验和
	if err := sg.save.FixChecksums(); err != nil {
		return nil, fmt.Errorf("校验和修复失败: %w", err)
	}

	// 写入输出文件
	if err := sg.save.Write(outputPath); err != nil {
		return nil, fmt.Errorf("写入输出文件失败: %w", err)
	}

	// 验证
	verified := 0
	verifySave, err := LoadSave(outputPath)
	if err == nil {
		for i, item := range expanded {
			gemUnitID := emptySlots[i]
			sigil, _ := sg.catalog.RequireSigil(item.SigilID)
			sigilHash, _ := ParseHashHex(sigil.Hash)
			primaryTrait, _ := sg.catalog.RequireTrait(sigil.PrimaryTraitID)
			primaryHash, _ := ParseHashHex(primaryTrait.Hash)

			var secondaryHash uint32
			var secondaryLevel int
			hasSecondary := sigil.SupportsSecondaryTrait != nil && *sigil.SupportsSecondaryTrait
			if hasSecondary {
				secondaryTrait, _ := sg.catalog.RequireTrait(item.SecondaryTraitID)
				secondaryHash, _ = ParseHashHex(secondaryTrait.Hash)
				secondaryLevel = item.SecondaryLevel
			}

			if verifySave.VerifySigil(gemUnitID, sigilHash, item.Level,
				primaryHash, item.PrimaryLevel,
				secondaryHash, secondaryLevel, hasSecondary) == nil {
				verified++
			}
		}
	}

	// 清空队列
	sg.queue = nil

	absPath, _ := filepath.Abs(outputPath)
	return &ApplyResult{
		CreatedCount: created,
		VerifiedCount: verified,
		OutputPath:   absPath,
	}, nil
}

// RemoveAllSigils 清除输出的存档中所有因子
func (sg *SigilGen) RemoveAllSigils(inputPath, outputPath string) (*ApplyResult, error) {
	s, err := LoadSave(inputPath)
	if err != nil {
		return nil, err
	}

	allGemUnits := s.findAllUnitsByType(GemIDType)
	removed := 0
	for _, u := range allGemUnits {
		if int(u.UnitID) >= GemSlotBaseID && u.Uint32() != EmptyHash {
			gemIndex := int(u.UnitID) - GemSlotBaseID
			primaryTraitUnit := uint32(TraitSlotBase + (gemIndex * 100))
			secondaryTraitUnit := primaryTraitUnit + 1

			s.tryPatchUint(GemIDType, u.UnitID, EmptyHash)
			s.tryPatchInt(GemLevelIDType, u.UnitID, 0)
			s.tryPatchUint(GemWornByIDType, u.UnitID, EmptyHash)
			s.tryPatchUint(GemFlagsIDType, u.UnitID, 0)
			s.tryPatchUint(TraitHashIDType, primaryTraitUnit, EmptyHash)
			s.tryPatchInt(TraitLevelIDType, primaryTraitUnit, 0)
			s.tryPatchUint(TraitHashIDType, secondaryTraitUnit, EmptyHash)
			s.tryPatchInt(TraitLevelIDType, secondaryTraitUnit, 0)
			removed++
		}
	}

	if err := s.FixChecksums(); err != nil {
		return nil, fmt.Errorf("校验和修复失败: %w", err)
	}
	if err := s.Write(outputPath); err != nil {
		return nil, fmt.Errorf("写入输出文件失败: %w", err)
	}

	verifySave, _ := LoadSave(outputPath)
	remaining := 0
	if verifySave != nil {
		remaining = verifySave.GetOccupiedGemCount()
	}

	absPath, _ := filepath.Abs(outputPath)
	return &ApplyResult{
		CreatedCount: removed,
		VerifiedCount: remaining,
		OutputPath:   absPath,
	}, nil
}

// ── 辅助函数 ──

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

func containsInt(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

