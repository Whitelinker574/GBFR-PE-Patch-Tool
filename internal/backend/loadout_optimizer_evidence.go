package backend

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
)

type LoadoutOptimizerTraitLevel struct {
	Level          int              `json:"level"`
	Effect         string           `json:"effect"`
	Totals         []EffectTotal    `json:"totals"`
	Components     []BonusComponent `json:"components"`
	MaxHPCondition float64          `json:"maxHpCondition,omitempty"`
	Warning        string           `json:"warning,omitempty"`
}

type LoadoutOptimizerTraitCurve struct {
	TraitID  string                       `json:"traitId"`
	Hash     string                       `json:"hash"`
	Name     string                       `json:"name"`
	CatLabel string                       `json:"catLabel"`
	MaxLevel int                          `json:"maxLevel"`
	Levels   []LoadoutOptimizerTraitLevel `json:"levels"`
}

type LoadoutOptimizerEvidence struct {
	SchemaVersion  int                          `json:"schemaVersion"`
	DataVersion    string                       `json:"dataVersion"`
	FormulaVersion string                       `json:"formulaVersion"`
	Traits         []LoadoutOptimizerTraitCurve `json:"traits"`
}

var (
	loadoutOptimizerEvidenceOnce sync.Once
	loadoutOptimizerEvidenceData *LoadoutOptimizerEvidence
	loadoutOptimizerEvidenceErr  error
)

func loadoutOptimizerEvidence() (*LoadoutOptimizerEvidence, error) {
	loadoutOptimizerEvidenceOnce.Do(func() {
		catalog, err := LoadCatalog()
		if err != nil {
			loadoutOptimizerEvidenceErr = err
			return
		}
		hashToID := buildTraitHashToID(catalog)
		curves := make([]LoadoutOptimizerTraitCurve, 0, len(catalog.Traits))
		seen := make(map[string]bool, len(catalog.Traits))
		for index := range catalog.Traits {
			trait := &catalog.Traits[index]
			hash, parseErr := ParseHashHex(trait.Hash)
			if parseErr != nil || hash == 0 || hash == EmptyHash {
				continue
			}
			traitID := canonicalTraitValueID(trait.InternalID)
			if traitID == "" || seen[traitID] {
				continue
			}
			definition := loadTraitValues()[traitID]
			if definition == nil || definition.MaxLevel <= 0 {
				continue
			}
			seen[traitID] = true
			curve := LoadoutOptimizerTraitCurve{
				TraitID: traitID, Hash: hashText(hash), Name: definition.Name,
				CatLabel: definition.CatLabel, MaxLevel: definition.MaxLevel,
				Levels: make([]LoadoutOptimizerTraitLevel, 0, definition.MaxLevel),
			}
			if curve.Name == "" {
				curve.Name = loadoutTraitDisplayName(catalog, hash)
			}
			for level := 1; level <= definition.MaxLevel; level++ {
				bonuses := simulateTraits([]struct {
					hash  uint32
					level int
				}{{hash: hash, level: level}}, hashToID)
				if len(bonuses) != 1 {
					loadoutOptimizerEvidenceErr = fmt.Errorf("优化证据 %s Lv%d 无法唯一解析", traitID, level)
					return
				}
				bonus := bonuses[0]
				curve.Levels = append(curve.Levels, LoadoutOptimizerTraitLevel{
					Level: level, Effect: bonus.Effect, Totals: aggregateTraitEffects(bonuses),
					Components:     append([]BonusComponent(nil), bonus.Components...),
					MaxHPCondition: bonus.MaxHPCondition, Warning: bonus.Warning,
				})
			}
			if loadoutOptimizerEvidenceErr != nil {
				return
			}
			curves = append(curves, curve)
		}
		if loadoutOptimizerEvidenceErr != nil {
			return
		}
		sort.Slice(curves, func(i, j int) bool { return curves[i].TraitID < curves[j].TraitID })
		loadoutOptimizerEvidenceData = &LoadoutOptimizerEvidence{
			SchemaVersion: 1, DataVersion: "2.0.2", FormulaVersion: "2.0.2-v2-tardis98", Traits: curves,
		}
	})
	if loadoutOptimizerEvidenceErr != nil {
		return nil, loadoutOptimizerEvidenceErr
	}
	return loadoutOptimizerEvidenceData, nil
}

func (a *App) LoadoutOptimizerEvidence() (*LoadoutOptimizerEvidence, error) {
	return loadoutOptimizerEvidence()
}

type LoadoutOptimizerFixedBonus struct {
	TraitID string `json:"traitId"`
	Name    string `json:"name,omitempty"`
	Level   int    `json:"level"`
}

type LoadoutOptimizerEquipmentOption struct {
	ID              string                             `json:"id"`
	Label           string                             `json:"label"`
	BaseStatDeltas  map[string]float64                 `json:"baseStatDeltas"`
	FixedBonuses    []LoadoutOptimizerFixedBonus       `json:"fixedBonuses"`
	FixedTotals     []EffectTotal                      `json:"fixedTotals"`
	ApplyPayload    map[string]any                     `json:"applyPayload"`
	Requires        map[string][]string                `json:"requires,omitempty"`
	UnresolvedAtoms []string                           `json:"unresolvedAtoms"`
	Variants        []LoadoutOptimizerEquipmentVariant `json:"variants,omitempty"`
}

type LoadoutOptimizerEquipmentVariant struct {
	ID              string                       `json:"id"`
	Label           string                       `json:"label"`
	FixedBonuses    []LoadoutOptimizerFixedBonus `json:"fixedBonuses"`
	ApplyPayload    map[string]any               `json:"applyPayload"`
	UnresolvedAtoms []string                     `json:"unresolvedAtoms"`
}

type LoadoutOptimizerEquipmentStage struct {
	Key     string                            `json:"key"`
	Choose  int                               `json:"choose"`
	Unique  bool                              `json:"unique"`
	Options []LoadoutOptimizerEquipmentOption `json:"options"`
}

type LoadoutOptimizerDomainSnapshot struct {
	SchemaVersion    int                              `json:"schemaVersion"`
	Domain           string                           `json:"domain"`
	DataVersion      string                           `json:"dataVersion"`
	FormulaVersion   string                           `json:"formulaVersion"`
	InputHash        string                           `json:"inputHash"`
	TableHash        string                           `json:"tableHash"`
	CatalogHash      string                           `json:"catalogHash"`
	BaseStats        map[string]float64               `json:"baseStats"`
	BaseFixedBonuses []LoadoutOptimizerFixedBonus     `json:"baseFixedBonuses"`
	BaseFixedTotals  []EffectTotal                    `json:"baseFixedTotals"`
	BaseDefenseZones []LoadoutDefenseZone             `json:"baseDefenseZones"`
	BaseSelection    map[string]any                   `json:"baseSelection"`
	CompleteStages   []string                         `json:"completeStages"`
	Stages           []LoadoutOptimizerEquipmentStage `json:"stages"`
	Warnings         []string                         `json:"warnings"`
}

func optimizerHash(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

func optimizerFixedBonus(traitID, name string, level int) LoadoutOptimizerFixedBonus {
	return LoadoutOptimizerFixedBonus{TraitID: canonicalTraitValueID(traitID), Name: name, Level: level}
}

func optimizerNonPanelTotals(totals []EffectTotal) []EffectTotal {
	result := make([]EffectTotal, 0, len(totals))
	for _, total := range totals {
		label := strings.TrimSpace(total.Label)
		if strings.Contains(label, "造成的伤害") || strings.Contains(label, "冷却时间") {
			result = append(result, total)
		}
	}
	return result
}

func optimizerNonPanelTotalDelta(current, baseline []EffectTotal) []EffectTotal {
	baselineByKey := make(map[string]float64, len(baseline))
	for _, total := range optimizerNonPanelTotals(baseline) {
		baselineByKey[total.Unit+"|"+strings.TrimSpace(total.Label)] = total.Value
	}
	result := optimizerNonPanelTotals(current)
	filtered := result[:0]
	for index := range result {
		key := result[index].Unit + "|" + strings.TrimSpace(result[index].Label)
		result[index].Value -= baselineByKey[key]
		if math.Abs(result[index].Value) > 1e-9 {
			filtered = append(filtered, result[index])
		}
	}
	return filtered
}

func optimizerFinalStatDelta(current, baseline *LoadoutFinalStats) map[string]float64 {
	result := map[string]float64{}
	if current == nil || baseline == nil {
		return result
	}
	result["attack"] = float64(current.Attack - baseline.Attack)
	result["hp"] = float64(current.HP - baseline.HP)
	result["critRate"] = current.CritRate - baseline.CritRate
	result["stun"] = current.StunPower - baseline.StunPower
	result["normalDamageCap"] = current.NormalDamageCap - baseline.NormalDamageCap
	result["abilityDamageCap"] = current.AbilityDamageCap - baseline.AbilityDamageCap
	result["skyboundDamageCap"] = current.SkyboundDamageCap - baseline.SkyboundDamageCap
	result["chainDamageCap"] = current.ChainDamageCap - baseline.ChainDamageCap
	return result
}

func optimizerSummonPanelDelta(summon LoadoutSummon) (map[string]float64, bool) {
	result := map[string]float64{}
	name := strings.TrimSpace(summon.SubParamName)
	value := summon.SubParamValue
	recognized := true
	switch {
	case strings.Contains(name, "攻击"):
		result["attack"] = value
	case strings.Contains(name, "HP") || strings.Contains(name, "体力"):
		result["hp"] = value
	case strings.Contains(name, "暴击"):
		result["critRate"] = value
	case strings.Contains(name, "昏厥"):
		result["stun"] = value * summonSubParamPanelScale(name, summon.SubParamUnit)
	default:
		recognized = name == ""
	}
	return result, recognized
}

func optimizerWeaponSkillVariants(weapon *LoadoutWeaponContext, traitIDs map[uint32]string) []LoadoutOptimizerEquipmentVariant {
	if weapon == nil || len(weapon.SkillSlots) == 0 {
		return nil
	}
	type choice struct {
		hash  string
		name  string
		level int
	}
	effectiveLevels := map[string]int{}
	for _, skill := range weapon.Skills {
		effectiveLevels[strings.ToUpper(strings.TrimPrefix(strings.TrimSpace(skill.TraitHash), "0x"))] = skill.Level
	}
	groups := make([][]choice, 0, len(weapon.SkillSlots))
	for _, slot := range weapon.SkillSlots {
		options := make([]choice, 0, len(slot.Options)+1)
		seen := map[string]bool{}
		for _, option := range slot.Options {
			hash := strings.ToUpper(strings.TrimPrefix(strings.TrimSpace(option.Hash), "0x"))
			if hash == "" || seen[hash] {
				continue
			}
			seen[hash] = true
			level := option.Level
			if effectiveLevels[hash] > 0 {
				level = effectiveLevels[hash]
			}
			options = append(options, choice{hash: hash, name: option.Name, level: level})
		}
		currentHash := strings.ToUpper(strings.TrimPrefix(strings.TrimSpace(slot.CurrentHash), "0x"))
		if currentHash != "" && !seen[currentHash] {
			options = append(options, choice{hash: currentHash, name: slot.CurrentName, level: slot.CurrentLevel})
		}
		if len(options) == 0 {
			return nil
		}
		groups = append(groups, options)
	}
	variants := make([]LoadoutOptimizerEquipmentVariant, 0, 96)
	var visit func(int, []choice)
	visit = func(index int, picked []choice) {
		if len(variants) >= 256 {
			return
		}
		if index == len(groups) {
			hashes := make([]string, 0, len(picked))
			names := make([]string, 0, len(picked))
			variant := LoadoutOptimizerEquipmentVariant{ApplyPayload: map[string]any{}, UnresolvedAtoms: []string{}}
			for _, item := range picked {
				hashes = append(hashes, item.hash)
				names = append(names, item.name)
				hash, err := ParseHashHex(item.hash)
				traitID := ""
				if err == nil {
					traitID = resolveTraitValueID(hash, traitIDs)
				}
				if traitID == "" {
					variant.UnresolvedAtoms = append(variant.UnresolvedAtoms, "weapon-skill:"+item.hash)
				} else if item.level > 0 {
					variant.FixedBonuses = append(variant.FixedBonuses, optimizerFixedBonus(traitID, item.name, item.level))
				}
			}
			variant.ID = strings.Join(hashes, ":")
			variant.Label = strings.Join(names, " / ")
			variant.ApplyPayload = map[string]any{"weaponSlotId": weapon.SlotID, "weaponSkillHashes": hashes}
			variants = append(variants, variant)
			return
		}
		for _, option := range groups[index] {
			visit(index+1, append(picked, option))
		}
	}
	visit(0, nil)
	sort.Slice(variants, func(i, j int) bool { return variants[i].ID < variants[j].ID })
	return variants
}

// LoadoutOptimizerInventorySnapshot builds a deployable, versioned equipment
// domain from one parsed save. Weapon and bound Wrightstone options are split
// into stages but linked with explicit requirements so invalid cross-weapon
// combinations can never be emitted by the solver.
func (a *App) LoadoutOptimizerInventorySnapshot(path, charaHex string, loadoutUnitID uint32) (*LoadoutOptimizerDomainSnapshot, error) {
	saveSnapshot, err := loadLoadoutReadSnapshot(path)
	if err != nil {
		return nil, err
	}
	parsed, err := saveSnapshot.parsedSave()
	if err != nil {
		return nil, err
	}
	edit, err := a.loadoutEditContextFromLoaded(saveSnapshot.save, charaHex)
	if err != nil {
		return nil, err
	}
	stats, err := a.loadoutStatContextFromLoaded(path, charaHex, parsed, saveSnapshot.save, false)
	if err != nil {
		return nil, err
	}
	catalog, err := LoadCatalog()
	if err != nil {
		return nil, err
	}
	evidence, err := loadoutOptimizerEvidence()
	if err != nil {
		return nil, err
	}
	baseline, err := a.loadoutSimulateBuildFromLoaded(path, charaHex, 0, nil, nil, nil, nil, catalog, saveSnapshot.save, stats, false)
	if err != nil {
		return nil, err
	}
	traitIDs := traitHashMapWithRawKeys(catalog)
	baseSelection := map[string]any{
		"weapon":      []string{},
		"wrightstone": []string{},
		"summons":     []string{},
		"mastery":     []string{},
	}
	if loadoutUnitID != 0 {
		groups, listErr := a.loadoutListFromLoaded(saveSnapshot.save)
		if listErr != nil {
			return nil, listErr
		}
		for _, group := range groups {
			if !strings.EqualFold(group.CharaHash, charaHex) {
				continue
			}
			for _, loadout := range group.Loadouts {
				if loadout.UnitID != loadoutUnitID {
					continue
				}
				if loadout.WeaponSlotID != 0 {
					weaponID := fmt.Sprintf("weapon:%d", loadout.WeaponSlotID)
					baseSelection["weapon"] = []string{weaponID}
					baseSelection["wrightstone"] = []string{"wrightstone:" + weaponID}
				}
				baseSelection["mastery"] = []string{fmt.Sprintf("mastery:%d", loadout.UnitID)}
				break
			}
		}
	}
	if len(stats.EquippedSummonSlotIDs) > 0 {
		summons := make([]string, 0, len(stats.EquippedSummonSlotIDs))
		for _, slotID := range stats.EquippedSummonSlotIDs {
			if slotID != 0 {
				summons = append(summons, fmt.Sprintf("summon:%d", slotID))
			}
		}
		baseSelection["summons"] = summons
	}
	result := &LoadoutOptimizerDomainSnapshot{
		SchemaVersion: 1, Domain: "inventory", DataVersion: evidence.DataVersion, FormulaVersion: evidence.FormulaVersion,
		InputHash: optimizerHash(saveSnapshot.save.data), TableHash: optimizerHash(evidence), CatalogHash: optimizerHash(catalog),
		BaseStats: map[string]float64{
			"attack": float64(baseline.FinalStats.Attack), "hp": float64(baseline.FinalStats.HP),
			"critRate": baseline.FinalStats.CritRate, "stun": baseline.FinalStats.StunPower,
			"normalDamageCap":   baseline.FinalStats.NormalDamageCap,
			"abilityDamageCap":  baseline.FinalStats.AbilityDamageCap,
			"skyboundDamageCap": baseline.FinalStats.SkyboundDamageCap,
			"chainDamageCap":    baseline.FinalStats.ChainDamageCap,
		},
		BaseFixedTotals: optimizerNonPanelTotals(baseline.DynamicTotals),
		BaseSelection:   baseSelection, CompleteStages: []string{"weapon", "wrightstone", "summons", "mastery"},
		Warnings: append([]string(nil), stats.Warnings...),
	}
	for _, bonus := range baseline.Bonuses {
		if bonus.TraitID != "" && bonus.Level > 0 {
			result.BaseFixedBonuses = append(result.BaseFixedBonuses, optimizerFixedBonus(bonus.TraitID, bonus.Name, bonus.Level))
		}
	}
	if baseline.FinalStats.DefenseModel != nil {
		result.BaseDefenseZones = append(result.BaseDefenseZones, baseline.FinalStats.DefenseModel.Zones...)
	}
	weaponStage := LoadoutOptimizerEquipmentStage{Key: "weapon", Choose: 1, Unique: true}
	wrightstoneStage := LoadoutOptimizerEquipmentStage{Key: "wrightstone", Choose: 1, Unique: true}
	for _, pick := range edit.Weapons {
		weapon, readErr := readLoadoutWeaponContext(saveSnapshot.save, pick.SlotID)
		if readErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("武器 SlotID %d 未进入优化域: %v", pick.SlotID, readErr))
			continue
		}
		applyMasterProgressWeaponSkillLevels(weapon.Skills, stats.PermanentGrowth)
		weaponID := fmt.Sprintf("weapon:%d", pick.SlotID)
		weaponOption := LoadoutOptimizerEquipmentOption{
			ID: weaponID, Label: localizedLoadoutWeaponName(weapon),
			BaseStatDeltas: map[string]float64{"attack": weapon.Total.ATK, "hp": weapon.Total.HP, "stun": weapon.Total.Stun, "critRate": weapon.Total.CritRate},
			ApplyPayload:   map[string]any{"weaponSlotId": pick.SlotID}, UnresolvedAtoms: []string{},
		}
		weaponOption.Variants = optimizerWeaponSkillVariants(weapon, traitIDs)
		if !weapon.FormulaVerified {
			weaponOption.UnresolvedAtoms = append(weaponOption.UnresolvedAtoms, "weapon-formula-unverified")
		}
		for _, skill := range weapon.Skills {
			if skill.Level > 0 && skill.TraitID != "" {
				weaponOption.FixedBonuses = append(weaponOption.FixedBonuses, optimizerFixedBonus(skill.TraitID, skill.Name, skill.Level))
			} else if skill.Level > 0 {
				weaponOption.UnresolvedAtoms = append(weaponOption.UnresolvedAtoms, "weapon-skill:"+skill.TraitHash)
			}
		}
		weaponStage.Options = append(weaponStage.Options, weaponOption)
		stoneOption := LoadoutOptimizerEquipmentOption{
			ID: "wrightstone:" + weaponID, Label: "未镶嵌祝福石", BaseStatDeltas: map[string]float64{},
			ApplyPayload: map[string]any{"weaponSlotId": pick.SlotID}, Requires: map[string][]string{"weapon": {weaponID}}, UnresolvedAtoms: []string{},
		}
		if weapon.Wrightstone != nil {
			stoneOption.Label = localizedLoadoutWrightstoneName(weapon.Wrightstone)
			stoneOption.ApplyPayload["wrightstoneHash"] = weapon.Wrightstone.Hash
			for _, trait := range weapon.Wrightstone.Traits {
				if trait.Level > 0 && trait.TraitID != "" {
					stoneOption.FixedBonuses = append(stoneOption.FixedBonuses, optimizerFixedBonus(trait.TraitID, trait.Name, trait.Level))
				}
			}
			if weapon.Wrightstone.Evidence == "" {
				stoneOption.UnresolvedAtoms = append(stoneOption.UnresolvedAtoms, "wrightstone-evidence-missing")
			}
		}
		wrightstoneStage.Options = append(wrightstoneStage.Options, stoneOption)
	}

	summonStage := LoadoutOptimizerEquipmentStage{Key: "summons", Choose: 0, Unique: true}
	if stats.SummonSystemAvailable {
		summonStage.Choose = 4
	}
	for _, summon := range stats.Summons {
		panelDelta, subParamResolved := optimizerSummonPanelDelta(summon)
		option := LoadoutOptimizerEquipmentOption{
			ID: fmt.Sprintf("summon:%d", summon.SlotID), Label: summon.Name,
			BaseStatDeltas: panelDelta, ApplyPayload: map[string]any{"slotId": summon.SlotID}, UnresolvedAtoms: []string{},
		}
		if !subParamResolved {
			option.UnresolvedAtoms = append(option.UnresolvedAtoms, "summon-sub-param:"+summon.SubParamHash)
		}
		if hash, parseErr := ParseHashHex(summon.MainTraitHash); parseErr == nil && summon.MainTraitLevel > 0 {
			traitID := resolveTraitValueID(hash, traitIDs)
			if traitID == "" {
				option.UnresolvedAtoms = append(option.UnresolvedAtoms, "summon-main-trait:"+summon.MainTraitHash)
			} else {
				option.FixedBonuses = append(option.FixedBonuses, optimizerFixedBonus(traitID, summon.MainTraitName, summon.MainTraitLevel))
			}
		}
		summonStage.Options = append(summonStage.Options, option)
	}

	masteryStage := LoadoutOptimizerEquipmentStage{Key: "mastery", Choose: 1, Unique: true}
	for _, source := range edit.MasterySources {
		simulation, simulationErr := a.loadoutSimulateBuildFromLoaded(path, charaHex, 0, nil, nil, source.NodeHashes, nil, catalog, saveSnapshot.save, stats, false)
		if simulationErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("专精方案 %s 未进入优化域: %v", source.Name, simulationErr))
			continue
		}
		label := strings.TrimSpace(source.Name)
		if label == "" {
			label = fmt.Sprintf("专精方案 %02d", source.Slot)
		}
		masteryStage.Options = append(masteryStage.Options, LoadoutOptimizerEquipmentOption{
			ID: fmt.Sprintf("mastery:%d", source.UnitID), Label: label,
			BaseStatDeltas: optimizerFinalStatDelta(simulation.FinalStats, baseline.FinalStats),
			FixedTotals:    optimizerNonPanelTotalDelta(simulation.DynamicTotals, baseline.DynamicTotals),
			ApplyPayload:   map[string]any{"unitId": source.UnitID, "nodeHashes": source.NodeHashes}, UnresolvedAtoms: []string{},
		})
	}
	if len(masteryStage.Options) == 0 {
		masteryStage.Options = append(masteryStage.Options, LoadoutOptimizerEquipmentOption{
			ID: "mastery:none", Label: "无专精方案", BaseStatDeltas: map[string]float64{},
			ApplyPayload: map[string]any{"nodeHashes": []string{}}, UnresolvedAtoms: []string{"mastery-source-missing"},
		})
	}
	result.Stages = []LoadoutOptimizerEquipmentStage{weaponStage, wrightstoneStage, summonStage, masteryStage}
	for index := range result.Stages {
		sort.Slice(result.Stages[index].Options, func(i, j int) bool { return result.Stages[index].Options[i].ID < result.Stages[index].Options[j].ID })
	}
	return result, nil
}
