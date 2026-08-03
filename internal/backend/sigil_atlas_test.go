package backend

import (
	"encoding/json"
	"testing"
)

func TestSigilAtlasUsesTheWritableCompatibilityCatalog(t *testing.T) {
	atlas, err := NewSigilGen().GetSigilAtlas()
	if err != nil {
		t.Fatal(err)
	}
	if atlas.DataVersion != "GBFR 2.0.2" || len(atlas.Sigils) == 0 || len(atlas.Traits) == 0 {
		t.Fatalf("incomplete atlas: version=%q sigils=%d traits=%d", atlas.DataVersion, len(atlas.Sigils), len(atlas.Traits))
	}
	if len(atlas.WritableSecondaryTraits) == 0 {
		t.Fatal("atlas omitted the shared writable secondary pool")
	}
	knownTraits := make(map[string]bool, len(atlas.Traits))
	for _, trait := range atlas.Traits {
		knownTraits[trait.InternalID] = true
	}
	for _, trait := range atlas.WritableSecondaryTraits {
		if !knownTraits[trait.InternalID] || trait.MaxLevel < 1 {
			t.Fatalf("atlas exposes invalid shared writable secondary %+v", trait)
		}
	}
	tableExactCount := 0
	for _, entry := range atlas.Sigils {
		if entry.TableExact {
			tableExactCount++
			if entry.Source == "" {
				t.Fatalf("%s claims table evidence without provenance", entry.InternalID)
			}
		}
		seen := map[string]bool{}
		for _, secondary := range entry.SecondaryTraits {
			if !entry.Constructible || !entry.SupportsSecondaryTrait {
				t.Fatalf("%s exposes a secondary pool without writable plus support", entry.InternalID)
			}
			if secondary.InternalID == entry.PrimaryTraitID || secondary.MaxLevel < 1 || len(secondary.AllowedLevels) == 0 {
				t.Fatalf("%s exposes invalid secondary %+v", entry.InternalID, secondary)
			}
			if seen[secondary.InternalID] {
				t.Fatalf("%s repeats secondary %s", entry.InternalID, secondary.InternalID)
			}
			seen[secondary.InternalID] = true
		}
	}
	if tableExactCount == 0 || tableExactCount >= len(atlas.Sigils) {
		t.Fatalf("table evidence classification is not selective: exact=%d total=%d", tableExactCount, len(atlas.Sigils))
	}
}

func TestSigilAtlasIndexFitsIPCAndPreservesSecondaryIdentity(t *testing.T) {
	generator := NewSigilGen()
	full, err := generator.GetSigilAtlas()
	if err != nil {
		t.Fatal(err)
	}
	compact, err := generator.GetSigilAtlasIndex()
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(compact)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) >= 256*1024 {
		t.Fatalf("compact atlas exceeds 256 KiB IPC budget: %d bytes", len(data))
	}
	var wire struct {
		Sigils []struct {
			InternalID              string   `json:"internalId"`
			SecondaryTraitIndexes   []uint16 `json:"secondaryTraitIndexes"`
			SecondaryTraitMaxLevels []uint16 `json:"secondaryTraitMaxLevels"`
		} `json:"sigils"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatalf("compact atlas is not a numeric-array JSON payload: %v", err)
	}
	for _, entry := range wire.Sigils {
		if len(entry.SecondaryTraitIndexes) != len(entry.SecondaryTraitMaxLevels) {
			t.Fatalf("%s wire secondary lengths do not match", entry.InternalID)
		}
	}
	t.Logf("compact atlas payload: %d bytes", len(data))
	if len(compact.Sigils) != len(full.Sigils) || len(compact.Traits) != len(full.Traits) {
		t.Fatalf("compact atlas lost catalog rows: compact=%d/%d full=%d/%d", len(compact.Sigils), len(compact.Traits), len(full.Sigils), len(full.Traits))
	}
	if len(compact.WritableSecondaryTraits) != len(full.WritableSecondaryTraits) {
		t.Fatalf("compact atlas lost shared writable traits: compact=%d full=%d", len(compact.WritableSecondaryTraits), len(full.WritableSecondaryTraits))
	}
	for index, entry := range compact.Sigils {
		original := full.Sigils[index]
		if entry.TableExact != original.TableExact {
			t.Fatalf("%s compact table evidence changed", entry.InternalID)
		}
		if len(entry.SecondaryTraitIndexes) != len(original.SecondaryTraits) || len(entry.SecondaryTraitMaxLevels) != len(original.SecondaryTraits) {
			t.Fatalf("%s compact secondary lengths do not match", entry.InternalID)
		}
		for secondaryIndex, traitIndex := range entry.SecondaryTraitIndexes {
			if int(traitIndex) >= len(compact.Traits) {
				t.Fatalf("%s has out-of-range trait index %d", entry.InternalID, traitIndex)
			}
			trait := compact.Traits[traitIndex]
			originalTrait := original.SecondaryTraits[secondaryIndex]
			if trait.InternalID != originalTrait.InternalID || int(entry.SecondaryTraitMaxLevels[secondaryIndex]) != originalTrait.MaxLevel {
				t.Fatalf("%s secondary %d changed identity or max level", entry.InternalID, secondaryIndex)
			}
		}
	}
}

func TestSigilAtlasReturnsIndependentSlices(t *testing.T) {
	gen := NewSigilGen()
	first, err := gen.GetSigilAtlas()
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Sigils) == 0 {
		t.Fatal("empty atlas")
	}
	original := first.Sigils[0].DisplayName
	first.Sigils[0].DisplayName = "mutated"
	second, err := gen.GetSigilAtlas()
	if err != nil {
		t.Fatal(err)
	}
	if second.Sigils[0].DisplayName != original {
		t.Fatal("atlas response leaked caller mutation")
	}
}
