package backend

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestOptimizerNonPanelTotalsExcludeValuesAlreadyRepresentedByBaseStats(t *testing.T) {
	current := []EffectTotal{
		{Label: "攻击力", Unit: "flat", Value: 100},
		{Label: "普通攻击伤害上限", Unit: "pct", Value: 30},
		{Label: "防御力", Unit: "pct", Value: 20},
		{Label: "造成的伤害", Unit: "pct", Value: 8},
		{Label: "冷却时间缩短", Unit: "pct", Value: -5},
	}
	baseline := []EffectTotal{{Label: "造成的伤害", Unit: "pct", Value: 3}}
	delta := optimizerNonPanelTotalDelta(current, baseline)
	if len(delta) != 2 || delta[0].Label != "造成的伤害" || delta[0].Value != 5 || delta[1].Label != "冷却时间缩短" || delta[1].Value != -5 {
		t.Fatalf("non-panel delta = %#v", delta)
	}
}

func TestLoadoutOptimizerEvidenceUsesAuditedTraitCurves(t *testing.T) {
	evidence, err := loadoutOptimizerEvidence()
	if err != nil {
		t.Fatalf("load optimizer evidence: %v", err)
	}
	if evidence.DataVersion != "2.0.2" || evidence.SchemaVersion != 1 {
		t.Fatalf("evidence version = %q/%d", evidence.DataVersion, evidence.SchemaVersion)
	}
	var damageCap *LoadoutOptimizerTraitCurve
	for index := range evidence.Traits {
		if evidence.Traits[index].TraitID == "SKILL_020_00" {
			damageCap = &evidence.Traits[index]
			break
		}
	}
	if damageCap == nil || damageCap.MaxLevel != 65 || len(damageCap.Levels) != 65 {
		t.Fatalf("damage-cap curve is incomplete: %#v", damageCap)
	}
	level := damageCap.Levels[64]
	want := map[string]float64{
		"普通攻击伤害上限": 250,
		"能力伤害上限":   250,
		"奥义伤害上限":   250,
	}
	for _, total := range level.Totals {
		if expected, ok := want[total.Label]; ok {
			if total.Unit != "pct" || total.Value != expected {
				t.Fatalf("%s = %v %s, want %v pct", total.Label, total.Value, total.Unit, expected)
			}
			delete(want, total.Label)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing damage-cap totals: %v", want)
	}
}

func TestLoadoutOptimizerEvidenceKeepsConditionalComponents(t *testing.T) {
	evidence, err := loadoutOptimizerEvidence()
	if err != nil {
		t.Fatalf("load optimizer evidence: %v", err)
	}
	for _, curve := range evidence.Traits {
		if curve.TraitID != "SKILL_233_00" {
			continue
		}
		level := curve.Levels[len(curve.Levels)-1]
		if len(level.Components) != 3 {
			t.Fatalf("berserker components = %d, want 3", len(level.Components))
		}
		return
	}
	t.Fatal("berserker curve missing")
}

func TestLoadoutOptimizerInventorySnapshotCarriesDeployableEquipmentStages(t *testing.T) {
	requireStatsSave(t)
	app := &App{}
	groups, err := app.LoadoutList(statsSaveFixturePath())
	if err != nil {
		t.Fatal(err)
	}
	var selected LoadoutEntry
	for _, group := range groups {
		if !strings.EqualFold(group.CharaHash, testIoHash) || len(group.Loadouts) == 0 {
			continue
		}
		selected = group.Loadouts[0]
		break
	}
	if selected.UnitID == 0 || selected.WeaponSlotID == 0 {
		t.Fatalf("fixture has no selectable Io loadout: %#v", selected)
	}
	snapshot, err := app.LoadoutOptimizerInventorySnapshot(statsSaveFixturePath(), testIoHash, selected.UnitID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.SchemaVersion != 1 || snapshot.Domain != "inventory" || snapshot.DataVersion != "2.0.2" {
		t.Fatalf("snapshot identity = %#v", snapshot)
	}
	if snapshot.InputHash == "" || snapshot.TableHash == "" || snapshot.CatalogHash == "" {
		t.Fatalf("snapshot evidence hashes are incomplete: %#v", snapshot)
	}
	wantWeapon := "weapon:" + fmt.Sprint(selected.WeaponSlotID)
	wantSelection := map[string][]string{
		"weapon": {wantWeapon}, "wrightstone": {"wrightstone:" + wantWeapon},
		"mastery": {"mastery:" + fmt.Sprint(selected.UnitID)},
	}
	for stage, want := range wantSelection {
		got, ok := snapshot.BaseSelection[stage].([]string)
		if !ok || !slices.Equal(got, want) {
			t.Fatalf("base selection %s = %#v, want %#v", stage, snapshot.BaseSelection[stage], want)
		}
	}
	stats, err := app.LoadoutStatContext(statsSaveFixturePath(), testIoHash)
	if err != nil {
		t.Fatal(err)
	}
	wantSummons := make([]string, 0, len(stats.EquippedSummonSlotIDs))
	for _, slotID := range stats.EquippedSummonSlotIDs {
		if slotID != 0 {
			wantSummons = append(wantSummons, "summon:"+fmt.Sprint(slotID))
		}
	}
	if got, ok := snapshot.BaseSelection["summons"].([]string); !ok || !slices.Equal(got, wantSummons) {
		t.Fatalf("base selection summons = %#v, want %#v", snapshot.BaseSelection["summons"], wantSummons)
	}
	for _, key := range []string{"attack", "hp", "normalDamageCap", "abilityDamageCap", "skyboundDamageCap", "chainDamageCap"} {
		if _, ok := snapshot.BaseStats[key]; !ok {
			t.Fatalf("gearless baseline is missing %s", key)
		}
	}
	for _, total := range snapshot.BaseFixedTotals {
		if !strings.Contains(total.Label, "造成的伤害") && !strings.Contains(total.Label, "冷却时间") {
			t.Fatalf("baseline total %q is already represented by base stats or defense zones", total.Label)
		}
	}
	wanted := []string{"weapon", "wrightstone", "summons", "mastery"}
	for _, key := range wanted {
		index := slices.IndexFunc(snapshot.Stages, func(stage LoadoutOptimizerEquipmentStage) bool { return stage.Key == key })
		if index < 0 {
			t.Fatalf("missing equipment stage %s", key)
		}
		stage := snapshot.Stages[index]
		if stage.Choose > 0 && len(stage.Options) < stage.Choose {
			t.Fatalf("stage %s cannot satisfy choose=%d with %d options", key, stage.Choose, len(stage.Options))
		}
	}
	weaponStage := snapshot.Stages[slices.IndexFunc(snapshot.Stages, func(stage LoadoutOptimizerEquipmentStage) bool { return stage.Key == "weapon" })]
	wrightstoneStage := snapshot.Stages[slices.IndexFunc(snapshot.Stages, func(stage LoadoutOptimizerEquipmentStage) bool { return stage.Key == "wrightstone" })]
	if len(weaponStage.Options) == 0 || len(wrightstoneStage.Options) == 0 {
		t.Fatal("weapon and bound-wrightstone options must be present")
	}
	for _, option := range weaponStage.Options {
		if len(option.Variants) == 0 {
			t.Fatalf("weapon %s has no legal five-slot skill variants", option.ID)
		}
		for _, variant := range option.Variants {
			hashes, ok := variant.ApplyPayload["weaponSkillHashes"].([]string)
			if !ok || len(hashes) != 5 {
				t.Fatalf("weapon variant %s payload = %#v", variant.ID, variant.ApplyPayload)
			}
		}
	}
	for _, option := range wrightstoneStage.Options {
		if len(option.Requires["weapon"]) != 1 {
			t.Fatalf("wrightstone option is not bound to one weapon: %#v", option)
		}
	}
	summonStage := snapshot.Stages[slices.IndexFunc(snapshot.Stages, func(stage LoadoutOptimizerEquipmentStage) bool { return stage.Key == "summons" })]
	for _, option := range summonStage.Options {
		if editable, _ := option.ApplyPayload["editableMainTrait"].(bool); !editable {
			t.Fatalf("summon option %s is missing the confirmed main-trait edit capability: %#v", option.ID, option.ApplyPayload)
		}
		for _, key := range []string{"slotId", "expectUnitId", "expectTypeHash", "expectMainTraitHash", "expectMainTraitLevel", "expectSubParamHash", "expectSubParamLevel", "expectRank", "subParamHash", "subParamLevel", "rank"} {
			if _, ok := option.ApplyPayload[key]; !ok {
				t.Fatalf("summon option %s is missing stale/readback field %s: %#v", option.ID, key, option.ApplyPayload)
			}
		}
	}
	masteryStage := snapshot.Stages[slices.IndexFunc(snapshot.Stages, func(stage LoadoutOptimizerEquipmentStage) bool { return stage.Key == "mastery" })]
	for _, option := range masteryStage.Options {
		if option.ID == "mastery:none" {
			continue
		}
		for _, key := range []string{"normalDamageCap", "abilityDamageCap", "skyboundDamageCap", "chainDamageCap"} {
			if _, ok := option.BaseStatDeltas[key]; !ok {
				t.Fatalf("mastery option %s is missing %s delta", option.ID, key)
			}
		}
		for _, total := range option.FixedTotals {
			if !strings.Contains(total.Label, "造成的伤害") && !strings.Contains(total.Label, "冷却时间") {
				t.Fatalf("mastery option %s repeats represented total %q", option.ID, total.Label)
			}
		}
	}
	t.Logf("optimizer inventory options: weapon=%d wrightstone=%d summons=%d mastery=%d", len(snapshot.Stages[0].Options), len(snapshot.Stages[1].Options), len(snapshot.Stages[2].Options), len(snapshot.Stages[3].Options))
	readSnapshot, err := loadLoadoutReadSnapshot(statsSaveFixturePath())
	if err != nil {
		t.Fatal(err)
	}
	edit, err := (&App{}).loadoutEditContextFromLoaded(readSnapshot.save, testIoHash)
	if err != nil {
		t.Fatal(err)
	}
	for _, pick := range edit.Weapons {
		weapon, readErr := readLoadoutWeaponContext(readSnapshot.save, pick.SlotID)
		if readErr != nil {
			continue
		}
		counts := make([]int, len(weapon.SkillSlots))
		for index, slot := range weapon.SkillSlots {
			counts[index] = len(slot.Options)
		}
		t.Logf("weapon slot %d replacement option counts: %v", pick.SlotID, counts)
	}
}

func TestEveryMaxStageWeaponSkillIsResolvableAndReachableByOptimizer(t *testing.T) {
	data, err := loadLoadoutWeaponStats()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	traitIDs := traitHashMapWithRawKeys(catalog)
	checkedWeapons := 0
	for rowKey, row := range data.Weapons {
		complete := true
		for _, group := range row.RebuildSkillLevelKeys {
			if strings.TrimSpace(group) == "" || len(rebuildSkillOptionsForSlot(data, catalog, group, 7)) == 0 {
				complete = false
				break
			}
		}
		if !complete {
			continue
		}
		checkedWeapons++
		weapon := &LoadoutWeaponContext{
			StoredHash: rowKey, SlotID: uint32(checkedWeapons), Level: 150, Uncap: 6, Transcendence: 6,
			SkillSlots: make([]LoadoutWeaponSkillSlot, 5),
		}
		variants := optimizerWeaponSkillVariants(weapon, traitIDs, LoadoutPermanentGrowth{})
		if len(variants) == 0 {
			t.Fatalf("weapon %s has no optimizer variants", rowKey)
		}
		type skillKey struct {
			hash  string
			level int
		}
		reachable := map[skillKey]bool{}
		for _, variant := range variants {
			stage, _ := variant.ApplyPayload["weaponTranscendence"].(int)
			if stage != 7 {
				continue
			}
			for _, bonus := range variant.FixedBonuses {
				reachable[skillKey{hash: bonus.TraitID, level: bonus.Level}] = true
			}
		}
		for _, group := range row.RebuildSkillLevelKeys {
			for _, option := range rebuildSkillOptionsForSlot(data, catalog, group, 7) {
				hash, parseErr := ParseHashHex(option.Hash)
				if parseErr != nil {
					t.Fatalf("weapon %s group %s has invalid hash %q", rowKey, group, option.Hash)
				}
				traitID := resolveTraitValueID(hash, traitIDs)
				skill := newLoadoutWeaponSkill(data, catalog, weapon.Name, 0, hash, option.Level, "weapon-rebuild", group, "超凡 7/7")
				if traitID == "" || skill.TraitID == "" || strings.TrimSpace(skill.Name) == "" || strings.TrimSpace(skill.Effect) == "" {
					t.Fatalf("weapon %s group %s special skill is unresolved: option=%+v skill=%+v traitID=%q", rowKey, group, option, skill, traitID)
				}
				if !reachable[skillKey{hash: canonicalTraitValueID(traitID), level: option.Level}] {
					t.Fatalf("weapon %s group %s skill %s Lv%d is parsed but absent from optimizer variants", rowKey, group, skill.Name, option.Level)
				}
			}
		}
	}
	if checkedWeapons == 0 {
		t.Fatal("no five-slot transcendence weapons were checked")
	}
	t.Logf("checked %d weapon rows; every weapon-specific max-stage option is resolvable and optimizer-reachable", checkedWeapons)
}
