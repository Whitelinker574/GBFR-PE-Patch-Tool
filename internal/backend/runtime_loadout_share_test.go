package backend

import (
	"strings"
	"testing"
)

func TestRuntimePatchPartyCombinedSkillsIncludesWeaponAndMasteryWithoutInventingOverlimitLevels(t *testing.T) {
	candidate := RuntimePatchPartyLoadout{
		CharacterCode:    "PL1600",
		MasteryAvailable: true,
		Mastery: []LoadoutMasteryNode{
			{Hash: "01EE7C0A", Cat: "SB_LIMIT", Rank: "R1", RankLabel: "R1", Desc: "最大HP+15000"},
		},
		Weapon: RuntimePatchPartyWeapon{
			Name: "阿尔贝斯之枪",
			Skills: []RuntimePatchPartyTrait{
				{Hash: 0x7EDD69D0, HashHex: "7EDD69D0", Name: "攻击力", Level: 15},
			},
		},
		OverLimit: []RuntimePatchPartyOverLimit{
			{AttributeHash: 0x52A207B5, HashHex: "52A207B5", Name: "暴击率", Level: 20},
		},
	}

	combined := runtimePatchPartyCombinedSkills(candidate)
	foundWeapon := false
	foundMastery := false
	for _, bonus := range combined {
		for _, source := range bonus.Sources {
			if strings.Contains(source, "阿尔贝斯之枪") || strings.Contains(source, "武器") {
				foundWeapon = true
			}
			if strings.Contains(source, "专精") || strings.HasPrefix(bonus.TraitID, "MASTERY:") {
				foundMastery = true
			}
		}
		if bonus.TraitID == "52A207B5" || strings.Contains(strings.Join(bonus.Sources, " "), "上限突破") {
			t.Fatalf("overlimit was incorrectly represented as a trait level: %+v", bonus)
		}
	}
	if !foundWeapon || !foundMastery {
		t.Fatalf("combined skill sources incomplete: weapon=%v mastery=%v bonuses=%+v", foundWeapon, foundMastery, combined)
	}
}

func TestPreviewForRuntimeLoadoutKeepsRuntimeCombinedSkills(t *testing.T) {
	index := 0
	candidate := RuntimePatchPartyLoadout{
		Available: true, Stable: true, SnapshotCount: 3,
		CharacterCode: "PL1600", CharacterHash: "4D0A60C3", CharacterName: "泽塔", MasteryAvailable: true,
		Mastery:   []LoadoutMasteryNode{{Hash: "01EE7C0A", Cat: "SB_LIMIT", Rank: "R1", RankLabel: "R1", Desc: "最大HP+15000"}},
		Weapon:    RuntimePatchPartyWeapon{Hash: 0x02352554, HashHex: "02352554", Name: "阿尔贝斯之枪", Skills: []RuntimePatchPartyTrait{{Hash: 0x7EDD69D0, HashHex: "7EDD69D0", Name: "攻击力", Level: 15}}},
		Sigils:    []RuntimePatchPartySigil{{Index: index, Hash: 0x2D7F2E70, HashHex: "2D7F2E70", Name: "攻击力 V+", Level: 15, PrimaryTraitHash: 0x50079A1C, PrimaryTraitHashHex: "50079A1C", PrimaryTraitName: "攻击力", PrimaryTraitLevel: 15}},
		OverLimit: []RuntimePatchPartyOverLimit{{AttributeHash: 0x52A207B5, HashHex: "52A207B5", Name: "暴击率", Level: 10, Value: 20}},
	}
	share, err := runtimeLoadoutShareFromCandidate(candidate, "测试")
	if err != nil {
		t.Fatal(err)
	}
	preview := previewForRuntimeLoadout(share, candidate)
	if preview == nil || len(preview.CombinedSkills) == 0 {
		t.Fatalf("runtime public preview lost combined skills: %+v", preview)
	}
	foundMastery := false
	for _, skill := range preview.CombinedSkills {
		if strings.HasPrefix(skill.Hash, "MASTERY:") {
			foundMastery = true
		}
	}
	if !foundMastery || len(preview.OverLimit) != 4 || preview.OverLimit[0].Name == "" || preview.OverLimit[0].Value <= 0 {
		t.Fatalf("runtime public preview missing derived sources: mastery=%v overlimit=%+v skills=%+v", foundMastery, preview.OverLimit, preview.CombinedSkills)
	}
}

func TestRuntimeLoadoutShareUsesPartialV11AndKeepsCapturedScopes(t *testing.T) {
	index := 0
	candidate := RuntimePatchPartyLoadout{
		Available: true, Stable: true, SnapshotCount: 3, CharacterCode: "PL1600", CharacterName: "泽塔",
		Abilities: []RuntimePatchPartyAbility{{Hash: 0x95E40E12, HashHex: "95E40E12", Key: "AB_PL1600_01", Name: "无尽奇观"}},
		Summons: []RuntimePatchPartySummon{{
			Index: 0, TypeHash: 0x3FD89C3A, TypeHashHex: "3FD89C3A", Name: "世须良加 · 史诗 · 伤害",
			MainTraitHash: 0x50079A1C, MainTraitHex: "50079A1C", MainTraitName: "攻击力", MainTraitLevel: 1,
			SubParamHash: 0x5A39D81B, SubParamHex: "5A39D81B", SubParamName: "奥义连锁伤害（低·最高50%）", SubParamLevel: 1,
		}},
		MasterLevel: 55, MasteryAvailable: true,
		Mastery: []LoadoutMasteryNode{{Hash: "01EE7C0A", Cat: "SB_LIMIT", Rank: "R1", RankLabel: "R1", Desc: "最大HP+15000"}},
		Weapon: RuntimePatchPartyWeapon{
			Hash: 0x02352554, HashHex: "02352554", Name: "阿尔贝斯之枪", XP: 123456,
			Level: 80, StarLevel: 2, PlusMarks: 3, AwakeningLevel: 1, WrightstoneID: 0x09E6F629,
			Traits: []RuntimePatchPartyTrait{{Hash: 0x50079A1C, HashHex: "50079A1C", Name: "攻击力", Level: 5}},
			Skills: []RuntimePatchPartyTrait{{Hash: 0x7EDD69D0, HashHex: "7EDD69D0", Name: "攻击力", Level: 15}},
		},
		Sigils: []RuntimePatchPartySigil{{
			Index: index, Hash: 0x2D7F2E70, HashHex: "2D7F2E70", Name: "攻击力 V+", Level: 15,
			PrimaryTraitHash: 0x50079A1C, PrimaryTraitHashHex: "50079A1C", PrimaryTraitName: "攻击力", PrimaryTraitLevel: 15,
			SecondaryTraitHash: 0xDC584F60, SecondaryTraitHashHex: "DC584F60", SecondaryTraitName: "伤害上限", SecondaryTraitLevel: 15,
		}},
		OverLimit: []RuntimePatchPartyOverLimit{
			{Index: 0, AttributeHash: 0x52A207B5, HashHex: "52A207B5", Level: 10},
			{Index: 1}, {Index: 2}, {Index: 3},
		},
	}
	for len(candidate.Summons) < 4 {
		copyValue := candidate.Summons[0]
		copyValue.Index = len(candidate.Summons)
		candidate.Summons = append(candidate.Summons, copyValue)
	}

	share, err := runtimeLoadoutShareFromCandidate(candidate, "泽塔常规毕业配装")
	if err != nil {
		t.Fatal(err)
	}
	if share.Version != 11 || share.SourceKind != loadoutShareSourceRuntime || share.ProgressionPolicy != loadoutProgressionEndgame {
		t.Fatalf("capture metadata=%+v", share)
	}
	if share.CharaHash != "0D21B430" || share.OwnerCode != "PL1600" || len(share.Sigils) != 1 {
		t.Fatalf("runtime share identity=%+v", share)
	}
	if len(share.Skills) != 1 || share.Skills[0].Hash != "95E40E12" || len(share.Summons) != 4 || share.Summons[0].TypeHash != "3FD89C3A" || len(share.MasteryHashes) != 50 || share.MasteryHashes[0] != "01EE7C0A" {
		t.Fatalf("runtime expansion share fields=%+v", share)
	}
	if len(share.WeaponSkillHashes) != 5 || share.WeaponSkillHashes[0] != "7EDD69D0" || share.WeaponSkillHashes[1] != "887AE0B0" {
		t.Fatalf("weapon skill snapshot=%v", share.WeaponSkillHashes)
	}
	if share.Weapon == nil || share.Weapon.XP != weaponExpByLevel[149] || share.Weapon.Uncap != 6 || share.Weapon.Mirage != 99 || share.Weapon.Awakening != 10 || share.Weapon.Transcendence != 7 {
		t.Fatalf("runtime weapon was not normalized to endgame progression: %+v", share.Weapon)
	}
	if share.Weapon.Wrightstone == nil || len(share.Weapon.Wrightstone.Traits) != 1 || share.Weapon.Wrightstone.Traits[0].Level != 20 {
		t.Fatalf("runtime wrightstone was not preserved and normalized: %+v", share.Weapon.Wrightstone)
	}
	if share.OverLimit[0].Level != 10 {
		t.Fatalf("runtime overlimit was not normalized: %+v", share.OverLimit)
	}
	preview := previewForRuntimeLoadout(share, candidate)
	if len(preview.WeaponSkills) != 1 || preview.WeaponSkills[0].Hash != "7EDD69D0" || preview.WeaponSkills[0].Name == "" {
		t.Fatalf("runtime public preview lost weapon skills: %+v", preview.WeaponSkills)
	}
	if preview.Wrightstone == nil || preview.Wrightstone.Hash != "09E6F629" || len(preview.Wrightstone.Traits) != 1 || preview.Wrightstone.Traits[0].Level != 20 {
		t.Fatalf("runtime public preview lost normalized wrightstone: %+v", preview.Wrightstone)
	}
	if preview.WeaponSkills[0].Level != 25 {
		t.Fatalf("runtime public preview kept captured weapon skill level instead of endgame level: %+v", preview.WeaponSkills)
	}
	if preview.Summons[0].MainTraitLevel != share.Summons[0].MainTraitLevel || preview.Summons[0].SubParamLevel != share.Summons[0].SubParamLevel || preview.Summons[0].MainTraitLevel == 1 {
		t.Fatalf("runtime public preview does not match normalized summon payload: preview=%+v share=%+v", preview.Summons[0], share.Summons[0])
	}
	if len(preview.Abilities) != 1 || len(preview.Summons) != 4 || len(preview.MasterySkills) != 1 || len(preview.CombinedSkills) == 0 {
		t.Fatalf("runtime public preview lost expansion fields: %+v", preview)
	}
	encoded, err := encodeLoadoutShareCode(share)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeLoadoutShareCode(encoded.CompatibilityCode)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.SourceKind != loadoutShareSourceRuntime || len(decoded.Summons) != 4 || len(decoded.MasteryHashes) != 50 || len(decoded.Skills) != 1 || len(decoded.OverLimit) != 4 {
		t.Fatalf("decoded partial share=%+v", decoded)
	}
	if decoded.ProgressionPolicy != loadoutProgressionEndgame || !loadoutShareHasCapturedField(decoded, "summons") || !loadoutShareHasCapturedField(decoded, "mastery") || !loadoutShareHasCapturedField(decoded, "skills") || loadoutShareHasCapturedField(decoded, "character") {
		t.Fatalf("partial share exposed an unobserved import scope: %+v", decoded)
	}
}

func TestRuntimeLoadoutShareRejectsUnstableOrUnknownCharacter(t *testing.T) {
	for _, candidate := range []RuntimePatchPartyLoadout{
		{Available: true, Stable: false, SnapshotCount: 3, CharacterCode: "PL1600"},
		{Available: true, Stable: true, SnapshotCount: 3, CharacterCode: "PL9999"},
	} {
		if _, err := runtimeLoadoutShareFromCandidate(candidate, ""); err == nil {
			t.Fatalf("candidate should fail: %+v", candidate)
		}
	}
}

func TestRuntimeLoadoutShareOmitsIncompleteSummonScope(t *testing.T) {
	for _, count := range []int{0, 1, 3} {
		candidate := RuntimePatchPartyLoadout{
			Available: true, Stable: true, SnapshotCount: 3, CharacterCode: "PL1600", CharacterName: "泽塔",
			Weapon: RuntimePatchPartyWeapon{Hash: 0x02352554, HashHex: "02352554", Name: "阿尔贝斯之枪"},
			Sigils: []RuntimePatchPartySigil{{
				Index: 0, Hash: 0x2D7F2E70, HashHex: "2D7F2E70", Name: "攻击力 V+", Level: 15,
				PrimaryTraitHash: 0x50079A1C, PrimaryTraitHashHex: "50079A1C", PrimaryTraitName: "攻击力", PrimaryTraitLevel: 15,
			}},
			OverLimit: []RuntimePatchPartyOverLimit{{Index: 0}, {Index: 1}, {Index: 2}, {Index: 3}},
		}
		candidate.Summons = make([]RuntimePatchPartySummon, count)
		share, err := runtimeLoadoutShareFromCandidate(candidate, "不完整召唤石捕获")
		if err != nil {
			t.Fatalf("summon count %d: %v", count, err)
		}
		if loadoutShareHasCapturedField(share, "summons") || len(share.Summons) != 0 {
			t.Fatalf("summon count %d exposed an incomplete scope: %+v", count, share)
		}
		if _, err := encodeLoadoutShareCode(share); err != nil {
			t.Fatalf("summon count %d cannot encode remaining scopes: %v", count, err)
		}
	}
}

func TestV11CaptureProvenanceRejectsUnknownSourcesAndUndeclaredPayloads(t *testing.T) {
	index := 0
	base := &LoadoutShare{
		Format: loadoutShareFormat, Version: 11, CharaHash: "0D21B430", CharaName: "泽塔", OwnerCode: "PL1600", Name: "测试配装",
		SourceKind: loadoutShareSourceRuntime, ProgressionPolicy: loadoutProgressionEndgame, CapturedFields: []string{"sigils"},
		Sigils: []LoadoutShareSigil{{Index: &index, Hash: "2D7F2E70", Name: "攻击力 V+", PrimaryTraitHash: "50079A1C"}},
	}
	for name, mutate := range map[string]func(*LoadoutShare){
		"unknown source": func(share *LoadoutShare) { share.SourceKind = "other" },
		"wrong policy":   func(share *LoadoutShare) { share.ProgressionPolicy = loadoutProgressionExact },
		"duplicate field": func(share *LoadoutShare) {
			share.CapturedFields = []string{"sigils", "sigils"}
		},
		"undeclared skills": func(share *LoadoutShare) { share.Skills = []LoadoutSkill{{Hash: "12345678"}} },
		"undeclared weapon": func(share *LoadoutShare) { share.WeaponHash = "02352554" },
		"captured weapon without payload": func(share *LoadoutShare) {
			share.CapturedFields = []string{"sigils", "weapon"}
		},
		"weapon skills without weapon": func(share *LoadoutShare) {
			share.CapturedFields = []string{"sigils", "weaponSkills"}
			share.WeaponSkillHashes = []string{"887AE0B0", "887AE0B0", "887AE0B0", "887AE0B0", "887AE0B0"}
		},
		"wrightstone without weapon": func(share *LoadoutShare) {
			share.CapturedFields = []string{"sigils", "wrightstone"}
		},
	} {
		t.Run(name, func(t *testing.T) {
			copyValue := *base
			copyValue.CapturedFields = append([]string(nil), base.CapturedFields...)
			mutate(&copyValue)
			if err := validateLoadoutShareProvenance(&copyValue); err == nil {
				t.Fatalf("invalid provenance was accepted: %+v", copyValue)
			}
		})
	}
}
