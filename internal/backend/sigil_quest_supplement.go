package backend

const rolandQuestPotentGreensSource = "GBFR 2.0.3 retail quest reward capture; local GBFR ID map GEEN_151_94"

// appendQuestRewardSupplementCatalog keeps quest-only fixed combinations out
// of the ordinary random secondary pool while still making their real item
// identity available to every editor that consumes the shared catalog.
func appendQuestRewardSupplementCatalog(c *Catalog) {
	const (
		internalID       = "GEEN_151_94"
		sigilHash        = "0x97CF485D"
		primaryTraitID   = "SKILL_023_00"
		secondaryTraitID = "SKILL_151_00"
	)
	for index := range c.Sigils {
		if c.Sigils[index].InternalID == internalID || c.Sigils[index].Hash == sigilHash {
			return
		}
	}
	var primary *TraitDef
	var secondary *TraitDef
	for index := range c.Traits {
		switch c.Traits[index].InternalID {
		case primaryTraitID:
			primary = &c.Traits[index]
		case secondaryTraitID:
			secondary = &c.Traits[index]
		}
	}
	if primary == nil || secondary == nil {
		return
	}

	category := "quest_reward"
	isPlus := true
	supportsSecondary := true
	level := 15
	primaryName := primary.DisplayName
	defaultSecondary := secondary.InternalID
	c.Sigils = append(c.Sigils, SigilDef{
		InternalID:                internalID,
		Hash:                      sigilHash,
		DisplayName:               "Potent Greens+",
		Notes:                     "Roland's quest reward is a fixed Potent Greens Lv15 + Supplementary DMG Lv15 record; it is not an ordinary random-secondary Potent Greens shell.",
		Source:                    rolandQuestPotentGreensSource,
		Confidence:                "high",
		Category:                  &category,
		IsPlusSigil:               &isPlus,
		SupportsSecondaryTrait:    &supportsSecondary,
		AllowedSigilLevels:        []int{level},
		DefaultSigilLevel:         &level,
		MaxSigilLevel:             &level,
		PrimaryTraitID:            primary.InternalID,
		PrimaryTraitName:          &primaryName,
		FirstTraitMaxLevel:        &level,
		AllowedFirstTraitLevels:   []int{level},
		AllowedSecondaryTraitIDs:  []string{secondary.InternalID},
		DefaultSecondaryTraitID:   &defaultSecondary,
		DefaultSecondaryTraitName: &secondary.DisplayName,
	})
}
