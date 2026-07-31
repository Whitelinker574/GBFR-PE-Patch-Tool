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

func TestCommunityRouteExactRuntimeShellsReachUnifiedCatalog(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}

	fatebreaker, err := catalog.RequireSigil("MEMORY_SIGIL_5BF84FD1")
	if err != nil {
		t.Fatal(err)
	}
	fatebreakerPrimary, err := catalog.RequireTrait(fatebreaker.PrimaryTraitID)
	if err != nil {
		t.Fatal(err)
	}
	if fatebreakerPrimary.Hash != "0xD029FE08" {
		t.Fatalf("Fatebreaker primary hash = %s; want 0xD029FE08", fatebreakerPrimary.Hash)
	}

	ventus, err := catalog.RequireSigil("MEMORY_SIGIL_9300FADB")
	if err != nil {
		t.Fatal(err)
	}
	ventusPrimary, err := catalog.RequireTrait(ventus.PrimaryTraitID)
	if err != nil {
		t.Fatal(err)
	}
	if ventusPrimary.Hash != "0x73220725" {
		t.Fatalf("Celestial Ventus primary hash = %s; want 0x73220725", ventusPrimary.Hash)
	}
	hasLumenSecondary := false
	for _, traitID := range ventus.AllowedSecondaryTraitIDs {
		if traitID == "MEMORY_TRAIT_A7726190" {
			hasLumenSecondary = true
			break
		}
	}
	if !hasLumenSecondary {
		t.Fatal("Celestial Ventus V+ does not expose the recorded Celestial Lumen secondary")
	}
}

func TestPreciseWrathVPlusRuntimeHashUsesTheUnifiedCatalog(t *testing.T) {
	previousLanguage := getCurrentLanguage()
	setCurrentLanguage("zh")
	defer setCurrentLanguage(previousLanguage)

	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	const sigilHash = uint32(0xCE6C62CF)
	sigil := catalog.LookupSigilByHash(sigilHash)
	if sigil == nil {
		t.Fatal("2.0.2 runtime factor 0xCE6C62CF is missing from the unified catalog")
	}
	if got := displaySigilName(sigil); got != "怒发冲冠 V+" {
		t.Fatalf("0xCE6C62CF display name=%q; want official 2.0.2 name 怒发冲冠 V+", got)
	}
	primary, err := catalog.RequireTrait(sigil.PrimaryTraitID)
	if err != nil {
		t.Fatal(err)
	}
	primaryHash, err := ParseHashHex(primary.Hash)
	if err != nil {
		t.Fatal(err)
	}
	if primaryHash != 0x7EDD69D0 {
		t.Fatalf("0xCE6C62CF primary hash=0x%08X; want Precise Wrath 0x7EDD69D0", primaryHash)
	}
	if !supportsGeneratedPlusSigil(sigil) {
		t.Fatal("the official V+ shell must expose its secondary-trait slot")
	}
	secondary := catalog.LookupTraitByHash(0xDC584F60) // Damage Cap
	if secondary == nil {
		t.Fatal("test catalog is missing Damage Cap")
	}
	if err := validateSigilMemoryUpdate(catalog, SigilMemoryUpdate{
		SigilHash:           sigilHash,
		SigilLevel:          15,
		PrimaryTraitHash:    primaryHash,
		PrimaryTraitLevel:   15,
		SecondaryTraitHash:  0xDC584F60,
		SecondaryTraitLevel: 15,
	}); err != nil {
		t.Fatalf("official Precise Wrath V+ combination was rejected: %v", err)
	}
	if err := validateSigilMemoryUpdate(catalog, SigilMemoryUpdate{
		SigilHash:           sigilHash,
		SigilLevel:          15,
		PrimaryTraitHash:    primaryHash,
		PrimaryTraitLevel:   15,
		SecondaryTraitHash:  0xF26BAEA5, // Divergence is present in the 2.0.3 synthesis trait table.
		SecondaryTraitLevel: 15,
	}); err != nil {
		t.Fatalf("the synthesis-table Precise Wrath V+ + Divergence combination was rejected: %v", err)
	}

	divergence := catalog.LookupTraitByHash(0xF26BAEA5)
	if divergence == nil {
		t.Fatal("test catalog is missing the captured Divergence trait")
	}
	invalidItem := QueueItem{
		SigilID: sigil.InternalID, SigilName: displaySigilName(sigil), Level: 15,
		PrimaryTraitID: primary.InternalID, PrimaryLevel: 15,
		SecondaryTraitID: divergence.InternalID, SecondaryLevel: 15,
		Quantity: 1,
	}
	report, err := (&SigilGen{catalog: catalog}).CheckLegality(invalidItem)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Writable || report.Status != LegalityForced {
		t.Fatalf("offline save factor editor should warn but accept Precise Wrath V+ + Divergence: %+v", report)
	}
	if _, err := prepareLoadoutSigil(catalog, LoadoutConstructedSigil{Index: 0, Item: invalidItem}); err != nil {
		t.Fatalf("loadout factor constructor rejected Precise Wrath V+ + Divergence: %v", err)
	}
	if _, err := prepareLoadoutSigil(catalog, LoadoutConstructedSigil{
		Index:                   0,
		ExactSigilHash:          "CE6C62CF",
		ExactPrimaryTraitHash:   "7EDD69D0",
		ExactSecondaryTraitHash: "F26BAEA5",
		Item:                    invalidItem,
	}); err != nil {
		t.Fatalf("exact loadout transport rejected a catalogued synthesis trait: %v", err)
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
