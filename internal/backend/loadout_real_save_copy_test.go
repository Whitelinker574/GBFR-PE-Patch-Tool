package backend

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const isolatedSaveQAEnv = "GBFR_ISOLATED_SAVE_QA"

func requireIsolatedSaveQA(t *testing.T) string {
	t.Helper()
	path := strings.TrimSpace(os.Getenv(isolatedSaveQAEnv))
	if path == "" {
		t.Skipf("set %s to an isolated SaveData2.dat copy", isolatedSaveQAEnv)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	if protected := strings.TrimSpace(os.Getenv("GBFR_REAL_SAVE_WRITE_QA_PATH")); protected != "" && samePath(abs, protected) {
		t.Fatalf("%s must point to an isolated copy, not GBFR_REAL_SAVE_WRITE_QA_PATH", isolatedSaveQAEnv)
	}
	info, err := os.Stat(abs)
	if err != nil {
		t.Fatal(err)
	}
	if info.IsDir() || !strings.EqualFold(filepath.Base(abs), "SaveData2.dat") {
		t.Fatalf("isolated QA fixture must be a SaveData2.dat file: %s", abs)
	}
	return abs
}

func isolatedSaveDigest(t *testing.T, path string) [sha256.Size]byte {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(payload)
}

func isolatedIoLoadout(t *testing.T, path string) (CharacterLoadouts, LoadoutEntry) {
	t.Helper()
	groups, err := (&App{}).LoadoutList(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range groups {
		if !strings.EqualFold(group.CharaHash, testIoHash) {
			continue
		}
		for _, loadout := range group.Loadouts {
			if !loadout.IsParty && len(loadout.Sigils) == loadoutMaxSigils &&
				len(loadout.Skills) == loadoutMaxSkills && len(loadout.Mastery) == loadoutMaxMastery {
				return group, loadout
			}
		}
	}
	t.Fatal("isolated real save has no complete Io loadout (12 factors / 4 skills / 50 mastery nodes)")
	return CharacterLoadouts{}, LoadoutEntry{}
}

func loadoutVectors(loadout LoadoutEntry) (sigils []uint32, skills, mastery []string) {
	sigils = make([]uint32, loadoutMaxSigils)
	for _, sigil := range loadout.Sigils {
		if sigil.Index >= 0 && sigil.Index < len(sigils) {
			sigils[sigil.Index] = sigil.SlotID
		}
	}
	for _, skill := range loadout.Skills {
		skills = append(skills, skill.Hash)
	}
	for _, node := range loadout.Mastery {
		mastery = append(mastery, node.Hash)
	}
	return sigils, skills, mastery
}

// This opt-in integration test reads only the explicitly supplied isolated
// fixture. It closes the UI/backend seam with the exact data used for desktop
// QA instead of silently falling back to a developer's other local save.
func TestIsolatedRealSaveLoadoutFeatureIntegration(t *testing.T) {
	path := requireIsolatedSaveQA(t)
	before := isolatedSaveDigest(t, path)
	app := &App{}
	group, loadout := isolatedIoLoadout(t, path)

	if loadout.WeaponSlotID == 0 || loadout.WeaponName == "" {
		t.Fatalf("complete real loadout has no resolved weapon: %+v", loadout)
	}
	seenIndices := make(map[int]bool, loadoutMaxSigils)
	hasSecondary := false
	for _, sigil := range loadout.Sigils {
		if sigil.Missing || sigil.Name == "" || sigil.PrimaryTraitName == "" || sigil.PrimaryTraitLevel <= 0 {
			t.Fatalf("factor is missing its real item/primary trait data: %+v", sigil)
		}
		for _, label := range []string{sigil.Name, sigil.PrimaryTraitName, sigil.SecondaryTraitName} {
			if strings.Contains(label, "未收录") || strings.Contains(label, "Uncatalogued") {
				t.Fatalf("real equipped factor still uses a placeholder instead of a game name: %+v", sigil)
			}
		}
		if sigil.Index < 0 || sigil.Index >= loadoutMaxSigils || seenIndices[sigil.Index] {
			t.Fatalf("factor position is not a unique saved 1403 slot: %+v", sigil)
		}
		seenIndices[sigil.Index] = true
		if sigil.SecondaryTraitName != "" {
			hasSecondary = true
			if sigil.SecondaryTraitLevel <= 0 {
				t.Fatalf("factor secondary trait has no level: %+v", sigil)
			}
		}
	}
	if !hasSecondary {
		t.Fatal("real complete loadout exposed no secondary traits")
	}
	for _, skill := range loadout.Skills {
		if skill.Hash == "" || skill.Name == "" || skill.Key == "" {
			t.Fatalf("active skill is not fully resolved from unpacked data: %+v", skill)
		}
	}
	for _, node := range loadout.Mastery {
		if node.Hash == "" || node.Cat == "" || node.Rank == "" || node.RankLabel == "" || node.Desc == "" {
			t.Fatalf("mastery node is not detailed enough for the three-direction stage UI: %+v", node)
		}
	}

	edit, err := app.LoadoutEditContext(path, group.CharaHash)
	if err != nil {
		t.Fatal(err)
	}
	if edit.OwnerCode != "PL0400" || len(edit.Slots) != loadoutSlotsPerChara || len(edit.Skills) < 8 {
		t.Fatalf("Io editor context is incomplete: owner=%q slots=%d skills=%d", edit.OwnerCode, len(edit.Slots), len(edit.Skills))
	}
	if len(edit.Weapons) == 0 || len(edit.Sigils) == 0 || len(edit.MasterySources) == 0 {
		t.Fatalf("editor resource pools are incomplete: weapons=%d factors=%d mastery=%d", len(edit.Weapons), len(edit.Sigils), len(edit.MasterySources))
	}

	_, _, masteryHashes := loadoutVectors(loadout)
	pools, err := app.MasteryNodePool(edit.OwnerCode)
	if err != nil {
		t.Fatal(err)
	}
	wantCaps := map[string]int{"R1": 10, "R2": 10, "R3": 10, "EX": 20}
	if len(pools) != len(wantCaps) {
		t.Fatalf("mastery pool ranks=%d, want 4", len(pools))
	}
	for _, pool := range pools {
		if pool.Cap != wantCaps[pool.Rank] {
			t.Fatalf("mastery %s cap=%d, want %d", pool.Rank, pool.Cap, wantCaps[pool.Rank])
		}
		categories := map[string]bool{}
		for _, node := range pool.Nodes {
			categories[node.Cat] = true
		}
		for _, cat := range []string{"SB_ATK", "SB_DEF", "SB_LIMIT"} {
			if !categories[cat] {
				t.Fatalf("mastery %s has no %s direction", pool.Rank, cat)
			}
		}
	}
	summary, err := app.MasterySummarize(edit.OwnerCode, masteryHashes)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Total != loadoutMaxMastery || len(summary.Ranks) != 4 {
		t.Fatalf("mastery summary is incomplete: %+v", summary)
	}
	for _, rank := range summary.Ranks {
		if rank.Count != wantCaps[rank.Rank] || rank.Cap != wantCaps[rank.Rank] || len(rank.Categories) != 3 {
			t.Fatalf("mastery rank summary mismatch: %+v", rank)
		}
	}

	stats, err := app.LoadoutStatContext(path, group.CharaHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats.EquippedSummonSlotIDs) != 4 || len(stats.EquippedSummons) != 4 {
		t.Fatalf("real four-summon configuration is incomplete: slots=%v items=%d warnings=%v", stats.EquippedSummonSlotIDs, len(stats.EquippedSummons), stats.Warnings)
	}
	for _, summon := range stats.EquippedSummons {
		if summon.Name == "" || summon.MainTraitName == "" || summon.MainTraitLevel <= 0 || summon.SubParamName == "" {
			t.Fatalf("summon is missing its real name/trait/sub-parameter: %+v", summon)
		}
	}

	sigilSlots, _, _ := loadoutVectors(loadout)
	simulation, err := app.LoadoutSimulateBuild(path, group.CharaHash, loadout.WeaponSlotID, sigilSlots, nil, masteryHashes, stats.EquippedSummonSlotIDs)
	if err != nil {
		t.Fatal(err)
	}
	if simulation.Weapon == nil || simulation.Weapon.Name == "" || len(simulation.WeaponSkills) == 0 {
		t.Fatalf("weapon or weapon skills are absent from full build simulation: %+v", simulation.Weapon)
	}
	for _, skill := range simulation.WeaponSkills {
		if skill.Name == "" || skill.Effect == "" || skill.SourceWeapon == "" || skill.Level <= 0 {
			t.Fatalf("weapon skill is not display-ready: %+v", skill)
		}
	}
	if simulation.FinalStats == nil || simulation.FinalStats.HP <= stats.BaseHP || simulation.FinalStats.Attack <= stats.BaseATK {
		t.Fatalf("calculated final panel did not include the selected build: base=%+v final=%+v", stats, simulation.FinalStats)
	}
	if len(simulation.Bonuses) == 0 || len(simulation.Totals) == 0 {
		t.Fatalf("full build produced no factor/effect totals: bonuses=%d totals=%d", len(simulation.Bonuses), len(simulation.Totals))
	}

	share, err := buildLoadoutShare(path, loadout.UnitID)
	if err != nil {
		t.Fatal(err)
	}
	if share.Format != loadoutShareFormat || share.Version != loadoutShareVersion ||
		len(share.Sigils) != loadoutMaxSigils || len(share.Skills) != loadoutMaxSkills ||
		len(share.MasteryHashes) != loadoutMaxMastery || len(share.Summons) != 4 {
		t.Fatalf("single-loadout export is incomplete: %+v", share)
	}
	draft, err := resolveLoadoutShare(path, group.CharaHash, share)
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Missing) != 0 || !reflect.DeepEqual(draft.SummonSlotIDs, stats.EquippedSummonSlotIDs) {
		t.Fatalf("same-save import did not resolve exactly: missing=%v summons=%v", draft.Missing, draft.SummonSlotIDs)
	}
	if len(draft.SigilSlotIDs) != loadoutMaxSigils || len(draft.SkillHashes) != loadoutMaxSkills || len(draft.MasteryHashes) != loadoutMaxMastery {
		t.Fatalf("same-save import lost fields: factors=%d skills=%d mastery=%d", len(draft.SigilSlotIDs), len(draft.SkillHashes), len(draft.MasteryHashes))
	}

	if after := isolatedSaveDigest(t, path); after != before {
		t.Fatal("read-only integration test changed the isolated fixture")
	}
}

// The write path works on a second-generation temp copy. It constructs one
// natural factor and binds it into an existing complete loadout in the same
// transaction, then relies on LoadoutApply's disk re-read verification.
func TestIsolatedRealSaveConstructAndReadback(t *testing.T) {
	fixture := requireIsolatedSaveQA(t)
	fixtureDigest := isolatedSaveDigest(t, fixture)
	payload, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(t.TempDir(), "SaveData2.dat")
	if err := os.WriteFile(work, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPDATA", filepath.Join(t.TempDir(), "appdata"))

	app := &App{}
	group, source := isolatedIoLoadout(t, work)
	stats, err := app.LoadoutStatContext(work, group.CharaHash)
	if err != nil {
		t.Fatal(err)
	}
	sigils, skills, mastery := loadoutVectors(source)
	oldFirstSlot := sigils[0]
	item := naturalConstructedSigilItem(t)
	result, err := app.LoadoutApply(work, work, []LoadoutWrite{{
		UnitID: source.UnitID, ExpectCharaHash: group.CharaHash, Op: "write", Name: source.Name,
		WeaponSlotID: source.WeaponSlotID, SigilSlotIDs: sigils, SkillHashes: skills,
		MasteryHashes: mastery, SummonSlotIDs: stats.EquippedSummonSlotIDs,
		ConstructedSigils: []LoadoutConstructedSigil{{Index: 0, Item: item}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if result.SlotsWritten != 1 || result.CreatedCount != 1 || result.VerifiedCount != 1 || result.VerifiedFields < 7 || len(result.SlotIDs) != 1 {
		t.Fatalf("atomic write/readback result is incomplete: %+v", result)
	}
	if result.BackupPath == "" {
		t.Fatal("in-place temp-copy write did not create a backup")
	}
	if _, err := os.Stat(result.BackupPath); err != nil {
		t.Fatalf("write backup cannot be read: %v", err)
	}

	_, after := isolatedIoLoadout(t, work)
	if after.UnitID != source.UnitID {
		t.Fatalf("readback selected a different loadout: before=%d after=%d", source.UnitID, after.UnitID)
	}
	var first *LoadoutSigil
	for index := range after.Sigils {
		if after.Sigils[index].Index == 0 {
			first = &after.Sigils[index]
			break
		}
	}
	if first == nil || first.SlotID == oldFirstSlot || first.SlotID != result.SlotIDs[0] || first.Missing ||
		first.PrimaryTraitName == "" || first.PrimaryTraitLevel != item.PrimaryLevel {
		t.Fatalf("constructed factor was not bound/read back with real traits: old=%d result=%v factor=%+v", oldFirstSlot, result.SlotIDs, first)
	}
	afterStats, err := app.LoadoutStatContext(work, group.CharaHash)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(afterStats.EquippedSummonSlotIDs, stats.EquippedSummonSlotIDs) {
		t.Fatalf("loadout write changed the four-summon selection: before=%v after=%v", stats.EquippedSummonSlotIDs, afterStats.EquippedSummonSlotIDs)
	}
	if got := isolatedSaveDigest(t, fixture); got != fixtureDigest {
		t.Fatal("temp-copy write changed the isolated source fixture")
	}
	t.Logf("verified isolated save integration: %s; backup=%s", fmt.Sprintf("new factor SlotID %d", first.SlotID), result.BackupPath)
}

func TestIsolatedRealSaveImportCreatesIndependentCompleteFactorSet(t *testing.T) {
	fixture := requireIsolatedSaveQA(t)
	payload, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(t.TempDir(), "SaveData2.dat")
	if err := os.WriteFile(work, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPDATA", filepath.Join(t.TempDir(), "appdata"))

	app := &App{}
	group, source := isolatedIoLoadout(t, work)
	var target LoadoutEntry
	for _, candidate := range group.Loadouts {
		if !candidate.IsParty && candidate.UnitID != source.UnitID {
			target = candidate
			break
		}
	}
	if target.UnitID == 0 {
		t.Fatal("isolated real save has no second character loadout slot")
	}
	share, err := buildLoadoutShare(work, source.UnitID)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := resolveLoadoutShare(work, group.CharaHash, share)
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Missing) != 0 || len(draft.ConstructedSigils) != loadoutMaxSigils {
		t.Fatalf("import draft is incomplete: missing=%v factors=%d", draft.Missing, len(draft.ConstructedSigils))
	}
	sourceSlots, _, _ := loadoutVectors(source)
	result, err := app.LoadoutApply(work, work, []LoadoutWrite{{
		UnitID: target.UnitID, ExpectCharaHash: group.CharaHash, Op: "write", Name: draft.Name,
		WeaponSlotID: draft.WeaponSlotID, SigilSlotIDs: draft.SigilSlotIDs,
		ConstructedSigils: draft.ConstructedSigils, SkillHashes: draft.SkillHashes,
		MasteryHashes: draft.MasteryHashes, SummonSlotIDs: draft.SummonSlotIDs,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if result.SlotsWritten != 1 || result.CreatedCount != loadoutMaxSigils || len(result.SlotIDs) != loadoutMaxSigils {
		t.Fatalf("complete import did not create twelve factors: %+v", result)
	}

	afterGroups, err := app.LoadoutList(work)
	if err != nil {
		t.Fatal(err)
	}
	var afterSource, afterTarget LoadoutEntry
	for _, afterGroup := range afterGroups {
		if !strings.EqualFold(afterGroup.CharaHash, group.CharaHash) {
			continue
		}
		for _, loadout := range afterGroup.Loadouts {
			switch loadout.UnitID {
			case source.UnitID:
				afterSource = loadout
			case target.UnitID:
				afterTarget = loadout
			}
		}
	}
	afterSourceSlots, _, _ := loadoutVectors(afterSource)
	if !reflect.DeepEqual(afterSourceSlots, sourceSlots) {
		t.Fatalf("import reused or changed source factor instances: before=%v after=%v", sourceSlots, afterSourceSlots)
	}
	afterTargetSlots, _, _ := loadoutVectors(afterTarget)
	sourceSet := make(map[uint32]bool, len(sourceSlots))
	for _, slotID := range sourceSlots {
		sourceSet[slotID] = true
	}
	seen := make(map[uint32]bool, len(afterTargetSlots))
	for index, slotID := range afterTargetSlots {
		if slotID == 0 || sourceSet[slotID] || seen[slotID] {
			t.Fatalf("imported factor slot %d is not an independent instance: %d", index+1, slotID)
		}
		seen[slotID] = true
	}
}

func TestIsolatedRealSaveImportRestoresProgressionWeaponAndMissingSummon(t *testing.T) {
	fixture := requireIsolatedSaveQA(t)
	payload, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(t.TempDir(), "SaveData2.dat")
	if err := os.WriteFile(work, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPDATA", filepath.Join(t.TempDir(), "appdata"))

	group, source := isolatedIoLoadout(t, work)
	share, err := buildLoadoutShare(work, source.UnitID)
	if err != nil {
		t.Fatal(err)
	}
	if share.Character == nil || share.Weapon == nil || share.Weapon.Wrightstone == nil || len(share.Summons) != 4 {
		t.Fatalf("v3 source is incomplete: %+v", share)
	}
	stats, err := (&App{}).LoadoutStatContext(work, group.CharaHash)
	if err != nil {
		t.Fatal(err)
	}
	save, err := LoadSave(work)
	if err != nil {
		t.Fatal(err)
	}
	if err := save.patchInt(1323, stats.CharaUnitID, 0); err != nil {
		t.Fatal(err)
	}
	if err := save.patchUint(1321, stats.CharaUnitID, 0); err != nil {
		t.Fatal(err)
	}
	weaponUnitID, err := exactWeaponUnitForSlot(save, source.WeaponSlotID)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []uint32{weaponXPIDType, weaponUncapIDType, weaponMirageIDType, weaponAwakeIDType, weaponTranscendenceIDType} {
		if field == weaponXPIDType {
			if err := save.patchUint(field, weaponUnitID, 0); err != nil {
				t.Fatal(err)
			}
		} else if err := save.patchInt(field, weaponUnitID, 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := save.patchUint(weaponStoneIDType, weaponUnitID, EmptyHash); err != nil {
		t.Fatal(err)
	}
	if err := save.patchUint(weaponStoneSubType, weaponUnitID, EmptyHash); err != nil {
		t.Fatal(err)
	}
	missingCollectionWeapon := ""
	for _, weapon := range progressionInventory(save, work).Weapons {
		if weapon.OwnerCode == stats.OwnerCode && weapon.UnitID != weaponUnitID && weapon.InternalID != "" {
			missingCollectionWeapon = weapon.InternalID
			if err := save.patchUint(weaponIDType, weapon.UnitID, EmptyHash); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if missingCollectionWeapon == "" {
		t.Fatal("真实存档没有可用于武器收集补建测试的第二把角色武器")
	}
	traitBase := weaponTraitBase + (int(weaponUnitID)-weaponSlotBase)*weaponTraitStride
	for index := 0; index < 3; index++ {
		if err := save.patchUint(TraitHashIDType, uint32(traitBase+index), EmptyHash); err != nil {
			t.Fatal(err)
		}
		if err := save.patchInt(TraitLevelIDType, uint32(traitBase+index), 0); err != nil {
			t.Fatal(err)
		}
	}
	firstSummon := stats.EquippedSummons[0]
	if err := save.patchUint(SummonSlotIDType, firstSummon.UnitID, 0); err != nil {
		t.Fatal(err)
	}
	if err := save.writeSummonSaveState(firstSummon.UnitID, SummonTraitState{
		TypeHash: EmptyHash, MainTraitHash: EmptyHash, SubParamHash: EmptyHash,
		MainTraitLevel: ^uint32(0), SubParamLevel: ^uint32(0), Rank: 0,
	}); err != nil {
		t.Fatal(err)
	}
	if err := save.FixChecksums(); err != nil {
		t.Fatal(err)
	}
	if err := save.Write(work); err != nil {
		t.Fatal(err)
	}

	draft, err := resolveLoadoutShare(work, group.CharaHash, share)
	if err != nil {
		t.Fatal(err)
	}
	if draft.ApplyPayload == nil || draft.ApplyPayload.Character == nil || draft.ApplyPayload.Weapon == nil || len(draft.ApplyPayload.ConstructedSummons) != 1 {
		t.Fatalf("import payload did not capture missing state: %+v", draft.ApplyPayload)
	}
	var target LoadoutEntry
	for _, candidate := range group.Loadouts {
		if !candidate.IsParty && candidate.UnitID != source.UnitID {
			target = candidate
			break
		}
	}
	if target.UnitID == 0 {
		t.Fatal("no target preset")
	}
	result, err := (&App{}).LoadoutApplyWithResources(work, work, LoadoutApplyRequest{
		Changes: []LoadoutWrite{{
			UnitID: target.UnitID, ExpectCharaHash: group.CharaHash, Op: "write", Name: draft.Name,
			WeaponSlotID: draft.WeaponSlotID, SigilSlotIDs: draft.SigilSlotIDs, ConstructedSigils: draft.ConstructedSigils,
			SkillHashes: draft.SkillHashes, MasteryHashes: draft.MasteryHashes, SummonSlotIDs: draft.SummonSlotIDs,
		}},
		ImportPayload: draft.ApplyPayload,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.CreatedSummonCount != 1 {
		t.Fatalf("missing summon was not generated: %+v", result)
	}
	afterStats, err := (&App{}).LoadoutStatContext(work, group.CharaHash)
	if err != nil {
		t.Fatal(err)
	}
	if afterStats.PermanentGrowth.MasterTotalMSP != share.Character.MasterTotalMSP || afterStats.PermanentGrowth.LegacyProgress != share.Character.LegacyProgress {
		t.Fatalf("character progression mismatch: got %+v want %+v", afterStats.PermanentGrowth, share.Character)
	}
	afterSave, err := LoadSave(work)
	if err != nil {
		t.Fatal(err)
	}
	afterWeapon, err := readLoadoutWeaponContext(afterSave, draft.WeaponSlotID)
	if err != nil {
		t.Fatal(err)
	}
	if afterWeapon.XP != share.Weapon.XP || afterWeapon.Uncap != share.Weapon.Uncap || afterWeapon.Mirage != share.Weapon.Mirage ||
		afterWeapon.Awakening != share.Weapon.Awakening || afterWeapon.Transcendence != share.Weapon.Transcendence ||
		afterWeapon.Wrightstone == nil || !strings.EqualFold(afterWeapon.Wrightstone.Hash, share.Weapon.Wrightstone.Hash) {
		t.Fatalf("weapon state mismatch: got %+v want %+v", afterWeapon, share.Weapon)
	}
	foundCollectionWeapon := false
	for _, weapon := range progressionInventory(afterSave, work).Weapons {
		if weapon.InternalID == missingCollectionWeapon {
			foundCollectionWeapon = true
			break
		}
	}
	if !foundCollectionWeapon {
		t.Fatalf("角色强化武器收集缺少自动补建的 %s", missingCollectionWeapon)
	}
	if len(afterStats.EquippedSummons) != 4 || !strings.EqualFold(afterStats.EquippedSummons[0].TypeHash, share.Summons[0].TypeHash) {
		t.Fatalf("missing summon was not rebuilt into the equipped set: %+v", afterStats.EquippedSummons)
	}
}
