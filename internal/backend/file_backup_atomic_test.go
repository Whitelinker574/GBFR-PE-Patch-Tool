package backend

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExecutableBackupAndRestoreUseVerifiedAtomicReplacement(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "game.exe")
	original := []byte("original executable")
	modified := []byte("modified executable")
	if err := os.WriteFile(executable, original, 0o600); err != nil {
		t.Fatal(err)
	}
	app := &App{exePath: executable}
	if err := app.BackupFile(false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executable, modified, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := app.RestoreFile(); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != string(original) {
		t.Fatalf("restored executable = %q, want %q", restored, original)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".*.tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("atomic backup/restore left temporary files: %v", matches)
	}
}
