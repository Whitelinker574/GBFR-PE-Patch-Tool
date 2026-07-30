package backend

import (
	"fmt"
	"math"
	"strings"
)

// SaveDiffEntity is resolved independently for each side of a comparison.
// UnitID is only a join key inside one save; it is never treated as a global
// character, item, or equipment identity.
type SaveDiffEntity struct {
	Kind       string `json:"kind,omitempty"`
	Key        string `json:"key,omitempty"`
	NameZh     string `json:"nameZh,omitempty"`
	NameEn     string `json:"nameEn,omitempty"`
	DetailZh   string `json:"detailZh,omitempty"`
	DetailEn   string `json:"detailEn,omitempty"`
	Confidence string `json:"confidence,omitempty"`
}

type saveDiffValueSummary struct {
	exists         bool
	numeric        string
	hasNumeric     bool
	firstUint32    uint32
	hasFirstUint32 bool
	singleBool     bool
	hasSingleBool  bool
}

func summarizeSaveDiffValues[T any](values []T) saveDiffValueSummary {
	if len(values) == 0 {
		return saveDiffValueSummary{}
	}
	summary := saveDiffValueSummary{exists: true}
	switch typed := any(values).(type) {
	case []int32:
		summary.numeric, summary.hasNumeric = fmt.Sprintf("%d", typed[0]), true
		summary.firstUint32, summary.hasFirstUint32 = uint32(typed[0]), true
	case []uint32:
		summary.numeric, summary.hasNumeric = fmt.Sprintf("%d", typed[0]), true
		summary.firstUint32, summary.hasFirstUint32 = typed[0], true
	case []bool:
		if len(typed) == 1 {
			summary.singleBool, summary.hasSingleBool = typed[0], true
		}
	case []float32:
		summary.numeric, summary.hasNumeric = fmt.Sprintf("%.3g", typed[0]), true
	case []int16:
		summary.numeric, summary.hasNumeric = fmt.Sprintf("%d", typed[0]), true
		summary.firstUint32, summary.hasFirstUint32 = uint32(typed[0]), true
	case []uint16:
		summary.numeric, summary.hasNumeric = fmt.Sprintf("%d", typed[0]), true
		summary.firstUint32, summary.hasFirstUint32 = uint32(typed[0]), true
	case []int64:
		summary.numeric, summary.hasNumeric = fmt.Sprintf("%d", typed[0]), true
	case []uint64:
		summary.numeric, summary.hasNumeric = fmt.Sprintf("%d", typed[0]), true
	}
	return summary
}

type saveDiffEntityIndex struct {
	save         *SaveGameFile
	uintScalar   map[string]map[uint64]uint32
	wrightstones *WrightstoneCatalog
	summonNames  map[uint32][2]string
}

func newSaveDiffEntityIndex(save *SaveGameFile) *saveDiffEntityIndex {
	index := &saveDiffEntityIndex{save: save, uintScalar: make(map[string]map[uint64]uint32, 2)}
	index.wrightstones, _ = LoadWrightstoneCatalog()
	index.summonNames, _, _, _, _ = parseNaturalDropNameCatalog()
	for _, section := range []string{"binary1", "slotData"} {
		data := saveDiffBinary(save, section)
		if data == nil {
			continue
		}
		values := make(map[uint64]uint32, len(data.UIntTable))
		for _, unit := range data.UIntTable {
			if len(unit.ValueData) != 1 {
				continue
			}
			key := uint64(unit.IDType)<<32 | uint64(unit.UnitID)
			if _, exists := values[key]; !exists {
				values[key] = unit.ValueData[0]
			}
		}
		index.uintScalar[section] = values
	}
	return index
}

func (index *saveDiffEntityIndex) scalar(section string, idType, unitID uint32) (uint32, bool) {
	if index == nil {
		return 0, false
	}
	value, ok := index.uintScalar[section][uint64(idType)<<32|uint64(unitID)]
	return value, ok
}

func saveDiffBinary(save *SaveGameFile, section string) *SaveDataBinary {
	if save == nil {
		return nil
	}
	if section == "binary1" {
		return save.Binary1
	}
	if section == "slotData" {
		return save.SlotData
	}
	return nil
}

func saveDiffCharacterEntity(hash uint32) SaveDiffEntity {
	for ownerCode, candidate := range runtimeOwnerCharacterHash {
		if candidate != hash {
			continue
		}
		names, ok := runtimePatchPartyCharacterNames[ownerCode]
		if !ok {
			break
		}
		return SaveDiffEntity{
			Kind: "character", Key: fmt.Sprintf("character:%08X", hash),
			NameZh: names[0], NameEn: names[1],
			DetailZh: ownerCode, DetailEn: ownerCode, Confidence: "known",
		}
	}
	return SaveDiffEntity{
		Kind: "character", Key: fmt.Sprintf("character:%08X", hash),
		NameZh:     fmt.Sprintf("未收录角色 0x%08X", hash),
		NameEn:     fmt.Sprintf("Uncatalogued Character 0x%08X", hash),
		Confidence: "unknown",
	}
}

func saveDiffCharacterForUnit(index *saveDiffEntityIndex, section string, unitID uint32) SaveDiffEntity {
	hash, ok := index.scalar(section, SaveID_CharacterID, unitID)
	if !ok || hash == 0 || hash == EmptyHash {
		return SaveDiffEntity{
			Kind: "character", Key: fmt.Sprintf("character-unit:%d", unitID),
			NameZh:     fmt.Sprintf("未识别角色记录 · UnitID %d", unitID),
			NameEn:     fmt.Sprintf("Unidentified Character Record · UnitID %d", unitID),
			Confidence: "unknown",
		}
	}
	return saveDiffCharacterEntity(hash)
}

func saveDiffItemEntity(index *saveDiffEntityIndex, section string, unitID uint32) SaveDiffEntity {
	hash, ok := index.scalar(section, SaveID_ItemID, unitID)
	if !ok || hash == 0 || hash == EmptyHash {
		return SaveDiffEntity{Kind: "item", Key: fmt.Sprintf("item-unit:%d", unitID), NameZh: fmt.Sprintf("未识别物品记录 · UnitID %d", unitID), NameEn: fmt.Sprintf("Unidentified Item Record · UnitID %d", unitID), Confidence: "unknown"}
	}
	_, _ = loadProgressionCatalog()
	if def, found := progressionItemByHash[hash]; found && strings.TrimSpace(def.NameCN) != "" && strings.TrimSpace(def.NameEN) != "" {
		return SaveDiffEntity{Kind: "item", Key: fmt.Sprintf("item:%08X", hash), NameZh: def.NameCN, NameEn: def.NameEN, DetailZh: fmt.Sprintf("物品 Hash 0x%08X", hash), DetailEn: fmt.Sprintf("Item Hash 0x%08X", hash), Confidence: "known"}
	}
	return SaveDiffEntity{Kind: "item", Key: fmt.Sprintf("item:%08X", hash), NameZh: fmt.Sprintf("未收录物品 0x%08X", hash), NameEn: fmt.Sprintf("Uncatalogued Item 0x%08X", hash), Confidence: "unknown"}
}

func saveDiffWeaponEntity(index *saveDiffEntityIndex, section string, unitID uint32) SaveDiffEntity {
	hash, ok := index.scalar(section, weaponIDType, unitID)
	if !ok || hash == 0 || hash == EmptyHash {
		return SaveDiffEntity{Kind: "weapon", Key: fmt.Sprintf("weapon-unit:%d", unitID), NameZh: fmt.Sprintf("未识别武器记录 · UnitID %d", unitID), NameEn: fmt.Sprintf("Unidentified Weapon Record · UnitID %d", unitID), Confidence: "unknown"}
	}
	_, _ = loadProgressionCatalog()
	if def, found := progressionWeaponDefForHash(hash); found && strings.TrimSpace(def.NameCN) != "" && strings.TrimSpace(def.Name) != "" {
		return SaveDiffEntity{Kind: "weapon", Key: fmt.Sprintf("weapon:%08X", hash), NameZh: def.NameCN, NameEn: def.Name, DetailZh: fmt.Sprintf("%s · 武器 Hash 0x%08X", def.OwnerCode, hash), DetailEn: fmt.Sprintf("%s · Weapon Hash 0x%08X", def.OwnerCode, hash), Confidence: "known"}
	}
	return SaveDiffEntity{Kind: "weapon", Key: fmt.Sprintf("weapon:%08X", hash), NameZh: fmt.Sprintf("未收录武器 0x%08X", hash), NameEn: fmt.Sprintf("Uncatalogued Weapon 0x%08X", hash), Confidence: "unknown"}
}

func saveDiffSigilEntity(index *saveDiffEntityIndex, section string, unitID uint32) SaveDiffEntity {
	hash, ok := index.scalar(section, SaveID_GemID, unitID)
	if !ok || hash == 0 || hash == EmptyHash {
		return SaveDiffEntity{Kind: "sigil", Key: fmt.Sprintf("sigil-unit:%d", unitID), NameZh: fmt.Sprintf("未识别因子记录 · UnitID %d", unitID), NameEn: fmt.Sprintf("Unidentified Sigil Record · UnitID %d", unitID), Confidence: "unknown"}
	}
	if catalog, err := LoadCatalog(); err == nil {
		if def := catalog.LookupSigilByHash(hash); def != nil {
			en := strings.TrimSpace(def.DisplayName)
			zh := strings.TrimSpace(supplementalSigilDisplayName(def))
			if zh == "" {
				zh = strings.TrimSpace(sigilCN[en])
			}
			if zh != "" && en != "" {
				return SaveDiffEntity{Kind: "sigil", Key: fmt.Sprintf("sigil:%08X", hash), NameZh: normalizeChineseSigilItemName(zh), NameEn: en, DetailZh: fmt.Sprintf("因子 Hash 0x%08X", hash), DetailEn: fmt.Sprintf("Sigil Hash 0x%08X", hash), Confidence: "known"}
			}
		}
	}
	return SaveDiffEntity{Kind: "sigil", Key: fmt.Sprintf("sigil:%08X", hash), NameZh: fmt.Sprintf("未收录因子 0x%08X", hash), NameEn: fmt.Sprintf("Uncatalogued Sigil 0x%08X", hash), Confidence: "unknown"}
}

func saveDiffWrightstoneEntity(index *saveDiffEntityIndex, section string, unitID uint32) SaveDiffEntity {
	hash, ok := index.scalar(section, WrightstoneItemIDType, unitID)
	if !ok || hash == 0 || hash == EmptyHash {
		return SaveDiffEntity{Kind: "wrightstone", Key: fmt.Sprintf("wrightstone-unit:%d", unitID), NameZh: fmt.Sprintf("未识别祝福石记录 · UnitID %d", unitID), NameEn: fmt.Sprintf("Unidentified Wrightstone Record · UnitID %d", unitID), Confidence: "unknown"}
	}
	if index.wrightstones != nil {
		if def := index.wrightstones.LookupWrightstoneByHash(hash); def != nil {
			en := strings.TrimSpace(def.DisplayName)
			zh := strings.TrimSpace(wrightstoneCN[en])
			if zh != "" && en != "" {
				return SaveDiffEntity{Kind: "wrightstone", Key: fmt.Sprintf("wrightstone:%08X", hash), NameZh: zh, NameEn: en, DetailZh: fmt.Sprintf("祝福石 Hash 0x%08X", hash), DetailEn: fmt.Sprintf("Wrightstone Hash 0x%08X", hash), Confidence: "known"}
			}
		}
	}
	return SaveDiffEntity{Kind: "wrightstone", Key: fmt.Sprintf("wrightstone:%08X", hash), NameZh: fmt.Sprintf("未收录祝福石 0x%08X", hash), NameEn: fmt.Sprintf("Uncatalogued Wrightstone 0x%08X", hash), Confidence: "unknown"}
}

func saveDiffSummonEntity(index *saveDiffEntityIndex, section string, unitID uint32) SaveDiffEntity {
	hash, ok := index.scalar(section, SummonTypeIDType, unitID)
	if !ok || hash == 0 || hash == EmptyHash {
		return SaveDiffEntity{Kind: "summon", Key: fmt.Sprintf("summon-unit:%d", unitID), NameZh: fmt.Sprintf("未识别召唤石记录 · UnitID %d", unitID), NameEn: fmt.Sprintf("Unidentified Summon Record · UnitID %d", unitID), Confidence: "unknown"}
	}
	if pair, found := index.summonNames[hash]; found && strings.TrimSpace(pair[0]) != "" && strings.TrimSpace(pair[1]) != "" {
		return SaveDiffEntity{Kind: "summon", Key: fmt.Sprintf("summon:%08X", hash), NameZh: pair[0], NameEn: pair[1], DetailZh: fmt.Sprintf("召唤石 Hash 0x%08X", hash), DetailEn: fmt.Sprintf("Summon Hash 0x%08X", hash), Confidence: "known"}
	}
	return SaveDiffEntity{Kind: "summon", Key: fmt.Sprintf("summon:%08X", hash), NameZh: fmt.Sprintf("未收录召唤石 0x%08X", hash), NameEn: fmt.Sprintf("Uncatalogued Summon 0x%08X", hash), Confidence: "unknown"}
}

func saveDiffTraitEntity(index *saveDiffEntityIndex, section string, unitID uint32) SaveDiffEntity {
	hash, ok := index.scalar(section, TraitHashIDType, unitID)
	if !ok || hash == 0 || hash == EmptyHash {
		return SaveDiffEntity{Kind: "trait", Key: fmt.Sprintf("trait-unit:%d", unitID), NameZh: fmt.Sprintf("未识别词条记录 · UnitID %d", unitID), NameEn: fmt.Sprintf("Unidentified Trait Record · UnitID %d", unitID), Confidence: "unknown"}
	}
	if catalog, err := LoadCatalog(); err == nil {
		if def := catalog.LookupTraitByHash(hash); def != nil {
			en := strings.TrimSpace(def.DisplayName)
			zh := strings.TrimSpace(traitCN[en])
			if zh != "" && en != "" {
				return SaveDiffEntity{Kind: "trait", Key: fmt.Sprintf("trait:%08X", hash), NameZh: zh, NameEn: en, DetailZh: fmt.Sprintf("词条 Hash 0x%08X", hash), DetailEn: fmt.Sprintf("Trait Hash 0x%08X", hash), Confidence: "known"}
			}
		}
	}
	return SaveDiffEntity{Kind: "trait", Key: fmt.Sprintf("trait:%08X", hash), NameZh: fmt.Sprintf("未收录词条 0x%08X", hash), NameEn: fmt.Sprintf("Uncatalogued Trait 0x%08X", hash), Confidence: "unknown"}
}

func saveDiffEntityFor(index *saveDiffEntityIndex, entry *SaveDiffEntry, values saveDiffValueSummary) SaveDiffEntity {
	if index == nil || entry == nil {
		return SaveDiffEntity{}
	}
	data := saveDiffBinary(index.save, entry.Section)
	if data == nil {
		return SaveDiffEntity{}
	}
	switch entry.Category {
	case "system", "currency", "title", "unlock":
		return SaveDiffEntity{Kind: "save", Key: "save:global", NameZh: "当前存档", NameEn: "Current Save", Confidence: "known"}
	case "character":
		if entry.IDType == SaveID_FavoriteChara {
			if values.hasFirstUint32 {
				return saveDiffCharacterEntity(values.firstUint32)
			}
		}
		return saveDiffCharacterForUnit(index, entry.Section, entry.UnitID)
	case "inventory":
		return saveDiffItemEntity(index, entry.Section, entry.UnitID)
	case "weapon":
		return saveDiffWeaponEntity(index, entry.Section, entry.UnitID)
	case "sigil":
		return saveDiffSigilEntity(index, entry.Section, entry.UnitID)
	case "wrightstone":
		return saveDiffWrightstoneEntity(index, entry.Section, entry.UnitID)
	case "summon":
		return saveDiffSummonEntity(index, entry.Section, entry.UnitID)
	case "trait":
		return saveDiffTraitEntity(index, entry.Section, entry.UnitID)
	case "loadout":
		hash, ok := index.scalar(entry.Section, loadoutCharIDType, entry.UnitID)
		if ok && hash != 0 && hash != EmptyHash {
			entity := saveDiffCharacterEntity(hash)
			entity.Kind = "loadout"
			entity.Key = fmt.Sprintf("loadout:%d:%08X", entry.UnitID, hash)
			entity.NameZh += " · 配装预设"
			entity.NameEn += " · Loadout Preset"
			return entity
		}
		return SaveDiffEntity{Kind: "loadout", Key: fmt.Sprintf("loadout-unit:%d", entry.UnitID), NameZh: fmt.Sprintf("未识别配装预设 · UnitID %d", entry.UnitID), NameEn: fmt.Sprintf("Unidentified Loadout Preset · UnitID %d", entry.UnitID), Confidence: "unknown"}
	case "quest":
		hash, ok := index.scalar(entry.Section, SaveID_QuestIDs, entry.UnitID)
		if ok {
			return SaveDiffEntity{Kind: "quest", Key: fmt.Sprintf("quest:%08X", hash), NameZh: questIDToNameCN(hash), NameEn: questIDToName(hash), DetailZh: fmt.Sprintf("任务 ID 0x%08X", hash), DetailEn: fmt.Sprintf("Quest ID 0x%08X", hash), Confidence: "known"}
		}
	}
	return SaveDiffEntity{Kind: entry.Category, Key: fmt.Sprintf("%s-unit:%d", entry.Category, entry.UnitID), NameZh: fmt.Sprintf("未识别记录 · UnitID %d", entry.UnitID), NameEn: fmt.Sprintf("Unidentified Record · UnitID %d", entry.UnitID), Confidence: "unknown"}
}

func saveDiffDisplay(entry *SaveDiffEntry, values saveDiffValueSummary, entity SaveDiffEntity, fallback string) (string, string) {
	if !values.exists {
		return "—", "—"
	}
	first, numeric := values.numeric, values.hasNumeric
	switch entry.IDType {
	case 1308:
		if numeric {
			return "Lv" + first, "Lv" + first
		}
	case 1309:
		if numeric {
			return first + " 基础 HP", first + " Base HP"
		}
	case 1310:
		if numeric {
			return first + " 基础攻击", first + " Base ATK"
		}
	case 1313:
		if numeric {
			return first + "% 基础暴击率", first + "% Base Critical Rate"
		}
	case SaveID_ItemCount:
		if numeric {
			return "×" + first, "×" + first
		}
	case SaveID_Rupees:
		if numeric {
			return first + " 卢比", first + " Rupees"
		}
	case SaveID_MasteryPoints:
		if numeric {
			return first + " MSP", first + " MSP"
		}
	case SaveID_Commendations:
		if numeric {
			return first + " 表彰章", first + " Commendations"
		}
	case SaveID_QuestCompleteCount:
		if numeric {
			return "完成 " + first + " 次", first + " Clears"
		}
	case GemLevelIDType, TraitLevelIDType, SummonRankIDType:
		if numeric {
			return "Lv" + first, "Lv" + first
		}
	case SaveID_CharacterID, SaveID_FavoriteChara, SaveID_ItemID, SaveID_GemID, weaponIDType, WrightstoneItemIDType, SummonTypeIDType, loadoutCharIDType:
		if entity.NameZh != "" || entity.NameEn != "" {
			return entity.NameZh, entity.NameEn
		}
	}
	if values.hasSingleBool {
		if values.singleBool {
			return "已开启", "Enabled"
		}
		return "未开启", "Disabled"
	}
	if entry.IDType == 1312 {
		if values.hasFirstUint32 {
			value := math.Float32frombits(values.firstUint32)
			return fmt.Sprintf("%.3g 基础昏厥值", value), fmt.Sprintf("%.3g Base Stun", value)
		}
	}
	return fallback, fallback
}

func saveDiffSetBlock(entry *SaveDiffEntry, zh, en, risk string) {
	entry.CopySupported = false
	entry.CopyBlockReasonZh = zh
	entry.CopyBlockReasonEn = en
	entry.CopyBlockReason = saveDiffText(zh, en)
	entry.RiskLevel = risk
	entry.RiskReasonZh = zh
	entry.RiskReasonEn = en
}

func saveDiffEntityMustMatch(category string) bool {
	switch category {
	case "character", "inventory", "quest", "trait", "sigil", "wrightstone", "summon", "weapon", "loadout":
		return true
	default:
		return false
	}
}

func enrichSaveDiffEntry(entry *SaveDiffEntry, left, right *saveDiffEntityIndex) {
	if entry == nil {
		return
	}
	entry.LeftEntity = saveDiffEntityFor(left, entry, entry.leftValue)
	entry.RightEntity = saveDiffEntityFor(right, entry, entry.rightValue)
	entry.LeftDisplayZh, entry.LeftDisplayEn = saveDiffDisplay(entry, entry.leftValue, entry.LeftEntity, entry.LeftPreview)
	entry.RightDisplayZh, entry.RightDisplayEn = saveDiffDisplay(entry, entry.rightValue, entry.RightEntity, entry.RightPreview)

	if entry.CopyBlockReasonZh != "" || entry.CopyBlockReasonEn != "" {
		saveDiffSetBlock(entry, entry.CopyBlockReasonZh, entry.CopyBlockReasonEn, "blocked")
		return
	}
	if entry.Status == "unchanged" {
		entry.RiskLevel = "low"
		return
	}
	if entry.SemanticConfidence == "unknown" {
		saveDiffSetBlock(entry,
			"字段用途尚未识别，只保留只读对比，不允许按原始数字直接写入",
			"The field purpose is unidentified. It remains read-only and cannot be written as an unexplained raw value.",
			"blocked",
		)
		return
	}
	if entry.SemanticConfidence != "known" {
		saveDiffSetBlock(entry,
			"目前只确认了代码读取位置，游戏内完整用途尚未闭环；先保留只读",
			"Only the code read location is confirmed; the complete in-game meaning is not closed, so this stays read-only.",
			"review",
		)
		return
	}
	if saveDiffEntityMustMatch(entry.Category) {
		if entry.LeftEntity.Confidence != "known" || entry.RightEntity.Confidence != "known" {
			saveDiffSetBlock(entry,
				"无法在两份存档中确认这是同一个角色、物品或装备实例，不能只凭 UnitID 复制",
				"The same character, item, or equipment identity could not be confirmed in both saves. UnitID alone is not enough to copy it.",
				"blocked",
			)
			return
		}
		if entry.LeftEntity.Key == "" || entry.LeftEntity.Key != entry.RightEntity.Key {
			saveDiffSetBlock(entry,
				"左右记录指向的具体对象不同，复制单个字段会把一件对象的数据套到另一件对象上",
				"The two records point to different entities. Copying one field would apply one entity's data to another.",
				"blocked",
			)
			return
		}
	}
	entry.CopySupported = true
	entry.RiskLevel = "low"
	entry.RiskReasonZh = "字段含义、左右记录结构和具体对象身份均已确认；写入仍会先备份并逐字段回读"
	entry.RiskReasonEn = "Field meaning, record structure, and entity identity are confirmed. Writing still backs up and verifies each field."
}
