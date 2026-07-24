package backend

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestClassifyRemoteCallWait(t *testing.T) {
	if err := classifyRemoteCallWait(uint32(windows.WAIT_OBJECT_0), nil); err != nil {
		t.Fatalf("completed thread = %v", err)
	}
	for _, tc := range []struct {
		name    string
		wait    uint32
		waitErr error
	}{
		{name: "timeout", wait: uint32(windows.WAIT_TIMEOUT)},
		{name: "wait failure", wait: 0xFFFFFFFF, waitErr: errors.New("wait failed")},
		{name: "unexpected", wait: uint32(windows.WAIT_ABANDONED)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := classifyRemoteCallWait(tc.wait, tc.waitErr)
			if !isRemoteCallIndeterminate(err) {
				t.Fatalf("result = %v, want indeterminate", err)
			}
		})
	}
}

func TestPruneUnlockedPatchCoreDLLsRemovesOnlyGeneratedDLLs(t *testing.T) {
	dir := t.TempDir()
	generated := filepath.Join(dir, "patch_core_123.dll")
	keep := filepath.Join(dir, "patch_core_command.txt")
	foreign := filepath.Join(dir, "other.dll")
	for _, path := range []string{generated, keep, foreign} {
		if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pruneUnlockedPatchCoreDLLs(dir)
	if _, err := os.Stat(generated); !os.IsNotExist(err) {
		t.Fatalf("generated DLL was not removed: %v", err)
	}
	for _, path := range []string{keep, foreign} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("unrelated temp file was removed: %v", err)
		}
	}
}

func TestRemoteCallIndeterminateSurvivesWrapping(t *testing.T) {
	err := errors.Join(errors.New("save failed"), newRemoteCallIndeterminateError("thread still running"))
	if !isRemoteCallIndeterminate(err) {
		t.Fatalf("wrapped error was not classified: %v", err)
	}
}

func TestClassifyRemoteCallWaitUsesOperationSpecificTimeout(t *testing.T) {
	err := classifyRemoteCallWaitWithTimeout(uint32(windows.WAIT_TIMEOUT), nil, "等待 10 秒后 DLL 仍未完成")
	if !isRemoteCallIndeterminate(err) || !strings.Contains(err.Error(), "10 秒") {
		t.Fatalf("DLL timeout classification = %v", err)
	}
}
