package backend

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runtimeLoadoutDetectorTestMembers() []RuntimeLoadoutDetectorMember {
	loadout := RuntimePatchPartyLoadout{
		Available: true, Stable: true, SnapshotCount: runtimePatchPartySnapshotCount,
		CharacterCode: "PL0400", CharacterHash: "4D0A60C3", CharacterName: "伊欧",
		Weapon:    RuntimePatchPartyWeapon{Hash: 0x12345678, HashHex: "12345678", Name: "测试武器", Level: 150},
		Sigils:    []RuntimePatchPartySigil{{Index: 0, Hash: 0x11111111, HashHex: "11111111", Name: "测试因子", Level: 15}},
		OverLimit: []RuntimePatchPartyOverLimit{{Index: 0}, {Index: 1}, {Index: 2}, {Index: 3}},
	}
	return []RuntimeLoadoutDetectorMember{{Role: "player", CharacterHash: loadout.CharacterHash, CharacterName: loadout.CharacterName, Loadout: loadout}}
}

func TestRuntimeLoadoutDetectorRecordsOneTeamPerTaskBoundary(t *testing.T) {
	members := runtimeLoadoutDetectorTestMembers()
	fingerprint, err := runtimeLoadoutDetectorFingerprint(members)
	if err != nil {
		t.Fatal(err)
	}
	session := &runtimeLoadoutDetectorSession{
		historyPath: filepath.Join(t.TempDir(), "history.json"), startedAt: time.Now(),
		sessionID: "test-session", state: "waiting_task", nextSequence: 1,
	}

	session.observeTeam(fingerprint, members)
	if len(session.history) != 0 || session.state != "stabilizing" {
		t.Fatalf("first observation was not held for stability: state=%s history=%d", session.state, len(session.history))
	}
	session.observeTeam(fingerprint, members)
	if len(session.history) != 1 || session.history[0].ID != "test-session-0001" {
		t.Fatalf("stable team was not recorded exactly once: %+v", session.history)
	}
	session.observeTeam(fingerprint, members)
	if len(session.history) != 1 {
		t.Fatalf("same active task was duplicated: %d", len(session.history))
	}

	session.observeAbsent()
	session.observeAbsent()
	session.observeTeam(fingerprint, members)
	session.observeTeam(fingerprint, members)
	if len(session.history) != 2 || session.history[1].ID != "test-session-0002" {
		t.Fatalf("same team after a task boundary was not recorded as a new task: %+v", session.history)
	}
}

func TestRuntimeLoadoutDetectorIgnoresTeamChangesUntilTaskBoundary(t *testing.T) {
	first := runtimeLoadoutDetectorTestMembers()
	second := runtimeLoadoutDetectorTestMembers()
	second[0].Loadout.Weapon.Hash = 0x87654321
	firstFingerprint, _ := runtimeLoadoutDetectorFingerprint(first)
	secondFingerprint, _ := runtimeLoadoutDetectorFingerprint(second)
	session := &runtimeLoadoutDetectorSession{
		historyPath: filepath.Join(t.TempDir(), "history.json"), sessionID: "test-session", nextSequence: 1,
	}
	session.observeTeam(firstFingerprint, first)
	session.observeTeam(firstFingerprint, first)
	session.observeTeam(secondFingerprint, second)
	session.observeTeam(secondFingerprint, second)
	if len(session.history) != 1 {
		t.Fatalf("one task was split into %d records after a team change", len(session.history))
	}
	session.observeAbsent()
	session.observeAbsent()
	session.observeTeam(secondFingerprint, second)
	session.observeTeam(secondFingerprint, second)
	if len(session.history) != 2 {
		t.Fatalf("new task after absence was not recorded: %+v", session.history)
	}
}

func TestRuntimeLoadoutDetectorHistoryPersistsOnlyLoadoutData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	records := []RuntimeLoadoutDetectorRecord{{
		ID: "session-0001", Sequence: 1, CapturedAt: time.Now().UnixMilli(), SessionID: "session",
		Members: runtimeLoadoutDetectorTestMembers(),
	}}
	const activeFingerprint = "active-team-fingerprint"
	if err := saveRuntimeLoadoutDetectorHistory(path, records, activeFingerprint); err != nil {
		t.Fatal(err)
	}
	loaded, next, loadedActiveFingerprint, err := loadRuntimeLoadoutDetectorHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || next != 2 || loadedActiveFingerprint != activeFingerprint || loaded[0].Members[0].CharacterName != "伊欧" {
		t.Fatalf("history round trip mismatch: next=%d records=%+v", next, loaded)
	}
	payload, err := json.Marshal(loaded)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(payload))
	for _, forbidden := range []string{"modulebase", "processcreated", "rootaddress", "\"address\"", "\"pid\""} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("persisted detector history leaked runtime identity field %q: %s", forbidden, text)
		}
	}
}

func TestRuntimeLoadoutDetectorHistoryKeepsOnlyNewestBoundedRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	records := make([]RuntimeLoadoutDetectorRecord, runtimeLoadoutDetectorMaximumHistory+5)
	for index := range records {
		records[index] = RuntimeLoadoutDetectorRecord{ID: fmt.Sprintf("record-%04d", index+1), Sequence: index + 1}
	}
	if err := saveRuntimeLoadoutDetectorHistory(path, records, ""); err != nil {
		t.Fatal(err)
	}
	loaded, next, _, err := loadRuntimeLoadoutDetectorHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != runtimeLoadoutDetectorMaximumHistory || loaded[0].Sequence != 6 || next != len(records)+1 {
		t.Fatalf("bounded history mismatch: len=%d first=%d next=%d", len(loaded), loaded[0].Sequence, next)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".runtime-loadout-history.json.tmp-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("atomic history save left temporary files: %v err=%v", matches, err)
	}
}

func TestRuntimeLoadoutDetectorHistoryRebuildsDerivedCombinedSkills(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	loadout := runtimeLoadoutDetectorTestMembers()[0].Loadout
	loadout.Weapon.Skills = []RuntimePatchPartyTrait{{Hash: 0x1E1CECCE, HashHex: "1E1CECCE", Name: "浩劫", Level: 35}}
	loadout.CombinedSkills = []TraitBonus{{TraitID: "1E1CECCE", Level: 35}}
	records := []RuntimeLoadoutDetectorRecord{{
		ID: "session-0001", Sequence: 1, CapturedAt: time.Now().UnixMilli(), SessionID: "session",
		Members: []RuntimeLoadoutDetectorMember{{Role: "player", CharacterHash: loadout.CharacterHash, CharacterName: loadout.CharacterName, Loadout: loadout}},
	}}
	if err := saveRuntimeLoadoutDetectorHistory(path, records, ""); err != nil {
		t.Fatal(err)
	}
	loaded, _, _, err := loadRuntimeLoadoutDetectorHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	combined := loaded[0].Members[0].Loadout.CombinedSkills
	if len(combined) != 1 || combined[0].TraitID != "SKILL_143_10" || combined[0].Name != "浩劫" || combined[0].Effect == "" {
		t.Fatalf("history kept stale derived skill data: %+v", combined)
	}
}

func TestRuntimeLoadoutDetectorStatusRestoresLatestCapture(t *testing.T) {
	records := []RuntimeLoadoutDetectorRecord{
		{CapturedAt: 1_700_000_000_000},
		{CapturedAt: 1_700_000_002_000},
		{CapturedAt: 0},
	}
	latest := runtimeLoadoutDetectorLastCapture(records)
	if got := runtimeLoadoutDetectorUnixMilli(latest); got != 1_700_000_002_000 {
		t.Fatalf("latest capture = %d, want %d", got, int64(1_700_000_002_000))
	}
	status := (&runtimeLoadoutDetectorSession{}).status()
	if status.LastPollAt != 0 || status.LastCaptureAt != 0 {
		t.Fatalf("unset detector times must stay zero: %+v", status)
	}
}

func TestRuntimeLoadoutDetectorRestartDoesNotDuplicateActiveTask(t *testing.T) {
	members := runtimeLoadoutDetectorTestMembers()
	fingerprint, err := runtimeLoadoutDetectorFingerprint(members)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "history.json")
	first := &runtimeLoadoutDetectorSession{historyPath: path, sessionID: "first", state: "waiting_task", nextSequence: 1}
	first.observeTeam(fingerprint, members)
	first.observeTeam(fingerprint, members)

	history, nextSequence, activeFingerprint, err := loadRuntimeLoadoutDetectorHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	restarted := &runtimeLoadoutDetectorSession{
		historyPath: path, history: history, sessionID: "restarted", state: "waiting_task",
		nextSequence: nextSequence, restoredFingerprint: activeFingerprint,
	}
	restarted.observeTeam(fingerprint, members)
	restarted.observeTeam(fingerprint, members)
	if len(restarted.history) != 1 {
		t.Fatalf("restart duplicated the active task: %+v", restarted.history)
	}
	restarted.observeAbsent()
	restarted.observeAbsent()
	restarted.observeTeam(fingerprint, members)
	restarted.observeTeam(fingerprint, members)
	if len(restarted.history) != 2 {
		t.Fatalf("confirmed task boundary did not allow the next identical team: %+v", restarted.history)
	}
}

func TestRuntimeLoadoutDetectorUpdatesIncompleteTeamWithinSameTask(t *testing.T) {
	partial := runtimeLoadoutDetectorTestMembers()
	complete := append(cloneRuntimeLoadoutDetectorMembers(partial), RuntimeLoadoutDetectorMember{
		Role: "party1", CharacterHash: "0D21B430", CharacterName: "泽塔", Loadout: partial[0].Loadout,
	})
	complete[1].Loadout.CharacterCode = "PL1600"
	complete[1].Loadout.CharacterHash = "0D21B430"
	complete[1].Loadout.CharacterName = "泽塔"
	partialFingerprint, _ := runtimeLoadoutDetectorFingerprint(partial)
	completeFingerprint, _ := runtimeLoadoutDetectorFingerprint(complete)
	session := &runtimeLoadoutDetectorSession{
		historyPath: filepath.Join(t.TempDir(), "history.json"), sessionID: "test-session", nextSequence: 1,
	}
	session.observeTeam(partialFingerprint, partial)
	session.observeTeam(partialFingerprint, partial)
	session.observeTeam(completeFingerprint, complete)
	session.observeTeam(completeFingerprint, complete)
	if len(session.history) != 1 || len(session.history[0].Members) != 2 || session.history[0].ID != "test-session-0001" {
		t.Fatalf("complete team did not replace the incomplete same-task record: %+v", session.history)
	}
}

func TestRuntimeLoadoutDetectorUpdatesIncompleteLoadoutWithoutTeamSizeChange(t *testing.T) {
	partial := runtimeLoadoutDetectorTestMembers()
	complete := cloneRuntimeLoadoutDetectorMembers(partial)
	complete[0].Loadout.Abilities = []RuntimePatchPartyAbility{{Hash: 0x95E40E12, HashHex: "95E40E12", Name: "测试技能"}}
	complete[0].Loadout.Summons = []RuntimePatchPartySummon{{Index: 0, TypeHash: 0x3FD89C3A, TypeHashHex: "3FD89C3A"}}
	complete[0].Loadout.MasteryAvailable = true
	complete[0].Loadout.Mastery = []LoadoutMasteryNode{{Hash: "01EE7C0A"}}
	complete[0].Loadout.Weapon.WrightstoneID = 0x09E6F629
	complete[0].Loadout.Weapon.Traits = []RuntimePatchPartyTrait{{Hash: 0x50079A1C, HashHex: "50079A1C", Level: 20}}
	partialFingerprint, _ := runtimeLoadoutDetectorFingerprint(partial)
	completeFingerprint, _ := runtimeLoadoutDetectorFingerprint(complete)
	session := &runtimeLoadoutDetectorSession{
		historyPath: filepath.Join(t.TempDir(), "history.json"), sessionID: "test-session", nextSequence: 1,
	}
	session.observeTeam(partialFingerprint, partial)
	session.observeTeam(partialFingerprint, partial)
	session.observeTeam(completeFingerprint, complete)
	session.observeTeam(completeFingerprint, complete)
	if len(session.history) != 1 {
		t.Fatalf("same task was split after loadout scopes became available: %+v", session.history)
	}
	loadout := session.history[0].Members[0].Loadout
	if len(loadout.Abilities) != 1 || len(loadout.Summons) != 1 || !loadout.MasteryAvailable || len(loadout.Mastery) != 1 || len(loadout.Weapon.Traits) != 1 {
		t.Fatalf("same-size team kept the incomplete loadout: %+v", loadout)
	}
}

func TestRuntimeLoadoutDetectorDoesNotReplaceChangedWeaponWhenAnotherScopeGrows(t *testing.T) {
	first := runtimeLoadoutDetectorTestMembers()
	changed := cloneRuntimeLoadoutDetectorMembers(first)
	changed[0].Loadout.Weapon.Hash = 0x87654321
	changed[0].Loadout.Summons = []RuntimePatchPartySummon{{Index: 0, TypeHash: 0x3FD89C3A, TypeHashHex: "3FD89C3A"}}
	firstFingerprint, _ := runtimeLoadoutDetectorFingerprint(first)
	changedFingerprint, _ := runtimeLoadoutDetectorFingerprint(changed)
	session := &runtimeLoadoutDetectorSession{
		historyPath: filepath.Join(t.TempDir(), "history.json"), sessionID: "test-session", nextSequence: 1,
	}
	session.observeTeam(firstFingerprint, first)
	session.observeTeam(firstFingerprint, first)
	session.observeTeam(changedFingerprint, changed)
	session.observeTeam(changedFingerprint, changed)
	if len(session.history) != 1 || session.history[0].Members[0].Loadout.Weapon.Hash != first[0].Loadout.Weapon.Hash {
		t.Fatalf("changed weapon overwrote the active task while another scope grew: %+v", session.history)
	}
}

func TestRuntimeLoadoutDetectorDoesNotReplaceChangedCharacterWhenAnotherScopeGrows(t *testing.T) {
	first := runtimeLoadoutDetectorTestMembers()
	changed := cloneRuntimeLoadoutDetectorMembers(first)
	changed[0].CharacterHash = "0D21B430"
	changed[0].CharacterName = "泽塔"
	changed[0].Loadout.CharacterCode = "PL1600"
	changed[0].Loadout.CharacterHash = "0D21B430"
	changed[0].Loadout.CharacterName = "泽塔"
	changed[0].Loadout.Abilities = []RuntimePatchPartyAbility{{Hash: 0x95E40E12, HashHex: "95E40E12"}}
	firstFingerprint, _ := runtimeLoadoutDetectorFingerprint(first)
	changedFingerprint, _ := runtimeLoadoutDetectorFingerprint(changed)
	session := &runtimeLoadoutDetectorSession{
		historyPath: filepath.Join(t.TempDir(), "history.json"), sessionID: "test-session", nextSequence: 1,
	}
	session.observeTeam(firstFingerprint, first)
	session.observeTeam(firstFingerprint, first)
	session.observeTeam(changedFingerprint, changed)
	session.observeTeam(changedFingerprint, changed)
	if len(session.history) != 1 || session.history[0].Members[0].Loadout.CharacterCode != "PL0400" {
		t.Fatalf("changed character overwrote the active task while another scope grew: %+v", session.history)
	}
}

func TestRuntimeLoadoutDetectorDoesNotReplaceLowerLevelsWhenAnotherScopeGrows(t *testing.T) {
	first := runtimeLoadoutDetectorTestMembers()
	first[0].Loadout.Weapon.Skills = []RuntimePatchPartyTrait{{Hash: 0x50079A1C, HashHex: "50079A1C", Level: 15}}
	first[0].Loadout.Sigils[0].PrimaryTraitHash = 0x50079A1C
	first[0].Loadout.Sigils[0].PrimaryTraitLevel = 15
	changed := cloneRuntimeLoadoutDetectorMembers(first)
	changed[0].Loadout.Weapon.Skills[0].Level = 1
	changed[0].Loadout.Sigils[0].PrimaryTraitLevel = 1
	changed[0].Loadout.Abilities = []RuntimePatchPartyAbility{{Hash: 0x95E40E12, HashHex: "95E40E12"}}
	firstFingerprint, _ := runtimeLoadoutDetectorFingerprint(first)
	changedFingerprint, _ := runtimeLoadoutDetectorFingerprint(changed)
	session := &runtimeLoadoutDetectorSession{
		historyPath: filepath.Join(t.TempDir(), "history.json"), sessionID: "test-session", nextSequence: 1,
	}
	session.observeTeam(firstFingerprint, first)
	session.observeTeam(firstFingerprint, first)
	session.observeTeam(changedFingerprint, changed)
	session.observeTeam(changedFingerprint, changed)
	loadout := session.history[0].Members[0].Loadout
	if len(session.history) != 1 || loadout.Weapon.Skills[0].Level != 15 || loadout.Sigils[0].PrimaryTraitLevel != 15 {
		t.Fatalf("lower scalar levels overwrote the active task while another scope grew: %+v", session.history)
	}
}

func TestRuntimeLoadoutDetectorRestartAllowsDifferentStableTask(t *testing.T) {
	first := runtimeLoadoutDetectorTestMembers()
	second := runtimeLoadoutDetectorTestMembers()
	second[0].Loadout.Weapon.Hash = 0x87654321
	firstFingerprint, _ := runtimeLoadoutDetectorFingerprint(first)
	secondFingerprint, _ := runtimeLoadoutDetectorFingerprint(second)
	path := filepath.Join(t.TempDir(), "history.json")
	initial := &runtimeLoadoutDetectorSession{historyPath: path, sessionID: "first", nextSequence: 1}
	initial.observeTeam(firstFingerprint, first)
	initial.observeTeam(firstFingerprint, first)
	history, nextSequence, restoredFingerprint, err := loadRuntimeLoadoutDetectorHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	restarted := &runtimeLoadoutDetectorSession{
		historyPath: path, history: history, sessionID: "restarted", nextSequence: nextSequence,
		restoredFingerprint: restoredFingerprint,
	}
	restarted.observeTeam(secondFingerprint, second)
	restarted.observeTeam(secondFingerprint, second)
	if len(restarted.history) != 2 || restarted.history[1].ID != "restarted-0002" {
		t.Fatalf("different stable task after restart was not recorded: %+v", restarted.history)
	}
}

func TestRuntimeLoadoutDetectorMembersDropsAddressesAndUnavailableSlots(t *testing.T) {
	loadout := runtimeLoadoutDetectorTestMembers()[0].Loadout
	teammate := loadout
	teammate.CharacterCode = "PL1600"
	teammate.CharacterHash = "0D21B430"
	teammate.CharacterName = "泽塔"
	snapshot := RuntimePatchPartyMonitor{Entities: []RuntimePatchPartyEntity{
		{Role: "player", Present: true, Address: 0x12345678, Loadout: &loadout},
		{Role: "party1", Present: true, Address: 0x87654321, Loadout: &teammate},
		{Role: "party2", Present: true, Address: 0x99999999, Loadout: unavailableRuntimePatchPartyLoadout(nil)},
		{Role: "companion", Present: true, Address: 0x22222222},
	}}
	members := runtimeLoadoutDetectorMembers(snapshot)
	if len(members) != 1 || members[0].Role != "party1" || members[0].CharacterName != "泽塔" || !members[0].Loadout.Stable {
		t.Fatalf("detector member sanitization mismatch: %+v", members)
	}
}
