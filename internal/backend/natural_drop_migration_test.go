package backend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Exercise the public operations with a real, verified current EXE/index and
// a simulated legacy installation. Never write to the user's game directory.
func TestNaturalDropMigrateLocalGameCopy(t *testing.T) {
	exeSource, indexSource := os.Getenv("GBFR_GAME_EXE_TEST"), os.Getenv("GBFR_DATA_INDEX_TEST")
	if exeSource == "" || indexSource == "" {
		t.Skip("set GBFR_GAME_EXE_TEST and GBFR_DATA_INDEX_TEST for cross-version deployment tests")
	}
	for _, operation := range []string{"redeploy", "restore"} {
		t.Run(operation, func(t *testing.T) {
			gameDir := t.TempDir()
			exePath := filepath.Join(gameDir, gameExeName)
			copyNaturalDropTestFile(t, exeSource, exePath)
			copyNaturalDropTestFile(t, indexSource, filepath.Join(gameDir, "data.i"))
			current, err := os.ReadFile(filepath.Join(gameDir, "data.i"))
			if err != nil {
				t.Fatal(err)
			}
			oldIndex := naturalDropTestIndex(t, nil)
			gamePath := naturalDropSummonTablePaths[0]
			relative := naturalDropRelativeTarget(gamePath)
			oldTable := []byte("previous tool-owned table")
			oldManifest := naturalDropManifest{
				SchemaVersion: 2, Owner: naturalDropModID,
				// Older tools retained the 2.0.2 label on later game indices.
				GameVersion: naturalDropGameVersion, GameExecutableSHA: runtimePatchCatalogGameSHA256,
				OriginalIndexSHA: fileSHA256(oldIndex),
				DeployedIndexSHA: fileSHA256(naturalDropTestIndex(t, map[string][]byte{gamePath: oldTable})),
				GeneratedFiles:   map[string]string{relative: fileSHA256(oldTable)},
			}
			manifestData, err := json.Marshal(oldManifest)
			if err != nil {
				t.Fatal(err)
			}
			oldFiles := map[string][]byte{naturalDropBackupName: oldIndex, naturalDropManifestName: manifestData, relative: oldTable}
			for relative, data := range oldFiles {
				path := filepath.Join(gameDir, filepath.FromSlash(relative))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			app := &App{configPathOverride: filepath.Join(t.TempDir(), "config.json")}
			workspace, err := app.GetNaturalDropWorkspace("", exePath)
			if err != nil || !workspace.MigrationReady || workspace.Installed {
				t.Fatalf("upgrade must be visible as migration-ready: %+v, %v", workspace, err)
			}
			if operation == "redeploy" {
				catalog, err := buildNaturalWrightstoneCatalog()
				if err != nil {
					t.Fatal(err)
				}
				family := catalog[0]
				result, err := app.DeployNaturalDropMod(NaturalDropDeployRequest{
					GameExePath:  exePath,
					Wrightstones: []NaturalDropWrightstoneSelection{{MainTrait: family.MainTrait.Hash, SubTrait1: family.SubTraits[0].Hash, SubTrait2: family.SubTraits[1].Hash}},
				})
				if err != nil {
					t.Fatalf("new original index with legacy deployment must redeploy: %v", err)
				}
				if result.SelectedWrightstones != 1 {
					t.Fatalf("wrong deployment: %+v", result)
				}
				if result.MigrationArchive == "" {
					t.Fatal("migration result did not report the preserved files")
				}
				manifest, _, err := naturalDropReadManifest(gameDir)
				if err != nil || manifest.OriginalIndexSHA != fileSHA256(current) {
					t.Fatalf("migration must use the NEW index as its baseline: %+v, %v", manifest, err)
				}
				backup, err := os.ReadFile(filepath.Join(gameDir, naturalDropBackupName))
				if err != nil || !bytes.Equal(backup, current) {
					t.Fatalf("backup was not rebased to the new original: %v", err)
				}
			}
			if err := app.RestoreNaturalDropDefaults(NaturalDropRestoreRequest{GameExePath: exePath}); err != nil {
				t.Fatalf("restore after a game upgrade must not downgrade data.i: %v", err)
			}
			restored, err := os.ReadFile(filepath.Join(gameDir, "data.i"))
			if err != nil || !bytes.Equal(restored, current) {
				t.Fatalf("restore replaced the new original index: %v", err)
			}
			for relative := range oldFiles {
				if _, err := os.Stat(filepath.Join(gameDir, filepath.FromSlash(relative))); !os.IsNotExist(err) {
					t.Fatalf("old active file remains: %s: %v", relative, err)
				}
			}
			archives, err := filepath.Glob(filepath.Join(gameDir, naturalDropHistoryDirectoryName, "migration-*"))
			if err != nil || len(archives) != 1 {
				t.Fatalf("expected one preserved legacy generation: %v, %v", archives, err)
			}
			oldFiles["data.i"] = current
			for relative, want := range oldFiles {
				got, err := os.ReadFile(filepath.Join(archives[0], filepath.FromSlash(relative)))
				if err != nil || !bytes.Equal(got, want) {
					t.Fatalf("legacy evidence lost: %s: %v", relative, err)
				}
			}
			if err := app.RestoreNaturalDropDefaults(NaturalDropRestoreRequest{GameExePath: exePath}); err == nil {
				t.Fatal("a second restore must not invent another owned installation")
			}
		})
	}
}

func TestNaturalDropKnownOriginalIndexPairs(t *testing.T) {
	for _, tc := range []struct{ version, exe, index string }{
		{naturalDropGame203Version, game203ExecutableSHA256, "F7C4A2D3AE65B85CEC96EF3A77B8C3A370A9A02584C6A77DD7AF5C676B1511EE"},
		{naturalDropGame204Version, game204ExecutableSHA256, "F7C4A2D3AE65B85CEC96EF3A77B8C3A370A9A02584C6A77DD7AF5C676B1511EE"},
		{naturalDropGame205Version, game205ExecutableSHA256, "4FA5FDE9A94D3F995CB3DCFADB1238DCA739CDD313C15BD59109E9DBF3385802"},
		{naturalDropGame206Version, game206ExecutableSHA256, "0A6B9989F0F0E59C4E67D5128A115DFB12560397A3421BDCCFB65D1DEBBB73F9"},
	} {
		t.Run(tc.version, func(t *testing.T) {
			identity := naturalDropGameIdentityRecord{Version: tc.version, SHA256: tc.exe}
			if !naturalDropKnownOriginalIndex(identity, tc.index) {
				t.Fatal("measured matching version pair rejected")
			}
			if naturalDropKnownOriginalIndex(identity, "unknown") || naturalDropKnownOriginalIndex(naturalDropGameIdentityRecord{Version: tc.version, SHA256: "foreign"}, tc.index) {
				t.Fatal("unknown executable/index pair accepted")
			}
		})
	}
}

func TestNaturalDropMigrationRollbackEveryStage(t *testing.T) {
	for _, operation := range []string{"redeploy", "restore"} {
		for stage := 0; stage <= 4; stage++ {
			t.Run(fmt.Sprintf("%s/stage-%d", operation, stage), func(t *testing.T) {
				dir := t.TempDir()
				oldIndex := naturalDropTestIndex(t, nil)
				current := naturalDropTestIndex(t, map[string][]byte{"new-version-resource": []byte("new")})
				relative := naturalDropRelativeTarget(naturalDropItemTablePaths[0])
				before := map[string][]byte{"data.i": current, naturalDropBackupName: oldIndex, naturalDropManifestName: []byte("old manifest"), relative: []byte("old table")}
				after := map[string][]byte{"data.i": naturalDropTestIndex(t, map[string][]byte{naturalDropItemTablePaths[0]: []byte("new table")}), naturalDropBackupName: current, naturalDropManifestName: []byte("new manifest"), relative: []byte("new table")}
				order := []string{naturalDropBackupName, relative, "data.i", naturalDropManifestName}
				if operation == "restore" {
					after = map[string][]byte{"data.i": current, naturalDropBackupName: nil, naturalDropManifestName: nil, relative: nil}
					order = []string{"data.i", relative, naturalDropManifestName, naturalDropBackupName}
				}
				for relative, data := range before {
					path := filepath.Join(dir, filepath.FromSlash(relative))
					if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, data, 0o644); err != nil {
						t.Fatal(err)
					}
				}
				archive, err := naturalDropArchiveLegacyInstall(dir, current, oldIndex, before[naturalDropManifestName], map[string][]byte{relative: before[relative]})
				if err != nil {
					t.Fatal(err)
				}
				if err := naturalDropBeginPreparedTransaction(dir, current, after["data.i"], map[string][]byte{relative: before[relative]}, before[naturalDropManifestName], map[string][]byte{relative: after[relative]}, false, after[naturalDropManifestName]); err != nil {
					t.Fatal(err)
				}
				if err := naturalDropAddPreparedBackupTransition(dir, oldIndex, after[naturalDropBackupName]); err != nil {
					t.Fatal(err)
				}
				for _, relative := range order[:stage] {
					path := filepath.Join(dir, filepath.FromSlash(relative))
					var err error
					if after[relative] == nil {
						err = os.Remove(path)
					} else {
						err = naturalDropWriteAtomic(path, after[relative])
					}
					if err != nil {
						t.Fatal(err)
					}
				}
				for attempt := 0; attempt < 2; attempt++ {
					if err := naturalDropRecoverPreparedTransaction(dir); err != nil {
						t.Fatal(err)
					}
					for relative, want := range before {
						got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(relative)))
						if err != nil || !bytes.Equal(got, want) {
							t.Fatalf("interrupted migration did not preserve %s: %v", relative, err)
						}
					}
				}
				if _, err := os.Stat(filepath.Join(archive, "archive.json")); err != nil {
					t.Fatalf("rollback removed the permanent history: %v", err)
				}
			})
		}
	}
}

func TestNaturalDropMigrationRejectsUnownedPaths(t *testing.T) {
	dir := t.TempDir()
	foreign := "data/other-mod.tbl"
	manifest := &naturalDropManifest{GeneratedFiles: map[string]string{foreign: fileSHA256([]byte("foreign"))}}
	if _, err := naturalDropVerifyOwnedFiles(dir, manifest); err == nil {
		t.Fatal("a manifest must not confer ownership of arbitrary paths")
	}
	if _, err := naturalDropArchiveLegacyInstall(dir, nil, nil, nil, map[string][]byte{foreign: []byte("foreign")}); err == nil {
		t.Fatal("archive accepted an unowned target")
	}
	if _, err := os.Stat(filepath.Join(dir, naturalDropHistoryDirectoryName)); !os.IsNotExist(err) {
		t.Fatal("unsafe archive request modified the filesystem")
	}
}

func TestNaturalDropMigrationRejectsUnsafeLocalGameCopy(t *testing.T) {
	exeSource, indexSource := os.Getenv("GBFR_GAME_EXE_TEST"), os.Getenv("GBFR_DATA_INDEX_TEST")
	if exeSource == "" || indexSource == "" {
		t.Skip("set GBFR_GAME_EXE_TEST and GBFR_DATA_INDEX_TEST for real-index migration guard tests")
	}
	for _, operation := range []string{"redeploy", "restore"} {
		for _, condition := range []string{"changed-table", "missing-table", "changed-backup", "unknown-index", "archive-blocked", "foreign-target"} {
			if operation == "restore" && condition == "foreign-target" {
				continue // Removal does not touch the new target at all.
			}
			t.Run(operation+"/"+condition, func(t *testing.T) {
				dir := t.TempDir()
				exePath := filepath.Join(dir, gameExeName)
				copyNaturalDropTestFile(t, exeSource, exePath)
				current, err := os.ReadFile(indexSource)
				if err != nil {
					t.Fatal(err)
				}
				original := naturalDropTestIndex(t, nil)
				relative := naturalDropRelativeTarget(naturalDropSummonTablePaths[0])
				table := []byte("old tool table")
				manifest, err := json.Marshal(naturalDropManifest{SchemaVersion: 2, Owner: naturalDropModID,
					GameVersion: naturalDropGameVersion, GameExecutableSHA: runtimePatchCatalogGameSHA256,
					OriginalIndexSHA: fileSHA256(original), DeployedIndexSHA: "old deployed hash", GeneratedFiles: map[string]string{relative: fileSHA256(table)}})
				if err != nil {
					t.Fatal(err)
				}
				before := map[string][]byte{"data.i": current, naturalDropBackupName: original, naturalDropManifestName: manifest, relative: table}
				switch condition {
				case "changed-table":
					before[relative] = []byte("third-party changes")
				case "missing-table":
					before[relative] = nil
				case "changed-backup":
					before[naturalDropBackupName] = []byte("bad backup")
				case "unknown-index":
					before["data.i"] = naturalDropTestIndex(t, map[string][]byte{"other/mod.tbl": []byte("unknown")})
				case "archive-blocked":
					before[naturalDropHistoryDirectoryName] = []byte("not a directory")
				case "foreign-target":
					before[naturalDropRelativeTarget(naturalDropWrightstoneTablePaths[0])] = []byte("another mod")
				}
				for relative, data := range before {
					if data == nil {
						continue
					}
					path := filepath.Join(dir, filepath.FromSlash(relative))
					if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, data, 0o644); err != nil {
						t.Fatal(err)
					}
				}
				app := &App{configPathOverride: filepath.Join(t.TempDir(), "config.json")}
				if operation == "restore" {
					err = app.RestoreNaturalDropDefaults(NaturalDropRestoreRequest{GameExePath: exePath})
				} else {
					catalog, catalogErr := buildNaturalWrightstoneCatalog()
					if catalogErr != nil {
						t.Fatal(catalogErr)
					}
					family := catalog[0]
					_, err = app.DeployNaturalDropMod(NaturalDropDeployRequest{GameExePath: exePath, Wrightstones: []NaturalDropWrightstoneSelection{{MainTrait: family.MainTrait.Hash, SubTrait1: family.SubTraits[0].Hash, SubTrait2: family.SubTraits[1].Hash}}})
				}
				if err == nil {
					t.Fatal("unsafe migration was accepted")
				}
				for relative, want := range before {
					got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(relative)))
					if want == nil && os.IsNotExist(err) {
						continue
					}
					if err != nil || !bytes.Equal(got, want) {
						t.Fatalf("rejected migration modified %s: %v", relative, err)
					}
				}
				if _, err := os.Stat(filepath.Join(dir, naturalDropPrepareJournalName)); !os.IsNotExist(err) {
					t.Fatal("rejected migration left an active transaction")
				}
			})
		}
	}
}
