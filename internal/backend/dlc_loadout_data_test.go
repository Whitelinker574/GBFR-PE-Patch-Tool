package backend

import (
	"testing"
)

var dlcLoadoutOwners = []string{"PL2400", "PL2500", "PL2600", "PL2700", "PL2800", "PL2900"}

func TestDLCLoadoutTablesAreCompleteAndOwnerScoped(t *testing.T) {
	loadSkillboard()
	growth, err := loadLegacyMasteryCatalog()
	if err != nil {
		t.Fatal(err)
	}
	weapons, err := loadProgressionCatalog()
	if err != nil {
		t.Fatal(err)
	}
	weaponStats, err := loadLoadoutWeaponStats()
	if err != nil {
		t.Fatal(err)
	}

	weaponIDsByOwner := make(map[string]map[string]bool)
	weaponCountsByOwner := make(map[string]int)
	for _, weapon := range weapons.Weapons {
		if _, watched := map[string]bool{
			"PL2400": true, "PL2500": true, "PL2600": true,
			"PL2700": true, "PL2800": true, "PL2900": true,
		}[weapon.OwnerCode]; !watched {
			continue
		}
		weaponCountsByOwner[weapon.OwnerCode]++
		if weaponIDsByOwner[weapon.OwnerCode] == nil {
			weaponIDsByOwner[weapon.OwnerCode] = make(map[string]bool)
		}
		if weapon.InternalID != "" {
			weaponIDsByOwner[weapon.OwnerCode][weapon.InternalID] = true
		}
		hash, parseErr := ParseHashHex(weapon.Hash)
		if parseErr != nil {
			t.Fatalf("%s weapon %q has invalid hash: %v", weapon.OwnerCode, weapon.Hash, parseErr)
		}
		if _, ok := resolveLoadoutWeaponTableRow(weaponStats, hash); !ok {
			t.Errorf("%s weapon %s is absent from the official weapon stat table", weapon.OwnerCode, weapon.Hash)
		}
	}

	expectedWeapons := map[string]int{
		"PL2400": 4, "PL2500": 4, "PL2600": 6,
		"PL2700": 6, "PL2800": 4, "PL2900": 4,
	}
	for _, owner := range dlcLoadoutOwners {
		nodeCount := 0
		groups := make(map[string]bool)
		for _, node := range skillboardAllNodes {
			if node.Char == owner {
				nodeCount++
				groups[node.Grp] = true
			}
		}
		if nodeCount != 99 {
			t.Errorf("%s skillboard has %d nodes, want 99 from its own 2.0.2 rows", owner, nodeCount)
		}
		for _, rank := range masteryRanks {
			if !groups[rank.Grp] {
				t.Errorf("%s skillboard lacks %s group %s", owner, rank.Rank, rank.Grp)
			}
		}

		character, ok := growth.Characters[owner]
		if !ok {
			t.Errorf("%s lacks a four-tab character-growth table", owner)
			continue
		}
		if character.Attack.FullMSP <= 0 || character.Defense.FullMSP <= 0 ||
			character.Attack.Attack <= 0 || character.Defense.HP <= 0 || len(character.WeaponNodes) == 0 {
			t.Errorf("%s character-growth table is incomplete: %+v", owner, character)
		}
		for _, node := range character.WeaponNodes {
			if !weaponIDsByOwner[owner][node.WeaponID] {
				t.Errorf("%s collection node references unknown character weapon %s", owner, node.WeaponID)
			}
		}
		if weaponCountsByOwner[owner] != expectedWeapons[owner] {
			t.Errorf("%s weapon catalog has %d entries, want %d", owner, weaponCountsByOwner[owner], expectedWeapons[owner])
		}
	}
}

func TestDLCFateGrowthUsesCharacterRows(t *testing.T) {
	hashes := map[string]uint32{
		"PL2400": 0x1BB37EF0, "PL2500": 0x25D46F4B, "PL2600": 0x9A8AF295,
		"PL2700": 0x9B15CFB1, "PL2800": 0x646C3168, "PL2900": 0x74DD4C79,
	}
	for _, owner := range dlcLoadoutOwners {
		t.Run(owner, func(t *testing.T) {
			growth, known := deriveFateGrowth(hashes[owner], 0x7FF)
			if !known {
				t.Fatalf("%s has no 2.0.2 Fate row", owner)
			}
			if owner == "PL2900" {
				if growth.HP != 0 || growth.ATK != 0 {
					t.Fatalf("%s zero-stat Fate table = %+v", owner, growth)
				}
				return
			}
			if growth.HP != 640 || growth.ATK != 165 {
				t.Fatalf("%s Fate growth = %+v, want +640 HP/+165 ATK", owner, growth)
			}
		})
	}
}

func TestDLCUnconditionalTranscendenceTraitsEnterTheCorrectPanelChannels(t *testing.T) {
	pairs := []struct {
		hash  uint32
		level int
	}{
		{0x235D86EF, 15}, // 超新星: attack + generic cap
		{0xBBD77C33, 15}, // normal cap only
		{0x020DB733, 15}, // ability cap only
		{0x3F682593, 15}, // SBA cap only
		{0x79027FC8, 55}, // generic cap
	}
	ids := map[uint32]string{
		0x235D86EF: "SKILL_313_00", 0xBBD77C33: "SKILL_316_00",
		0x020DB733: "SKILL_317_00", 0x3F682593: "SKILL_318_00",
		0x79027FC8: "SKILL_319_00",
	}
	bonuses := simulateTraits(pairs, ids)
	if len(bonuses) != len(pairs) {
		t.Fatalf("DLC transcendence definitions joined %d/%d traits: %+v", len(bonuses), len(pairs), bonuses)
	}
	stats := calculateLoadoutFinalStats(loadoutPanelInputs{CharacterATK: 1000, Bonuses: bonuses})
	if stats.Attack != 1400 {
		t.Errorf("Supernova attack = %d, want 1400", stats.Attack)
	}
	if stats.NormalDamageCap != 430 || stats.AbilityDamageCap != 430 || stats.SkyboundDamageCap != 450 {
		t.Errorf("DLC cap channels = normal %.1f ability %.1f SBA %.1f, want 430/430/450", stats.NormalDamageCap, stats.AbilityDamageCap, stats.SkyboundDamageCap)
	}
	if stats.ChainDamageCap != 0 {
		t.Errorf("generic cap was inferred to affect chain burst without evidence: %.1f", stats.ChainDamageCap)
	}
}

func TestDLCUnboundMasterUsesCharacterMasterProgressLevel(t *testing.T) {
	skills := []LoadoutWeaponSkill{{
		TraitID: "SKILL_319_00", TraitHash: "79027FC8", Level: 1,
		UnlockCondition: "超凡 7/7 · 当前阶段技能表",
	}}
	applyMasterProgressWeaponSkillLevels(skills, LoadoutPermanentGrowth{
		MasterSystemAvailable: true,
		MasterProgressIndex:   55,
	})
	if skills[0].StaticLevel != 1 || skills[0].Level != 55 || skills[0].Effect != "伤害上限+50.0%" {
		t.Fatalf("Unbound Master effective level = %+v", skills[0])
	}
	if skills[0].UnlockCondition != "超凡 7/7 · 当前阶段技能表 · 按角色专精进度 Lv55 生效" {
		t.Fatalf("Unbound Master level evidence is missing: %q", skills[0].UnlockCondition)
	}

	runtime := []LoadoutWeaponSkill{{
		TraitID: "SKILL_319_00", Level: 42, RuntimeObserved: true,
	}}
	applyMasterProgressWeaponSkillLevels(runtime, LoadoutPermanentGrowth{
		MasterSystemAvailable: true,
		MasterProgressIndex:   55,
	})
	if runtime[0].Level != 42 {
		t.Fatalf("runtime-observed skill level was overwritten: %+v", runtime[0])
	}
}
