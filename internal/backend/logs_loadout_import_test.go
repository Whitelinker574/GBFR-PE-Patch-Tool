package backend

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/klauspost/compress/zstd"
)

func TestReadLogsLoadoutSharesProducesPartialDeployableCode(t *testing.T) {
	player := &logsLoadoutPlayer{
		DisplayName: "Player", CharacterName: "Zeta", CharacterType: "Pl1600",
		Sigils: []logsLoadoutSigil{{
			FirstTraitID: 0x50079A1C, FirstTraitLevel: 1,
			SecondTraitID: 0xDC584F60, SecondTraitLevel: 1,
			SigilID: 0x2D7F2E70, SigilLevel: 1,
		}},
		WeaponInfo: &logsLoadoutWeaponInfo{
			WeaponID: 0x02352554, StarLevel: 2, PlusMarks: 3, AwakeningLevel: 1,
			WrightstoneID: 0x09E6F629, WeaponLevel: 80,
			WrightstoneTraits: []logsLoadoutTrait{{ID: 0x50079A1C, Level: 5}, {ID: 0xDC584F60, Level: 3}},
			InnateTraits:      []logsLoadoutTrait{{ID: 0x7EDD69D0, Level: 1}},
		},
		OvermasteryInfo: &logsLoadoutOvermasteryInfo{Overmasteries: []logsLoadoutOvermastery{{ID: 0x52A207B5, Flags: 1, Value: 2}}},
		PlayerStats:     &logsLoadoutPlayerStats{Level: 100, TotalHP: 10000, TotalAttack: 20000, TotalPower: 30000},
	}
	if _, convertErr := logsPlayerLoadoutShare(123, player); convertErr != nil {
		t.Fatalf("fixture cannot be converted before database round-trip: %v", convertErr)
	}
	payload, err := cbor.Marshal(logsLoadoutEncounter{PlayerData: [4]*logsLoadoutPlayer{player}})
	if err != nil {
		t.Fatal(err)
	}
	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatal(err)
	}
	compressed := encoder.EncodeAll(payload, nil)
	encoder.Close()

	path := filepath.Join(t.TempDir(), "logs.dbw")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TABLE logs (time INTEGER NOT NULL, data BLOB NOT NULL, version INTEGER NOT NULL)`); err == nil {
		_, err = db.Exec(`INSERT INTO logs (time, data, version) VALUES (?, ?, 1)`, 123, compressed)
	}
	if closeErr := db.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}

	result, err := readLogsLoadoutShares(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result[0].OwnerCode != "PL1600" || result[0].CharacterHash != "0D21B430" || result[0].SigilCount != 1 || result[0].WeaponName == "" || result[0].OverLimitCount != 1 {
		t.Fatalf("logs candidates=%+v", result)
	}
	preview := result[0].Preview
	if preview == nil || !preview.Available || preview.CharacterHash != "0D21B430" || preview.CharacterName == "" {
		t.Fatalf("logs preview identity=%+v", preview)
	}
	if preview.Stats.Level != 100 {
		t.Fatalf("endgame Logs preview level=%d, want 100", preview.Stats.Level)
	}
	if len(preview.Sigils) != 1 || preview.Sigils[0].Level <= 1 || preview.Sigils[0].PrimaryTraitLevel <= 1 || preview.Sigils[0].SecondaryTraitLevel <= 1 {
		t.Fatalf("logs preview factors were not normalized: %+v", preview.Sigils)
	}
	if preview.Weapon.HashHex != "02352554" || len(preview.Weapon.Traits) != 2 || len(preview.Weapon.Skills) != 1 {
		t.Fatalf("logs preview weapon is incomplete: %+v", preview.Weapon)
	}
	if len(preview.CombinedSkills) == 0 {
		t.Fatal("logs preview did not build the merged skill ledger")
	}
	if len(preview.OverLimit) != 4 || preview.OverLimit[0].Name == "" || preview.OverLimit[0].Level != 10 {
		t.Fatalf("logs preview over mastery is incomplete: %+v", preview.OverLimit)
	}
	share, err := decodeLoadoutShareCode(result[0].CompatibilityCode)
	if err != nil {
		t.Fatal(err)
	}
	if share.SourceKind != loadoutShareSourceLogsDB || len(share.Sigils) != 1 || len(share.Summons) != 0 || len(share.MasteryHashes) != 0 {
		t.Fatalf("logs share=%+v", share)
	}
	if share.Sigils[0].Level <= 1 || share.Sigils[0].PrimaryTraitLevel <= 1 || share.Sigils[0].SecondaryTraitLevel <= 1 {
		t.Fatalf("endgame normalization was not applied: %+v", share.Sigils[0])
	}
	if share.Weapon == nil || share.Weapon.XP != weaponExpByLevel[149] || share.Weapon.Uncap != 6 || share.Weapon.Mirage != 99 || share.Weapon.Awakening != 10 || share.Weapon.Transcendence != 7 {
		t.Fatalf("weapon endgame normalization was not applied: %+v", share.Weapon)
	}
	if share.Weapon.Wrightstone == nil || len(share.Weapon.Wrightstone.Traits) != 2 || share.Weapon.Wrightstone.Traits[0].Level != 20 || share.Weapon.Wrightstone.Traits[1].Level != 15 {
		t.Fatalf("wrightstone endgame normalization was not applied: %+v", share.Weapon.Wrightstone)
	}
	if len(share.OverLimit) != 4 || share.OverLimit[0].Level != 10 || share.WeaponSkillHashes[0] != "7EDD69D0" {
		t.Fatalf("logs weapon/overlimit capture was incomplete: %+v", share)
	}
}

func TestLogsPlayerLoadoutRejectsUnknownCharacter(t *testing.T) {
	_, err := logsPlayerLoadoutShare(1, &logsLoadoutPlayer{CharacterType: "Unknown", Sigils: []logsLoadoutSigil{{SigilID: 1}}})
	if err == nil {
		t.Fatal("unknown logs character was accepted")
	}
}

func TestLogsPlayerLoadoutSupportsLegacyFixedWrightstoneTraits(t *testing.T) {
	player := &logsLoadoutPlayer{
		CharacterType: "Pl1600",
		Sigils: []logsLoadoutSigil{{
			FirstTraitID: 0x50079A1C, FirstTraitLevel: 1,
			SigilID: 0x2D7F2E70, SigilLevel: 1,
		}},
		WeaponInfo: &logsLoadoutWeaponInfo{
			WeaponID:          0x02352554,
			WrightstoneID:     0x09E6F629,
			LegacyTrait1ID:    0x50079A1C,
			LegacyTrait1Level: 5,
			LegacyTrait2ID:    0xDC584F60,
			LegacyTrait2Level: 3,
			LegacyTrait3ID:    EmptyHash,
		},
	}
	candidate, err := logsPlayerLoadoutShare(1, player)
	if err != nil {
		t.Fatal(err)
	}
	share, err := decodeLoadoutShareCode(candidate.CompatibilityCode)
	if err != nil {
		t.Fatal(err)
	}
	if share.Weapon == nil || share.Weapon.Wrightstone == nil || len(share.Weapon.Wrightstone.Traits) != 2 {
		t.Fatalf("legacy wrightstone traits were not preserved: %+v", share.Weapon)
	}
	if share.Weapon.Wrightstone.Traits[0].Hash != "50079A1C" || share.Weapon.Wrightstone.Traits[1].Hash != "DC584F60" {
		t.Fatalf("legacy wrightstone trait hashes changed: %+v", share.Weapon.Wrightstone.Traits)
	}
}
