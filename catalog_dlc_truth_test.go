package main

import "testing"

type verifiedTraitTruth struct {
	id       string
	hash     string
	name     string
	cn       string
	maxLevel int
}

var verifiedWrightstoneTraitTruth = []verifiedTraitTruth{
	{id: "SKILL_078_00", hash: "0x6018372B", name: "Provoke", cn: "挑衅", maxLevel: 15},
	{id: "SKILL_088_00", hash: "0x9702860F", name: "Blight Resistance", cn: "灾祸抗性", maxLevel: 20},
	{id: "SKILL_079_00", hash: "0xF687C5EF", name: "Fast Learner", cn: "获得经验值", maxLevel: 20},
	{id: "SKILL_080_00", hash: "0xC86F3082", name: "Rupie Tycoon", cn: "获得金币", maxLevel: 20},
	{id: "SKILL_147_00", hash: "0x5E422AE5", name: "Path to Mastery", cn: "获得MSP", maxLevel: 20},
	{id: "SKILL_055_00", hash: "0x2242921F", name: "Paralysis Resistance", cn: "麻痹抗性", maxLevel: 20},
	{id: "SKILL_058_00", hash: "0xCFB48782", name: "SBA Sealed Resistance", cn: "奥义封印抗性", maxLevel: 20},
	{id: "SKILL_057_00", hash: "0x50B453DD", name: "Skill Sealed Resistance", cn: "能力封印抗性", maxLevel: 20},
	{id: "SKILL_052_00", hash: "0xFB572681", name: "Glaciate Resistance", cn: "冰冻抗性", maxLevel: 20},
	{id: "SKILL_051_00", hash: "0xD54F8CA7", name: "Sandtomb Resistance", cn: "泥沙抗性", maxLevel: 20},
	{id: "SKILL_065_00", hash: "0x9389CC06", name: "Improved Healing", cn: "回复性能", maxLevel: 20},
	{id: "SKILL_139_00", hash: "0x66DE60B1", name: "Defense Down Resistance", cn: "防御DOWN抗性", maxLevel: 20},
	{id: "SKILL_054_00", hash: "0x3759A5B9", name: "Dizzy Resistance", cn: "昏迷抗性", maxLevel: 20},
	{id: "SKILL_046_00", hash: "0x973B49AF", name: "Poison Resistance", cn: "中毒抗性", maxLevel: 20},
	{id: "SKILL_137_00", hash: "0xDD4A701E", name: "Darkflame Resistance", cn: "异能耐受", maxLevel: 20},
	{id: "SKILL_136_00", hash: "0x1DC9D7E7", name: "Held Under Resistance", cn: "水牢抗性", maxLevel: 20},
	{id: "SKILL_047_00", hash: "0x7C84A6B3", name: "Burn Resistance", cn: "灼热抗性", maxLevel: 20},
	{id: "SKILL_086_00", hash: "0xA2FA9685", name: "Slow Resistance", cn: "缓速抗性", maxLevel: 20},
}

var verifiedDLC202TraitTruth = []verifiedTraitTruth{
	{id: "SKILL_173_00", hash: "0x1DE14C65", name: "Gladiator's Frenzy", cn: "狼王的激昂", maxLevel: 15},
	{id: "SKILL_173_01", hash: "0x26956F25", name: "Gladiator's Top", cn: "狼王的大转轮", maxLevel: 15},
	{id: "SKILL_173_02", hash: "0xDBA19768", name: "Gladiator's Warpath", cn: "狼王的战气", maxLevel: 15},
	{id: "SKILL_174_00", hash: "0x7B5B081D", name: "Bladequeen's Serenade", cn: "刃姬的小夜曲", maxLevel: 15},
	{id: "SKILL_174_01", hash: "0x9ACE140B", name: "Bladequeen's Circuit", cn: "刃姬的轮舞曲", maxLevel: 15},
	{id: "SKILL_174_02", hash: "0x79266456", name: "Bladequeen's Warpath", cn: "刃姬的战气", maxLevel: 15},
	{id: "SKILL_175_00", hash: "0xD176D262", name: "Ultramarine's Flash", cn: "群青的剑光", maxLevel: 15},
	{id: "SKILL_175_01", hash: "0x461A8E07", name: "Ultramarine's Adversity", cn: "群青的逆境", maxLevel: 15},
	{id: "SKILL_175_02", hash: "0xB953CC1E", name: "Ultramarine's Warpath", cn: "群青的战气", maxLevel: 15},
	{id: "SKILL_176_00", hash: "0x7D75D904", name: "Thunderwolf's Recharge", cn: "雷狼的弹匣", maxLevel: 15},
	{id: "SKILL_176_01", hash: "0xBE3404B9", name: "Thunderwolf's Acuity", cn: "雷狼的慧眼", maxLevel: 15},
	{id: "SKILL_176_02", hash: "0x3EB345D7", name: "Thunderwolf's Warpath", cn: "雷狼的战气", maxLevel: 15},
	{id: "SKILL_177_00", hash: "0x47384248", name: "Enchantress's Blessing", cn: "转世的恩宠", maxLevel: 15},
	{id: "SKILL_177_01", hash: "0x30773197", name: "Enchantress's Rhythm", cn: "转世的跃动", maxLevel: 15},
	{id: "SKILL_177_02", hash: "0x807B6684", name: "Enchantress's Warpath", cn: "转世的战气", maxLevel: 15},
	{id: "SKILL_178_01", hash: "0xED8D8AD8", name: "The Black's Impulse", cn: "黑龙的折跃", maxLevel: 15},
	{id: "SKILL_178_02", hash: "0x5559232F", name: "The Black's Warpath", cn: "黑龙的战气", maxLevel: 15},
}

func TestVerifiedWrightstoneTraitCatalogIncludesV184Choices(t *testing.T) {
	// IDs and hashes are fixed by the locally unpacked GBFRDataTools ids.txt.
	// The 17 additional blessing choices and their level-20 storage range were
	// reported as in-game verified for v1.8.4; Provoke follows the verified
	// ordinary V-trait level-15 rule. Character-exclusive DLC traits are not
	// inferred to be blessing choices merely because their hashes are known.
	catalog, err := LoadWrightstoneCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(verifiedWrightstoneTraitTruth) != 18 {
		t.Fatalf("truth fixture must contain Provoke plus 17 v1.8.4 choices, got %d", len(verifiedWrightstoneTraitTruth))
	}
	for _, want := range verifiedWrightstoneTraitTruth {
		got, err := catalog.RequireTrait(want.id)
		if err != nil {
			t.Errorf("missing %s: %v", want.id, err)
			continue
		}
		if got.Hash != want.hash || got.DisplayName != want.name || got.MaxLevel == nil || *got.MaxLevel != want.maxLevel {
			t.Errorf("%s = hash %s name %q max %v; want %s %q %d", want.id, got.Hash, got.DisplayName, got.MaxLevel, want.hash, want.name, want.maxLevel)
		}
		if gotCN := wrightstoneTraitCN[want.name]; gotCN != want.cn {
			t.Errorf("%s Chinese name = %q; want %q", want.id, gotCN, want.cn)
		}
	}
	for _, forbidden := range verifiedDLC202TraitTruth {
		if got, err := catalog.RequireTrait(forbidden.id); err == nil {
			t.Errorf("character-exclusive trait leaked into blessing catalog: %+v", got)
		}
	}
}

func TestDLC202TraitsUseLocallyVerifiedIDsInSharedCatalog(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range verifiedDLC202TraitTruth {
		got, err := catalog.RequireTrait(want.id)
		if err != nil {
			t.Errorf("missing %s: %v", want.id, err)
			continue
		}
		if got.Hash != want.hash || got.DisplayName != want.name || got.MaxLevel == nil || *got.MaxLevel != want.maxLevel {
			t.Errorf("%s = hash %s name %q max %v; want %s %q %d", want.id, got.Hash, got.DisplayName, got.MaxLevel, want.hash, want.name, want.maxLevel)
		}
	}
}

func TestCorrectedFirmStanceAndFerryTruthMappings(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	firm, err := catalog.RequireSigil("GEEN_087_04")
	if err != nil {
		t.Fatal(err)
	}
	if firm.Hash != "0x80DC0FB8" {
		t.Fatalf("GEEN_087_04 hash = %s; want 0x80DC0FB8", firm.Hash)
	}
	for id, name := range map[string]string{
		"SKILL_120_00": "Phantasm's Harmony",
		"SKILL_120_01": "Phantasm's Concord",
	} {
		trait, err := catalog.RequireTrait(id)
		if err != nil {
			t.Fatal(err)
		}
		if trait.DisplayName != name {
			t.Errorf("%s name = %q; want %q", id, trait.DisplayName, name)
		}
	}
	awakening, err := catalog.RequireSigil("GEEN_120_90")
	if err != nil {
		t.Fatal(err)
	}
	if awakening.PrimaryTraitName == nil || *awakening.PrimaryTraitName != "Phantasm's Harmony" {
		t.Errorf("GEEN_120_90 primary name = %v; want Phantasm's Harmony", awakening.PrimaryTraitName)
	}
}

func TestMemoryDLCEnglishAliasesUseCurrentOfficialNames(t *testing.T) {
	for cn, want := range map[string]string{
		"狼王的激昂":     "Gladiator's Frenzy",
		"群青的剑光":     "Ultramarine's Flash",
		"相扑斗力":      "Sumo Force",
		"漆黑之谊":      "In a Pinch",
		"可怕的漆黑钳蟹因子": "Immortal Shell",
	} {
		if got := sigilMemoryEnglishNames[cn]; got != want {
			t.Errorf("memory alias %q = %q; want %q", cn, got, want)
		}
	}
}
