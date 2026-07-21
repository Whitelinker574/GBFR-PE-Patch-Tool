package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed data/summon_natural_rules_202.json
var summonNaturalRulesJSON []byte

// SummonNaturalRule describes one summon type's naturally generated pool.
// The type/main/sub hashes are cross-checked against the embedded game-table
// catalogs. Empty level lists deliberately mean that the secondary source only
// proves the fixed traits, not their fixed level values.
type SummonNaturalRule struct {
	TypeHash        string   `json:"typeHash"`
	Name            string   `json:"name"`
	EquipCost       int      `json:"equipCost"`
	Tier            string   `json:"tier"`
	TierIndex       int      `json:"tierIndex"`
	Variant         int      `json:"variant"`
	TypeName        string   `json:"typeName"`
	Mode            string   `json:"mode"`
	MainTraitHashes []string `json:"mainTraitHashes"`
	SubParamHashes  []string `json:"subParamHashes"`
	MainTraitLevels []int    `json:"mainTraitLevels"`
	SubParamLevels  []int    `json:"subParamLevels"`
}

type summonNaturalRuleFile struct {
	SchemaVersion int                 `json:"schemaVersion"`
	GameVersion   string              `json:"gameVersion"`
	SourceURL     string              `json:"sourceUrl"`
	SourceRole    string              `json:"sourceRole"`
	Rows          []SummonNaturalRule `json:"rows"`
}

func loadSummonNaturalRules() ([]SummonNaturalRule, error) {
	var payload summonNaturalRuleFile
	if err := json.Unmarshal(summonNaturalRulesJSON, &payload); err != nil {
		return nil, fmt.Errorf("解析召唤石天然规则失败: %w", err)
	}
	if payload.SchemaVersion != 1 || payload.GameVersion != "2.0.2" || len(payload.Rows) != 189 {
		return nil, fmt.Errorf("召唤石天然规则版本/数量异常: schema=%d game=%q rows=%d", payload.SchemaVersion, payload.GameVersion, len(payload.Rows))
	}
	return payload.Rows, nil
}

func summonNaturalRuleByHash(rules []SummonNaturalRule) (map[uint32]SummonNaturalRule, error) {
	result := make(map[uint32]SummonNaturalRule, len(rules))
	for _, rule := range rules {
		hash, err := ParseHashHex(rule.TypeHash)
		if err != nil {
			return nil, fmt.Errorf("召唤石天然规则种类哈希无效 %q: %w", rule.TypeHash, err)
		}
		if _, duplicate := result[hash]; duplicate {
			return nil, fmt.Errorf("召唤石天然规则种类哈希重复 0x%08X", hash)
		}
		result[hash] = rule
	}
	return result, nil
}

func containsSummonRuleHash(values []string, hash uint32) bool {
	for _, value := range values {
		parsed, err := ParseHashHex(value)
		if err == nil && parsed == hash {
			return true
		}
	}
	return false
}

func containsSummonRuleLevel(values []int, level uint32) bool {
	for _, value := range values {
		if value >= 0 && uint32(value) == level {
			return true
		}
	}
	return false
}

func validateSummonNaturalChange(draft, existing SummonTraitState) error {
	rules, err := loadSummonNaturalRules()
	if err != nil {
		return err
	}
	byHash, err := summonNaturalRuleByHash(rules)
	if err != nil {
		return err
	}
	rule, ok := byHash[draft.TypeHash]
	if !ok {
		return fmt.Errorf("召唤石种类 0x%08X 没有 2.0.2 天然规则", draft.TypeHash)
	}
	creating := existing.TypeHash == EmptyHash
	sameType := !creating && draft.TypeHash == existing.TypeHash
	mainUnchanged := sameType && draft.MainTraitHash == existing.MainTraitHash && draft.MainTraitLevel == existing.MainTraitLevel
	subUnchanged := sameType && draft.SubParamHash == existing.SubParamHash && draft.SubParamLevel == existing.SubParamLevel
	legacyUnchanged := mainUnchanged && subUnchanged
	if legacyUnchanged {
		// Rank is a separate saved field. A real 102-record save proves it does
		// not equal the summon rarity tier, so changing Rank must not be checked
		// against Tier/TierIndex.
		return nil
	}
	if !mainUnchanged && !containsSummonRuleHash(rule.MainTraitHashes, draft.MainTraitHash) {
		return fmt.Errorf("主加护 0x%08X 不在召唤石 0x%08X 的天然词池", draft.MainTraitHash, draft.TypeHash)
	}
	if !subUnchanged && !containsSummonRuleHash(rule.SubParamHashes, draft.SubParamHash) {
		return fmt.Errorf("副词条 0x%08X 不在召唤石 0x%08X 的天然词池", draft.SubParamHash, draft.TypeHash)
	}
	if rule.Mode == "固定" {
		// The referenced probability page proves the fixed hashes but explicitly
		// omits fixed-template levels. Do not invent those levels for a new item.
		if creating || draft.TypeHash != existing.TypeHash {
			return fmt.Errorf("固定模板 %s 的词条已验证，但固定等级尚无表内证据；暂不允许新增/换入", rule.Name)
		}
		if draft.MainTraitLevel != existing.MainTraitLevel || draft.SubParamLevel != existing.SubParamLevel {
			return fmt.Errorf("固定模板 %s 的等级尚无表内证据，只能保留原值", rule.Name)
		}
		return nil
	}
	if !mainUnchanged && !containsSummonRuleLevel(rule.MainTraitLevels, draft.MainTraitLevel) {
		return fmt.Errorf("主加护等级 %d 不在召唤石 0x%08X 的天然等级集合", draft.MainTraitLevel, draft.TypeHash)
	}
	if !subUnchanged && !containsSummonRuleLevel(rule.SubParamLevels, draft.SubParamLevel) {
		return fmt.Errorf("副词条档位 %d 不在召唤石 0x%08X 的天然档位集合", draft.SubParamLevel, draft.TypeHash)
	}
	return nil
}
