package main

import (
	"encoding/json"
	"fmt"
	"math/bits"
	"sort"
	"strings"
)

// LoadoutStatContext is the save-backed character/summon context used by the
// loadout editor. BaseHP and BaseATK deliberately retain the save-field names:
// they are not a claim that the final in-combat panel has been reconstructed.
type LoadoutStatContext struct {
	CharaHash             string                  `json:"charaHash"`
	CharaUnitID           uint32                  `json:"charaUnitId"`
	Level                 int                     `json:"level"`
	BaseHP                int                     `json:"baseHp"`
	BaseATK               int                     `json:"baseAtk"`
	BaseStun              float64                 `json:"baseStun"`
	BaseCritRate          float64                 `json:"baseCritRate"`
	Summons               []LoadoutSummon         `json:"summons"`
	EquippedSummonSlotIDs []uint32                `json:"equippedSummonSlotIds"`
	EquippedSummons       []LoadoutSummon         `json:"equippedSummons"`
	OverLimit             []LoadoutOverLimitBonus `json:"overLimit"`
	Warnings              []string                `json:"warnings"`
}

type LoadoutSummon struct {
	UnitID         uint32  `json:"unitId"`
	SlotID         uint32  `json:"slotId"`
	TypeHash       string  `json:"typeHash"`
	Name           string  `json:"name"`
	MainTraitHash  string  `json:"mainTraitHash"`
	MainTraitName  string  `json:"mainTraitName"`
	MainTraitLevel int     `json:"mainTraitLevel"`
	SubParamHash   string  `json:"subParamHash"`
	SubParamName   string  `json:"subParamName"`
	SubParamLevel  int     `json:"subParamLevel"`
	SubParamValue  float64 `json:"subParamValue"`
	SubParamUnit   string  `json:"subParamUnit"`
	Rank           int     `json:"rank"`
}

type LoadoutOverLimitBonus struct {
	Index         int     `json:"index"`
	UnitID        uint32  `json:"unitId"`
	AttributeHash string  `json:"attributeHash"`
	Name          string  `json:"name"`
	Level         int     `json:"level"`
	Value         float64 `json:"value"`
	Unit          string  `json:"unit"`
}

type summonStatCatalog struct {
	types map[uint32]SummonOption
	main  map[uint32]SummonOption
	sub   map[uint32]SummonOption
}

func loadSummonStatCatalog() (*summonStatCatalog, error) {
	var types summonTypeFile
	var skills summonSkillFile
	var subs summonSubParamFile
	if err := json.Unmarshal(summonTypesJSON, &types); err != nil {
		return nil, fmt.Errorf("解析召唤石名称目录失败: %w", err)
	}
	if err := json.Unmarshal(summonSkillsJSON, &skills); err != nil {
		return nil, fmt.Errorf("解析召唤石加护目录失败: %w", err)
	}
	if err := json.Unmarshal(summonSubParamsJSON, &subs); err != nil {
		return nil, fmt.Errorf("解析召唤石副参数目录失败: %w", err)
	}
	catalog := &summonStatCatalog{
		types: make(map[uint32]SummonOption, len(types.Summons)),
		main:  make(map[uint32]SummonOption, len(skills.Skills)),
		sub:   make(map[uint32]SummonOption, len(subs.SubParams)),
	}
	for _, item := range types.Summons {
		hash, err := ParseHashHex(item.Hash)
		if err != nil {
			continue
		}
		catalog.types[hash] = SummonOption{Hash: hash, Name: item.DisplayName, Cost: item.Cost, TypeName: item.TypeName}
	}
	for _, item := range skills.Skills {
		hash, err := ParseHashHex(item.Hash)
		if err != nil {
			continue
		}
		catalog.main[hash] = SummonOption{Hash: hash, Name: item.DisplayName, MaxLevel: item.MaxLevel}
	}
	for _, item := range subs.SubParams {
		hash, err := ParseHashHex(item.Hash)
		if err != nil {
			continue
		}
		catalog.sub[hash] = SummonOption{
			Hash: hash, Name: item.DisplayName, MaxLevel: item.MaxLevel,
			IsPercent: item.IsPercent, Values: item.Values,
		}
	}
	return catalog, nil
}

func uintUnitExact(data *SaveDataBinary, idType, unitID uint32) (*UIntSaveDataUnit, bool) {
	if data == nil {
		return nil, false
	}
	for i := range data.UIntTable {
		unit := &data.UIntTable[i]
		if unit.IDType == idType && unit.UnitID == unitID {
			return unit, true
		}
	}
	return nil, false
}

func intUnitExact(data *SaveDataBinary, idType, unitID uint32) (*IntSaveDataUnit, bool) {
	if data == nil {
		return nil, false
	}
	for i := range data.IntTable {
		unit := &data.IntTable[i]
		if unit.IDType == idType && unit.UnitID == unitID {
			return unit, true
		}
	}
	return nil, false
}

func uintUnitsByType(data *SaveDataBinary, idType uint32) []UIntSaveDataUnit {
	if data == nil {
		return nil
	}
	result := make([]UIntSaveDataUnit, 0)
	for _, unit := range data.UIntTable {
		if unit.IDType == idType {
			result = append(result, unit)
		}
	}
	return result
}

func uintUnitMap(data *SaveDataBinary, idType uint32) map[uint32]UIntSaveDataUnit {
	result := map[uint32]UIntSaveDataUnit{}
	for _, unit := range uintUnitsByType(data, idType) {
		if _, exists := result[unit.UnitID]; !exists {
			result[unit.UnitID] = unit
		}
	}
	return result
}

func intUnitMap(data *SaveDataBinary, idType uint32) map[uint32]IntSaveDataUnit {
	result := map[uint32]IntSaveDataUnit{}
	if data == nil {
		return result
	}
	for _, unit := range data.IntTable {
		if unit.IDType == idType {
			if _, exists := result[unit.UnitID]; !exists {
				result[unit.UnitID] = unit
			}
		}
	}
	return result
}

func requireIntScalar(data *SaveDataBinary, idType, unitID uint32, label string) (int, error) {
	unit, ok := intUnitExact(data, idType, unitID)
	if !ok || len(unit.ValueData) != 1 {
		return 0, fmt.Errorf("存档缺少角色 %d 的 %d %s 标量", unitID, idType, label)
	}
	return int(unit.ValueData[0]), nil
}

func hashText(hash uint32) string { return fmt.Sprintf("%08X", hash) }

func appendWarning(warnings *[]string, format string, args ...any) {
	*warnings = append(*warnings, fmt.Sprintf(format, args...))
}

func readSummonInventory(data *SaveDataBinary, catalog *summonStatCatalog, warnings *[]string) []LoadoutSummon {
	types := uintUnitMap(data, 1457)
	traits := uintUnitMap(data, 1458)
	levels := intUnitMap(data, 1459)
	ranks := uintUnitMap(data, 1460)
	bySlot := map[uint32]bool{}
	result := make([]LoadoutSummon, 0)

	// Use the typed FlatBuffers table here. A raw byte scan of 1457 has been
	// observed to produce false-positive records in real saves.
	for _, slotUnit := range uintUnitsByType(data, 1456) {
		if len(slotUnit.ValueData) != 1 {
			appendWarning(warnings, "召唤石实例 %d 的 1456 不是标量，已忽略", slotUnit.UnitID)
			continue
		}
		slotID := slotUnit.ValueData[0]
		if slotID == 0 || slotID == EmptyHash {
			continue
		}
		if bySlot[slotID] {
			appendWarning(warnings, "召唤石 SlotID %d 重复，后续实例已忽略", slotID)
			continue
		}
		bySlot[slotID] = true

		typeUnit, typeOK := types[slotUnit.UnitID]
		traitUnit, traitOK := traits[slotUnit.UnitID]
		levelUnit, levelOK := levels[slotUnit.UnitID]
		rankUnit, rankOK := ranks[slotUnit.UnitID]
		if !typeOK || len(typeUnit.ValueData) != 1 || !traitOK || len(traitUnit.ValueData) != 2 ||
			!levelOK || len(levelUnit.ValueData) != 2 || !rankOK || len(rankUnit.ValueData) != 1 {
			appendWarning(warnings, "召唤石 SlotID %d 的 1457..1460 联表不完整，已忽略", slotID)
			continue
		}
		typeHash := typeUnit.ValueData[0]
		if typeHash == 0 || typeHash == EmptyHash || typeHash == summonInvalidTypeHash {
			appendWarning(warnings, "召唤石 SlotID %d 没有有效类型，已忽略", slotID)
			continue
		}
		if levelUnit.ValueData[0] < 0 || levelUnit.ValueData[1] < 0 {
			appendWarning(warnings, "召唤石 SlotID %d 的效果等级为负数，不参与数值模拟", slotID)
		}

		mainHash, subHash := traitUnit.ValueData[0], traitUnit.ValueData[1]
		summon := LoadoutSummon{
			UnitID: slotUnit.UnitID, SlotID: slotID,
			TypeHash: hashText(typeHash), MainTraitHash: hashText(mainHash), SubParamHash: hashText(subHash),
			MainTraitLevel: int(levelUnit.ValueData[0]), SubParamLevel: int(levelUnit.ValueData[1]),
			Rank: int(rankUnit.ValueData[0]),
		}
		if option, ok := catalog.types[typeHash]; ok {
			summon.Name = option.Name
		} else {
			appendWarning(warnings, "召唤石 SlotID %d 的类型 %s 不在内置目录中", slotID, summon.TypeHash)
		}
		if option, ok := catalog.main[mainHash]; ok {
			summon.MainTraitName = option.Name
		} else if mainHash != 0 && mainHash != EmptyHash {
			appendWarning(warnings, "召唤石 SlotID %d 的主加护 %s 不在内置目录中", slotID, summon.MainTraitHash)
		}
		if option, ok := catalog.sub[subHash]; ok {
			summon.SubParamName = option.Name
			// 1459 stores a zero-based level index for summon sub parameters.
			if summon.SubParamLevel >= 0 && summon.SubParamLevel < len(option.Values) {
				if option.IsPercent {
					summon.SubParamUnit = "pct"
				} else {
					summon.SubParamUnit = "flat"
				}
				summon.SubParamValue = option.Values[summon.SubParamLevel]
			} else {
				appendWarning(warnings, "召唤石 SlotID %d 的副参数等级 %d 超出目录范围", slotID, summon.SubParamLevel)
			}
		} else if subHash != 0 && subHash != EmptyHash {
			appendWarning(warnings, "召唤石 SlotID %d 的副参数 %s 不在内置目录中", slotID, summon.SubParamHash)
		}
		result = append(result, summon)
	}

	sort.Slice(result, func(i, j int) bool { return result[i].SlotID < result[j].SlotID })
	return result
}

type overLimitDefinition struct {
	name   string
	unit   string
	values [10]float64
}

var overLimitDefinitions = func() map[uint32]overLimitDefinition {
	definitions := make(map[uint32]overLimitDefinition, len(overLimitCatalog))
	for hash, entry := range overLimitCatalog {
		definitions[hash] = overLimitDefinition{name: entry.name, unit: entry.unit, values: entry.values}
	}
	return definitions
}()

func readOverLimit(data *SaveDataBinary, charaUnitID uint32, warnings *[]string) ([]LoadoutOverLimitBonus, error) {
	result := make([]LoadoutOverLimitBonus, 0, 4)
	if charaUnitID < 10000 {
		return nil, fmt.Errorf("角色 UnitID %d 不能映射到 1606/1607 极限加成槽", charaUnitID)
	}
	base := uint32(10000000) + (charaUnitID-10000)*1000
	for index := 0; index < 4; index++ {
		unitID := base + uint32(index)
		attribute, attributeOK := uintUnitExact(data, 1606, unitID)
		level, levelOK := intUnitExact(data, 1607, unitID)
		if !attributeOK || len(attribute.ValueData) != 1 || !levelOK || len(level.ValueData) != 1 {
			appendWarning(warnings, "极限加成槽 %d（UnitID %d）的 1606/1607 不完整", index+1, unitID)
			continue
		}
		hash := attribute.ValueData[0]
		levelBit := level.ValueData[0]
		if levelBit == 0 {
			if hash != 0 && hash != EmptyHash {
				appendWarning(warnings, "极限加成槽 %d 有属性 %s 但等级为空", index+1, hashText(hash))
			}
			continue
		}
		if levelBit < 0 || levelBit > 0x200 || (uint32(levelBit)&(uint32(levelBit)-1)) != 0 {
			return nil, fmt.Errorf("1607 UnitID %d 必须是单 bit 且不超过 0x200，实际 0x%X", unitID, uint32(levelBit))
		}
		levelNumber := bits.TrailingZeros32(uint32(levelBit)) + 1
		definition, ok := overLimitCatalog[hash]
		if !ok {
			appendWarning(warnings, "极限加成槽 %d 的属性 %s 不在已审计目录中，已忽略", index+1, hashText(hash))
			continue
		}
		result = append(result, LoadoutOverLimitBonus{
			Index: index, UnitID: unitID, AttributeHash: hashText(hash), Name: definition.name,
			Level: levelNumber, Value: definition.values[levelNumber-1], Unit: definition.unit,
		})
	}
	return result, nil
}

// LoadoutStatContext reads character base fields, the complete summon
// inventory, equipped summon order, and the character's four over-limit slots.
func (a *App) LoadoutStatContext(path, charaHex string) (*LoadoutStatContext, error) {
	charaHash, err := ParseHashHex(charaHex)
	if err != nil {
		return nil, fmt.Errorf("角色 hash 无效: %w", err)
	}
	parsed, err := LoadSaveFile(path)
	if err != nil {
		return nil, err
	}
	if parsed.SlotData == nil {
		return nil, fmt.Errorf("存档没有 SlotData")
	}
	data := parsed.SlotData

	var charaUnitID uint32
	found := false
	for _, unit := range uintUnitsByType(data, 1301) {
		if len(unit.ValueData) == 1 && unit.ValueData[0] == charaHash {
			charaUnitID = unit.UnitID
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("存档里找不到角色 %08X", charaHash)
	}
	level, err := requireIntScalar(data, 1308, charaUnitID, "Level")
	if err != nil {
		return nil, err
	}
	baseHP, err := requireIntScalar(data, 1309, charaUnitID, "BaseHP")
	if err != nil {
		return nil, err
	}
	baseATK, err := requireIntScalar(data, 1310, charaUnitID, "BaseATK")
	if err != nil {
		return nil, err
	}
	catalog, err := loadSummonStatCatalog()
	if err != nil {
		return nil, err
	}
	context := &LoadoutStatContext{
		CharaHash: hashText(charaHash), CharaUnitID: charaUnitID,
		Level: level, BaseHP: baseHP, BaseATK: baseATK,
	}
	// 2.0 chara_status.tbl stores these two panel bases separately from the
	// save-backed HP/ATK fields. Every player character currently has Stun 8
	// and Crit 5; fail closed for unknown hashes instead of inventing values.
	if _, known := characterNameByHash[charaHash]; known {
		context.BaseStun = 8
		context.BaseCritRate = 5
	} else {
		appendWarning(&context.Warnings, "角色 %08X 未收录于 2.0 基础暴击/昏厥目录", charaHash)
	}
	context.Summons = readSummonInventory(data, catalog, &context.Warnings)

	if equipped, ok := uintUnitExact(data, 1451, 0); ok {
		count := len(equipped.ValueData)
		if count > 4 {
			appendWarning(&context.Warnings, "1451 有 %d 个值，只读取前 4 个", count)
			count = 4
		}
		context.EquippedSummonSlotIDs = append([]uint32(nil), equipped.ValueData[:count]...)
	} else {
		appendWarning(&context.Warnings, "存档缺少 1451 UnitID 0 的召唤石配置")
	}
	bySlot := make(map[uint32]LoadoutSummon, len(context.Summons))
	for _, summon := range context.Summons {
		bySlot[summon.SlotID] = summon
	}
	for _, slotID := range context.EquippedSummonSlotIDs {
		if slotID == 0 || slotID == EmptyHash {
			continue
		}
		if summon, ok := bySlot[slotID]; ok {
			context.EquippedSummons = append(context.EquippedSummons, summon)
		} else {
			appendWarning(&context.Warnings, "1451 引用了悬空召唤石 SlotID %d", slotID)
		}
	}
	context.OverLimit, err = readOverLimit(data, charaUnitID, &context.Warnings)
	if err != nil {
		return nil, err
	}
	return context, nil
}

func traitHashMapWithRawKeys(cat *Catalog) map[uint32]string {
	result := buildTraitHashToID(cat)
	// trait_values also contains raw eight-hex keys for some summon-only
	// skills that are intentionally absent from the craftable sigil catalog.
	for key := range loadTraitValues() {
		if len(key) != 8 {
			continue
		}
		if hash, err := ParseHashHex(key); err == nil {
			if _, exists := result[hash]; !exists {
				result[hash] = key
			}
		}
	}
	return result
}

func addTotal(totals *[]EffectTotal, label, unit string, value float64, catLabel, source string) {
	for _, canonical := range canonicalEffectLabels(label) {
		key := unit + "|" + canonical
		index := -1
		for i := range *totals {
			if (*totals)[i].Key == key {
				index = i
				break
			}
		}
		if index < 0 {
			*totals = append(*totals, EffectTotal{Key: key, Label: canonical, Unit: unit, CatLabel: catLabel})
			index = len(*totals) - 1
		}
		(*totals)[index].Value += value
		addTotalSource(&(*totals)[index], source)
	}
}

func addTotalSource(total *EffectTotal, source string) {
	if source == "" {
		return
	}
	for _, existing := range total.Sources {
		if existing == source {
			return
		}
	}
	total.Sources = append(total.Sources, source)
}

func summonSubParamLabel(name string) string {
	if index := strings.IndexAny(name, "（("); index >= 0 {
		name = name[:index]
	}
	switch strings.TrimSpace(name) {
	case "体力":
		return "最大HP"
	case "昏厥":
		return "昏厥值"
	default:
		return strings.TrimSpace(name)
	}
}

func sortEffectTotals(totals []EffectTotal) {
	sort.SliceStable(totals, func(i, j int) bool {
		ci, cj := catRank(totals[i].CatLabel), catRank(totals[j].CatLabel)
		if ci != cj {
			return ci < cj
		}
		return totals[i].Label < totals[j].Label
	})
}

func traitUsesSingleFixedLevel(definition *traitValueDef, level int) bool {
	if definition == nil || level <= 0 {
		return false
	}
	nonZeroLevels := map[int]struct{}{}
	for _, placeholder := range definition.Placeholders {
		for index, value := range placeholder.Values {
			if value != 0 {
				nonZeroLevels[index+1] = struct{}{}
			}
		}
	}
	if len(nonZeroLevels) != 1 {
		return false
	}
	_, fixed := nonZeroLevels[level]
	return fixed
}

// 因子强化会提升普通词条等级，但本地表中也有只在一个指定等级保存
// 定值的特殊因子（例如 SKILL_133_00 只在 Lv6 有效）。把这种词条机械
// 抬到 Lv7 会读到零值，因此必须保留其审计过的固定等级。
func applyFactorLevelBoost(pairs []struct {
	hash  uint32
	level int
}, boost int, hashToID map[uint32]string) {
	if boost <= 0 {
		return
	}
	values := loadTraitValues()
	for index := range pairs {
		if pairs[index].level <= 0 {
			continue
		}
		definition := values[hashToID[pairs[index].hash]]
		if traitUsesSingleFixedLevel(definition, pairs[index].level) {
			continue
		}
		pairs[index].level += boost
	}
}

// LoadoutSimulateBuild simulates the complete editor draft without writing it:
// weapon stats and weapon skills, stored/constructed sigils, selected mastery,
// four summons and the selected character's audited over-limit bonuses.
func (a *App) LoadoutSimulateBuild(path, charaHex string, weaponSlotID uint32, sigilSlotIDs []uint32, constructed []LoadoutConstructedSigil, masteryHexes []string, summonSlotIDs []uint32) (*LoadoutSimulation, error) {
	cat, err := LoadCatalog()
	if err != nil {
		return nil, err
	}
	save, err := LoadSave(path)
	if err != nil {
		return nil, err
	}
	ix := buildLoadoutIndex(save)
	charaHash, err := ParseHashHex(charaHex)
	if err != nil {
		return nil, fmt.Errorf("角色 hash 无效: %w", err)
	}
	ownerCode := ix.deriveOwnerCode(save, charaHash)

	var weapon *LoadoutWeaponContext
	if weaponSlotID != 0 {
		weapon, err = readLoadoutWeaponContext(save, weaponSlotID)
		if err != nil {
			return nil, err
		}
		storedHash, parseErr := ParseHashHex(weapon.StoredHash)
		if parseErr != nil {
			return nil, fmt.Errorf("武器 %q hash 无效: %w", weapon.StoredHash, parseErr)
		}
		if _, validateErr := validateLoadoutWeaponDefinition(storedHash, ownerCode); validateErr != nil {
			return nil, validateErr
		}
	}

	factorPairs := collectStoredSigilPairs(save, ix, sigilSlotIDs)
	factorPrimaryHashes := make([]uint32, 0, len(sigilSlotIDs)+len(constructed))
	for _, slotID := range sigilSlotIDs {
		gemUnitID, ok := ix.gemBySlotID[slotID]
		if !ok {
			continue
		}
		primaryHash, primaryLevel, _, _ := readSigilTraits(save, gemUnitID)
		if primaryHash != 0 && primaryHash != EmptyHash && primaryLevel > 0 {
			factorPrimaryHashes = append(factorPrimaryHashes, primaryHash)
		}
	}
	for _, draft := range constructed {
		prepared, err := prepareLoadoutSigil(cat, draft)
		if err != nil {
			return nil, fmt.Errorf("第 %d 个构造草稿无法模拟: %w", draft.Index+1, err)
		}
		factorPairs = append(factorPairs, struct {
			hash  uint32
			level int
		}{prepared.primaryHash, prepared.item.PrimaryLevel})
		factorPrimaryHashes = append(factorPrimaryHashes, prepared.primaryHash)
		if prepared.secondaryHash != 0 && prepared.secondaryHash != EmptyHash && prepared.secondaryLevel > 0 {
			factorPairs = append(factorPairs, struct {
				hash  uint32
				level int
			}{prepared.secondaryHash, prepared.secondaryLevel})
		}
	}
	// 因子强化 is a weapon skill that raises every equipped factor trait before
	// equal traits are merged and capped. It must not boost summon/weapon skills.
	factorBoost := 0
	if weapon != nil {
		for _, skill := range weapon.Skills {
			if skill.TraitID == "SKILL_113_00" && skill.Level > factorBoost {
				factorBoost = skill.Level
			}
		}
	}
	if factorBoost > 0 {
		applyFactorLevelBoost(factorPairs, factorBoost, buildTraitHashToID(cat))
	}
	pairs := append([]struct {
		hash  uint32
		level int
	}(nil), factorPairs...)
	if weapon != nil {
		for _, skill := range weapon.Skills {
			hash, parseErr := ParseHashHex(skill.TraitHash)
			if parseErr == nil && hash != 0 && hash != EmptyHash && skill.Level > 0 {
				pairs = append(pairs, struct {
					hash  uint32
					level int
				}{hash, skill.Level})
			}
		}
	}

	context, err := a.LoadoutStatContext(path, charaHex)
	if err != nil {
		return nil, err
	}
	if len(summonSlotIDs) > 4 {
		return nil, fmt.Errorf("召唤石最多选择 4 个")
	}
	bySlot := make(map[uint32]LoadoutSummon, len(context.Summons))
	for _, summon := range context.Summons {
		bySlot[summon.SlotID] = summon
	}
	selected := make([]LoadoutSummon, 0, len(summonSlotIDs))
	seen := map[uint32]bool{}
	for _, slotID := range summonSlotIDs {
		if slotID == 0 || slotID == EmptyHash {
			return nil, fmt.Errorf("召唤石 SlotID 必须为非零有效值")
		}
		if seen[slotID] {
			return nil, fmt.Errorf("召唤石 SlotID %d 重复", slotID)
		}
		seen[slotID] = true
		summon, ok := bySlot[slotID]
		if !ok {
			return nil, fmt.Errorf("召唤石 SlotID %d 不存在", slotID)
		}
		selected = append(selected, summon)
		mainHash, parseErr := ParseHashHex(summon.MainTraitHash)
		if parseErr == nil && mainHash != 0 && mainHash != EmptyHash && summon.MainTraitLevel > 0 {
			pairs = append(pairs, struct {
				hash  uint32
				level int
			}{mainHash, summon.MainTraitLevel})
		}
	}

	hashToID := traitHashMapWithRawKeys(cat)
	// Rebuilt weapon-only skills are not necessarily present in the craftable
	// sigil catalog. Their official ID was resolved from weapon/skill tables,
	// so prefer that ID over the raw-hash fallback used for unknown traits.
	if weapon != nil {
		for _, skill := range weapon.Skills {
			hash, parseErr := ParseHashHex(skill.TraitHash)
			if parseErr == nil && skill.TraitID != "" {
				hashToID[hash] = skill.TraitID
			}
		}
	}
	bonuses := simulateTraits(pairs, hashToID)
	totals := aggregateTraitEffects(bonuses)

	// Preserve real summon instance names in main-trait source attribution.
	sourcesByTraitID := map[string][]string{}
	if weapon != nil {
		for _, skill := range weapon.Skills {
			if skill.TraitID == "" {
				continue
			}
			source := fmt.Sprintf("武器 · %s · %s Lv%d", weapon.Name, skill.Name, skill.Level)
			sourcesByTraitID[skill.TraitID] = append(sourcesByTraitID[skill.TraitID], source)
		}
	}
	for _, summon := range selected {
		hash, parseErr := ParseHashHex(summon.MainTraitHash)
		if parseErr != nil {
			continue
		}
		traitID := hashToID[hash]
		if traitID != "" {
			sourcesByTraitID[traitID] = append(sourcesByTraitID[traitID], summon.Name)
		}
	}
	for _, bonus := range bonuses {
		for _, component := range bonus.Components {
			if !component.Additive || component.Label == "" {
				continue
			}
			for _, label := range canonicalEffectLabels(component.Label) {
				key := component.Unit + "|" + label
				for i := range totals {
					if totals[i].Key != key {
						continue
					}
					for _, source := range sourcesByTraitID[bonus.TraitID] {
						addTotalSource(&totals[i], source)
					}
				}
			}
		}
	}
	if weapon != nil {
		weaponSource := "武器 · " + weapon.Name
		if weapon.Total.HP != 0 {
			addTotal(&totals, "最大HP", "flat", weapon.Total.HP, "基础能力", weaponSource)
		}
		if weapon.Total.ATK != 0 {
			addTotal(&totals, "攻击力", "flat", weapon.Total.ATK, "基础能力", weaponSource)
		}
		if weapon.Total.CritRate != 0 {
			addTotal(&totals, "暴击率", "pct", weapon.Total.CritRate, "基础能力", weaponSource)
		}
		if weapon.Total.Stun != 0 {
			addTotal(&totals, "昏厥值", "flat", weapon.Total.Stun, "基础能力", weaponSource)
		}
	}
	factorCounts := loadoutPrimaryFactorCategoryCounts(cat, factorPrimaryHashes)
	masteryBonuses, err := loadoutMasteryPanelBonuses(ownerCode, masteryHexes, factorCounts)
	if err != nil {
		return nil, err
	}
	addPanelBonusesToTotals(&totals, masteryBonuses)
	panelBonuses := append([]LoadoutPanelBonus(nil), masteryBonuses...)
	for _, summon := range selected {
		label := summonSubParamLabel(summon.SubParamName)
		if label == "" || summon.SubParamUnit == "" {
			continue
		}
		addTotal(&totals, label, summon.SubParamUnit, summon.SubParamValue, "召唤石", summon.Name)
		panelBonuses = append(panelBonuses, LoadoutPanelBonus{Label: label, Unit: summon.SubParamUnit, Value: summon.SubParamValue, Source: summon.Name})
	}
	for _, bonus := range context.OverLimit {
		addTotal(&totals, bonus.Name, bonus.Unit, bonus.Value, "极限加成", "极限加成")
	}
	sortEffectTotals(totals)
	panelInput := loadoutPanelInputs{
		CharacterHP: float64(context.BaseHP), CharacterATK: float64(context.BaseATK),
		CharacterCritRate: context.BaseCritRate, CharacterStun: context.BaseStun,
		Bonuses: bonuses, Mastery: panelBonuses, OverLimit: context.OverLimit,
		Warnings: append([]string(nil), context.Warnings...),
	}
	if weapon != nil {
		panelInput.WeaponHP = weapon.Total.HP
		panelInput.WeaponATK = weapon.Total.ATK
		panelInput.WeaponCritRate = weapon.Total.CritRate
		panelInput.WeaponStun = weapon.Total.Stun
		panelInput.Warnings = append(panelInput.Warnings, weapon.Warnings...)
	}
	finalStats := calculateLoadoutFinalStats(panelInput)
	result := &LoadoutSimulation{Totals: totals, Bonuses: bonuses, FinalStats: &finalStats, Weapon: weapon}
	if weapon != nil {
		result.WeaponSkills = append([]LoadoutWeaponSkill(nil), weapon.Skills...)
	}
	return result, nil
}
