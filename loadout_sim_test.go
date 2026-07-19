package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func simulateSingleSummonMainSkill(t *testing.T, hash uint32) TraitBonus {
	t.Helper()
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	bonuses := simulateTraits([]struct {
		hash  uint32
		level int
	}{{hash: hash, level: 15}}, traitHashMapWithRawKeys(catalog))
	if len(bonuses) != 1 {
		t.Fatalf("summon main skill %08X produced %d bonuses; missing definitions must return one visible diagnostic bonus", hash, len(bonuses))
	}
	return bonuses[0]
}

func TestSummonMainSkillMissingDefinitionProducesVisibleWarning(t *testing.T) {
	const hash = uint32(0x9300FADB)
	bonus := simulateSingleSummonMainSkill(t, hash)
	stats := calculateLoadoutFinalStats(loadoutPanelInputs{Bonuses: []TraitBonus{bonus}})
	warnings := strings.Join(stats.Warnings, "\n")
	if stats.FormulaVerified || !strings.Contains(warnings, "9300FADB") || !strings.Contains(warnings, "未计入") {
		t.Fatalf("missing summon warning did not reach final stats or formula became verified: %+v", stats)
	}
}

func TestSummonMainSkillBlankNameAndFormatProducesVisibleWarning(t *testing.T) {
	const hash = uint32(0xF26BAEA5)
	bonus := simulateSingleSummonMainSkill(t, hash)
	stats := calculateLoadoutFinalStats(loadoutPanelInputs{Bonuses: []TraitBonus{bonus}})
	warnings := strings.Join(stats.Warnings, "\n")
	if stats.FormulaVerified || !strings.Contains(warnings, "F26BAEA5") || !strings.Contains(warnings, "未计入") {
		t.Fatalf("blank summon warning did not reach final stats or formula became verified: %+v", stats)
	}
}

func TestSummonMainSkillCatalogCoverageIsExplicit(t *testing.T) {
	var payload summonSkillFile
	if err := json.Unmarshal(summonSkillsJSON, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Skills) != 230 {
		t.Fatalf("summon main-skill denominator = %d, want audited 230", len(payload.Skills))
	}
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	hashToID := traitHashMapWithRawKeys(catalog)
	resolved, warned := 0, 0
	for _, skill := range payload.Skills {
		hash, err := ParseHashHex(skill.Hash)
		if err != nil {
			t.Errorf("invalid summon skill hash %q: %v", skill.Hash, err)
			continue
		}
		bonuses := simulateTraits([]struct {
			hash  uint32
			level int
		}{{hash: hash, level: skill.MaxLevel}}, hashToID)
		if len(bonuses) != 1 {
			t.Errorf("summon main skill %s returned %d bonuses instead of an effect or diagnostic", skill.Hash, len(bonuses))
			continue
		}
		stats := calculateLoadoutFinalStats(loadoutPanelInputs{Bonuses: bonuses})
		if strings.Contains(strings.Join(stats.Warnings, "\n"), strings.TrimPrefix(strings.ToUpper(skill.Hash), "0X")) {
			warned++
		} else if bonuses[0].Effect != "" {
			resolved++
		} else {
			t.Errorf("summon main skill %s returned neither an effect nor a warning: %+v", skill.Hash, bonuses[0])
		}
	}
	if resolved != 170 || warned != 60 {
		t.Fatalf("summon main-skill coverage = resolved %d, warned %d; want 170 authoritative effects and 60 explicit unresolved warnings out of 230", resolved, warned)
	}
	t.Logf("summon main-skill coverage: total=%d authoritativeEffects=%d explicitWarnings=%d", len(payload.Skills), resolved, warned)
}

func TestCatastropheHashUsesAuthoritativeSkill14300(t *testing.T) {
	const hash = uint32(0x40223C28)
	shared, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	sharedTrait := shared.LookupTraitByHash(hash)
	if sharedTrait == nil || sharedTrait.InternalID != "SKILL_143_00" {
		t.Fatalf("shared trait 40223C28 = %+v; local ids.txt and generated weapon traitIds require SKILL_143_00", sharedTrait)
	}
	wrightstones, err := LoadWrightstoneCatalog()
	if err != nil {
		t.Fatal(err)
	}
	wrightstoneTrait := wrightstones.LookupTraitByHash(hash)
	if wrightstoneTrait == nil || wrightstoneTrait.InternalID != "SKILL_143_00" {
		t.Fatalf("wrightstone trait 40223C28 = %+v; local ids.txt requires SKILL_143_00", wrightstoneTrait)
	}
	traitID := buildTraitHashToID(shared)[hash]
	definition := loadTraitValues()[traitID]
	if traitID != "SKILL_143_00" || definition == nil || definition.Name != "浩劫" || definition.Format == "" {
		t.Fatalf("Catastrophe simulation mapping = id %q definition %+v", traitID, definition)
	}
}

func TestDLC2TerminusSkillUsesIndependentAuditedLevel30Definition(t *testing.T) {
	values := loadTraitValues()
	definition := values["SKILL_143_10"]
	if definition == nil || definition.MaxLevel != 35 || definition == values["SKILL_143_00"] {
		t.Fatalf("SKILL_143_10 must be an independent max-level-35 definition: %#v", definition)
	}
	effect, components := renderTraitEffect(definition, 30)
	if effect == "" || len(components) != 5 {
		t.Fatalf("SKILL_143_10 Lv30 did not render five audited fields: effect=%q components=%+v", effect, components)
	}
	want := map[string]float64{"攻击力": 60, "普通攻击伤害上限": 430, "能力伤害上限": 430, "奥义伤害上限": 430}
	for _, component := range components[:4] {
		if got, ok := want[component.Label]; !ok || got != component.Value {
			t.Fatalf("unexpected SKILL_143_10 component: %+v; want=%v", component, want)
		}
	}
	if components[4].Value != 150000 || components[4].Additive {
		t.Fatalf("SKILL_143_10 HP threshold must remain a non-additive condition: %+v", components[4])
	}
	for _, check := range []struct {
		level                    int
		attack, cap, hpThreshold float64
	}{{25, 50, 100, 45000}, {35, 70, 500, 200000}} {
		_, edge := renderTraitEffect(definition, check.level)
		if len(edge) != 5 || edge[0].Value != check.attack || edge[1].Value != check.cap || edge[2].Value != check.cap || edge[3].Value != check.cap || edge[4].Value != check.hpThreshold {
			t.Fatalf("SKILL_143_10 Lv%d does not match local skill_status.tbl: %+v", check.level, edge)
		}
	}
}

func TestMageSavvyUsesVerifiedIoTraitIDAndLevel15GlobalCap(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	trait := catalog.LookupTraitByHash(0x11AAE5F5)
	if trait == nil || trait.InternalID != "SKILL_117_01" || trait.DisplayName != "Mage's Savvy" {
		t.Fatalf("0x11AAE5F5 is not the verified Io Mage's Savvy trait: %+v", trait)
	}
	definition := loadTraitValues()[trait.InternalID]
	if definition == nil || definition.Name != "魔法师的伶俐" || definition.MaxLevel != 15 {
		t.Fatalf("Mage's Savvy definition is not localized/audited: %+v", definition)
	}
	effect, components := renderTraitEffect(definition, 15)
	if effect != "伤害上限+50%" || len(components) != 1 || components[0].Label != "伤害上限" || components[0].Unit != "pct" || components[0].Value != 50 || !components[0].Additive {
		t.Fatalf("Mage's Savvy Lv15 must be a global +50%% damage cap: effect=%q components=%+v", effect, components)
	}
}

func TestAggregateTraitEffectsMergesEquivalentAdditiveComponents(t *testing.T) {
	globalCap := &traitValueDef{Format: "伤害上限+{0}%", Placeholders: []traitPlaceholder{{Ph: 0, Unit: "pct", Values: []float64{30}}}}
	attackCap := &traitValueDef{Format: "攻击和攻击的伤害上限+{0}%\n能力的伤害上限+{1}%，奥义的伤害上限+{2}%\n冷却时间-{3:.1f}%", Placeholders: []traitPlaceholder{
		{Ph: 0, Unit: "pct", Values: []float64{220}},
		{Ph: 1, Unit: "pct", Values: []float64{220}},
		{Ph: 2, Unit: "pct", Values: []float64{220}},
		{Ph: 3, Unit: "pct", Values: []float64{1.5}},
	}}
	alphaCap := &traitValueDef{Format: "攻击和攻击的伤害上限+{0}%", Placeholders: []traitPlaceholder{{Ph: 0, Unit: "pct", Values: []float64{25}}}}
	betaCap := &traitValueDef{Format: "能力的伤害上限+{0}%", Placeholders: []traitPlaceholder{{Ph: 0, Unit: "pct", Values: []float64{35}}}}
	quickCooldown := &traitValueDef{Format: "冷却时间-{0:.1f}%", Placeholders: []traitPlaceholder{{Ph: 0, Unit: "pct", Values: []float64{10}}}}
	thresholdOnly := &traitValueDef{Format: "基础攻击力高于{0}时触发", Placeholders: []traitPlaceholder{{Ph: 0, Unit: "flat", Values: []float64{20000}}}}

	makeBonus := func(name string, def *traitValueDef) TraitBonus {
		effect, components := renderTraitEffect(def, 1)
		return TraitBonus{Name: name, CatLabel: "攻击类", Effect: effect, Components: components}
	}
	totals := aggregateTraitEffects([]TraitBonus{
		makeBonus("γ秘纹", globalCap),
		makeBonus("伤害上限", attackCap),
		makeBonus("α秘纹", alphaCap),
		makeBonus("β秘纹", betaCap),
		makeBonus("迅捷能力", quickCooldown),
		makeBonus("狂战士", thresholdOnly),
	})

	if len(totals) != 4 {
		t.Fatalf("expected three cap totals and cooldown, got %+v", totals)
	}
	byLabel := map[string]EffectTotal{}
	for _, total := range totals {
		byLabel[total.Label] = total
	}
	if got := byLabel["普通攻击伤害上限"]; got.Value != 275 || len(got.Sources) != 3 {
		t.Fatalf("merged normal cap = %+v, want 275%% from three sources", got)
	}
	if got := byLabel["能力伤害上限"]; got.Value != 285 || len(got.Sources) != 3 {
		t.Fatalf("merged skill cap = %+v, want 285%% from three sources", got)
	}
	if got := byLabel["奥义伤害上限"]; got.Value != 250 || len(got.Sources) != 2 {
		t.Fatalf("merged SBA cap = %+v, want 250%% from two sources", got)
	}
	if got := byLabel["冷却时间"]; got.Value != -11.5 || len(got.Sources) != 2 {
		t.Fatalf("merged cooldown = %+v, want -11.5%% from two sources", got)
	}
	if _, exists := byLabel["基础攻击力高于"]; exists {
		t.Fatal("threshold placeholders must not be presented as additive totals")
	}
}

func TestConditionalDLCWeaponSkillsStayOutOfTotalsWithoutBattleState(t *testing.T) {
	tests := []struct {
		id      string
		details []string
	}{
		{"SKILL_311_00", []string{"效果最小时攻击力+40%", "效果最大时攻击力+50%", "最大HP不低于200000"}},
		{"SKILL_312_00", []string{"伤害上限+280%", "疾天Ⅴ", "连锁计数每增加20%"}},
		{"SKILL_314_00", []string{"伤害上限+270%", "暴击率不低于200%"}},
		{"SKILL_315_00", []string{"伤害上限+310%", "红天Ⅴ", "效果持续时间60秒"}},
	}

	values := loadTraitValues()
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			definition := values[tt.id]
			if definition == nil {
				t.Fatalf("missing definition %s", tt.id)
			}
			if definition.AggregationPolicy != traitAggregationDetailOnly {
				t.Fatalf("conditional definition policy = %q, want %q", definition.AggregationPolicy, traitAggregationDetailOnly)
			}
			effect, components := renderTraitEffect(definition, 15)
			for _, detail := range tt.details {
				if !strings.Contains(effect, detail) {
					t.Errorf("detail %q missing from rendered effect %q", detail, effect)
				}
			}
			for _, component := range components {
				if component.AggregationPolicy != traitAggregationDetailOnly || component.Additive {
					t.Errorf("unknown battle-state component must remain detail-only: %+v", component)
				}
			}
			bonus := TraitBonus{TraitID: tt.id, Name: definition.Name, CatLabel: definition.CatLabel, Effect: effect, Components: components}
			if totals := aggregateTraitEffects([]TraitBonus{bonus}); len(totals) != 0 {
				t.Fatalf("conditional skill leaked into static totals: %+v", totals)
			}
		})
	}
}

func TestRenderTraitEffectSeparatesAdjacentPlaceholderLabels(t *testing.T) {
	def := &traitValueDef{
		Format: "治疗药水+{0} 群疗药水+{1}\n强效药水+{2} 复活药水+{3}",
		Placeholders: []traitPlaceholder{
			{Ph: 0, Unit: "flat", Values: []float64{4}},
			{Ph: 1, Unit: "flat", Values: []float64{5}},
			{Ph: 2, Unit: "flat", Values: []float64{5}},
			{Ph: 3, Unit: "flat", Values: []float64{1}},
		},
	}
	_, components := renderTraitEffect(def, 1)
	want := []string{"治疗药水", "群疗药水", "强效药水", "复活药水"}
	for i := range want {
		if components[i].Label != want[i] {
			t.Fatalf("component %d label=%q, want %q", i, components[i].Label, want[i])
		}
	}
}

// 模拟器聚合规则：同名词条等级相加→封顶→查一次表。锚点用游戏原表实测值。
func TestSimulateTraitsAnchors(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	hashToID := buildTraitHashToID(cat)
	// ATK = SKILL_000_00 = 0x50079A1C（flat，Lv30=? Lv50=2000）
	const atk = 0x50079A1C
	if hashToID[atk] != "SKILL_000_00" {
		t.Fatalf("ATK hash 未映射到 SKILL_000_00，得 %q", hashToID[atk])
	}

	mk := func(hash uint32, lv int) struct {
		hash  uint32
		level int
	} {
		return struct {
			hash  uint32
			level int
		}{hash, lv}
	}

	// 两颗 ATK 各 15 → 合并 30（未封顶，max50）
	got := simulateTraits([]struct {
		hash  uint32
		level int
	}{mk(atk, 15), mk(atk, 15)}, hashToID)
	if len(got) != 1 {
		t.Fatalf("期望 1 行，得 %d", len(got))
	}
	if got[0].Level != 30 || got[0].Capped {
		t.Errorf("合并等级=%d capped=%v，期望 30/false", got[0].Level, got[0].Capped)
	}
	if got[0].Name != "攻击力" {
		t.Errorf("名字=%q，期望 攻击力", got[0].Name)
	}
	t.Logf("两颗ATK15 → %s（Lv%d）", got[0].Effect, got[0].Level)

	// 封顶：四颗 ATK 各 15 → 合并 60 → 封顶 50，值应为 2000
	got = simulateTraits([]struct {
		hash  uint32
		level int
	}{mk(atk, 15), mk(atk, 15), mk(atk, 15), mk(atk, 15)}, hashToID)
	if got[0].Level != 50 || !got[0].Capped || got[0].RawLevel != 60 {
		t.Errorf("封顶结果 lv=%d raw=%d capped=%v，期望 50/60/true", got[0].Level, got[0].RawLevel, got[0].Capped)
	}
	if len(got[0].Components) != 1 || got[0].Components[0].Value != 2000 || got[0].Components[0].Unit != "flat" {
		t.Errorf("ATK Lv50 分量=%+v，期望 flat 2000", got[0].Components)
	}
	t.Logf("四颗ATK15 → %s（Lv%d/%d，raw%d）", got[0].Effect, got[0].Level, got[0].MaxLevel, got[0].RawLevel)

	// 暴击率 SKILL_003_00 = 0x8D78A19B（pct，Lv45=50）
	const crit = 0x8D78A19B
	got = simulateTraits([]struct {
		hash  uint32
		level int
	}{mk(crit, 45)}, hashToID)
	if len(got) != 1 || got[0].Components[0].Unit != "pct" || got[0].Components[0].Value != 50 {
		t.Errorf("暴击率 Lv45 = %+v，期望 pct 50", got)
	}
	t.Logf("暴击率45 → %s", got[0].Effect)
}

// LoadoutSimulate 端到端：对真实存档某套配装的因子跑模拟。
func TestLoadoutSimulateReal(t *testing.T) {
	if !haveSave(testLoadoutSave) {
		t.Skipf("无测试存档")
	}
	app := &App{}
	groups, err := app.LoadoutList(testLoadoutSave)
	if err != nil {
		t.Fatal(err)
	}
	// 找一套有因子的配装，取它的因子 SlotID
	var slotIDs []uint32
	for _, g := range groups {
		for _, lo := range g.Loadouts {
			if len(lo.Sigils) >= 6 {
				for _, s := range lo.Sigils {
					if s.SlotID != 0 {
						slotIDs = append(slotIDs, s.SlotID)
					}
				}
				break
			}
		}
		if len(slotIDs) > 0 {
			break
		}
	}
	if len(slotIDs) == 0 {
		t.Skip("没有可模拟的配装")
	}
	bonuses, err := app.LoadoutSimulate(testLoadoutSave, slotIDs)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("对 %d 颗因子模拟出 %d 条词条加成:", len(slotIDs), len(bonuses))
	for _, b := range bonuses {
		cap := ""
		if b.Capped {
			cap = "(封顶)"
		}
		t.Logf("  [%s] %s Lv%d%s → %s", b.CatLabel, b.Name, b.Level, cap, b.Effect)
	}
	if len(bonuses) == 0 {
		t.Error("模拟结果为空——可能是主/副词条读取或 join 失败")
	}
	merged, err := app.LoadoutSimulateDraft(testLoadoutSave, slotIDs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(merged.Totals) == 0 {
		t.Fatal("真实配装没有生成任何可加总效果")
	}
	mergedSourceFound := false
	for _, total := range merged.Totals {
		if len(total.Sources) >= 2 {
			mergedSourceFound = true
		}
		t.Logf("  合并 [%s] %s = %g %s（%v）", total.CatLabel, total.Label, total.Value, total.Unit, total.Sources)
	}
	if !mergedSourceFound {
		t.Fatal("真实配装没有任何同类效果被跨词条合并")
	}
}

func TestLoadoutSimulateDraftIncludesConstructedFactorWithoutWriting(t *testing.T) {
	if !haveSave(testLoadoutSave) {
		t.Skipf("无测试存档")
	}
	item := naturalConstructedSigilItem(t)
	result, err := (&App{}).LoadoutSimulateDraft(testLoadoutSave, nil, []LoadoutConstructedSigil{{Index: 0, Item: item}})
	if err != nil {
		t.Fatal(err)
	}
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	sigil, err := cat.RequireSigil(item.SigilID)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{sigil.PrimaryTraitID: false}
	if item.SecondaryTraitID != "" {
		want[item.SecondaryTraitID] = false
	}
	for _, bonus := range result.Bonuses {
		if _, ok := want[bonus.TraitID]; ok {
			want[bonus.TraitID] = true
		}
	}
	for traitID, found := range want {
		if !found {
			t.Fatalf("构造草稿词条 %s 未进入模拟结果: %+v", traitID, result.Bonuses)
		}
	}
}
