package backend

import "testing"

func customSecondaryUpdate(t *testing.T, sigilID, secondaryTraitID string) (*Catalog, SigilMemoryUpdate) {
	t.Helper()
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	sigil, err := catalog.RequireSigil(sigilID)
	if err != nil {
		t.Fatal(err)
	}
	primary, err := catalog.RequireTrait(sigil.PrimaryTraitID)
	if err != nil {
		t.Fatal(err)
	}
	secondary, err := catalog.RequireTrait(secondaryTraitID)
	if err != nil {
		t.Fatal(err)
	}
	sigilHash, err := ParseHashHex(sigil.Hash)
	if err != nil {
		t.Fatal(err)
	}
	primaryHash, err := ParseHashHex(primary.Hash)
	if err != nil {
		t.Fatal(err)
	}
	secondaryHash, err := ParseHashHex(secondary.Hash)
	if err != nil {
		t.Fatal(err)
	}
	return catalog, SigilMemoryUpdate{
		SigilHash:           sigilHash,
		SigilLevel:          15,
		PrimaryTraitHash:    primaryHash,
		PrimaryTraitLevel:   15,
		SecondaryTraitHash:  secondaryHash,
		SecondaryTraitLevel: 15,
	}
}

func TestCompatibleSecondaryChoicesIncludeKnownCustomCombinations(t *testing.T) {
	tests := []struct {
		name, sigilID, secondaryTraitID string
	}{
		{name: "steel nerves plus garrison", sigilID: "GEEN_096_24", secondaryTraitID: "SKILL_036_00"},
		{name: "double damage cap", sigilID: "GEEN_020_24", secondaryTraitID: "SKILL_020_00"},
	}
	gen := NewSigilGen()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			traits, err := gen.GetCompatibleSecondaryTraits(test.sigilID)
			if err != nil {
				t.Fatal(err)
			}
			for _, trait := range traits {
				if trait.InternalID == test.secondaryTraitID {
					return
				}
			}
			t.Fatalf("%s is missing from writable secondary choices for %s", test.secondaryTraitID, test.sigilID)
		})
	}
}

func TestKnownCustomSecondaryCombinationsRemainWritable(t *testing.T) {
	tests := []struct {
		name, sigilID, secondaryTraitID string
	}{
		{name: "steel nerves plus garrison", sigilID: "GEEN_096_24", secondaryTraitID: "SKILL_036_00"},
		{name: "double damage cap", sigilID: "GEEN_020_24", secondaryTraitID: "SKILL_020_00"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			catalog, update := customSecondaryUpdate(t, test.sigilID, test.secondaryTraitID)
			if err := validateSigilMemoryWriteRequest(catalog, update); err != nil {
				t.Fatalf("runtime editor rejected writable combination: %v", err)
			}

			sigil, _ := catalog.RequireSigil(test.sigilID)
			gen := &SigilGen{catalog: catalog}
			report, err := gen.CheckLegality(QueueItem{
				SigilID: test.sigilID, Level: 15,
				PrimaryTraitID: sigil.PrimaryTraitID, PrimaryLevel: 15,
				SecondaryTraitID: test.secondaryTraitID, SecondaryLevel: 15,
				Quantity: 1,
			})
			if err != nil {
				t.Fatal(err)
			}
			if !report.Writable {
				t.Fatalf("offline generator rejected writable combination: %+v", report)
			}

			prepared, err := prepareLoadoutSigil(catalog, LoadoutConstructedSigil{
				Index: 0,
				Item: QueueItem{
					SigilID: test.sigilID, Level: 15,
					PrimaryTraitID: sigil.PrimaryTraitID, PrimaryLevel: 15,
					SecondaryTraitID: test.secondaryTraitID, SecondaryLevel: 15,
					Quantity: 1,
				},
			})
			if err != nil {
				t.Fatalf("loadout editor rejected writable combination: %v", err)
			}
			if prepared == nil {
				t.Fatal("loadout editor returned no prepared factor")
			}
		})
	}
}

func TestSigilAndWrightstoneLevelsAboveEffectCurvesRemainWritable(t *testing.T) {
	catalog, update := customSecondaryUpdate(t, "GEEN_096_24", "SKILL_036_00")
	primary := catalog.LookupTraitByHash(update.PrimaryTraitHash)
	primaryLevels, err := requireTraitLevels(primary, "test primary")
	if err != nil {
		t.Fatal(err)
	}
	update.PrimaryTraitLevel = uint32(effectCurveMax(primaryLevels, 15) + 1)
	if err := validateSigilMemoryWriteRequest(catalog, update); err != nil {
		t.Fatalf("runtime sigil editor rejected a field-encodable level above the effect curve: %v", err)
	}
	report, err := (&SigilGen{catalog: catalog}).CheckLegality(QueueItem{
		SigilID: "GEEN_096_24", Level: 16,
		PrimaryTraitID: "SKILL_096_00", PrimaryLevel: int(update.PrimaryTraitLevel),
		SecondaryTraitID: "SKILL_036_00", SecondaryLevel: 46,
		Quantity: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Writable || report.Status != LegalityForced {
		t.Fatalf("offline sigil editor should warn but allow curve overflow: %+v", report)
	}

	wrightstones, wrightstoneUpdate := validWrightstoneMemoryUpdate(t)
	wrightstoneTrait := wrightstones.LookupTraitByHash(wrightstoneUpdate.FirstHash)
	wrightstoneLevels, err := requireWrightstoneTraitLevels(wrightstoneTrait)
	if err != nil {
		t.Fatal(err)
	}
	wrightstoneUpdate.FirstLevel = uint32(effectCurveMax(wrightstoneLevels, 20) + 1)
	if err := validateWrightstoneMemoryWriteRequest(wrightstones, wrightstoneUpdate); err != nil {
		t.Fatalf("runtime wrightstone editor rejected a field-encodable level above the effect curve: %v", err)
	}
}
