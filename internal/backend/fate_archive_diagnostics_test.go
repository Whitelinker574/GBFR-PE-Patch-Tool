package backend

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestFateArchiveDiagnosticsIncludesFedielAndExactMissionState(t *testing.T) {
	fixture := prepareFateEpisodeWritableFixture(t)
	save, err := LoadSave(fixture.work)
	if err != nil {
		t.Fatal(err)
	}
	layout, err := inspectFateEpisodeLayout(save)
	if err != nil {
		t.Fatal(err)
	}
	index, _, err := missionStateIndex(layout, 0x00301029)
	if err != nil {
		t.Fatal(err)
	}
	if err := layout.missionStates.SetUint32At(index, 3); err != nil {
		t.Fatal(err)
	}
	if err := save.FixChecksums(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.work, save.data, 0o644); err != nil {
		t.Fatal(err)
	}
	before := append([]byte(nil), save.data...)
	raw, err := fateArchiveEvidenceFromPath(fixture.work, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var evidence FateEpisodeEvidenceExport
	if err := json.Unmarshal(raw, &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence.SchemaVersion != 2 || len(evidence.Missions) != 56 || len(evidence.UncheckedArchives) != 0 || len(evidence.StoryArchives) != 29 {
		t.Fatalf("incomplete evidence: schema %d, missions %d, unchecked %d", evidence.SchemaVersion, len(evidence.Missions), len(evidence.UncheckedArchives))
	}
	found := false
	for _, mission := range evidence.Missions {
		if mission.MissionCode == "00301029" {
			found = mission.State == 3 && mission.Completed
		}
	}
	if !found {
		t.Fatal("Fediel mission raw state lost")
	}
	found = false
	for _, row := range evidence.StoryArchives {
		if row.ArchiveID == "ARC_OTHER_069" {
			found = row.Recorded && row.RecordUnitID >= 0 && row.FinalEpisodeKeys[0] == "FATE_PL2900_10"
		}
	}
	if !found {
		t.Fatal("native Fediel archive record missing from evidence")
	}
	if bytes.Contains(raw, []byte(`"path"`)) || bytes.Contains(raw, []byte("FateWriteFixture.dat")) {
		t.Fatal("export leaked local path")
	}
	if len(evidence.ArchiveObservations) != 2 {
		t.Fatal("candidate observations absent")
	}
	for _, row := range evidence.ArchiveObservations {
		if !row.Available || len(row.Values) != 100 || (row.IDType != 7901 && row.IDType != 7902) {
			t.Fatalf("invalid observation: %d", row.IDType)
		}
	}
	after, err := os.ReadFile(fixture.work)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("diagnostic export modified save")
	}
}
