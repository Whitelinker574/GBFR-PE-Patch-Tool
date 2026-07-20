package main

import (
	"testing"
)

func TestWrightstoneTraitCatalogIsIsolatedByLanguage(t *testing.T) {
	old := getCurrentLanguage()
	t.Cleanup(func() { setCurrentLanguage(old) })
	setCurrentLanguage("zh")
	zh, err := NewWrightstoneGen().GetTraitList()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := LoadWrightstoneCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, trait := range zh {
		raw := catalog.traitByID[trait.InternalID]
		if raw == nil || trait.DisplayName != cnWrightstoneTrait(raw.DisplayName) {
			t.Fatalf("Chinese blessing trait does not use the shared catalog: %s=%q", trait.InternalID, trait.DisplayName)
		}
		if _, direct := wrightstoneTraitCN[raw.DisplayName]; !direct {
			if _, shared := traitCN[raw.DisplayName]; !shared {
				t.Fatalf("Chinese blessing trait fell back to English: %s=%q", trait.InternalID, trait.DisplayName)
			}
		}
	}

	setCurrentLanguage("en")
	en, err := NewWrightstoneGen().GetTraitList()
	if err != nil {
		t.Fatal(err)
	}
	for _, trait := range en {
		raw := catalog.traitByID[trait.InternalID]
		if raw == nil || trait.DisplayName != raw.DisplayName {
			t.Fatalf("English blessing trait did not retain its English catalog name: %s=%q", trait.InternalID, trait.DisplayName)
		}
	}

	wantZH := map[string]string{
		"SKILL_004_00": "昏厥",
		"SKILL_030_00": "Overdrive特效",
		"SKILL_156_00": "万能药",
	}
	for _, trait := range zh {
		if want, ok := wantZH[trait.InternalID]; ok && trait.DisplayName != want {
			t.Fatalf("%s Chinese name=%q, want %q", trait.InternalID, trait.DisplayName, want)
		}
	}
}
