package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFateEpisodeEvidenceExportOmitsLocalSavePath(t *testing.T) {
	status := &FateEpisodeStatus{
		Path: "C:\\Users\\someone\\AppData\\Local\\GBFR\\SaveData1.dat", DataVersion: "2.0.2",
		Completed: 1, Total: 319, MissionCompleted: 2, MissionTotal: 56, AuxiliaryPreserved: 5,
		Characters: []FateEpisodeCharacterStatus{{Code: "PL0400", Completed: 1, Total: 11, Episodes: []FateEpisodeEntryStatus{{Key: "FATE_PL0400_00", TitleZh: "测试", RequiredLevel: 1, Completed: true}}}},
	}
	data, err := fateEpisodeEvidenceJSON(status, time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "someone") || strings.Contains(string(data), `"path"`) {
		t.Fatalf("evidence export leaked the local path: %s", data)
	}
	var decoded FateEpisodeEvidenceExport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SchemaVersion != 1 || decoded.GeneratedAt != "2026-07-27T00:00:00Z" || len(decoded.Characters) != 1 {
		t.Fatalf("unexpected evidence export: %+v", decoded)
	}
}

func TestGBFRHash32KnownFateKeys(t *testing.T) {
	tests := map[string]uint32{
		"FATE_PL0000_00": 0x45FC88BE,
		"FATE_PL0000_04": 0x7681A900,
		"FATE_PL2900_10": 0xAC4CC068,
	}
	for text, want := range tests {
		if got := gbfrHash32(text); got != want {
			t.Fatalf("gbfrHash32(%q)=%08X, want %08X", text, got, want)
		}
	}
}

func TestFateHashCatalogIsCompleteAndUnique(t *testing.T) {
	byCharacter := fateHashesByCharacter()
	if len(byCharacter) != 29 {
		t.Fatalf("character count=%d, want 29", len(byCharacter))
	}
	seen := make(map[uint32]string, fateEpisodeCount)
	for code, hashes := range byCharacter {
		if len(hashes) != 11 {
			t.Fatalf("%s has %d hashes, want 11", code, len(hashes))
		}
		for _, hash := range hashes {
			if previous, exists := seen[hash]; exists {
				t.Fatalf("hash %08X is shared by %s and %s", hash, previous, code)
			}
			seen[hash] = code
		}
	}
	if len(seen) != fateEpisodeCount || len(sortedFateHashes()) != fateEpisodeCount {
		t.Fatalf("fate hash count=%d/%d, want %d", len(seen), len(sortedFateHashes()), fateEpisodeCount)
	}
}

func TestFateEpisodeCatalogMatchesUnpacked202Tables(t *testing.T) {
	catalog, err := loadFateEpisodeCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if catalog.DataVersion != "2.0.2" || len(catalog.Episodes) != fateEpisodeCount {
		t.Fatalf("catalog version/count=%s/%d", catalog.DataVersion, len(catalog.Episodes))
	}
	byKey := make(map[string]fateEpisodeCatalogEntry, len(catalog.Episodes))
	bonusRows := 0
	for _, episode := range catalog.Episodes {
		byKey[episode.Key] = episode
		if episode.StaticBonus != nil {
			bonusRows++
		}
	}
	if bonusRows != 29*9 {
		t.Fatalf("static Fate bonus rows=%d, want %d", bonusRows, 29*9)
	}
	checks := map[string][2]string{
		"FATE_PL0000_00": {"与露莉亚的邂逅", "Her Name Is Lyria"},
		"FATE_PL2900_10": {"致后世的夙愿", "Axiom of Our Provenance"},
	}
	for key, names := range checks {
		episode, ok := byKey[key]
		if !ok || episode.TitleZh != names[0] || episode.TitleEn != names[1] {
			t.Fatalf("catalog %s=%+v, want %q/%q", key, episode, names[0], names[1])
		}
	}
}

func TestFateAuxiliaryAndMissionCatalogsAreCompleteAndUnique(t *testing.T) {
	auxiliary := make(map[uint32]struct{}, len(fateAuxiliaryHashes))
	for _, hash := range fateAuxiliaryHashes {
		if _, duplicate := auxiliary[hash]; duplicate {
			t.Fatalf("duplicate REMI hash %08X", hash)
		}
		auxiliary[hash] = struct{}{}
	}
	if len(auxiliary) != fateAuxiliaryCount {
		t.Fatalf("REMI hash count=%d, want %d", len(auxiliary), fateAuxiliaryCount)
	}
	if missions := fateMissionIDs(); len(missions) != fateMissionCount {
		t.Fatalf("mission catalog count=%d, want %d", len(missions), fateMissionCount)
	}
}

func TestFateEpisodeInspectRealSaveCopyIsReadOnly(t *testing.T) {
	source := filepath.Join(defaultSaveGamesDir(), "SaveData3.dat")
	if _, err := os.Stat(source); err != nil {
		t.Skipf("real SaveData3 fixture is unavailable: %v", err)
	}
	dir := t.TempDir()
	work := filepath.Join(dir, "SaveData3.dat")
	input, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(work, input, 0o644); err != nil {
		t.Fatal(err)
	}

	app := &App{}
	beforeBytes, err := os.ReadFile(work)
	if err != nil {
		t.Fatal(err)
	}
	status, err := app.FateEpisodeInspect(work)
	if err != nil {
		t.Fatalf("inspect real save: %v", err)
	}
	if status.Total != fateEpisodeCount || status.MissionTotal != fateMissionCount || status.AuxiliaryPreserved != fateAuxiliaryCount {
		t.Fatalf("unexpected catalog status: %+v", status)
	}
	if status.DataVersion != "2.0.2" || len(status.Characters) != len(fateCharacterCodes) {
		t.Fatalf("unexpected detailed Fate status: %+v", status)
	}
	for _, character := range status.Characters {
		if len(character.Episodes) != 11 {
			t.Fatalf("%s detailed episodes=%d, want 11", character.Code, len(character.Episodes))
		}
		completed := 0
		for _, episode := range character.Episodes {
			if episode.Completed {
				completed++
			}
			if episode.TitleZh == "" || episode.TitleEn == "" || episode.Hash == "" {
				t.Fatalf("%s has incomplete episode metadata: %+v", character.Code, episode)
			}
		}
		if completed != character.Completed {
			t.Fatalf("%s detailed completed=%d, summary=%d", character.Code, completed, character.Completed)
		}
	}
	afterBytes, err := os.ReadFile(work)
	if err != nil {
		t.Fatal(err)
	}
	if string(beforeBytes) != string(afterBytes) {
		t.Fatal("read-only inspection changed the save copy")
	}
}

func TestFateEpisodeInspectAllLocalSaveVariants(t *testing.T) {
	dir := defaultSaveGamesDir()
	paths, err := filepath.Glob(filepath.Join(dir, "SaveData*.dat"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Skip("local save matrix is unavailable")
	}
	app := &App{}
	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			status, err := app.FateEpisodeInspect(path)
			if err != nil {
				t.Fatalf("inspect %s: %v", path, err)
			}
			if status.Total != fateEpisodeCount || status.MissionTotal != fateMissionCount || status.AuxiliaryPreserved != fateAuxiliaryCount || len(status.Characters) != len(fateCharacterCodes) {
				t.Fatalf("unexpected catalog status: %+v", status)
			}
		})
	}
}

func TestCompleteAllFateEpisodesRealSaveCopyWritesOnlyValidatedLayout(t *testing.T) {
	source := filepath.Join(defaultSaveGamesDir(), "SaveData3.dat")
	input, err := os.ReadFile(source)
	if err != nil {
		t.Skipf("real SaveData3 fixture is unavailable: %v", err)
	}
	work := filepath.Join(t.TempDir(), "SaveData3.dat")
	if err := os.WriteFile(work, input, 0o644); err != nil {
		t.Fatal(err)
	}

	app := &App{}
	before, err := app.FateEpisodeInspect(work)
	if err != nil {
		t.Fatalf("inspect before write: %v", err)
	}
	result, err := completeAllFateEpisodesLocked(work)
	if err != nil {
		t.Fatalf("complete all Fate episodes: %v", err)
	}
	if result.EpisodesChanged != before.Total-before.Completed || result.MissionsChanged != before.MissionTotal-before.MissionCompleted {
		t.Fatalf("changed counts=%d/%d, want %d/%d", result.EpisodesChanged, result.MissionsChanged, before.Total-before.Completed, before.MissionTotal-before.MissionCompleted)
	}
	if result.VerifiedEpisodes != fateEpisodeCount || result.VerifiedMissions != fateMissionCount || result.Status.Completed != fateEpisodeCount || result.Status.MissionCompleted != fateMissionCount {
		t.Fatalf("write was not fully verified: %+v", result)
	}
	if result.AuxiliaryPreserved != fateAuxiliaryCount || result.PlaceholdersPreserved != fatePlaceholderCount {
		t.Fatalf("protected counts=%d/%d", result.AuxiliaryPreserved, result.PlaceholdersPreserved)
	}
	if result.BackupPath == "" {
		t.Fatal("in-place write did not report an automatic backup")
	}
	if _, err := os.Stat(result.BackupPath); err != nil {
		t.Fatalf("automatic backup is missing: %v", err)
	}

	second, err := completeAllFateEpisodesLocked(work)
	if err != nil {
		t.Fatalf("idempotent completion: %v", err)
	}
	if second.EpisodesChanged != 0 || second.MissionsChanged != 0 || second.BackupPath != "" {
		t.Fatalf("completed save should be a no-op without another backup: %+v", second)
	}
}

func TestCompleteAllFateEpisodesPublicEndpointRemainsDisabledUntilFieldAcceptance(t *testing.T) {
	if _, err := (&App{}).CompleteAllFateEpisodes("unused.dat"); err == nil || !strings.Contains(err.Error(), "只读研究") {
		t.Fatalf("public Fate write endpoint must fail closed, got %v", err)
	}
}
