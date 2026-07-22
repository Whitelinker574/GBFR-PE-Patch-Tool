package backend

import (
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

func TestLoadoutShareVersionEightCarriesExactRuntimeSnapshots(t *testing.T) {
	if loadoutShareVersion != 8 {
		t.Fatalf("loadout share version=%d, want 8", loadoutShareVersion)
	}
	payload := LoadoutImportApplyPayload{}
	if payload.ApplyMasteryConfiguration || payload.ApplyMasterProgress || payload.ApplyCharacterGrowth || payload.ApplyCharacterWeaponCollection || payload.ApplyCharacterWeaponWrightstones ||
		payload.ApplyWeaponEnhancement || payload.ApplyWeaponWrightstone || payload.ApplyOverLimit {
		t.Fatal("new import payload must default every destructive scope to false")
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
