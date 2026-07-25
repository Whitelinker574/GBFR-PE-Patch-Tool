package backend

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var runtimeOwnerCharacterHash = map[string]uint32{
	"PL0000": 0x2A26B1B2, "PL0100": 0xA4ACBA76, "PL0200": 0x18E2F9F9, "PL0300": 0x079DF0CC,
	"PL0400": 0x4D0A60C3, "PL0500": 0xDD7A151E, "PL0600": 0xC8616284, "PL0700": 0xC3FFD418,
	"PL0800": 0x22E437E5, "PL0900": 0x2EBE91D5, "PL1000": 0xBDEF7181, "PL1100": 0x627BCB0D,
	"PL1200": 0xFD3BE362, "PL1300": 0xFC6CDF7B, "PL1400": 0xE7053919, "PL1500": 0x978E4B18,
	"PL1600": 0x0D21B430, "PL1700": 0xF0EB77EF, "PL1800": 0xAA66178A, "PL1900": 0xA3A3CB2F,
	"PL2100": 0x718E1A14, "PL2200": 0x296471BE, "PL2300": 0xBAD16E3B, "PL2400": 0x1BB37EF0,
	"PL2500": 0x25D46F4B, "PL2600": 0x9A8AF295, "PL2700": 0x9B15CFB1, "PL2800": 0x646C3168,
	"PL2900": 0x74DD4C79,
}

func maximizeCapturedSigil(catalog *Catalog, source RuntimePatchPartySigil) RuntimePatchPartySigil {
	definition := catalog.LookupSigilByHash(source.Hash)
	if definition == nil {
		return source
	}
	if levels, err := catalog.RequireSigilLevels(definition); err == nil {
		if level := maxNaturalSigilLevel(naturalSigilLevelsForDefinition(definition, levels)); level > 0 {
			source.Level = uint32(level)
		}
	}
	if levels, err := catalog.RequirePrimaryTraitLevels(definition); err == nil {
		if level := maxNaturalSigilLevel(naturalSigilLevelsForDefinition(definition, levels)); level > 0 {
			source.PrimaryTraitLevel = uint32(level)
		}
	}
	if source.SecondaryTraitHash != 0 && source.SecondaryTraitHash != EmptyHash {
		if trait := catalog.LookupTraitByHash(source.SecondaryTraitHash); trait != nil {
			if levels, err := catalog.RequireSecondaryTraitLevels(definition, trait); err == nil {
				if level := maxNaturalSigilLevel(naturalSigilLevels(levels)); level > 0 {
					source.SecondaryTraitLevel = uint32(level)
				}
			}
		}
	}
	return source
}

func maximizeCapturedWrightstone(candidate RuntimePatchPartyWeapon) (*LoadoutWeaponWrightstone, error) {
	if runtimePatchPartyEmptyHash(candidate.WrightstoneID) || len(candidate.Traits) == 0 {
		return nil, nil
	}
	catalog, err := LoadWrightstoneCatalog()
	if err != nil {
		return nil, err
	}
	result := &LoadoutWeaponWrightstone{
		Hash: hashText(candidate.WrightstoneID), Evidence: "runtime-2.0.2-three-stable-reads",
		RuntimeObserved: true, StableReads: runtimePatchPartySnapshotCount,
	}
	if definition := catalog.LookupWrightstoneByHash(candidate.WrightstoneID); definition != nil {
		result.InternalID = definition.InternalID
		result.Name = cnWrightstone(definition.DisplayName)
	}
	slotMaximums := [...]int{20, 15, 10}
	for index, source := range candidate.Traits {
		if index >= len(slotMaximums) {
			break
		}
		level := int(source.Level)
		traitID, traitName := "", source.Name
		if definition := catalog.LookupTraitByHash(source.Hash); definition != nil {
			traitID = definition.InternalID
			traitName = cnWrightstoneTrait(definition.DisplayName)
			if levels, levelErr := requireWrightstoneTraitLevels(definition); levelErr == nil {
				for _, allowed := range levels {
					if allowed <= slotMaximums[index] && allowed > level {
						level = allowed
					}
				}
			}
		}
		if level > slotMaximums[index] {
			level = slotMaximums[index]
		}
		result.Traits = append(result.Traits, LoadoutWeaponWrightstoneTrait{
			Index: index, Hash: hashText(source.Hash), TraitID: traitID, Name: traitName, Level: level,
		})
	}
	return result, nil
}

func endgameCapturedWeapon(candidate RuntimePatchPartyWeapon, definition ProgressionWeaponDef) (*LoadoutShareWeaponState, error) {
	xp, err := weaponXPForLevel(definition.MaxLevel)
	if err != nil {
		return nil, err
	}
	skills := make([]string, 5)
	for index := range skills {
		skills[index] = hashText(EmptyHash)
	}
	for index, skill := range candidate.Skills {
		if index >= len(skills) {
			break
		}
		skills[index] = hashText(skill.Hash)
	}
	wrightstone, err := maximizeCapturedWrightstone(candidate)
	if err != nil {
		return nil, err
	}
	awakening := 0
	if definition.CanAwaken {
		awakening = 10
	}
	state := &LoadoutShareWeaponState{
		StoredHash: hashText(candidate.Hash), XP: xp, Uncap: 6, Mirage: 99,
		Awakening: awakening, Transcendence: 7, ExactState: false,
		SkillHashes: skills, Wrightstone: wrightstone,
	}
	if wrightstone != nil {
		state.WrightstoneReference = wrightstone.Hash
	}
	return state, nil
}

func runtimeLoadoutShareFromCandidate(candidate RuntimePatchPartyLoadout, title string) (*LoadoutShare, error) {
	if !candidate.Available || !candidate.Stable || candidate.SnapshotCount != runtimePatchPartySnapshotCount {
		return nil, fmt.Errorf("运行时配装尚未通过连续三快照稳定校验")
	}
	characterHash, known := runtimeOwnerCharacterHash[candidate.CharacterCode]
	if !known {
		return nil, fmt.Errorf("运行时角色 %s 没有可验证的存档角色映射", candidate.CharacterCode)
	}
	if _, err := loadProgressionCatalog(); err != nil {
		return nil, err
	}
	weaponHash := candidate.Weapon.Hash
	definition, known := progressionWeaponDefForHash(weaponHash)
	if !known || definition.OwnerCode != candidate.CharacterCode {
		return nil, fmt.Errorf("运行时武器 %08X 与角色 %s 不匹配", weaponHash, candidate.CharacterCode)
	}
	catalog, err := LoadCatalog()
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(title)
	if name == "" {
		name = candidate.CharacterName + runtimePatchMonitorText(" · 实时捕获配装", " · Runtime Capture")
	}
	share := &LoadoutShare{
		Format: loadoutShareFormat, Version: loadoutShareVersion,
		CharaHash: hashText(characterHash), CharaName: candidate.CharacterName, OwnerCode: candidate.CharacterCode, Name: name,
		WeaponHash: hashText(weaponHash), WeaponName: candidate.Weapon.Name,
		SourceKind: loadoutShareSourceRuntime, ProgressionPolicy: loadoutProgressionEndgame,
		CapturedFields: []string{"stats", "sigils", "skills", "weapon", "weaponSkills", "overLimit"},
	}
	for _, ability := range candidate.Abilities {
		share.Skills = append(share.Skills, LoadoutSkill{Hash: hashText(ability.Hash), Key: ability.Key, Name: ability.Name})
	}
	summonCatalog, err := loadSummonStatCatalog()
	if err != nil {
		return nil, err
	}
	if len(candidate.Summons) == 4 {
		share.CapturedFields = append(share.CapturedFields, "summons")
		for _, summon := range candidate.Summons {
			mainLevel, subLevel := int(summon.MainTraitLevel), int(summon.SubParamLevel)
			if definition, ok := summonCatalog.main[summon.MainTraitHash]; ok && definition.MaxLevel > mainLevel {
				mainLevel = definition.MaxLevel
			}
			if definition, ok := summonCatalog.sub[summon.SubParamHash]; ok && definition.MaxLevel > subLevel {
				subLevel = definition.MaxLevel
			}
			share.Summons = append(share.Summons, LoadoutShareSummon{
				TypeHash: hashText(summon.TypeHash), Name: summon.Name,
				MainTraitHash: hashText(summon.MainTraitHash), MainTraitLevel: mainLevel,
				SubParamHash: hashText(summon.SubParamHash), SubParamLevel: subLevel, Rank: 3,
			})
		}
	} else if len(candidate.Summons) > 4 {
		return nil, fmt.Errorf("运行时召唤石槽位 %d 个，超过 4 槽上限", len(candidate.Summons))
	}
	if candidate.MasteryAvailable {
		if len(candidate.Mastery) > loadoutMaxMastery {
			return nil, fmt.Errorf("运行时专精节点 %d 个，超过配装 50 槽上限", len(candidate.Mastery))
		}
		share.CapturedFields = append(share.CapturedFields, "mastery")
		share.MasteryHashes = make([]string, loadoutMaxMastery)
		for index := range share.MasteryHashes {
			share.MasteryHashes[index] = hashText(EmptyHash)
		}
		for index, node := range candidate.Mastery {
			share.MasteryHashes[index] = node.Hash
		}
	}
	for _, source := range candidate.Sigils {
		sigil := maximizeCapturedSigil(catalog, source)
		index := sigil.Index
		share.Sigils = append(share.Sigils, LoadoutShareSigil{
			Index: &index, Hash: hashText(sigil.Hash), Name: sigil.Name, Level: int(sigil.Level),
			PrimaryTraitHash: hashText(sigil.PrimaryTraitHash), PrimaryTraitLevel: int(sigil.PrimaryTraitLevel),
			SecondaryTraitHash: shareHex(sigil.SecondaryTraitHash), SecondaryTraitLevel: int(sigil.SecondaryTraitLevel),
		})
	}
	weaponSkills := make([]string, 5)
	for index := range weaponSkills {
		weaponSkills[index] = hashText(EmptyHash)
	}
	for index, skill := range candidate.Weapon.Skills {
		if index >= len(weaponSkills) {
			break
		}
		weaponSkills[index] = hashText(skill.Hash)
	}
	share.WeaponSkillHashes = weaponSkills
	share.Weapon, err = endgameCapturedWeapon(candidate.Weapon, definition)
	if err != nil {
		return nil, err
	}
	if share.Weapon.Wrightstone != nil {
		share.CapturedFields = append(share.CapturedFields, "wrightstone")
	}
	share.OverLimit = make([]LoadoutShareOverLimit, 4)
	for index := range share.OverLimit {
		share.OverLimit[index].Index = index
	}
	for _, slot := range candidate.OverLimit {
		if slot.Index < 0 || slot.Index >= len(share.OverLimit) {
			return nil, fmt.Errorf("运行时上限突破槽位 %d 越界", slot.Index+1)
		}
		level := 0
		if !runtimePatchPartyEmptyHash(slot.AttributeHash) {
			level = 10
		}
		share.OverLimit[slot.Index] = LoadoutShareOverLimit{Index: slot.Index, AttributeHash: shareHex(slot.AttributeHash), Level: level}
	}
	return share, nil
}

func previewForRuntimeLoadout(share *LoadoutShare, candidate RuntimePatchPartyLoadout) *loadoutSharePreview {
	entry := &LoadoutEntry{Mastery: append([]LoadoutMasteryNode(nil), candidate.Mastery...)}
	context := normalizedRuntimeSummonContext(share, candidate)
	normalizedCandidate := candidate
	normalizedCandidate.Weapon.Skills = normalizedRuntimeWeaponSkills(candidate)
	simulation := simulateRuntimePatchPartyTraits(share, normalizedCandidate)
	preview := previewForLoadout(share, entry, context, simulation)
	if preview == nil {
		return nil
	}
	for _, skill := range normalizedCandidate.Weapon.Skills {
		if runtimePatchPartyEmptyHash(skill.Hash) {
			continue
		}
		preview.WeaponSkills = append(preview.WeaponSkills, loadoutSharePreviewSkill{
			Hash: hashText(skill.Hash), Name: previewChineseName(hashText(skill.Hash), skill.Name), Level: int(skill.Level),
		})
	}
	if share.Weapon != nil && share.Weapon.Wrightstone != nil {
		source := share.Weapon.Wrightstone
		wrightstone := &loadoutSharePreviewWrightstone{Hash: source.Hash, Name: previewChineseWrightstoneName(source.Name)}
		for _, trait := range source.Traits {
			wrightstone.Traits = append(wrightstone.Traits, loadoutSharePreviewSkill{
				Hash: trait.Hash, Name: previewChineseName(trait.Hash, trait.Name), Level: trait.Level,
			})
		}
		preview.Wrightstone = wrightstone
	}
	return preview
}

func normalizedRuntimeSummonContext(share *LoadoutShare, candidate RuntimePatchPartyLoadout) *LoadoutStatContext {
	context := &LoadoutStatContext{}
	if share == nil {
		return context
	}
	catalog, _ := loadSummonStatCatalog()
	for index, source := range share.Summons {
		summon := LoadoutSummon{
			TypeHash: source.TypeHash, Name: source.Name, Rank: source.Rank,
			MainTraitHash: source.MainTraitHash, MainTraitLevel: source.MainTraitLevel,
			SubParamHash: source.SubParamHash, SubParamLevel: source.SubParamLevel,
		}
		if index < len(candidate.Summons) {
			observed := candidate.Summons[index]
			summon.MainTraitName = observed.MainTraitName
			summon.SubParamName = observed.SubParamName
		}
		if catalog != nil {
			if hash, err := ParseHashHex(source.MainTraitHash); err == nil {
				if definition, ok := catalog.main[hash]; ok {
					summon.MainTraitName = definition.Name
				}
			}
			if hash, err := ParseHashHex(source.SubParamHash); err == nil {
				if definition, ok := catalog.sub[hash]; ok {
					summon.SubParamName = definition.Name
					if source.SubParamLevel >= 0 && source.SubParamLevel < len(definition.Values) {
						if definition.IsPercent {
							summon.SubParamUnit = "pct"
						} else {
							summon.SubParamUnit = "flat"
						}
						summon.SubParamValue = definition.Values[source.SubParamLevel] * summonSubParamPanelScale(definition.Name, summon.SubParamUnit)
					}
				}
			}
		}
		context.EquippedSummons = append(context.EquippedSummons, summon)
	}
	return context
}

func normalizedRuntimeWeaponSkills(candidate RuntimePatchPartyLoadout) []RuntimePatchPartyTrait {
	result := append([]RuntimePatchPartyTrait(nil), candidate.Weapon.Skills...)
	data, _ := loadLoadoutWeaponStats()
	var row loadoutWeaponTableRow
	rowKnown := false
	if data != nil {
		row, rowKnown = resolveLoadoutWeaponTableRow(data, candidate.Weapon.Hash)
	}
	values := loadTraitValues()
	for index := range result {
		level := int(result[index].Level)
		hashTextValue := hashText(result[index].Hash)
		traitID := ""
		if data != nil {
			traitID = data.TraitIDs[hashTextValue]
		}
		if traitID == "SKILL_319_00" {
			if definition := values[traitID]; definition != nil && definition.MaxLevel > level {
				level = definition.MaxLevel
			}
			result[index].Level = uint32(level)
			continue
		}
		resolved := 0
		if data != nil && rowKnown && index < len(row.RebuildSkillLevelKeys) {
			group := row.RebuildSkillLevelKeys[index]
			if definition, ok := data.rebuildByGroupTrait[group+"|"+hashTextValue]; ok {
				resolved = definition.Levels[len(definition.Levels)-1]
			}
		}
		if data != nil && rowKnown && resolved <= 0 {
			for slot, value := range row.SkillHashes {
				if value == hashTextValue {
					resolved = max(resolved, weaponSkillLevel(data.SkillLevels[row.SkillLevelKeys[slot]], 6, 10))
				}
			}
			for slot, value := range row.AwakeningSkillHashes {
				if value == hashTextValue {
					resolved = max(resolved, weaponSkillLevel(data.SkillLevels[row.AwakeningSkillLevelKeys[slot]], 6, 10))
				}
			}
		}
		if data != nil && resolved <= 0 {
			for _, definition := range data.RebuildSkillLevels {
				if definition.Trait == hashTextValue {
					resolved = max(resolved, definition.Levels[len(definition.Levels)-1])
				}
			}
		}
		if resolved > level {
			level = resolved
		}
		result[index].Level = uint32(level)
	}
	return result
}

func simulateRuntimePatchPartyTraits(share *LoadoutShare, candidate RuntimePatchPartyLoadout) *LoadoutSimulation {
	catalog, err := LoadCatalog()
	if err != nil || share == nil {
		return nil
	}
	type pair struct {
		hash  uint32
		level int
	}
	factorPairs := make([]pair, 0, len(share.Sigils)*2)
	factorSources := make(map[uint32][]string)
	for index, sigil := range share.Sigils {
		for _, trait := range []struct {
			hash  string
			level int
			name  string
		}{{sigil.PrimaryTraitHash, sigil.PrimaryTraitLevel, previewChineseName(sigil.PrimaryTraitHash, "")}, {sigil.SecondaryTraitHash, sigil.SecondaryTraitLevel, previewChineseName(sigil.SecondaryTraitHash, "")}} {
			hash, parseErr := ParseHashHex(trait.hash)
			if parseErr != nil || runtimePatchPartyEmptyHash(hash) || trait.level <= 0 {
				continue
			}
			factorPairs = append(factorPairs, pair{hash: hash, level: trait.level})
			factorSources[hash] = append(factorSources[hash], loadoutFactorSource(index, trait.name, true))
		}
	}
	hashToID := traitHashMapWithRawKeys(catalog)
	factorBoost := 0
	for _, skill := range candidate.Weapon.Skills {
		if resolveTraitValueID(skill.Hash, hashToID) == "SKILL_113_00" {
			factorBoost = max(factorBoost, factorBoosterNumericLevelDelta(int(skill.Level)))
		}
	}
	convertedFactors := make([]struct {
		hash  uint32
		level int
	}, len(factorPairs))
	for index, item := range factorPairs {
		convertedFactors[index] = struct {
			hash  uint32
			level int
		}{item.hash, item.level}
	}
	if factorBoost > 0 {
		applyFactorLevelBoost(convertedFactors, factorBoost, hashToID)
	}
	pairs := append([]struct {
		hash  uint32
		level int
	}(nil), convertedFactors...)
	sourcesByTraitID := make(map[string][]string)
	addSource := func(hash uint32, source string) {
		id := canonicalTraitValueID(resolveTraitValueID(hash, hashToID))
		if id == "" || source == "" {
			return
		}
		for _, existing := range sourcesByTraitID[id] {
			if existing == source {
				return
			}
		}
		sourcesByTraitID[id] = append(sourcesByTraitID[id], source)
	}
	for hash, sources := range factorSources {
		for _, source := range sources {
			addSource(hash, source)
		}
	}
	if share.Weapon != nil && share.Weapon.Wrightstone != nil {
		for _, trait := range share.Weapon.Wrightstone.Traits {
			hash, parseErr := ParseHashHex(trait.Hash)
			if parseErr != nil || runtimePatchPartyEmptyHash(hash) || trait.Level <= 0 {
				continue
			}
			pairs = append(pairs, struct {
				hash  uint32
				level int
			}{hash, trait.Level})
			addSource(hash, loadoutWrightstoneSource(share.Weapon.Wrightstone.Name, trait.Name, trait.Level))
		}
	}
	for _, skill := range candidate.Weapon.Skills {
		if runtimePatchPartyEmptyHash(skill.Hash) || skill.Level == 0 {
			continue
		}
		pairs = append(pairs, struct {
			hash  uint32
			level int
		}{skill.Hash, int(skill.Level)})
		addSource(skill.Hash, loadoutWeaponSource(candidate.Weapon.Name, skill.Name, int(skill.Level)))
	}
	for index, summon := range share.Summons {
		hash, parseErr := ParseHashHex(summon.MainTraitHash)
		if parseErr != nil || runtimePatchPartyEmptyHash(hash) || summon.MainTraitLevel <= 0 {
			continue
		}
		pairs = append(pairs, struct {
			hash  uint32
			level int
		}{hash, summon.MainTraitLevel})
		addSource(hash, loadoutSummonSource(index, LoadoutSummon{Name: summon.Name, MainTraitHash: summon.MainTraitHash, MainTraitLevel: summon.MainTraitLevel}))
	}
	bonuses := simulateTraits(pairs, hashToID)
	for index := range bonuses {
		bonuses[index].Sources = append([]string(nil), sourcesByTraitID[bonuses[index].TraitID]...)
	}
	return &LoadoutSimulation{Bonuses: bonuses}
}

func runtimePatchPartyCombinedSkills(candidate RuntimePatchPartyLoadout) []TraitBonus {
	share := &LoadoutShare{}
	for _, sigil := range candidate.Sigils {
		index := sigil.Index
		share.Sigils = append(share.Sigils, LoadoutShareSigil{
			Index: &index, Hash: sigil.HashHex, Name: sigil.Name, Level: int(sigil.Level),
			PrimaryTraitHash: sigil.PrimaryTraitHashHex, PrimaryTraitLevel: int(sigil.PrimaryTraitLevel),
			SecondaryTraitHash: sigil.SecondaryTraitHashHex, SecondaryTraitLevel: int(sigil.SecondaryTraitLevel),
		})
	}
	if len(candidate.Weapon.Traits) > 0 {
		wrightstone := &LoadoutWeaponWrightstone{Hash: hashText(candidate.Weapon.WrightstoneID), Name: runtimePatchMonitorText("武器祝福", "Wrightstone")}
		for index, trait := range candidate.Weapon.Traits {
			wrightstone.Traits = append(wrightstone.Traits, LoadoutWeaponWrightstoneTrait{
				Index: index, Hash: trait.HashHex, Name: trait.Name, Level: int(trait.Level),
			})
		}
		share.Weapon = &LoadoutShareWeaponState{Wrightstone: wrightstone}
	}
	for _, summon := range candidate.Summons {
		share.Summons = append(share.Summons, LoadoutShareSummon{
			TypeHash: summon.TypeHashHex, Name: summon.Name,
			MainTraitHash: summon.MainTraitHex, MainTraitLevel: int(summon.MainTraitLevel),
			SubParamHash: summon.SubParamHex, SubParamLevel: int(summon.SubParamLevel),
		})
	}
	simulation := simulateRuntimePatchPartyTraits(share, candidate)
	if simulation == nil {
		return nil
	}
	return simulation.Bonuses
}

func (a *App) runtimePatchPartyLoadoutShareAndCandidateOwned(token, role, title string) (*LoadoutShare, *RuntimePatchPartyLoadout, error) {
	snapshot, err := a.RuntimePatchPartyMonitorOwned(token)
	if err != nil {
		return nil, nil, err
	}
	for _, entity := range snapshot.Entities {
		if entity.Role != role {
			continue
		}
		if !entity.Present || entity.Loadout == nil || !entity.Loadout.Available {
			return nil, nil, fmt.Errorf("%s 当前没有可用的稳定配装候选", runtimePatchPartyRoleName(role))
		}
		share, shareErr := runtimeLoadoutShareFromCandidate(*entity.Loadout, title)
		if shareErr != nil {
			return nil, nil, shareErr
		}
		candidate := *entity.Loadout
		return share, &candidate, nil
	}
	return nil, nil, fmt.Errorf("队伍中没有角色槽位 %s", role)
}

func (a *App) runtimePatchPartyLoadoutShareOwned(token, role, title string) (*LoadoutShare, error) {
	share, _, err := a.runtimePatchPartyLoadoutShareAndCandidateOwned(token, role, title)
	return share, err
}

func (a *App) RuntimePatchPartyLoadoutShareOwned(token, role, title string) (*LoadoutShareCodeResult, error) {
	share, err := a.runtimePatchPartyLoadoutShareOwned(token, role, title)
	if err != nil {
		return nil, err
	}
	return encodeLoadoutShareCode(share)
}

func (a *App) RuntimePatchPartyLoadoutExportOwned(token, role, title string) (string, error) {
	share, err := a.runtimePatchPartyLoadoutShareOwned(token, role, title)
	if err != nil {
		return "", err
	}
	outputPath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出实时捕获配装",
		DefaultFilename: safeLoadoutFilename(share.Name) + ".gbfr-loadout.json",
		Filters:         []runtime.FileFilter{{DisplayName: "GBFR 配装", Pattern: "*.gbfr-loadout.json"}},
	})
	if err != nil || outputPath == "" {
		return outputPath, err
	}
	payload, err := marshalLoadoutShare(share)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Clean(outputPath), payload, 0o600); err != nil {
		return "", fmt.Errorf("写出实时配装失败: %w", err)
	}
	return outputPath, nil
}

func (a *App) RuntimePatchPartyLoadoutPublishOwned(token, role, title string) (*LoadoutPublishedShare, error) {
	share, candidate, err := a.runtimePatchPartyLoadoutShareAndCandidateOwned(token, role, title)
	if err != nil {
		return nil, err
	}
	encoded, err := encodeLoadoutShareCode(share)
	if err != nil {
		return nil, err
	}
	frame, err := loadoutShareFrameFromCompatibilityCode(encoded.CompatibilityCode)
	if err != nil {
		return nil, err
	}
	preview := previewForRuntimeLoadout(share, *candidate)
	return publishLoadoutShareFrameWithMetadata(a.ctx, loadoutShareHTTPClient(), loadoutShareServiceURL, frame, preview)
}
