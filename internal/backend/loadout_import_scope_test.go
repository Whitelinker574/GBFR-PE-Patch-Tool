package backend

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadoutImportDefaultsDoNotPreparePermanentOrWeaponChanges(t *testing.T) {
	payload := &LoadoutImportApplyPayload{
		Character: &LoadoutShareCharacterProgression{
			MasterTotalMSP: 3309499,
			LegacyProgress: 999,
			Weapons:        []LoadoutShareProgressionWeapon{{InternalID: "must-not-be-touched", Level: 150}},
		},
		Weapon: &LoadoutShareWeaponState{
			StoredHash: "DEADBEEF", XP: ^uint32(0), Uncap: 6, Mirage: 99, Awakening: 10, Transcendence: 7,
		},
	}
	prepared, err := prepareLoadoutImport(&SaveData{}, []LoadoutWrite{{Op: "write", ExpectCharaHash: "4D0A60C3"}}, payload)
	if err != nil {
		t.Fatalf("unselected source data must remain inert: %v", err)
	}
	if prepared.masterTotalMSP != nil || prepared.legacyProgress != nil || prepared.weapon != nil ||
		len(prepared.characterWeaponChanges) != 0 || len(prepared.overLimit) != 0 {
		t.Fatalf("default import prepared hidden writes: %+v", prepared)
	}
}

func TestMasterProgressSelectionPreservesSourceTotalOrUsesAuditedThreshold(t *testing.T) {
	const sourceTotal = 3309499
	got, err := masterTotalMSPForProgress(sourceTotal, 55)
	if err != nil || got != sourceTotal {
		t.Fatalf("unchanged progress should preserve source MSP: got=%d err=%v", got, err)
	}
	got, err = masterTotalMSPForProgress(sourceTotal, 20)
	if err != nil || got != characterMasterExpThresholds[20] {
		t.Fatalf("edited progress should use exact 2.0.2 threshold: got=%d want=%d err=%v", got, characterMasterExpThresholds[20], err)
	}
	for _, invalid := range []int{0, 56} {
		if _, err := masterTotalMSPForProgress(sourceTotal, invalid); err == nil {
			t.Fatalf("invalid mastery progress %d was accepted", invalid)
		}
	}
}

func TestLoadoutShareVersionNineCarriesExactRuntimeSnapshots(t *testing.T) {
	if loadoutShareVersion != 9 {
		t.Fatalf("loadout share version=%d, want 9", loadoutShareVersion)
	}
	payload := LoadoutImportApplyPayload{}
	if payload.ApplyCharacterLevel || payload.ApplyMasteryConfiguration || payload.ApplyMasterProgress || payload.ApplyCharacterGrowth || payload.ApplyCharacterWeaponCollection || payload.ApplyCharacterWeaponWrightstones ||
		payload.ApplyWeaponEnhancement || payload.ApplyWeaponWrightstone || payload.ApplyOverLimit {
		t.Fatal("new import payload must default every destructive scope to false")
	}
}

func exactCharacterBaseFixture() *LoadoutShareCharacterProgression {
	return &LoadoutShareCharacterProgression{
		CharacterLevel:        100,
		BaseHP:                3156,
		BaseATK:               666,
		BaseStunBits:          math.Float32bits(8),
		BaseCritRate:          5,
		CharacterBaseCaptured: true,
	}
}

func prepareCharacterLevelFixture(t *testing.T) (*SaveData, uint32, uint32) {
	t.Helper()
	path, share := actualLoadoutShareFixture(t)
	save, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	charaHash, err := ParseHashHex(share.CharaHash)
	if err != nil {
		t.Fatal(err)
	}
	unitID, err := loadoutCharacterUnitForHash(save, charaHash)
	if err != nil {
		t.Fatal(err)
	}
	fate, ok := save.findUnitExact(1318, unitID)
	if !ok || fate.ValueCnt != 1 {
		t.Fatal("fixture lacks Fate field")
	}
	for idType, value := range map[uint32]uint32{
		1308: 80,
		1309: 2000,
		1310: 400,
		1312: math.Float32bits(5),
		1313: 3,
	} {
		if err := save.patchUintExact(idType, unitID, value); err != nil {
			t.Fatal(err)
		}
	}
	return save, unitID, fate.Uint32()
}

func TestCharacterLevelImportCopiesExactBaseSnapshotAndPreservesFate(t *testing.T) {
	save, unitID, wantFate := prepareCharacterLevelFixture(t)
	payload := &LoadoutImportApplyPayload{
		Character:           exactCharacterBaseFixture(),
		ApplyCharacterLevel: true,
	}
	prepared, err := prepareLoadoutImport(save, []LoadoutWrite{{Op: "write", ExpectCharaHash: "4D0A60C3"}}, payload)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.characterBase == nil || prepared.characterBase.level != 100 {
		t.Fatalf("character base snapshot was not prepared: %+v", prepared.characterBase)
	}
	if _, err := applyPreparedLoadoutImport(save, prepared); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyPreparedLoadoutImport(save, prepared); err != nil {
		t.Fatal(err)
	}
	for idType, want := range map[uint32]uint32{
		1308: 100,
		1309: 3156,
		1310: 666,
		1312: math.Float32bits(8),
		1313: 5,
	} {
		entry, ok := save.findUnitExact(idType, unitID)
		if !ok || entry.ValueCnt != 1 || entry.Uint32() != want {
			t.Fatalf("character base field %d readback=%v/%08X, want %08X", idType, ok, entry.Uint32(), want)
		}
	}
	fate, ok := save.findUnitExact(1318, unitID)
	if !ok || fate.Uint32() != wantFate {
		t.Fatalf("character level import changed Fate: got=%v/%08X want=%08X", ok, fate.Uint32(), wantFate)
	}
}

func TestLowLevelTargetAutoPromotesForMasteryOrCharacterGrowth(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mastery bool
		growth  bool
	}{
		{name: "mastery", mastery: true},
		{name: "character-growth", growth: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			save, _, _ := prepareCharacterLevelFixture(t)
			_, source := actualLoadoutShareFixture(t)
			source.Character.CharacterLevel = 100
			source.Character.BaseHP = 3156
			source.Character.BaseATK = 666
			source.Character.BaseStunBits = math.Float32bits(8)
			source.Character.BaseCritRate = 5
			source.Character.CharacterBaseCaptured = true
			payload := &LoadoutImportApplyPayload{
				Character:                 source.Character,
				ApplyMasteryConfiguration: tc.mastery,
				ApplyCharacterGrowth:      tc.growth,
			}
			prepared, err := prepareLoadoutImport(save, []LoadoutWrite{{Op: "write", ExpectCharaHash: source.CharaHash}}, payload)
			if err != nil {
				t.Fatal(err)
			}
			if prepared.characterBase == nil || prepared.characterBase.level != 100 {
				t.Fatalf("low-level target was not auto-promoted: %+v", prepared.characterBase)
			}
		})
	}
}

func TestLowLevelTargetRejectsAutoPromotionWithoutLevel100Snapshot(t *testing.T) {
	save, _, _ := prepareCharacterLevelFixture(t)
	source := exactCharacterBaseFixture()
	source.CharacterLevel = 80
	_, err := prepareLoadoutImport(save, []LoadoutWrite{{Op: "write", ExpectCharaHash: "4D0A60C3"}}, &LoadoutImportApplyPayload{
		Character:                 source,
		ApplyMasteryConfiguration: true,
	})
	if err == nil || !strings.Contains(err.Error(), "Lv100") {
		t.Fatalf("non-Lv100 source was accepted for automatic promotion: %v", err)
	}
}

func TestCharacterLevelImportRejectsInvalidOrIncompleteBaseSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*LoadoutShareCharacterProgression)
	}{
		{name: "not-captured", mutate: func(source *LoadoutShareCharacterProgression) { source.CharacterBaseCaptured = false }},
		{name: "invalid-level", mutate: func(source *LoadoutShareCharacterProgression) { source.CharacterLevel = 101 }},
		{name: "nan-stun", mutate: func(source *LoadoutShareCharacterProgression) {
			source.BaseStunBits = math.Float32bits(float32(math.NaN()))
		}},
		{name: "infinite-stun", mutate: func(source *LoadoutShareCharacterProgression) {
			source.BaseStunBits = math.Float32bits(float32(math.Inf(1)))
		}},
		{name: "negative-stun", mutate: func(source *LoadoutShareCharacterProgression) { source.BaseStunBits = math.Float32bits(-1) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			save, _, _ := prepareCharacterLevelFixture(t)
			source := exactCharacterBaseFixture()
			tc.mutate(source)
			_, err := prepareLoadoutImport(save, []LoadoutWrite{{Op: "write", ExpectCharaHash: "4D0A60C3"}}, &LoadoutImportApplyPayload{
				Character: source, ApplyCharacterLevel: true,
			})
			if err == nil {
				t.Fatal("invalid character base snapshot was accepted")
			}
		})
	}
}

func TestCharacterLevelImportSupportsNonMaxSourceWhenSelectedAlone(t *testing.T) {
	save, _, _ := prepareCharacterLevelFixture(t)
	source := exactCharacterBaseFixture()
	source.CharacterLevel = 80
	source.BaseHP = 2000
	source.BaseATK = 400
	source.BaseStunBits = math.Float32bits(5)
	source.BaseCritRate = 3
	prepared, err := prepareLoadoutImport(save, []LoadoutWrite{{Op: "write", ExpectCharaHash: "4D0A60C3"}}, &LoadoutImportApplyPayload{
		Character: source, ApplyCharacterLevel: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.characterBase == nil || prepared.characterBase.level != 80 ||
		prepared.characterBase.baseHP != 2000 || prepared.characterBase.baseATK != 400 ||
		prepared.characterBase.baseStunBits != math.Float32bits(5) || prepared.characterBase.baseCritRate != 3 {
		t.Fatalf("non-max source snapshot changed: %+v", prepared.characterBase)
	}
}

func TestCharacterLevelImportRejectsMalformedTargetBaseField(t *testing.T) {
	for _, idType := range []uint32{1308, 1309, 1310, 1312, 1313} {
		t.Run(fmt.Sprintf("field-%d", idType), func(t *testing.T) {
			save, unitID, _ := prepareCharacterLevelFixture(t)
			entry, ok := save.findUnitExact(idType, unitID)
			if !ok {
				t.Fatalf("fixture lacks field %d", idType)
			}
			entry.ValueCnt = 2
			_, err := prepareLoadoutImport(save, []LoadoutWrite{{Op: "write", ExpectCharaHash: "4D0A60C3"}}, &LoadoutImportApplyPayload{
				Character: exactCharacterBaseFixture(), ApplyCharacterLevel: true,
			})
			if err == nil {
				t.Fatalf("malformed target field %d was accepted", idType)
			}
		})
	}
}

func TestUnselectedCharacterLevelLeavesBaseFieldsUntouched(t *testing.T) {
	save, unitID, _ := prepareCharacterLevelFixture(t)
	before := make(map[uint32]uint32)
	for _, idType := range []uint32{1308, 1309, 1310, 1312, 1313} {
		entry, _ := save.findUnitExact(idType, unitID)
		before[idType] = entry.Uint32()
	}
	prepared, err := prepareLoadoutImport(save, []LoadoutWrite{{Op: "write", ExpectCharaHash: "4D0A60C3"}}, &LoadoutImportApplyPayload{
		Character: exactCharacterBaseFixture(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.characterBase != nil {
		t.Fatal("unselected character level prepared a hidden write")
	}
	if _, err := applyPreparedLoadoutImport(save, prepared); err != nil {
		t.Fatal(err)
	}
	for idType, want := range before {
		entry, _ := save.findUnitExact(idType, unitID)
		if entry.Uint32() != want {
			t.Fatalf("unselected character base field %d changed", idType)
		}
	}
}

func TestActualSaveCopyAutoPromotesBeforeMasteryAndGrowthImport(t *testing.T) {
	fixturePath, share := actualLoadoutShareFixture(t)
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(t.TempDir(), "SaveData-input.dat")
	output := filepath.Join(t.TempDir(), "SaveData-output.dat")
	if err := os.WriteFile(input, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	save, err := LoadSave(input)
	if err != nil {
		t.Fatal(err)
	}
	charaHash, err := ParseHashHex(share.CharaHash)
	if err != nil {
		t.Fatal(err)
	}
	characterUnitID, err := loadoutCharacterUnitForHash(save, charaHash)
	if err != nil {
		t.Fatal(err)
	}
	fate, ok := save.findUnitExact(1318, characterUnitID)
	if !ok || fate.ValueCnt != 1 {
		t.Fatal("fixture lacks Fate field")
	}
	wantFate := fate.Uint32()
	for idType, value := range map[uint32]uint32{
		1308: 80,
		1309: 2000,
		1310: 400,
		1312: math.Float32bits(5),
		1313: 3,
	} {
		if err := save.patchUintExact(idType, characterUnitID, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := save.FixChecksums(); err != nil {
		t.Fatal(err)
	}
	if err := save.Write(input); err != nil {
		t.Fatal(err)
	}

	groups, err := (&App{}).LoadoutList(input)
	if err != nil {
		t.Fatal(err)
	}
	var target *LoadoutEntry
	for groupIndex := range groups {
		for loadoutIndex := range groups[groupIndex].Loadouts {
			candidate := &groups[groupIndex].Loadouts[loadoutIndex]
			if !candidate.IsParty && strings.EqualFold(candidate.CharaHash, share.CharaHash) && candidate.Name == share.Name {
				target = candidate
				break
			}
		}
	}
	if target == nil {
		t.Fatal("copied fixture no longer contains the shared loadout")
	}
	draft, err := resolveLoadoutShare(input, share.CharaHash, share)
	if err != nil {
		t.Fatal(err)
	}
	if draft.Capabilities.TargetCharacterLevel != 80 || draft.ApplyPayload == nil {
		t.Fatalf("low-level target was not resolved correctly: %+v", draft)
	}
	draft.ApplyPayload.ApplyMasteryConfiguration = true
	draft.ApplyPayload.ApplyCharacterGrowth = true
	sigils, skills, mastery := loadoutVectors(*target)
	result, err := (&App{}).LoadoutApplyWithResources(input, output, LoadoutApplyRequest{
		Changes: []LoadoutWrite{{
			UnitID: target.UnitID, ExpectCharaHash: target.CharaHash, Op: "write", Name: target.Name,
			WeaponSlotID: draft.WeaponSlotID, SigilSlotIDs: sigils, SkillHashes: skills,
			WeaponSkillHashes: append([]string(nil), draft.WeaponSkillHashes...), MasteryHashes: mastery,
		}},
		ImportPayload: draft.ApplyPayload,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.SlotsWritten != 1 {
		t.Fatalf("actual save copy did not write exactly one loadout: %+v", result)
	}
	written, err := LoadSave(output)
	if err != nil {
		t.Fatal(err)
	}
	for idType, want := range map[uint32]uint32{
		1308: uint32(share.Character.CharacterLevel),
		1309: uint32(share.Character.BaseHP),
		1310: uint32(share.Character.BaseATK),
		1312: share.Character.BaseStunBits,
		1313: uint32(share.Character.BaseCritRate),
	} {
		entry, exists := written.findUnitExact(idType, characterUnitID)
		if !exists || entry.ValueCnt != 1 || entry.Uint32() != want {
			t.Fatalf("actual save field %d readback=%v/%08X, want %08X", idType, exists, entry.Uint32(), want)
		}
	}
	gotFate, ok := written.findUnitExact(1318, characterUnitID)
	if !ok || gotFate.ValueCnt != 1 || gotFate.Uint32() != wantFate {
		t.Fatalf("actual save import changed Fate: got=%v/%08X want=%08X", ok, gotFate.Uint32(), wantFate)
	}
}

func TestActualSave2BeatrixImportsNonMaxCharacterLevelSnapshot(t *testing.T) {
	path := strings.TrimSpace(os.Getenv("GBFR_TEST_BEATRIX_SAVE"))
	if path == "" {
		t.Skip("set GBFR_TEST_BEATRIX_SAVE to a read-only SaveData2 fixture")
	}
	save, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	const beatrixHash = uint32(0x9A8AF295)
	unitID, err := loadoutCharacterUnitForHash(save, beatrixHash)
	if err != nil {
		t.Fatal(err)
	}
	sourceValues := make(map[uint32]uint32, 6)
	for _, idType := range []uint32{1308, 1309, 1310, 1312, 1313, 1318} {
		entry, ok := save.findUnitExact(idType, unitID)
		if !ok || entry.ValueCnt != 1 {
			t.Fatalf("Beatrix field %d is missing or malformed", idType)
		}
		sourceValues[idType] = entry.Uint32()
	}
	sourceLevel := int(int32(sourceValues[1308]))
	if sourceLevel < 1 || sourceLevel >= 100 {
		t.Fatalf("SaveData2 Beatrix level=%d, want a non-max test source", sourceLevel)
	}
	source := &LoadoutShareCharacterProgression{
		CharacterLevel: sourceLevel, BaseHP: int(int32(sourceValues[1309])),
		BaseATK: int(int32(sourceValues[1310])), BaseStunBits: sourceValues[1312],
		BaseCritRate: int(int32(sourceValues[1313])), CharacterBaseCaptured: true,
	}
	for idType, value := range map[uint32]uint32{
		1308: 1, 1309: 1, 1310: 1, 1312: math.Float32bits(1), 1313: 1,
	} {
		if err := save.patchUintExact(idType, unitID, value); err != nil {
			t.Fatal(err)
		}
	}
	prepared, err := prepareLoadoutImport(save, []LoadoutWrite{{Op: "write", ExpectCharaHash: hashText(beatrixHash)}}, &LoadoutImportApplyPayload{
		Character: source, ApplyCharacterLevel: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := applyPreparedLoadoutImport(save, prepared); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyPreparedLoadoutImport(save, prepared); err != nil {
		t.Fatal(err)
	}
	for _, idType := range []uint32{1308, 1309, 1310, 1312, 1313} {
		entry, _ := save.findUnitExact(idType, unitID)
		if entry.Uint32() != sourceValues[idType] {
			t.Fatalf("Beatrix field %d readback=%08X, want %08X", idType, entry.Uint32(), sourceValues[idType])
		}
	}
	fate, _ := save.findUnitExact(1318, unitID)
	if fate.Uint32() != sourceValues[1318] {
		t.Fatalf("Beatrix level import changed Fate: got=%08X want=%08X", fate.Uint32(), sourceValues[1318])
	}
}

func TestActualBeatrixLevel100SourceImportsIntoSave2Level60Target(t *testing.T) {
	sourcePath := strings.TrimSpace(os.Getenv("GBFR_TEST_BEATRIX_LEVEL100_SAVE"))
	targetPath := strings.TrimSpace(os.Getenv("GBFR_TEST_BEATRIX_LEVEL60_SAVE"))
	if sourcePath == "" || targetPath == "" {
		t.Skip("set GBFR_TEST_BEATRIX_LEVEL100_SAVE and GBFR_TEST_BEATRIX_LEVEL60_SAVE to read-only fixtures")
	}
	sourceSave, err := LoadSave(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	targetSave, err := LoadSave(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	const beatrixHash = uint32(0x9A8AF295)
	sourceUnitID, err := loadoutCharacterUnitForHash(sourceSave, beatrixHash)
	if err != nil {
		t.Fatal(err)
	}
	targetUnitID, err := loadoutCharacterUnitForHash(targetSave, beatrixHash)
	if err != nil {
		t.Fatal(err)
	}
	sourceValues := make(map[uint32]uint32, 5)
	for _, idType := range []uint32{1308, 1309, 1310, 1312, 1313} {
		entry, ok := sourceSave.findUnitExact(idType, sourceUnitID)
		if !ok || entry.ValueCnt != 1 {
			t.Fatalf("source Beatrix field %d is missing or malformed", idType)
		}
		sourceValues[idType] = entry.Uint32()
	}
	if level := int(int32(sourceValues[1308])); level != 100 {
		t.Fatalf("source Beatrix level=%d, want 100", level)
	}
	targetLevel, ok := targetSave.findUnitExact(1308, targetUnitID)
	if !ok || targetLevel.ValueCnt != 1 || targetLevel.Int32() >= 100 {
		t.Fatalf("target Beatrix is not a valid below-Lv100 fixture")
	}
	targetFate, ok := targetSave.findUnitExact(1318, targetUnitID)
	if !ok || targetFate.ValueCnt != 1 {
		t.Fatal("target Beatrix Fate field is missing or malformed")
	}
	wantFate := targetFate.Uint32()
	source := &LoadoutShareCharacterProgression{
		CharacterLevel: int(int32(sourceValues[1308])),
		BaseHP:         int(int32(sourceValues[1309])),
		BaseATK:        int(int32(sourceValues[1310])),
		BaseStunBits:   sourceValues[1312],
		BaseCritRate:   int(int32(sourceValues[1313])),

		CharacterBaseCaptured: true,
	}
	prepared, err := prepareLoadoutImport(targetSave, []LoadoutWrite{{
		Op: "write", ExpectCharaHash: hashText(beatrixHash),
	}}, &LoadoutImportApplyPayload{
		Character: source, ApplyCharacterLevel: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := applyPreparedLoadoutImport(targetSave, prepared); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyPreparedLoadoutImport(targetSave, prepared); err != nil {
		t.Fatal(err)
	}
	for idType, want := range sourceValues {
		entry, exists := targetSave.findUnitExact(idType, targetUnitID)
		if !exists || entry.ValueCnt != 1 || entry.Uint32() != want {
			t.Fatalf("target Beatrix field %d readback=%v/%08X, want %08X", idType, exists, entry.Uint32(), want)
		}
	}
	gotFate, _ := targetSave.findUnitExact(1318, targetUnitID)
	if gotFate.Uint32() != wantFate {
		t.Fatalf("cross-save Beatrix level import changed Fate: got=%08X want=%08X", gotFate.Uint32(), wantFate)
	}
}

func TestOverLimitImportRequiresACompleteFourSlotSnapshot(t *testing.T) {
	for _, slots := range [][]LoadoutShareOverLimit{nil, {{Index: 0}}} {
		_, err := prepareLoadoutImport(&SaveData{}, []LoadoutWrite{{Op: "write", ExpectCharaHash: "4D0A60C3"}}, &LoadoutImportApplyPayload{
			ApplyOverLimit: true,
			OverLimit:      slots,
		})
		if err == nil || !strings.Contains(err.Error(), "完整 4 槽快照") {
			t.Fatalf("partial over-limit snapshot was accepted: slots=%d err=%v", len(slots), err)
		}
	}
}
