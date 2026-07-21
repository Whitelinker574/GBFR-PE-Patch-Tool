package main

import (
	"reflect"
	"testing"
)

func TestSigilMemoryOptionsUseSharedSecondaryRules(t *testing.T) {
	options, err := (&App{}).SigilMemoryGetOptions()
	if err != nil {
		t.Fatal(err)
	}

	const pursuitVPlus = uint32(0x035A4DDD)
	const preciseWrath = uint32(0x7EDD69D0)
	for _, sigil := range options.Sigils {
		if sigil.Hash != pursuitVPlus {
			continue
		}
		for _, hash := range sigil.AllowedSecondaryTraitHashes {
			if hash == preciseWrath {
				return
			}
		}
		t.Fatalf("追击 V+ 合规列表缺少怒发冲冠 (0x%08X)", preciseWrath)
	}
	t.Fatalf("因子选项中未找到追击 V+ (0x%08X)", pursuitVPlus)
}

func TestSigilMemoryOptionsExposeFixedPrimaryTraitTruth(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	options, err := (&App{}).SigilMemoryGetOptions()
	if err != nil {
		t.Fatal(err)
	}
	for _, option := range options.Sigils {
		sigil := catalog.LookupSigilByHash(option.Hash)
		if sigil == nil {
			t.Fatalf("catalog option 0x%08X has no catalog sigil", option.Hash)
		}
		trait, err := catalog.RequireTrait(sigil.PrimaryTraitID)
		if err != nil {
			t.Fatal(err)
		}
		wantHash, err := ParseHashHex(trait.Hash)
		if err != nil {
			t.Fatal(err)
		}
		wantLevels, err := catalog.RequirePrimaryTraitLevels(sigil)
		if err != nil {
			t.Fatal(err)
		}
		wantLevels = naturalSigilLevels(wantLevels)
		if option.PrimaryTraitHash != wantHash || !reflect.DeepEqual(option.AllowedPrimaryTraitLevels, wantLevels) {
			t.Fatalf("sigil %s primary truth = hash 0x%08X levels %v, want 0x%08X %v", sigil.InternalID, option.PrimaryTraitHash, option.AllowedPrimaryTraitLevels, wantHash, wantLevels)
		}
	}
}

func TestSigilMemoryOptionsContainOnlyUnifiedCatalogRows(t *testing.T) {
	options, err := (&App{}).SigilMemoryGetOptions()
	if err != nil {
		t.Fatal(err)
	}
	for _, option := range append(options.Sigils, options.Traits...) {
		if option.Source != "catalog" {
			t.Fatalf("non-unified runtime option leaked: %+v", option)
		}
	}
}
