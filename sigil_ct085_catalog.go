package main

import (
	"fmt"
	"strings"
)

const ct085SupplementSource = "GBFR 0.8.5 CT memory catalog; V9 factor and trait hash tables; upstream c7f45b7"

var memoryAwakeningPrimaryTraits = map[string]string{
	"转世之觉醒": "转世的恩宠",
	"刃姬之觉醒": "刃姬的小夜曲",
	"狼王之觉醒": "狼王的激昂",
	"黑龙之觉醒": "黑龙的咒印",
	"雷狼之觉醒": "雷狼的弹匣",
	"群青之觉醒": "群青的剑光",
}

func levelsUpTo(maxLevel int) []int {
	levels := make([]int, maxLevel)
	for index := range levels {
		levels[index] = index + 1
	}
	return levels
}

func singleTraitMemorySigil(name string) bool {
	return name == "相扑斗力" || name == "漆黑之谊" || name == "可怕的漆黑钳蟹因子"
}

func ct085TraitMaxLevel(name string) int {
	switch name {
	case "相扑斗力":
		return 5
	case "可怕的漆黑钳蟹因子":
		return 20
	default:
		return 15
	}
}

func ct085TraitEnglishName(name string) string {
	if english := sigilMemoryEnglishNames[name]; english != "" {
		return english
	}
	return name
}

func ct085SupplementalTraitDefs() []TraitDef {
	category := "ct085_supplement"
	canAppear := true
	result := make([]TraitDef, 0, len(sigilMemoryTraits))
	seen := make(map[uint32]bool, len(sigilMemoryTraits))
	for _, entry := range sigilMemoryTraits {
		if seen[entry.Hash] {
			continue
		}
		seen[entry.Hash] = true
		maxLevel := ct085TraitMaxLevel(entry.Name)
		result = append(result, TraitDef{
			InternalID:           fmt.Sprintf("MEMORY_TRAIT_%08X", entry.Hash),
			Hash:                 fmt.Sprintf("0x%08X", entry.Hash),
			DisplayName:          ct085TraitEnglishName(entry.Name),
			Category:             &category,
			MaxLevel:             &maxLevel,
			AllowedLevels:        levelsUpTo(maxLevel),
			ObservedLevels:       levelsUpTo(maxLevel),
			CanAppearAsPrimary:   &canAppear,
			CanAppearAsSecondary: &canAppear,
		})
	}
	return result
}

func appendCT085Catalog(c *Catalog) {
	traitByHash := make(map[uint32]*TraitDef, len(c.Traits)+len(sigilMemoryTraits))
	for index := range c.Traits {
		if hash, err := ParseHashHex(c.Traits[index].Hash); err == nil {
			traitByHash[hash] = &c.Traits[index]
		}
	}
	for _, trait := range ct085SupplementalTraitDefs() {
		hash, _ := ParseHashHex(trait.Hash)
		if traitByHash[hash] != nil {
			continue
		}
		c.Traits = append(c.Traits, trait)
		traitByHash[hash] = &c.Traits[len(c.Traits)-1]
	}

	traitHashByName := make(map[string]uint32, len(sigilMemoryTraits))
	for _, entry := range sigilMemoryTraits {
		traitHashByName[entry.Name] = entry.Hash
	}
	secondaryIDs := make([]string, 0, len(traitHashByName))
	seenSecondary := make(map[string]bool, len(traitHashByName))
	for _, entry := range sigilMemoryTraits {
		trait := traitByHash[entry.Hash]
		if trait == nil || seenSecondary[trait.InternalID] {
			continue
		}
		seenSecondary[trait.InternalID] = true
		secondaryIDs = append(secondaryIDs, trait.InternalID)
	}

	sigilHashes := make(map[uint32]bool, len(c.Sigils)+len(sigilMemorySigils))
	for index := range c.Sigils {
		if hash, err := ParseHashHex(c.Sigils[index].Hash); err == nil {
			sigilHashes[hash] = true
		}
	}
	category := "ct085_supplement"
	for _, entry := range sigilMemorySigils {
		if sigilHashes[entry.Hash] {
			continue
		}
		primaryName := entry.Name
		if mapped := memoryAwakeningPrimaryTraits[primaryName]; mapped != "" {
			primaryName = mapped
		}
		traitHash := traitHashByName[primaryName]
		primary := traitByHash[traitHash]
		if primary == nil {
			continue
		}
		maxLevel := ct085TraitMaxLevel(primaryName)
		sigilLevel := 15
		if entry.Name == "可怕的漆黑钳蟹因子" || entry.Name == "相扑斗力" {
			sigilLevel = 0
		}
		supportsSecondary := !singleTraitMemorySigil(entry.Name)
		name := ctHashToNameEN[entry.Hash]
		if name == "" {
			name = ct085TraitEnglishName(entry.Name)
		}
		allowedSecondary := []string(nil)
		if supportsSecondary {
			allowedSecondary = append([]string(nil), secondaryIDs...)
		}
		c.Sigils = append(c.Sigils, SigilDef{
			InternalID:               fmt.Sprintf("MEMORY_SIGIL_%08X", entry.Hash),
			Hash:                     fmt.Sprintf("0x%08X", entry.Hash),
			DisplayName:              name,
			Notes:                    "Hash and identity are present in the 0.8.5 CT memory catalog and the local V9 tables; this record is supplemental because the reduced gem.tbl constructor catalog omits it.",
			Source:                   ct085SupplementSource,
			Confidence:               "high",
			Category:                 &category,
			SupportsSecondaryTrait:   &supportsSecondary,
			AllowedSigilLevels:       []int{sigilLevel},
			DefaultSigilLevel:        &sigilLevel,
			MaxSigilLevel:            &sigilLevel,
			PrimaryTraitID:           primary.InternalID,
			PrimaryTraitName:         &primary.DisplayName,
			FirstTraitMaxLevel:       &maxLevel,
			AllowedFirstTraitLevels:  levelsUpTo(maxLevel),
			AllowedSecondaryTraitIDs: allowedSecondary,
		})
		sigilHashes[entry.Hash] = true
	}
}

func supplementalSigilDisplayName(sigil *SigilDef) string {
	if sigil == nil || !strings.HasPrefix(sigil.InternalID, "MEMORY_SIGIL_") {
		return ""
	}
	hash, err := ParseHashHex(sigil.Hash)
	if err != nil {
		return ""
	}
	name := ctName(hash)
	if name == "" {
		return ""
	}
	if useChinese() && strings.HasPrefix(name, "天星之") && strings.HasSuffix(name, "V+") {
		name = strings.TrimSuffix(name, "V+") + " V+"
	}
	return name
}

func ct085SupplementalWrightstoneTraits() []WrightstoneTraitDef {
	traits := ct085SupplementalTraitDefs()
	result := make([]WrightstoneTraitDef, 0, len(traits))
	for _, trait := range traits {
		result = append(result, WrightstoneTraitDef{
			InternalID:     trait.InternalID,
			Hash:           trait.Hash,
			DisplayName:    trait.DisplayName,
			Category:       trait.Category,
			MaxLevel:       trait.MaxLevel,
			AllowedLevels:  append([]int(nil), trait.AllowedLevels...),
			ObservedLevels: append([]int(nil), trait.ObservedLevels...),
		})
	}
	return result
}

func appendCT085WrightstoneTraits(c *WrightstoneCatalog) {
	seen := make(map[uint32]bool, len(c.Traits)+len(sigilMemoryTraits))
	for _, trait := range c.Traits {
		if hash, err := ParseHashHex(trait.Hash); err == nil {
			seen[hash] = true
		}
	}
	for _, trait := range ct085SupplementalWrightstoneTraits() {
		hash, _ := ParseHashHex(trait.Hash)
		if seen[hash] {
			continue
		}
		c.Traits = append(c.Traits, trait)
		seen[hash] = true
	}
}
