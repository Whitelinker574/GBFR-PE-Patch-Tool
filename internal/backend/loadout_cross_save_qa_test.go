package backend

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const (
	loadoutSourceSaveQAEnv = "GBFR_LOADOUT_SOURCE_SAVE_QA"
	loadoutTargetSaveQAEnv = "GBFR_LOADOUT_TARGET_SAVE_QA"
)

func requireLoadoutCrossSaveQA(t *testing.T) (string, string) {
	t.Helper()
	source := strings.TrimSpace(os.Getenv(loadoutSourceSaveQAEnv))
	target := strings.TrimSpace(os.Getenv(loadoutTargetSaveQAEnv))
	if source == "" || target == "" {
		t.Skipf("set %s and %s to read-only source/target save fixtures", loadoutSourceSaveQAEnv, loadoutTargetSaveQAEnv)
	}
	if samePath(source, target) {
		t.Fatal("cross-save QA requires two different save fixtures")
	}
	for _, path := range []string{source, target} {
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			t.Fatalf("invalid cross-save fixture %q: info=%v err=%v", path, info, err)
		}
	}
	return source, target
}

func findCompleteCharacterLoadout(t *testing.T, path, charaHash string) (CharacterLoadouts, LoadoutEntry) {
	t.Helper()
	groups, err := (&App{}).LoadoutList(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range groups {
		if !strings.EqualFold(group.CharaHash, charaHash) {
			continue
		}
		for _, loadout := range group.Loadouts {
			if !loadout.IsParty && len(loadout.Mastery) == loadoutMaxMastery && loadout.Weapon != nil && loadout.Weapon.Wrightstone != nil {
				return group, loadout
			}
		}
	}
	t.Fatalf("save %s has no complete %s loadout", filepath.Base(path), charaHash)
	return CharacterLoadouts{}, LoadoutEntry{}
}

func findCharacterTargetLoadout(t *testing.T, path, charaHash string) (CharacterLoadouts, LoadoutEntry) {
	t.Helper()
	groups, err := (&App{}).LoadoutList(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range groups {
		if !strings.EqualFold(group.CharaHash, charaHash) {
			continue
		}
		for _, loadout := range group.Loadouts {
			if !loadout.IsParty {
				return group, loadout
			}
		}
	}
	t.Fatalf("save %s has no target %s loadout", filepath.Base(path), charaHash)
	return CharacterLoadouts{}, LoadoutEntry{}
}

func TestCrossSaveLoadoutCopiesExactPositionalAndProgressionFields(t *testing.T) {
	sourcePath, targetPath := requireLoadoutCrossSaveQA(t)
	const ferryHash = "C3FFD418"
	_, source := findCompleteCharacterLoadout(t, sourcePath, ferryHash)
	targetGroup, target := findCharacterTargetLoadout(t, targetPath, ferryHash)

	share, err := buildLoadoutShare(sourcePath, source.UnitID)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := resolveLoadoutShare(targetPath, ferryHash, share)
	if err != nil {
		t.Fatal(err)
	}
	if draft.ApplyPayload == nil || draft.ApplyPayload.Weapon == nil {
		t.Fatal("cross-save draft omitted exact weapon/character payload")
	}
	draft.ApplyPayload.ConstructedSummons = nil
	draft.ApplyPayload.ApplyMasteryConfiguration = true
	draft.ApplyPayload.ApplyMasterProgress = true
	draft.ApplyPayload.MasterProgressIndex = draft.Capabilities.SourceMasterProgressIndex
	draft.ApplyPayload.ApplyCharacterGrowth = true
	draft.ApplyPayload.ApplyCharacterWeaponCollection = true
	draft.ApplyPayload.ApplyCharacterWeaponWrightstones = true
	draft.ApplyPayload.ApplyWeaponEnhancement = true
	draft.ApplyPayload.ApplyWeaponWrightstone = true

	targetSigils, targetSkills, _ := loadoutVectors(target)
	output := filepath.Join(t.TempDir(), "SaveData2.dat")
	_, err = (&App{}).LoadoutApplyWithResources(targetPath, output, LoadoutApplyRequest{
		Changes: []LoadoutWrite{{
			UnitID: target.UnitID, ExpectCharaHash: targetGroup.CharaHash, Op: "write", Name: share.Name,
			WeaponSlotID: draft.WeaponSlotID, SigilSlotIDs: targetSigils, SkillHashes: targetSkills,
			WeaponSkillHashes: draft.WeaponSkillHashes, MasteryHashes: draft.MasteryHashes,
		}},
		ImportPayload: draft.ApplyPayload,
	})
	if err != nil {
		t.Fatal(err)
	}

	sourceSave, err := LoadSave(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	gotSave, err := LoadSave(output)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := readFixedVec(gotSave, loadoutMasteryIDType, target.UnitID, 50), readFixedVec(sourceSave, loadoutMasteryIDType, source.UnitID, 50); !reflect.DeepEqual(got, want) {
		t.Fatalf("3007 mastery slot order differs after import\ngot=%v\nwant=%v", got, want)
	}
	if got, want := readFixedVec(gotSave, loadoutWeaponSkillsIDType, target.UnitID, 5), readFixedVec(sourceSave, loadoutWeaponSkillsIDType, source.UnitID, 5); !reflect.DeepEqual(got, want) {
		t.Fatalf("3005 weapon skill snapshot differs after import: got=%v want=%v", got, want)
	}

	sourceWeaponUnit, err := exactWeaponUnitForSlot(sourceSave, source.WeaponSlotID)
	if err != nil {
		t.Fatal(err)
	}
	targetWeaponUnit, err := exactWeaponUnitForSlot(gotSave, draft.WeaponSlotID)
	if err != nil {
		t.Fatal(err)
	}
	for _, idType := range []uint32{weaponFlagsIDType, weaponStateIDType, weaponStoneSubType} {
		want, wok := sourceSave.findUnitExact(idType, sourceWeaponUnit)
		got, gok := gotSave.findUnitExact(idType, targetWeaponUnit)
		if !wok || !gok || got.Uint32() != want.Uint32() {
			var gotValue, wantValue uint32
			if gok {
				gotValue = got.Uint32()
			}
			if wok {
				wantValue = want.Uint32()
			}
			t.Fatalf("weapon field %d differs after import: got=%08X want=%08X", idType, gotValue, wantValue)
		}
	}
	if got, want := readFixedVec(gotSave, weaponExtraIDType, targetWeaponUnit, 5), readFixedVec(sourceSave, weaponExtraIDType, sourceWeaponUnit, 5); !reflect.DeepEqual(got, want) {
		t.Fatalf("2818 weapon skill vector differs after import: got=%v want=%v", got, want)
	}
	sourceTraitBase, err := weaponImbuedTraitUnitBase(sourceWeaponUnit)
	if err != nil {
		t.Fatal(err)
	}
	targetTraitBase, err := weaponImbuedTraitUnitBase(targetWeaponUnit)
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 3; index++ {
		for _, idType := range []uint32{TraitHashIDType, TraitLevelIDType} {
			want, wok := sourceSave.findUnitExact(idType, sourceTraitBase+uint32(index))
			got, gok := gotSave.findUnitExact(idType, targetTraitBase+uint32(index))
			if !wok || !gok || got.Uint32() != want.Uint32() {
				t.Fatalf("weapon wrightstone field %d slot %d differs after import", idType, index+1)
			}
		}
	}

	sourceCharacterUnit, err := loadoutCharacterUnitForHash(sourceSave, 0xC3FFD418)
	if err != nil {
		t.Fatal(err)
	}
	targetCharacterUnit, err := loadoutCharacterUnitForHash(gotSave, 0xC3FFD418)
	if err != nil {
		t.Fatal(err)
	}
	for _, idType := range []uint32{1321, 1323} {
		want, _ := sourceSave.findUnitExact(idType, sourceCharacterUnit)
		got, _ := gotSave.findUnitExact(idType, targetCharacterUnit)
		if want == nil || got == nil || got.Uint32() != want.Uint32() {
			t.Fatalf("character field %d differs after import", idType)
		}
	}
	if got, want := readFixedVec(gotSave, 1503, targetCharacterUnit, 2), readFixedVec(sourceSave, 1503, sourceCharacterUnit, 2); !reflect.DeepEqual(got, want) {
		t.Fatalf("1503 character enhancement panel differs after import: got=%v want=%v", got, want)
	}
	sourceNodeBase := uint32(10000000) + (sourceCharacterUnit-10000)*1000
	targetNodeBase := uint32(10000000) + (targetCharacterUnit-10000)*1000
	sourceNodes := make(map[uint32]int32)
	targetNodes := make(map[uint32]int32)
	for index := uint32(0); index < 1000; index++ {
		want, wok := sourceSave.findUnitExact(1602, sourceNodeBase+index)
		if wok {
			sourceNodes[index] = want.Int32()
		}
		got, gok := gotSave.findUnitExact(1602, targetNodeBase+index)
		if gok {
			targetNodes[index] = got.Int32()
		}
	}
	if !reflect.DeepEqual(targetNodes, sourceNodes) {
		t.Fatalf("1602 character enhancement node block differs after import: got=%d nodes want=%d nodes", len(targetNodes), len(sourceNodes))
	}
	sourceWeapons := make(map[string]LoadoutShareProgressionWeapon, len(share.Character.Weapons))
	for _, weapon := range share.Character.Weapons {
		sourceWeapons[weapon.InternalID] = weapon
	}
	verifiedWrightstones := 0
	for _, weapon := range progressionInventory(gotSave, output).Weapons {
		want, ok := sourceWeapons[weapon.InternalID]
		if !ok {
			continue
		}
		snapshot, prepareErr := prepareWeaponWrightstone(want.Wrightstone)
		if prepareErr != nil {
			t.Fatal(prepareErr)
		}
		if verifyErr := verifyPreparedWeaponWrightstone(gotSave, expectedWeaponWrightstone{unitID: weapon.UnitID, snapshot: snapshot}); verifyErr != nil {
			t.Fatalf("character weapon %s wrightstone differs after import: %v", weapon.InternalID, verifyErr)
		}
		verifiedWrightstones++
	}
	if verifiedWrightstones != len(sourceWeapons) {
		t.Fatalf("verified %d character weapon wrightstones, want %d", verifiedWrightstones, len(sourceWeapons))
	}
}

func TestSourceSaveExposesSavedDLCLoadouts(t *testing.T) {
	sourcePath, _ := requireLoadoutCrossSaveQA(t)
	groups, err := (&App{}).LoadoutList(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, group := range groups {
		if len(group.Loadouts) > 0 {
			found[group.CharaName] = true
		}
	}
	for _, name := range []string{"芙劳", "菲迪埃尔"} {
		if !found[name] {
			t.Fatalf("saved DLC loadout for %s was filtered from LoadoutList", name)
		}
	}
}
