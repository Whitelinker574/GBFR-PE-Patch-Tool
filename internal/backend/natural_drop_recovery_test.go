package backend

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNaturalDropOwnedCurrentAfterExternalIndexRestore(t *testing.T) {
	for _, currentState := range []string{"deployed", "original"} {
		for _, condition := range []string{
			"intact", "unknown-index", "changed-table", "missing-table",
			"changed-backup", "missing-backup", "invalid-backup", "foreign-backup",
		} {
			t.Run(currentState+"/"+condition, func(t *testing.T) {
				gameDir := t.TempDir()
				gamePath := naturalDropItemTablePaths[0]
				relative := naturalDropRelativeTarget(gamePath)
				table := []byte("owned reward table")
				original := naturalDropTestIndex(t, nil)
				deployed := naturalDropTestIndex(t, map[string][]byte{gamePath: table})
				current := deployed
				if currentState == "original" {
					current = original
				}
				backup := original
				manifest := naturalDropManifest{
					SchemaVersion:     4,
					Owner:             naturalDropModID,
					GameVersion:       naturalDropGameVersion,
					GameExecutableSHA: runtimePatchCatalogGameSHA256,
					OriginalIndexSHA:  fileSHA256(original),
					DeployedIndexSHA:  fileSHA256(deployed),
					GeneratedFiles:    map[string]string{relative: fileSHA256(table)},
				}
				switch condition {
				case "unknown-index":
					current = naturalDropTestIndex(t, map[string][]byte{"other/mod.tbl": []byte("third-party")})
				case "changed-table":
					table = []byte("third-party reward table")
				case "missing-table":
					table = nil
				case "changed-backup":
					backup = []byte("corrupted backup")
				case "missing-backup":
					backup = nil
				case "invalid-backup", "foreign-backup":
					backup = []byte("not a data.i")
					if condition == "foreign-backup" {
						var err error
						backup, err = buildGBFRDataIndex(&gbfrDataIndex{Codename: "other-loader"})
						if err != nil {
							t.Fatal(err)
						}
					}
					// Matching hashes alone cannot make an invalid/foreign backup safe.
					manifest.OriginalIndexSHA = fileSHA256(backup)
					if currentState == "original" {
						current = backup
					}
				}
				manifestData, err := json.Marshal(manifest)
				if err != nil {
					t.Fatal(err)
				}
				before := map[string][]byte{
					"data.i": current, naturalDropBackupName: backup,
					naturalDropManifestName: manifestData, relative: table,
				}
				for relative, data := range before {
					if data == nil {
						continue
					}
					path := filepath.Join(gameDir, filepath.FromSlash(relative))
					if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, data, 0o644); err != nil {
						t.Fatal(err)
					}
				}
				readManifest, _, err := naturalDropReadManifest(gameDir)
				if err != nil {
					t.Fatal(err)
				}
				base, err := naturalDropVerifyOwnedCurrent(gameDir, readManifest, current)
				if condition == "intact" {
					if err != nil || !bytes.Equal(base, original) {
						t.Fatalf("known %s state rejected or wrong baseline: %v", currentState, err)
					}
					got, err := naturalDropBuildIndex(base, map[string][]byte{gamePath: table})
					if err != nil || !bytes.Equal(got, deployed) {
						t.Fatalf("redeploy after external restore changed the deployment: %v", err)
					}
				} else if err == nil || base != nil {
					t.Fatalf("unsafe %s state accepted: base=%x err=%v", condition, base, err)
				} else if condition == "unknown-index" {
					for _, digest := range []string{fileSHA256(current), manifest.OriginalIndexSHA, manifest.DeployedIndexSHA} {
						if !strings.Contains(err.Error(), digest) {
							t.Fatalf("unknown index error lacks diagnostic SHA-256 %s: %v", digest, err)
						}
					}
				}
				// Both accepted and rejected verification must leave every file alone.
				for relative, want := range before {
					got, err := os.ReadFile(filepath.Join(gameDir, filepath.FromSlash(relative)))
					if want == nil {
						if !os.IsNotExist(err) {
							t.Fatalf("verification created %s: %v", relative, err)
						}
					} else if err != nil || !bytes.Equal(got, want) {
						t.Fatalf("verification modified %s: %v", relative, err)
					}
				}
			})
		}
	}
}

func TestNaturalDropExternallyRestoredIndexTransactionRollback(t *testing.T) {
	for _, operation := range []string{"redeploy", "restore"} {
		t.Run(operation, func(t *testing.T) {
			gameDir := t.TempDir()
			original := naturalDropTestIndex(t, nil)
			gamePath := naturalDropItemTablePaths[0]
			relative := naturalDropRelativeTarget(gamePath)
			oldTable := []byte("old owned reward table")
			newTable := []byte("new owned reward table")
			beforeManifest := []byte("old manifest")
			afterManifest := []byte("new manifest")
			afterIndex := naturalDropTestIndex(t, map[string][]byte{gamePath: newTable})
			if operation == "restore" {
				afterIndex, newTable, afterManifest = original, nil, nil
			}
			before := map[string][]byte{
				"data.i": original, naturalDropBackupName: original,
				naturalDropManifestName: beforeManifest, relative: oldTable,
			}
			for relative, data := range before {
				path := filepath.Join(gameDir, filepath.FromSlash(relative))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := naturalDropBeginPreparedTransaction(gameDir, original, afterIndex,
				map[string][]byte{relative: oldTable}, beforeManifest,
				map[string][]byte{relative: newTable}, false, afterManifest); err != nil {
				t.Fatal(err)
			}
			if operation == "restore" {
				if err := naturalDropAddPreparedBackupTransition(gameDir, original, nil); err != nil {
					t.Fatal(err)
				}
			}
			after := map[string][]byte{
				"data.i": afterIndex, naturalDropManifestName: afterManifest, relative: newTable,
			}
			if operation == "restore" {
				after[naturalDropBackupName] = nil
			}
			for relative, data := range after {
				path := filepath.Join(gameDir, filepath.FromSlash(relative))
				var err error
				if data == nil {
					err = os.Remove(path)
				} else {
					err = naturalDropWriteAtomic(path, data)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := naturalDropRecoverPreparedTransaction(gameDir); err != nil {
				t.Fatal(err)
			}
			for relative, want := range before {
				got, err := os.ReadFile(filepath.Join(gameDir, filepath.FromSlash(relative)))
				if err != nil || !bytes.Equal(got, want) {
					t.Fatalf("rollback did not preserve externally restored %s: %v", relative, err)
				}
			}
		})
	}
}
