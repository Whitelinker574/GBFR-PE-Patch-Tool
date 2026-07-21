package main

import (
	"encoding/json"
	"os"
	"slices"
	"sort"
	"testing"
)

type sigilTableAuditEvidence struct {
	TableCounts struct {
		Gem          int `json:"gem"`
		SkillStatus  int `json:"skill_status"`
		SkillLot     int `json:"skill_lot"`
		SkillTypeLot int `json:"skill_type_lot"`
	} `json:"tableCounts"`
	RawTableSHA256 map[string]string `json:"rawTableSha256"`
	Summary        map[string]int    `json:"summary"`
	TraitSummary   struct {
		Verified int `json:"verified"`
		Mismatch int `json:"mismatch"`
	} `json:"traitSummary"`
	Rows           []struct {
		InternalID               string   `json:"internalId"`
		Status                   string   `json:"status"`
		GamePrimaryTraitID       string   `json:"gamePrimaryTraitId"`
		GameSecondaryTraitIDs    []string `json:"gameSecondaryTraitIds"`
		CatalogSecondaryTraitIDs []string `json:"catalogSecondaryTraitIds"`
	} `json:"rows"`
	TraitRows []struct {
		InternalID                       string `json:"internalId"`
		Status                           string `json:"status"`
		GameEffectCurveMaxLevel          int    `json:"gameEffectCurveMaxLevel"`
		AppearsAsPrimaryInGemTable       bool   `json:"appearsAsPrimaryInGemTable"`
		AppearsAsSecondaryInGemLotTables bool   `json:"appearsAsSecondaryInGemLotTables"`
	} `json:"traitRows"`
}

func TestSigilCatalogMatchesFreshLocal202TableEvidence(t *testing.T) {
	raw, err := os.ReadFile("docs/evidence/sigil-table-audit-202.json")
	if err != nil {
		t.Fatal(err)
	}
	var evidence sigilTableAuditEvidence
	if err := json.Unmarshal(raw, &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence.TableCounts.Gem != 1034 || evidence.TableCounts.SkillStatus != 6320 || evidence.TableCounts.SkillLot != 439 || evidence.TableCounts.SkillTypeLot != 21 {
		t.Fatalf("fresh local table counts changed: %+v", evidence.TableCounts)
	}
	for name, want := range map[string]string{
		"gem.tbl":            "F6B39C0FF9A190B3DD44FEFEAC84180D5FDA8254636DEEA1A8181336B1EA2C99",
		"skill_status.tbl":   "96D56E65F107FD925B131D86959C9F829CE7102E6BDD39C7C6F3E80F663E7563",
		"skill_lot.tbl":      "CDE2D4DD7D0874941E3148ED5FB50537638D51C222C213682316DDCAC17D2BDA",
		"skill_type_lot.tbl": "D7589A89E7AAE25DD966CCF5E9F75126C4B62E8464E16372553E67187D3BE025",
	} {
		if got := evidence.RawTableSHA256[name]; got != want {
			t.Errorf("%s SHA-256 = %s; want %s", name, got, want)
		}
	}
	if len(evidence.Summary) != 1 || evidence.Summary["verified"] != 184 || len(evidence.Rows) != 184 {
		t.Fatalf("sigil table audit is not closed: summary=%v rows=%d", evidence.Summary, len(evidence.Rows))
	}
	if evidence.TraitSummary.Verified != 184 || evidence.TraitSummary.Mismatch != 0 || len(evidence.TraitRows) != 184 {
		t.Fatalf("trait table audit is not closed: summary=%+v rows=%d", evidence.TraitSummary, len(evidence.TraitRows))
	}

	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Sigils) != len(evidence.Rows) {
		t.Fatalf("catalog rows=%d, fresh table-backed rows=%d", len(catalog.Sigils), len(evidence.Rows))
	}
	for _, row := range evidence.Rows {
		if row.Status != "verified" {
			t.Errorf("%s evidence status=%s", row.InternalID, row.Status)
			continue
		}
		sigil, err := catalog.RequireSigil(row.InternalID)
		if err != nil {
			t.Error(err)
			continue
		}
		if sigil.PrimaryTraitID != row.GamePrimaryTraitID {
			t.Errorf("%s primary=%s, gem.tbl=%s", row.InternalID, sigil.PrimaryTraitID, row.GamePrimaryTraitID)
		}
		got := append([]string(nil), sigil.AllowedSecondaryTraitIDs...)
		sort.Strings(got)
		if !slices.Equal(got, row.GameSecondaryTraitIDs) || !slices.Equal(row.CatalogSecondaryTraitIDs, row.GameSecondaryTraitIDs) {
			t.Errorf("%s secondary pool differs from local lots", row.InternalID)
		}
	}
	for _, row := range evidence.TraitRows {
		if row.Status != "verified" {
			t.Errorf("%s trait evidence status=%s", row.InternalID, row.Status)
			continue
		}
		trait, err := catalog.RequireTrait(row.InternalID)
		if err != nil {
			t.Error(err)
			continue
		}
		if trait.MaxLevel == nil || *trait.MaxLevel != row.GameEffectCurveMaxLevel {
			t.Errorf("%s effect-curve cap=%v, skill_status.tbl=%d", row.InternalID, trait.MaxLevel, row.GameEffectCurveMaxLevel)
		}
		if trait.CanAppearAsPrimary == nil || *trait.CanAppearAsPrimary != row.AppearsAsPrimaryInGemTable {
			t.Errorf("%s primary reachability differs from gem.tbl", row.InternalID)
		}
		if trait.CanAppearAsSecondary == nil || *trait.CanAppearAsSecondary != row.AppearsAsSecondaryInGemLotTables {
			t.Errorf("%s secondary reachability differs from joined lot tables", row.InternalID)
		}
	}
}
