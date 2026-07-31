package backend

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/klauspost/compress/zstd"
)

const relinkLogsPlayerJSON = `{
  "actorIndex": 2,
  "displayName": "Clipboard Player",
  "characterName": "Zeta",
  "characterType": "Pl1600",
  "sigils": [{
    "firstTraitId": 1342675484,
    "firstTraitLevel": 15,
    "secondTraitId": 3696775008,
    "secondTraitLevel": 15,
    "sigilId": 763309680,
    "equippedCharacter": 0,
    "sigilLevel": 15,
    "acquisitionCount": 1,
    "notificationEnum": 0
  }],
  "summons": [
    {"summonId":1071160378,"mainTraitId":1342675484,"mainTraitLevel":1,"bonusId":1513740315,"bonusLevel":0},
    {"summonId":1071160378,"mainTraitId":1342675484,"mainTraitLevel":1,"bonusId":1513740315,"bonusLevel":0},
    {"summonId":1071160378,"mainTraitId":1342675484,"mainTraitLevel":1,"bonusId":1513740315,"bonusLevel":0},
    {"summonId":1071160378,"mainTraitId":1342675484,"mainTraitLevel":1,"bonusId":1513740315,"bonusLevel":0}
  ],
  "abilities": [2514750994,1730610451,2537767594,1390669521],
  "weaponKey": "",
  "masterLevel": 55,
  "skillboard": [32406538],
  "stats": {"level": 100, "hp": 20000, "attack": 30000, "unk50": 0, "stunPower": 300, "criticalRate": 100, "power": 40000},
  "weaponState": {
    "weaponId": 37037396,
    "exp": 1234,
    "starLevel": 6,
    "plusMarks": 99,
    "awakeningLevel": 10,
    "wrightstoneId": 166131241,
    "wrightstoneTraits": [{"id": 1342675484, "level": 20}],
    "innateTraits": [{"id": 2128439760, "level": 25}]
  },
  "isOnline": true,
  "weaponInfo": null,
  "overmasteryInfo": {"overmasteries": [{"id": 1386350517, "flags": 512, "value": 20}]},
  "playerStats": null
}`

const logsSourceSchema = `CREATE TABLE logs (
	id INTEGER PRIMARY KEY,
	name TEXT NOT NULL,
	time INTEGER NOT NULL,
	duration INTEGER NOT NULL,
	data BLOB NOT NULL,
	version INTEGER NOT NULL DEFAULT 0,
	primary_target INTEGER,
	p1_name TEXT,
	p1_type TEXT,
	p2_name TEXT,
	p2_type TEXT,
	p3_name TEXT,
	p3_type TEXT,
	p4_name TEXT,
	p4_type TEXT,
	quest_id INTEGER,
	quest_elapsed_time INTEGER,
	quest_completed BOOLEAN,
	run_id INTEGER,
	room_index INTEGER,
	total_damage INTEGER
)`

func encodeLogsTestEncounter(t *testing.T, encounter logsLoadoutEncounter) []byte {
	t.Helper()
	payload, err := cbor.Marshal(encounter)
	if err != nil {
		t.Fatal(err)
	}
	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer encoder.Close()
	return encoder.EncodeAll(payload, nil)
}

func createLogsTestDatabase(t *testing.T, version int, blob []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "logs.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(logsSourceSchema); err == nil && blob != nil {
		_, err = db.Exec(`INSERT INTO logs (name, time, duration, data, version) VALUES (?, ?, ?, ?, ?)`, "fixture", 123, 1000, blob, version)
	}
	if closeErr := db.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestReadLogsLoadoutSharesProducesPartialDeployableCode(t *testing.T) {
	player := &logsLoadoutPlayer{
		DisplayName: "Player", CharacterName: "Zeta", CharacterType: logsCharacterType{Code: "Pl1600"},
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
	path := createLogsTestDatabase(t, 1, encodeLogsTestEncounter(t, logsLoadoutEncounter{PlayerData: [4]*logsLoadoutPlayer{player}}))

	result, err := readLogsLoadoutShares(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result[0].OwnerCode != "PL1600" || result[0].CharacterHash != "0D21B430" || result[0].SigilCount != 1 || result[0].WeaponName == "" || result[0].OverLimitCount != 1 {
		t.Fatalf("logs candidates=%+v", result)
	}
	if result[0].ProtocolLabel == "" || !containsString(result[0].CapturedFields, "weaponSkills") || containsString(result[0].MissingFields, "weaponSkills") {
		t.Fatalf("logs capability metadata=%+v", result[0])
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

func TestLogsBattleArchiveAndLoadoutImportReuseOneDatabaseSession(t *testing.T) {
	player := &logsLoadoutPlayer{
		DisplayName: "Player", CharacterName: "Zeta", CharacterType: logsCharacterType{Code: "Pl1600"},
		Sigils: []logsLoadoutSigil{{
			FirstTraitID: 0x50079A1C, FirstTraitLevel: 15,
			SecondTraitID: 0xDC584F60, SecondTraitLevel: 15,
			SigilID: 0x2D7F2E70, SigilLevel: 15,
		}},
	}
	path := createLogsTestDatabase(t, 1, encodeLogsTestEncounter(t, logsLoadoutEncounter{PlayerData: [4]*logsLoadoutPlayer{player}}))
	app := &App{logsArchivePath: path}
	t.Cleanup(app.CloseLogsBattleArchive)

	if _, err := app.LogsBattleArchivePage(LogsBattleArchiveRequest{Limit: 1}); err != nil {
		t.Fatal(err)
	}
	opened := app.logsArchiveDB
	if opened == nil {
		t.Fatal("battle archive did not retain the shared database session")
	}

	if _, err := app.SelectLogsLoadoutShares(); err != nil {
		t.Fatal(err)
	}
	if app.logsArchiveDB != opened {
		t.Fatal("loadout import replaced the database session opened by the battle archive")
	}

	app.CloseLogsBattleArchive()
	if app.logsArchiveDB != nil || app.logsArchivePath != "" || app.logsArchiveColumns != nil {
		t.Fatal("disconnect did not clear the shared Logs database session")
	}
}

func TestReadLogsLoadoutSharesRejectsOversizedBlobBeforeDecode(t *testing.T) {
	path := createLogsTestDatabase(t, 1, nil)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(
		`INSERT INTO logs (name, time, duration, data, version) VALUES (?, ?, ?, zeroblob(?), 1)`,
		"oversized", 123, 1000, logsLoadoutMaximumBlob+1,
	); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()
	_, err = readLogsLoadoutShares(path)
	if err == nil || !strings.Contains(err.Error(), "压缩记录大小") {
		t.Fatalf("oversized Logs blob was not rejected at the SQL boundary: %v", err)
	}
}

func TestLogsPlayerLoadoutRejectsUnknownCharacter(t *testing.T) {
	_, err := logsPlayerLoadoutShare(1, &logsLoadoutPlayer{CharacterType: logsCharacterType{UnknownHash: 1}, Sigils: []logsLoadoutSigil{{SigilID: 1}}})
	if err == nil {
		t.Fatal("unknown logs character was accepted")
	}
}

func TestLogsPlayerLoadoutSupportsLegacyFixedWrightstoneTraits(t *testing.T) {
	player := &logsLoadoutPlayer{
		CharacterType: logsCharacterType{Code: "Pl1600"},
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
	if containsString(share.CapturedFields, "weaponSkills") || len(share.WeaponSkillHashes) != 0 || len(share.Weapon.SkillHashes) != 0 {
		t.Fatalf("legacy log invented weapon skills: %+v", share)
	}
}

func TestLegacyWrightstoneFallbackHandlesMissingWeaponInfo(t *testing.T) {
	var source *logsLoadoutWeaponInfo
	if traits := source.effectiveWrightstoneTraits(); len(traits) != 0 {
		t.Fatalf("nil weapon info returned traits: %+v", traits)
	}
}

func TestLogsPlayerLoadoutKeepsWeaponOnlyPartialCapture(t *testing.T) {
	player := &logsLoadoutPlayer{
		CharacterType: logsCharacterType{Code: "Pl1600"},
		WeaponInfo:    &logsLoadoutWeaponInfo{WeaponID: 0x02352554, WeaponLevel: 150},
	}
	candidate, err := logsPlayerLoadoutShare(1, player)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.SigilCount != 0 || !containsString(candidate.CapturedFields, "weapon") || containsString(candidate.CapturedFields, "sigils") {
		t.Fatalf("weapon-only candidate metadata=%+v", candidate)
	}
	share, err := decodeLoadoutShareCode(candidate.CompatibilityCode)
	if err != nil {
		t.Fatal(err)
	}
	if share.Weapon == nil || len(share.Sigils) != 0 || len(share.WeaponSkillHashes) != 0 {
		t.Fatalf("weapon-only share=%+v", share)
	}
}

func TestUnknownOptionalWeaponAndOvermasteryFieldsDoNotHideSigils(t *testing.T) {
	player := &logsLoadoutPlayer{
		CharacterType: logsCharacterType{Code: "Pl1600"},
		Sigils:        []logsLoadoutSigil{{SigilID: 0x2D7F2E70, SigilLevel: 15, FirstTraitID: 0x50079A1C, FirstTraitLevel: 15}},
		WeaponInfo: &logsLoadoutWeaponInfo{
			WeaponID: 0x02352554, WrightstoneID: 0x09E6F629,
			WrightstoneTraits: []logsLoadoutTrait{{ID: 0xDEADBEEF, Level: 20}},
			InnateTraits:      []logsLoadoutTrait{{ID: 0xBAADF00D, Level: 25}},
		},
		OvermasteryInfo: &logsLoadoutOvermasteryInfo{Overmasteries: []logsLoadoutOvermastery{{ID: 0x52A207B5, Flags: 3, Value: 2}}},
	}
	candidate, err := logsPlayerLoadoutShare(1, player)
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(candidate.CapturedFields, "sigils") || !containsString(candidate.CapturedFields, "weapon") ||
		containsString(candidate.CapturedFields, "wrightstone") || containsString(candidate.CapturedFields, "weaponSkills") ||
		containsString(candidate.CapturedFields, "overLimit") || len(candidate.Warnings) < 3 {
		t.Fatalf("optional field degradation metadata=%+v", candidate)
	}
	share, err := decodeLoadoutShareCode(candidate.CompatibilityCode)
	if err != nil {
		t.Fatal(err)
	}
	if len(share.Sigils) != 1 || share.Weapon == nil || share.Weapon.Wrightstone != nil || share.Weapon.WrightstoneReference != "" ||
		len(share.WeaponSkillHashes) != 0 || len(share.Weapon.SkillHashes) != 0 || len(share.OverLimit) != 0 {
		t.Fatalf("optional invalid scopes leaked into share=%+v", share)
	}
	share.Weapon.WrightstoneReference = "09E6F629"
	if err := validateLoadoutShareProvenance(share); err == nil {
		t.Fatal("undeclared wrightstone reference was accepted")
	}
}

func TestOversizedOrUnknownWeaponSubscopesRemainPreviewOnly(t *testing.T) {
	base := logsLoadoutPlayer{
		CharacterType: logsCharacterType{Code: "Pl1600"},
		WeaponInfo:    &logsLoadoutWeaponInfo{WeaponID: 0x02352554, WrightstoneID: 0x09E6F629},
	}
	base.WeaponInfo.WrightstoneTraits = []logsLoadoutTrait{
		{ID: 0x50079A1C, Level: 20}, {ID: 0xDC584F60, Level: 15}, {ID: 0x50079A1C, Level: 10}, {ID: 0xDC584F60, Level: 5},
	}
	base.WeaponInfo.InnateTraits = []logsLoadoutTrait{
		{ID: 0x7EDD69D0, Level: 25}, {ID: 0x7EDD69D0, Level: 25}, {ID: 0x7EDD69D0, Level: 25},
		{ID: 0x7EDD69D0, Level: 25}, {ID: 0x7EDD69D0, Level: 25}, {ID: 0x7EDD69D0, Level: 25},
	}
	candidate, err := logsPlayerLoadoutShare(1, &base)
	if err != nil {
		t.Fatal(err)
	}
	if containsString(candidate.CapturedFields, "wrightstone") || containsString(candidate.CapturedFields, "weaponSkills") || len(candidate.Warnings) < 2 {
		t.Fatalf("oversized weapon subscopes became writable: %+v", candidate)
	}

	unknownStone := base
	unknownStone.WeaponInfo = &logsLoadoutWeaponInfo{
		WeaponID: 0x02352554, WrightstoneID: 0xDEADC0DE,
		WrightstoneTraits: []logsLoadoutTrait{{ID: 0x50079A1C, Level: 20}},
	}
	candidate, err = logsPlayerLoadoutShare(1, &unknownStone)
	if err != nil {
		t.Fatal(err)
	}
	if containsString(candidate.CapturedFields, "wrightstone") || candidate.Preview.Weapon.WrightstoneID != 0xDEADC0DE || len(candidate.Warnings) == 0 {
		t.Fatalf("unknown wrightstone became writable or disappeared from preview: %+v", candidate)
	}
}

func TestRelinkLogsExpandedProtocolNormalizesAllAvailableLoadoutScopes(t *testing.T) {
	player := &logsLoadoutPlayer{
		DisplayName: "Expanded", CharacterName: "Zeta", CharacterType: logsCharacterType{Code: "Pl1600"},
		Sigils: []logsLoadoutSigil{{
			FirstTraitID: 0x50079A1C, FirstTraitLevel: 15, SecondTraitID: 0xDC584F60, SecondTraitLevel: 15,
			SigilID: 0x2D7F2E70, SigilLevel: 15,
		}},
		Summons: []logsLoadoutSummon{
			{SummonID: 0x3FD89C3A, MainTraitID: 0x50079A1C, MainTraitLevel: 1, BonusID: 0x5A39D81B, BonusLevel: 0},
			{SummonID: 0x3FD89C3A, MainTraitID: 0x50079A1C, MainTraitLevel: 1, BonusID: 0x5A39D81B, BonusLevel: 0},
			{SummonID: 0x3FD89C3A, MainTraitID: 0x50079A1C, MainTraitLevel: 1, BonusID: 0x5A39D81B, BonusLevel: 0},
			{SummonID: 0x3FD89C3A, MainTraitID: 0x50079A1C, MainTraitLevel: 1, BonusID: 0x5A39D81B, BonusLevel: 0},
		},
		Abilities:   []uint32{0x95E40E12, 0x67270513, 0x974342AA, 0x52E3EED1},
		MasterLevel: 55,
		Skillboard:  []uint32{0x01EE7C0A},
		Stats:       &logsLoadoutRecordStats{Level: 100, HP: 20000, Attack: 30000, StunPower: 300, CriticalRate: 100, Power: 40000},
		WeaponState: &logsLoadoutWeaponState{
			WeaponID: 0x02352554, EXP: 1234, StarLevel: 6, PlusMarks: 99, AwakeningLevel: 10, WrightstoneID: 0x09E6F629,
			WrightstoneTraits: []logsLoadoutTrait{{ID: 0x50079A1C, Level: 20}},
			InnateTraits:      []logsLoadoutTrait{{ID: 0x7EDD69D0, Level: 25}},
		},
		OvermasteryInfo: &logsLoadoutOvermasteryInfo{Overmasteries: []logsLoadoutOvermastery{{ID: 0x52A207B5, Flags: 0x200, Value: 20}}},
	}
	path := createLogsTestDatabase(t, 1, encodeLogsTestEncounter(t, logsLoadoutEncounter{PlayerData: [4]*logsLoadoutPlayer{player}}))
	candidates, err := readLogsLoadoutShares(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expanded candidates=%d", len(candidates))
	}
	candidate := candidates[0]
	for _, field := range []string{"stats", "sigils", "skills", "summons", "weapon", "weaponSkills", "wrightstone", "mastery", "overLimit"} {
		if !containsString(candidate.CapturedFields, field) {
			t.Fatalf("expanded protocol missed %s: %+v", field, candidate.CapturedFields)
		}
	}
	if len(candidate.MissingFields) != 0 || len(candidate.Warnings) != 0 {
		t.Fatalf("expanded protocol metadata=%+v", candidate)
	}
	preview := candidate.Preview
	if preview.MasterLevel != 55 || !preview.MasteryAvailable || len(preview.Mastery) != 1 || len(preview.Abilities) != 4 || len(preview.Summons) != 4 {
		t.Fatalf("expanded preview lost fields: %+v", preview)
	}
	if preview.Stats.CriticalRate != 100 || len(preview.Weapon.Skills) != 1 || len(preview.Weapon.Traits) != 1 || len(preview.CombinedSkills) == 0 {
		t.Fatalf("expanded preview values=%+v", preview)
	}
	share, err := decodeLoadoutShareCode(candidate.CompatibilityCode)
	if err != nil {
		t.Fatal(err)
	}
	if len(share.Skills) != 4 || len(share.Summons) != 4 || len(share.MasteryHashes) != 50 || share.MasteryHashes[0] != "01EE7C0A" || len(share.WeaponSkillHashes) != 5 {
		t.Fatalf("expanded share=%+v", share)
	}
}

func TestUnknownCharacterEnumDoesNotHideOtherPartyMembers(t *testing.T) {
	unknown := &logsLoadoutPlayer{
		CharacterType: logsCharacterType{UnknownHash: 0xDEADBEEF},
		Sigils:        []logsLoadoutSigil{{SigilID: 0x2D7F2E70, FirstTraitID: 0x50079A1C}},
	}
	known := &logsLoadoutPlayer{
		CharacterType: logsCharacterType{Code: "Pl1600"},
		Sigils:        []logsLoadoutSigil{{SigilID: 0x2D7F2E70, SigilLevel: 15, FirstTraitID: 0x50079A1C, FirstTraitLevel: 15}},
	}
	path := createLogsTestDatabase(t, 1, encodeLogsTestEncounter(t, logsLoadoutEncounter{PlayerData: [4]*logsLoadoutPlayer{unknown, known}}))
	candidates, err := readLogsLoadoutShares(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].OwnerCode != "PL1600" {
		t.Fatalf("known party member was lost: %+v", candidates)
	}
}

func TestReadLogsLoadoutSharesPreservesIdenticalBuildsPerPartyMember(t *testing.T) {
	makePlayer := func(name string) *logsLoadoutPlayer {
		return &logsLoadoutPlayer{
			DisplayName: name, CharacterType: logsCharacterType{Code: "Pl1600"},
			Sigils: []logsLoadoutSigil{{SigilID: 0x2D7F2E70, SigilLevel: 15, FirstTraitID: 0x50079A1C, FirstTraitLevel: 15}},
		}
	}
	encounter := logsLoadoutEncounter{PlayerData: [4]*logsLoadoutPlayer{makePlayer("Alpha"), makePlayer("Beta")}}
	path := createLogsTestDatabase(t, 1, encodeLogsTestEncounter(t, encounter))
	candidates, err := readLogsLoadoutShares(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 || candidates[0].PlayerName == candidates[1].PlayerName {
		t.Fatalf("identical party builds were collapsed: %+v", candidates)
	}
}

func TestLogsPlayerLoadoutAllowsCapturedScopesWithoutSigils(t *testing.T) {
	player := &logsLoadoutPlayer{
		DisplayName: "WeaponOnly", CharacterType: logsCharacterType{Code: "Pl1600"},
		WeaponInfo: &logsLoadoutWeaponInfo{WeaponID: 0x02352554, WeaponLevel: 150},
	}
	candidate, err := logsPlayerLoadoutShare(1, player)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.SigilCount != 0 || !containsString(candidate.CapturedFields, "weapon") || containsString(candidate.CapturedFields, "sigils") {
		t.Fatalf("weapon-only candidate metadata=%+v", candidate)
	}
	share, err := decodeLoadoutShareCode(candidate.CompatibilityCode)
	if err != nil {
		t.Fatal(err)
	}
	if len(share.Sigils) != 0 || share.Weapon == nil || !containsString(share.CapturedFields, "weapon") {
		t.Fatalf("weapon-only share=%+v", share)
	}
}

func TestLogsPlayerLoadoutDegradesMalformedOptionalScopes(t *testing.T) {
	player := &logsLoadoutPlayer{
		DisplayName: "Partial", CharacterType: logsCharacterType{Code: "Pl1600"},
		Sigils: []logsLoadoutSigil{{SigilID: 0x2D7F2E70, SigilLevel: 15, FirstTraitID: 0x50079A1C, FirstTraitLevel: 15}},
		WeaponInfo: &logsLoadoutWeaponInfo{
			WeaponID: 0x02352554, WrightstoneID: 0x09E6F629,
			WrightstoneTraits: []logsLoadoutTrait{{ID: 0xDEADBEEF, Level: 20}},
			InnateTraits:      []logsLoadoutTrait{{ID: 0xCAFEBABE, Level: 25}},
		},
		OvermasteryInfo: &logsLoadoutOvermasteryInfo{Overmasteries: []logsLoadoutOvermastery{{ID: 0xDEADBEEF, Flags: 3}}},
	}
	candidate, err := logsPlayerLoadoutShare(1, player)
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(candidate.CapturedFields, "sigils") || !containsString(candidate.CapturedFields, "weapon") ||
		containsString(candidate.CapturedFields, "wrightstone") || containsString(candidate.CapturedFields, "weaponSkills") ||
		containsString(candidate.CapturedFields, "overLimit") || len(candidate.Warnings) < 3 {
		t.Fatalf("malformed scopes were not isolated: %+v", candidate)
	}
	share, err := decodeLoadoutShareCode(candidate.CompatibilityCode)
	if err != nil {
		t.Fatal(err)
	}
	if share.Weapon == nil || share.Weapon.Wrightstone != nil || len(share.Weapon.SkillHashes) != 0 || len(share.OverLimit) != 0 {
		t.Fatalf("malformed optional scopes leaked into share: %+v", share)
	}
}

func TestLogsImporterPreservesIdenticalLoadoutsAcrossPlayersAndEncounters(t *testing.T) {
	first := &logsLoadoutPlayer{
		ActorIndex: 1, DisplayName: "First", CharacterType: logsCharacterType{Code: "Pl1600"},
		Sigils: []logsLoadoutSigil{{SigilID: 0x2D7F2E70, SigilLevel: 15, FirstTraitID: 0x50079A1C, FirstTraitLevel: 15}},
	}
	second := *first
	second.ActorIndex = 2
	second.DisplayName = "Second"
	blob := encodeLogsTestEncounter(t, logsLoadoutEncounter{PlayerData: [4]*logsLoadoutPlayer{first, &second}})
	path := createLogsTestDatabase(t, 1, blob)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO logs (name, time, duration, data, version) VALUES (?, ?, ?, ?, ?)`, "fixture-2", 124, 1000, blob, 1)
	if closeErr := db.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := readLogsLoadoutShares(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 4 {
		t.Fatalf("identical loadouts were collapsed across players or encounters: %+v", candidates)
	}
}

func TestExpandedLogsRejectsMasteryFromAnotherCharacter(t *testing.T) {
	loadSkillboard()
	var foreignHash uint32
	for _, node := range skillboardAllNodes {
		if node.Char == "" || node.Char == "PL1600" {
			continue
		}
		hash, err := ParseHashHex(node.Hash)
		if err == nil {
			if _, _, ranked := masteryRankOfGrp(node.Grp); ranked {
				foreignHash = hash
				break
			}
		}
	}
	if foreignHash == 0 {
		t.Fatal("fixture could not find a foreign mastery node")
	}
	player := &logsLoadoutPlayer{
		CharacterType: logsCharacterType{Code: "Pl1600"},
		Sigils:        []logsLoadoutSigil{{SigilID: 0x2D7F2E70, SigilLevel: 15, FirstTraitID: 0x50079A1C, FirstTraitLevel: 15}},
		Skillboard:    []uint32{foreignHash},
	}
	candidate, err := logsPlayerLoadoutShare(1, player)
	if err != nil {
		t.Fatal(err)
	}
	if containsString(candidate.CapturedFields, "mastery") || candidate.Preview.MasteryAvailable || len(candidate.Warnings) == 0 {
		t.Fatalf("foreign mastery was treated as deployable: %+v", candidate)
	}
	share, err := decodeLoadoutShareCode(candidate.CompatibilityCode)
	if err != nil {
		t.Fatal(err)
	}
	if len(share.MasteryHashes) != 0 {
		t.Fatalf("foreign mastery leaked into share code: %+v", share.MasteryHashes)
	}
}

func TestLogsDatabaseRejectsMalformedSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unrelated.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE logs (time INTEGER, data BLOB)`)
	_ = db.Close()
	if err != nil {
		t.Fatal(err)
	}
	_, err = readLogsLoadoutShares(path)
	if err == nil || !strings.Contains(err.Error(), "缺少 version") {
		t.Fatalf("malformed schema error=%v", err)
	}
}

func TestLogsDatabaseReportsUnsupportedProtocolVersions(t *testing.T) {
	path := createLogsTestDatabase(t, 7, []byte{1, 2, 3})
	_, err := readLogsLoadoutShares(path)
	if err == nil || !strings.Contains(err.Error(), "实际版本：7") {
		t.Fatalf("unsupported version error=%v", err)
	}
}

func TestReadLogsLoadoutSharesEscapesDatabaseURICharacters(t *testing.T) {
	player := &logsLoadoutPlayer{
		CharacterType: logsCharacterType{Code: "Pl1600"},
		Sigils: []logsLoadoutSigil{{
			SigilID: 0x2D7F2E70, SigilLevel: 15, FirstTraitID: 0x50079A1C, FirstTraitLevel: 15,
		}},
	}
	original := createLogsTestDatabase(t, 1, encodeLogsTestEncounter(t, logsLoadoutEncounter{PlayerData: [4]*logsLoadoutPlayer{player}}))
	escaped := filepath.Join(filepath.Dir(original), "party #1 logs.db")
	if err := os.Rename(original, escaped); err != nil {
		t.Fatal(err)
	}
	candidates, err := readLogsLoadoutShares(escaped)
	if err != nil {
		t.Fatalf("read escaped database path: %v", err)
	}
	if len(candidates) != 1 || candidates[0].OwnerCode != "PL1600" {
		t.Fatalf("escaped path candidates=%+v", candidates)
	}
}

func TestParseLogsLoadoutJSONAcceptsCopiedPlayerData(t *testing.T) {
	candidates, err := parseLogsLoadoutJSON([]byte(relinkLogsPlayerJSON))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates=%d, want 1", len(candidates))
	}
	candidate := candidates[0]
	if candidate.OwnerCode != "PL1600" || candidate.PlayerName != "Clipboard Player" || candidate.SigilCount != 1 {
		t.Fatalf("JSON candidate identity=%+v", candidate)
	}
	if !strings.Contains(candidate.ProtocolLabel, "Relink Logs") || !strings.Contains(candidate.ProtocolLabel, "JSON") || candidate.Preview == nil || candidate.Preview.Evidence != candidate.ProtocolLabel {
		t.Fatalf("JSON protocol metadata=%+v preview=%+v", candidate, candidate.Preview)
	}
	for _, field := range []string{"stats", "sigils", "skills", "summons", "weapon", "weaponSkills", "wrightstone", "mastery", "overLimit"} {
		if !containsString(candidate.CapturedFields, field) {
			t.Fatalf("JSON capture missed %s: %+v", field, candidate.CapturedFields)
		}
	}
}

func TestParseLogsLoadoutJSONAcceptsPlayerArrayAndEncounterWrapper(t *testing.T) {
	for name, payload := range map[string]string{
		"array":   `[ ` + relinkLogsPlayerJSON + `, ` + relinkLogsPlayerJSON + ` ]`,
		"wrapper": `{"playerData":[` + relinkLogsPlayerJSON + `]}`,
	} {
		t.Run(name, func(t *testing.T) {
			candidates, err := parseLogsLoadoutJSON([]byte(payload))
			if err != nil {
				t.Fatal(err)
			}
			want := 1
			if name == "array" {
				want = 2
			}
			if len(candidates) != want {
				t.Fatalf("candidates=%d, want %d", len(candidates), want)
			}
		})
	}
}

func TestParseLogsLoadoutJSONReportsSkippedArrayMembers(t *testing.T) {
	unknown := strings.Replace(relinkLogsPlayerJSON, `"characterType": "Pl1600"`, `"characterType": {"Unknown": 3735928559}`, 1)
	candidates, err := parseLogsLoadoutJSON([]byte(`[` + unknown + `,` + relinkLogsPlayerJSON + `]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || len(candidates[0].Warnings) == 0 || !strings.Contains(candidates[0].Warnings[len(candidates[0].Warnings)-1], "3735928559") && !strings.Contains(candidates[0].Warnings[len(candidates[0].Warnings)-1], "DEADBEEF") {
		t.Fatalf("partial JSON import did not report the skipped member: %+v", candidates)
	}
}

func TestLogsCharacterTypeJSONAcceptsStringAndUnknownVariant(t *testing.T) {
	known := strings.Replace(relinkLogsPlayerJSON, `"characterType": "Pl1600"`, `"characterType": "Pl1600"`, 1)
	if candidates, err := parseLogsLoadoutJSON([]byte(known)); err != nil || len(candidates) != 1 {
		t.Fatalf("string characterType: candidates=%d err=%v", len(candidates), err)
	}
	unknown := strings.Replace(relinkLogsPlayerJSON, `"characterType": "Pl1600"`, `"characterType": {"Unknown": 3735928559}`, 1)
	if _, err := parseLogsLoadoutJSON([]byte(unknown)); err == nil || !strings.Contains(err.Error(), "无法映射") {
		t.Fatalf("unknown characterType error=%v", err)
	}
}

func TestParseLogsLoadoutJSONAcceptsWeaponOnlyAndDegradesUnknownOptionalData(t *testing.T) {
	weaponOnly := `{"characterType":"Pl1600","weaponState":{"weaponId":37037396,"wrightstoneId":166131241,"wrightstoneTraits":[{"id":3735928559,"level":20}],"innateTraits":[]}}`
	candidates, err := parseLogsLoadoutJSON([]byte(weaponOnly))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || !containsString(candidates[0].CapturedFields, "weapon") || containsString(candidates[0].CapturedFields, "wrightstone") || len(candidates[0].Warnings) == 0 {
		t.Fatalf("weapon-only JSON candidate=%+v", candidates)
	}
}

func TestParseLogsLoadoutJSONRejectsUnsafeOrUselessPayloads(t *testing.T) {
	tooManyPlayers := "[" + strings.TrimSuffix(strings.Repeat(relinkLogsPlayerJSON+",", logsLoadoutJSONMaximumPlayers+1), ",") + "]"
	tooManySigils := fmt.Sprintf(`{"characterType":"Pl1600","sigils":[%s]}`,
		strings.TrimSuffix(strings.Repeat(`{"sigilId":763309680,"firstTraitId":1342675484},`, loadoutMaxSigils+1), ","))
	for name, payload := range map[string][]byte{
		"empty":            nil,
		"malformed":        []byte(`{"characterType":`),
		"trailing":         []byte(relinkLogsPlayerJSON + ` {}`),
		"oversized":        []byte(strings.Repeat(" ", logsLoadoutJSONMaximumBytes+1)),
		"too many players": []byte(tooManyPlayers),
		"too many sigils":  []byte(tooManySigils),
		"no deployable":    []byte(`{"characterType":"Pl1600","stats":{"level":100}}`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseLogsLoadoutJSON(payload); err == nil {
				t.Fatal("payload was accepted")
			}
		})
	}
}
