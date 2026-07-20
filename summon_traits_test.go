package main

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
	for hash := range catalog.types {
		typeHash = hash
		break
	}
	for hash, option := range catalog.main {
		if option.MaxLevel > 0 {
			mainHash = hash
			mainLevel = uint32(option.MaxLevel)
			if mainLevel > summonMainTraitSafetyMaxLevel {
				mainLevel = summonMainTraitSafetyMaxLevel
			}
			break
		}
	}
	for hash, option := range catalog.sub {
		if option.MaxLevel >= 0 && option.MaxLevel < len(option.Values) {
			subHash = hash
			subLevel = uint32(option.MaxLevel)
			if subLevel > summonSubParamSafetyMaxLevel {
				subLevel = summonSubParamSafetyMaxLevel
			}
			break
		}
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
