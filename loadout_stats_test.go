package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const testStatsSave = `D:\gbf\Saved\SaveGames\SaveData2.dat`

const testIoHash = "4D0A60C3"

func requireStatsSave(t *testing.T) {
	t.Helper()
	if !haveSave(testStatsSave) {
		t.Skipf("真实 SaveData2 样本不存在: %s", testStatsSave)
	}
}

func copyStatsSave(t *testing.T) string {
	t.Helper()
	requireStatsSave(t)
	t.Setenv("APPDATA", filepath.Join(t.TempDir(), "appdata"))
	data, err := os.ReadFile(testStatsSave)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "SaveData2.dat")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func findSummonBySlot(summons []LoadoutSummon, slotID uint32) (LoadoutSummon, bool) {
	for _, summon := range summons {
		if summon.SlotID == slotID {
			return summon, true
		}
	}
	return LoadoutSummon{}, false
}

func TestLoadoutStatContextReadsRealIoBaseStatsAndEquippedSummons(t *testing.T) {
	requireStatsSave(t)
	ctx, err := (&App{}).LoadoutStatContext(testStatsSave, testIoHash)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.CharaHash != testIoHash || ctx.CharaUnitID != 10004 {
		t.Fatalf("角色定位错误: hash=%s unit=%d", ctx.CharaHash, ctx.CharaUnitID)
	}
	if ctx.Level != 100 || ctx.BaseHP != 3156 || ctx.BaseATK != 666 || ctx.BaseStun != 8 || ctx.BaseCritRate != 5 {
		t.Fatalf("伊欧基础字段错误: level=%d baseHP=%d baseATK=%d stun=%g crit=%g", ctx.Level, ctx.BaseHP, ctx.BaseATK, ctx.BaseStun, ctx.BaseCritRate)
	}
	wantSlots := []uint32{35, 43, 52, 95}
	if !reflect.DeepEqual(ctx.EquippedSummonSlotIDs, wantSlots) {
		t.Fatalf("1451 装备顺序=%v，期望 %v", ctx.EquippedSummonSlotIDs, wantSlots)
	}
	if len(ctx.EquippedSummons) != 4 {
		t.Fatalf("已装备召唤石数=%d，期望4: warnings=%v", len(ctx.EquippedSummons), ctx.Warnings)
	}

	expected := map[uint32]struct {
		typeHash, name, mainHash, mainName, subHash, subName, unit string
	}{
		35: {"F2BE819E", "火龙布雷扎莱克 · 传说 · 伤害", "318D12E9", "迅捷能力", "A66241C9", "普通攻击伤害上限（低·最高50%）", "pct"},
		43: {"3BAD1134", "地龙杜雷贝尔 · 传说 · 伤害", "E69A4694", "豪胆", "A66241C9", "普通攻击伤害上限（低·最高50%）", "pct"},
		52: {"52B14B66", "胡拉坎 · 传说 · 伤害", "F26BAEA5", "分歧", "2270BC40", "HP回复上限（低·最高50%）", "pct"},
		95: {"B960D2BB", "冰龙威利努斯 · 传说 · 伤害", "48A95B8D", "金刚", "A66241C9", "普通攻击伤害上限（低·最高50%）", "pct"},
	}
	for _, slotID := range wantSlots {
		summon, ok := findSummonBySlot(ctx.Summons, slotID)
		if !ok {
			t.Fatalf("全部召唤背包没有装备中的 SlotID %d", slotID)
		}
		want := expected[slotID]
		if summon.TypeHash != want.typeHash || summon.Name != want.name ||
			summon.MainTraitHash != want.mainHash || summon.MainTraitName != want.mainName ||
			summon.SubParamHash != want.subHash || summon.SubParamName != want.subName {
			t.Fatalf("SlotID %d 映射错误: %+v", slotID, summon)
		}
		if summon.MainTraitLevel != 15 || summon.SubParamLevel != 9 || summon.SubParamValue != 50 || summon.SubParamUnit != want.unit || summon.Rank != 2 {
			t.Fatalf("SlotID %d 等级/副参数错误: %+v", slotID, summon)
		}
	}
	for index, summon := range ctx.EquippedSummons {
		if summon.SlotID != wantSlots[index] {
			t.Fatalf("已装备第%d只 SlotID=%d，期望%d", index+1, summon.SlotID, wantSlots[index])
		}
	}
}

func TestLoadoutStatContextReadsAuditedIoOverLimitCurves(t *testing.T) {
	requireStatsSave(t)
	ctx, err := (&App{}).LoadoutStatContext(testStatsSave, testIoHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.OverLimit) != 4 {
		t.Fatalf("极限加成槽=%d，期望4: %+v", len(ctx.OverLimit), ctx.OverLimit)
	}
	values := map[string]float64{}
	counts := map[string]int{}
	for index, bonus := range ctx.OverLimit {
		if bonus.Index != index || bonus.Level != 10 {
			t.Fatalf("极限加成顺序/等级错误: %+v", bonus)
		}
		values[bonus.Name] += bonus.Value
		counts[bonus.Name]++
	}
	if counts["昏厥值"] != 2 || values["昏厥值"] != 4 {
		t.Fatalf("昏厥值极限加成=%g/%d槽，期望+4/2槽", values["昏厥值"], counts["昏厥值"])
	}
	if values["普通攻击伤害上限"] != 20 || values["能力伤害上限"] != 20 {
		t.Fatalf("伤害上限极限加成错误: %v", values)
	}
}

func TestLoadoutStatContextWarnsAndDoesNotInventDanglingSummon(t *testing.T) {
	path := copyStatsSave(t)
	save, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	equipped, ok := save.findUnitExact(1451, 0)
	if !ok {
		t.Fatal("找不到1451 Unit0")
	}
	const dangling = uint32(0x7FFFFFFE)
	if err := equipped.SetUint32At(1, dangling); err != nil {
		t.Fatal(err)
	}
	if err := save.FixChecksums(); err != nil {
		t.Fatal(err)
	}
	if err := save.Write(path); err != nil {
		t.Fatal(err)
	}

	ctx, err := (&App{}).LoadoutStatContext(path, testIoHash)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ctx.EquippedSummonSlotIDs, []uint32{35, dangling, 52, 95}) {
		t.Fatalf("悬空1451原值未保留: %v", ctx.EquippedSummonSlotIDs)
	}
	if len(ctx.EquippedSummons) != 3 {
		t.Fatalf("悬空槽不得伪造召唤石: %+v", ctx.EquippedSummons)
	}
	if _, found := findSummonBySlot(ctx.EquippedSummons, dangling); found {
		t.Fatal("悬空 SlotID 被伪造成召唤石")
	}
	if !warningsContain(ctx.Warnings, "悬空") || !warningsContain(ctx.Warnings, fmt.Sprint(dangling)) {
		t.Fatalf("悬空 join 未产生可诊断 warning: %v", ctx.Warnings)
	}
}

func TestLoadoutStatContextPreservesUnknownSummonHashWithoutInventingName(t *testing.T) {
	path := copyStatsSave(t)
	save, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	var typeEntry *unitEntry
	for _, entry := range save.findAllUnitsByType(1457) {
		if entry.UnitID == 34 && entry.ValueCnt == 1 { // SlotID 35 in the real fixture
			typeEntry = entry
			break
		}
	}
	if typeEntry == nil {
		t.Fatal("找不到 SlotID 35 对应的 1457 UnitID 34")
	}
	typeEntry.SetUint32(0xDEADBEEF)
	if err := save.FixChecksums(); err != nil {
		t.Fatal(err)
	}
	if err := save.Write(path); err != nil {
		t.Fatal(err)
	}

	ctx, err := (&App{}).LoadoutStatContext(path, testIoHash)
	if err != nil {
		t.Fatal(err)
	}
	summon, found := findSummonBySlot(ctx.Summons, 35)
	if !found {
		t.Fatal("真实未知召唤石实例不应从背包结果消失")
	}
	if summon.TypeHash != "DEADBEEF" || summon.Name != "" {
		t.Fatalf("未知类型必须保留原始 hash 且不得伪造名字: %+v", summon)
	}
	if !warningsContain(ctx.Warnings, "DEADBEEF") || !warningsContain(ctx.Warnings, "内置目录") {
		t.Fatalf("未知类型没有诊断 warning: %v", ctx.Warnings)
	}
}

func TestLoadoutStatContextRejectsInvalidOverLimitBitmask(t *testing.T) {
	path := copyStatsSave(t)
	save, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	level, ok := save.findUnitExact(1607, 10004000)
	if !ok {
		t.Fatal("找不到伊欧1607槽0")
	}
	level.SetUint32(3) // 非单 bit
	if err := save.FixChecksums(); err != nil {
		t.Fatal(err)
	}
	if err := save.Write(path); err != nil {
		t.Fatal(err)
	}
	_, err = (&App{}).LoadoutStatContext(path, testIoHash)
	if err == nil || !strings.Contains(err.Error(), "1607") || !strings.Contains(err.Error(), "单 bit") {
		t.Fatalf("非法1607位掩码应被拒绝，实际: %v", err)
	}
}

func warningsContain(warnings []string, part string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, part) {
			return true
		}
	}
	return false
}

func totalByLabel(totals []EffectTotal, label string) (EffectTotal, bool) {
	for _, total := range totals {
		if total.Label == label {
			return total, true
		}
	}
	return EffectTotal{}, false
}

func sourceContains(sources []string, want string) bool {
	for _, source := range sources {
		if source == want {
			return true
		}
	}
	return false
}

func TestSummonSubParamLabelCanonicalizesCharacterPanelNames(t *testing.T) {
	tests := map[string]string{
		"体力（高·最高10000）":     "最大HP",
		"昏厥（高·最高20）":        "昏厥值",
		"普通攻击伤害上限（低·最高50%）": "普通攻击伤害上限",
		"HP回复上限（低·最高50%）":   "HP回复上限",
	}
	for input, want := range tests {
		if got := summonSubParamLabel(input); got != want {
			t.Errorf("summonSubParamLabel(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestFactorBoostPreservesAuditedFixedLevelTraits(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	var fixedHash, attackHash uint32
	for _, trait := range cat.Traits {
		hash, parseErr := ParseHashHex(trait.Hash)
		if parseErr != nil {
			continue
		}
		switch trait.InternalID {
		case "SKILL_133_00":
			fixedHash = hash
		case "SKILL_000_00":
			attackHash = hash
		}
	}
	if fixedHash == 0 || attackHash == 0 {
		t.Fatalf("测试词条 hash 未收录: fixed=%08X attack=%08X", fixedHash, attackHash)
	}
	pairs := []struct {
		hash  uint32
		level int
	}{{fixedHash, 6}, {attackHash, 15}}
	applyFactorLevelBoost(pairs, 1, buildTraitHashToID(cat))
	if pairs[0].level != 6 || pairs[1].level != 16 {
		t.Fatalf("因子强化后的等级=%v，固定 Lv6 应保持、普通词条应升到 Lv16", pairs)
	}
	bonuses := simulateTraits(pairs, buildTraitHashToID(cat))
	foundFixed := false
	for _, bonus := range bonuses {
		if bonus.TraitID == "SKILL_133_00" {
			foundFixed = strings.Contains(bonus.Effect, "最大HP+5%") && strings.Contains(bonus.Effect, "回复量+10%")
		}
	}
	if !foundFixed {
		t.Fatalf("固定等级特殊因子被因子强化抹成零值: %+v", bonuses)
	}
}

func TestLoadoutSimulateBuildMergesRealSummonsAndOverLimit(t *testing.T) {
	requireStatsSave(t)
	result, err := (&App{}).LoadoutSimulateBuild(testStatsSave, testIoHash, 0, nil, nil, nil, []uint32{35, 43, 52, 95})
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		label string
		value float64
		unit  string
	}{
		{"普通攻击伤害上限", 170, "pct"},
		{"HP回复上限", 50, "pct"},
		{"能力伤害上限", 20, "pct"},
		{"昏厥值", 4, "flat"},
	}
	for _, check := range checks {
		total, ok := totalByLabel(result.Totals, check.label)
		if !ok || total.Value != check.value || total.Unit != check.unit {
			t.Fatalf("总计 %s=%+v，期望 %g %s", check.label, total, check.value, check.unit)
		}
		if !sourceContains(total.Sources, "极限加成") && check.label != "HP回复上限" {
			t.Fatalf("%s 未标极限加成来源: %+v", check.label, total)
		}
	}
	normal, _ := totalByLabel(result.Totals, "普通攻击伤害上限")
	for _, summonName := range []string{
		"火龙布雷扎莱克 · 传说 · 伤害",
		"地龙杜雷贝尔 · 传说 · 伤害",
		"冰龙威利努斯 · 传说 · 伤害",
	} {
		if !sourceContains(normal.Sources, summonName) {
			t.Fatalf("普通攻击上限没有真实召唤石来源 %q: %v", summonName, normal.Sources)
		}
	}
	healing, _ := totalByLabel(result.Totals, "HP回复上限")
	if !sourceContains(healing.Sources, "胡拉坎 · 传说 · 伤害") {
		t.Fatalf("回复上限没有真实召唤石来源: %v", healing.Sources)
	}
}

func realIoTerminusLoadout(t *testing.T) LoadoutEntry {
	t.Helper()
	groups, err := (&App{}).LoadoutList(testStatsSave)
	if err != nil {
		t.Fatal(err)
	}
	var best LoadoutEntry
	bestScore := -1
	for _, group := range groups {
		if group.CharaHash != testIoHash {
			continue
		}
		for _, loadout := range group.Loadouts {
			if loadout.WeaponSlotID == 52 {
				score := len(loadout.Mastery)*100 + len(loadout.Sigils)
				if score > bestScore {
					best, bestScore = loadout, score
				}
			}
		}
	}
	if bestScore >= 0 {
		return best
	}
	t.Fatal("真实伊欧配装没有 SlotID 52 的双蛇十字权杖")
	return LoadoutEntry{}
}

func TestLoadoutSimulateBuildIncludesRealWeaponSkillsMasteryAndFinalStats(t *testing.T) {
	requireStatsSave(t)
	loadout := realIoTerminusLoadout(t)
	sigilSlots := make([]uint32, loadoutMaxSigils)
	for _, sigil := range loadout.Sigils {
		if sigil.Index >= 0 && sigil.Index < len(sigilSlots) {
			sigilSlots[sigil.Index] = sigil.SlotID
		}
	}
	mastery := make([]string, 0, len(loadout.Mastery))
	for _, node := range loadout.Mastery {
		mastery = append(mastery, node.Hash)
	}

	result, err := (&App{}).LoadoutSimulateBuild(testStatsSave, testIoHash, loadout.WeaponSlotID, sigilSlots, nil, mastery, []uint32{35, 43, 52, 95})
	if err != nil {
		t.Fatal(err)
	}
	if result.Weapon == nil || result.Weapon.SlotID != 52 || result.Weapon.Total.ATK != 17083 || result.Weapon.Total.HP != 1149 {
		t.Fatalf("真实武器上下文没有进入模拟: %+v", result.Weapon)
	}
	if !result.Weapon.FormulaVerified {
		t.Fatalf("2.0.2 武器四段聚合已经由运行时代码、表和真实存档闭环: %+v", result.Weapon.Warnings)
	}
	if len(result.WeaponSkills) != 4 {
		t.Fatalf("真实超凡武器技能=%d，期望4: %+v", len(result.WeaponSkills), result.WeaponSkills)
	}
	for _, skill := range result.WeaponSkills {
		if skill.Name == "" || skill.Level <= 0 || skill.SourceWeapon != result.Weapon.Name {
			t.Fatalf("武器技能缺少可显示名称/等级/来源: %+v", skill)
		}
	}
	mageSavvyFound := false
	celestialTerraFound := false
	for _, bonus := range result.Bonuses {
		if bonus.TraitID == "SKILL_117_01" {
			mageSavvyFound = bonus.Level == 15 && strings.Contains(bonus.Effect, "50")
		}
		if bonus.TraitID == "9232DC17" || bonus.TraitID == "D29CD8E0" {
			celestialTerraFound = bonus.Level == 15 && strings.Contains(bonus.Effect, "最大HP-30%") && strings.Contains(bonus.Effect, "伤害上限+70%")
		}
	}
	if !mageSavvyFound {
		t.Fatalf("真实伊欧配装没有解析出魔法师的伶俐 Lv15 / 全局伤害上限+50%%: %+v", result.Bonuses)
	}
	if !celestialTerraFound {
		t.Fatalf("真实伊欧配装没有解析出天星之界 Lv15 的 HP-30%% / 全局上限+70%%: %+v", result.Bonuses)
	}
	if result.FinalStats == nil || result.FinalStats.HP <= 3156 || result.FinalStats.Attack <= 666 || result.FinalStats.CritRate < 5 || result.FinalStats.StunPower < 8 {
		t.Fatalf("最终人物属性没有合算配装: %+v", result.FinalStats)
	}
	t.Logf("真实伊欧配装最终属性: HP=%d ATK=%d Crit=%g Stun=%g Cap=%g", result.FinalStats.HP, result.FinalStats.Attack, result.FinalStats.CritRate, result.FinalStats.StunPower, result.FinalStats.DamageCap)
	t.Logf("真实伊欧配装完整最终属性: %+v", result.FinalStats)
	// skill_status stores the Lv45 stun curve as 10 with format {0:10}; the
	// format multiplier makes its panel contribution +100, so this real build's
	// merged stun result is 63.4 rather than the old under-scaled 17.5.
	if result.FinalStats.HP != 89390 || result.FinalStats.Attack != 76963 || result.FinalStats.CritRate != 30 || result.FinalStats.StunPower != 63.4 ||
		result.FinalStats.DamageCap != 1265 || result.FinalStats.NormalDamageCap != 1575 || result.FinalStats.AbilityDamageCap != 1355 || result.FinalStats.SkyboundDamageCap != 1265 {
		t.Fatalf("真实伊欧武器技能/魔法师的伶俐未进入最终属性: %+v", result.FinalStats)
	}
	if result.FinalStats.Scope != loadoutFinalStatsScope {
		t.Fatalf("最终属性计算范围不明确: %+v", result.FinalStats)
	}
	if result.FinalStats.FormulaVerified {
		t.Fatalf("武器公式已闭环不等于最终人物面板的百分比分组与舍入也已闭环")
	}
	weaponSourceFound := false
	for _, total := range result.Totals {
		for _, source := range total.Sources {
			if strings.Contains(source, result.Weapon.Name) {
				weaponSourceFound = true
			}
		}
	}
	if !weaponSourceFound {
		t.Fatalf("总计加成没有保留武器/武器技能来源: %+v", result.Totals)
	}
}

func TestLoadoutSimulateBuildFinalStatsReactToUnconditionalMasteryNode(t *testing.T) {
	requireStatsSave(t)
	without, err := (&App{}).LoadoutSimulateBuild(testStatsSave, testIoHash, 0, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	with, err := (&App{}).LoadoutSimulateBuild(testStatsSave, testIoHash, 0, nil, nil, []string{"317E9A83"}, nil) // 最大HP+15000
	if err != nil {
		t.Fatal(err)
	}
	if without.Weapon != nil || len(without.WeaponSkills) != 0 {
		t.Fatalf("未选择武器时不得伪造武器或技能: %+v / %+v", without.Weapon, without.WeaponSkills)
	}
	if without.FinalStats == nil || with.FinalStats == nil || with.FinalStats.HP-without.FinalStats.HP != 15000 {
		t.Fatalf("无条件专精节点没有改变最终 HP: without=%+v with=%+v", without.FinalStats, with.FinalStats)
	}
}

func TestValidateLoadoutSummonSlotIDsRejectsDuplicatesAndDanglingSlots(t *testing.T) {
	requireStatsSave(t)
	save, err := LoadSave(testStatsSave)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := validateLoadoutSummonSlotIDs(save, []uint32{35, 35, 52, 95}); err == nil || !strings.Contains(err.Error(), "重复") {
		t.Fatalf("重复召唤 SlotID 应拒绝，实际: %v", err)
	}
	if _, err := validateLoadoutSummonSlotIDs(save, []uint32{}); err == nil || !strings.Contains(err.Error(), "恰好") {
		t.Fatalf("非 nil 空召唤配置应按四槽要求拒绝，实际: %v", err)
	}
	if _, err := validateLoadoutSummonSlotIDs(save, []uint32{35, 43, 52, 0}); err == nil || !strings.Contains(err.Error(), "非零") {
		t.Fatalf("0召唤 SlotID 应拒绝，实际: %v", err)
	}
	if _, err := validateLoadoutSummonSlotIDs(save, []uint32{35, 43, 52, 999999}); err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("悬空召唤 SlotID 应拒绝，实际: %v", err)
	}
	if got, err := validateLoadoutSummonSlotIDs(save, []uint32{35, 43, 52, 95}); err != nil || !reflect.DeepEqual(got, []uint32{35, 43, 52, 95}) {
		t.Fatalf("真实四召唤配置未通过: got=%v err=%v", got, err)
	}
}

func TestOverLimitDefinitionsIncludeAuditedHighTierAliases(t *testing.T) {
	aliases := map[uint32]uint32{
		0xCB63BE55: 0xC4925BD7, 0xDCBD8423: 0xC4925BD7, 0x59DCE1E8: 0xC4925BD7, 0xF203BB15: 0xC4925BD7,
		0x57BBC478: 0x52A207B5, 0x5A51F0CB: 0x52A207B5, 0x9C6375CF: 0x52A207B5, 0xF004E9F2: 0x52A207B5,
		0xC4B86ED7: 0x45C65767, 0xCEB0DBD2: 0x45C65767,
		0xA3545CA1: 0x6CB38EF3, 0x59FBB7D8: 0x6CB38EF3,
	}
	for alias, canonical := range aliases {
		got, ok := overLimitDefinitions[alias]
		if !ok {
			t.Errorf("高档极限加成别名 %08X 未收录", alias)
			continue
		}
		want := overLimitDefinitions[canonical]
		if got != want {
			t.Errorf("高档极限加成别名 %08X=%+v，期望与 %08X=%+v 相同", alias, got, canonical, want)
		}
	}
}

func TestValidateLoadoutEquippedSummonTypedRequiresOneUIntVec4(t *testing.T) {
	valid := &SaveDataBinary{UIntTable: []UIntSaveDataUnit{{IDType: 1451, UnitID: 0, ValueData: []uint32{35, 43, 52, 95}}}}
	if err := validateLoadoutEquippedSummonTyped(valid); err != nil {
		t.Fatalf("合法 UInt/Unit0/vec4 被拒绝: %v", err)
	}
	for name, data := range map[string]*SaveDataBinary{
		"missing":      {},
		"int-only":     {IntTable: []IntSaveDataUnit{{IDType: 1451, UnitID: 0, ValueData: []int32{35, 43, 52, 95}}}},
		"wrong-length": {UIntTable: []UIntSaveDataUnit{{IDType: 1451, UnitID: 0, ValueData: []uint32{35, 43, 52}}}},
		"duplicate": {UIntTable: []UIntSaveDataUnit{
			{IDType: 1451, UnitID: 0, ValueData: []uint32{35, 43, 52, 95}},
			{IDType: 1451, UnitID: 0, ValueData: []uint32{35, 43, 52, 95}},
		}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateLoadoutEquippedSummonTyped(data); err == nil {
				t.Fatal("非法 typed 1451 应在 raw 写入前被拒绝")
			}
		})
	}
}

func loadoutWriteFromEntry(lo LoadoutEntry, summonSlotIDs []uint32) LoadoutWrite {
	sigils := make([]uint32, loadoutMaxSigils)
	for _, sigil := range lo.Sigils {
		if sigil.Index >= 0 && sigil.Index < len(sigils) {
			sigils[sigil.Index] = sigil.SlotID
		}
	}
	skills := make([]string, 0, len(lo.Skills))
	for _, skill := range lo.Skills {
		skills = append(skills, skill.Hash)
	}
	mastery := make([]string, 0, len(lo.Mastery))
	for _, node := range lo.Mastery {
		mastery = append(mastery, node.Hash)
	}
	return LoadoutWrite{
		UnitID: lo.UnitID, ExpectCharaHash: lo.CharaHash, Op: "write", Name: lo.Name,
		WeaponSlotID: lo.WeaponSlotID, SigilSlotIDs: sigils, SkillHashes: skills,
		MasteryHashes: mastery, SummonSlotIDs: summonSlotIDs,
	}
}

func firstIoLoadout(t *testing.T, path string) LoadoutEntry {
	t.Helper()
	groups, err := (&App{}).LoadoutList(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range groups {
		if group.CharaHash == testIoHash && len(group.Loadouts) > 0 {
			return group.Loadouts[0]
		}
	}
	t.Fatal("真实样本找不到伊欧配装")
	return LoadoutEntry{}
}

func summonInstanceSnapshot(t *testing.T, save *SaveData) map[uint32][]uint32 {
	t.Helper()
	result := map[uint32][]uint32{}
	for _, slotEntry := range save.findAllUnitsByType(1456) {
		slotID := slotEntry.Uint32()
		if slotID == 0 || slotID == EmptyHash {
			continue
		}
		typeEntry, typeOK := save.findUnitExact(1457, slotEntry.UnitID)
		traits, traitsOK := save.findUnitExact(1458, slotEntry.UnitID)
		levels, levelsOK := save.findUnitExact(1459, slotEntry.UnitID)
		rank, rankOK := save.findUnitExact(1460, slotEntry.UnitID)
		if !typeOK || !traitsOK || !levelsOK || !rankOK || typeEntry.Uint32() == 0 || typeEntry.Uint32() == EmptyHash {
			continue
		}
		mainHash, _ := traits.Uint32At(0)
		subHash, _ := traits.Uint32At(1)
		mainLevel, _ := levels.Uint32At(0)
		subLevel, _ := levels.Uint32At(1)
		result[slotID] = []uint32{slotEntry.UnitID, typeEntry.Uint32(), mainHash, subHash, mainLevel, subLevel, rank.Uint32()}
	}
	return result
}

func TestLoadoutApplyWritesSummonsAtomicallyAndDoesNotMutateInstances(t *testing.T) {
	input := copyStatsSave(t)
	output := filepath.Join(t.TempDir(), "written.dat")
	lo := firstIoLoadout(t, input)
	wantSlots := []uint32{95, 52, 43, 35}
	write := loadoutWriteFromEntry(lo, wantSlots)
	beforeSave, err := LoadSave(input)
	if err != nil {
		t.Fatal(err)
	}
	beforeInstances := summonInstanceSnapshot(t, beforeSave)

	result, err := (&App{}).LoadoutApply(input, output, []LoadoutWrite{write})
	if err != nil {
		t.Fatal(err)
	}
	if result.VerifiedFields < 7 {
		t.Fatalf("召唤配置未计入严格回读字段: %+v", result)
	}
	afterSave, err := LoadSave(output)
	if err != nil {
		t.Fatal(err)
	}
	if got := readLoadoutEquippedSummonSlotIDs(afterSave); !reflect.DeepEqual(got, wantSlots) {
		t.Fatalf("1451写后回读=%v，期望%v", got, wantSlots)
	}
	if afterInstances := summonInstanceSnapshot(t, afterSave); !reflect.DeepEqual(afterInstances, beforeInstances) {
		t.Fatal("写1451时修改了1456..1460召唤实例")
	}
}

func TestDifferentSummonConfigurationsAcrossWritesAreRejected(t *testing.T) {
	changes := []LoadoutWrite{
		{Op: "write", SummonSlotIDs: []uint32{35, 43, 52, 95}},
		{Op: "write", SummonSlotIDs: []uint32{95, 52, 43, 35}},
	}
	if _, err := sharedLoadoutSummonSlotIDs(changes); err == nil || !strings.Contains(err.Error(), "不同") {
		t.Fatalf("同批write不同召唤配置应拒绝，实际: %v", err)
	}
	changes[1].SummonSlotIDs = append([]uint32(nil), changes[0].SummonSlotIDs...)
	if got, err := sharedLoadoutSummonSlotIDs(changes); err != nil || !reflect.DeepEqual(got, changes[0].SummonSlotIDs) {
		t.Fatalf("同批write相同召唤配置应通过: got=%v err=%v", got, err)
	}
	changes = []LoadoutWrite{{Op: "clone", SummonSlotIDs: []uint32{1, 2, 3, 4}}, {Op: "clear", SummonSlotIDs: []uint32{5, 6, 7, 8}}}
	if got, err := sharedLoadoutSummonSlotIDs(changes); err != nil || got != nil {
		t.Fatalf("clear/clone不得请求全局召唤修改: got=%v err=%v", got, err)
	}
	changes = []LoadoutWrite{{Op: "write", SummonSlotIDs: []uint32{}}}
	if got, err := sharedLoadoutSummonSlotIDs(changes); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("非 nil 空配置不得被误当成未请求配置: got=%v nil=%v err=%v", got, got == nil, err)
	}
}
