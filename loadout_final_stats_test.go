package main

import (
	"math"
	"strings"
	"testing"
)

func TestParseMasteryPanelBonusesOnlyCountsUnconditionalPanelStats(t *testing.T) {
	tests := []struct {
		desc      string
		wantLabel string
		wantUnit  string
		wantValue float64
		want      bool
	}{
		{"最大HP+15000", "最大HP", "flat", 15000, true},
		{"攻击力+500", "攻击力", "flat", 500, true},
		{"攻击力+12.5%", "攻击力", "pct", 12.5, true},
		{"暴击率+25%", "暴击率", "pct", 25, true},
		{"昏厥值+0.4", "昏厥值", "flat", 0.4, true},
		{"伤害上限+30%", "伤害上限", "pct", 30, true},
		{"攻击的伤害上限+35%", "攻击的伤害上限", "pct", 35, true},
		{"能力的伤害上限+35%", "能力的伤害上限", "pct", 35, true},
		{"奥义的伤害上限+25%", "奥义的伤害上限", "pct", 25, true},
		{"花耀七闪的伤害上限+45%", "", "", 0, false},
		{"自身陷入中毒状态期间伤害上限+100%", "", "", 0, false},
		{"攻击力随攻击类因子装备数量增加而提升", "", "", 0, false},
		{"专精生效时 攻击力+20%", "", "", 0, false},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			got, ok := parseMasteryPanelBonus(test.desc, "专精")
			if ok != test.want {
				t.Fatalf("parseMasteryPanelBonus(%q) ok=%v, want %v: %+v", test.desc, ok, test.want, got)
			}
			if !test.want {
				return
			}
			if got.Label != test.wantLabel || got.Unit != test.wantUnit || got.Value != test.wantValue || got.Source != "专精" {
				t.Fatalf("parseMasteryPanelBonus(%q)=%+v", test.desc, got)
			}
		})
	}
}

func TestAddPanelBonusesToTotalsCanonicalizesMasteryDamageCaps(t *testing.T) {
	totals := []EffectTotal{{Key: "pct|普通攻击伤害上限", Label: "普通攻击伤害上限", Unit: "pct", Value: 10, CatLabel: "攻击类", Sources: []string{"因子"}}}
	addPanelBonusesToTotals(&totals, []LoadoutPanelBonus{
		{Label: "伤害上限", Unit: "pct", Value: 20, Source: "专精 · 1阶"},
		{Label: "攻击的伤害上限", Unit: "pct", Value: 35, Source: "专精 · 2阶"},
		{Label: "能力的伤害上限", Unit: "pct", Value: 15, Source: "专精 · 3阶"},
	})
	byLabel := map[string]EffectTotal{}
	for _, total := range totals {
		byLabel[total.Label] = total
	}
	if got := byLabel["普通攻击伤害上限"]; got.Value != 65 || len(got.Sources) != 3 {
		t.Fatalf("普通攻击上限未合并: %+v", got)
	}
	if got := byLabel["能力伤害上限"]; got.Value != 35 || len(got.Sources) != 2 {
		t.Fatalf("能力上限未合并: %+v", got)
	}
	if got := byLabel["奥义伤害上限"]; got.Value != 20 || len(got.Sources) != 1 {
		t.Fatalf("奥义上限未合并: %+v", got)
	}
	if _, exists := byLabel["伤害上限"]; exists {
		t.Fatal("全局伤害上限不应保留为无法与三类上限合并的独立行")
	}
}

func TestCalculateLoadoutFinalStatsUsesSafeUnconditionalSources(t *testing.T) {
	input := loadoutPanelInputs{
		CharacterHP:       3156,
		CharacterATK:      666,
		CharacterCritRate: 5,
		CharacterStun:     8,
		WeaponHP:          1099,
		WeaponATK:         9283,
		WeaponCritRate:    3,
		WeaponStun:        2,
		Bonuses: []TraitBonus{
			{Name: "体力", Components: []BonusComponent{{Label: "最大HP", Unit: "flat", Value: 10000, Additive: true}}},
			{Name: "终极钳蟹因子", Components: []BonusComponent{{Label: "最大HP", Unit: "flat", Value: 8000, Additive: true}}},
			{Name: "攻击力", Components: []BonusComponent{{Label: "攻击力", Unit: "flat", Value: 2000, Additive: true}}},
			{Name: "钳蟹的共鸣", Components: []BonusComponent{{Label: "攻击力", Unit: "flat", Value: 1000, Additive: true}}},
			{Name: "暴击率", Components: []BonusComponent{{Label: "暴击率", Unit: "pct", Value: 50, Additive: true}}},
			{Name: "昏厥", Components: []BonusComponent{{Label: "昏厥值", Unit: "flat", Value: 10, Additive: true}}},
			{Name: "伤害上限", Components: []BonusComponent{
				{Label: "攻击和攻击的伤害上限", Unit: "pct", Value: 250, Additive: true},
				{Label: "能力的伤害上限", Unit: "pct", Value: 250, Additive: true},
				{Label: "奥义的伤害上限", Unit: "pct", Value: 250, Additive: true},
			}},
			{Name: "金刚", Components: []BonusComponent{{Label: "最大HP", Unit: "pct", Value: 80, Additive: true}}},
			{Name: "暴君", Components: []BonusComponent{
				{Label: "攻击力", Unit: "pct", Value: 50, Additive: true},
				{Label: "最大HP", Unit: "pct", Value: -20, Additive: true},
			}},
			{Name: "钳蟹的报恩", Components: []BonusComponent{{Label: "最大HP", Unit: "pct", Value: 20, Additive: true}}},
			{Name: "刀上舞", Components: []BonusComponent{
				{Label: "攻击力", Unit: "pct", Value: 30, Additive: true},
				{Label: "伤害上限", Unit: "pct", Value: 30, Additive: true},
			}},
			{Name: "穷寇心", Components: []BonusComponent{{Label: "攻击力", Unit: "pct", Value: 20, Additive: true}}},
			// 快速蓄力会被现有文本聚合器概括成“攻击力”，但它只影响蓄力攻击，
			// 不得污染人物面板攻击力。
			{Name: "快速蓄力", Components: []BonusComponent{{Label: "攻击力", Unit: "pct", Value: 14, Additive: true}}},
		},
		Mastery: []LoadoutPanelBonus{
			{Label: "最大HP", Unit: "flat", Value: 15000, Source: "专精"},
			{Label: "攻击力", Unit: "flat", Value: 500, Source: "专精"},
			{Label: "攻击力", Unit: "pct", Value: 10, Source: "专精"},
			{Label: "暴击率", Unit: "pct", Value: 25, Source: "专精"},
			{Label: "昏厥值", Unit: "flat", Value: 0.4, Source: "专精"},
			{Label: "伤害上限", Unit: "pct", Value: 30, Source: "专精"},
		},
		OverLimit: []LoadoutOverLimitBonus{
			{Name: "昏厥值", Value: 4, Unit: "flat"},
			{Name: "普通攻击伤害上限", Value: 20, Unit: "pct"},
			{Name: "能力伤害上限", Value: 20, Unit: "pct"},
		},
	}

	got := calculateLoadoutFinalStats(input)
	// HP: trunc((3156 + 1099 + 10000 + 8000 + 15000) * 1.8 * 0.8 * 1.2)
	if got.HP != 64376 {
		t.Fatalf("HP=%d, want 64376: %+v", got.HP, got)
	}
	// ATK: trunc((666 + 9283 + 2000 + 1000 + 500) * 1.10 * 1.50 * 1.30 * 1.20)
	if got.Attack != 34617 {
		t.Fatalf("Attack=%d, want 34617: %+v", got.Attack, got)
	}
	if got.CritRate != 83 || math.Abs(got.StunPower-24.4) > 1e-9 {
		t.Fatalf("crit/stun wrong: %+v", got)
	}
	if got.DamageCap != 310 || got.NormalDamageCap != 330 || got.AbilityDamageCap != 330 || got.SkyboundDamageCap != 310 {
		t.Fatalf("damage caps wrong: %+v", got)
	}
	if got.Scope == "" || got.Mode == "" {
		t.Fatalf("final stats must describe their calculation scope: %+v", got)
	}
}

func TestLoadoutMasteryPanelBonusesApplyVerifiedFactorSynergies(t *testing.T) {
	// 伊欧的 R3/EX 解包节点分别提供攻击、基础能力、防御/支援三类联动。
	// 这些是普通子词条，不依赖 2/3 阶主方向；每选中一次就应用一次。
	bonuses, err := loadoutMasteryPanelBonuses("PL0400", []string{
		"DE9B3B33", // R3: 每个攻击类主因子 +10%，至多5个
		"24D9FB9F", // EX: 同上，可与 R3 叠加
		"E03C3AD2", // R3: 每个基础能力主因子伤害上限 +20%，至多5个
		"EEE3407D", // EX: 同上，可与 R3 叠加
		"DF6E655E", // R3: 每个防御/支援主因子最大HP +10000，至多4个
	}, loadoutFactorCategoryCounts{Basic: 8, Attack: 3, DefenseSupport: 6})
	if err != nil {
		t.Fatal(err)
	}
	totals := map[string]float64{}
	for _, bonus := range bonuses {
		totals[bonus.Unit+"|"+bonus.Label] += bonus.Value
	}
	if totals["pct|攻击力"] != 60 {
		t.Fatalf("攻击类因子联动=%g，期望两次 3×10%%=60%%: %+v", totals["pct|攻击力"], bonuses)
	}
	if totals["pct|伤害上限"] != 200 {
		t.Fatalf("基础能力因子联动=%g，期望两次 5×20%%=200%%: %+v", totals["pct|伤害上限"], bonuses)
	}
	if totals["flat|最大HP"] != 40000 {
		t.Fatalf("防御/支援因子联动=%g，期望 4×10000=40000: %+v", totals["flat|最大HP"], bonuses)
	}
}

func TestLoadoutPrimaryFactorCategoryCountsUseOnlyPrimaryTraits(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	counts := loadoutPrimaryFactorCategoryCounts(cat, []uint32{
		0xF372F096, // 体力：基础能力
		0xDC584F60, // 伤害上限：攻击类
		0xE0ABFDFE, // 守护：防御类
		0xB5FF9FD3, // 高扬：支援类
	})
	if counts.Basic != 1 || counts.Attack != 1 || counts.DefenseSupport != 2 {
		t.Fatalf("主因子分类计数错误: %+v", counts)
	}
}

func TestCalculateLoadoutFinalStatsPreservesRealZeroAndFractionalStun(t *testing.T) {
	got := calculateLoadoutFinalStats(loadoutPanelInputs{CharacterStun: 8, Mastery: []LoadoutPanelBonus{{Label: "昏厥值", Unit: "flat", Value: 0.4}}})
	if got.HP != 0 || got.Attack != 0 || got.CritRate != 0 || math.Abs(got.StunPower-8.4) > 1e-9 || got.DamageCap != 0 {
		t.Fatalf("zero/fractional values were distorted: %+v", got)
	}
}

func TestCalculateLoadoutFinalStatsTruncatesPositiveHPAndAttackLikeRuntime(t *testing.T) {
	got := calculateLoadoutFinalStats(loadoutPanelInputs{
		CharacterHP:  101,
		CharacterATK: 101,
		Mastery: []LoadoutPanelBonus{
			{Label: "最大HP", Unit: "pct", Value: 0.5},
			{Label: "攻击力", Unit: "pct", Value: 0.5},
		},
	})
	if got.HP != 101 || got.Attack != 101 {
		t.Fatalf("2.0.2 HP/ATK 转整数使用向零截断，不是 ceil/round: %+v", got)
	}
}

func TestCalculateLoadoutFinalStatsUsesRuntimeFloat32BeforeTruncation(t *testing.T) {
	// Binary64 evaluates 25 * 1.16 just below 29 and would truncate to 28.
	// The observed VCVTSI2SS/VMULSS/VCVTTSS2SI path evaluates it to float32 29.
	got := calculateLoadoutFinalStats(loadoutPanelInputs{
		CharacterHP:  25,
		CharacterATK: 25,
		Mastery: []LoadoutPanelBonus{
			{Label: "最大HP", Unit: "pct", Value: 16},
			{Label: "攻击力", Unit: "pct", Value: 16},
		},
	})
	if got.HP != 29 || got.Attack != 29 {
		t.Fatalf("HP/ATK 聚合必须保留实机 scalar float32 精度: %+v", got)
	}
}

func TestUnverifiedFinalStatsCarryRuntimeFormulaWarning(t *testing.T) {
	got := calculateLoadoutFinalStats(loadoutPanelInputs{
		CharacterHP:  100,
		CharacterATK: 200,
		Warnings:     []string{"保留已有诊断"},
	})
	if got.FormulaVerified {
		t.Fatal("最终 HP/攻击百分比来源尚未闭环时不得标记为已验证")
	}
	if !warningsContain(got.Warnings, "百分比") || !warningsContain(got.Warnings, "估算") ||
		!warningsContain(got.Warnings, "向零截断") || !warningsContain(got.Warnings, "保留已有诊断") {
		t.Fatalf("未验证结果必须同时说明估算边界、已验证取整并保留原诊断: %v", got.Warnings)
	}
}

func TestCalculateLoadoutFinalStatsAppliesOnlyDeterminableWeaponAndFactorCaps(t *testing.T) {
	terminus := TraitBonus{TraitID: "SKILL_143_10", Name: "浩劫", MaxHPCondition: 150000, Components: []BonusComponent{
		{Label: "攻击力", Unit: "pct", Value: 60, Additive: true},
		{Label: "普通攻击伤害上限", Unit: "pct", Value: 430, Additive: true},
		{Label: "能力伤害上限", Unit: "pct", Value: 430, Additive: true},
		{Label: "奥义伤害上限", Unit: "pct", Value: 430, Additive: true},
		{Label: "最大HP阈值", Unit: "flat", Value: 150000, Additive: false},
	}}
	mageSavvy := TraitBonus{TraitID: "SKILL_117_01", Name: "魔法师的伶俐", Components: []BonusComponent{{Label: "伤害上限", Unit: "pct", Value: 50, Additive: true}}}

	below := calculateLoadoutFinalStats(loadoutPanelInputs{CharacterHP: 100000, CharacterATK: 1000, Bonuses: []TraitBonus{terminus, mageSavvy}})
	if below.Attack != 1600 || below.DamageCap != 480 || below.NormalDamageCap != 480 || below.AbilityDamageCap != 480 || below.SkyboundDamageCap != 480 {
		t.Fatalf("determinable HP threshold/global cap were not applied: %+v", below)
	}
	above := calculateLoadoutFinalStats(loadoutPanelInputs{CharacterHP: 150001, CharacterATK: 1000, Bonuses: []TraitBonus{terminus, mageSavvy}})
	if above.Attack != 1000 || above.DamageCap != 50 || above.NormalDamageCap != 50 || above.AbilityDamageCap != 50 || above.SkyboundDamageCap != 50 {
		t.Fatalf("HP-gated terminus effect must be excluded above 150000 while Mage's Savvy remains global: %+v", above)
	}
}

func TestCelestialTerraAliasesApplyAuditedHPTradeoffAndGlobalCap(t *testing.T) {
	for _, hash := range []uint32{0x9232DC17, 0xD29CD8E0} {
		id := hashText(hash)
		bonuses := simulateTraits([]struct {
			hash  uint32
			level int
		}{{hash: hash, level: 15}}, map[uint32]string{hash: id})
		if len(bonuses) != 1 || bonuses[0].Name != "天星之界" || !strings.Contains(bonuses[0].Effect, "最大HP-30%") || !strings.Contains(bonuses[0].Effect, "伤害上限+70%") {
			t.Fatalf("天星之界 alias %s 定义未恢复: %+v", id, bonuses)
		}
		got := calculateLoadoutFinalStats(loadoutPanelInputs{CharacterHP: 10000, Bonuses: bonuses})
		if got.HP != 7000 || got.DamageCap != 70 || got.NormalDamageCap != 70 || got.AbilityDamageCap != 70 || got.SkyboundDamageCap != 70 {
			t.Fatalf("天星之界 alias %s 最终属性错误: %+v", id, got)
		}
	}
}

func TestSpiritEdgesFuryCombatConditionIsExcludedFromStaticPanel(t *testing.T) {
	definition := loadTraitValues()["SKILL_170_01"]
	if definition == nil {
		t.Fatal("SKILL_170_01 definition missing")
	}
	effect, components := renderTraitEffect(definition, 15)
	if !strings.Contains(effect, "攻击力+30%") || !strings.Contains(effect, "昏厥值+30%") || !strings.Contains(effect, "伤害上限+15%") {
		t.Fatalf("剑圣的闪刃 Lv15 表定义错误: %q %+v", effect, components)
	}
	got := calculateLoadoutFinalStats(loadoutPanelInputs{
		CharacterATK:  1000,
		CharacterStun: 10,
		Bonuses:       []TraitBonus{{TraitID: "SKILL_170_01", Name: "剑圣的闪刃", Components: components}},
		Mastery:       []LoadoutPanelBonus{{Label: "昏厥值", Unit: "flat", Value: 2}},
		OverLimit:     []LoadoutOverLimitBonus{{Name: "昏厥值", Unit: "flat", Value: 3}},
	})
	if got.Attack != 1000 || math.Abs(got.StunPower-15) > 1e-9 || got.DamageCap != 0 || got.NormalDamageCap != 0 || got.AbilityDamageCap != 0 || got.SkyboundDamageCap != 0 {
		t.Fatalf("剑神/分身召唤中的战斗条件加成不应污染默认静态面板: %+v", got)
	}
}

func TestCelestialLumenAliasesStayVisibleButOutOfStaticPanel(t *testing.T) {
	for _, hash := range []uint32{0x20492635, 0xA7726190} {
		id := hashText(hash)
		bonuses := simulateTraits([]struct {
			hash  uint32
			level int
		}{{hash: hash, level: 15}}, map[uint32]string{hash: id})
		if len(bonuses) != 1 || bonuses[0].Name != "天星之煌" || !strings.Contains(bonuses[0].Effect, "HP不低于75%") || !strings.Contains(bonuses[0].Effect, "攻击力+20%") || !strings.Contains(bonuses[0].Effect, "伤害上限+70%") {
			t.Fatalf("天星之煌 alias %s 条件效果未恢复: %+v", id, bonuses)
		}
		got := calculateLoadoutFinalStats(loadoutPanelInputs{CharacterHP: 10000, CharacterATK: 1000, Bonuses: bonuses})
		if got.HP != 10000 || got.Attack != 1000 || got.DamageCap != 0 {
			t.Fatalf("当前HP比例未知时，天星之煌不应进入默认静态面板: %+v", got)
		}
	}
}
