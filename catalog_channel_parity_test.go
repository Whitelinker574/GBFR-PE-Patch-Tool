package main

import (
	"os"
	"reflect"
	"testing"
)

func TestSummonSaveAndRuntimeUseIdenticalOptions(t *testing.T) {
	runtimeOptions, err := (&App{}).SummonGetOptions()
	if err != nil {
		t.Fatal(err)
	}
	saveOptions, err := NewSummonSaveGen().GetOptions()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(runtimeOptions, saveOptions) {
		t.Fatal("summon save and runtime option tables diverged")
	}
	if len(runtimeOptions.Types) != 189 || len(runtimeOptions.Traits) != 82 || len(runtimeOptions.SubParams) != 22 || len(runtimeOptions.Rules) != 189 {
		t.Fatalf("unexpected summon catalog sizes: types=%d traits=%d sub=%d rules=%d", len(runtimeOptions.Types), len(runtimeOptions.Traits), len(runtimeOptions.SubParams), len(runtimeOptions.Rules))
	}
}

func TestWrightstoneSaveAndRuntimeTraitTablesMatch(t *testing.T) {
	saveTraits, err := NewWrightstoneGen().GetTraitList()
	if err != nil {
		t.Fatal(err)
	}
	runtimeOptions, err := (&App{}).WrightstoneMemoryGetOptions()
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		Name   string
		Max    int
		Levels []int
	}
	saveByHash := make(map[uint32]row, len(saveTraits))
	for _, item := range saveTraits {
		hash, err := ParseHashHex(item.Hash)
		if err != nil {
			t.Fatal(err)
		}
		saveByHash[hash] = row{item.DisplayName, item.MaxLevel, item.AllowedLevels}
	}
	runtimeByHash := make(map[uint32]row, len(runtimeOptions.Traits))
	for _, item := range runtimeOptions.Traits {
		runtimeByHash[item.Hash] = row{item.DisplayName, derefInt(item.MaxLevel), item.AllowedLevels}
	}
	if !reflect.DeepEqual(saveByHash, runtimeByHash) {
		t.Fatalf("wrightstone save/runtime trait tables diverged: save=%d runtime=%d", len(saveByHash), len(runtimeByHash))
	}
}

func TestSigilSaveRuntimeAndConstructorsUseExactUnifiedCatalog(t *testing.T) {
	saveSigils, err := NewSigilGen().GetSigilList()
	if err != nil {
		t.Fatal(err)
	}
	saveTraits, err := NewSigilGen().GetTraitList()
	if err != nil {
		t.Fatal(err)
	}
	runtimeOptions, err := (&App{}).SigilMemoryGetOptions()
	if err != nil {
		t.Fatal(err)
	}
	runtimeSigils := make(map[uint32]SigilMemoryOption)
	runtimeTraits := make(map[uint32]SigilMemoryOption)
	for _, item := range runtimeOptions.Sigils {
		if item.Source != "catalog" {
			t.Fatalf("runtime sigil 0x%08X is outside unified catalog", item.Hash)
		}
		if _, duplicate := runtimeSigils[item.Hash]; duplicate {
			t.Fatalf("duplicate runtime sigil hash 0x%08X", item.Hash)
		}
		runtimeSigils[item.Hash] = item
	}
	for _, item := range runtimeOptions.Traits {
		if item.Source != "catalog" {
			t.Fatalf("runtime trait 0x%08X is outside unified catalog", item.Hash)
		}
		if _, duplicate := runtimeTraits[item.Hash]; duplicate {
			t.Fatalf("duplicate runtime trait hash 0x%08X", item.Hash)
		}
		runtimeTraits[item.Hash] = item
	}
	if len(runtimeSigils) != len(saveSigils) || len(runtimeTraits) != len(saveTraits) {
		t.Fatalf("unified factor table count differs: save=%d/%d runtime=%d/%d", len(saveSigils), len(saveTraits), len(runtimeSigils), len(runtimeTraits))
	}
	if len(saveSigils) != 219 {
		t.Fatalf("unified factor table has %d items; want 184 gem.tbl rows plus 35 unique CT 0.8.5 supplemental rows", len(saveSigils))
	}
	gen := NewSigilGen()
	for _, item := range saveSigils {
		hash, err := ParseHashHex(item.Hash)
		if err != nil {
			t.Fatal(err)
		}
		runtimeItem, ok := runtimeSigils[hash]
		if !ok {
			t.Fatalf("save sigil %s missing from runtime catalog", item.Hash)
		}
		if item.DisplayName != runtimeItem.DisplayName || item.FirstTraitMaxLevel != derefInt(runtimeItem.FirstTraitMaxLevel) ||
			!reflect.DeepEqual(item.AllowedSigilLevels, runtimeItem.AllowedLevels) || !reflect.DeepEqual(item.AllowedFirstTraitLevels, runtimeItem.AllowedPrimaryTraitLevels) {
			t.Fatalf("save/runtime sigil metadata differs for %s", item.Hash)
		}
		compatible, err := gen.GetCompatibleSecondaryTraits(item.InternalID)
		if err != nil {
			t.Fatal(err)
		}
		compatibleHashes := make([]uint32, 0, len(compatible))
		for _, trait := range compatible {
			traitHash, err := ParseHashHex(trait.Hash)
			if err != nil {
				t.Fatal(err)
			}
			compatibleHashes = append(compatibleHashes, traitHash)
		}
		if !reflect.DeepEqual(compatibleHashes, runtimeItem.AllowedSecondaryTraitHashes) {
			t.Fatalf("save/runtime secondary pool differs for %s: save=%d runtime=%d", item.Hash, len(compatibleHashes), len(runtimeItem.AllowedSecondaryTraitHashes))
		}
	}
	for _, item := range saveTraits {
		hash, err := ParseHashHex(item.Hash)
		if err != nil {
			t.Fatal(err)
		}
		runtimeItem, ok := runtimeTraits[hash]
		if !ok {
			t.Fatalf("save trait %s missing from runtime catalog", item.Hash)
		}
		if item.DisplayName != runtimeItem.DisplayName || item.MaxLevel != derefInt(runtimeItem.MaxLevel) || !reflect.DeepEqual(item.AllowedLevels, runtimeItem.AllowedLevels) {
			t.Fatalf("save/runtime trait metadata differs for %s", item.Hash)
		}
	}
}

func TestRealSaveProvesSummonRankIsIndependentFromRarityTierWhenRequested(t *testing.T) {
	path := os.Getenv("GBFR_TEST_STATS_SAVE")
	if path == "" {
		t.Skip("GBFR_TEST_STATS_SAVE not set")
	}
	save, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	inv, err := save.InspectSummonInventory()
	if err != nil {
		t.Fatal(err)
	}
	rules, err := loadSummonNaturalRules()
	if err != nil {
		t.Fatal(err)
	}
	byHash, err := summonNaturalRuleByHash(rules)
	if err != nil {
		t.Fatal(err)
	}
	matrix := make(map[[2]uint32]int)
	for _, record := range inv.Records {
		rule, ok := byHash[record.State.TypeHash]
		if ok {
			matrix[[2]uint32{uint32(rule.TierIndex), record.State.Rank}]++
		}
	}
	if len(matrix) < 4 {
		t.Fatalf("real save did not prove rank/tier independence: %v", matrix)
	}
	t.Logf("tier-index/live-rank matrix: %v", matrix)
}
