package backend

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fateEpisodeWritableFixture struct {
	work      string
	episode   string
	missionID uint32
}

func prepareFateEpisodeWritableFixture(t *testing.T) fateEpisodeWritableFixture {
	t.Helper()
	source := filepath.Join(defaultSaveGamesDir(), "SaveData3.dat")
	sourceData, err := os.ReadFile(source)
	if err != nil {
		t.Skipf("real SaveData3 fixture is unavailable: %v", err)
	}
	// Keep the real bytes but use a non-managed filename so the transaction
	// cannot create or rotate backups beside the user's live save slots.
	work := filepath.Join(t.TempDir(), "FateWriteFixture.dat")
	if err := os.WriteFile(work, sourceData, 0o644); err != nil {
		t.Fatal(err)
	}
	save, err := LoadSave(work)
	if err != nil {
		t.Fatal(err)
	}
	layout, err := inspectFateEpisodeLayout(save)
	if err != nil {
		t.Fatal(err)
	}
	const episodeKey = "FATE_PL0000_00"
	episode := layout.stateByHash[gbfrHash32(episodeKey)]
	if episode == nil {
		t.Fatalf("real save copy lacks %s", episodeKey)
	}
	episode.SetUint32(0)
	var missionID uint32
	for index := 0; index < fateMissionVectorLength; index++ {
		value, readErr := layout.missionIDs.Uint32At(index)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if value == 0 {
			continue
		}
		missionID = value
		if err := layout.missionStates.SetUint32At(index, 0); err != nil {
			t.Fatal(err)
		}
		break
	}
	if missionID == 0 {
		t.Fatal("real save copy has no Fate mission row")
	}
	if err := save.FixChecksums(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(work, save.data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		after, readErr := os.ReadFile(source)
		if readErr == nil && !bytes.Equal(after, sourceData) {
			t.Errorf("real source save was modified")
		}
	})
	return fateEpisodeWritableFixture{work: work, episode: episodeKey, missionID: missionID}
}

func fateEpisodeFixtureRequest(t *testing.T, fixture fateEpisodeWritableFixture) FateEpisodeFieldWriteRequest {
	t.Helper()
	snapshot, err := (&App{}).FateEpisodeEditableInspect(fixture.work)
	if err != nil {
		t.Fatal(err)
	}
	return FateEpisodeFieldWriteRequest{
		Path: fixture.work, ExpectedRevision: snapshot.Revision,
		Changes: []FateEpisodeFieldChange{
			{Field: fateEpisodeStateField, EpisodeKey: fixture.episode, ExpectedValue: 0, TargetValue: fateCompletedState},
			{Field: fateEpisodeMissionField, MissionID: fixture.missionID, ExpectedValue: 0, TargetValue: 1},
		},
	}
}

func TestFateEpisodeEditableInspectRealSaveCopyReturnsBoundedFields(t *testing.T) {
	fixture := prepareFateEpisodeWritableFixture(t)
	snapshot, err := (&App{}).FateEpisodeEditableInspect(fixture.work)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Revision) != 64 || snapshot.Path == "" {
		t.Fatalf("invalid editable snapshot identity: %+v", snapshot)
	}
	if snapshot.Status.Total != fateEpisodeCount || snapshot.Status.MissionTotal != fateMissionCount {
		t.Fatalf("unexpected Fate status: %+v", snapshot.Status)
	}
	if len(snapshot.Fields) != fateEpisodeMaxFieldChanges {
		t.Fatalf("editable fields=%d, want %d", len(snapshot.Fields), fateEpisodeMaxFieldChanges)
	}
	episodeFields, missionFields := 0, 0
	for _, field := range snapshot.Fields {
		switch field.Field {
		case fateEpisodeStateField:
			episodeFields++
			if len(field.AllowedTargetValues) != 1 || field.AllowedTargetValues[0] != fateCompletedState {
				t.Fatalf("episode field exposes unsafe targets: %+v", field)
			}
		case fateEpisodeMissionField:
			missionFields++
			if len(field.AllowedTargetValues) != 1 || field.AllowedTargetValues[0] != 1 {
				t.Fatalf("mission field exposes unsafe targets: %+v", field)
			}
		default:
			t.Fatalf("unexpected editable field: %+v", field)
		}
	}
	if episodeFields != fateEpisodeCount || missionFields != fateMissionCount {
		t.Fatalf("field split=%d/%d, want %d/%d", episodeFields, missionFields, fateEpisodeCount, fateMissionCount)
	}
	if snapshot.FieldWriteVerified || snapshot.GameEffectVerified || snapshot.RewardClaimVerified {
		t.Fatalf("read-only inspection must not claim write/effect evidence: %+v", snapshot)
	}
}

func TestWriteFateEpisodeFieldsRealSaveCopyBacksUpAndReadsEveryField(t *testing.T) {
	fixture := prepareFateEpisodeWritableFixture(t)
	request := fateEpisodeFixtureRequest(t, fixture)
	before, err := os.ReadFile(fixture.work)
	if err != nil {
		t.Fatal(err)
	}
	result, err := (&App{}).WriteFateEpisodeFields(request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Requested != 2 || result.Changed != 2 || result.Verified != 2 || len(result.Readback) != 2 {
		t.Fatalf("unexpected field transaction counts: %+v", result)
	}
	if result.BackupPath == "" {
		t.Fatal("field write did not report an automatic backup")
	}
	if _, err := os.Stat(result.BackupPath); err != nil {
		t.Fatalf("automatic backup is missing: %v", err)
	}
	timestamped, err := filepath.Glob(fixture.work + ".pre-edit-*.bak")
	if err != nil || len(timestamped) == 0 {
		t.Fatalf("timestamped pre-write backup is missing: %v / %v", timestamped, err)
	}
	backup, err := os.ReadFile(timestamped[len(timestamped)-1])
	if err != nil {
		t.Fatalf("read timestamped backup: %v", err)
	}
	if !bytes.Equal(backup, before) {
		t.Fatal("automatic backup does not match the exact pre-write save")
	}
	if result.Revision == request.ExpectedRevision || len(result.Revision) != 64 {
		t.Fatalf("written revision was not refreshed: %+v", result)
	}
	if !result.FieldWriteVerified || result.GameEffectVerified || result.RewardClaimVerified {
		t.Fatalf("result evidence boundary is wrong: %+v", result)
	}
	verified, err := (&App{}).FateEpisodeEditableInspect(fixture.work)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Revision != result.Revision {
		t.Fatalf("inspect revision=%s, result=%s", verified.Revision, result.Revision)
	}
	values := make(map[string]uint32)
	for _, field := range verified.Fields {
		if field.EpisodeKey == fixture.episode {
			values[fateEpisodeStateField] = field.CurrentValue
		}
		if field.MissionID == fixture.missionID {
			values[fateEpisodeMissionField] = field.CurrentValue
		}
	}
	if values[fateEpisodeStateField] != fateCompletedState || values[fateEpisodeMissionField] != 1 {
		t.Fatalf("field readback=%v", values)
	}
}

func TestWriteFateEpisodeFieldsRejectsStaleRevisionWithoutTouchingFile(t *testing.T) {
	fixture := prepareFateEpisodeWritableFixture(t)
	request := fateEpisodeFixtureRequest(t, fixture)
	save, err := LoadSave(fixture.work)
	if err != nil {
		t.Fatal(err)
	}
	layout, err := inspectFateEpisodeLayout(save)
	if err != nil {
		t.Fatal(err)
	}
	layout.stateByHash[gbfrHash32(fixture.episode)].SetUint32(10)
	if err := save.FixChecksums(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.work, save.data, 0o644); err != nil {
		t.Fatal(err)
	}
	externallyChanged := append([]byte(nil), save.data...)
	if _, err := (&App{}).WriteFateEpisodeFields(request); err == nil || !strings.Contains(err.Error(), "检查后已被") {
		t.Fatalf("stale revision was not rejected: %v", err)
	}
	after, err := os.ReadFile(fixture.work)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, externallyChanged) {
		t.Fatal("stale write rejection changed the externally updated save")
	}
}

func TestWriteFateEpisodeFieldsReadbackFailureRestoresOriginalCopy(t *testing.T) {
	fixture := prepareFateEpisodeWritableFixture(t)
	request := fateEpisodeFixtureRequest(t, fixture)
	before, err := os.ReadFile(fixture.work)
	if err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("forced Fate readback failure")
	if _, err := writeFateEpisodeFieldsLocked(request, func(string) (*SaveData, error) {
		return nil, sentinel
	}); !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "已自动恢复") {
		t.Fatalf("write error=%v, want restored sentinel failure", err)
	}
	after, err := os.ReadFile(fixture.work)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("readback failure did not restore the exact pre-write save")
	}
	backups, err := filepath.Glob(fixture.work + ".pre-edit-*.bak")
	if err != nil || len(backups) == 0 {
		t.Fatalf("failed transaction did not retain a backup: %v / %v", backups, err)
	}
}

func TestWriteFateEpisodeFieldsRejectsUnknownOrRegressiveValues(t *testing.T) {
	fixture := prepareFateEpisodeWritableFixture(t)
	request := fateEpisodeFixtureRequest(t, fixture)
	before, err := os.ReadFile(fixture.work)
	if err != nil {
		t.Fatal(err)
	}
	request.Changes = []FateEpisodeFieldChange{{
		Field: fateEpisodeStateField, EpisodeKey: fixture.episode,
		ExpectedValue: 0, TargetValue: fateCompletedState - 1,
	}}
	if _, err := (&App{}).WriteFateEpisodeFields(request); err == nil || !strings.Contains(err.Error(), "完成值") {
		t.Fatalf("unsafe episode value was not rejected: %v", err)
	}
	request.Changes = []FateEpisodeFieldChange{{
		Field: "unknown", EpisodeKey: fixture.episode, ExpectedValue: 0, TargetValue: fateCompletedState,
	}}
	if _, err := (&App{}).WriteFateEpisodeFields(request); err == nil || !strings.Contains(err.Error(), "未知") {
		t.Fatalf("unknown field was not rejected: %v", err)
	}
	after, err := os.ReadFile(fixture.work)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("rejected Fate changes modified the save copy")
	}
}

func TestWriteFateEpisodeFieldsDefaultManagedSaveRequiresGameExit(t *testing.T) {
	original := generatorFindProcessByName
	generatorFindProcessByName = func(string) (uint32, error) { return 1234, nil }
	t.Cleanup(func() { generatorFindProcessByName = original })

	request := FateEpisodeFieldWriteRequest{Path: filepath.Join(defaultSaveGamesDir(), "SaveData1.dat")}
	if _, err := (&App{}).WriteFateEpisodeFields(request); err == nil || !strings.Contains(err.Error(), "退出游戏") {
		t.Fatalf("managed-save write did not require the game to exit: %v", err)
	}
}
