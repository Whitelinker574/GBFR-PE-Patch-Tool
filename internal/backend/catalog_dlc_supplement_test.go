package backend

import (
	"fmt"
	"strings"
	"testing"
	"unicode"
)

func TestDLCSupplementCelestialSigilsReachSaveAndLoadoutConstructors(t *testing.T) {
	previousLanguage := getCurrentLanguage()
	setCurrentLanguage("zh")
	defer setCurrentLanguage(previousLanguage)

	items, err := NewSigilGen().GetSigilList()
	if err != nil {
		t.Fatal(err)
	}
	byHash := make(map[string]SigilInfo, len(items))
	for _, item := range items {
		byHash[strings.ToUpper(strings.TrimPrefix(item.Hash, "0x"))] = item
	}
	want := map[string]string{
		"9300FADB": "天星之止息 V+",
		"D29CD8E0": "天星之界 V+",
		"8B8085C0": "天星之炼 V+",
		"E14E1598": "天星之雪 V+",
		"74061B0C": "天星之焰 V+",
		"20492635": "天星之煌 V+",
	}
	for hash, name := range want {
		item, ok := byHash[hash]
		if !ok {
			t.Errorf("DLC 2.0.2 runtime catalog DLC factor %s (%s) is missing from the unified constructor catalog", name, hash)
			continue
		}
		if item.DisplayName != name {
			t.Errorf("DLC factor %s display name = %q; want %q", hash, item.DisplayName, name)
		}
		if !item.Constructible || !item.SupportsSecondaryTrait {
			t.Errorf("DLC factor %s must remain selectable and writable: %+v", hash, item)
		}
		traits, err := NewSigilGen().GetCompatibleSecondaryTraits(item.InternalID)
		if err != nil {
			t.Errorf("DLC factor %s compatible traits: %v", hash, err)
		} else if len(traits) == 0 {
			t.Errorf("DLC factor %s exposed no selectable secondary traits", hash)
		}
	}
	for _, entry := range sigilMemorySigils {
		primaryName := entry.Name
		if mapped := memoryAwakeningPrimaryTraits[primaryName]; mapped != "" {
			primaryName = mapped
		}
		primaryFound := false
		for _, trait := range sigilMemoryTraits {
			if trait.Name == primaryName {
				primaryFound = true
				break
			}
		}
		if !primaryFound {
			continue
		}
		hash := strings.ToUpper(fmt.Sprintf("%08X", entry.Hash))
		if _, ok := byHash[hash]; !ok {
			t.Errorf("DLC supplemental factor %s (%s) is missing", entry.Name, hash)
		}
		if runtimeNameCN[entry.Hash] == "" && runtimeNameEN[entry.Hash] == "" {
			t.Errorf("DLC supplemental factor %s (%s) is absent from the runtime name tables", entry.Name, hash)
		}
	}
}

func TestDLCSupplementSupplementalBlessingTraitsReachSaveAndMemoryEditors(t *testing.T) {
	catalog, err := LoadWrightstoneCatalog()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"0x73220725": "Celestial Ventus",
		"0x0DE887A0": "Celestial Nyx",
		"0xA7726190": "Celestial Lumen",
		"0x9232DC17": "Celestial Terra",
		"0x36E3848D": "Celestial Incendo",
		"0xA898E283": "Celestial Aqua",
		"0x1DE14C65": "Gladiator's Frenzy",
		"0x7B5B081D": "Bladequeen's Serenade",
		"0xD176D262": "Ultramarine's Flash",
		"0x7D75D904": "Thunderwolf's Recharge",
		"0x47384248": "Enchantress's Blessing",
		"0x06719232": "The Black's Mark",
	}
	for hash, name := range want {
		value, err := ParseHashHex(hash)
		if err != nil {
			t.Fatal(err)
		}
		trait := catalog.LookupTraitByHash(value)
		if trait == nil {
			t.Errorf("DLC 2.0.2 runtime catalog blessing trait %s (%s) is missing", name, hash)
			continue
		}
		if trait.DisplayName != name || trait.MaxLevel == nil {
			t.Errorf("blessing trait %s = %+v; want name %q with a writable level range", hash, trait, name)
		}
	}
	for _, entry := range sigilMemoryTraits {
		trait := catalog.LookupTraitByHash(entry.Hash)
		if trait == nil {
			t.Errorf("DLC supplemental blessing trait %s (0x%08X) is missing", entry.Name, entry.Hash)
		}
		if runtimeNameCN[entry.Hash] == "" && runtimeNameEN[entry.Hash] == "" {
			t.Errorf("DLC supplemental blessing trait %s (0x%08X) is absent from the runtime name tables", entry.Name, entry.Hash)
		}
	}
}

func TestUnifiedFactorCatalogHasNoDuplicateHashesAfterDLCSupplementSupplement(t *testing.T) {
	items, err := NewSigilGen().GetSigilList()
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]string, len(items))
	for _, item := range items {
		hash := strings.ToUpper(strings.TrimPrefix(item.Hash, "0x"))
		if previous, duplicate := seen[hash]; duplicate {
			t.Fatalf("factor hash %s is exposed twice: %s and %s", hash, previous, item.InternalID)
		}
		seen[hash] = item.InternalID
	}
}

func TestDLCSupplementSupplementalFactorNamesStayLanguageIsolated(t *testing.T) {
	previousLanguage := getCurrentLanguage()
	defer setCurrentLanguage(previousLanguage)
	setCurrentLanguage("en")
	items, err := NewSigilGen().GetSigilList()
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, item := range items {
		if !strings.HasPrefix(item.InternalID, "MEMORY_SIGIL_") {
			continue
		}
		found++
		if strings.ContainsFunc(item.DisplayName, func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
			t.Errorf("English supplemental factor contains Chinese text: %s=%q", item.InternalID, item.DisplayName)
		}
	}
	if found != 32 {
		t.Fatalf("English supplemental factor count=%d; want 32 unique DLC 2.0.2 runtime catalog rows", found)
	}
}
