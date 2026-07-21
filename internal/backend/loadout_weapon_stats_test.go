package backend

import (
	"strings"
	"testing"
)

func TestReadLoadoutWeaponContextReadsRealIoTerminusWeapon(t *testing.T) {
	requireStatsSave(t)
	save, err := LoadSave(testStatsSave)
	if err != nil {
		t.Fatal(err)
	}

	context, err := readLoadoutWeaponContext(save, 52)
	if err != nil {
		t.Fatal(err)
	}

	if context.UnitID != 40049 || context.SlotID != 52 {
		t.Fatalf("weapon identity = unit %d slot %d, want unit 40049 slot 52", context.UnitID, context.SlotID)
	}
	if context.StoredHash != "1779CD60" || context.BaseHash != "FEBAC81A" {
		t.Fatalf("weapon hashes = stored %s base %s", context.StoredHash, context.BaseHash)
	}
	if context.InternalID != "WEP_PL0400_06" || context.Name == "" {
		t.Fatalf("weapon definition not resolved: %+v", context)
	}
	if context.Level != 150 || context.XP != 162540 || context.Uncap != 6 || context.Mirage != 99 || context.Awakening != 10 || context.Transcendence != 6 {
		t.Fatalf("weapon progression fields are wrong: %+v", context)
	}

	wantBase := WeaponStatLine{ATK: 2285, HP: 99}
	// weapon_status_awake stores per-level increments.  The Lv10 row is not a
	// replacement for Lv1-Lv9: summing all ten rows is what reproduces the
	// published max-awakening Terminus total (6283 ATK with +99, before rebuild).
	wantAwakening := WeaponStatLine{ATK: 3800}
	wantMirage := WeaponStatLine{ATK: 198, HP: 990}
	wantRebuild := WeaponStatLine{ATK: 10800, HP: 60}
	wantTotal := WeaponStatLine{ATK: 17083, HP: 1149}
	if context.Base != wantBase || context.AwakeningBonus != wantAwakening || context.MirageBonus != wantMirage || context.RebuildBonus != wantRebuild || context.Total != wantTotal {
		t.Fatalf("weapon stat ledger = base %+v awake %+v mirage %+v rebuild %+v total %+v", context.Base, context.AwakeningBonus, context.MirageBonus, context.RebuildBonus, context.Total)
	}

	wantSkills := []LoadoutWeaponSkill{
		{Slot: 0, TraitHash: "1E1CECCE", TraitID: "SKILL_143_10", Level: 30, Source: "weapon-rebuild"},
		{Slot: 1, TraitHash: "7CCFF74F", TraitID: "SKILL_067_00", Level: 20, Source: "weapon-rebuild"},
		{Slot: 2, TraitHash: "DC584F60", TraitID: "SKILL_020_00", Level: 10, Source: "weapon-rebuild"},
		{Slot: 3, TraitHash: "57E8A93F", TraitID: "SKILL_113_00", Level: 1, Source: "weapon-rebuild"},
	}
	if len(context.Skills) != len(wantSkills) {
		t.Fatalf("weapon skills = %+v, want %d entries", context.Skills, len(wantSkills))
	}
	for index, want := range wantSkills {
		got := context.Skills[index]
		if got.Slot != want.Slot || got.TraitHash != want.TraitHash || got.TraitID != want.TraitID || got.Level != want.Level || got.Source != want.Source || got.Name == "" || got.SourceWeapon != context.Name || got.LevelTableHash == "" {
			t.Fatalf("skill %d = %+v, want core fields %+v", index, got, want)
		}
	}
	if context.Skills[0].Name != "浩劫" || !strings.Contains(context.Skills[0].Effect, "150000") || !strings.Contains(context.Skills[0].Effect, "60.0%") || strings.Count(context.Skills[0].Effect, "430.0%") != 3 {
		t.Fatalf("SKILL_143_10 must use its independent DLC 2.0 Lv30 values, got %+v", context.Skills[0])
	}
	if context.Wrightstone == nil || context.Wrightstone.Hash != "09E6F629" || len(context.Wrightstone.Traits) != 3 {
		t.Fatalf("equipped weapon wrightstone was not resolved: %+v", context.Wrightstone)
	}
	wantWrightstoneTraits := []struct {
		Hash  string
		ID    string
		Level int
	}{{"CEB700EE", "SKILL_004_00", 8}, {"7CCFF74F", "SKILL_067_00", 7}, {"6018372B", "SKILL_078_00", 4}}
	for index, want := range wantWrightstoneTraits {
		got := context.Wrightstone.Traits[index]
		if got.Hash != want.Hash || got.TraitID != want.ID || got.Level != want.Level || got.Name == "" {
			t.Fatalf("wrightstone trait %d = %+v, want %+v", index, got, want)
		}
	}
	if !context.FormulaVerified {
		t.Fatalf("the 2.0.2 runtime aggregation path now verifies the complete weapon ledger: %+v", context.Warnings)
	}
	for _, warning := range context.Warnings {
		if strings.Contains(warning, "超越武器属性暂按解包表加算预览") {
			t.Fatalf("verified rebuild contribution must not keep the obsolete calibration warning: %v", context.Warnings)
		}
	}
}

func TestWeaponSkillsExplainTheirCurrentUnlockStage(t *testing.T) {
	requireStatsSave(t)
	save, err := LoadSave(testStatsSave)
	if err != nil {
		t.Fatal(err)
	}

	rebuild, err := readLoadoutWeaponContext(save, 52)
	if err != nil {
		t.Fatal(err)
	}
	for _, skill := range rebuild.Skills {
		if !strings.Contains(skill.UnlockCondition, "超凡 6/7") {
			t.Fatalf("rebuild skill has no current transcendence stage: %+v", skill)
		}
	}

	transcendence, ok := save.findUnitExact(weaponTranscendenceIDType, 40049)
	if !ok {
		t.Fatal("real Io terminus weapon has no 2817 field")
	}
	transcendence.SetInt32(0)
	awakened, err := readLoadoutWeaponContext(save, 52)
	if err != nil {
		t.Fatal(err)
	}
	for _, skill := range awakened.Skills {
		switch skill.Source {
		case "weapon-base":
			if !strings.Contains(skill.UnlockCondition, "上限突破 6/6") || !strings.Contains(skill.UnlockCondition, "觉醒 10/10") {
				t.Fatalf("base skill has no effective level basis: %+v", skill)
			}
		case "weapon-awakening":
			if !strings.Contains(skill.UnlockCondition, "觉醒") || !strings.Contains(skill.UnlockCondition, "解锁") {
				t.Fatalf("awakening skill has no unlock threshold: %+v", skill)
			}
		}
	}
}

func TestReadLoadoutWeaponContextReadsAllFiveTranscendenceSkills(t *testing.T) {
	requireStatsSave(t)
	save, err := LoadSave(testStatsSave)
	if err != nil {
		t.Fatal(err)
	}

	context, err := readLoadoutWeaponContext(save, 76)
	if err != nil {
		t.Fatal(err)
	}
	if context.UnitID != 40073 || context.StoredHash != "BE1BA9E3" || context.Transcendence != 7 {
		t.Fatalf("unexpected Slot76 weapon: %+v", context)
	}
	if context.TranscendenceSkill != "020DB733" || context.TranscendenceSkillName == "" {
		t.Fatalf("Slot76 raw seventh-stage effect was not exposed for stale-guarded editing: %+v", context)
	}
	if context.Total != (WeaponStatLine{ATK: 19497, HP: 3256}) {
		t.Fatalf("Slot76 total = %+v, want attack 19497 hp 3256; ledger=%+v", context.Total, context)
	}
	wantHashes := []string{"8D78A19B", "C0979A17", "AEFEB1BC", "E69A4694", "020DB733"}
	wantIDs := []string{"SKILL_003_00", "SKILL_013_00", "SKILL_314_00", "SKILL_045_00", "SKILL_317_00"}
	wantLevels := []int{35, 25, 15, 10, 15}
	if len(context.Skills) != 5 {
		t.Fatalf("Slot76 skills = %+v, want all five", context.Skills)
	}
	for index := range wantHashes {
		got := context.Skills[index]
		if got.TraitHash != wantHashes[index] || got.TraitID != wantIDs[index] || got.Level != wantLevels[index] {
			t.Fatalf("Slot76 skill %d = %+v", index, got)
		}
	}
	if context.Skills[2].Name != "伤害上限·苍天" || !strings.Contains(context.Skills[2].Effect, "伤害上限+270%") ||
		context.Skills[4].Name != "超凡技艺" || context.Skills[4].Effect != "能力伤害上限+30%" {
		t.Fatalf("2.0.2 transcendence skills must use the unpacked official names and values: %+v", context.Skills)
	}
	if len(context.SkillSlots) != 5 {
		t.Fatalf("editable weapon skill slots=%d, want full 2818 vector", len(context.SkillSlots))
	}
	if len(context.SkillSlots[1].Options) != 8 || len(context.SkillSlots[3].Options) != 9 {
		t.Fatalf("weapon-specific choice sets are incomplete: %+v", context.SkillSlots)
	}
	if context.SkillSlots[1].Editable != true || context.SkillSlots[3].Editable != true || context.SkillSlots[4].Editable {
		t.Fatalf("editable flags must follow exact unpacked group cardinality: %+v", context.SkillSlots)
	}
}

func TestWeaponStatTableSemanticsKeepKeyframesAndCumulativeGainsSeparate(t *testing.T) {
	rows := []weaponStatKeyframe{
		{Level: 1, Attack: 2, HP: 10},
		{Level: 2, Attack: 2, HP: 10},
		{Level: 3, Attack: 20, HP: 30},
	}
	if got := cumulativeWeaponStat(rows, 2); got != (WeaponStatLine{ATK: 4, HP: 20}) {
		t.Fatalf("cumulative lookup = %+v", got)
	}
	interpolated := interpolateWeaponStat([]weaponStatKeyframe{
		{Level: 100, Attack: 1000, HP: 100, Stun: 1, CritRate: 5},
		{Level: 150, Attack: 2001, HP: 201, Stun: 2, CritRate: 6},
	}, 125)
	if interpolated != (WeaponStatLine{ATK: 1500, HP: 150, Stun: 1.5, CritRate: 5}) {
		t.Fatalf("interpolated keyframe = %+v", interpolated)
	}

	// Awake/rebuild convert after summing the selected stages, while + mirage
	// converts after each stage. These differ for fractional synthetic rows even
	// though every official 2.0.2 row currently contains integral HP/ATK values.
	fractional := []weaponStatKeyframe{
		{Level: 1, Attack: 1.8, HP: 1.8, Stun: 0.8},
		{Level: 2, Attack: 1.8, HP: 1.8, Stun: 0.8},
	}
	if got := truncateRuntimeWeaponEnhancement(cumulativeWeaponStat(fractional, 2)); got != (WeaponStatLine{ATK: 3, HP: 3, Stun: 1}) {
		t.Fatalf("awake/rebuild runtime conversion = %+v", got)
	}
	if got := cumulativeRuntimeWeaponPlus(fractional, 2); got != (WeaponStatLine{ATK: 2, HP: 2}) {
		t.Fatalf("plus runtime per-stage conversion = %+v", got)
	}
}

func TestRebuildStatusAccumulatesRealHerculesStages(t *testing.T) {
	data, err := loadLoadoutWeaponStats()
	if err != nil {
		t.Fatal(err)
	}
	row, ok := data.Weapons["91DDC1F1"] // WEP_PL2300_04, Tweyen's Hercules.
	if !ok {
		t.Fatal("Hercules is missing from the embedded weapon table")
	}
	base := interpolateWeaponStat(data.Status[row.Key], 150)
	mirage := cumulativeWeaponStat(data.PlusStatus[row.PlusKey], 99)
	rebuild := cumulativeWeaponStat(data.RebuildStatus[row.RebuildStatusKey], 5)
	if rebuild != (WeaponStatLine{ATK: 14500, HP: 500}) {
		t.Fatalf("Hercules rebuild T5 = %+v, want cumulative 14500 ATK / 500 HP", rebuild)
	}
	if got := addWeaponStatLines(base, mirage, rebuild); got != (WeaponStatLine{ATK: 16997, HP: 3032}) {
		t.Fatalf("Hercules T5 +99 = %+v, want the independently observed 16997 ATK / 3032 HP", got)
	}
}

func TestUnresolvedWeaponSkillsDowngradeFormulaVerification(t *testing.T) {
	context := &LoadoutWeaponContext{
		FormulaVerified: true,
		Skills: []LoadoutWeaponSkill{
			{Slot: 0, TraitHash: "DEADBEEF", TraitID: "SKILL_UNKNOWN", Level: 15},
			{Slot: 1, TraitHash: "50079A1C", TraitID: "SKILL_000_00", Name: "攻击力", Effect: "攻击力+40", Level: 15},
		},
	}
	markUnresolvedWeaponSkills(context)
	if context.FormulaVerified || len(context.Warnings) != 1 || !strings.Contains(context.Warnings[0], "DEADBEEF") {
		t.Fatalf("未识别武器技能必须降级并说明来源: %+v", context)
	}
	verified := &LoadoutWeaponContext{FormulaVerified: true, Skills: context.Skills[1:]}
	markUnresolvedWeaponSkills(verified)
	if !verified.FormulaVerified || len(verified.Warnings) != 0 {
		t.Fatalf("完整解析的武器技能不应误报: %+v", verified)
	}
}

func TestFirstWeaponTableRowForBasePrefersTheLeastAwakenedVariant(t *testing.T) {
	rows := map[string]loadoutWeaponTableRow{
		"00000001": {Key: "00000001", WeaponID: "AAAAAAAA", AwakeningSkillHashes: [4]string{"11111111", "22222222", hashText(EmptyHash), hashText(EmptyHash)}},
		"FFFFFFFF": {Key: "FFFFFFFF", WeaponID: "AAAAAAAA", AwakeningSkillHashes: [4]string{"11111111", hashText(EmptyHash), hashText(EmptyHash), hashText(EmptyHash)}},
	}
	got, ok := firstWeaponTableRowForBase(rows, "AAAAAAAA")
	if !ok || got.Key != "FFFFFFFF" {
		t.Fatalf("fallback row = %+v, %v; want the variant with the fewest awakened skills", got, ok)
	}
}

func TestReadLoadoutWeaponContextFallsBackToBaseAndAwakeningSkillsWithoutTranscendence(t *testing.T) {
	requireStatsSave(t)
	save, err := LoadSave(testStatsSave)
	if err != nil {
		t.Fatal(err)
	}
	transcendence, ok := save.findUnitExact(weaponTranscendenceIDType, 40049)
	if !ok {
		t.Fatal("real Io terminus weapon has no 2817 field")
	}
	transcendence.SetInt32(0) // in-memory only; the real save is never written

	context, err := readLoadoutWeaponContext(save, 52)
	if err != nil {
		t.Fatal(err)
	}
	if !context.FormulaVerified || context.RebuildBonus != (WeaponStatLine{}) || context.Total != (WeaponStatLine{ATK: 6283, HP: 1089}) {
		t.Fatalf("non-transcended stat ledger = %+v", context)
	}
	wantHashes := []string{"40223C28", "6085DA25", "DC584F60", "57E8A93F"}
	wantLevels := []int{25, 15, 5, 1}
	wantSources := []string{"weapon-base", "weapon-base", "weapon-awakening", "weapon-awakening"}
	if len(context.Skills) != len(wantHashes) {
		t.Fatalf("non-transcended skills = %+v", context.Skills)
	}
	for index := range wantHashes {
		got := context.Skills[index]
		if got.TraitHash != wantHashes[index] || got.Level != wantLevels[index] || got.Source != wantSources[index] || got.Name == "" || got.SourceWeapon != context.Name {
			t.Fatalf("non-transcended skill %d = %+v", index, got)
		}
	}
}

func TestApplyRuntimeWeaponSkillLevelsUsesCharacterSpecificEffectiveLevel(t *testing.T) {
	skills := []LoadoutWeaponSkill{{TraitHash: "1E1CECCE", TraitID: "SKILL_143_10", Level: 30}}
	applyRuntimeWeaponSkillLevels(skills, []runtimeWeaponTrait{{Hash: 0x1E1CECCE, Level: 32}})
	if skills[0].Level != 32 || skills[0].StaticLevel != 30 || !skills[0].RuntimeObserved || skills[0].StableReads != 3 {
		t.Fatalf("runtime effective weapon skill was not preserved with its static source: %+v", skills[0])
	}
	if !strings.Contains(skills[0].Effect, "攻击力+64.0%") {
		t.Fatalf("runtime Lv32 effect was not re-rendered: %q", skills[0].Effect)
	}
}
