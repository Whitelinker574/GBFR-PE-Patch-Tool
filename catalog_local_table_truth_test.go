package main

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestFerryWrightstoneNamesMatchSharedTraitTruth(t *testing.T) {
	shared, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	wrightstones, err := LoadWrightstoneCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"SKILL_120_00", "SKILL_120_01"} {
		want, err := shared.RequireTrait(id)
		if err != nil {
			t.Fatal(err)
		}
		got, err := wrightstones.RequireTrait(id)
		if err != nil {
			t.Fatal(err)
		}
		if got.Hash != want.Hash || got.DisplayName != want.DisplayName {
			t.Errorf("wrightstone %s = %s %q; shared trait truth is %s %q", id, got.Hash, got.DisplayName, want.Hash, want.DisplayName)
		}
	}
}

func TestSummonSkillNamesMatchLocalSimplifiedChineseText(t *testing.T) {
	// These mappings are the exact TXT_SKILL_* strings from the locally
	// unpacked 2.0.2 simplified-Chinese message table. The eight entries are
	// independently referenced by summon_lot and summon_preset.
	want := map[string]string{
		"0x0DE887A0": "天星之炼",
		"0xA7726190": "天星之煌",
		"0x9232DC17": "天星之界",
		"0x36E3848D": "天星之焰",
		"0xA898E283": "天星之雪",
		"0xD029FE08": "浪迹天涯",
		"0x73220725": "天星之止息",
		"0xF26BAEA5": "分歧",
	}
	var payload summonSkillFile
	if err := json.Unmarshal(summonSkillsJSON, &payload); err != nil {
		t.Fatal(err)
	}
	got := make(map[string]string, len(payload.Skills))
	for _, skill := range payload.Skills {
		got[skill.Hash] = skill.DisplayName
	}
	for hash, name := range want {
		if got[hash] != name {
			t.Errorf("summon skill %s = %q; want %q", hash, got[hash], name)
		}
	}
}

func TestPhantasmEnglishAliasesUseTheSameHashBackedChineseTruth(t *testing.T) {
	for english, want := range map[string]string{
		"Phantasm's Harmony":  "幽幻之谊",
		"Phantasm's Concord":  "幽幻之应",
		"Phantasm's Harmony+": "幽幻之谊+",
		"Phantasm's Concord+": "幽幻之应+",
	} {
		var got string
		if english[len(english)-1] == '+' {
			got = sigilCN[english]
		} else {
			got = traitCN[english]
		}
		if got != want {
			t.Errorf("%s Chinese alias = %q; want %q", english, got, want)
		}
	}
}

func TestFirmStanceGradeFiveUsesNaturalLevelFifteen(t *testing.T) {
	// Local gem.tbl identifies GEEN_087_04 as the fifth-rank row whose primary
	// trait is SKILL_087_00. The constructor's highest-tier-only policy uses
	// item/primary level 15; level 5 was a rank-number transcription and
	// contradicted the trait.
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	firm, err := catalog.RequireSigil("GEEN_087_04")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(firm.AllowedSigilLevels, []int{15}) || firm.DefaultSigilLevel == nil || *firm.DefaultSigilLevel != 15 || firm.MaxSigilLevel == nil || *firm.MaxSigilLevel != 15 {
		t.Fatalf("GEEN_087_04 sigil levels = allowed %v default %v max %v; want [15], 15, 15", firm.AllowedSigilLevels, firm.DefaultSigilLevel, firm.MaxSigilLevel)
	}
	if firm.PrimaryTraitID != "SKILL_087_00" || !slices.Equal(firm.AllowedFirstTraitLevels, []int{15}) {
		t.Fatalf("GEEN_087_04 primary = %s levels %v; want SKILL_087_00 [15]", firm.PrimaryTraitID, firm.AllowedFirstTraitLevels)
	}
	if !catalog.IsSigilConstructible(firm) {
		t.Fatal("GEEN_087_04 must be constructible after gem.tbl verifies it has no secondary slot")
	}
	if len(firm.AllowedSecondaryTraitIDs) != 0 {
		t.Fatalf("GEEN_087_04 secondary pool = %v; local 2.0.2 gem.tbl has no secondary lot", firm.AllowedSecondaryTraitIDs)
	}
}

func TestWeaponTranscendenceSkillsMatchLocal202Tables(t *testing.T) {
	// Names/formats come from table/text/cs/text.msg. Numeric anchors come from
	// the 52-byte 2.0.2 skill_status.tbl rows (ten LevelValue columns).
	tests := []struct {
		id, hash, name, format string
		max, ph, level         int
		value                  float64
	}{
		{"SKILL_311_00", "3B71AF12", "伤害上限·轰天", "效果最小时攻击力+{0}% / 伤害上限+{2}%（最大HP不低于{4}）\n效果最大时攻击力+{1}% / 伤害上限+{3}%（最大HP不低于{5}）", 15, 5, 15, 200000},
		{"SKILL_312_00", "FFF8CF64", "伤害上限·疾天", "效果最大时攻击力+{5}% / 伤害上限+{6}%（疾天Ⅴ）\n连锁计数每增加{4}%提升1阶段强化效果", 15, 6, 15, 280},
		{"SKILL_313_00", "235D86EF", "超新星", "攻击力+{0}% / 伤害上限+{1}%", 15, 1, 15, 350},
		{"SKILL_314_00", "AEFEB1BC", "伤害上限·苍天", "效果最小时攻击力+{0}% / 伤害上限+{2}%（暴击率不低于{4}%）\n效果最大时攻击力+{1}% / 伤害上限+{3}%（暴击率不低于{5}%）", 15, 3, 15, 270},
		{"SKILL_315_00", "0151CF9E", "伤害上限·红天", "效果最大时攻击力+{5}% / 伤害上限+{6}%（红天Ⅴ）\n每造成一定伤害后提升1阶段强化效果（效果持续时间{4}秒）", 15, 6, 15, 310},
		{"SKILL_316_00", "BBD77C33", "超凡强击", "普通攻击伤害上限+{0}%", 15, 0, 15, 30},
		{"SKILL_317_00", "020DB733", "超凡技艺", "能力伤害上限+{0}%", 15, 0, 15, 30},
		{"SKILL_318_00", "3F682593", "超凡奥秘", "奥义伤害上限+{0}%", 15, 0, 15, 50},
		{"SKILL_319_00", "79027FC8", "超凡破限", "伤害上限+{0:.1f}%", 55, 0, 55, 50},
	}
	values := loadTraitValues()
	for _, tt := range tests {
		definition := values[tt.id]
		if definition == nil {
			definition = values[tt.hash]
		}
		if definition == nil {
			t.Errorf("missing %s/%s", tt.id, tt.hash)
			continue
		}
		if definition.Name != tt.name || definition.Format != tt.format || definition.MaxLevel != tt.max {
			t.Errorf("%s metadata = name %q format %q max %d; want %q %q %d", tt.id, definition.Name, definition.Format, definition.MaxLevel, tt.name, tt.format, tt.max)
		}
		found := false
		for _, placeholder := range definition.Placeholders {
			if placeholder.Ph != tt.ph {
				continue
			}
			found = true
			if len(placeholder.Values) < tt.level || placeholder.Values[tt.level-1] != tt.value {
				t.Errorf("%s placeholder %d level %d = %v; want %v", tt.id, tt.ph, tt.level, placeholder.Values, tt.value)
			}
		}
		if !found {
			t.Errorf("%s missing placeholder %d", tt.id, tt.ph)
		}
	}
}
