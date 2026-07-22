package backend

import (
	"strings"
	"testing"
)

func auditedSummonTraitStates(t *testing.T) (*summonStatCatalog, SummonTraitState, SummonTraitState) {
	t.Helper()
	catalog, err := loadSummonStatCatalog()
	if err != nil {
		t.Fatal(err)
	}
	var typeHash, mainHash, subHash uint32
	var mainLevel, subLevel uint32
	rules, err := loadSummonNaturalRules()
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range rules {
		if rule.Mode != "随机" || len(rule.MainTraitHashes) == 0 || len(rule.SubParamHashes) == 0 || len(rule.MainTraitLevels) == 0 || len(rule.SubParamLevels) == 0 {
			continue
		}
		typeHash, err = ParseHashHex(rule.TypeHash)
		if err != nil {
			t.Fatal(err)
		}
		mainHash, err = ParseHashHex(rule.MainTraitHashes[0])
		if err != nil {
			t.Fatal(err)
		}
		subHash, err = ParseHashHex(rule.SubParamHashes[0])
		if err != nil {
			t.Fatal(err)
		}
		mainLevel, subLevel = uint32(rule.MainTraitLevels[0]), uint32(rule.SubParamLevels[0])
		break
	}
	if typeHash == 0 || mainHash == 0 || subHash == 0 {
		t.Fatal("audited summon catalog did not contain one complete natural state")
	}
	existing := SummonTraitState{
		TypeHash: typeHash, MainTraitHash: mainHash, SubParamHash: subHash,
		MainTraitLevel: mainLevel, SubParamLevel: subLevel, Rank: 2,
	}
	draft := existing
	draft.Rank = 3
	return catalog, existing, draft
}

func TestValidateSummonTraitChangeAcceptsAuditedNaturalState(t *testing.T) {
	catalog, existing, draft := auditedSummonTraitStates(t)
	if err := validateSummonTraitChange(catalog, draft, existing); err != nil {
		t.Fatalf("audited natural summon edit was rejected: %v", err)
	}
}

func TestValidateSummonTraitChangePreservesButCannotChangeUnknownLegacyMain(t *testing.T) {
	catalog, existing, draft := auditedSummonTraitStates(t)
	existing.MainTraitHash = 0xDEADBEEF
	existing.MainTraitLevel = 14
	draft.MainTraitHash = existing.MainTraitHash
	draft.MainTraitLevel = existing.MainTraitLevel
	if err := validateSummonTraitChange(catalog, draft, existing); err != nil {
		t.Fatalf("unchanged legacy main trait should be preservable: %v", err)
	}

	draft.MainTraitLevel--
	err := validateSummonTraitChange(catalog, draft, existing)
	if err == nil || !strings.Contains(err.Error(), "main") {
		t.Fatalf("changing a legacy main trait must fail closed, got: %v", err)
	}

	_, _, audited := auditedSummonTraitStates(t)
	draft.MainTraitHash = audited.MainTraitHash
	draft.MainTraitLevel = audited.MainTraitLevel
	err = validateSummonTraitChange(catalog, draft, existing)
	if err == nil || !strings.Contains(err.Error(), "main") {
		t.Fatalf("replacing an unknown legacy main trait with a catalogued trait must fail closed, got: %v", err)
	}
}

func TestValidateSummonTraitChangeRejectsUnsafeSubLevelAndRank(t *testing.T) {
	catalog, existing, draft := auditedSummonTraitStates(t)
	draft.SubParamLevel = summonSubParamSafetyMaxLevel + 1
	if err := validateSummonTraitChange(catalog, draft, existing); err == nil {
		t.Fatal("unsafe summon sub level was accepted")
	}
	draft = existing
	draft.Rank = 4
	if err := validateSummonTraitChange(catalog, draft, existing); err == nil {
		t.Fatal("unsafe summon rank was accepted")
	}
}

func TestValidateSummonTraitChangeRejectsCrossPoolCombination(t *testing.T) {
	catalog, existing, draft := auditedSummonTraitStates(t)
	rules, err := loadSummonNaturalRules()
	if err != nil {
		t.Fatal(err)
	}
	rulesByHash, err := summonNaturalRuleByHash(rules)
	if err != nil {
		t.Fatal(err)
	}
	rule, ok := rulesByHash[draft.TypeHash]
	if !ok {
		t.Fatalf("missing natural rule for summon type 0x%08X", draft.TypeHash)
	}
	found := false
	for hash := range catalog.main {
		if !containsSummonRuleHash(rule.MainTraitHashes, hash) {
			draft.MainTraitHash = hash
			found = true
			break
		}
	}
	if !found {
		t.Fatal("catalog did not contain an incompatible main trait")
	}
	if err := validateSummonTraitChange(catalog, draft, existing); err == nil || !strings.Contains(err.Error(), "天然词池") {
		t.Fatalf("cross-pool main trait was not rejected: %v", err)
	}
}

func TestFixedSummonRulesCarryUnpackedLevelEvidence(t *testing.T) {
	rules, err := loadSummonNaturalRules()
	if err != nil {
		t.Fatal(err)
	}
	var roland *SummonNaturalRule
	for index := range rules {
		rule := &rules[index]
		if rule.Mode == "固定" && (len(rule.MainTraitLevels) == 0 || len(rule.SubParamLevels) == 0) {
			t.Fatalf("固定模板 %s (%s) 缺少解包等级证据", rule.Name, rule.TypeHash)
		}
		if rule.TypeHash == "0x81ECEC7F" {
			roland = rule
		}
	}
	if roland == nil || len(roland.MainTraitLevels) != 1 || roland.MainTraitLevels[0] != 11 ||
		len(roland.SubParamLevels) != 1 || roland.SubParamLevels[0] != 5 {
		t.Fatalf("罗兰固定模板等级与 2.0.2 解包表不一致: %+v", roland)
	}
}
