package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFailedGeneratorRetryIsBoundToExactSamePathBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SaveData1.dat")
	written := []byte("failed-commit")
	if err := os.WriteFile(path, written, 0o600); err != nil {
		t.Fatal(err)
	}
	retry := &failedGeneratorCommit{outputPath: path, written: append([]byte(nil), written...)}
	allowed, err := allowExactFailedCommitRetry(path, path, retry)
	if err != nil || !allowed {
		t.Fatalf("exact failed commit was not retryable: allowed=%v err=%v", allowed, err)
	}
	if err := os.WriteFile(path, []byte("newer-editor-write"), 0o600); err != nil {
		t.Fatal(err)
	}
	if allowed, err = allowExactFailedCommitRetry(path, path, retry); err == nil || allowed {
		t.Fatalf("newer save bytes were not protected: allowed=%v err=%v", allowed, err)
	}
	other := filepath.Join(t.TempDir(), "export.dat")
	if allowed, err = allowExactFailedCommitRetry(path, other, retry); err != nil || allowed {
		t.Fatalf("different output path incorrectly bypassed source validation: allowed=%v err=%v", allowed, err)
	}
}

func TestFailedGeneratorSeparateOutputMustRemainExactBeforeRetry(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "source.dat")
	output := filepath.Join(directory, "separate-output.dat")
	written := []byte("failed-commit-output")
	if err := os.WriteFile(source, []byte("unchanged-source"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, written, 0o600); err != nil {
		t.Fatal(err)
	}
	retry := &failedGeneratorCommit{outputPath: output, written: append([]byte(nil), written...)}
	allowed, err := allowExactFailedCommitRetry(source, output, retry)
	if err != nil || allowed {
		t.Fatalf("separate output should be verified without bypassing source validation: allowed=%v err=%v", allowed, err)
	}
	if err := os.WriteFile(output, []byte("newer-editor-write"), 0o600); err != nil {
		t.Fatal(err)
	}
	if allowed, err = allowExactFailedCommitRetry(source, output, retry); err == nil || allowed {
		t.Fatalf("modified separate output was not protected: allowed=%v err=%v", allowed, err)
	}
}

func TestReloadingSaveClearsFailedGeneratorCommitState(t *testing.T) {
	path := copyStatsSave(t)
	sigil := NewSigilGen()
	sigil.retryAfterFailedCommit = &failedGeneratorCommit{outputPath: path, written: []byte("old")}
	if _, err := sigil.LoadSaveFile(path); err != nil {
		t.Fatal(err)
	}
	if sigil.retryAfterFailedCommit != nil {
		t.Fatal("sigil generator kept failed-commit state after a successful reload")
	}
	wrightstone := NewWrightstoneGen()
	wrightstone.retryAfterFailedCommit = &failedGeneratorCommit{outputPath: path, written: []byte("old")}
	if _, err := wrightstone.LoadSaveFile(path); err != nil {
		t.Fatal(err)
	}
	if wrightstone.retryAfterFailedCommit != nil {
		t.Fatal("wrightstone generator kept failed-commit state after a successful reload")
	}
}

func TestOfflineSaveWritersShareOneTransactionLock(t *testing.T) {
	writers := map[string]int{
		"loadout_write.go":      1,
		"progression_editor.go": 1,
		"badge_store.go":        1,
		"save_app.go":           2,
		"sigil_gen.go":          3,
		"wrightstone_gen.go":    2,
		"save_backup.go":        1,
	}
	for file, minimum := range writers {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(source)
		if got := strings.Count(text, "offlineSaveMutationMu.Lock()"); got < minimum {
			t.Fatalf("%s has %d shared transaction locks, want at least %d", file, got, minimum)
		}
		if got := strings.Count(text, "defer offlineSaveMutationMu.Unlock()"); got < minimum {
			t.Fatalf("%s has %d shared transaction unlocks, want at least %d", file, got, minimum)
		}
	}
}

func TestLongLivedGeneratorsRefreshTheirSourceInsideTheTransaction(t *testing.T) {
	for file, marker := range map[string]string{
		"sigil_gen.go":       "ensureOfflineSaveSnapshotCurrent(sg.savePath, sg.save.data)",
		"wrightstone_gen.go": "ensureOfflineSaveSnapshotCurrent(wg.savePath, wg.save.data)",
	} {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(source), marker) {
			t.Fatalf("%s does not refresh its long-lived save snapshot before applying", file)
		}
	}
}
